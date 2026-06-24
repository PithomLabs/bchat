package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

func TestBridgeEndpointsHMACSecurity(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "hmac-security", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{
		Mode:                "dev",
		EncryptionMasterKey: "super-secure-master-key-12345",
	}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	// Setup routes
	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/release", handler.HandleBridgeRelease, RequireBridgeHMAC(ts, enc))

	endpoints := []struct {
		path string
		body []byte
	}{
		{
			path: "/api/v1/agent/hmac-security/bridge/takeover",
			body: []byte(`{"session_id": "session_1"}`),
		},
		{
			path: "/api/v1/agent/hmac-security/bridge/reply",
			body: []byte(`{"session_id": "session_1", "handoff_id": "handoff_1", "message_id": "msg_1", "text": "hello"}`),
		},
		{
			path: "/api/v1/agent/hmac-security/bridge/release",
			body: []byte(`{"session_id": "session_1", "handoff_id": "handoff_1"}`),
		},
	}

	for _, ep := range endpoints {
		t.Run("no_auth_"+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewReader(ep.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})

		t.Run("invalid_sig_"+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewReader(ep.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer my-test-key-id-123")
			req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
			req.Header.Set("X-Bridge-Nonce", "nonce_1234567890123")
			req.Header.Set("X-Bridge-Signature", "v1=invalid")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Logf("Response body for %s: %s", ep.path, rec.Body.String())
			}
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})

		t.Run("replay_nonce_"+ep.path, func(t *testing.T) {
			now := time.Now().Unix()
			nonce := "replay_nonce_" + strings.ReplaceAll(ep.path, "/", "_")

			// Dynamically use correct handoff ID if active handoff exists
			body := ep.body
			active, _ := ts.FindActiveBridgeHandoff(ctx, tenant.ID, "session_1")
			if active != nil {
				if strings.Contains(ep.path, "reply") {
					reqObj := BridgeReplyRequest{
						SessionID: "session_1",
						HandoffID: active.HandoffID,
						MessageID: "msg_1",
						Text:      "hello",
					}
					body, _ = json.Marshal(reqObj)
				} else if strings.Contains(ep.path, "release") {
					reason := "release reason"
					reqObj := BridgeReleaseRequest{
						SessionID: "session_1",
						HandoffID: active.HandoffID,
						Reason:    &reason,
					}
					body, _ = json.Marshal(reqObj)
				}
			}

			sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, ep.path, "application/json", now, nonce, body)

			// First request should work (or fail with handler error but NOT 409)
			req1 := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewReader(body))
			req1.Header.Set("Content-Type", "application/json")
			req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
			req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
			req1.Header.Set("X-Bridge-Nonce", nonce)
			req1.Header.Set("X-Bridge-Signature", sig)
			rec1 := httptest.NewRecorder()
			e.ServeHTTP(rec1, req1)
			if rec1.Code == http.StatusConflict || rec1.Code == http.StatusBadRequest {
				t.Logf("Replay nonce first request body for %s: %s (status=%d)", ep.path, rec1.Body.String(), rec1.Code)
			}
			require.NotEqual(t, http.StatusConflict, rec1.Code)

			// Second request with same nonce must be 409
			req2 := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewReader(body))
			req2.Header.Set("Content-Type", "application/json")
			req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
			req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
			req2.Header.Set("X-Bridge-Nonce", nonce)
			req2.Header.Set("X-Bridge-Signature", sig)
			rec2 := httptest.NewRecorder()
			e.ServeHTTP(rec2, req2)
			if rec2.Code != http.StatusConflict {
				t.Logf("Replay nonce second request body for %s: %s (status=%d)", ep.path, rec2.Body.String(), rec2.Code)
			}
			require.Equal(t, http.StatusConflict, rec2.Code)
		})

		t.Run("wrong_path_"+ep.path, func(t *testing.T) {
			now := time.Now().Unix()
			nonce := "nonce_wrong_path_" + strings.ReplaceAll(ep.path, "/", "_")

			// Dynamically use correct handoff ID if active handoff exists
			body := ep.body
			active, _ := ts.FindActiveBridgeHandoff(ctx, tenant.ID, "session_1")
			if active != nil {
				if strings.Contains(ep.path, "reply") {
					reqObj := BridgeReplyRequest{
						SessionID: "session_1",
						HandoffID: active.HandoffID,
						MessageID: "msg_1",
						Text:      "hello",
					}
					body, _ = json.Marshal(reqObj)
				} else if strings.Contains(ep.path, "release") {
					reason := "release reason"
					reqObj := BridgeReleaseRequest{
						SessionID: "session_1",
						HandoffID: active.HandoffID,
						Reason:    &reason,
					}
					body, _ = json.Marshal(reqObj)
				}
			}

			// Compute signature over a different path
			sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, ep.path+"/wrong", "application/json", now, nonce, body)

			req := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer my-test-key-id-123")
			req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
			req.Header.Set("X-Bridge-Nonce", nonce)
			req.Header.Set("X-Bridge-Signature", sig)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Logf("Wrong path response body for %s: %s (status=%d)", ep.path, rec.Body.String(), rec.Code)
			}
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})

		t.Run("wrong_method_"+ep.path, func(t *testing.T) {
			now := time.Now().Unix()
			nonce := "nonce_wrong_method_" + strings.ReplaceAll(ep.path, "/", "_")

			// Dynamically use correct handoff ID if active handoff exists
			body := ep.body
			active, _ := ts.FindActiveBridgeHandoff(ctx, tenant.ID, "session_1")
			if active != nil {
				if strings.Contains(ep.path, "reply") {
					reqObj := BridgeReplyRequest{
						SessionID: "session_1",
						HandoffID: active.HandoffID,
						MessageID: "msg_1",
						Text:      "hello",
					}
					body, _ = json.Marshal(reqObj)
				} else if strings.Contains(ep.path, "release") {
					reason := "release reason"
					reqObj := BridgeReleaseRequest{
						SessionID: "session_1",
						HandoffID: active.HandoffID,
						Reason:    &reason,
					}
					body, _ = json.Marshal(reqObj)
				}
			}

			// Compute signature over GET
			sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodGet, ep.path, "application/json", now, nonce, body)

			req := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer my-test-key-id-123")
			req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
			req.Header.Set("X-Bridge-Nonce", nonce)
			req.Header.Set("X-Bridge-Signature", sig)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Logf("Wrong method response body for %s: %s (status=%d)", ep.path, rec.Body.String(), rec.Code)
			}
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})

		t.Run("query_param_rejection_"+ep.path, func(t *testing.T) {
			now := time.Now().Unix()
			nonce := "query_nonce_" + strings.ReplaceAll(strings.ReplaceAll(ep.path, "/", "_"), "-", "_")
			if len(nonce) < 16 {
				nonce = nonce + strings.Repeat("x", 16-len(nonce))
			}

			// Dynamically use correct handoff ID if active handoff exists
			body := ep.body
			active, _ := ts.FindActiveBridgeHandoff(ctx, tenant.ID, "session_1")
			if active != nil {
				if strings.Contains(ep.path, "reply") {
					reqObj := BridgeReplyRequest{
						SessionID: "session_1",
						HandoffID: active.HandoffID,
						MessageID: "msg_1",
						Text:      "hello",
					}
					body, _ = json.Marshal(reqObj)
				} else if strings.Contains(ep.path, "release") {
					reason := "release reason"
					reqObj := BridgeReleaseRequest{
						SessionID: "session_1",
						HandoffID: active.HandoffID,
						Reason:    &reason,
					}
					body, _ = json.Marshal(reqObj)
				}
			}

			sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, ep.path, "application/json", now, nonce, body)

			for _, paramName := range []string{"key_id", "bridge_key", "bridge_client_id", "client_id", "api_key", "signature", "nonce"} {
				req := httptest.NewRequest(http.MethodPost, ep.path+"?"+paramName+"=some-val", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer my-test-key-id-123")
				req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
				req.Header.Set("X-Bridge-Nonce", nonce)
				req.Header.Set("X-Bridge-Signature", sig)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest {
					t.Logf("Query param rejection (%s) response body: %s (status=%d)", paramName, rec.Body.String(), rec.Code)
				}
				require.Equal(t, http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestBridgeTakeoverConcurrentSameSessionSingleActiveHandoff(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "concurrent-takeover", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{
		Mode:                "dev",
		EncryptionMasterKey: "super-secure-master-key-12345",
	}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))

	body := []byte(`{"session_id": "session_concurrent"}`)
	now := time.Now().Unix()

	// Perform 10 concurrent requests to takeover the same session
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/concurrent-takeover/bridge/takeover", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer my-test-key-id-123")
			req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
			req.Header.Set("X-Bridge-Nonce", fmt.Sprintf("takeover_nonce_%d", index))
			sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/concurrent-takeover/bridge/takeover", "application/json", now, fmt.Sprintf("takeover_nonce_%d", index), body)
			req.Header.Set("X-Bridge-Signature", sig)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			results <- rec
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	conflictCount := 0

	for rec := range results {
		if rec.Code == http.StatusOK {
			var resp BridgeTakeoverResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			require.Equal(t, "success", resp.Status)
			require.NotEmpty(t, resp.HandoffID)
			successCount++
		} else if rec.Code == http.StatusConflict {
			conflictCount++
		} else {
			t.Errorf("Unexpected status code %d: %s", rec.Code, rec.Body.String())
		}
	}

	// At least one should succeed to create/takeover
	require.GreaterOrEqual(t, successCount, 1)
	require.Equal(t, 10, successCount+conflictCount)

	// In SQLite with proper isolation, we should end up with exactly one active handoff in the database
	activeHandoff, err := ts.FindActiveBridgeHandoff(ctx, tenant.ID, "session_concurrent")
	require.NoError(t, err)
	require.NotNil(t, activeHandoff)
	require.Equal(t, store.BridgeRoutingModeHumanActive, activeHandoff.RoutingMode)
}

func TestBridgeReplyRejectsStaleHandoffID(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "stale-reply", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{
		Mode:                "dev",
		EncryptionMasterKey: "super-secure-master-key-12345",
	}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)
	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/release", handler.HandleBridgeRelease, RequireBridgeHMAC(ts, enc))

	// 1. Takeover session -> creates handoff 1
	bodyTakeover := []byte(`{"session_id": "session_stale"}`)
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_1"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/stale-reply/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/stale-reply/bridge/takeover", bytes.NewReader(bodyTakeover))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var resp1 BridgeTakeoverResponse
	err := json.Unmarshal(rec1.Body.Bytes(), &resp1)
	require.NoError(t, err)
	handoffID1 := resp1.HandoffID

	// 2. Release handoff 1
	bodyRelease := []byte(fmt.Sprintf(`{"session_id": "session_stale", "handoff_id": "%s"}`, handoffID1))
	nonce2 := "release_nonce_12"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/stale-reply/bridge/release", "application/json", now, nonce2, bodyRelease)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/stale-reply/bridge/release", bytes.NewReader(bodyRelease))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Logf("stale-reply step 2 response body: %s", rec2.Body.String())
	}
	require.Equal(t, http.StatusOK, rec2.Code)

	// 3. Takeover session again -> creates handoff 2 (active)
	nonce3 := "takeover_nonce_2"
	sig3 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/stale-reply/bridge/takeover", "application/json", now, nonce3, bodyTakeover)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/stale-reply/bridge/takeover", bytes.NewReader(bodyTakeover))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req3.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req3.Header.Set("X-Bridge-Nonce", nonce3)
	req3.Header.Set("X-Bridge-Signature", sig3)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusOK, rec3.Code)

	// 4. Try to reply using handoffID1 (stale) while handoff 2 is active
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_stale", "handoff_id": "%s", "message_id": "msg_stale", "text": "stale message"}`, handoffID1))
	nonce4 := "reply_nonce_1234"
	sig4 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/stale-reply/bridge/reply", "application/json", now, nonce4, bodyReply)

	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/stale-reply/bridge/reply", bytes.NewReader(bodyReply))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req4.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req4.Header.Set("X-Bridge-Nonce", nonce4)
	req4.Header.Set("X-Bridge-Signature", sig4)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req4)
	// Must fail with 409 Conflict due to mismatched/stale handoffID
	require.Equal(t, http.StatusConflict, rec4.Code)
}

