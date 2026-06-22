package agent

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/crypto"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

// Helper to create a signature for testing
func computeSignature(secret, keyID, slug, method, path, contentType string, ts int64, nonce string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	bodyHashBase64 := base64.RawURLEncoding.EncodeToString(bodyHash[:])
	canonicalString := fmt.Sprintf(
		"BCHAT-BRIDGE-V1\n%s\n%s\n%s\n%s\n%s\n%d\n%s\n%s",
		keyID,
		slug,
		strings.ToUpper(method),
		path,
		"application/json", // Exactly application/json
		ts,
		nonce,
		bodyHashBase64,
	)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonicalString))
	return "v1=" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func setupMiddlewareTestStore(t *testing.T, ctx context.Context, slug string, active bool) (*store.Store, *store.AgentTenant, *store.BridgeAuthKey, *crypto.EncryptionService) {
	ts := teststore.NewTestingStore(ctx, t)
	
	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug:        slug,
		CompanyName: slug,
		Vertical:    "test",
		IsActive:    active,
	})
	require.NoError(t, err)

	masterKey := "super-secure-master-key-12345"
	salt := []byte("12345678901236") // 16 bytes for salt
	enc := crypto.NewEncryptionService(masterKey, salt)

	plainSecret := "secret-key-material-999"
	encrypted, nonce, err := enc.Encrypt(plainSecret)
	require.NoError(t, err)

	key := &store.BridgeAuthKey{
		TenantID:           tenant.ID,
		KeyID:              "my-test-key-id-123",
		SecretKeyEncrypted: encrypted,
		SecretKeyNonce:     nonce,
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, err = ts.CreateBridgeAuthKey(ctx, key)
	require.NoError(t, err)

	return ts, tenant, key, enc
}

func TestBridgeAuthMiddlewareValidSignature(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "valid-sig", true)
	defer ts.Close()

	e := echo.New()
	handlerCalled := false
	middleware := RequireBridgeHMAC(ts, enc)
	
	handler := middleware(func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{"message": "hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/valid-sig/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "valid-sig", http.MethodPost, "/api/v1/agent/valid-sig/chat", "application/json", now, nonce, body)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("valid-sig")

	err := handler(c)
	require.NoError(t, err)
	require.True(t, handlerCalled)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestBridgeAuthMiddlewareInvalidSignature(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "invalid-sig", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{"message": "hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/invalid-sig/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", "v1=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))) // Valid base64url, wrong signature

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("invalid-sig")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthMiddlewareQueryParamKeysRejected(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "query-rejected", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{"message": "hello"}`)
	
	for _, param := range []string{"key_id", "bridge_key", "bridge_client_id", "client_id", "api_key", "signature", "nonce"} {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/agent/query-rejected/chat?%s=some-val", param), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		now := time.Now().Unix()
		nonce := "random_nonce_12345"
		sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "query-rejected", http.MethodPost, "/api/v1/agent/query-rejected/chat", "application/json", now, nonce, body)

		req.Header.Set("Authorization", "Bearer my-test-key-id-123")
		req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
		req.Header.Set("X-Bridge-Nonce", nonce)
		req.Header.Set("X-Bridge-Signature", sig)

		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug")
		c.SetParamValues("query-rejected")

		err := handler(c)
		require.Error(t, err)
		var echoErr *echo.HTTPError
		require.True(t, errors.As(err, &echoErr))
		require.Equal(t, http.StatusBadRequest, echoErr.Code)
		require.Equal(t, store.ErrBridgeAuthMalformedRequest.Error(), echoErr.Message)
	}
}

func TestBridgeAuthMiddlewareStrictNoLogs(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "strict-logs", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{"message": "hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/strict-logs/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "log_nonce_123456789"
	sig := "v1=" + base64.RawURLEncoding.EncodeToString([]byte("malformed-or-wrong-sig-here-abcdef"))

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	// Capture slog output
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("strict-logs")

	err := handler(c)
	require.Error(t, err)

	logOutput := buf.String()
	require.NotContains(t, logOutput, "secret-key-material-999")
	require.NotContains(t, logOutput, "hello")
	require.NotContains(t, logOutput, sig)
	require.NotContains(t, logOutput, nonce)
	require.NotContains(t, logOutput, "BCHAT-BRIDGE-V1")
}

func TestBridgeAuthMiddlewareConcurrentNonceReplay(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "concurrent-replay", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{"message": "hello"}`)
	now := time.Now().Unix()
	nonce := "concurrent_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "concurrent-replay", http.MethodPost, "/api/v1/agent/concurrent-replay/chat", "application/json", now, nonce, body)

	var wg sync.WaitGroup
	results := make(chan error, 5)
	
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/concurrent-replay/chat", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer my-test-key-id-123")
			req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
			req.Header.Set("X-Bridge-Nonce", nonce)
			req.Header.Set("X-Bridge-Signature", sig)

			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("slug")
			c.SetParamValues("concurrent-replay")

			results <- handler(c)
		}()
	}
	wg.Wait()
	close(results)

	successCount := 0
	conflictCount := 0

	for err := range results {
		if err == nil {
			successCount++
		} else {
			var echoErr *echo.HTTPError
			if errors.As(err, &echoErr) {
				if echoErr.Code == http.StatusConflict {
					conflictCount++
					require.Equal(t, store.ErrBridgeAuthReplay.Error(), echoErr.Message)
				}
			}
		}
	}

	require.Equal(t, 1, successCount)
	require.Equal(t, 4, conflictCount)
}

