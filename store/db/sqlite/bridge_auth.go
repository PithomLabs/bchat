package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateBridgeAuthKey(ctx context.Context, key *store.BridgeAuthKey) (*store.BridgeAuthKey, error) {
	var labelVal any
	if key.Label != nil {
		labelVal = *key.Label
	}

	result, err := d.db.ExecContext(ctx, `
		INSERT INTO bridge_auth_keys (
			tenant_id, key_id, label, secret_key_encrypted, secret_key_nonce, status, created_at, updated_at, last_used_at, revoked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, key.TenantID, key.KeyID, labelVal, key.SecretKeyEncrypted, key.SecretKeyNonce, key.Status, key.CreatedAt.Unix(), key.UpdatedAt.Unix(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create bridge auth key: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get bridge auth key insert id: %w", err)
	}
	key.ID = id
	return key, nil
}

func (d *DB) GetBridgeAuthKey(ctx context.Context, tenantID int32, keyID string) (*store.BridgeAuthKey, error) {
	var key store.BridgeAuthKey
	var label sql.NullString
	var status string
	var createdAt, updatedAt int64
	var lastUsedAt, revokedAt sql.NullInt64

	err := d.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, key_id, label, secret_key_encrypted, secret_key_nonce, status, created_at, updated_at, last_used_at, revoked_at
		FROM bridge_auth_keys
		WHERE tenant_id = ? AND key_id = ?
	`, tenantID, keyID).Scan(
		&key.ID, &key.TenantID, &key.KeyID, &label, &key.SecretKeyEncrypted, &key.SecretKeyNonce,
		&status, &createdAt, &updatedAt, &lastUsedAt, &revokedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrBridgeAuthKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bridge auth key: %w", err)
	}

	key.Label = nullableStringPtr(label)
	key.Status = status
	key.CreatedAt = time.Unix(createdAt, 0)
	key.UpdatedAt = time.Unix(updatedAt, 0)
	key.LastUsedAt = nullableUnixTime(lastUsedAt)
	key.RevokedAt = nullableUnixTime(revokedAt)

	return &key, nil
}

func (d *DB) ListBridgeAuthKeys(ctx context.Context, tenantID int32) ([]*store.BridgeAuthKey, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, key_id, label, secret_key_encrypted, secret_key_nonce, status, created_at, updated_at, last_used_at, revoked_at
		FROM bridge_auth_keys
		WHERE tenant_id = ?
		ORDER BY id DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list bridge auth keys: %w", err)
	}
	defer rows.Close()

	var keys []*store.BridgeAuthKey
	for rows.Next() {
		var key store.BridgeAuthKey
		var label sql.NullString
		var status string
		var createdAt, updatedAt int64
		var lastUsedAt, revokedAt sql.NullInt64

		err := rows.Scan(
			&key.ID, &key.TenantID, &key.KeyID, &label, &key.SecretKeyEncrypted, &key.SecretKeyNonce,
			&status, &createdAt, &updatedAt, &lastUsedAt, &revokedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan bridge auth key: %w", err)
		}

		key.Label = nullableStringPtr(label)
		key.Status = status
		key.CreatedAt = time.Unix(createdAt, 0)
		key.UpdatedAt = time.Unix(updatedAt, 0)
		key.LastUsedAt = nullableUnixTime(lastUsedAt)
		key.RevokedAt = nullableUnixTime(revokedAt)

		keys = append(keys, &key)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list bridge auth keys rows: %w", err)
	}

	return keys, nil
}

func (d *DB) UpdateBridgeAuthKeyLastUsed(ctx context.Context, tenantID int32, keyID string, now time.Time) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE bridge_auth_keys
		SET last_used_at = ?
		WHERE tenant_id = ? AND key_id = ?
	`, now.Unix(), tenantID, keyID)
	if err != nil {
		return fmt.Errorf("update bridge auth key last used: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read last used result: %w", err)
	}
	if rows == 0 {
		return store.ErrBridgeAuthKeyNotFound
	}
	return nil
}

func (d *DB) RevokeBridgeAuthKey(ctx context.Context, tenantID int32, keyID string, now time.Time) error {
	result, err := d.db.ExecContext(ctx, `
		UPDATE bridge_auth_keys
		SET status = 'revoked', revoked_at = ?, updated_at = ?
		WHERE tenant_id = ? AND key_id = ? AND status = 'active'
	`, now.Unix(), now.Unix(), tenantID, keyID)
	if err != nil {
		return fmt.Errorf("revoke bridge auth key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read revoke result: %w", err)
	}
	if rows == 0 {
		var exists int
		err := d.db.QueryRowContext(ctx, `
			SELECT 1 FROM bridge_auth_keys WHERE tenant_id = ? AND key_id = ?
		`, tenantID, keyID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrBridgeAuthKeyNotFound
		}
		return nil
	}
	return nil
}

func (d *DB) StoreBridgeAuthNonce(ctx context.Context, nonce *store.BridgeAuthNonce) error {
	result, err := d.db.ExecContext(ctx, `
		INSERT INTO bridge_auth_nonces (
			tenant_id, key_id, nonce, timestamp, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, key_id, nonce) DO NOTHING
	`, nonce.TenantID, nonce.KeyID, nonce.Nonce, nonce.Timestamp, nonce.CreatedAt.Unix(), nonce.ExpiresAt.Unix())
	if err != nil {
		return fmt.Errorf("store bridge auth nonce: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read nonce result: %w", err)
	}
	if rows == 0 {
		return store.ErrBridgeAuthReplay
	}
	return nil
}

func (d *DB) CleanupBridgeAuthNonces(ctx context.Context, now time.Time) error {
	_, err := d.db.ExecContext(ctx, `
		DELETE FROM bridge_auth_nonces
		WHERE expires_at <= ?
	`, now.Unix())
	if err != nil {
		return fmt.Errorf("cleanup bridge auth nonces: %w", err)
	}
	return nil
}
