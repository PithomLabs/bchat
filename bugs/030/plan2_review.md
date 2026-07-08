# Adversarial Security Review — `bugs/030/plan2.md`

**Reviewer:** Claude (Fable 5)
**Date:** 2026-07-08
**Subject:** Revised Security Remediation Plan v2 (Phases 0–2)
**Goal of this review:** Confirm the plan is a *minimum-viable, implementable* spec — closing the Critical/High issues with no remaining planning rounds. Nits below are the coding agent's implementation checklist, not a request to re-plan.

## Verdict: ✅ APPROVE WITH NITS

v2 correctly resolves all three blocking findings (B1–B3) and the six nits (N1–N6) from the prior review. The critical cross-tenant IDOR is closed for **both** `RoleUser` and scoped admins (H1 is the load-bearing control, now explicitly stated and correctly implemented; C1 adds defense-in-depth via the new `isSuperAdmin`). Ship it — with the must-fix items below applied *during* implementation.

| Phase | Verdict |
|-------|---------|
| 0 — Emergency (H7, C2, C3) | ✅ Approve |
| 1 — Tenant isolation (H1, H2, C1) | ✅ Approve w/ must-fix M2/M3 |
| 2 — Hardening (H3, H4, H5, H6) | ✅ Approve w/ must-fix M1/M4 |

---

## 🔧 Must-fix during implementation (blocking correctness, not blocking the plan)

These are code-level defects/omissions in the plan's snippets. The coding agent must handle them inline — they don't need another review cycle, but if ignored the fix won't compile or won't actually protect.

### M1 — H6 webhook `http.Transport` has a duplicate `DialContext` field (won't compile)
The snippet (plan2.md:872-883) declares `DialContext` **twice** in one struct literal — a Go compile error — and the first one has the wrong signature (`(*net.Dialer, error)`). Keep only the second (correct) `DialContext` that pins to `dialIP`. Also: `parsed.Port()` is empty when the URL has no explicit port, so `net.JoinHostPort(dialIP, "")` produces an invalid address — default to `80`/`443` based on `parsed.Scheme`. Otherwise the dispatch-time enforcement (the actual fix for N4) is sound.

### M2 — H2 Part B: `TenantIDs` doesn't exist yet and must be honored by the SQL layer
Verified: `FindTicket` (store/ticket.go:39) and `FindMemo` (store/memo.go:61) have only singular `TenantID`; there is no `TenantIDs`. The plan adds the field and the `resolveTenantIDsFromGUIDs`/`getUserFromContext` helpers, but the fix is only real if the **query builders in all three dialects** (`store/db/sqlite`, `postgres`, `mysql` for both memo and ticket) emit a `tenant_id IN (...)` clause when `TenantIDs` is set. If the SQL layer silently ignores the new field, scoped-admin list queries **fail open** (still return all tenants) — i.e. B2 is not actually fixed. Make "implement `IN` in all 3 dialects + test" an explicit sub-task.

### M3 — H5: do NOT replace `scripts/entrypoint.sh`; merge into it
Verified: the existing `entrypoint.sh` implements `_FILE` secret injection (`file_env` for `MEMOS_DSN`, `OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY`, `AWS_ACCESS_KEY_ID/SECRET`) before its final `exec "$@"`. The plan's rewrite drops all of it — implementing it verbatim breaks Docker/K8s secret injection. Insert the root-check + `chown` + `gosu` privilege-drop **around the existing `file_env` block**, keeping it, and end with `exec gosu memos "$@"` (root) / `exec "$@"` (non-root). The rest of H5 (install `gosu`, no `USER` line, chown-then-drop) is correct.

### M4 — H4: skip CSRF middleware for Authorization-header (Bearer) requests
Wiring `CSRFProtectionMiddleware` onto `gwGroup` will also gate programmatic API clients that authenticate via `Authorization: Bearer` (PATs), which carry **no CSRF risk** (no ambient cookie). Add an early `return next(c)` when the request has an `Authorization` header and no auth cookie, so cross-origin API automation isn't broken. The empty-`Origin`→allow path is acceptable given `SameSite=Lax` is the primary defense, and `Sec-Fetch-Site` handling is correct.

---

## 📝 Recommended (small, closes residual gaps — optional for MVP)

### R1 — Scope `RoleAdmin`'s implicit grants in `ResolveEffectivePermissions`
Even with the new `isSuperAdmin`, `hasPermission` still returns `true` for a `RoleAdmin` on **any** tenant, because `ResolveEffectivePermissions` (permissions.go:144-148) hands every `RoleAdmin` `tenant:read`+`api:config` regardless of `AllowedTenantIDs`. So C1's `hasPermission` branch remains imperfect for scoped admins — the H1 middleware is what actually blocks them. That matches the plan's stated Layer-1 architecture and the 9 handlers are all behind H1 with a `:slug`, so **it's safe as designed**. A ~3-line change (intersect `RoleAdmin`'s implicit grants with `AllowedTenantIDs`) would make C1 self-sufficient and remove the reliance-on-one-layer fragility. Worth doing if time permits.

### R2 — Confirm scoped-admin permissions aren't over-restricted on their *own* tenant
Consequence of the same resolver: scoped admins have only `{tenant:read, api:config}`, so after C1 they'll get **403 on their own tenant** for `chat:logs` handlers (transcripts) and `tenant:write` handlers (update lead status, update settings). Confirm that's intended; if scoped admins are meant to manage their tenant's transcripts/leads, grant them those permissions (via role template) or widen the resolver. Not a security issue — a functionality check.

### R3 — Note the list-route blind spot (out of documented scope)
H1's empty-slug early-return is correctly justified (no-slug routes like `HandleUpdateRoleTemplate` self-check ownership from the record — verified). But no-slug **list** routes in `adminGroup` (e.g. `HandleListTenants`) aren't in the 9-handler scope; verify a scoped admin listing tenants doesn't enumerate others. Pre-existing and outside this plan's stated scope — track separately, don't expand Phase 1.

---

## ✅ Confirmed correct in v2
- **B1 fixed:** `isSuperAdmin` (Option B) makes the C1 guard effective for scoped admins; H1 explicitly named as the primary control; test matrix now covers `RoleUser`, ScopedAdmin, GlobalAdmin, RoleHost.
- **B2 addressed:** filter now derives from `AllowedTenantIDs` (subject to M2 being fully implemented).
- **B3 fixed:** `isSuperUser` keeps `RoleHost` — instance owner retains super-user status.
- **N1:** full 29-site `isSuperUser` cascade inventory + role-matrix tests.
- **N2:** reuses existing `ListUserTenantPermissions` instead of a new method.
- **N4:** SSRF enforcement moved to dispatch with an IP-pinned dialer + redirect re-validation (modulo M1).
- **N6:** pprof loopback step dropped; dev-gate + `MEMOS_ENABLE_PPROF` (default off) retained.
- **H3:** `filepath.Base` + containment assertion, applied to read/delete paths, with a backfill-migration note.

## Recommendation to the implementer
Proceed. Apply **M1–M4** inline (they are mechanical: a compile fix, a "wire it through all 3 DB dialects" task, a file-merge, and a middleware guard). Treat **R1–R2** as a 30-minute follow-up if the schedule allows — they harden defense-in-depth but the plan is safe without them because H1 is the enforced boundary. No further review round needed once M1–M4 are in the diff; catch them in the code review of the implementation PR instead.
