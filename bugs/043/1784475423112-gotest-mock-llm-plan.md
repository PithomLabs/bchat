# Plan: Fix `go test ./...` failures — pluggable mock LLM client

**Folder:** `bugs/043/`
**Source:** `bugs/043/gotest_fail.md` (15 failing tests in `server/router/api/v1/agent`)
**Decision (confirmed with user):** Pluggable mock client (production untouched when no mock configured).

---

## Problem statement (root cause)

All 15 failing tests fail with the same error:

```
failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
```

This comes from `requireLLMConfig` (`server/router/api/v1/agent/service.go:1659-1671`),
which hard-returns an error when `apiKey == ""` for **both** the tenant key and the
env fallback (`s.profile.OpenRouterAPIKey`). The failing tests deliberately do
`t.Setenv("OPENROUTER_API_KEY", "")` and provide no tenant key, so `apiKey` is empty.

**Confirmed pre-existing:** `git stash` + run `TestChatExternalClientMessageIDIsIdempotent`
fails identically on the unmodified baseline (`master`, commit `de1f1b0`). This is NOT
caused by the Code 7 hardening changes. The underlying defect is that **there is no
mock LLM client**, so any test that exercises real chat generation is unrunnable without
a live OpenRouter key — unlike the embedding tests, which use a fake `sk-or-v1-*` key
and an injected API key via `WithEmbeddingOpenRouterAPIKey`.

The tests assert behavior *around* generation (idempotency, lead/ticket creation,
handoff, content mismatch, materialization warnings), not specific LLM output — a canned
mock reply satisfies them (verified by reading `TestChatExternalClientMessageIDIsIdempotent`
and peers).

---

## Goal

Make the 15 tests pass without a real OpenRouter key by introducing a **pluggable LLM
client seam** on `Service`, with a test-only mock backed by `httptest` returning canned
chat completions. Production behavior is unchanged when no mock is configured.

---

## Design

### 1. Make `newOpenRouterClient` honor a configurable base URL (env)
**File:** `server/router/api/v1/agent/service.go` (`newOpenRouterClient`, ~line 55)

Today it hardcodes `openrouter.DefaultConfig(apiKey)` (BaseURL `https://openrouter.ai/api/v1`).
Add an env override:

```go
func newOpenRouterClient(apiKey string) *openrouter.Client {
    config := openrouter.DefaultConfig(apiKey)
    if base := os.Getenv("OPENROUTER_API_BASE_URL"); base != "" {
        config.BaseURL = base
    }
    config.HTTPClient = &http.Client{Timeout: defaultLLMTimeout}
    return openrouter.NewClientWithConfig(*config)
}
```

This is the only production-side change and is inert unless `OPENROUTER_API_BASE_URL`
is set (it never is in prod). The `openrouter.ClientConfig` already exposes `BaseURL`
(confirmed in `github.com/revrost/go-openrouter@v1.6.0/config.go:9`).

### 2. Test harness: in-memory mock LLM server
**New file:** `server/router/api/v1/agent/llm_mock_test.go`

Provide a helper that spins an `httptest.Server` mimicking the OpenRouter chat
completions endpoint and returns a canned valid response:

```go
func newMockLLMServer(t *testing.T, reply string) *httptest.Server {
    s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "id": "mock", "object": "chat.completion", "model": "mock",
            "choices": []map[string]any{{
                "index": 0,
                "message": map[string]any{
                    "role": "assistant",
                    "content": map[string]any{"text": reply},
                },
                "finish_reason": "stop",
            }},
        })
    }))
    t.Cleanup(s.Close)
    return s
}
```

Response shape verified against `openrouter` types:
`ChatCompletionResponse.Choices[].Message.Content` is `Content{ Text string }`
(`chat.go:566-569`, `:599`), so `content.text` decodes correctly.

### 3. Wire mock into the 15 tests
Each failing test currently calls `t.Setenv("OPENROUTER_API_KEY", "")` (directly or via
`newBridgeChatTestService`/`NewService` profiles). Update them to instead:

