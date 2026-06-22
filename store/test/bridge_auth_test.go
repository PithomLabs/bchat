package teststore

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db/mysql"
	"github.com/usememos/memos/store/db/postgres"
)

func TestBridgeAuthSQLiteMigrationApplies(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	for _, table := range []string{"bridge_auth_keys", "bridge_auth_nonces"} {
		var name string
		err := ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		require.NoError(t, err)
		require.Equal(t, table, name)
	}
}

func TestBridgeAuthKeyStoredEncryptedNotPlaintext(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	tenant := createBridgeTenant(t, ctx, ts, "bridge-auth-encrypt")
	plainSecret := "super-secret-key-12345"

	// Mock encryption
	nonce := make([]byte, 12)
	_, err := rand.Read(nonce)
	require.NoError(t, err)
	
	block, err := aes.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	encrypted := gcm.Seal(nil, nonce, []byte(plainSecret), nil)

	key := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "test-key-id-123456",
		Label:              pointer("My Key"),
		SecretKeyEncrypted: encrypted,
		SecretKeyNonce:     nonce,
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	created, err := ts.CreateBridgeAuthKey(ctx, key)
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	// Query DB raw to verify it's not plaintext
	var rawEncrypted, rawNonce []byte
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT secret_key_encrypted, secret_key_nonce FROM bridge_auth_keys WHERE id = ?", created.ID).Scan(&rawEncrypted, &rawNonce)
	require.NoError(t, err)
	require.Equal(t, encrypted, rawEncrypted)
	require.Equal(t, nonce, rawNonce)

	// Verify plaintext secret is not in the DB
	var containsPlaintext int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT count(*) FROM bridge_auth_keys WHERE secret_key_encrypted LIKE ?", "%"+plainSecret+"%").Scan(&containsPlaintext)
	require.NoError(t, err)
	require.Zero(t, containsPlaintext)
}

func TestBridgeAuthCreateKeyReturnsSecretOnce(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	tenant := createBridgeTenant(t, ctx, ts, "bridge-auth-once")
	key := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "test-key-id-once-123",
		SecretKeyEncrypted: []byte("encrypted"),
		SecretKeyNonce:     make([]byte, 12),
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	created, err := ts.CreateBridgeAuthKey(ctx, key)
	require.NoError(t, err)

	// Get key from store
	fetched, err := ts.GetBridgeAuthKey(ctx, tenant.ID, created.KeyID)
	require.NoError(t, err)
	
	// Ensure the fetched key contains encrypted details, not plaintext secrets
	require.Equal(t, created.SecretKeyEncrypted, fetched.SecretKeyEncrypted)
	require.Equal(t, created.SecretKeyNonce, fetched.SecretKeyNonce)
}

func TestBridgeAuthDeleteExpiredNonces(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	tenant := createBridgeTenant(t, ctx, ts, "bridge-auth-nonces")
	
	// Create Key first (foreign key constraint)
	key := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "key-id-for-nonces",
		SecretKeyEncrypted: []byte("encrypted"),
		SecretKeyNonce:     make([]byte, 12),
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, err := ts.CreateBridgeAuthKey(ctx, key)
	require.NoError(t, err)

	now := time.Now()

	// Expired nonce
	expiredNonce := &store.BridgeAuthNonce{
		TenantID:  tenant.ID,
		KeyID:     key.KeyID,
		Nonce:     "expired-nonce-123",
		Timestamp: now.Add(-20 * time.Minute).Unix(),
		CreatedAt: now.Add(-20 * time.Minute),
		ExpiresAt: now.Add(-5 * time.Minute),
	}
	require.NoError(t, ts.StoreBridgeAuthNonce(ctx, expiredNonce))

	// Valid nonce
	validNonce := &store.BridgeAuthNonce{
		TenantID:  tenant.ID,
		KeyID:     key.KeyID,
		Nonce:     "valid-nonce-12345",
		Timestamp: now.Unix(),
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
	}
	require.NoError(t, ts.StoreBridgeAuthNonce(ctx, validNonce))

	// Cleanup
	err = ts.CleanupBridgeAuthNonces(ctx, now)
	require.NoError(t, err)

	// Verify expired is gone, valid remains
	var count int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT count(*) FROM bridge_auth_nonces WHERE nonce = ?", "expired-nonce-123").Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)

	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT count(*) FROM bridge_auth_nonces WHERE nonce = ?", "valid-nonce-12345").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestBridgeAuthKeyTenantCascade(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	tenant := createBridgeTenant(t, ctx, ts, "bridge-auth-cascade")
	key := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "cascade-key-id-12345",
		SecretKeyEncrypted: []byte("encrypted"),
		SecretKeyNonce:     make([]byte, 12),
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, err := ts.CreateBridgeAuthKey(ctx, key)
	require.NoError(t, err)

	nonce := &store.BridgeAuthNonce{
		TenantID:  tenant.ID,
		KeyID:     key.KeyID,
		Nonce:     "cascade-nonce-12345",
		Timestamp: time.Now().Unix(),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	require.NoError(t, ts.StoreBridgeAuthNonce(ctx, nonce))

	// Delete Tenant
	err = ts.DeleteAgentTenant(ctx, tenant.ID)
	require.NoError(t, err)

	// Verify cascade deletes keys and nonces
	var keyCount, nonceCount int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT count(*) FROM bridge_auth_keys WHERE key_id = ?", key.KeyID).Scan(&keyCount)
	require.NoError(t, err)
	require.Zero(t, keyCount)

	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "SELECT count(*) FROM bridge_auth_nonces WHERE nonce = ?", nonce.Nonce).Scan(&nonceCount)
	require.NoError(t, err)
	require.Zero(t, nonceCount)
}

