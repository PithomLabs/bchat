# Comprehensive Security Audit: bchat Deployment to Fly.io

## Executive Summary

bchat is a **multi-tenant AI chat agent platform** built on top of Memos. The codebase has implemented a **custom RBAC system** (role templates, tenant-level permissions) that goes significantly beyond what vanilla Memos provides. However, there are **critical gaps** between the current state and the security requirements defined in `plan_sec.md`, `prompt_sec.md`, `security.md`, `security_kimi.md`, and `fly_checklist.md`.

---

## 1. RBAC & Permission System (vs. plan_sec.md)

### ✅ Implemented (Good)
- **Role templates** (`role_template.go`, `permissions.go`) — system templates with sentinel `-1`, custom templates per tenant
- **`ResolvedPermission`** struct with source metadata (`SourceGlobalRole`, `SourceTenantTemplate`, `SourceExplicit`)
- **`ResolveEffectivePermissions()`** — unions ALL rows via `ListUserTenantPermissions`
- **`hasPermission()`** with per-request caching in Echo context
- **`ValidatePermissions()`** — rejects wildcard-only templates
- **`checkAdminMutationRateLimit()`** — RPM from `TenantConfig.AdminMutationRateLimitRPM` (default 30)
- **`HandleGrantPermission`** — targets `source_template_id IS NULL` only (explicit grants)
- **System template protection** — `tenant_id = -1` sentinel, deletion guard with `SELECT COUNT(*)`
- **Visibility control** — system template names visible to `tenant:read`, contents only to `tenant:admin`

### ❌ Gaps / Concerns
1. **`PermWildcard` still exists** in `ContainsPermission()` — HOST users get wildcard, which bypasses all permission checks. This is by design for HOST, but the wildcard check in `ContainsPermission` could allow privilege escalation if a non-HOST user somehow gets `*` in their permissions.
2. **No `PermTenantAdmin` in `AllPermissions`** — The plan says to add it, but it's already there at line 12 of `permissions.go`. ✅ Actually this is fine.
3. **Rate limiting is per-request, not per-endpoint** — The `checkAdminMutationRateLimit` is only called on role-template mutation endpoints, not on all admin mutations (e.g., `HandleUpdateTenant`, `HandleDeleteTenant`, `HandleGrantPermission` are NOT rate-limited).
4. **No pre-migration assertion** — The plan specifies `SELECT COUNT(*) FROM agent_tenant WHERE id <= 0` must return 0 before migration. This is a one-time check that needs to be verified before deployment.

---

## 2. Authentication & Session Security (vs. security.md / security_kimi.md)

### Current State
- **JWT tokens**: HS256, 7-day access tokens (`AccessTokenDuration = 7 * 24 * time.Hour`)
- **Cookie-based**: `memos.access-token` cookie
- **Password hashing**: bcrypt
- **SSO support**: OAuth2 identity providers
- **Bridge HMAC middleware**: HMAC-SHA256 for external bridge API calls

### ❌ Critical Issues
1. **7-day access tokens** — The security docs say 15-minute access tokens. Your code has **7-day tokens**. This is a significant deviation from the security recommendations. Long-lived JWTs increase the blast radius of token theft.
2. **No refresh token rotation** — The security docs describe a 3-token system (15-min access + 30-day refresh + PATs). Your code only has a single 7-day access token with no refresh mechanism.
3. **No session revocation on password change** — `GO-2025-4127` (tokens remain valid after password change) is NOT mitigated. The code has no mechanism to invalidate existing sessions when a password changes.
4. **No PAT (Personal Access Token) audit** — No code found for managing/rotating PATs.
5. **JWT secret management** — The code uses `GenerateAccessToken` with a `secret []byte` parameter. Need to verify this comes from a secure source (not hardcoded). The `MEMOS_MODE=prod` is set, but the actual JWT secret source needs verification.

---

## 3. Infrastructure & Deployment (vs. fly_checklist.md)

### Current `fly.toml` Analysis

```toml
app = 'bchat'
primary_region = 'sjc'
```

