# Adversarial Security Review — `bugs/030/plan.md`

**Reviewer:** Claude (Fable 5)
**Date:** 2026-07-08
**Subject:** Security Remediation Implementation Plan (Phases 0–2), derived from `docs_fable_sec.md`
**Method:** Every technical claim and proposed fix was checked against the working tree. Findings below cite verified `file:line` evidence.

## Verdict: 🔴 REWORK

Phase 0 is sound (approve with one nit). **Phase 1 (tenant isolation) must be reworked** — as written it does not close the Critical/High issues it targets. Phase 2 is approve-with-nits but contains two fixes that would fail or regress if implemented verbatim.

| Phase | Verdict | Reason |
|-------|---------|--------|
| 0 — Emergency (H7, C2, C3) | ✅ Approve w/ nit | Correct; one impractical sub-step (pprof loopback binding) |
| 1 — Tenant isolation (C1, H1, H2) | 🔴 Rework | C1 no-op for scoped admins; H2 doesn't fix the real bypass and demotes RoleHost |
| 2 — Hardening (H3, H4, H5, H6) | 🟡 Approve w/ significant nits | H5 breaks the build; H6 reintroduces DNS-rebinding; H4 has an Origin bypass |

---

## 🔴 Blocking findings (must fix before implementation)

### B1 — C1's per-handler guard is a no-op for *scoped admins* (Phase 1)
The plan adds `if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermX)` to the 9 handlers and presents this as the primary isolation fix. Verified:
- `isAdmin(c)` (`handlers.go:2222`) returns `true` for **any** `RoleHost`/`RoleAdmin`, regardless of `AllowedTenantIDs`. So the guard short-circuits (`!isAdmin` = false) and a tenant-A-scoped admin passes on **any** tenant's slug.
- Even the `hasPermission` branch doesn't help: `ResolveEffectivePermissions` (`permissions.go:141-149`) grants every `RoleAdmin` `tenant:read`+`api:config` and every `RoleHost` `*` on **every** tenant — it never consults `AllowedTenantIDs`.

**Consequence:** After C1 ships exactly as written, a scoped admin can still export any tenant's leads. C1 only closes the `RoleUser` path. The only control that actually contains scoped admins is the H1 middleware's `contains(AllowedTenantIDs, tenant.GUID)` check — meaning **C1's real enforcement is entirely dependent on H1**, which the plan never states. If H1 has any gap, admin isolation silently fails.

