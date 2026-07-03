# Prioritized Security Issues: Highest to Lowest Impact

## Tier 1: Active Data Loss & Availability Risks (Fix BEFORE any customer goes live)

### 1. 🔴 SQLite in Production (Single Point of Failure)
- **Impact**: If the single Fly Machine crashes or the volume fails, **all data is lost** — customer KBs, policies, transcripts, user accounts, RAG embeddings. No failover, no horizontal scaling, no zero-downtime deploys.
- **Evidence**: `Dockerfile.local.fly` uses SQLite + a single Fly Volume (`mounts.source = memos_data`). `Taskfile.yml` `fly:ssh:db` confirms SQLite: `sqlite3 /var/opt/memos/memos_prod.db`.
- **Fix**: Migrate to Postgres (Neon or Fly Managed Postgres). Even Neon free tier is better than SQLite for production.

### 2. 🔴 `min_machines_running = 0` (Zero Redundancy)
- **Impact**: Your app can scale to **zero instances**. If the last Machine goes down, your entire service is unreachable until Fly spins up a new one. Combined with SQLite, this means **zero availability SLA**.
- **Evidence**: `fly.toml` line 37: `min_machines_running = 0`
- **Fix**: Set `min_machines_running = 1` (or higher for multi-region).

### 3. 🔴 Shared CPU (`cpu_kind = 'shared'`)
- **Impact**: Shared CPUs are burstable and throttle under sustained load. For a latency-sensitive AI chat app with LLM calls and embedding computations, this means **unpredictable response times and timeouts** for customers.
- **Evidence**: `fly.toml` lines 46-48: `cpu_kind = 'shared'`
- **Fix**: Change to `cpu_kind = 'performance'`. This costs more but is mandatory for production per Fly's own checklist.

### 4. 🔴 No Health Checks
- **Impact**: Fly cannot detect when your app is hung, crashed, or in an unhealthy state. **No automatic recovery** from failures.
- **Evidence**: `fly.toml` has no `[[checks]]` block.
- **Fix**: Add HTTP health check: `[[checks]]` with `path = "/api/health"`.

---

## Tier 2: Authentication & Session Vulnerabilities (Fix now, before customers onboard)

### 5. 🔴 7-Day JWT Access Tokens (vs. recommended 15 minutes)
- **Impact**: If a token is stolen (XSS, compromised client, MITM), the attacker has **7 days of full access**. The upstream CVE reports confirm Memos stores tokens in browser sessionStorage, making them extractable by any injected script.
- **Evidence**: `server/router/api/v1/auth.go` line 18: `AccessTokenDuration = 7 * 24 * time.Hour`
- **Fix**: Reduce to 15 minutes, implement refresh token rotation (30-day HTTP-only cookie).

### 6. 🔴 No Session Revocation on Password Change (CVE GO-2025-4127)
- **Impact**: When a user changes their password (e.g., after account compromise), all existing sessions remain valid. **The attacker keeps access**. This undermines your entire incident response story.
- **Evidence**: No code found that invalidates sessions/tokens on password change.
- **Fix**: Increment a token version in the user record; check it during auth. Or simply delete all refresh tokens on password change.

### 7. 🔴 No WAF/Proxy Rules for CVE-2026-6634 (Auth Bypass)
- **Impact**: **Unpatched auth bypass** lets any authenticated user modify `additionalStyle`/`additionalScript` instance settings — **stored XSS vector**. Any script injected can steal all user tokens from sessionStorage. This is a live, unpatched CVE.
- **Evidence**: This is an upstream Memos CVE. Your codebase inherits all upstream Memos code. No nginx/WAF sidecar is configured.
- **Fix**: Add an nginx sidecar or Fly edge rule to block `/api/v1/*instance*` and admin settings routes for non-admin roles. Or deploy `Dockerfile.fly` (S3-based) instead of `Dockerfile.local.fly` which at least has a different code path.

---

## Tier 3: Multi-Tenant Isolation Gaps (Fix before onboarding 2nd+ customer)