| Checklist Item | Status | Issue |
|---|---|---|
| **SSO for Fly org** | ❌ Not configured | No evidence of Fly org SSO |
| **Staging/prod isolation** | ❌ Not configured | Single `fly.toml`, no separate staging org |
| **Least privilege access** | ❌ Not verified | No access token policy documented |
| **Secrets in `[env]`** | ❌ **CRITICAL** | `fly.toml` has `EMBEDDING_MODEL`, `LLM_MODEL`, `LLM_MODEL_REASONING`, `MEMOS_MODE`, `RAG_PIPELINE_ENABLED`, etc. in `[env]` — these are not secrets per se, but the pattern is dangerous |
| **Private services exposure** | ❌ Not verified | No `fly ips list` check documented |
| **`force_https = true`** | ✅ Configured | Line 34 |
| **Custom domain + TLS** | ❌ Not configured | No `fly certs` setup documented |
| **Dedicated IPv4** | ❌ Not configured | Using shared IPv4 |
| **Performance CPUs** | ❌ **CRITICAL** | `cpu_kind = 'shared'` — should be `performance` for production |
| **Swap to disk** | ❌ Not configured | No `swap_size_mb` |
| **Multiple Machines** | ❌ **CRITICAL** | `min_machines_running = 0` — zero redundancy |
| **Multi-region** | ❌ Not configured | Single region `sjc` |
| **Health checks** | ❌ Not configured | No `[[checks]]` block in `fly.toml` |
| **Managed Postgres** | ❌ **CRITICAL** | Using SQLite on a single volume |
| **Backup/DR plan** | ❌ Not documented | No backup strategy documented |
| **Metrics/Grafana** | ❌ Not configured | No monitoring setup |
| **Sentry error tracking** | ❌ Not configured | Not integrated |
| **Log export** | ❌ Not configured | No log shipper |
| **CI/CD pipeline** | ❌ Not configured | No GitHub Actions |
| **Review apps** | ❌ Not configured | No ephemeral preview environments |

### Critical Infrastructure Issues

1. **SQLite in production** — The `Dockerfile.local.fly` uses SQLite on a single Fly Volume. This is the #1 issue flagged by ALL security documents. Single-writer, single-point-of-failure, no horizontal scaling, no zero-downtime deploys.
2. **`min_machines_running = 0`** — Your app can scale to zero, meaning cold starts. Combined with 7-day JWTs, this creates a UX problem but not a security issue per se. However, for production with paying customers, this is unacceptable availability.
3. **Shared CPU** — `cpu_kind = 'shared'` is explicitly warned against for production workloads in the Fly.io checklist.
4. **No health checks** — Fly cannot detect unhealthy instances without `[[checks]]` configuration.
5. **Volume encryption** — Need to verify `fly volumes list` shows `ENCRYPTED` for the `memos_data` volume.

---

## 4. Application Security (vs. prompt_sec.md / security.md)

### ✅ Good
- `MEMOS_MODE=prod` — Not demo mode
- `force_https = true` — TLS enforced
- **Domain allowlist** — `isDomainAllowed()` checks Origin/Referer for widget endpoints
- **Input validation** — `clientMessageIDPattern` regex, `ValidateExternalSessionID`, body size limits (1 MiB in bridge middleware)
- **Rate limiting** — `RateLimitRPM` from tenant config, admin mutation rate limiting
- **Encryption at rest** — `ENCRYPTION_MASTER_KEY` for tenant API keys (AES-GCM via `crypto.EncryptionService`)
- **HMAC bridge auth** — Strong HMAC-SHA256 with nonce replay protection, timestamp freshness (±5 min), body hash canonicalization

### ❌ Issues

1. **CVE-2026-6634 (Auth bypass)** — The security docs warn about an auth bypass in `UpdateInstanceSetting` via `memos_access_token`. This is an upstream Memos vulnerability. Your codebase inherits it. **No WAF/proxy rules** are configured to mitigate this.
2. **CVE GO-2025-3492 (SSRF)** — The `plugin/httpgetter` package exists in the codebase. If link-preview or webhook features are enabled, this is a risk. Need to verify `MEMOS_ALLOW_PRIVATE_WEBHOOKS` is `false`.
3. **CVE GO-2025-3936 (Path traversal)** — The `CreateResource` endpoint for attachment uploads. Need to verify file upload sanitization.
4. **`isDomainAllowed()` fail-open** — Lines 1882-1892: If `allowedDomainsJSON` is empty, invalid JSON, or empty array, the function returns `true` (allow all). This is documented as "no restrictions" but could be surprising if misconfigured.
5. **Widget script injection** — The `generateWidgetScript()` fallback (line 1661) injects `baseURL`, `tenantSlug`, `companyName` directly into JavaScript via string concatenation. While these are server-controlled values, the `companyName` comes from tenant configuration which could contain malicious input.
6. **No Content Security Policy (CSP)** headers observed.
7. **No rate limiting on external chat** — The `HandleChatExternal` endpoint has no rate limiting beyond what the LLM provider imposes. The security docs recommend Arcjet or similar.

---

## 5. Data Security & RAG Pipeline

### ✅ Good
- **LanceDB S3 support** — `Dockerfile.fly` configures Tigris S3 storage for LanceDB
- **Encryption service** — AES-GCM encryption for tenant API keys
- **`_FILE` suffix support** — `entrypoint.sh` supports Docker secrets via `_FILE` env vars

