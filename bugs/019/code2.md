# Implementation Plan: Apply `plan4_prompt_mimo.md`

**Date:** 2026-07-04
**Prompt file:** `plan4_prompt_mimo.md`
**Scope:** Re-verify and complete the 6 fixes listed in `plan4_prompt_mimo.md` against the current working tree.

---

## Current State Assessment

The repository already has uncommitted changes that implement most of the fixes from `plan4_prompt_mimo.md`. The working tree does **not** currently compile due to a syntax error in `generateWidgetScript` (`server/router/api/v1/agent/handlers.go`).

| Fix | Prompt target | Current status | Action required? |
|-----|---------------|----------------|------------------|
| Fix 1 (P0-3) Revert token redaction | `user_service.go:417` | **Done.** Returns `AccessToken: userAccessToken.AccessToken`. | None |
| Fix 2 (P0-1) Goroutine context leak | `user_service.go:244-249` | **Done.** Goroutine uses `bgCtx := context.Background()`. | None |
| Fix 3 (P0-1) Wrong error variable | `user_service.go:250-252` | **Done.** Error message uses `retryErr`. | None |
| Fix 4 (P0-4) Missing domain log | `handlers.go:1923-1924` | **Done.** `slog.Error(...)` added; `return false`. | None |
| Fix 5 (P1-9) Log unparseable tokens | `user_service.go:553-554` | **Done.** `slog.Warn(...)` added. | None |
| Fix 6 (P1-10) Discarded error | `workspace_setting_service.go:107` | **Partially done.** Error captured and logged with `slog.Warn`. | Change log level to `slog.Error` (prompt-acceptable alternative). |

---

## Blockers Found

### Blocker 1 — Broken build in `generateWidgetScript` (`handlers.go:1692`)
The XSS fix introduced a syntax error that prevents compilation.

**Current broken code (lines 1688-1700 approx.):**
```go
func generateWidgetScript(baseURL, tenantSlug, companyName string) string {
	safeBaseURL, _ := json.Marshal(baseURL)
	safeSlug, _ := json.Marshal(tenantSlug)
	_, _ = json.Marshal(companyName) // kept for future use (displaying company name in widget UI)

	return fmt.Sprintf(`(function() {
  'use strict';

  // Configuration -- all values are json.Marshal-safe (no XSS via </script>)
  var config = {
    baseURL: %s,
    tenantSlug: %s,
    primaryColor: '#0d9488'
  };
```

**Issues:**
1. `fmt.Sprintf(` has no closing `)`.
2. `%s` verbs have no arguments.
3. `safeBaseURL` / `safeSlug` are unused.

**Fix:** Replace `fmt.Sprintf` with raw-string concatenation using the marshaled values.
```go
func generateWidgetScript(baseURL, tenantSlug, companyName string) string {
	safeBaseURL, _ := json.Marshal(baseURL)
	safeSlug, _ := json.Marshal(tenantSlug)
	_, _ = json.Marshal(companyName) // kept for future use

	return `(function() {
  'use strict';

  // Configuration -- all values are json.Marshal-safe (no XSS via </script>)
  var config = {
    baseURL: ` + string(safeBaseURL) + `,
    tenantSlug: ` + string(safeSlug) + `,
    primaryColor: '#0d9488'
  };
  // ... rest remains unchanged ...
})();`
}
```

This preserves the XSS mitigation, removes the syntax error, and keeps `companyName` unused (as the prompt's P0-6 finding notes it is dead code).

### Blocker 2 — Build failure from `scripts/migrate-old-tokens/main.go`
`scripts/migrate-old-tokens/main.go` imports `github.com/mattn/go-sqlite3`, which is **not** in `go.mod`. This breaks `go build ./...`.

**Recommended fix:** Remove the `scripts/migrate-old-tokens/` directory. It was a one-time migration artifact and is not referenced by the prompt. If the team wants to keep it, add the sqlite3 dependency instead (not recommended because it requires CGO).

---

## Step-by-step Fixes

### Step 1 — Fix `generateWidgetScript` syntax error
**File:** `server/router/api/v1/agent/handlers.go` (~line 1692)
**Change:** Replace the broken `fmt.Sprintf` with raw-string concatenation as shown in Blocker 1.
**Rationale:** The prompt mandates `json.Marshal`-safe values (P0-6). The broken code cannot compile. This fix stays true to the plan's intent while restoring buildability.

### Step 2 — Complete Fix 6 (P1-10) error log level
**File:** `server/router/api/v1/workspace_setting_service.go` (~line 110)
**Change:**
```go
// Change from:
slog.Warn("failed to extract workspace setting key from name, defaulting to GeneralSetting",
	"name", setting.Name, "error", err)

// To:
slog.Error("failed to extract workspace setting key from name, defaulting to GeneralSetting",
	"name", setting.Name, "error", err)
```
**Rationale:** The prompt accepts this as an alternative to full error propagation. It is minimal, low-risk, and aligns with the severity.

### Step 3 — Remove migration-script build blocker
**File:** `scripts/migrate-old-tokens/main.go`
**Change:** Delete the file and its directory.
**Rationale:** Unblocks `go build ./...`. The script is not part of the prompt's scope.

---

## Out-of-prompt Recommendation (Not Included in Plan)

`code_review.md` identifies **P1-8 — `requireLLMConfig` not implemented** as HIGH severity. The current `getLLMConfig` at `service.go:1198` silently falls back to the global `OpenRouterAPIKey` when a tenant's encrypted key fails to decrypt, breaking tenant LLM billing isolation.

If requested, the fix would be:
- Add a `requireLLMConfig(ctx, tenantID)` wrapper that returns `("", "", error)` when a tenant key exists but decryption fails.
- Apply it only to the two chat-critical endpoints (`generateResponse` at line 2151 and `generateRAGResponse` at line 2614).
- Keep soft fallback for the remaining 7 call sites (`simulation.go`, `embedding.go`, `GenerateAnnotatedKB`, `GenerateAnnotatedPolicy`, etc.).

This change is **not** in `plan4_prompt_mimo.md` and is therefore deferred unless explicitly requested.

---

## Verification

After applying the fixes, run:

```bash
# Build only packages under our control (avoid CGO sqlite3 if present)
go build ./server/router/api/v1/...

# Run tests
go test ./server/router/api/v1/... -count=1 -race
go test ./server/router/api/v1/agent/... -count=1 -race

# Verify P0-5 — no "usememos" references
grep -r '"usememos"' --include='*.go' .

# Verify P0-2 constant exists
grep -rn "MaxNeverExpireDuration" server/router/api/v1/auth.go

# Verify P0-1 uses target user ID
grep -rn "deleteAllUserAccessTokens" server/router/api/v1/
```

---

## Risks & Tradeoffs

1. **Fix 6 (slog.Error vs propagation):** Full error propagation would be more robust but touches function signatures and all callers. The prompt explicitly permits the `slog.Error` alternative, so it is the safer, lower-risk choice.
2. **P0-3 token redaction:** The codebase already returns raw tokens. If the frontend is not yet deployed with P0-3 Phase 2 (ID-based deletion), the token management page remains at risk of exposing raw JWTs. This should be tracked as a follow-up.
3. **Migration script:** If the sqlite3 migration script is needed later, it should be rebuilt as a standalone tool with an explicit `go.mod`.
