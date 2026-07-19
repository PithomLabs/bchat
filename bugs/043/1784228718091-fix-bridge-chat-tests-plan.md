# Fix `go test ./...` Failures in `server/router/api/v1/agent`

## Symptom

Running `go test ./...` fails 14 tests in the `agent` package, all with the same root error:

```
failed to generate response: chat service unavailable for tenant 1: no OpenRouter API key configured for tenant 1
```

Failing tests:
- `TestBChatLiveReleaseAllowsAIResume`
- `TestBChatLiveEndToEndVisitorHumanReplyFlow`
- `TestChatExternalClientMessageIDIsIdempotent`
- `TestChatExternalClientMessageIDIsIdempotent_Concurrent`
- `TestChatExternalClientMessageIDIsIdempotent_Restart`
- `TestChatExternalClientMessageIDPersistsToDatabase`
- `TestChatExternalEscalationCreatesLeadAndTicketWithoutHandoff`
- `TestChatExternalEscalationDedupesTicketAcrossServiceRestart`
- `TestChatExternalEscalationWithIncompleteContactAsksForContactInfo`
- `TestChatExternalClientMessageIDContentMismatch`
- `TestMaterializationFailureLogsSanitizedWarningOnce`
- `TestUnsupportedDBPathCreatesNoWarnings`
- `TestChatExternalAfterReleaseResumesAIBehavior`
- `TestChatExternalHumanActiveHandoffDoesNotAppendUserOrAIMessage`
- `TestChatExternalUnsupportedBridgeDBDoesNotBreakNormalChat`

## Root Cause

`getLLMConfig` in `server/router/api/v1/agent/service.go:1626-1659` claims in its doc comment to "fallback to environment variables", but the code only falls back to `s.profile.OpenRouterAPIKey` — a struct field populated at `Service` construction time. It does **not** read `os.Getenv("OPENROUTER_API_KEY")`.

In production, `bin/memos/main.go` populates `profile.OpenRouterAPIKey` from viper (which reads env vars / CLI flags), so the omission is invisible.

In tests, `newBridgeChatTestService` constructs `Service` with a bare `&profile.Profile{...}` where `OpenRouterAPIKey` is empty. The test helper `withMockLLM` sets `OPENROUTER_API_KEY` via `t.Setenv`, expecting `getLLMConfig` to pick it up. Because the env var is never read, `requireLLMConfig` returns the error and every bridge/chat test that reaches `generateResponse` fails.

`newOpenRouterClient` already reads `OPENROUTER_API_BASE_URL` from env at call time (line 60), so the mock-server routing works; only the API key path is broken.

## Fix

### File: `server/router/api/v1/agent/service.go`

In `getLLMConfig`, after falling back to `s.profile.OpenRouterAPIKey`, add a final fallback to `os.Getenv("OPENROUTER_API_KEY")`:

```go
// 2. Fallback to environment variables
if model == "" {
    model = s.profile.LLMModel
    if model == "" {
        model = os.Getenv("LLM_MODEL")
        if model == "" {
            model = "openrouter/free"
        }
    }
}
if apiKey == "" {
    apiKey = s.profile.OpenRouterAPIKey
    if apiKey == "" {
        apiKey = os.Getenv("OPENROUTER_API_KEY")
    }
}
```

This makes the function behavior match its documented contract ("fallback to environment variables") and unblocks the test helper `withMockLLM`.

## Verification

```bash
go test ./server/router/api/v1/agent/...
```

All 14 previously-failing tests should pass. `go vet ./...` and `gofmt -l` should remain clean.