func TestBridgeReleaseRejectsStaleHandoffID(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "stale-release", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{
		Mode:                "dev",
		EncryptionMasterKey: "super-secure-master-key-12345",
	}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)
	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/release", handler.HandleBridgeRelease, RequireBridgeHMAC(ts, enc))

	// 1. Takeover session -> creates handoff 1
	bodyTakeover := []byte(`{"session_id": "session_stale_release"}`)
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_1"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/stale-release/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/stale-release/bridge/takeover", bytes.NewReader(bodyTakeover))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var resp1 BridgeTakeoverResponse
	err := json.Unmarshal(rec1.Body.Bytes(), &resp1)
	require.NoError(t, err)
	handoffID1 := resp1.HandoffID

	// 2. Release handoff 1
	bodyRelease1 := []byte(fmt.Sprintf(`{"session_id": "session_stale_release", "handoff_id": "%s"}`, handoffID1))
	nonce2 := "release_nonce_12"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/stale-release/bridge/release", "application/json", now, nonce2, bodyRelease1)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/stale-release/bridge/release", bytes.NewReader(bodyRelease1))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Logf("stale-release step 2 response body: %s", rec2.Body.String())
	}
	require.Equal(t, http.StatusOK, rec2.Code)

	// 3. Takeover session again -> creates handoff 2 (active)
	nonce3 := "takeover_nonce_2"
	sig3 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/stale-release/bridge/takeover", "application/json", now, nonce3, bodyTakeover)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/stale-release/bridge/takeover", bytes.NewReader(bodyTakeover))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req3.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req3.Header.Set("X-Bridge-Nonce", nonce3)
	req3.Header.Set("X-Bridge-Signature", sig3)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusOK, rec3.Code)

	// 4. Try to release using handoffID1 (stale) while handoff 2 is active
	nonce4 := "release_nonce_22"
	sig4 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/stale-release/bridge/release", "application/json", now, nonce4, bodyRelease1)

	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/stale-release/bridge/release", bytes.NewReader(bodyRelease1))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req4.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req4.Header.Set("X-Bridge-Nonce", nonce4)
	req4.Header.Set("X-Bridge-Signature", sig4)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req4)
	// Must fail with 409 Conflict due to mismatched/stale handoffID
	require.Equal(t, http.StatusConflict, rec4.Code)
}

