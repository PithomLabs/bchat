# Hybrid Search Production Rollout Guide

**Status:** Active
**Date:** 2026-01-23
**Related Documents:**
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment configuration
- [DOCS_EVALUATION.MD](./DOCS_EVALUATION.MD) - Quality metrics
- [DOCS_HYBRID_SEARCH.MD](./DOCS_HYBRID_SEARCH.MD) - Implementation details
- [DOCS_HALLUCINATION_GROUNDING.MD](./DOCS_HALLUCINATION_GROUNDING.MD) - Response validation

---

## Overview

This document describes the gradual rollout process for hybrid search from development to production. The rollout follows a **staged approach** to minimize risk and ensure quality.

### Rollout Philosophy

```
┌─────────────────────────────────────────────────────────────────┐
│                    ROLLOUT STAGES                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   Phase 1          Phase 2          Phase 3          Phase 4    │
│   ┌──────┐         ┌──────┐         ┌──────┐         ┌──────┐  │
│   │ Test │   →     │ 10%  │   →     │ 50%  │   →     │ 100% │  │
│   │Tenant│         │Tenants│         │Tenants│         │  GA  │  │
│   └──────┘         └──────┘         └──────┘         └──────┘  │
│                                                                  │
│   Validate    →    Monitor    →    Expand    →    General       │
│   Metrics          Metrics         Carefully       Availability  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Risk Mitigation Strategy

| Risk | Mitigation |
|------|------------|
| Search quality degradation | Monitor Recall/Precision, auto-rollback threshold |
| Latency increase | Monitor p99 latency, fallback to vector-only |
| Index corruption | Separate indexes per tenant, isolated failures |
| LLM cost spike | Monitor API usage, set budget alerts |

---

## Phase 1: Test Tenant Enablement

### Step 1.1: Create Test Tenant

```bash
# Create a dedicated test tenant via API
curl -X POST http://localhost:8080/api/v1/agent/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "name": "Hybrid Search Test",
    "slug": "hybrid-test",
    "description": "Test tenant for hybrid search validation"
  }'
```

### Step 1.2: Upload Test Content

```bash
# Upload KB with diverse content types
curl -X POST http://localhost:8080/api/v1/agent/hybrid-test/kb/import \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -F "file=@docs/evaluation/datasets/kb_fixtures/test_kb.md"

# Upload Policy
curl -X POST http://localhost:8080/api/v1/agent/hybrid-test/policy/import \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -F "file=@docs/evaluation/datasets/kb_fixtures/test_policy.md"
```

### Step 1.3: Enable Hybrid Search for Test Tenant

**Option A: Global Enable (All Tenants)**
```bash
# Environment variables
export HYBRID_SEARCH_ENABLED=true
export HYBRID_VECTOR_WEIGHT=0.7
export HYBRID_TEXT_WEIGHT=0.3
```

**Option B: Per-Tenant Enable (Recommended for Rollout)**
```sql
-- Enable hybrid search for specific tenant only
UPDATE agent_tenant_config
SET hybrid_search_enabled = true,
    hybrid_vector_weight = 0.7,
    hybrid_text_weight = 0.3
WHERE tenant_id = (SELECT id FROM agent_tenant WHERE slug = 'hybrid-test');
```

### Step 1.4: Verification Checklist

Run these checks before proceeding:

| Check | Command | Expected |
|-------|---------|----------|
| FTS Index Created | Check server logs | "FTS index created" |
| Hybrid Mode Active | Query with `hybrid: true` | `SearchMode: "hybrid"` |
| BM25 Scores Present | Check response | `BM25Scores: [0.x, ...]` |
| Latency Acceptable | Check `Latency` field | < 300ms |

**Verification Script:**
```bash
# Test hybrid search endpoint
curl -X POST http://localhost:8080/api/v1/agent/hybrid-test/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "water damage emergency",
    "hybrid": true,
    "top_k": 10
  }' | jq '{
    search_mode: .search_mode,
    latency_ms: .latency_ms,
    result_count: .results | length,
    has_bm25_scores: (.bm25_scores | length > 0)
  }'
