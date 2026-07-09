# bchat Security Review — Findings & Remediation Plan

**Date:** 2026-07-08
**Reviewer:** Claude (Fable 5)
**Scope:** bchat-specific code (multi-tenant agent platform layered on Memos). Upstream-Memos-inherited issues are noted in passing but not the focus.
**Assumed deployment:** Public, internet-facing, **multi-tenant SaaS** — multiple untrusted tenants share one instance. Severity is ranked against this threat model.
**Method:** Four parallel source audits (auth/tenant-isolation, injection/data-layer, secrets/crypto/LLM, web/config/infra) plus manual verification of every Critical and the headline High findings against the working tree.

---

## Executive Summary

The core cryptographic and token plumbing is **sound** — AES-256-GCM with Argon2id, JWT algorithm pinning with no fallback secret, HMAC bridge auth with replay protection, and layered denial-of-wallet limits on the public chat endpoint. The real risk is concentrated in **authorization consistency**, **multi-tenant isolation**, and **operational/infra exposure**.

The single most serious issue is a **cross-tenant IDOR** that lets any authenticated user export another tenant's lead PII (names/emails/phones) as CSV. This is compounded by a tenant-binding middleware that, by design, waves through every non-admin user — meaning the "admin only" route group is not actually an access-control boundary. Alongside this, **publicly exposed pprof/debug endpoints** and **Echo debug mode forced on in production** are trivially exploitable for information disclosure and denial of service.

| Severity | Count | Headline items |
|----------|-------|----------------|
| 🔴 Critical | 3 | Cross-tenant lead/transcript/settings IDOR; public pprof; Echo debug mode in prod |
| 🟠 High | 7 | Broken tenant-binding; inconsistent super-user; path traversal; CSRF; root container; webhook SSRF; live secrets in tree |
| 🟡 Medium | ~10 | Wildcard CORS; missing security headers; unbounded upload body; DNS-rebinding SSRF; non-blocking prompt-injection defense; tenant-less PATs; committed example master key |
| 🟢 Low | ~6 | Verifier fails open; spoofable rate-limit key; 122-bit JWT secret; nil-tenant legacy rows |

Legend for origin tags: **[bchat]** introduced by this team · **[upstream]** inherited from Memos (noted, not prioritized) · **[mixed]** upstream surface, bchat-specific configuration.

---

## 🔴 Critical

### C1 — Cross-tenant IDOR on leads / transcripts / settings → PII exfiltration **[bchat]**

**Location:** `server/router/api/v1/agent/handlers.go`
- `HandleListTranscripts` (:5896), `HandleGetTranscript` (:5924), `HandleDeleteTranscript` (:5953)
- `HandleListLeads` (:5984), `HandleGetLead` (:6019), `HandleUpdateLeadStatus` (:6040), `HandleExportLeads` (:6076)
- `HandleGetTenantSettings` (:6147), `HandleUpdateTenantSettings` (:6177)

**Description:** Each of these handlers resolves the tenant **solely from the user-supplied `:slug`** (`GetAgentTenant{Slug:&slug}`) and then reads/modifies/deletes/exports data scoped only to that slug. Unlike ~90 sibling handlers in the same file, they contain **no `isAdmin(c)` and no `hasPermission(...)` check**. Verified directly: the dangerous *mutation* handlers registered in the same group (`HandleDeleteTenant`, `HandleUpdateTenant`, `HandleImportSingleFile`, `HandleGetTenantFullConfig`) **do** have internal checks — these nine do not.

They are registered only in `adminGroup` (`v1.go:368-380`), whose gates are `AuthMiddleware` + `adminCORS` + `TenantBindingMiddleware`. `AuthMiddleware` accepts any valid user token; `TenantBindingMiddleware` waves through all non-admin users (see H1); `adminCORS` is a browser hint, not server-side access control.

**Exploit:** A `RoleUser` legitimately scoped to tenant A obtains a normal JWT, then:
```
GET /api/v1/agent/<tenant-B-slug>/leads/export
```
returns tenant B's CSV of `name, email, phone, topic, location, detected_intent, ...` (`handlers.go:6093-6116`). The attacker can enumerate slugs, read/delete any tenant's transcripts, and toggle transcript recording tenant-wide.

