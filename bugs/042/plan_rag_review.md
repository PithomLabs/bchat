# Plan Review: Bug 042 — RAG Agent Chat Service Unavailable

## Overview

The plan documents that `task run:rag` returns `"I apologize, but the chat service is not currently available. Please call us directly."` to the user. The error originates from `requireLLMConfig` in `service.go:1447`, which fails because `s.profile.OpenRouterAPIKey` is empty.

The plan attributes this to a **stale server process** on port `:8081`. Below I assess this theory and surface other root causes the plan misses.

---

## Root Cause Analysis — Needs Rework

### Claim: "Likely cause: stale server process"

**Partially correct but incomplete.** The plan says an old instance without the env var is still running on `:8081`, so when the user tests, they hit the old instance.

**Counterargument:** If `:8081` were already bound, the *new* `task run:rag` process would fail to bind with `EADDRINUSE` (address already in use). The user would see an error on stderr, and the old process would still serve requests — but without the env var sourced. This is plausible **only if** the user ignores the bind error and continues testing against the old instance.

A more likely scenario: `task run:rag` **succeeds** (binds `:8081`), but the env var was **never loaded** into the process. This points to a Taskfile shell compatibility issue.

### Stronger candidate: `source` is a bashism — dash incompatibility

All Taskfiles (`Taskfile.yml:96`, `Taskfile1.yml:96`, `Taskfile_pg.yml:98`) use:

```sh
set -a && source .env && set +a
```

On **Ubuntu/Debian**, `/bin/sh` is **dash**, which does NOT have a `source` builtin. Only `.` (dot) is available. When `task` (or any Go-based task runner) invokes shell commands, it typically uses `sh -c`, which maps to `/bin/sh` → **dash** on these systems.

**What happens:**
1. `set -a` succeeds (allexport on)
2. `source .env` **fails silently** — dash errors to stderr but the `&&` chain stops
3. `set +a` never runs
4. The server binary starts on the next line — but **no env vars were exported**

**Result:** `OPENROUTER_API_KEY` is empty → `requireLLMConfig` returns "no API key configured" → polite apology returned.

**Verification:** Add `which sh && sh --version` before the source line in the Taskfile to check the shell. Or replace `source .env` with `. .env` (POSIX-compatible).

This is a **config/cross-platform bug**, not a stale-process issue. The fix belongs in the Taskfile(s).

---

## Code-Level Issues

### 1. Error is swallowed — no server-side diagnostics

At `service.go:2541` and `service.go:3063`:

```go
model, apiKey, err := s.requireLLMConfig(ctx, config.TenantID)
if err != nil {
    return "I apologize, but the chat service is not currently available. Please call us directly.", nil  // error discarded!
}
```

The `err` from `requireLLMConfig` is **discarded** and `nil` is returned to the caller. There is no `slog.Warn` or `slog.Error` call to record *why* the API key is missing. The server log gives zero indication of the failure cause.

**Fix:** Add a `slog.Warn` (or `slog.Error`) call before returning the apology:

```go
if err != nil {
    slog.Warn("chat: failed to get LLM config", "tenant", config.TenantID, "error", err)
    return "I apologize, but the chat service is not currently available. Please call us directly.", nil
}
```

### 2. `requireLLMConfig` guards against nil encryption service — no panic risk ✅

At `service.go:1449`:

```go
if config != nil && len(config.OpenRouterAPIKeyEncrypted) > 0 && s.encryptionService != nil {
```

The nil check on `s.encryptionService` prevents a panic when `ENCRYPTION_MASTER_KEY` is not set. If decryption fails, `requireLLMConfig` returns the error (tighter than `getLLMConfig` which silently falls back). This is correct — no code fix needed here.

---

## Adversarial Questions Answered

| # | Question | Answer |
|---|----------|--------|
| 1 | Is stale process theory correct? | **Partially.** Possible, but `source`/dash incompatibility is a stronger candidate. |
| 2 | Could tenant config have a corrupted encrypted key? | If decryption fails, `requireLLMConfig` returns an error explicitly. It would not silently fall back to the empty global key. Low probability unless someone uploaded a bad key. |
| 3 | Should the backend return a more specific error? | **Yes.** The apology message is appropriate for end users, but the server log should record the specific reason. At minimum, add `slog.Warn`. |
| 4 | Do `generateResponse` and `generateRAGResponse` differ in pre-conditions? | Both call `requireLLMConfig` identically. They will fail the same way. However, `generateRAGResponse` → line 3065 also calls `WithEmbeddingOpenRouterAPIKey(ctx, apiKey)` with the same (empty) key — `embedding.go:150` would use `os.Getenv("OPENROUTER_API_KEY")` as a last resort. |
| 5 | Is `task run:rag` env sourcing reliable? | **Not on dash.** `source .env` fails silently. Replace with `. .env`. |
| 6 | Code fix or config fix? | **Config fix** — the Taskfile needs `. .env`. **Code fix** (optional) — add `slog.Warn` on `requireLLMConfig` failure to aid future debugging. |

---

## Recommended Actions

| Priority | Fix | Location | Type |
|----------|-----|----------|------|
| **High** | Replace `source .env` with `. .env` in all Taskfiles | `Taskfile.yml:96`, `Taskfile1.yml:96`, `Taskfile_pg.yml:98`, etc. | Config |
| **Medium** | Add `slog.Warn` before apology in `generateResponse` and `generateRAGResponse` | `service.go:2541`, `service.go:3063` | Code |
| **Low** | Verify stale process: `lsof -i :8081` before starting | N/A | Debugging |

---

## Decision

| Item | Verdict |
|------|---------|
| Root cause (stale process) | **Partially correct** — but missing the dash incompatibility issue |
| Investigation steps | **Adequate** — add a shell version check (`sh --version`) |
| Code fixes | **Missing** — add server-side logging on `requireLLMConfig` failure |
| Config fixes | **Missing** — replace `source` with `.` in all Taskfiles |

**Overall: REWORK** — address the dash/source issue as the primary suspect, add `slog.Warn` on config failure, and downgrade the stale-process theory to secondary.