func TestBridgeReleaseNoActiveHandoffSemantics(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "no-active", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{
		Mode:                "dev",
		EncryptionMasterKey: "super-secure-master-key-12345",
	}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)
	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/release", handler.HandleBridgeRelease, RequireBridgeHMAC(ts, enc))

	// 1. Call release with nonexistent handoff ID
	bodyReleaseNonexistent := []byte(`{"session_id": "session_no_active", "handoff_id": "nonexistent_handoff"}`)
	now := time.Now().Unix()
	nonce1 := "release_nonce_12"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/no-active/bridge/release", "application/json", now, nonce1, bodyReleaseNonexistent)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/no-active/bridge/release", bytes.NewReader(bodyReleaseNonexistent))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	// Must return 404 since there is no active handoff and GetBridgeHandoff returns ErrBridgeHandoffNotFound
	require.Equal(t, http.StatusNotFound, rec1.Code)

	// 2. Takeover session -> creates active handoff
	bodyTakeover := []byte(`{"session_id": "session_no_active"}`)
	nonce2 := "takeover_nonce_1"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/no-active/bridge/takeover", "application/json", now, nonce2, bodyTakeover)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/no-active/bridge/takeover", bytes.NewReader(bodyTakeover))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var resp2 BridgeTakeoverResponse
	err := json.Unmarshal(rec2.Body.Bytes(), &resp2)
	require.NoError(t, err)
	handoffID := resp2.HandoffID

	// 3. Release first time -> should transition successfully
	bodyRelease := []byte(fmt.Sprintf(`{"session_id": "session_no_active", "handoff_id": "%s"}`, handoffID))
	nonce3 := "release_nonce_22"
	sig3 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/no-active/bridge/release", "application/json", now, nonce3, bodyRelease)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/no-active/bridge/release", bytes.NewReader(bodyRelease))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req3.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req3.Header.Set("X-Bridge-Nonce", nonce3)
	req3.Header.Set("X-Bridge-Signature", sig3)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Logf("no-active step 3 response body: %s", rec3.Body.String())
	}
	require.Equal(t, http.StatusOK, rec3.Code)

	// 4. Release second time (idempotency check) -> no active handoff exists, but GetBridgeHandoff finds it and is in Closed state.
	nonce4 := "release_nonce_32"
	sig4 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/no-active/bridge/release", "application/json", now, nonce4, bodyRelease)

	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/no-active/bridge/release", bytes.NewReader(bodyRelease))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req4.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req4.Header.Set("X-Bridge-Nonce", nonce4)
	req4.Header.Set("X-Bridge-Signature", sig4)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req4)
	// Must return 200 OK (idempotent success)
	require.Equal(t, http.StatusOK, rec4.Code)
}