**Impact:** Cross-tenant PII disclosure and tampering — a direct breach of the platform's core "every request scoped to a single tenant" guarantee, with regulatory (GDPR/CCPA) exposure given the PII involved.

**Fix:**
1. Add the standard guard to every one of the nine handlers:
   `if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermLeadsRead /* or appropriate perm */) { return 403 }` — mirroring `HandleListSessions` (:3188) and `HandleChatInternal` (:610).
2. Choose the correct permission per action (read vs. write vs. delete vs. export).
3. Add a regression test asserting a tenant-A `RoleUser` gets 403 on a tenant-B slug for each handler.
4. See H1 — fix the middleware so this class of omission cannot silently expose data again.

---

### C2 — pprof / debug profiler endpoints exposed publicly with no authentication **[bchat]**

**Location:** `server/profiler/profiler.go:26-60`, registered at `server/server.go:57-58`.

**Description:** `s.profiler.RegisterRoutes(echoServer)` mounts `/debug/pprof/*` on the **root** Echo instance, before any auth middleware and with no mode gating. Exposed: `/debug/pprof`, `/cmdline`, `/profile`, `/trace`, `/symbol`, `/allocs`, `/block`, `/goroutine`, `/heap`, `/mutex`, `/threadcreate`, `/memstats`. Verified: registration is unconditional (not behind `IsDev()` or an env flag).

**Exploit / Impact:**
- `GET /debug/pprof/cmdline` → leaks process command line and arguments.
- `GET /debug/pprof/heap` and `/goroutine?debug=2` → memory/stack dumps that can contain in-flight secrets (JWTs, decrypted API keys, request bodies).
- `GET /debug/pprof/profile?seconds=30` and `/trace` → trivial CPU-pegging / blocking **DoS**, no auth required.

**Fix:** Gate the entire profiler behind (a) `profile.IsDev()` **and/or** (b) an explicit `MEMOS_ENABLE_PPROF` env flag, **and** (c) bind it to loopback or require an auth/IP-allowlist middleware even when enabled. Do not mount it on the public listener in prod.

---

### C3 — Echo debug mode hard-coded on in production **[bchat]**

**Location:** `server/server.go:50` — `echoServer.Debug = true` (unconditional).

**Description:** Debug mode is set regardless of `MEMOS_MODE`. With it on, Echo's default HTTP error handler serializes internal error messages (and, for panics recovered by the middleware, stack details) into HTTP responses.

**Impact:** Information disclosure on every error path — internal file paths, dependency internals, query fragments — to any client, always-on in prod.

**Fix:** `echoServer.Debug = profile.IsDev()` (or drive from `MEMOS_MODE == "dev"/"demo"`). Pair with a custom `HTTPErrorHandler` that returns generic messages in prod and logs details server-side.

---

## 🟠 High

### H1 — `TenantBindingMiddleware` bypasses all non-admin users (root cause of C1) **[bchat]**

**Location:** `server/router/api/v1/tenant_binding.go:31-38`.

```go
if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0 { return next(c) } // superuser
if user.Role != store.RoleAdmin { return next(c) }                                    // any RoleUser/RoleHost passes
```

**Description:** The middleware only enforces slug↔tenant binding for `RoleAdmin` users that have a non-empty `AllowedTenantIDs`. Every `RoleUser` is explicitly passed through with no tenant check. Handlers in `adminGroup` that assume this middleware protects them (C1) therefore have no isolation at all. The group is named "admin only" (`v1.go:331`) but is not an access-control boundary.

**Fix:** Enforce binding for **all** authenticated, tenant-scoped roles: resolve the slug's tenant, and require the user to have an explicit grant for it (via `AllowedTenantIDs` for scoped admins **and** the RBAC permission grant for `RoleUser`). Treat "no slug match to a permitted tenant" as 403. This makes the middleware a genuine defense-in-depth layer behind the per-handler checks from C1.

### H2 — Inconsistent super-user definition lets scoped admins cross tenants **[bchat]**

**Location:** `server/router/api/v1/common.go:66-68` vs `tenant_binding.go:31`.