```

**Expected Output:**
```json
{
  "search_mode": "hybrid",
  "latency_ms": 125,
  "result_count": 10,
  "has_bm25_scores": true
}
```

---

## Phase 2: Metrics Monitoring

### Key Metrics Dashboard

#### Retrieval Quality Metrics

| Metric | Target | Alert Threshold | Critical |
|--------|--------|-----------------|----------|
| Recall@10 | > 0.85 | < 0.80 | < 0.70 |
| Precision@10 | > 0.70 | < 0.65 | < 0.55 |
| MRR | > 0.65 | < 0.60 | < 0.50 |
| NDCG@10 | > 0.80 | < 0.75 | < 0.65 |

#### Latency Metrics

| Metric | Target | Alert Threshold | Critical |
|--------|--------|-----------------|----------|
| p50 Total Latency | < 150ms | > 200ms | > 300ms |
| p99 Total Latency | < 300ms | > 400ms | > 500ms |
| Embedding Latency | < 100ms | > 150ms | > 200ms |
| Search Latency | < 50ms | > 75ms | > 100ms |

#### Error Metrics

| Metric | Target | Alert Threshold | Critical |
|--------|--------|-----------------|----------|
| Search Error Rate | < 0.1% | > 0.5% | > 1% |
| FTS Index Errors | 0 | > 1/hour | > 5/hour |
| Embedding Failures | < 0.1% | > 0.5% | > 1% |

### Monitoring Commands

**Real-time Log Monitoring:**
```bash
# Watch for hybrid search events
tail -f logs/app.log | grep -E "(hybrid|bm25|fts)"

# Monitor search latency
tail -f logs/app.log | grep "search completed" | \
  jq -r '[.timestamp, .latency_ms, .search_mode] | @tsv'
```

**Metrics Aggregation:**
```bash
# Calculate average latency over last hour
grep "search completed" logs/app.log | \
  tail -1000 | \
  jq -r '.latency_ms' | \
  awk '{sum+=$1; count++} END {print "Avg:", sum/count, "ms"}'

# Count searches by mode
grep "search completed" logs/app.log | \
  tail -1000 | \
  jq -r '.search_mode' | \
  sort | uniq -c
```

**Database Metrics:**
```sql
-- Search statistics by tenant
SELECT
    t.slug,
    COUNT(*) as total_searches,
    AVG(s.latency_ms) as avg_latency,
    SUM(CASE WHEN s.search_mode = 'hybrid' THEN 1 ELSE 0 END) as hybrid_searches
FROM search_logs s
JOIN agent_tenant t ON s.tenant_id = t.id
WHERE s.created_at > datetime('now', '-1 hour')
GROUP BY t.slug;
```

### Alert Configuration

**Prometheus Alert Rules (Example):**
```yaml
groups:
  - name: hybrid_search_alerts
    rules:
      - alert: HybridSearchLatencyHigh
        expr: histogram_quantile(0.99, search_latency_seconds) > 0.4
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Hybrid search p99 latency > 400ms"

      - alert: HybridSearchErrorRate
        expr: rate(search_errors_total[5m]) / rate(search_total[5m]) > 0.005
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Search error rate > 0.5%"
```

### Monitoring Duration

| Phase | Duration | Focus |
|-------|----------|-------|
| Initial (24h) | 24 hours | All metrics, high attention |
| Stabilization (7d) | 7 days | Trends, edge cases |
| Ongoing | Continuous | Anomaly detection |

---

## Phase 3: Gradual Production Rollout

### Stage 3.1: 10% of Tenants (Canary)

**Tenant Selection Criteria:**
- Low traffic volume (< 100 queries/day)
- Non-critical use cases
- Engaged customer for feedback
- Diverse content types

**Enable for Selected Tenants:**
```sql
-- Select 10% of tenants for canary
WITH canary_tenants AS (
    SELECT id, slug
    FROM agent_tenant
    WHERE is_active = true
    ORDER BY RANDOM()
    LIMIT (SELECT COUNT(*) * 0.1 FROM agent_tenant WHERE is_active = true)
)
UPDATE agent_tenant_config
SET hybrid_search_enabled = true
WHERE tenant_id IN (SELECT id FROM canary_tenants);

-- Log which tenants are in canary
SELECT slug FROM agent_tenant
WHERE id IN (
    SELECT tenant_id FROM agent_tenant_config
    WHERE hybrid_search_enabled = true
);
```

**Duration:** 3-5 days
**Success Criteria:** All metrics within target ranges

### Stage 3.2: 25% of Tenants

**Additional Selection:**
- Include medium-traffic tenants
- Include different content domains
- Maintain geographic diversity

```sql
-- Expand to 25% of tenants
WITH expansion_tenants AS (
    SELECT id FROM agent_tenant
    WHERE is_active = true
    AND id NOT IN (
        SELECT tenant_id FROM agent_tenant_config
        WHERE hybrid_search_enabled = true
    )
    ORDER BY RANDOM()
    LIMIT (SELECT COUNT(*) * 0.15 FROM agent_tenant WHERE is_active = true)
)
UPDATE agent_tenant_config
SET hybrid_search_enabled = true
WHERE tenant_id IN (SELECT id FROM expansion_tenants);
```

**Duration:** 5-7 days
**Success Criteria:** Stable metrics, no degradation

### Stage 3.3: 50% of Tenants

**Selection:**
- Include high-traffic tenants
- Include enterprise customers
- Full content diversity

```sql
-- Expand to 50%
UPDATE agent_tenant_config
SET hybrid_search_enabled = true
WHERE tenant_id IN (
    SELECT id FROM agent_tenant
    WHERE is_active = true
    ORDER BY RANDOM()
    LIMIT (SELECT COUNT(*) * 0.5 FROM agent_tenant WHERE is_active = true)
);
```

**Duration:** 7-14 days
**Success Criteria:** Stable at scale, positive feedback

### Stage 3.4: 100% General Availability

**Final Rollout:**
```bash
# Enable globally via environment
export HYBRID_SEARCH_ENABLED=true