### 8. 🟡 Rate Limiting Missing on Most Admin Mutation Endpoints
- **Impact**: An attacker with `tenant:admin` permission can spam `HandleUpdateTenant`, `HandleGrantPermission`, `HandleOnboard`, `HandleDeleteTenant` with **no rate limiting**. This enables brute-force attacks and DoS.
- **Evidence**: `checkAdminMutationRateLimit()` is only called in role-template endpoints (`HandleCreateRoleTemplate`, `HandleUpdateRoleTemplate`, `HandleDeleteRoleTemplate`, `HandleAssignRoleTemplate`). It is NOT called in `HandleUpdateTenant`, `HandleGrantPermission`, `HandleRevokePermission`, `HandleOnboard`, `HandleDeleteTenant`, `HandleRestoreFileVersion`, `HandleImport`, `HandleImportSingleFile`, `HandleSetLLMConfig`, `HandleReindexTenant`.
- **Fix**: Add `checkAdminMutationRateLimit(c, tenant.ID)` to all mutation endpoints that require `tenant:admin` or `api:config`.

### 9. 🟡 `isDomainAllowed()` Fail-Open on Invalid Config
- **Impact**: If `AllowedDomains` contains invalid JSON (e.g., corrupted data), the function returns `true` (allow all origins). This silently disables the CORS-like protection. Worse, an empty list also allows all.
- **Evidence**: `handlers.go` lines 1887-1891: `if err != nil { return true }` and `if len(domains) == 0 { return true }`
- **Fix**: Fail-closed instead of fail-open: return `false` on parse errors. Empty list should also deny (or at least log a warning).

### 10. 🟡 Rate Limiting Missing on External Chat Endpoint
- **Impact**: The `HandleChatExternal` endpoint (customer-facing chat widget) has **no rate limiting** beyond what the LLM provider imposes. A single IP can hammer your LLM API, running up your OpenRouter bill and degrading service for everyone.
- **Evidence**: `HandleChatExternal` checks no rate limit. `HandleChatInternal` similarly has none.
- **Fix**: Add IP-based rate limiting using the existing `CheckRateLimit` infrastructure with a dedicated rate limit key.

---

## Tier 4: Operational & Monitoring Risks (Fix before scaling)

### 11. 🟡 No Monitoring / Observability
- **Impact**: You have **no visibility** into application health, error rates, or performance. If something breaks, you'll only know when a customer complains. No metrics, no alerting, no log aggregation.
- **Fix**: Set up Fly managed Prometheus/Grafana, Sentry error tracking, and Fly Log Shipper.

### 12. 🟡 No Backup / Disaster Recovery Plan
- **Impact**: If the Fly Volume is corrupted or the database is accidentally dropped, **all data is permanently lost**. No restore procedure exists.
- **Fix**: Document and test: (a) Neon Postgres backup/restore, (b) LanceDB rebuild from source files, (c) JWT secret rotation procedure.

### 13. 🟡 No CI/CD Pipeline
- **Impact**: All deployments are manual (`fly deploy`). No automated testing, no security scanning, no review apps. A bad deploy takes down production.
- **Fix**: Set up GitHub Actions with: build + test + security scan → deploy to staging → manual approval → deploy to production.

---

## Tier 5: Defense-in-Depth (Fix when mature)

### 14. 🟢 LanceDB Backup Strategy Missing
- RAG embeddings can be rebuilt from source files, but this takes time. Document the restore procedure.

### 15. 🟢 No Content Security Policy Headers
- Adding CSP would mitigate XSS risks from injected scripts.

### 16. 🟢 No Dependency Vulnerability Scanning
- `gosec` and `npm audit` should be in the build pipeline.

---

## Summary: Minimum Viable Security for First Customer

If you want to onboard **one paying customer safely**, here's the absolute minimum:
1. ✅ **Change `cpu_kind` to `performance`** (2-line config change)
2. ✅ **Set `min_machines_running = 1`** (1-line config change)
3. ✅ **Add `[[checks]]` health check** (10-line config addition)
4. ✅ **Reduce JWT lifetime** from 7 days to 1 hour (1-line code change)
5. ✅ **Add WAF/nginx sidecar** to block CVE-2026-6634 paths
6. ✅ **Add rate limiting to external chat** (prevent LLM bill shock)

These 6 fixes take ~2 hours of engineering time and drop the "will lose all data" risk from 100% to near-zero.

Everything else (Postgres migration, session revocation, monitoring, CI/CD) can be done in subsequent sprints — but **SQLite + `min_machines_running=0` + shared CPU is an incident waiting to happen on day one.**