**Description:** `isSuperUser` treats **every** `RoleAdmin` (and `RoleHost`) as a global super-user, bypassing tenant-ownership checks in tickets (`ticket_service.go:192,279,341,417`) and memos (`memo_service.go:270,280,318,325,451,458`). But `TenantBindingMiddleware` defines a super-user narrowly as `RoleAdmin` **with empty `AllowedTenantIDs`**. Because admin sign-in never sets a `tenant_id` claim (`auth_service.go:172-188`), `getTenantFromContext` returns nil for admins, so `ApplyTenantFilter`/`ApplyTicketTenantFilter` become no-ops. Net: a tenant-A-restricted admin can read/update **all** tenants' tickets and memos.

**Fix:** Unify on one super-user definition — `RoleAdmin && len(AllowedTenantIDs) == 0`. Make `isSuperUser` reject scoped admins, and for scoped admins derive an effective tenant filter from `AllowedTenantIDs` rather than treating a nil claim as "see everything."

### H3 — Path traversal: arbitrary file read / write / delete via unsanitized upload filename **[upstream]**

**Location:** `server/router/api/v1/resource_service.go:79,319-341,460`; `store/resource.go:112-130`; `memo_resource_service.go:252-263`.

**Description:** User-controlled `Filename` is stored and substituted into the storage path template (`{filename}`) with no `filepath.Base`/`Clean` and no containment check. `filepath.Join(profile.Data, osPath)` collapses `../` segments, so `Filename: "../../../../etc/cron.d/x"` escapes the data dir. Yields authenticated arbitrary file **write** (`os.WriteFile`, :338), **read** (download), and **delete** (`os.Remove`). An absolute `FilepathTemplate` skips the join entirely (:324). *Inherited from Memos, but live and multi-tenant here.*

**Fix:** `filename = filepath.Base(filename)`, reject `..`/absolute components, and after building the path assert `strings.HasPrefix(filepath.Clean(osPath), filepath.Clean(profile.Data)+string(os.PathSeparator))` before any FS operation.

### H4 — CSRF: `SameSite=None` cookie auth with no CSRF token **[mixed]**

**Location:** cookie set `auth_service.go:293-320,535-548`; read by gateway/gRPC-web `acl.go:136-152`; wired `v1.go:198-200,219`.

**Description:** In prod (https Origin) the auth cookie is issued `SameSite=None; Secure`, and the gRPC-gateway (`/api/v1/*`) and gRPC-web endpoints authenticate from it. There is no CSRF/anti-forgery token anywhere. `SameSite=None` means the browser attaches the cookie on cross-site requests, so a malicious page can drive state-changing calls as a logged-in admin. Wildcard CORS (M1) does not mitigate blind writes.

**Fix:** Prefer `SameSite=Lax` unless a genuine cross-site embedding need exists; where cross-site is required, add a double-submit CSRF token or an `Origin`/`Sec-Fetch-Site` allowlist check on state-changing methods.

### H5 — Container runs as root **[bchat]**

**Location:** `Dockerfile.fly`, `Dockerfile.s3.fly` (no `USER` directive); `scripts/entrypoint.sh`.

**Description:** Final image runs `ENTRYPOINT ["./entrypoint.sh", "./memos"]` as UID 0; the data volume is root-owned. Any RCE/container escape runs as root.

**Fix:** Add a non-root user (`useradd`), `chown` the data dir, and `USER app` in the final stage. Verify the Fly volume mount permissions accommodate the non-root UID.

### H6 — Webhook SSRF (no URL validation) **[upstream]**

**Location:** `webhook_service.go:18-33,76-96` (create/update store URL with only `TrimSpace`); dispatched `plugin/webhook/webhook.go:23-38`; triggered `memo_service.go:800-809`.

**Description:** User-supplied webhook URLs get no scheme allowlist, no internal-IP/metadata block, and no redirect restriction. A user can target `http://169.254.169.254/…`, `http://localhost:PORT`, or internal hosts. Primarily blind SSRF/port-probing; note `webhook.go:50,62` embed response body/status in returned errors, so any path surfacing the sync error leaks internal response data. *Upstream surface, but exploitable.*

