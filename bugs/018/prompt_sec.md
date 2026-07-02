Here is the fully revised prompt, now incorporating both the CVE/security hardening document and Fly.io's official **Going to Production Checklist**:

---

## Prompt: Securely Harden Memos + Neon Postgres + LanceDB on Fly.io (Production-Ready)

**Context:** I have a modified fork of [usememos/memos](https://github.com/usememos/memos) deployed on Fly.io. My fork adds **LanceDB** for RAG (Retrieval-Augmented Generation) vector search. I am migrating from SQLite to **Neon Postgres** and need to harden the entire stack for production use with **real, paying customers**.

**Critical Security Context:** Memos has recent, unpatched CVEs (including an auth bypass in `UpdateInstanceSetting`). The app is fundamentally **single-tenant** with no built-in tenant isolation. The architecture must assume mutually-untrusting customers require infrastructure-level isolation.

**Goal:** Produce a secure, production-ready deployment package (`fly.toml`, migration scripts, secrets manifest, WAF rules, CI/CD pipeline, and hardening checklist) that mitigates all known CVEs, eliminates single-points-of-failure, and enforces defense-in-depth per Fly.io's official production checklist.

---

### 1. CVE Mitigation & Version Pinning (Mandatory)

Do **not** use `neosmemo/memos:stable` (it moves). Pin an exact, verified patched tag.

**Known CVEs to verify are patched in your chosen tag:**
- **CVE-2026-6634** (unpatched as of last advisory) — Auth bypass in `UpdateInstanceSetting` via `memos_access_token` handling. Authenticated under-privileged users can manipulate `additionalStyle`/`additionalScript` (stored XSS vector). **Mitigate at the proxy/WAF layer** since no official patch may exist.
- **GO-2025-3937** — Stored XSS.
- **GO-2025-3492** — SSRF via `plugin/httpgetter` (link-preview/webhook features fetch arbitrary URLs server-side).
- **GO-2025-3936** — Path traversal via `CreateResource` (attachment uploads).
- **GO-2025-4127** — Access tokens remain valid after password change (session revocation failure).
- **CVE-2022-4685** — Improper access control (fixed in 0.9.0+).

**Required mitigations:**
- Pin an exact image digest (e.g., `neosmemo/memos@sha256:...`) after diffing against GitHub Security Advisories.
- Implement **WAF/reverse-proxy rules** at the Fly edge or via an nginx sidecar to block/restrict access to:
  - `/api/v1/*instance*` endpoints
  - Admin-settings routes
  - Any route modifying `additionalStyle` or `additionalScript` for non-admin roles
- Disable link-preview/webhook HTTP fetching if not required, or restrict `plugin/httpgetter` to a strict allowlist of domains.
- Implement **session/token revocation logic** — since Memos doesn't invalidate tokens on password change, document the manual revocation procedure (delete refresh tokens from DB, rotate JWT secret in emergency).

---

### 2. Database Migration: Neon Postgres (Mandatory for Production)

SQLite is unacceptable for real customers (single-writer, single-point-of-failure, no horizontal scaling, no zero-downtime deploys).

**Neon configuration:**
- Set `MEMOS_DRIVER=postgres`
- DSN must use **PgBouncer pooled connection string** (port `5433`) to avoid connection exhaustion under load
- **Force `sslmode=require`** — never `disable`
- Store `MEMOS_DSN` in `fly secrets`, never in `[env]`
- Provide a migration script (`migrate-to-neon.sh`) that exports SQLite and imports to Neon
- Use Neon branches: `main` for production, separate branch for staging/preview
- Set `MEMOS_INSTANCE_URL=https://your-custom-domain.com` (required for correct redirects/cookie `SameSite` behavior)

**Note:** Fly.io officially recommends their own **Managed Postgres** for production. Since I'm using Neon, document the trade-offs (connection pooling limits, external network dependency) and ensure the DSN uses PgBouncer.

---

### 3. LanceDB Persistence & Security

LanceDB stores vector embeddings of customer memos (potentially sensitive).

**Storage decision:**
- **Primary recommendation:** Use **S3-compatible object storage** (Fly Tigris, AWS S3, or Cloudflare R2) with private buckets. This avoids single-machine lock-in.
- If local storage is required for performance, use a **dedicated Fly Volume** with `auto_stop_machines = false` (if LanceDB is in-memory or requires continuous access).
- **Encryption:** Ensure S3 buckets are private, encrypted at rest, with no public access policies.
- **Backup:** Enable S3 versioning. If using volumes, document snapshot procedures.
- **Access keys:** Use dedicated, least-privilege keys stored in `fly secrets` (never `[env]`).

---

### 4. Memos Application Security (Per Official Security Docs + CVE Mitigation)

- **Never run demo mode:** Confirm `MEMOS_MODE=prod` (not `demo`). Demo mode uses hardcoded JWT secret `usememos` — anyone can forge tokens.
- **Disable open registration** after initial admin creation unless self-serve is explicitly required.
- **Disable public memo visibility** by default. Public memos expose body content, attachments, and embedded links unauthenticated.
- **Lock down admin/host account** — this is the primary target for the CVE-2026-6634 auth bypass.
- **Audit Personal Access Tokens (PATs):** Restrict who can mint `memos_pat_*` tokens. Rotate old ones. PATs are long-lived and stored as SHA-256 hashes.
- **SSO readiness:** If using OAuth, ensure callback URL matches `MEMOS_INSTANCE_URL`.
- **Webhook security:** Keep `MEMOS_ALLOW_PRIVATE_WEBHOOKS=false` unless explicitly required.
- **Session hygiene:** Document that access tokens are 15-minute JWTs and refresh tokens are 30-day HTTP-only cookies. Plan for `auto_stop_machines` potentially causing mid-session cold starts.

---

### 5. Fly.io Organization & Access Security (Per Official Checklist)

- **Enable SSO for the Fly organization:** Use Google or GitHub SSO for the Fly.io organization itself to secure infrastructure access.
- **Isolate staging and production:** Use separate Fly.io organizations for staging and production. Never share orgs between environments. Document the org structure.
- **Enforce least privilege:** Use Fly access tokens (not personal tokens) with the minimum required scope. Rotate tokens regularly. Review org member access same-day when collaborators depart.
- **Secrets management:** All DB credentials, S3/Litestream keys, SMTP passwords, OAuth secrets, `BOT_TOKEN`, and LanceDB access keys must use `fly secrets set`. Never commit secrets to `fly.toml` `[env]` or git history. Verify secrets sync: `fly secrets list`.
- **Private services:** Ensure internal services (Postgres, admin interfaces, metrics) have no public IPs. Run `fly ips list` and release unnecessary public IPs. Use **Flycast** (private 6PN addresses) for internal app-to-app communication.
- **Arcjet (if applicable):** If the frontend is JavaScript-heavy or customer-facing, consider Arcjet for rate limiting, bot protection, and email validation.

---

### 6. Fly.io Infrastructure Hardening

**Networking:**
- `force_https = true` is mandatory in `fly.toml`.
- Verify `fly certs` shows a valid, complete chain for custom domains.
- Confirm `[[services]]`/`[http_service]` only exposes the app's HTTP port. No stray `allowed_public_ports`.
- Use `.internal` hostnames for any internal services.

**Machine Sizing & Performance:**
- Use **performance CPUs** (not shared) for production workloads.
- Ensure sufficient RAM for Memos + LanceDB. If brief memory spikes are expected, enable **swap to disk** via `swap_size_mb` in `fly.toml`.
- Document the resource requirements and sizing rationale.

**Volume & Storage:**
- If using volumes for LanceDB or legacy SQLite migration, verify encryption at rest: `fly volumes list` must show `ENCRYPTED`.
- **Do not rely on Fly's daily volume snapshots** (5-day retention) as your sole backup. They are not a substitute for real backups.

**Resiliency & Scaling:**
- **Use multiple Machines** for resiliency against single-host failures. Configure at least 2 Machines in the primary region, with additional Machines in other regions if customers are geo-distributed.
- **Scale into multiple regions** closest to your users. Document the region strategy.
- `auto_stop_machines = true` / `auto_start_machines = true` for cost savings, **but**:
  - Set `min_machines_running = 1` (or higher if multi-region) to ensure availability.
  - Test token/session behavior under realistic idle patterns — cold starts may interact badly with 15-minute access tokens.
  - If LanceDB requires warm vector indices or volume persistence, `auto_stop_machines` may need to be `false` for that component.
- **Autoscale by metric** (optional): For non-HTTP workloads or background processing, configure the Fly autoscaler to scale based on custom metrics (e.g., queue depth, CPU).

**Dedicated IPv4 (Optional):**
- Consider a **dedicated IPv4 address** to eliminate risks from shared IP blacklisting (e.g., spammers affecting your reputation). Document the cost trade-off.

---

### 7. Architectural Decision: Single-Tenant Isolation

Memos is **fundamentally single-tenant** with no built-in tenant isolation. Given CVE-2026-6634 allows authenticated users to reach instance-wide settings, **do not put mutually-untrusting customers on one shared instance**.

**Choose one architecture:**

**Option A: One Fly App per Customer (Recommended)**
- Each customer gets their own Fly app, Neon database branch, and LanceDB storage.
- Isolation is by infrastructure. Higher ops overhead but eliminates cross-tenant data leakage risk.
- Provide a Terraform script or `flyctl` automation to provision new customer stacks.

**Option B: Single Shared Instance (Acceptable Only for Internal Teams)**
- Only use this if all users are trusted colleagues who are meant to see each other's public memos.
- **Not suitable for unrelated paying customers.**

The deliverables must support **Option A** as the primary path, with clear instructions for per-customer provisioning.

---

### 8. Monitoring, Error Tracking & Logging (Per Official Checklist)

- **Metrics:** Set up Fly's **managed Prometheus and Grafana** dashboards to monitor app health, DB connections, and LanceDB performance.
- **Error Tracking:** Integrate **Sentry** (Fly.io organizations get a year's worth of Sentry Team Plan credits). Configure it to capture backend panics and frontend errors.
- **Log Export:** Set up the **Fly Log Shipper** to aggregate logs to an external service (e.g., Datadog, Better Stack, Loki, or S3). Do not rely solely on Fly's ephemeral log storage.
- **Neon:** Automatic backups are provided, but document the point-in-time recovery procedure.
- **LanceDB:** If using S3, enable versioning and document restore. If using volumes, document `fly volumes snapshots`.
- **Litestream (if still used for any SQLite component):** Stream WAL to B2/S3. Credentials must be `fly secrets`, not `[env]`. Verify restore works before go-live.
- **Disaster recovery runbook:** Include steps for:
  - Restoring Neon from backup/branch
  - Rebuilding LanceDB embeddings from source memos if vector data is lost
  - Emergency JWT secret rotation (to invalidate all sessions if breach suspected)

---

### 9. CI/CD & Review Apps (Per Official Checklist)

- **Review Apps:** Configure GitHub Actions to generate ephemeral **review apps** on Fly.io for each pull request. This isolates testing from production.
- **Continuous Deployment:** Set up GitHub Actions for automated deployment to Fly.io from the repository. Use branch protection rules to prevent direct pushes to `main`.
- **Deployment Safety:** Require CI passes (tests, security scans) before deploy. Use blue-green or canary deployment patterns if possible.

---

### 10. Deliverables

Produce the following as a cohesive package:

1. **`fly.toml`** — Production-hardened, comments explaining every security choice. No secrets in `[env]`. Includes `swap_size_mb`, performance CPU, health checks, and multi-region scaling config.
2. **`fly.staging.toml`** — For Neon dev branch / preview deployments in an isolated Fly org.
3. **`fly.customer.toml`** (or Terraform module) — Template for per-customer provisioning (Option A).
4. **`SECRETS.md`** — Complete manifest of every `fly secrets set` command, with descriptions and security rationale.
5. **`migrate-to-neon.sh`** — SQLite → Neon migration script with rollback plan.
6. **`waf-rules.conf`** or **nginx-sidecar config** — Rules restricting `/api/v1/*instance*` and admin routes.
7. **`.github/workflows/`** — CI/CD pipeline for review apps + production deployment.
8. **`DEPLOYMENT.md`** — Step-by-step guide:
   - Fly org setup with SSO
   - Staging/production isolation
   - Neon provisioning (main + branch)
   - Volume creation (if needed for LanceDB)
   - Secret injection
   - Domain & TLS setup
   - Customer provisioning workflow (Option A)
   - Machine sizing and swap configuration
9. **`SECURITY_CHECKLIST.md`** — Pre-launch checklist covering:
   - All CVE mitigations
   - All Fly.io official production checklist items (security, databases, performance, availability, networking, monitoring, CI/CD)
   - Per-customer isolation verification

---

### 11. Constraints & Assumptions

- **Do not use SQLite in production.**
- **Do not commit secrets to the repository.**
- **Do not enable demo mode.**
- Assume Go/GORM backend with Memos v0.22.x+.
- LanceDB may use Go client or S3-backed storage.
- Target Fly.io Machines (not Kubernetes).
- All files must include inline comments explaining the security rationale for critical choices.
- The final deliverable must map directly to Fly.io's official production checklist categories: Security, Databases, App Performance, Availability/Resiliency, Networking, Monitoring, and CI/CD.