# Or enable for all tenants
sqlite3 data/memos.db "UPDATE agent_tenant_config SET hybrid_search_enabled = true;"
```

**Post-GA Monitoring:** Continue monitoring for 30 days

---

## Phase 4: Rollback Procedures

### Immediate Rollback (Emergency)

**Disable Hybrid Search Globally:**
```bash
# Set environment variable
export HYBRID_SEARCH_ENABLED=false

# Restart server
systemctl restart bchat
```

**Disable for Specific Tenant:**
```sql
UPDATE agent_tenant_config
SET hybrid_search_enabled = false
WHERE tenant_id = (SELECT id FROM agent_tenant WHERE slug = 'affected-tenant');
```

### Gradual Rollback

**Reduce Rollout Percentage:**
```sql
-- Roll back to 25% (disable 75%)
WITH rollback_tenants AS (
    SELECT tenant_id FROM agent_tenant_config
    WHERE hybrid_search_enabled = true
    ORDER BY RANDOM()
    LIMIT (SELECT COUNT(*) * 0.75 FROM agent_tenant_config WHERE hybrid_search_enabled = true)
)
UPDATE agent_tenant_config
SET hybrid_search_enabled = false
WHERE tenant_id IN (SELECT tenant_id FROM rollback_tenants);
```

### Data Preservation

**FTS indexes are preserved** even when hybrid search is disabled:
- No data loss on rollback
- Re-enable is instant (no reindexing needed)
- Vector indexes remain intact

### Rollback Decision Tree

```
Is search completely broken?
├── YES → Immediate global rollback
└── NO
    └── Is latency > 500ms p99?
        ├── YES → Rollback affected tenants
        └── NO
            └── Is error rate > 1%?
                ├── YES → Rollback affected tenants
                └── NO
                    └── Is Recall < 0.70?
                        ├── YES → Investigate, consider rollback
                        └── NO → Continue monitoring
```

---

## Success Criteria

### Metrics Thresholds for Advancing

| Stage | Recall@10 | Precision@10 | p99 Latency | Error Rate |
|-------|-----------|--------------|-------------|------------|
| Test → 10% | > 0.80 | > 0.65 | < 350ms | < 0.5% |
| 10% → 25% | > 0.82 | > 0.67 | < 320ms | < 0.3% |
| 25% → 50% | > 0.84 | > 0.69 | < 300ms | < 0.2% |
| 50% → 100% | > 0.85 | > 0.70 | < 280ms | < 0.1% |

### Go/No-Go Checklist

Before advancing to next stage:

- [ ] All metrics within target ranges for 48+ hours
- [ ] No critical alerts triggered
- [ ] Error rate stable or decreasing
- [ ] Latency stable or improving
- [ ] No customer complaints related to search
- [ ] FTS indexes healthy (no corruption)
- [ ] Embedding service stable

### Sign-Off Requirements

| Stage | Sign-Off Required |
|-------|-------------------|
| Test → 10% | Engineering Lead |
| 10% → 25% | Engineering Lead |
| 25% → 50% | Engineering Lead + Product |
| 50% → 100% | Engineering Lead + Product + Ops |

---

## Appendix: Quick Reference

### Environment Variables

```bash
# Core hybrid search
HYBRID_SEARCH_ENABLED=true|false
HYBRID_VECTOR_WEIGHT=0.7
HYBRID_TEXT_WEIGHT=0.3

# RAG pipeline (required)
RAG_PIPELINE_ENABLED=true
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
```

### Useful Commands

```bash
# Check current rollout percentage
sqlite3 data/memos.db "SELECT
    COUNT(CASE WHEN hybrid_search_enabled THEN 1 END) * 100.0 / COUNT(*)
    AS rollout_percentage
FROM agent_tenant_config;"

# List tenants with hybrid enabled
sqlite3 data/memos.db "SELECT t.slug, c.hybrid_search_enabled
FROM agent_tenant t
JOIN agent_tenant_config c ON t.id = c.tenant_id
WHERE c.hybrid_search_enabled = true;"

# Check recent search modes
grep "search completed" logs/app.log | tail -100 | \
  jq -r '.search_mode' | sort | uniq -c
```

### Emergency Contacts

| Role | Responsibility |
|------|----------------|
| On-Call Engineer | Immediate response, rollback authority |
| Engineering Lead | Escalation, architecture decisions |
| Product Owner | Customer communication |

---

*Document Version: 1.0*
*Created: 2026-01-23*