**Fix:** Validate on save and on dispatch: require `http(s)`, resolve host and reject loopback/private/link-local/CGNAT/unspecified, pin the connection to the validated IP, and cap/deny redirects.

### H7 — Live credentials in the working tree (rotate) **[bchat, operational]**

**Location:** `.env` (real `OPENROUTER_API_KEY=sk-or-v1-…`, `ENCRYPTION_MASTER_KEY=REDACTED-UUID`); `bugs/026/s3_probe/.env` (real Tigris `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`).

**Description:** Verified **gitignored, not tracked, not in current history**, and excluded from the Docker image (`.dockerignore`). However these are genuine, working credentials on disk, and `session-ses_0c0f.md` indicates the OpenRouter key previously tripped GitHub Push Protection (i.e., was committed once). The encryption master key protects **all** tenant API keys.

**Fix:** **Rotate** the OpenRouter key, the encryption master key, and the Tigris S3 credentials. Move developer secrets out of repo-local `.env` files into a secret manager or per-developer untracked location outside the checkout. After rotating the master key, plan re-encryption of stored tenant keys (the backup-key path in `crypto/encryption.go` supports a migration window).

---

## 🟡 Medium

- **M1 — Wildcard CORS on the whole gateway [bchat].** `v1.go:170-171` applies `middleware.CORS()` (default `AllowOrigins: *`) to `gwGroup` serving `Any("/api/v1/*")` and `/file/*`. No `Allow-Credentials`, so responses aren't cross-origin readable, but the intended restrictive `ADMIN_CORS_ORIGINS` policy is bypassed for the core gateway surface. **Fix:** apply the restrictive CORS to the gateway; keep `*` only on the genuinely public widget/chat routes.
- **M2 — No security headers [bchat].** No `X-Frame-Options`/CSP/HSTS globally → the React admin app can be framed (clickjacking against admins) and there's no CSP to contain XSS. **Fix:** add a headers middleware; `DENY` framing for the admin app (leave the widget iframe route exempt), add HSTS + a baseline CSP.
- **M3 — Unbounded upload body on admin/import routes [bchat].** `BodyLimit("16KB")` is only on the public group (`v1.go:254`); admin/import handlers `io.ReadAll` whole files (`handlers.go:1074,1364,1437`), and gRPC allows `MaxRecvMsgSize = MaxInt32` (~2GB, `server.go:91`). Low-privilege `files:upload` users can exhaust the 1GB VM. **Fix:** per-route `BodyLimit`, a sane gRPC max message size, and streaming/size caps on import.
- **M4 — DNS-rebinding / TOCTOU SSRF in link preview [upstream].** `plugin/httpgetter/html_meta.go:35-40` validates the URL then re-resolves DNS on the actual `Get`; also omits `0.0.0.0`/CGNAT. Reachable via `markdown_service.go:44`. **Fix:** resolve once and pin the dialer to the validated IP; extend the block list. (`plugin/httpgetter/image.go` has no validation at all — latent, currently no caller.)
- **M5 — Prompt-injection defense is non-blocking + indirect injection [bchat].** `detectPromptInjection` (`service.go:1903-1929`) matches ~8 fixed English phrases and only logs. Tenant/RAG/observation content is concatenated verbatim into the system prompt with plaintext `=== SECTION ===` delimiters (`service.go:2461-2730,2841`), so a poisoned KB page or planted observation memory can carry a competing instruction block. **Fix:** treat detection as defense-in-depth only (correct), but harden the prompt: use unambiguous, hard-to-forge delimiters, clearly mark all tenant/RAG content as untrusted data, and keep the `verifier` (fix M9). Cross-tenant retrieval itself **is** correctly isolated (`vectordb_lance.go:891`).
- **M6 — Committed example `ENCRYPTION_MASTER_KEY` [bchat].** `build/README.MD:74` and `docs/DOCS_UNIFIED_ENV_WORKFLOW.MD:108` ship a concrete UUID value. A copy-paste into prod yields a publicly known master key. Not a code fallback. **Fix:** replace with an obvious placeholder + a "generate with `uuidgen`/`openssl rand`" instruction.
- **M7 — Self-issuable tenant-less, non-expiring PATs [bchat].** `CreateUserAccessToken` (`user_service.go:456-461`) mints tokens with `tenantID = nil` (tenant filters become no-ops) and, if `ExpiresAt` is omitted, no `exp` claim at all; the documented `MaxNeverExpireDuration` cap (`auth.go:20`) is referenced nowhere. **Fix:** carry the caller's tenant into the PAT, enforce a max expiry, and require an explicit expiry.
- **M8 — No file-type/content validation on KB/POLICY/SCRIPT uploads [bchat].** `handlers.go:1058-1101` ingests arbitrary content into the RAG/LLM pipeline (feeds M5 and token-cost abuse). No archive extraction exists, so no zip-bomb path. **Fix:** validate MIME/extension and cap total token volume per upload.
- **M9 — Verifier fails open [bchat].** `parseVerificationResult` returns `Compliant: true` on unparseable JSON (`verifier.go:323-328`), so a confused/injected verifier silently passes output. **Fix:** fail closed (treat parse failure as non-compliant / retry).
- **M10 — `ListMemos`/`GetMemo` unauthenticated + cross-tenant [upstream].** In the no-auth allowlist (`acl_config.go:20-21`) → no tenant context → public memos of every tenant are enumerable anonymously. *Upstream default; security-relevant in multi-tenant.* **Fix:** require tenant scoping on these even for anonymous reads, or remove from the public allowlist.