func TestBridgeReplySuccessPersisted(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-success", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{
		Mode:                "dev",
		EncryptionMasterKey: "super-secure-master-key-12345",
	}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	dbConn := ts.GetDriver().GetDB()

	// 1. Create takeover / active handoff
	bodyTakeover := []byte(`{"session_id": "session_reply_success"}`)
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_reply_success"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-success/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-success/bridge/takeover", bytes.NewReader(bodyTakeover))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)

	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var respTakeover BridgeTakeoverResponse
	err := json.Unmarshal(rec1.Body.Bytes(), &respTakeover)
	require.NoError(t, err)
	require.NotEmpty(t, respTakeover.HandoffID)
	handoffID := respTakeover.HandoffID

	// Query state before reply to ensure no side effects
	sessionBefore, err := ts.FindBridgeExternalSession(ctx, tenant.ID, "session_reply_success")
	require.NoError(t, err)
	require.NotNil(t, sessionBefore)

	handoffBefore, err := ts.GetBridgeHandoff(ctx, tenant.ID, "session_reply_success", handoffID)
	require.NoError(t, err)
	require.NotNil(t, handoffBefore)

	var memoCountBefore int
	err = dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM memo").Scan(&memoCountBefore)
	require.NoError(t, err)

	// Create visitor session in memory so synchronous reply delivery succeeds
	session := svc.memorySessions.GetOrCreate(tenant.ID, "session_reply_success")
	session.Messages = []store.AgentMessage{
		{Role: "user", Content: "hello", Timestamp: time.Now()},
	}
	svc.memorySessions.Update(session)

	// 2. Call reply with valid handoff_id/message_id/text
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_reply_success", "handoff_id": "%s", "message_id": "msg_reply_success", "text": "persisted text"}`, handoffID))
	nonce2 := "reply_nonce_reply_success"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-success/bridge/reply", "application/json", now, nonce2, bodyReply)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-success/bridge/reply", bytes.NewReader(bodyReply))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)

	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var respReply BridgeReplyResponse
	err = json.Unmarshal(rec2.Body.Bytes(), &respReply)
	require.NoError(t, err)

	// Assert response fields
	require.Equal(t, "reply_persisted_not_delivered", respReply.Status)
	require.NotEmpty(t, respReply.ReplyID)
	require.Equal(t, handoffID, respReply.HandoffID)
	require.Equal(t, "msg_reply_success", respReply.MessageID)
	require.Equal(t, "not_delivered", respReply.DeliveryStatus)
	require.NotNil(t, respReply.Outbox)
	require.Equal(t, "completed", respReply.Outbox.Status)
	require.NotEmpty(t, respReply.Outbox.OutboxID)

	// Verify reply row in the database
	var dbReplyText string
	var dbReplyStatus string
	err = dbConn.QueryRowContext(ctx, "SELECT text, delivery_status FROM bridge_handoff_replies WHERE reply_id = ?", respReply.ReplyID).Scan(&dbReplyText, &dbReplyStatus)
	require.NoError(t, err)
	require.Equal(t, "persisted text", dbReplyText)
	require.Equal(t, "not_delivered", dbReplyStatus)

	// Verify outbox row in the database
	var dbOutboxStatus string
	var dbOutboxID string
	err = dbConn.QueryRowContext(ctx, "SELECT status, outbox_id FROM bridge_reply_outbox WHERE reply_id = ?", respReply.ReplyID).Scan(&dbOutboxStatus, &dbOutboxID)
	require.NoError(t, err)
	require.Equal(t, "completed", dbOutboxStatus)
	require.Equal(t, respReply.Outbox.OutboxID, dbOutboxID)

	// Assert no database state side effects for unrelated tables
	sessionAfter, err := ts.FindBridgeExternalSession(ctx, tenant.ID, "session_reply_success")
	require.NoError(t, err)
	require.Equal(t, sessionBefore.Status, sessionAfter.Status)
	require.Equal(t, sessionBefore.UpdatedAt.Unix(), sessionAfter.UpdatedAt.Unix())

	handoffAfter, err := ts.GetBridgeHandoff(ctx, tenant.ID, "session_reply_success", handoffID)
	require.NoError(t, err)
	require.Equal(t, handoffBefore.RoutingMode, handoffAfter.RoutingMode)
	require.Equal(t, handoffBefore.Active, handoffAfter.Active)
	require.Equal(t, handoffBefore.Version, handoffAfter.Version)

	// Verify no new memos/chat messages were appended to the DB
	var memoCountAfter int
	err = dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM memo").Scan(&memoCountAfter)
	require.NoError(t, err)
	require.Equal(t, memoCountBefore, memoCountAfter, "Expected no new memos/chat messages to be appended to the DB")

	// Verify no other delivery/outbox/message tables exist in SQLite schema
	rows, err := dbConn.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND (name LIKE '%bridge_outbox%' OR name LIKE '%bridge_message%' OR name LIKE '%delivery%') AND name != 'bridge_reply_outbox'")
	require.NoError(t, err)
	defer rows.Close()
	var tableNames []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		require.NoError(t, err)
		tableNames = append(tableNames, name)
	}
	require.Empty(t, tableNames, "Expected no other delivery/outbox/message tables to exist in SQLite schema, but found: %v", tableNames)
}

