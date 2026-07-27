package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

// newObserverTestService creates a lean Service without a mock LLM.
// Used for Tier 2 tests (BENCHMARK_REAL_LLM=true) that need a real LLM.
func newObserverTestService(t *testing.T, slug string) (context.Context, *store.Store, *Service, *store.AgentTenant) {
	t.Helper()
	t.Setenv("RAG_PIPELINE_ENABLED", "false")
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)
	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug:        slug,
		CompanyName: slug,
		Vertical:    "test",
		IsActive:    true,
	})
	require.NoError(t, err)
	_, err = ts.CreateAgentAudience(ctx, &store.AgentAudience{
		TenantID:      tenant.ID,
		AudienceType:  "external",
		Role:          "assistant",
		Tone:          "helpful",
		EmergencyPhone: "",
		RateLimitRPM:  60,
	})
	require.NoError(t, err)
	service := NewService(ts, &profile.Profile{
		Driver: "sqlite",
		Mode:   "prod",
	})
	return ctx, ts, service, tenant
}

// newObserverTestServiceWithMock creates a Service with a mock LLM server.
// Used for Tier 1 tests (default, no API key needed).
func newObserverTestServiceWithMock(t *testing.T, slug, reply string) (context.Context, *store.Store, *Service, *store.AgentTenant) {
	t.Helper()
	withMockLLM(t, reply)
	return newObserverTestService(t, slug)
}

// newCountingMockLLM sets up a mock LLM that returns different replies for
// successive calls. Used for reflector tests where the observer and reflector
// LLM calls need different responses.
func newCountingMockLLM(t *testing.T, replies ...string) {
	t.Helper()
	var callCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(callCount.Load())
		if idx >= len(replies) {
			idx = len(replies) - 1
		}
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "mock",
			"object":  "chat.completion",
			"model":   "mock",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": replies[idx],
				},
				"finish_reason": "stop",
			}},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-mock")
	t.Setenv("OPENROUTER_API_BASE_URL", srv.URL)
}

// createTestSession creates a session in both the database and in-memory store,
// then populates it with the given messages. This is required because RunObserver
// persists observation logs to the DB with a foreign key on session_id, so the
// session must exist in the database.
func createTestSession(t *testing.T, ctx context.Context, ts *store.Store, service *Service, tenantID int32, sessionID string, messages []store.AgentMessage) string {
	t.Helper()
	_, err := ts.CreateAgentSession(ctx, &store.AgentSession{
		ID:           sessionID,
		TenantID:     tenantID,
		AudienceType: "external",
		Phase:        "triage",
		Messages:     messages,
		MessageCount: len(messages),
	})
	require.NoError(t, err, "failed to persist session to DB")

	memSession := service.memorySessions.GetOrCreate(tenantID, sessionID)
	memSession.Messages = messages
	memSession.MessageCount = len(messages)
	service.memorySessions.Update(memSession)

	return sessionID
}

// mustMakeMessages creates AgentMessage slices from role/content pairs.
// Convenience helper for test setup.
func mustMakeMessages(pairs ...string) []store.AgentMessage {
	if len(pairs)%2 != 0 {
		panic("mustMakeMessages requires role/content pairs")
	}
	msgs := make([]store.AgentMessage, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		msgs = append(msgs, store.AgentMessage{
			Role:      pairs[i],
			Content:   pairs[i+1],
			Timestamp: time.Now(),
		})
	}
	return msgs
}

// convertTurnsToMessages converts HaystackSession turn maps to AgentMessages.
// Used for loading LongMemEval embedded questions into test sessions.
func convertTurnsToMessages(turns []map[string]string) []store.AgentMessage {
	msgs := make([]store.AgentMessage, 0, len(turns))
	baseTime := time.Now()
	for i, turn := range turns {
		msgs = append(msgs, store.AgentMessage{
			Role:      turn["role"],
			Content:   turn["content"],
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		})
	}
	return msgs
}

// resetObservationLog clears the observation log for a session so RunObserver
// re-processes all messages on the next call. Used for two-pass testing:
// pass 1 (default threshold) gets raw observations, pass 2 (low threshold)
// triggers the reflector.
func resetObservationLog(t *testing.T, ctx context.Context, ts *store.Store, sessionID string) {
	t.Helper()
	obsLog, err := ts.GetObservationLog(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, obsLog, "observation log must exist before reset")
	obsLog.ObservationLog = ""
	obsLog.LastObservedMsgIndex = -1
	obsLog.TokensInLog = 0
	_, err = ts.UpsertObservationLog(ctx, obsLog)
	require.NoError(t, err, "failed to reset observation log")
}

