Now I have enough context to deliver the security review.

---

# Security Review: bchat vs `/home/chaschel/Desktop/bchat/biz/prompt_sec.md`

## Executive Summary

The bchat codebase is a multi-tenant AI chat platform built on a fork of usememos/memos with LanceDB RAG. Against the production-hardening prompt, it currently satisfies only a small fraction of the required controls. There are **critical** secret-management and data-tier issues, **high** severity gaps in CVE mitigation, multi-tenancy isolation, CI/CD, and access control, plus several **medium/low** gaps in Fly.io infrastructure hardening.

---

## 1. CVE Mitigation & Version Pinning

| Finding | Severity | Evidence |
|---------|----------|----------|
| Image tag is not pinned by digest | **High** | `fly.toml` and `Dockerfile*.fly` use `Dockerfile.local.fly` / `Dockerfile.fly` built from source; no pinned base image digest or memos version tag is recorded anywhere. |
| CVE-2026-6634 (`UpdateInstanceSetting` auth bypass) partially addressed at app layer | **Medium** | `SetWorkspaceSetting` (`workspace_setting_service.go:68`) restricts writes to `RoleHost`. However `GetWorkspaceSetting` for `GENERAL` (which exposes `AdditionalScript`/`AdditionalStyle`) is readable by any authenticated user (`authenticationAllowlistMethods` + no additional read guard in `GetWorkspaceSetting`). No WAF/nginx rules exist to block `/api/v1/*instance*` for non-admins at the proxy layer. |
| CVE-2025-4127 — token invalidation on password change | **High** | `UpdateUser` (`user_service.go:209`) hashes the new password but does **not** call `DeleteUserAccessToken` or otherwise revoke existing access tokens. Stored tokens remain valid after password change. |
| SSRF (`plugin/httpgetter`) for link preview | **Medium** | `GetLinkMetadata` (`markdown_service.go:45`) calls `httpgetter.GetHTMLMeta` without auth restriction. The `httpgetter` plugin does block loopback/private IPs (`html_meta.go:134-152`), but internal hostnames, IPv6 link-local, and DNS rebinding are not fully mitigated; no allowlist is enforced. |
| Path traversal in resource upload | **Medium** | `SaveResourceBlob` uses `filepath.FromSlash` on `profile.Data` + user-controlled template but does not sanitize `..` sequences in `internalPath`. An attacker-controlled `FilepathTemplate` could escape the data directory. |
| Stored XSS via `AdditionalScript`/`AdditionalStyle` | **Medium** | `workspace_setting_service.go:133-136` returns raw `AdditionalScript`/`AdditionalStyle` to the client. These are rendered in the frontend without sanitization (standard Memos behavior). Any authenticated user can read them; host-only write is enforced, but a compromised host account injects persistent XSS. |

---

## 2. Database Migration: Neon Postgres

| Finding | Severity | Evidence |
|---------|----------|----------|
| SQLite is the primary production datastore | **Critical** | `fly.toml` mounts `memos_data` to `/var/opt/memos`; `MEMOS_DRIVER` is not set to `postgres`; no `MEMOS_DSN` is configured. Prompt explicitly forbids SQLite in production. |
| No Neon migration script exists | **Critical** | No `migrate-to-neon.sh` or equivalent in the repository. |
| No PgBouncer DSN / `sslmode=require` | **Critical** | No `MEMOS_DSN` configured anywhere; `MEMOS_MODE=prod` is set but Postgres driver is not activated. |
| Postgres driver code exists but is unused in prod | **Info** | `store/db/postgres/` exists, indicating fork capacity, but Fly deploy uses SQLite. |

---

## 3. LanceDB Persistence & Security

| Finding | Severity | Evidence |
|---------|----------|----------|
| LanceDB local-only storage on Fly volume | **High** | `fly.toml` sets `LANCEDB_STORAGE_PROVIDER=local`. No S3/Tigris configuration. `Dockerfile.fly` has S3 env vars commented out; `Dockerfile.local.fly` hardcodes local. |
| No S3 backup / versioning | **High** | No Tigris/AWS S3 setup, no snapshot procedure documented. Fly volume snapshots (5-day retention) are the only implicit backup — prompt explicitly says this is insufficient. |
| Volume encryption not verified | **Medium** | `fly.toml` uses a volume but no `fly volumes list` verification is documented; encryption-at-rest for Fly volumes is default but not explicitly confirmed. |
| No least-privilege access keys for object storage | **Medium** | No S3 keys exist at all in the current config. |