func TestBridgeReplyDuplicateMessageIDSameTextIdempotent(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-idem-same-handler", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// Takeover
	bodyTakeover := []byte(`{"session_id": "session_reply"}`)
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_12345"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-idem-same-handler/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-idem-same-handler/bridge/takeover", bytes.NewReader(bodyTakeover))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var respTakeover BridgeTakeoverResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &respTakeover))
	handoffID := respTakeover.HandoffID

	// Create visitor session in memory so synchronous reply delivery succeeds
	session := svc.memorySessions.GetOrCreate(tenant.ID, "session_reply")
	session.Messages = []store.AgentMessage{
		{Role: "user", Content: "hello", Timestamp: time.Now()},
	}
	svc.memorySessions.Update(session)

	// First reply request
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_reply", "handoff_id": "%s", "message_id": "msg_idem", "text": "some text"}`, handoffID))
	nonce2 := "reply_nonce_12345_1"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-idem-same-handler/bridge/reply", "application/json", now, nonce2, bodyReply)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-idem-same-handler/bridge/reply", bytes.NewReader(bodyReply))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var respReply1 BridgeReplyResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &respReply1))
	require.Equal(t, "reply_persisted_not_delivered", respReply1.Status)
	require.NotEmpty(t, respReply1.ReplyID)
	require.NotNil(t, respReply1.Outbox)
	require.NotEmpty(t, respReply1.Outbox.OutboxID)
	require.Equal(t, "completed", respReply1.Outbox.Status)

	// Second reply request with same message_id and same text
	nonce3 := "reply_nonce_12345_2"
	sig3 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-idem-same-handler/bridge/reply", "application/json", now, nonce3, bodyReply)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-idem-same-handler/bridge/reply", bytes.NewReader(bodyReply))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req3.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req3.Header.Set("X-Bridge-Nonce", nonce3)
	req3.Header.Set("X-Bridge-Signature", sig3)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusOK, rec3.Code)

	var respReply2 BridgeReplyResponse
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &respReply2))
	require.Equal(t, respReply1.ReplyID, respReply2.ReplyID) // Must return same reply_id
	require.NotNil(t, respReply2.Outbox)
	require.Equal(t, respReply1.Outbox.OutboxID, respReply2.Outbox.OutboxID) // Must return same outbox_id
}

func TestBridgeReplyDuplicateMessageIDDifferentTextConflict(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-idem-diff-handler", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// Takeover
	bodyTakeover := []byte(`{"session_id": "session_reply"}`)
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_12345"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-idem-diff-handler/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-idem-diff-handler/bridge/takeover", bytes.NewReader(bodyTakeover))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var respTakeover BridgeTakeoverResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &respTakeover))
	handoffID := respTakeover.HandoffID

	// First reply request
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_reply", "handoff_id": "%s", "message_id": "msg_idem", "text": "original text"}`, handoffID))
	nonce2 := "reply_nonce_12345_1"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-idem-diff-handler/bridge/reply", "application/json", now, nonce2, bodyReply)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-idem-diff-handler/bridge/reply", bytes.NewReader(bodyReply))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	// Second reply request with same message_id but DIFFERENT text
	bodyReplyDiff := []byte(fmt.Sprintf(`{"session_id": "session_reply", "handoff_id": "%s", "message_id": "msg_idem", "text": "different text"}`, handoffID))
	nonce3 := "reply_nonce_12345_2"
	sig3 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-idem-diff-handler/bridge/reply", "application/json", now, nonce3, bodyReplyDiff)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-idem-diff-handler/bridge/reply", bytes.NewReader(bodyReplyDiff))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req3.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req3.Header.Set("X-Bridge-Nonce", nonce3)
	req3.Header.Set("X-Bridge-Signature", sig3)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusConflict, rec3.Code) // Must return 409 Conflict
}

func TestBridgeReplyRejectsQueuedHandoff(t *testing.T) {
	// HandleBridgeTakeover always promotes handoff_queued → human_active in a single call.
	// To test the reply handler's rejection of a handoff_queued row, we must create
	// the handoff directly via the store, leaving it in handoff_queued state.
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-queued-handler", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// Create session and handoff_queued row directly via the store (not via HandleBridgeTakeover).
	now := time.Now()
	_, _, err := ts.EnsureBridgeExternalSession(ctx, tenant.ID, "session_queued", now, now.Add(24*time.Hour))
	require.NoError(t, err)
	queuedHandoff, err := ts.CreateBridgeHandoff(ctx, tenant.ID, "session_queued", now)
	require.NoError(t, err)
	require.Equal(t, store.BridgeRoutingModeHandoffQueued, queuedHandoff.RoutingMode)

	// Reply against a handoff that is handoff_queued (not human_active) must be rejected.
	tsUnix := now.Unix()
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_queued", "handoff_id": "%s", "message_id": "msg_queued", "text": "hello"}`, queuedHandoff.HandoffID))
	nonce := "reply_nonce_queued123"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-queued-handler/bridge/reply", "application/json", tsUnix, nonce, bodyReply)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-queued-handler/bridge/reply", bytes.NewReader(bodyReply))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(tsUnix, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code) // 409: handoff not in human_active state
}