// reloadOMConfigForTest reloads OM config and registers cleanup to reload again.
// Use after setting env vars with setOMEnvAndReload, NOT with t.Setenv (which
// has cleanup ordering issues with the OM singleton).
func reloadOMConfigForTest(t *testing.T) {
	t.Helper()
	ReloadOMConfig()
	t.Cleanup(func() {
		ReloadOMConfig()
	})
}

// setOMEnvAndReload sets an OM environment variable, reloads the config, and
// registers cleanup that restores the original value and reloads again.
// Unlike t.Setenv + reloadOMConfigForTest, this handles cleanup ordering correctly
// because the OM singleton must be reloaded AFTER the env var is restored.
func setOMEnvAndReload(t *testing.T, key, value string) {
	t.Helper()
	orig := os.Getenv(key)
	os.Setenv(key, value)
	ReloadOMConfig()
	t.Cleanup(func() {
		if orig == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, orig)
		}
		ReloadOMConfig()
	})
}

// --- Canned mock observer outputs ---

var mockObservations = map[string]string{
	"specific_name": `<observations>
Date: Jan 1, 2025
* 🔴 (14:30) User stated they created playlist "Summer Vibes" on Spotify
* 🟡 (14:31) Assistant confirmed playlist has chill tracks
</observations>
<current-task>None</current-task>
<suggested-response>Acknowledge the playlist.</suggested-response>`,

	"implicit_context": `<observations>
Date: Feb 10, 2025
* 🔴 (14:30) User discussed chess game moves: 27. Kg2 Bd5+ followed by 28. f3
* 🟡 (14:31) Assistant analyzed the position
</observations>
<current-task>None</current-task>
<suggested-response>Reference the game.</suggested-response>`,

	"temporal_anchor": `<observations>
Date: Mar 5, 2025
* 🔴 (14:30) User visited museum with friend Alex (meaning Jan 2025)
* 🟡 (14:31) User enjoyed the modern art exhibit
</observations>
<current-task>None</current-task>
<suggested-response>Reference the museum visit.</suggested-response>`,

	"knowledge_update": `<observations>
Date: Apr 1, 2025
* 🔴 (14:30) User stated they have 3 bikes (original count)
* 🔴 (14:35) User stated they got a new hybrid bike (now has 4 bikes total)
</observations>
<current-task>None</current-task>
<suggested-response>Acknowledge the new bike.</suggested-response>`,

	"cross_session": `<observations>
Date: May 1, 2025
* 🔴 (10:00) User owns a Korg B1 keyboard
* 🟡 (10:01) User mentioned playing piano

Date: May 3, 2025
* 🔴 (11:00) User owns a Yamaha acoustic guitar
* 🟡 (11:01) User enjoys playing both instruments
</observations>
<current-task>None</current-task>
<suggested-response>Ask about music.</suggested-response>`,

	"abstention": `<observations>
Date: Jun 1, 2025
* 🔴 (14:30) User discussed aquarium setup
* 🟡 (14:31) User has a 10-gallon freshwater tank
</observations>
<current-task>None</current-task>
<suggested-response>Ask about the aquarium.</suggested-response>`,

	"trivial_response": `<observations></observations>`,

	"empty_response": "",

	"malformed_no_closing": `<observations>
* 🔴 (14:30) User stated something important`,

	"malformed_content_outside": `Here are your observations:

<observations>
* 🔴 (14:30) User discussed project
</observations>

By the way, I also noticed some other things.`,

	"malformed_multiple_blocks": `<observations>first block</observations> <observations>second block</observations>`,

	"malformed_extra_whitespace": `<observations>
  
  * 🔴 (14:30) detail here  
  
</observations>`,

	"edge_long_content": `<observations>
Date: Jul 1, 2025
* 🟡 (14:30) User provided extensive code review with 5000+ characters covering authentication, database migrations, and API endpoint design
</observations>
<current-task>Review code</current-task>
<suggested-response>Acknowledge the review.</suggested-response>`,

	"edge_unicode": `<observations>
Date: Aug 1, 2025
* 🔴 (14:30) User discussed 日本語テスト and émojis 🎉
</observations>
<current-task>None</current-task>
<suggested-response>Continue in the same language.</suggested-response>`,

	// Reflector compressed versions — shorter but preserving key details.
	"reflector_compressed": `<observations>
* 🔴 User created playlist "Summer Vibes" on Spotify (chill tracks)
</observations>`,

	"reflector_compressed_multi": `<observations>
* 🔴 User owns Korg B1 keyboard and Yamaha acoustic guitar
</observations>`,
}