---

## 4. Memos Application Security

| Finding | Severity | Evidence |
|---------|----------|----------|
| Demo mode correctly disabled | **Pass** | `MEMOS_MODE=prod` is set in both `fly.toml` files. |
| Host secret key rotation on first boot | **Pass** | `server.go:218` auto-generates `SecretKey` on first boot if empty. |
| Open registration / public visibility not explicitly disabled | **Medium** | No `DisallowUserRegistration` or `DisallowPublicVisibility` settings are set in `fly.toml` `[env]`. These are workspace-level DB settings that should be bootstrapped. |
| SSO callback URL not enforced | **Low** | `idp_service.go` stores OAuth configs but does not validate callback URLs against `MEMOS_INSTANCE_URL`. |
| `MEMOS_ALLOW_PRIVATE_WEBHOOKS` not set | **Low** | Not present in environment; defaults to base Memos behavior. |
| Access token duration is 7 days, not 15 min | **Info** | `auth.go:18` sets `AccessTokenDuration = 7 * 24 * time.Hour`. Prompt assumes 15-min access + 30-day refresh. Long-lived JWTs increase blast radius. |

---

## 5. Fly.io Organization & Access Security

| Finding | Severity | Evidence |
|---------|----------|----------|
| No Fly org SSO documented or configured | **High** | Prompt requires SSO for the Fly organization itself; repo contains no IaC or docs for this. |
| No staging/production org separation | **High** | Only one app (`bchat`) exists; no separate Fly org or staging app is defined in repo. |
| Secrets partially in `[env]` | **High** | `fly.toml` contains `OPENROUTER_API_KEY`-equivalent usage indirectly via envs that imply secret values, but `.env` with the actual key is committed (see next row). |
| `.env` with real secrets committed to git | **Critical** | `.env` contains `OPENROUTER_API_KEY=REDACTED_KEY...` and `ENCRYPTION_MASTER_KEY=e2590f42-...`. `.gitignore` has `.env`, but the file is already tracked. |
| No `fly secrets` manifest (`SECRETS.md`) | **High** | Prompt requires a complete `fly secrets set` manifest. No such file exists. |
| Public IPs not audited | **Medium** | No documentation of `fly ips list` / removing unnecessary public IPs. |
| No Arcjet / rate-limiting on public chat | **Medium** | `ChatExternal` has per-tenant in-memory rate limiting (`CheckRateLimit`), but no edge layer rate limiting or bot protection. |

---

## 6. Fly.io Infrastructure Hardening

| Finding | Severity | Evidence |
|---------|----------|----------|
| `force_https = true` | **Pass** | Present in `fly.toml:34`, `fly_prod.toml:39`. |
| `auto_stop_machines = true`, `min_machines_running = 0` | **High** | `fly.toml:35-37`. Zero min machines contradicts availability requirements and risks cold-start interactivity with long-lived tokens. |
| Shared CPU, 1 vCPU / 1024MB | **High** | `fly.toml:45-49`. Prompt requires performance CPUs. |
| No `swap_size_mb` | **Medium** | LanceDB + Memos can spike memory; swap is not configured. |
| No health checks defined | **Medium** | Only app-level `/healthz` exists; Fly `[http_service]` has no `health_check` config. |
| No multi-region / multi-machine config | **High** | Single region `sjc`, single machine implied. Prompt requires ≥2 machines in primary region. |
| No dedicated IPv4 documented | **Low** | Not required, but not documented. |
| `cpu_kind = "shared"` | **High** | Must be `performance` per prompt. |

---

## 7. Architectural Decision: Single-Tenant Isolation

| Finding | Severity | Evidence |
|---------|----------|----------|
| Application-level multi-tenancy only; shared Memos instance | **Critical** | `AgentTenant` rows in SQLite are the only tenant boundary. `store/agent.go` shows all tenant data co-located. Prompt requires Option A (per-customer Fly app + Neon branch + isolated storage) for mutually-untrusting customers. |
| `ListUsers` leaks all users globally | **High** | `user_service.go:38` returns all users without tenant scoping. |
| `GetUser` is in the public auth allowlist | **Medium** | `acl_config.go:14` allows unauthenticated `GetUser`; combined with `ListUsers`, this is a user enumeration oracle. |
| No per-tenant Fly app provisioning automation | **High** | No `fly.customer.toml` template or Terraform module exists. |