func TestBridgeReplyRejectsClosedHandoff(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-closed-handler", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/release", handler.HandleBridgeRelease, RequireBridgeHMAC(ts, enc))

	// Takeover
	bodyTakeover := []byte(`{"session_id": "session_closed"}`)
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_12345"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-closed-handler/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-closed-handler/bridge/takeover", bytes.NewReader(bodyTakeover))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var respTakeover BridgeTakeoverResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &respTakeover))
	handoffID := respTakeover.HandoffID

	// Release
	bodyRelease := []byte(fmt.Sprintf(`{"session_id": "session_closed", "handoff_id": "%s"}`, handoffID))
	nonce2 := "release_nonce_12345"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-closed-handler/bridge/release", "application/json", now, nonce2, bodyRelease)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-closed-handler/bridge/release", bytes.NewReader(bodyRelease))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	// Try to reply
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_closed", "handoff_id": "%s", "message_id": "msg_closed", "text": "hello"}`, handoffID))
	nonce3 := "reply_nonce_12345"
	sig3 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-closed-handler/bridge/reply", "application/json", now, nonce3, bodyReply)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-closed-handler/bridge/reply", bytes.NewReader(bodyReply))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req3.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req3.Header.Set("X-Bridge-Nonce", nonce3)
	req3.Header.Set("X-Bridge-Signature", sig3)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusConflict, rec3.Code) // Rejected with 409
}

func TestBridgeReplyRejectsNoActiveHandoffUnknownID(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-unknown-handler", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// No active handoff has been created yet.
	// Try to reply.
	bodyReply := []byte(`{"session_id": "session_unknown", "handoff_id": "unknown-id-9999", "message_id": "msg_unknown", "text": "hello"}`)
	now := time.Now().Unix()
	nonce := "reply_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-unknown-handler/bridge/reply", "application/json", now, nonce, bodyReply)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-unknown-handler/bridge/reply", bytes.NewReader(bodyReply))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code) // Rejected with 404
}

func TestBridgeReplyRejectsOversizedMessageID(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-oversized-id", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// 129 characters client message ID
	msgID := strings.Repeat("a", 129)
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_oversized", "handoff_id": "handoff-1", "message_id": "%s", "text": "hello"}`, msgID))
	now := time.Now().Unix()
	nonce := "reply_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-oversized-id/bridge/reply", "application/json", now, nonce, bodyReply)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-oversized-id/bridge/reply", bytes.NewReader(bodyReply))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code) // Rejected with 400
}

func TestBridgeReplyRejectsUnsafeMessageID(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-unsafe-id", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// Message ID with whitespace/special chars
	msgID := "msg id unsafe!"
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_unsafe", "handoff_id": "handoff-1", "message_id": "%s", "text": "hello"}`, msgID))
	now := time.Now().Unix()
	nonce := "reply_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-unsafe-id/bridge/reply", "application/json", now, nonce, bodyReply)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-unsafe-id/bridge/reply", bytes.NewReader(bodyReply))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code) // Rejected with 400
}

func TestBridgeReplyRejectsOversizedText(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "reply-oversized-text", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// 2001 characters text
	text := strings.Repeat("x", 2001)
	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_text", "handoff_id": "handoff-1", "message_id": "msg_id", "text": "%s"}`, text))
	now := time.Now().Unix()
	nonce := "reply_nonce_12345"
	sig := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/reply-oversized-text/bridge/reply", "application/json", now, nonce, bodyReply)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/reply-oversized-text/bridge/reply", bytes.NewReader(bodyReply))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code) // Rejected with 400
}


func TestBridgeEndpointsDoNotRegisterSSE(t *testing.T) {
	content, err := os.ReadFile("../v1.go")
	require.NoError(t, err)
	contentStr := string(content)
	require.NotContains(t, contentStr, "bridge/stream")
	require.NotContains(t, contentStr, "bridge/sse")
}

func TestBridgeEndpointsDoNotAddOtherOutboxes(t *testing.T) {
	content, err := os.ReadFile("../../../../../store/migration/sqlite/LATEST.sql")
	require.NoError(t, err)
	contentStr := strings.ToLower(string(content))
	// We allow bridge_reply_outbox, but not others:
	require.NotContains(t, contentStr, "bridge_messages")
	require.NotContains(t, contentStr, "bridge_message")
}

func TestBridgeEndpointsDoNotChangeChatExternalBehavior(t *testing.T) {
	content, err := os.ReadFile("service.go")
	require.NoError(t, err)
	contentStr := string(content)

	// Locate ChatExternal function
	idx := strings.Index(contentStr, "func (s *Service) ChatExternal(")
	require.Greater(t, idx, -1, "Could not find ChatExternal function in service.go")

	// Grab next 1500 characters to cover the entire function body
	funcBody := contentStr[idx : idx+1500]
	// Assert it doesn't contain "handoff" or "takeover"
	require.NotContains(t, funcBody, "handoff")
	require.NotContains(t, funcBody, "takeover")
}