### ❌ Issues
1. **LanceDB local storage in production** — `Dockerfile.local.fly` uses local volume for LanceDB. If the volume is lost, all vector embeddings are lost. No backup strategy documented.
2. **No LanceDB backup strategy** — If using S3 (Tigris), need to verify bucket versioning. If using local volume, need snapshot procedures.
3. **Embedding data sensitivity** — LanceDB stores embeddings of customer KB/Policy content. If the LanceDB storage is compromised, an attacker could potentially reconstruct sensitive information from embeddings.
4. **No data encryption at rest for LanceDB** — If using local volume, encryption depends on Fly volume encryption. If using S3, need to verify bucket encryption.

---

## 6. Taskfile.yml Security Analysis

| Task | Security Implication |
|---|---|
| `run` | Sources `.env` file — dangerous if `.env` is committed to git |
| `fly:ssh:db` | Opens SQLite console on remote Fly instance — powerful debugging but needs access control |
| `fly:check` | Validates env chain — good practice |
| `fly:pre-deploy` | Runs pre-deployment checks — good practice |
| `build:backend:rag` | Uses CGO with LanceDB — introduces native library dependency that must be kept updated |

### ❌ Issues
1. **`.env` file sourcing** — Multiple run tasks source `.env` files. If `.env` is accidentally committed, secrets are exposed.
2. **No security scanning** — No `gosec` or similar security scanning in the build pipeline.
3. **No dependency vulnerability scanning** — No `go mod verify` or `npm audit` in the build pipeline.

---

## 7. Fly.io Production Readiness Scorecard

| Category | Score | Critical Blockers |
|---|---|---|
| **Security** | 5/10 | SQLite in prod, no WAF, 7-day tokens, no session revocation |
| **Databases** | 2/10 | SQLite single-writer, no Postgres, no backup DR tested |
| **App Performance** | 3/10 | Shared CPU, no swap, no sizing rationale |
| **Availability** | 1/10 | `min_machines_running=0`, single region, no health checks |
| **Networking** | 6/10 | `force_https=true`, but no custom domain, no dedicated IPv4 |
| **Monitoring** | 1/10 | No metrics, no Sentry, no log export |
| **CI/CD** | 1/10 | No GitHub Actions, no review apps |

**Overall: 2.7/10 — NOT READY FOR PRODUCTION**

---

## 8. Recommended Remediation Priority

### P0 (Must fix before any customer goes live)
1. **Migrate to Postgres** (Neon or Fly Managed Postgres) — SQLite is unacceptable for multi-tenant production
2. **Set `min_machines_running = 1`** — Ensure at least one instance is always running
3. **Use performance CPUs** — Change `cpu_kind = 'shared'` to `cpu_kind = 'performance'`
4. **Add health checks** — Configure `[[checks]]` in `fly.toml`
5. **Reduce JWT token lifetime** — Change from 7 days to 15 minutes with refresh token rotation
6. **Implement session revocation** — Invalidate tokens on password change
7. **Add WAF/proxy rules** — Mitigate CVE-2026-6634 (auth bypass) at the proxy layer

### P1 (Fix before scaling beyond first customer)
8. **Set up Fly org SSO** — Google/GitHub SSO for infrastructure access
9. **Isolate staging/production** — Separate Fly organizations
10. **Add rate limiting to all admin mutation endpoints** — Not just role-template endpoints
11. **Configure monitoring** — Prometheus/Grafana, Sentry, log shipper
12. **Set up CI/CD** — GitHub Actions with security scanning
13. **Implement LanceDB backup strategy** — S3 versioning or volume snapshots

### P2 (Fix for mature production readiness)
14. **Multi-region deployment** — Add additional regions
15. **Dedicated IPv4** — Avoid shared IP reputation issues
16. **Review apps** — Ephemeral preview environments per PR
17. **CSP headers** — Content Security Policy
18. **Dependency scanning** — `gosec`, `npm audit` in CI
19. **Penetration testing** — Before public launch

---

## Summary

Your **custom RBAC implementation** (role templates, resolved permissions, admin mutation rate limiting) is well-designed and goes significantly beyond what vanilla Memos provides. The **bridge HMAC middleware** is particularly strong with nonce replay protection, timestamp freshness, and body canonicalization.

However, the **infrastructure layer** is not production-ready. The critical blockers are: **SQLite in production**, **zero redundancy** (`min_machines_running=0`), **shared CPUs**, **7-day JWTs with no revocation**, and **no monitoring/backup strategy**. The upstream Memos CVEs (especially the auth bypass CVE-2026-6634) are unmitigated at the WAF/proxy layer.

The `plan_sec.md` RBAC hardening has been **substantially implemented** (I estimate ~85% complete), but the deployment configuration and infrastructure security lag significantly behind.