---

## 8. Monitoring, Error Tracking & Logging

| Finding | Severity | Evidence |
|---------|----------|----------|
| No Sentry integration | **High** | Prompt requires Sentry; no `SENTRY_DSN` or SDK initialization found. |
| No Prometheus / Grafana | **High** | No managed Prometheus config or `/metrics` endpoint found. |
| No log shipper / log export | **Medium** | Only local `slog` output; no Fly Log Shipper or external aggregation. |
| No disaster recovery runbook | **High** | No `DEPLOYMENT.md` or runbook for Neon restore, LanceDB rebuild, or JWT secret rotation. |

---

## 9. CI/CD & Review Apps

| Finding | Severity | Evidence |
|---------|----------|----------|
| No CI/CD pipelines | **High** | No `.github/workflows/` directory found. |
| No review apps | **High** | Prompt requires ephemeral review apps per PR. |
| No branch protection / automated deploy | **High** | No GitHub Actions or equivalent automation. |

---

## 10. Deliverables Status

| Deliverable | Status |
|-------------|--------|
| `fly.toml` (production-hardened) | **Partial** — `force_https` and mounts present, but CPU, swap, health checks, multi-region missing |
| `fly.staging.toml` | **Missing** |
| `fly.customer.toml` / Terraform | **Missing** |
| `SECRETS.md` | **Missing** |
| `migrate-to-neon.sh` | **Missing** |
| `waf-rules.conf` / nginx sidecar | **Missing** |
| `.github/workflows/` | **Missing** |
| `DEPLOYMENT.md` | **Missing** |
| `SECURITY_CHECKLIST.md` | **Missing** |

---

## Immediate Recommendations (Top 5)

1. **Rotate and remove committed secrets.** The `.env` file with a live OpenRouter key and encryption master key is tracked in git. Rotate both immediately, purge from history with `git filter-repo`, and add `.env` to `.gitignore` enforcement.
2. **Do not deploy current `fly.toml` to production.** It uses SQLite on a local volume, shared CPU, zero-minimum machines, and no Postgres. Migrate to Neon (`MEMOS_DRIVER=postgres`, PgBouncer DSN, `sslmode=require`) before serving real customers.
3. **Implement infrastructure-level tenant isolation (Option A).** One Fly app + Neon branch + Tigris bucket per customer. Application-level row isolation is explicitly deemed insufficient for paying customers in the prompt.
4. **Add WAF/proxy rules for `/api/v1/*instance*` and admin routes.** `SetWorkspaceSetting` is host-only, but `GetWorkspaceSetting` for `GENERAL` exposes `AdditionalScript`/`AdditionalStyle` to any authenticated user. A proxy rule blocking non-admin access to instance-settings routes mitigates CVE-2026-6634 at the edge.
5. **Revoke all access tokens on password change.** `UpdateUser` changes the password hash but leaves stored access tokens intact. Delete or invalidate `ACCESS_TOKENS` on password update.

---

## Additional Hardening Checklist

- [ ] Pin Docker image digest and record version.
- [ ] Set `cpu_kind = "performance"`.
- [ ] Add `swap_size_mb` ≥ 512.
- [ ] Set `min_machines_running ≥ 1` per region.
- [ ] Configure `[http_service.health_check]` with adequate grace periods.
- [ ] Move `OPENROUTER_API_KEY`, `ENCRYPTION_MASTER_KEY`, and any future S3 keys to `fly secrets`.
- [ ] Add Sentry, Log Shipper, and managed Prometheus.
- [ ] Create staging Fly org + `fly.staging.toml`.
- [ ] Build GitHub Actions for review apps + production deploy with branch protection.
- [ ] Document DR runbook for Neon PITR, LanceDB reindex, and JWT secret rotation.
- [ ] Restrict CORS origins to the actual frontend domain instead of `*`.
- [ ] Sanitize `filepathTemplate` in `SaveResourceBlob` to prevent path traversal.
- [ ] Reduce `AccessTokenDuration` to ≤ 15 minutes and implement refresh tokens (currently 7 days).