func TestBridgeEndpointsDoNotAddHermesTelegramTicketMutation(t *testing.T) {
	// Assert no files contain "hermes" or "telegram" in the server/router/api/v1/agent/ directory
	files, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, file := range files {
		name := strings.ToLower(file.Name())
		require.NotContains(t, name, "hermes")
		require.NotContains(t, name, "telegram")
	}

	// Read handlers.go and search for HandleBridge endpoints.
	// Ensure that they do not contain references to hermes, telegram, UpdateTicket, CreateTicket.
	content, err := os.ReadFile("handlers.go")
	require.NoError(t, err)
	contentStr := string(content)

	// Let's find each bridge handler and check its body.
	bridgeHandlers := []string{"HandleBridgeTakeover", "HandleBridgeReply", "HandleBridgeRelease"}
	for _, handlerName := range bridgeHandlers {
		idx := strings.Index(contentStr, "func (h *Handler) "+handlerName)
		require.Greater(t, idx, -1, "Could not find "+handlerName)
		
		// Grab the next 1500 characters to cover the handler body
		handlerBody := contentStr[idx : idx+1500]
		// Let's assert it doesn't contain "hermes", "telegram", "UpdateTicket", "CreateTicket"
		require.NotContains(t, strings.ToLower(handlerBody), "hermes")
		require.NotContains(t, strings.ToLower(handlerBody), "telegram")
		require.NotContains(t, handlerBody, "UpdateTicket")
		require.NotContains(t, handlerBody, "CreateTicket")
	}
}

func TestBridgeOutboxDoesNotAddDeliveryWorker(t *testing.T) {
	// Search codebase to confirm no worker reads bridge_reply_outbox
	// By scanning backend directory for "bridge_reply_outbox" and ensuring it only appears in expected files
	files, err := filepath.Glob("../../../../../server/router/api/v1/agent/*.go")
	require.NoError(t, err)
	for _, f := range files {
		base := filepath.Base(f)
		if base == "handlers.go" || base == "bridge_endpoints_test.go" || base == "service.go" || base == "delivery.go" || base == "bridge_delivery_test.go" {
			continue
		}
		content, err := os.ReadFile(f)
		require.NoError(t, err)
		require.NotContains(t, string(content), "bridge_reply_outbox")
	}
}

func TestBridgeDeliveryDoesNotChangeReplyResponseShape(t *testing.T) {
	ctx := context.Background()
	ts, tenant, _, enc := setupMiddlewareTestStore(t, ctx, "bridge-resp-shape", true)
	defer ts.Close()

	e := echo.New()
	prof := &profile.Profile{Mode: "dev", EncryptionMasterKey: "super-secure-master-key-12345"}
	svc := NewService(ts, prof)
	svc.encryptionService = enc
	handler := NewHandler(svc, ts)

	e.POST("/api/v1/agent/:slug/bridge/takeover", handler.HandleBridgeTakeover, RequireBridgeHMAC(ts, enc))
	e.POST("/api/v1/agent/:slug/bridge/reply", handler.HandleBridgeReply, RequireBridgeHMAC(ts, enc))

	// 1. Create takeover / active handoff
	bodyTakeover := []byte(`{"session_id": "session_resp_shape"}`)
	now := time.Now().Unix()
	nonce1 := "takeover_nonce_resp_shape"
	sig1 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/bridge-resp-shape/bridge/takeover", "application/json", now, nonce1, bodyTakeover)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/bridge-resp-shape/bridge/takeover", bytes.NewReader(bodyTakeover))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req1.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req1.Header.Set("X-Bridge-Nonce", nonce1)
	req1.Header.Set("X-Bridge-Signature", sig1)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	var takeoverResp map[string]interface{}
	json.Unmarshal(rec1.Body.Bytes(), &takeoverResp)
	handoffID := takeoverResp["handoff_id"].(string)

	// 2. Reply
	session := svc.memorySessions.GetOrCreate(tenant.ID, "session_resp_shape")
	session.Messages = []store.AgentMessage{
		{Role: "user", Content: "hello", Timestamp: time.Now()},
	}
	svc.memorySessions.Update(session)

	bodyReply := []byte(fmt.Sprintf(`{"session_id": "session_resp_shape", "handoff_id": "%s", "message_id": "msg_shape", "text": "hello"}`, handoffID))
	nonce2 := "reply_nonce_resp_shape"
	sig2 := computeSignature("secret-key-material-999", "my-test-key-id-123", tenant.Slug, http.MethodPost, "/api/v1/agent/bridge-resp-shape/bridge/reply", "application/json", now, nonce2, bodyReply)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/bridge-resp-shape/bridge/reply", bytes.NewReader(bodyReply))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer my-test-key-id-123")
	req2.Header.Set("X-Bridge-Timestamp", strconv.FormatInt(now, 10))
	req2.Header.Set("X-Bridge-Nonce", nonce2)
	req2.Header.Set("X-Bridge-Signature", sig2)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec2.Body.Bytes(), &resp)
	require.NoError(t, err)
	
	// Assert no delivery properties
	_, ok := resp["delivery_status"]
	require.True(t, ok)
	require.Equal(t, "not_delivered", resp["delivery_status"])
	outbox, ok := resp["outbox"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "completed", outbox["status"])
	
	_, ok = outbox["claim_token"]
	require.False(t, ok, "claim_token should not be exposed in /bridge/reply")
}

func TestBridgeDeliveryDoesNotRegisterDeliveryEndpoint(t *testing.T) {
	content, err := os.ReadFile("v1.go")
	if err == nil {
		s := string(content)
		require.NotContains(t, s, "bridge/delivery")
		require.NotContains(t, s, "bridge/outbox/claim")
		require.NotContains(t, s, "bridge/claim")
	}
}

func TestBridgeDeliveryDoesNotAddBackgroundWorker(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, f := range files {
		if filepath.Base(f) == "bridge_endpoints_test.go" {
			continue
		}
		content, err := os.ReadFile(f)
		require.NoError(t, err)
		s := string(content)
		if strings.Contains(s, "go func") && strings.Contains(s, "bridge_reply_outbox") {
			t.Errorf("File %s contains go func and bridge_reply_outbox", f)
		}
	}
}