**Rework:** State explicitly that H1 is the load-bearing control for admins and C1 is defense-in-depth for `RoleUser`. Better: make `isAdmin(c)` (or a new `isSuperAdmin(c)`) return false for scoped admins so the C1 guard is genuinely effective on its own, and add a cross-tenant test with a *scoped admin* token (the plan's test matrix only covers `RoleUser`).

### B2 — H2 flips `isSuperUser` but leaves the actual cross-tenant bypass in place (Phase 1)
The review's H2 root cause is: scoped admins have a **nil tenant claim**, so the tenant filter is skipped. Verified in `tenant_context.go:25-38`:
```go
func ApplyTicketTenantFilter(c echo.Context, find *store.FindTicket) {
    tenantID := getTenantFromContext(c) // nil for admins — no tenant_id claim
    if tenantID != nil { find.TenantID = tenantID } // skipped → query returns ALL tenants
}
```
The plan's H2 only rewrites `isSuperUser`. But for **List** operations (`ticket_service.go:192`, memo list paths) isolation depends on the *filter*, not `isSuperUser`. Flipping `isSuperUser` to false for a scoped admin therefore:
- does **not** scope List results (filter is still a nil no-op → still all tenants), **or**
- for Get/Update/Delete, forces the fallback creator-ownership check, which **over-restricts** a scoped admin to only rows they personally created — breaking legitimate management of their own tenant.

**Rework:** H2 must also fix the filter derivation — for scoped admins, populate `find.TenantID` (or a new `find.TenantIDs`) from `AllowedTenantIDs` (resolved GUID→ID). Without this, H2 does not achieve tenant isolation and may cause a functional regression.

### B3 — H2 "Decision Point 4 → Option A" (demote `RoleHost` to regular user) is dangerous (Phase 1)
Verified: `RoleHost` is the **instance owner / super-admin** — the first user created (`auth_service.go:234-243`), the only role allowed to change workspace settings (`workspace_setting_service.go:56,69`) and IdP config (`idp_service.go:21`), and it resolves to wildcard permissions (`permissions.go:141-142`). The plan's proposed `isSuperUser` body drops `RoleHost` entirely and its recommendation treats HOST as a regular user.

**Consequence:** The instance owner loses super-user status across tickets/memos/resources and could be locked out of cross-tenant administration.

**Rework:** `isSuperUser` must remain true for `RoleHost`. Correct form:
```go
func isSuperUser(u *store.User) bool {
    return u.Role == store.RoleHost || (u.Role == store.RoleAdmin && len(u.AllowedTenantIDs) == 0)
}
```

---

## 🟡 Significant nits (fix before/at implementation)

### N1 — H2 cascade inventory is incomplete (Phase 1)
`isSuperUser` has ~25 call sites. The plan lists only `ticket_service.go` and `memo_service.go`. It **misses** `resource_service.go:119,547,571,585` and `memo_relation_service.go:96`. Changing `isSuperUser` semantics silently alters resource visibility and memo-relation access. Enumerate and re-test all sites (`grep -rn isSuperUser server/router/api/v1`).

### N2 — H1 proposes a redundant new store method (Phase 1)
The plan adds `HasTenantAccess(userID, tenantID)`. `ListUserTenantPermissions` already exists (`store/rbac.go:99`) and is the established pattern (used by `ResolveEffectivePermissions`). Reuse it (non-empty result ⇒ access) instead of adding a parallel method. Also note the middleware's empty-slug early-return (`return next(c)`) is acceptable only because no-slug tenant-scoped routes (e.g. `HandleUpdateRoleTemplate`, `handlers.go`) self-check ownership from the record — call this out so it isn't "fixed" into a regression.

### N3 — H5 container fix would fail to build/run (Phase 2)
Verified against `Dockerfile.fly`:
- **`gosu` is not installed** — the entrypoint's `exec gosu memos "$@"` fails with "not found".
- **`USER memos` + a root-check entrypoint are contradictory**: with `USER memos`, the entrypoint never runs as UID 0, so the `chown` branch is skipped and a freshly-mounted Fly volume (root-owned) is not writable.

**Fix:** Choose one coherent approach. Recommended: install `gosu`, keep the container entering as root, `chown` the volume in the entrypoint, then `exec gosu memos "$@"` — and drop the `USER memos` line. Apply the same to `Dockerfile.s3.fly`.

### N4 — H6 webhook fix reintroduces DNS-rebinding TOCTOU (Phase 2)
The plan validates with `net.LookupHost` at create/update time, but dispatch (`plugin/webhook/webhook.go:29-38`) builds `http.NewRequest(url)` + `client.Do`, which **re-resolves DNS**. An attacker registers a webhook on a host that resolves public at validation and to `169.254.169.254`/RFC1918 at dispatch. This is exactly the M4 class the review itself flagged. Validation-at-save also doesn't cover later DNS changes.

**Fix:** Enforce at *dispatch* with an IP-pinned `DialContext`: resolve once, reject internal IPs, and dial that exact IP (or a custom `Control` hook that re-checks the connected IP). Keep the `CheckRedirect` re-validation (that part is correct). Save-time validation is fine as a UX pre-check but is not the security boundary.

### N5 — H4 CSRF middleware has an Origin-bypass and isn't wired up (Phase 2)
`if origin == "" { return next(c) }` lets any client that omits `Origin` through — a well-known CSRF-filter weakness. Prefer `Sec-Fetch-Site` when present, and for state-changing methods treat a *missing* Origin conservatively (or rely on `SameSite=Lax`, which already blocks the cross-site POST). Also the plan defines `CSRFProtectionMiddleware` but never shows it registered on the gateway/gRPC-web groups — specify where it mounts. Confirm the widget's cross-site embedding uses widget-key auth (it does), so `SameSite=Lax` won't break it.

### N6 — C2 pprof "loopback binding" is impractical (Phase 0)
pprof is registered as a route group on the single shared Echo listener; you can't bind one group to `127.0.0.1` only without a second listener. Drop that sub-step. The dev-mode gate + `MEMOS_ENABLE_PPROF` flag (default off) is the correct and sufficient control.

---

## ✅ What the plan gets right
- **Phase 0 sequencing** (rotate secrets → gate pprof → fix debug mode) and the dual-key re-encryption migration idea (H7 Option A) are correct; `crypto/encryption.go`'s backup-key path genuinely supports it.
- **C3** (`echoServer.Debug = profile.IsDev()` + custom prod error handler) is correct and low-risk.
- **H3 path traversal**: `filepath.Base` + a `HasPrefix(cleanData)` containment assertion is the right shape and (unlike the current code) correctly catches the absolute-template case too. Nit: also apply to the read/delete paths (plan mentions this) and note that pre-existing tainted `Reference` rows remain until backfilled.
- Correctly identifies C1+H1+H2 as one workstream and mandates a cross-tenant test harness (just extend it per B1).
- The permission mapping decision (reuse `tenant:read`/`tenant:write`/`chat:logs` rather than invent `leads:*`) is sensible — those constants exist (`permissions.go:11-20`).

---

## Required changes to reach "Approve"
1. **B1–B3 (Phase 1):** redesign the isolation fix so scoped admins are actually contained — fix `isAdmin`/effective-permission scoping *or* explicitly make H1 the enforced control and test it with scoped-admin tokens; derive the tenant filter from `AllowedTenantIDs`; keep `RoleHost` super.
2. **N1:** complete the `isSuperUser` cascade inventory and re-test all sites.
3. **N3:** make the Dockerfile/entrypoint coherent (install `gosu`, chown-then-drop, remove `USER`).
4. **N4:** move webhook SSRF enforcement to dispatch with an IP-pinned dialer.
5. **N5/N6:** close the CSRF Origin-bypass and wire the middleware; drop the pprof loopback step.

With B1–B3 reworked and N1/N3/N4 addressed, the plan is approvable.
