Based on current CVE data and Fly.io's official docs, here's what needs addressing, split into app-level vulnerabilities, config hardening, and Fly-specific infrastructure concerns.

## 1. Known vulnerabilities — pin a patched version first

Memos has had a steady stream of real CVEs, several of which are directly relevant to a customer-facing deploy:

- **CVE-2026-6634 (unpatched as of the last advisory)** — an auth bypass in `UpdateInstanceSetting` (via `memos_access_token` handling in `App.tsx`) lets an authenticated but under-privileged user manipulate `additionalStyle`/`additionalScript` instance settings beyond their role. That's a stored-XSS-via-admin-settings vector. No official patch existed at time of disclosure and the vendor reportedly didn't respond to the report — treat this as currently live risk and mitigate at the network/proxy layer (see below) rather than assuming it's fixed.
- **GO-2025-3937** — stored XSS in Memos.
- **GO-2025-3492** — SSRF (relevant if you use link-preview/webhook/httpgetter features — Memos has a `plugin/httpgetter` package that fetches arbitrary URLs server-side).
- **GO-2025-3936** — path traversal via the `CreateResource` endpoint (attachment upload). Directly matters since you're presumably accepting user-uploaded files.
- **GO-2025-4127** — access tokens remain valid after a password change, which undermines your incident-response story ("user's account was compromised, they changed their password" doesn't actually revoke sessions).
- **CVE-2022-4685** — older improper access control on memo data (fixed in 0.9.0, just don't run anything that old).

Action: check exactly which tag you're deploying (`neosmemo/memos:stable` moves), diff it against the GitHub Security Advisories list, and don't assume "stable" means "patched." Given the auth-bypass and SSRF issues, I'd put a WAF/reverse-proxy rule in front of `/api/v1/*instance*` endpoints and admin-settings routes as defense-in-depth even after upgrading.

## 2. Memos application config (per their own security doc)

- **Never run demo mode in production** — it uses a hardcoded JWT secret (`usememos`), which means anyone can forge valid access tokens. Confirm your `MEMOS_MODE` env var is `prod`, not `demo`.
- Set `MEMOS_INSTANCE_URL` correctly if you're behind Fly's proxy — otherwise redirect/cookie logic can break in ways that leak tokens or misbehave with `SameSite`.
- **Disable open registration** unless you genuinely want self-serve signups; otherwise anyone can create an account on what should be your customers' instance.
- **Disable public memo visibility** by default unless customers explicitly want public sharing — public memos expose body content, attachments, and embedded links to anyone with the link, unauthenticated.
- Audit who can mint personal access tokens (`memos_pat_` prefixed) and rotate/expire old ones — PATs are long-lived by design.
- Lock down the admin/host account specifically; it's the account the settings-bypass CVE above targets.

## 3. Fly.io infrastructure specifics

- **Secrets vs `[env]`**: `fly.toml`'s `[env]` table gets committed to git and lives in history forever. Any DB credentials, S3/Litestream keys, SMTP passwords, or OAuth client secrets must go through `fly secrets set`, never in `[env]`.
- **Volume encryption**: Fly volumes are encrypted at rest by default (LUKS/XTS) as of current flyctl — verify with `fly volumes list` (`ENCRYPTED` column) if you provisioned the volume with an older script; some older third-party memos-on-fly guides predate this default.
- **SQLite + single volume = single point of failure and no horizontal scaling.** A volume attaches to exactly one Machine, so you're capped at one instance for the app process. For real paying customers this means: no zero-downtime multi-region failover, and every deploy is a rolling restart of your only writer. If you need actual availability guarantees, migrate to Postgres (Memos supports it) and use Fly Postgres or an external managed Postgres over the private network — this also sidesteps SQLite's single-writer lock contention under concurrent customer load.
- **Backups aren't optional infrastructure, they're your data-loss plan.** Fly's daily volume snapshots (5-day retention default) are not a substitute for real backups. The common community pattern (Litestream → B2/S3) works for SQLite, but verify restore actually works before go-live, not after an incident — and make sure the Litestream S3/B2 credentials are `fly secrets`, not `[env]`.
- **Network exposure**: check `[[services]]`/`[http_service]` in `fly.toml` — only the app's HTTP port should be publicly routed (`internal_port` correct, no stray `allowed_public_ports`). If you add Postgres, Redis, or an internal admin surface, put it on the private 6PN network (`.internal` DNS) rather than a public IP.
- **`force_https = true`** and confirm `fly certs` shows a valid, complete chain for your custom domain — don't rely on the bare `.fly.dev` cert if customers hit a branded domain.
- **`min_machines_running` / `auto_stop_machines`**: cost-saving cold-start behavior can interact badly with session/token expectations (15-min access tokens) if a Machine spins down and back up mid-session — worth testing under realistic idle patterns before customers hit it.
- **Org membership**: anyone with member access can read secrets and `fly ssh console` into the Machine. Treat your Fly org member list like a production credentials list — review it, remove departed collaborators same-day.

## 4. The architectural question you should settle before "real customers"

Memos is fundamentally a **single-tenant, single-workspace app** — one instance, one shared visibility/permission model, no built-in tenant isolation. If "real customers" means multiple unrelated parties, there are two honest options:
1. **One Fly app + volume per customer** (isolation by infrastructure, more ops overhead, but avoids relying on Memos' internal ACLs to separate tenants it was never designed to separate), or
2. **One shared instance** — acceptable only if all users are meant to see each other's public memos/workspace (e.g., an internal team tool), not for mutually-untrusting customers.

Given the recent auth-bypass CVE lets a lower-privileged authenticated user reach instance-wide settings, option 2 with unrelated customers sharing one instance is the higher-risk path right now.

If it'd help, I can sketch a hardened `fly.toml` (secrets-only, private-network Postgres, `force_https`, no `[env]` secrets) for a Postgres-backed multi-instance-per-customer setup.
