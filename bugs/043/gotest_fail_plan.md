# Fix `go test ./...` Failures in `server/router/api/v1/agent`

## Symptom

Running `go test ./...` fails 1 test in the `agent` package:

```
--- FAIL: TestBChatLiveEndToEndVisitorHumanReplyFlow (0.13s)
    bridge_delivery_test.go:930:
        	Error: Received unexpected error:
        	       code=403, message=Invalid or expired token
```

## Root Cause

The test constructs a transcript URL using `testSigningSeed` as the HMAC seed, but the handler verifies the token using the tenant's `transcript_signing_key` decrypted via `getTranscriptSigningSeed`. The test never called `setupTestSigningKey`, so the tenant's signing key was empty/NULL in the database. Additionally, the test did not set up the mock LLM, causing earlier failures that masked this issue.

## Fix

### File: `server/router/api/v1/agent/bridge_delivery_test.go`

1. Added `withMockLLM(t, "...")` and `t.Setenv("RAG_PIPELINE_ENABLED", "false")` to `TestBChatLiveEndToEndVisitorHumanReplyFlow` so the chat path exercises the mock LLM instead of requiring a live OpenRouter key.

2. Added `setupTestSigningKey(t, ctx, ts, tenant, svc)` after service construction so the tenant has a valid encrypted `transcript_signing_key` in the database.

3. Changed `testTranscriptURL("live-e2e-flow", sessionID, tenant.WidgetKey)` to `testTranscriptURL("live-e2e-flow", sessionID, testSigningSeed)` at line 924, so the token is generated with the correct seed that matches the tenant's stored signing key.

### File: `server/router/api/v1/agent/bridge_foundation_test.go`

Added `withMockLLM(t, "...")` to `TestUnsupportedDBPathCreatesNoWarnings` and removed the explicit `OPENROUTER_API_KEY=""` override.

## Verification

```bash
go test ./...
```

All tests pass. `go vet ./...` and `gofmt -l` remain clean.
