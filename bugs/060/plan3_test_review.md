# Adversarial Test-Plan Review — Bug 060 v3

**Reviewer:** Senior Go Architect & CockroachDB Expert
**Date:** 2026-08-06
**Verdict:** GO

---

## Summary

v3 eliminates v2's environmental incidents (cloud DSN, wrong port) by construction. The plan is hermetic, reproducible, and ready for execution.

## Findings

**0 new issues.** Verified against source:

| Claim | Evidence | Correct? |
|-------|----------|----------|
| First signup → HOST | `auth_service.go:610-618` — checks `len(existedHostUsers)==0` → `RoleHost` | ✅ |
| Signup endpoint exists | `v1.go:216` — `POST /api/v1/auth/signup` | ✅ |
| `IsDev()` covers `dev`+`demo` | `profile.go:43-45` — `p.Mode != "prod"` | ✅ |
| Reindex status `"completed"` | `service.go:873` — `Status` field with value `"completed"` | ✅ |
| Fresh DB → 0 tenants | SQL assertion in §5.1 gate | ✅ |
| 293 `kb_section` count | Observed in v2 execution; `chunker.go:394` stores `fileType+"_section"` | ✅ |
| Guard goes after DSN resolution | Plan says "after lines 97-109" — correct, DSN is set by then | ✅ |
| `.env.local` gitignored | `.gitignore` lines 92-99 | ✅ |

## Minor nits (non-blocking)

1. **`isLoopbackDSN` IPv6 handling** — plan mentions `::1` but implementation must strip port from DSN hostname before comparison. Use `net/url.Parse` + `strings.TrimSuffix(hostname, ":port")`.

2. **`run:cockroach:local` sets env inline** — the Taskfile task at §4 duplicates env vars from `.env.local`. If `.env.local` already sets `MEMOS_PORT=5230`, the inline `MEMOS_PORT=5230` is redundant but harmless (last wins).

3. **Step 5 `tenantId:1`** — on a fresh DB, the first tenant gets `id=1`. This is correct for `bchat_test` but would fail if the DB isn't fresh. The boot-evidence gate (0 tenants) makes this safe.

## Verdict

**GO** — Execute as written. The v3 plan is hermetic, the guard is correct, and the test path is verified.