---

## 🟢 Low

- **L1 — Spoofable rate-limit key [bchat].** Limits key on `c.RealIP()` / `X-Forwarded-For` (`auth_service.go:364`, `handlers.go:439`). If Echo's `IPExtractor` isn't pinned to the Fly proxy, `X-Forwarded-For` spoofing bypasses login and denial-of-wallet limits. **Fix:** set a trusted-proxy `IPExtractor`.
- **L2 — JWT secret is a 122-bit UUID [bchat].** `uuid.NewString()` (`server.go:225`) vs a 256-bit random key. Adequate for HS256 in practice. **Fix (defense-in-depth):** `crypto/rand` 32 bytes.
- **L3 — Widget key isn't a real secret [bchat].** Served in public `embed.js`/iframe (`handlers.go:2056-2061,2116`) yet used as the chat "edge gate." Anti-abuse then rests on the RPM limiter + optional `AllowedDomains` (disabled by default). **Fix:** document that `AllowedDomains` is the real gate; consider requiring it for production tenants.
- **L4 — Legacy `TenantID == nil` rows skip tenant checks [bchat].** Guards like `if record.TenantID != nil` (`ticket_service.go:279`, `memo_service.go:268`) exempt unscoped rows; mitigated today by creator checks. **Fix:** backfill tenant IDs; treat nil as deny.
- **L5 — `HandleSelectTenant` O(users×tokens) scan + non-constant-time match [bchat].** `auth_service.go:470-496`; also orphan `selection:` tokens never GC'd. Not an auth bypass (JWT parse fails first). **Fix:** index selection tokens; constant-time compare; TTL cleanup.
- **L6 — Reflected `Host`/`X-Forwarded-Proto` in widget base URL [bchat].** `handlers.go:2036-2041,2093-2098` embeds client-controlled values into served JS/HTML. Minor link-injection/cache-poisoning. **Fix:** derive base URL from configured `InstanceURL`.

---

## Verified Clean / Done Right (no action)

- **Crypto** (`internal/crypto/encryption.go`): AES-256-GCM authenticated encryption, Argon2id KDF, random per-message nonce, random salt, distinct backup-key salt. No ECB/static-IV/hardcoded-key issues.
- **JWT**: HS256 with algorithm pinning in every verifier (`acl.go:93`, `v1.go:426`, `user_service.go:403,468`); no default/fallback secret (start fails safe if empty).
- **Bridge HMAC auth** (`agent/bridge_middleware.go`): canonical string + body hash, constant-time compare, nonce replay store, ±5-min freshness, 1 MiB cap, encrypted shared secret.
- **RAG cross-tenant isolation**: vector search always prepends `tenant_id = …` (`vectordb_lance.go:891`), indexed; deletes tenant+audience scoped.
- **LLM output rendering**: `textContent` / escaped JSX (`widget/src/ui/Messages.ts:95`, `web/src/components/AgentChatWidget.tsx:313`) — no XSS via agent output.
- **Denial-of-wallet on public chat**: per-IP RPM + global per-tenant cap + per-session turn cap + 16KB body + message-length check + Fly request timeout.
- **Secret logging**: keys/tokens never logged; gRPC logger logs method+error only; OpenRouter errors scrubbed.
- **SQL / CEL / command injection**: parameterized across sqlite/postgres/mysql; CEL identifiers whitelisted and values bound; `bd`/beads uses argv slices (no shell).

