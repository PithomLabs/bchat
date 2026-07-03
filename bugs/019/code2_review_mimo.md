# Review of `code2.md`

**Reviewer:** mimo
**Date:** 2026-07-04
**Verdict:** Approved with nits. Fix the 2 blockers before go-signal.

---

## Blocker 1: Confirmed — Build Broken by `generateWidgetScript`

**Verified:** `go build ./server/router/api/v1/agent/...` fails:

```
handlers.go:1899:7: syntax error: unexpected newline in argument list; possibly missing comma or )
```

The `fmt.Sprintf` at line 1699 opens a backtick string but never closes with `, safeBaseURL, safeSlug)`. The string ends at line 1899 with `` })();` `` and the function returns — no arguments are passed to `%s`.

code2.md's fix (raw-string concatenation) is correct and minimal. The XSS mitigation is preserved since `json.Marshal` output is concatenated directly.

---

## Blocker 2: Confirmed — Migration Script Breaks Build

**Verified:** `go build ./...` fails:

```
scripts/migrate-old-tokens/main.go:25:2: no required module provides package github.com/mattn/go-sqlite3
```

code2.md's recommendation to delete the script is correct. It's a one-time tool not in the prompt's scope and requires CGO. If needed later, it should have its own `go.mod`.

---

## Fix 6: Minor Nit — `slog.Warn` vs `slog.Error`

code2.md acknowledges this at line 20 and proposes changing `slog.Warn` to `slog.Error` at step 2. This is the correct fix per plan4_prompt_mimo.md. Low risk, minimal change.

---

## What's Solid

- All 6 fixes from plan4_prompt_mimo.md are present in the working tree.
- Blockers are correctly identified with root causes.
- The proposed fixes for both blockers are correct and minimal.
- Verification commands are comprehensive.
- P1-8 (`requireLLMConfig`) is correctly deferred as out-of-scope.
- The risks section accurately notes the P0-3 follow-up needed.

---

## Summary

| Item | Status | Action |
|------|--------|--------|
| Blocker 1: `generateWidgetScript` syntax error | Confirmed | Apply code2.md's raw-string fix |
| Blocker 2: migration script missing dep | Confirmed | Delete `scripts/migrate-old-tokens/` |
| Fix 6: `slog.Warn` → `slog.Error` | Minor nit | Apply per code2.md step 2 |
| Fixes 1-5 | All done | No action |

**Go-signal:** Yes, after applying the 3 items above. The build must pass `go build ./server/router/api/v1/...` and `go test ./server/router/api/v1/... -count=1 -race` before merge.