func TestBridgeAuthMiddlewareExpiredTimestamp(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "expired-ts", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{"message": "hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/expired-ts/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	// Timestamp 10 minutes in the past
	now := time.Now().Add(-10 * time.Minute).Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "expired-ts", http.MethodPost, "/api/v1/agent/expired-ts/chat", "application/json", now, nonce, body)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("expired-ts")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidTimestamp.Error(), echoErr.Message)
}

func TestBridgeAuthMiddlewareMissingHeaders(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "missing-headers", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/missing-headers/chat", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("missing-headers")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusBadRequest, echoErr.Code) // Missing Content-Type or Auth headers is HTTP 400
}

func TestBridgeAuthMiddlewareMaxBodySizeLimit(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "body-limit", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Body slightly larger than 1 MiB
	largeBody := make([]byte, 1*1024*1024+10)
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/body-limit/chat", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", "v1=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32)))

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("body-limit")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusRequestEntityTooLarge, echoErr.Code)
}

func TestBridgeAuthDoesNotRegisterBridgeEndpoints(t *testing.T) {
	content, err := os.ReadFile("../v1.go")
	require.NoError(t, err)
	s := string(content)

	require.NotContains(t, s, "RequireBridgeHMAC")
	require.NotContains(t, s, "takeover")
	require.NotContains(t, s, "reply")
	require.NotContains(t, s, "release")
}

func TestBridgeAuthDoesNotChangeChatExternal(t *testing.T) {
	content, err := os.ReadFile("../v1.go")
	require.NoError(t, err)
	s := string(content)

	// Ensure HandleChatExternal is public and does not have RequireBridgeHMAC applied to it
	require.Contains(t, s, "publicGroup.POST(\"/:slug/chat/ext\", s.agentHandler.HandleChatExternal)")
}