- Start `srv := newMockLLMServer(t, "This is a mock assistant reply.")`
- `t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-mock")` (any non-empty value; the real API is
  never contacted because BaseURL points at the mock)
- `t.Setenv("OPENROUTER_API_BASE_URL", srv.URL)`

This satisfies `requireLLMConfig` (`apiKey != ""`) and routes generation to the mock.

**Affected tests (call sites from `gotest_fail.md`):**
- `bridge_foundation_test.go`: `TestChatExternalClientMessageIDIsIdempotent` (193),
  `..._Concurrent` (230), `..._Restart` (254), `...PersistsToDatabase` (274),
  `TestChatExternalEscalationCreatesLeadAndTicketWithoutHandoff` (303),
  `...DedupesTicketAcrossServiceRestart` (343),
  `...WithIncompleteContactAsksForContactInfo` (365), `TestChatExternalClientMessageIDContentMismatch` (402),
  `TestMaterializationFailureLogsSanitizedWarningOnce` (485), `TestUnsupportedDBPathCreatesNoWarnings` (578)
- `bridge_runtime_test.go`: `TestChatExternalAfterReleaseResumesAIBehavior` (91),
  `TestChatExternalHumanActiveHandoffDoesNotAppendUserOrAIMessage` (105),
  `TestChatExternalUnsupportedBridgeDBDoesNotBreakNormalChat` (143)
- `bridge_delivery_test.go`: `TestBChatLiveReleaseAllowsAIResume` (238),
  `TestBChatLiveEndToEndVisitorHumanReplyFlow` (854)

Centralize the setup in a helper (e.g., `bridgeChatTestServiceWithMockLLM(t, slug)` or
extend `newBridgeChatTestService`) so each test sets the three env vars once. Confirm
none of these tests assert on specific LLM *text* — they check structural/idempotency/
side-effect behavior, so a fixed reply is acceptable. (Verify during implementation by
re-reading each test's assertions.)

### 4. No change to production auth semantics
- `requireLLMConfig` keeps returning an error when `apiKey == ""` for real prod calls.
- `getLLMConfig` (silent fallback) unchanged.
- Only the `OPENROUTER_API_BASE_URL` env (never set in prod) alters routing.

---

## Affected files
| File | Change |
|------|--------|
| `server/router/api/v1/agent/service.go` | `newOpenRouterClient` reads `OPENROUTER_API_BASE_URL` |
| `server/router/api/v1/agent/llm_mock_test.go` | NEW: mock server helper |
| `server/router/api/v1/agent/bridge_foundation_test.go` | wire mock into 10 tests |
| `server/router/api/v1/agent/bridge_runtime_test.go` | wire mock into 3 tests |
| `server/router/api/v1/agent/bridge_delivery_test.go` | wire mock into 2 tests |

No schema migration. No production code-path change beyond the inert `BaseURL` env hook.

---

## Validation
1. `go test ./server/router/api/v1/agent/ -run 'TestChatExternal|TestBChatLive|TestMaterializationFailure|TestUnsupportedDBPath' -count=1` → all PASS.
2. `go test ./...` → no package-level FAIL (the `server/router/api/v1/agent` package passes).
3. `go build ./...` and `go vet ./server/router/api/v1/...` → clean.
4. Regression guard: with `OPENROUTER_API_BASE_URL` unset, behavior identical to today
   (real OpenRouter). Confirm by NOT setting the env in any non-mock test.
5. Spot-check that the mock reply does not break assertions in
   `TestChatExternalClientMessageIDContentMismatch` (it checks mismatch detection, not
   reply text) and `TestMaterializationFailureLogsSanitizedWarningOnce`.

## Risks / notes
- If any test asserts on specific generated text, the canned reply may need per-test
  customization (pass `reply` per test). Re-read each test's assertions during impl.
- The mock server must be torn down (`t.Cleanup`) to avoid port leaks in parallel tests.
- Embedding tests already work via fake keys + `WithEmbeddingOpenRouterAPIKey`; they are
  unaffected.
- This does not make the chat path "free" in CI beyond tests — production still requires
  a real key.