*(Note: `web/src/components/MemoContent/CodeBlock.tsx` `__html` raw-HTML fence and `web/src/App.tsx` `additionalScript`/`additionalStyle` innerHTML are upstream-Memos XSS surfaces — admin/author-scoped, not agent/LLM-driven. Flagged for awareness; out of the bchat-specific focus.)*

---

## Prioritized Remediation Plan

Sequenced by impact and by unblocking dependencies. Effort is rough dev-time (S ≤ 0.5d, M ≈ 1–2d, L ≈ 3–5d).

### Phase 0 — Emergency (do today, hours)
| # | Action | Effort |
|---|--------|--------|
| H7 | Rotate OpenRouter key, encryption master key, Tigris S3 creds; move dev secrets out of repo-local `.env` | S |
| C2 | Gate pprof behind dev-mode + env flag + loopback/allowlist; remove from public listener | S |
| C3 | `echoServer.Debug = profile.IsDev()`; add prod error handler | S |

### Phase 1 — Critical isolation (this sprint)
| # | Action | Effort |
|---|--------|--------|
| C1 | Add `isAdmin`/`hasPermission` guards to the 9 lead/transcript/settings handlers + regression tests | M |
| H1 | Rework `TenantBindingMiddleware` to enforce binding for all tenant-scoped roles | M |
| H2 | Unify super-user definition; derive tenant filter from `AllowedTenantIDs` for scoped admins | M |

> C1/H1/H2 are one workstream — fix the middleware (H1/H2) so per-handler omissions can't leak again, and add the explicit per-handler checks (C1) as the primary boundary. Ship together with cross-tenant denial tests.

### Phase 2 — High-impact hardening (next sprint)
| # | Action | Effort |
|---|--------|--------|
| H3 | `filepath.Base` + containment assertion on resource paths (read/write/delete) | M |
| H4 | `SameSite=Lax` or CSRF token / Origin check on state-changing routes | M |
| H5 | Non-root container user + volume permissions | S |
| H6 | Webhook URL validation (scheme + internal-IP block + IP-pinned dial + redirect cap) | M |

### Phase 3 — Defense in depth
| # | Action | Effort |
|---|--------|--------|
| M1 | Restrictive CORS on gateway; `*` only on public widget/chat | S |
| M2 | Security-headers middleware (X-Frame-Options, CSP, HSTS) | S |
| M3 | Per-route body limits + sane gRPC max message size | S |
| M4 | Fix DNS-rebinding in link preview; add validation to `image.go` or remove it | M |
| M5/M8/M9 | Harden system-prompt delimiters + mark untrusted data; validate upload types; verifier fail-closed | M |
| M6/M7 | Placeholder-ize example master key; scope+expire PATs | S |

### Phase 4 — Cleanups
L1–L6 as capacity allows (trusted-proxy IP extractor, 256-bit JWT secret, `AllowedDomains` guidance, nil-tenant backfill, selection-token indexing/TTL, base-URL from config).

---

## Suggested Verification

- **Cross-tenant test harness:** for every tenant-scoped route, assert a tenant-A `RoleUser`/scoped-admin token gets 403 against a tenant-B slug. This would have caught C1, H1, and H2.
- **Auth default-deny lint:** a test or CI check that every handler in `adminGroup`/tenant-scoped groups calls `isAdmin` or `hasPermission`.
- **External exposure check:** confirm `/debug/pprof`, `/metrics`, and error-verbosity are not reachable on the public Fly URL in prod.
