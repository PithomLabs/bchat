# Plan 024 (v3) — Adversarial Review

**Reviewer:** Senior Go Architect
**Date:** 2026-07-07
**Target:** `plan_024_v3.md`
**Verdict:** APPROVED WITH NITS

---

## Summary

The plan is architecturally sound. Per-tenant S3 prefix isolation in a shared bucket, lazy connection pool, and Dockerfile parity are all correct. The `ForcePathStyle` bug fix is well-identified. Seven items below need attention before implementation.

---

## Critical Issues (must fix)

### 1. `AllowHTTP` is on `StorageOptions`, not `S3Config`

The plan Step 1 says:

> pass `AllowHTTP: ptr(config.S3AllowHTTP)`

But in `contracts/config.go:22`, `AllowHTTP` is a field of `StorageOptions`, not `S3Config`:

```go
type StorageOptions struct {
    S3Config   *S3Config   `json:"s3_config,omitempty"`
    AllowHTTP  *bool       `json:"allow_http,omitempty"`  // ← here
    // ...
}
```

The correct wiring:

```go
connOpts = &contracts.ConnectionOptions{
    StorageOptions: &contracts.StorageOptions{
        AllowHTTP: ptr(config.S3AllowHTTP),
        S3Config: &contracts.S3Config{
            Endpoint:       ptr(config.S3Endpoint),
            Region:         ptr(config.S3Region),
            AccessKeyID:    ptr(config.S3AccessKey),
            SecretAccessKey: ptr(config.S3SecretKey),
            ForcePathStyle: ptr(config.S3ForcePathStyle),
        },
    },
}
```

### 2. `Insert` does not validate single-tenant assumption

Step 3 says the pool routes Insert via "first chunk's TenantID". If a batch mixes chunks from different tenants (possible in `ReindexAllContent` if iteration order is non-deterministic), all chunks silently land in the wrong tenant's connection.

**Fix:** Either validate all chunks share the same TenantID at the start of Insert, or pre-group the batch by tenant before passing to the pool. The pool's Insert method should reject mixed-tenant batches:

```go
func (p *TenantVectorDBPool) Insert(ctx context.Context, chunks []DocumentChunk) error {
    if len(chunks) == 0 { return nil }
    tenantID := chunks[0].TenantID
    for _, c := range chunks[1:] {
        if c.TenantID != tenantID {
            return fmt.Errorf("mixed-tenant batch: first=%d, found=%d", tenantID, c.TenantID)
        }
    }
    db, err := p.Get(ctx, tenantID)
    // ...
}
```

### 3. `Stats` enumeration on lazy-loaded pool

Step 3 says Stats should "enumerate tenants, aggregate per-tenant Stats". The pool is lazy-loaded — tenants without a cached connection are invisible to Stats.

**Fix:** The pool needs a `ListTenants()` call to the store at Stats time, or maintain a `knownTenants` set populated during `NewService`/startup. Without this, Stats silently underreports.

---

## Minor Issues (should fix)

### 4. Dockerfile.s3.fly `mkdir` step misleading

Step 8 says keep `RUN mkdir -p /var/opt/memos/lancedb` for parity. With S3 storage, LanceDB data lives in S3 — the local `lancedb` directory is unused. Only `RUN mkdir -p /var/opt/memos` (for SQLite) is needed. The "for parity" justification is confusing; drop it or reword.

### 5. Old data orphaning with no cleanup path

The migration section says old shared table data is orphaned and "delete old shared table from bucket later" but gives no mechanism. Add: use `tigris ls s3://<bucket>/lancedb/` to list the old `kb_documents_*` objects, then `tigris rm` them after confirming per-tenant data is populated. Or use `fly storage dashboard` to browse/delete manually.

### 6. Tenant override connection caching has no eviction path

When `TenantConfig.VectorDBS3Override` changes, the pool's cached connection still uses the old S3Config. The plan should specify:

```go
// After upserting the override:
pool.Evict(tenantID) // Close() + delete from map
```

The next access creates a fresh connection with the new override. Without eviction, override changes are silently ignored.

### 7. `RefreshVectorDB` does not drain connections

Step 5 says RefreshVectorDB should "rebuild pool" but omits draining existing connections. The implementation must call `Close()` on all cached `LanceVectorDB` instances before replacing the map, or connections leak.

---

## Confirmed Correct

| Item | Status |
|------|--------|
| `ForcePathStyle: false` for Tigris (bug at `vectordb_lance.go:66`) | ✅ Correct |
| Per-tenant S3 prefix `s3://<bucket>/lancedb/<tenant_id>/` | ✅ Correct |
| `TenantConfig` as opaque JSON string (matches `Features` pattern) | ✅ Correct |
| Dockerfile parity items (vendor copy, mui CSS test, env vars) | ✅ Correct |
| Build tag `rag` for pool file | ✅ Correct |
| Lazy connection creation (avoids startup latency) | ✅ Correct |
| `newLanceVectorDB` called per-tenant with resolved S3Config | ✅ Correct |
| `ReindexAllContent` already iterates tenants (pool routing works) | ✅ Correct |
| `RefreshVectorDB` rebuilds pool (correct scope) | ✅ Correct |
| `GetVectorDB()` returns pool (still satisfies `VectorDB` interface) | ✅ Correct |

---

## Recommended Implementation Order

1. Fix Steps 1-2 (config struct + `newLanceVectorDB` changes) — foundation
2. Step 3 (pool) — with the three fixes above
3. Step 4 (per-tenant URI) — depends on pool
4. Step 5 (wire into Service) — depends on pool
5. Step 6 (migration + store) — independent, can parallel with 2-4
6. Step 7 (admin API) — after store
7. Step 8 (Dockerfile) — independent, can parallel with 1-6
8. Step 9 (provisioning) — after deploy

Steps 6 and 8 are independent of the pool work and can be done in parallel with Steps 2-5.