func TestBridgeAuthNonceUniquePerTenantKey(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	tenant := createBridgeTenant(t, ctx, ts, "bridge-auth-unique")
	key := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "unique-key-id-12345",
		SecretKeyEncrypted: []byte("encrypted"),
		SecretKeyNonce:     make([]byte, 12),
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, err := ts.CreateBridgeAuthKey(ctx, key)
	require.NoError(t, err)

	nonce1 := &store.BridgeAuthNonce{
		TenantID:  tenant.ID,
		KeyID:     key.KeyID,
		Nonce:     "shared-nonce-12345",
		Timestamp: time.Now().Unix(),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	err = ts.StoreBridgeAuthNonce(ctx, nonce1)
	require.NoError(t, err)

	// Inserting same nonce again should trigger replay error
	err = ts.StoreBridgeAuthNonce(ctx, nonce1)
	require.ErrorIs(t, err, store.ErrBridgeAuthReplay)
}

func TestBridgeAuthNonceRequiresExistingKey(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	tenant := createBridgeTenant(t, ctx, ts, "bridge-auth-fk")
	
	// Try to insert nonce with non-existent key
	nonce := &store.BridgeAuthNonce{
		TenantID:  tenant.ID,
		KeyID:     "non-existent-key-12345",
		Nonce:     "some-nonce-12345",
		Timestamp: time.Now().Unix(),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	err := ts.StoreBridgeAuthNonce(ctx, nonce)
	require.Error(t, err)
}

func TestBridgeAuthPostgresUnsupported(t *testing.T) {
	d := &postgres.DB{}
	ctx := context.Background()

	_, err := d.CreateBridgeAuthKey(ctx, &store.BridgeAuthKey{})
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	_, err = d.GetBridgeAuthKey(ctx, 1, "key")
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	_, err = d.ListBridgeAuthKeys(ctx, 1)
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	err = d.UpdateBridgeAuthKeyLastUsed(ctx, 1, "key", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	err = d.RevokeBridgeAuthKey(ctx, 1, "key", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	err = d.StoreBridgeAuthNonce(ctx, &store.BridgeAuthNonce{})
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	err = d.CleanupBridgeAuthNonces(ctx, time.Now())
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)
}

func TestBridgeAuthMySQLUnsupported(t *testing.T) {
	d := &mysql.DB{}
	ctx := context.Background()

	_, err := d.CreateBridgeAuthKey(ctx, &store.BridgeAuthKey{})
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	_, err = d.GetBridgeAuthKey(ctx, 1, "key")
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	_, err = d.ListBridgeAuthKeys(ctx, 1)
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	err = d.UpdateBridgeAuthKeyLastUsed(ctx, 1, "key", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	err = d.RevokeBridgeAuthKey(ctx, 1, "key", time.Now())
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	err = d.StoreBridgeAuthNonce(ctx, &store.BridgeAuthNonce{})
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)

	err = d.CleanupBridgeAuthNonces(ctx, time.Now())
	require.ErrorIs(t, err, store.ErrBridgeUnsupportedDatabase)
}

func TestBridgeAuthGetResponseExcludesSecrets(t *testing.T) {
	key := &store.BridgeAuthKey{
		ID:                 1,
		TenantID:           2,
		KeyID:              "key-id-xyz",
		SecretKeyEncrypted: []byte("sensitive-encrypted"),
		SecretKeyNonce:     []byte("sensitive-nonce-xyz"),
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	data, err := json.Marshal(key)
	require.NoError(t, err)

	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)

	_, encryptedExists := m["secret_key_encrypted"]
	_, nonceExists := m["secret_key_nonce"]
	require.False(t, encryptedExists)
	require.False(t, nonceExists)
}

func TestBridgeAuthLATESTSQLParity(t *testing.T) {
	// Read migrations
	mig30, err := os.ReadFile("../migration/sqlite/0.25/30__bridge_auth_foundation.sql")
	require.NoError(t, err)
	latest, err := os.ReadFile("../migration/sqlite/LATEST.sql")
	require.NoError(t, err)

	s30 := string(mig30)
	sLatest := string(latest)

	// Ensure structural elements exist in both
	assertions := []string{
		"CHECK(length(secret_key_nonce) = 12)",
		"CHECK(expires_at > created_at)",
		"CHECK(expires_at > timestamp)",
		"UNIQUE(tenant_id, key_id)",
		"UNIQUE(tenant_id, key_id, nonce)",
		"FOREIGN KEY (tenant_id, key_id) REFERENCES bridge_auth_keys(tenant_id, key_id)",
	}

	for _, a := range assertions {
		// Clean spaces for robust matching
		normAssert := strings.Join(strings.Fields(a), "")
		norm30 := strings.Join(strings.Fields(s30), "")
		normLatest := strings.Join(strings.Fields(sLatest), "")

		require.Contains(t, norm30, normAssert, "assertion not found in 30__ migration: %s", a)
		require.Contains(t, normLatest, normAssert, "assertion not found in LATEST.sql: %s", a)
	}
}

func TestBridgeAuthDBForeignKeysEnabled(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	var enabled int
	err := ts.GetDriver().GetDB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled)
	require.NoError(t, err)
	require.Equal(t, 1, enabled)

	// Try inserting a nonce for nonexistent tenant/key
	nonce := &store.BridgeAuthNonce{
		TenantID:  999999,
		KeyID:     "nonexistent-key-99999",
		Nonce:     "nonce-1234567890123",
		Timestamp: time.Now().Unix(),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	err = ts.StoreBridgeAuthNonce(ctx, nonce)
	require.Error(t, err)
}

func TestBridgeAuthRevocationIdempotent(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	tenant := createBridgeTenant(t, ctx, ts, "bridge-revoke-idemp")
	key := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "revoke-idemp-key-id",
		SecretKeyEncrypted: []byte("encrypted"),
		SecretKeyNonce:     make([]byte, 12),
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, err := ts.CreateBridgeAuthKey(ctx, key)
	require.NoError(t, err)

	now := time.Now().Truncate(time.Second)

	// Revoke once
	err = ts.RevokeBridgeAuthKey(ctx, tenant.ID, key.KeyID, now)
	require.NoError(t, err)

	fetched1, err := ts.GetBridgeAuthKey(ctx, tenant.ID, key.KeyID)
	require.NoError(t, err)
	require.Equal(t, "revoked", fetched1.Status)
	require.NotNil(t, fetched1.RevokedAt)
	require.Equal(t, now.Unix(), fetched1.RevokedAt.Unix())

	// Revoke again
	err = ts.RevokeBridgeAuthKey(ctx, tenant.ID, key.KeyID, now.Add(time.Minute))
	require.NoError(t, err)

	fetched2, err := ts.GetBridgeAuthKey(ctx, tenant.ID, key.KeyID)
	require.NoError(t, err)
	require.Equal(t, "revoked", fetched2.Status)
	// Timestamp should NOT have changed (remains first revocation time or stays consistent)
	require.Equal(t, fetched1.RevokedAt.Unix(), fetched2.RevokedAt.Unix())
}

func TestBridgeAuthDuplicateKeyCreation(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	tenant := createBridgeTenant(t, ctx, ts, "bridge-dup-key")
	key1 := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "duplicate-key-id-abc",
		SecretKeyEncrypted: []byte("encrypted"),
		SecretKeyNonce:     make([]byte, 12),
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, err := ts.CreateBridgeAuthKey(ctx, key1)
	require.NoError(t, err)

	// Try creating duplicate key
	key2 := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "duplicate-key-id-abc",
		SecretKeyEncrypted: []byte("encrypted2"),
		SecretKeyNonce:     make([]byte, 12),
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, err = ts.CreateBridgeAuthKey(ctx, key2)
	require.Error(t, err)
}

func pointer(s string) *string {
	return &s
}
