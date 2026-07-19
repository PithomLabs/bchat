# Plan3: Underlying Root Cause — Chat Service Unavailable

## Root Cause (confirmed)

The process on `:8081` (PID 44353) is a stale test process from the embed.js debugging — started with `setsid` **without** sourcing `.env`. Zero `OPENROUTER` env vars in the process. The user's `task run:rag` likely failed silently with `EADDRINUSE`.

## Code path that produces the error

1. User sends message on `/internal-agent` page
2. `HandleChatInternal` → `ChatInternal` → `processChat` (service.go:2098)
3. `processChat` calls `generateRAGResponse` (RAG pipeline is enabled)
4. `generateRAGResponse` calls `requireLLMConfig` (service.go:3061)
5. `requireLLMConfig` → `getLLMConfig` → `s.profile.OpenRouterAPIKey` is empty
6. Returns error `"no OpenRouter API key configured for tenant X"`
7. `generateRAGResponse` catches the error, returns apology message with `nil` error
8. `processChat` treats this as a successful response (no error!), passes through sanitization
9. Apology returned to user as if it were a real LLM response

**Key insight**: The "chat service not available" message is NOT an HTTP error. It is a successful response that looks like a real message. The `nil` error means `processChat` never retries with `generateResponse`.

## Fixes

| # | Fix | Status | Type |
|---|-----|--------|------|
| 1 | Kill stale process (`kill 44353`) | Needed now | Operational |
| 2 | `source .env` to `. .env` in all Taskfiles | Already done | Config |
| 3 | Add `slog.Warn` on config failure | Already done | Code |
| 4 | Return error instead of apology with `nil` error in `generateRAGResponse` / `generateResponse` | Should implement | Code |
| 5 | Log whether `OPENROUTER_API_KEY` is set on startup in `main.go` | Should implement | Code |

### Fix 4 detail (error propagation)

At `service.go:2541` and `service.go:3064`, change:

```go
return "I apologize...", nil
```

to:

```go
return "", fmt.Errorf("chat service unavailable: %w", err)
```

This way the HTTP layer returns a proper 500 error instead of a fake success response.

### Fix 5 detail (startup logging)

In `main.go`, after building `instanceProfile`:

```go
if instanceProfile.OpenRouterAPIKey != "" {
    slog.Info("OpenRouter API key loaded", "prefix", instanceProfile.OpenRouterAPIKey[:10]+"...")
} else {
    slog.Warn("OpenRouter API key is NOT set - chat will be unavailable")
}
```

## Verification

1. `kill 44353` - stale process gone
2. `task run:rag` - server starts, logs show `OpenRouter API key loaded`
3. Test internal agent - chat works
