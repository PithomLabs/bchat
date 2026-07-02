# Implementation Plan: Secure Production Deployment

> **Status:** Draft - Ready for Review and Iteration

---

## Executive Summary

This plan addresses deploying bchat (Memos fork) to production on Fly.io with:
- CVE mitigation for unpatched vulnerabilities
- Neon Postgres migration (replacing SQLite)
- Secure LanceDB vector storage
- Infrastructure-level tenant isolation
- Full compliance with Fly.io's production checklist

**Recommended Architecture:** **Option A - One Fly App per Customer** for maximum isolation.

---

## Phase 1: Foundation & CVE Mitigation

### 1.1 Version Pinning & CVE Audit

**Tasks:**
- [ ] Audit Memos tags for patched CVEs: CVE-2026-6634, GO-2025-3937, GO-2025-3492, GO-2025-3936, GO-2025-4127
- [ ] Select exact patched commit/tag
- [ ] Pin Docker image to exact digest: `neosmemo/memos@sha256:...`
- [ ] Document any unpatched CVEs requiring WAF mitigation

**Unpatched CVE Mitigation:**
| CVE | Risk | WAF Rule Needed |
|-----|------|-----------------|
| CVE-2026-6634 | Auth bypass in UpdateInstanceSetting | Block `/api/v1/*instance*` for non-admin |
| GO-2025-3492 | SSRF via httpgetter plugin | Block link-preview for external URLs |
| GO-2025-3936 | Path traversal via CreateResource | Restrict attachment uploads |

### 1.2 WAF / Reverse Proxy Setup

**Tasks:**
- [ ] Create nginx sidecar container configuration
- [ ] Add rules to block:
  - `/api/v1/*instance*` endpoints (except admin)
  - Admin settings modification for non-admin roles
  - Link preview for non-whitelisted domains
- [ ] Deploy WAF rules via Fly edge or sidecar

---

## Phase 2: Neon Postgres Migration

### 2.1 Neon Configuration

**Tasks:**
- [ ] Create Neon account and project
- [ ] Provision `main` branch for production
- [ ] Create separate branch for staging
- [ ] Generate PgBouncer connection string (port 5433)
- [ ] Configure secrets: `MEMOS_DSN`, `MEMOS_DRIVER=postgres`

### 2.2 Migration Script

**Tasks:**
- [ ] Create `migrate-to-neon.sh`:
  - Export SQLite data
  - Transform schema for Postgres compatibility
  - Import to Neon with validation
  - Include rollback procedure
- [ ] Test migration in staging environment

---

## Phase 3: LanceDB Security & Storage

### 3.1 Storage Decision

**Selected:** Fly Tigris object storage for multi-machine accessibility.

**Tasks:**
- [ ] Create private bucket for vector embeddings
- [ ] Enable bucket encryption at rest
- [ ] Enable versioning for backup
- [ ] Configure secrets: `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`

### 3.2 Alternative: Fly Volume (if required)

- [ ] Dedicated volume with encryption
- [ ] Consider `auto_stop_machines = false` if persistence needed

---

## Phase 4: Application Security Hardening

### 4.1 Memos Configuration

**Tasks:**
- [ ] Set `MEMOS_MODE=prod` (never demo)
- [ ] Disable open registration after admin creation
- [ ] Set private memos as default
- [ ] Lock down admin account with 2FA
- [ ] Rotate all PAT tokens
- [ ] Configure OAuth callback URLs
- [ ] Set `MEMOS_ALLOW_PRIVATE_WEBHOOKS=false`

### 4.2 Session Management

- [ ] Document token revocation procedure
- [ ] Implement emergency JWT secret rotation script
- [ ] Plan for cold start handling with access tokens

---

## Phase 5: Fly.io Infrastructure Hardening

### 5.1 Organization Security

**Tasks:**
- [ ] Enable SSO for Fly organization
- [ ] Create separate staging/production orgs
- [ ] Create least-privilege Fly access tokens
- [ ] Remove unused public IPs
- [ ] Use `.internal` for internal service communication

### 5.2 Machine Configuration

**Tasks:**
- [ ] Use performance CPUs for production
- [ ] Add `swap_size_mb` for memory spikes
- [ ] Set `force_https = true`
- [ ] Configure multi-region deployment
- [ ] Set `min_machines_running = 2` in primary region
- [ ] Consider dedicated IPv4 for reputation isolation

---

## Phase 6: Monitoring & Logging

### 6.1 Observability Stack

**Tasks:**
- [ ] Enable Fly managed Prometheus + Grafana
- [ ] Integrate Sentry for error tracking
- [ ] Set up Fly Log Shipper to external service
- [ ] Create Neon backup/restore runbook
- [ ] Document LanceDB rebuild procedure

---

## Phase 7: CI/CD Pipeline

### 7.1 GitHub Actions

**Tasks:**
- [ ] Create review apps workflow (per PR)
- [ ] Create production deployment workflow
- [ ] Add security scan steps
- [ ] Configure branch protection rules
- [ ] Set up blue-green/canary deployment

---

## Phase 8: Customer Provisioning (Option A)

### 8.1 Infrastructure-as-Code

**Tasks:**
- [ ] Create Terraform module for per-customer stack
- [ ] Or create `fly.customer.toml` template
- [ ] Automate:
  - Fly app creation
  - Neon branch provisioning
  - LanceDB storage setup
  - Secret injection
  - Domain mapping

---

## Deliverables Checklist

| File | Description | Status |
|------|-------------|--------|
| `fly.toml` | Production-hardened config | Pending |
| `fly.staging.toml` | Staging config | Pending |
| `fly.customer.toml` | Per-customer template | Pending |
| `SECRETS.md` | Secrets manifest | Pending |
| `migrate-to-neon.sh` | Migration script | Pending |
| `waf-rules.conf` | WAF configuration | Pending |
| `.github/workflows/` | CI/CD pipelines | Pending |
| `DEPLOYMENT.md` | Step-by-step guide | Pending |
| `SECURITY_CHECKLIST.md` | Pre-launch checklist | Pending |

---

## Questions for Review

1. **Storage preference:** Should we use Fly Tigris, AWS S3, or Cloudflare R2 for LanceDB?
2. **Tenant isolation:** Do you want to proceed with Option A (per-customer app) or reconsider Option B?
3. **Monitoring target:** Which log aggregation service do you prefer (Datadog, Better Stack, Loki, S3)?
4. **Domain strategy:** Will each customer get their own subdomain or path-based isolation?

---

*Ready for your feedback on this plan structure. What should we adjust or clarify?*