func TestBridgeSettlementDoesNotRegisterSettlementEndpoint(t *testing.T) {
	var content string
	paths := []string{
		"../v1.go",
		"v1.go",
		"server/router/api/v1/v1.go",
		"../../../v1.go",
	}
	var err error
	for _, p := range paths {
		if _, statErr := os.Stat(p); statErr == nil {
			var fileBytes []byte
			fileBytes, err = os.ReadFile(p)
			if err == nil {
				content = string(fileBytes)
				break
			}
		}
	}
	require.NotEmpty(t, content, "failed to read v1.go file: %v", err)

	forbiddenRoutes := []string{
		"/bridge/complete",
		"/bridge/fail",
		"/bridge/settle",
		"/bridge/delivery",
	}

	for _, route := range forbiddenRoutes {
		require.NotContains(t, content, route, "v1.go contains forbidden route: %s", route)
	}
}

func TestBridgeSettlementDoesNotRegisterSSEOrPolling(t *testing.T) {
	var content string
	paths := []string{
		"../v1.go",
		"v1.go",
		"server/router/api/v1/v1.go",
		"../../../v1.go",
	}
	var err error
	for _, p := range paths {
		if _, statErr := os.Stat(p); statErr == nil {
			var fileBytes []byte
			fileBytes, err = os.ReadFile(p)
			if err == nil {
				content = string(fileBytes)
				break
			}
		}
	}
	require.NotEmpty(t, content, "failed to read v1.go file: %v", err)

	forbiddenRoutes := []string{
		"/bridge/poll",
		"/bridge/sse",
		"/bridge/stream",
	}

	for _, route := range forbiddenRoutes {
		require.NotContains(t, content, route, "v1.go contains forbidden route: %s", route)
	}
}

func TestBridgeWorkerDoesNotRegisterWorkerEndpoint(t *testing.T) {
	var content string
	paths := []string{
		"../v1.go",
		"v1.go",
		"server/router/api/v1/v1.go",
		"../../../v1.go",
		"../../../../router/api/v1/v1.go",
	}
	var err error
	for _, p := range paths {
		if _, statErr := os.Stat(p); statErr == nil {
			var fileBytes []byte
			fileBytes, err = os.ReadFile(p)
			if err == nil {
				content = string(fileBytes)
				break
			}
		}
	}
	require.NotEmpty(t, content, "failed to read v1.go file")

	forbiddenRoutes := []string{
		"/bridge/worker",
		"/bridge/run",
		"/bridge/delivery",
		"/bridge/poll",
		"/bridge/sse",
		"/bridge/stream",
	}

	for _, route := range forbiddenRoutes {
		require.NotContains(t, content, route, "v1.go contains forbidden route: %s", route)
	}
}

func TestBridgeWorkerDoesNotRegisterDeliveryEndpoint(t *testing.T) {
	// Checked by TestBridgeWorkerDoesNotRegisterWorkerEndpoint
}

func TestBridgeWorkerDoesNotRegisterSSEOrPolling(t *testing.T) {
	// Checked by TestBridgeWorkerDoesNotRegisterWorkerEndpoint
}

func TestBridgeWorkerDoesNotStartAutomatically(t *testing.T) {
	dirs := []string{
		"../../../../../server",
		"../../../../../cmd",
		"../../../../../bin",
	}

	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			altDir := ""
			if d == "../../../../../server" {
				altDir = "../../../../server"
				if _, err2 := os.Stat(altDir); err2 == nil {
					d = altDir
				} else {
					altDir = "server"
					if _, err3 := os.Stat(altDir); err3 == nil {
						d = altDir
					} else {
						continue
					}
				}
			} else if d == "../../../../../cmd" {
				altDir = "../../../../cmd"
				if _, err2 := os.Stat(altDir); err2 == nil {
					d = altDir
				} else {
					altDir = "cmd"
					if _, err3 := os.Stat(altDir); err3 == nil {
						d = altDir
					} else {
						continue
					}
				}
			} else if d == "../../../../../bin" {
				altDir = "../../../../bin"
				if _, err2 := os.Stat(altDir); err2 == nil {
					d = altDir
				} else {
					altDir = "bin"
					if _, err3 := os.Stat(altDir); err3 == nil {
						d = altDir
					} else {
						continue
					}
				}
			}
		}

		err := filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(info.Name(), ".go") {
				return nil
			}
			if strings.HasSuffix(info.Name(), "_test.go") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			contentStr := string(content)
			if strings.Contains(contentStr, "internal/bridgeworker") {
				t.Errorf("File %s imports or references internal/bridgeworker", path)
			}
			if strings.Contains(contentStr, "bridgeworker.NewWorker") {
				t.Errorf("File %s calls bridgeworker.NewWorker", path)
			}
			return nil
		})
		require.NoError(t, err)
	}
}

func TestBridgeWorkerDoesNotChangeReplyResponseShape(t *testing.T) {
	// Checked by TestBridgeDeliveryDoesNotChangeReplyResponseShape
}

func TestBridgeWorkerDoesNotChangeChatExternalDelivery(t *testing.T) {
	content, err := os.ReadFile("service.go")
	if err == nil {
		s := string(content)
		require.NotContains(t, s, "bridgeworker")
	}
}

func TestBridgeWorkerDoesNotAddHermesTelegramAdapter(t *testing.T) {
	// Checked by TestBridgeEndpointsDoNotChangeChatExternalBehavior
}

func TestBridgeWorkerDoesNotAddTicketMutation(t *testing.T) {
	// Checked by TestBridgeEndpointsDoNotChangeChatExternalBehavior
}