func TestBridgeAuthMiddlewareNilEncryptionService(t *testing.T) {
	ctx := context.Background()
	ts, _, _, _ := setupMiddlewareTestStore(t, ctx, "nil-enc", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, nil) // Nil encryption service
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/nil-enc/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Bridge-Nonce", "random_nonce_12345")
	req.Header.Set("X-Bridge-Signature", "v1=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32)))

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("nil-enc")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusInternalServerError, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthSecretUnavailable.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsInactiveTenant(t *testing.T) {
	ctx := context.Background()
	// Create inactive tenant
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "inactive-tenant", false)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/inactive-tenant/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Bridge-Nonce", "random_nonce_12345")
	req.Header.Set("X-Bridge-Signature", "v1=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32)))

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("inactive-tenant")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInactiveTenant.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsNonNumericTimestamp(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "non-numeric-ts", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/non-numeric-ts/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", "not-a-number-123")
	req.Header.Set("X-Bridge-Nonce", "random_nonce_12345")
	req.Header.Set("X-Bridge-Signature", "v1=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32)))

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("non-numeric-ts")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidTimestamp.Error(), echoErr.Message)
}

func TestBridgeAuthLastUsedAtUpdateFailureDoesNotFailRequest(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "last-used-fail", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	
	handlerCalled := false
	handler := middleware(func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/last-used-fail/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "last_used_nonce_99"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "last-used-fail", http.MethodPost, "/api/v1/agent/last-used-fail/chat", "application/json", now, nonce, body)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	// Create a SQLite trigger to force update to fail
	_, err := ts.GetDriver().GetDB().ExecContext(ctx, `
		CREATE TRIGGER force_update_fail BEFORE UPDATE OF last_used_at ON bridge_auth_keys
		BEGIN
			SELECT RAISE(FAIL, 'forced update failure');
		END;
	`)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("last-used-fail")

	err = handler(c)
	require.NoError(t, err)
	require.True(t, handlerCalled)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestBridgeAuthRejectsContentTypeMismatch(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "ct-mismatch", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/ct-mismatch/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain") // Wrong Content-Type
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "ct-mismatch", http.MethodPost, "/api/v1/agent/ct-mismatch/chat", "text/plain", now, nonce, body)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("ct-mismatch")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusBadRequest, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthMalformedRequest.Error(), echoErr.Message)
}

func TestBridgeAuthCanonicalizesJSONContentType(t *testing.T) {
	ctx := context.Background()
	
	for _, contentType := range []string{"application/json; charset=utf-8", "application/json; charset=UTF-8"} {
		ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "ct-canon", true)

		e := echo.New()
		handlerCalled := false
		middleware := RequireBridgeHMAC(ts, enc)
		handler := middleware(func(c echo.Context) error {
			handlerCalled = true
			return c.String(http.StatusOK, "OK")
		})

		body := []byte(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/ct-canon/chat", bytes.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		
		now := time.Now().Unix()
		nonce := "random_nonce_12345"
		sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "ct-canon", http.MethodPost, "/api/v1/agent/ct-canon/chat", contentType, now, nonce, body)

		req.Header.Set("Authorization", "Bearer my-test-key-id-123")
		req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
		req.Header.Set("X-Bridge-Nonce", nonce)
		req.Header.Set("X-Bridge-Signature", sig)

		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug")
		c.SetParamValues("ct-canon")

		err := handler(c)
		require.NoError(t, err)
		require.True(t, handlerCalled)
		require.Equal(t, http.StatusOK, rec.Code)
		ts.Close()
	}
}

func TestBridgeAuthRejectsMissingContentType(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "ct-missing", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/ct-missing/chat", bytes.NewReader(body))
	// No Content-Type header
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "ct-missing", http.MethodPost, "/api/v1/agent/ct-missing/chat", "", now, nonce, body)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("ct-missing")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusBadRequest, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthMalformedRequest.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsMalformedContentType(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "ct-malformed", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/ct-malformed/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; malformed-parameter") // Malformed Content-Type
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "ct-malformed", http.MethodPost, "/api/v1/agent/ct-malformed/chat", "application/json; malformed-parameter", now, nonce, body)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("ct-malformed")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusBadRequest, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthMalformedRequest.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsNonUTF8JSONCharset(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "ct-charset", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/ct-charset/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=iso-8859-1") // Non-UTF-8 charset
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "ct-charset", http.MethodPost, "/api/v1/agent/ct-charset/chat", "application/json; charset=iso-8859-1", now, nonce, body)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("ct-charset")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusBadRequest, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthMalformedRequest.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsMalformedBase64Signature(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "sig-base64", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sig-base64/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", "v1=!!!!_invalid_base64_symbols_!!!!") // Malformed base64url

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("sig-base64")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsUnsupportedSignatureVersion(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "sig-version", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sig-version/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", "v2=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))) // Unsupported version

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("sig-version")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsEmptySignatureValue(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "sig-empty", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sig-empty/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", "v1=") // Empty signature value

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("sig-empty")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsHexSignature(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "sig-hex", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sig-hex/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	hexSig := hex.EncodeToString(make([]byte, 32)) // Hex signature format (64 chars)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", "v1="+hexSig) // hex instead of base64url

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("sig-hex")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsWrongSignature(t *testing.T) {
	TestBridgeAuthMiddlewareInvalidSignature(t)
}

func TestBridgeAuthAcceptsBase64URLSignature(t *testing.T) {
	TestBridgeAuthMiddlewareValidSignature(t)
}

func TestBridgeAuthRejectsRevokedKey(t *testing.T) {
	ctx := context.Background()
	ts, tenant, key, enc := setupMiddlewareTestStore(t, ctx, "sig-revoked", true)
	defer ts.Close()

	// Revoke the key
	err := ts.RevokeBridgeAuthKey(ctx, tenant.ID, key.KeyID, time.Now())
	require.NoError(t, err)

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sig-revoked/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "sig-revoked", http.MethodPost, "/api/v1/agent/sig-revoked/chat", "application/json", now, nonce, body)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("sig-revoked")

	err = handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthKeyRevoked.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsWrongTenant(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "tenant-a", true)
	defer ts.Close()

	// Create tenant B
	tenantB, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug:        "tenant-b",
		CompanyName: "tenant-b",
		Vertical:    "test",
		IsActive:    true,
	})
	require.NoError(t, err)

	// Create key for tenant B with same ID but different secret
	plainSecretB := "secret-key-material-tenant-b-999"
	encryptedB, nonceB, err := enc.Encrypt(plainSecretB)
	require.NoError(t, err)

	keyB := &store.BridgeAuthKey{
		TenantID:           tenantB.ID,
		KeyID:              "my-test-key-id-123", // same Key ID
		SecretKeyEncrypted: encryptedB,
		SecretKeyNonce:     nonceB,
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, err = ts.CreateBridgeAuthKey(ctx, keyB)
	require.NoError(t, err)

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	
	// Sign for tenant A
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sigA := computeSignature("secret-key-material-999", "my-test-key-id-123", "tenant-a", http.MethodPost, "/api/v1/agent/tenant-b/chat", "application/json", now, nonce, body)

	// Send to tenant B
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/tenant-b/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sigA) // signed for tenant-a, not tenant-b!

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("tenant-b")

	err = handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsWrongPath(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "wrong-path", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	
	// Sign for "/expected-path"
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "wrong-path", http.MethodPost, "/api/v1/agent/wrong-path/expected-path", "application/json", now, nonce, body)

	// Send to different path
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/wrong-path/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("wrong-path")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsWrongMethod(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "wrong-method", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	
	// Sign for POST
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "wrong-method", http.MethodPost, "/api/v1/agent/wrong-method/chat", "application/json", now, nonce, body)

	// Send as PUT
	req := httptest.NewRequest(http.MethodPut, "/api/v1/agent/wrong-method/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("wrong-method")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsBodyTampering(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "body-tamper", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Sign for body A
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "body-tamper", http.MethodPost, "/api/v1/agent/body-tamper/chat", "application/json", now, nonce, []byte(`{"body": "A"}`))

	// Send body B
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/body-tamper/chat", bytes.NewReader([]byte(`{"body": "B"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("body-tamper")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}

