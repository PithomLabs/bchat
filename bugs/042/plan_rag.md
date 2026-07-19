# Bug 042 — RAG Agent Chat Service Unavailable

## Symptom

Running `task run:rag` and testing the internal agent under agent admin returns:
"I apologize, but the chat service is not currently available. Please call us directly."

## Root Cause

The error originates from `service.go:2541` and `service.go:3063` — both in `requireLLMConfig`, which fails when no OpenRouter API key is available for the tenant.

### Code path

1. Agent chat request → `generateResponse()` or `generateRAGResponse()`
2. Both call `requireLLMConfig(ctx, tenantID)` (lines 2539, 3061)
3. `requireLLMConfig` → `getLLMConfig` → checks tenant config first, then falls back to `s.profile.OpenRouterAPIKey`
4. `s.profile.OpenRouterAPIKey` comes from `viper.GetString("openrouter-api-key")` → mapped to env var `OPENROUTER_API_KEY` via `viper.BindEnv` (main.go:172)
5. If key is empty → returns error → fallback message returned to user

### Why it fails

The `.env` file has the key set:
```
OPENROUTER_API_KEY=redacted
```

The `task run:rag` task sources `.env` via `set -a && source .env && set +a` before starting the server. So the env var should be available.

### Likely cause: stale server process

My earlier test started a server on `:8081` which may still be running. When `task run:rag` tried to bind the same port, it either:
- Failed silently and the user hit the old instance (without the env var sourced)
- Or the old instance is still serving requests

## Investigation steps

1. Check if any orphaned bchat/memos process is on port 8081:
   ```
   lsof -i :8081
   ```

2. Kill any stale processes:
   ```
   kill <pid>
   ```

3. Verify `.env` sources correctly:
   ```
   source .env && echo $OPENROUTER_API_KEY
   ```

4. Check the server startup log from `task run:rag` — look for:
   - `Version 0.31.0 has been started on port 8081` (confirm it started)
   - Any warnings about missing API keys
   - `ENCRYPTION_MASTER_KEY is not set` (expected, not a blocker)

5. Test the API directly:
   ```
   curl -X POST http://localhost:8081/api/v1/agent/evpn/chat \
     -H "Content-Type: application/json" \
     -d '{"message": "hello"}'
   ```

## Notes

- **Never include real API keys in markdown files** — use placeholders only
- `ENCRYPTION_MASTER_KEY` not being set is expected — it only affects tenant-specific encrypted API keys, not the global env var fallback
- This is unrelated to the embed.js 404 fix (Bug 042 primary issue)
- The `requireLLMConfig` function is stricter than `getLLMConfig` — it returns errors instead of silently absorbing them, which is correct for chat-critical paths

## Adversarial Review

Before acting on this plan, challenge these assumptions:

1. **Is the stale process theory correct?** Could there be a different reason the API key is empty — e.g., viper env binding order, flag override, or a different server binary?
2. **Could the tenant config have a corrupted encrypted key?** If a tenant config row exists with encrypted data but the encryption service is nil, does `requireLLMConfig` skip decryption safely, or could it error?
3. **Is the fallback message appropriate?** Should the backend return a more specific error (e.g., "API key not configured") to aid debugging, or is the vague message intentional for end users?
4. **Are there other code paths that hit the same error?** Check if `generateRAGResponse` (line 3063) vs `generateResponse` (line 2541) have different pre-conditions — could one work while the other fails?
5. **Is the `task run:rag` env sourcing reliable?** Could `set -a && source .env && set +a` fail silently in certain shell environments (e.g., dash vs bash)?
6. **Should this be a code fix or a config fix?** If the env var is correctly set but the process doesn't see it, is that a bchat bug (should re-read config) or a deployment issue?