func TestBridgeAuthRejectsReplayNonce(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "replay-nonce", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	now := time.Now().Unix()
	nonce := "replay_nonce_val_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "replay-nonce", http.MethodPost, "/api/v1/agent/replay-nonce/chat", "application/json", now, nonce, body)

	// Send once
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/replay-nonce/chat", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce)
	req1.Header.Set("X-Bridge-Signature", sig)

	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	c1.SetParamNames("slug")
	c1.SetParamValues("replay-nonce")

	err := handler(c1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec1.Code)

	// Send twice (replay)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/replay-nonce/chat", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce)
	req2.Header.Set("X-Bridge-Signature", sig)

	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetParamNames("slug")
	c2.SetParamValues("replay-nonce")

	err = handler(c2)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusConflict, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthReplay.Error(), echoErr.Message)
}

func TestBridgeAuthRestoresRequestBody(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "restore-body", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	
	bodyBytes := []byte(`{"value": "correct-restored-data"}`)
	var downstreamReadBytes []byte

	handler := middleware(func(c echo.Context) error {
		var err error
		downstreamReadBytes, err = io.ReadAll(c.Request().Body)
		require.NoError(t, err)
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/restore-body/chat", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "restore-body", http.MethodPost, "/api/v1/agent/restore-body/chat", "application/json", now, nonce, bodyBytes)

	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("restore-body")

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, bodyBytes, downstreamReadBytes)
}

func TestBridgeAuthRejectsLowercaseBearerScheme(t *testing.T) {
	ctx := context.Background()
	ts, _, _, enc := setupMiddlewareTestStore(t, ctx, "bearer-case", true)
	defer ts.Close()

	e := echo.New()
	middleware := RequireBridgeHMAC(ts, enc)
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/bearer-case/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	now := time.Now().Unix()
	nonce := "random_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", "bearer-case", http.MethodPost, "/api/v1/agent/bearer-case/chat", "application/json", now, nonce, body)

	req.Header.Set("Authorization", "bearer my-test-key-id-123") // Lowercase bearer
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("bearer-case")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusUnauthorized, echoErr.Code)
	require.Equal(t, store.ErrBridgeAuthInvalidSignature.Error(), echoErr.Message)
}
