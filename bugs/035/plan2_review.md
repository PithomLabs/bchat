# Adversarial Plan Review: Plan 035 v2 — Versioned RAG Index

**Reviewer:** Senior Go Architect  
**Date:** 2026-07-12  
**Verdict:** **Approved with nits** — All previous CRITICAL/SERIOUS items resolved. Three new nits remain.

---

## Previous review items — resolution status

| ID | Issue | Status | Notes |
|----|-------|--------|-------|
| C1 | Missing `HandleRAGSearch` | **Fixed** | §5 now lists all three handlers: `HandleTestRAGSearch`, `HandleTenantRAGSearch`, `HandleRAGSearch`. Grounded code gaps table also updated. |
| C2 | Postgres/MySQL drivers missing | **Fixed** | Locked decision #5, §1 migration paths, §4 all-three-drivers note. |
| C3 | `ReindexCheckpoint` schema underspecified | **Fixed** | §3 explicitly adds `FileType *string` / `Version *int32` to the struct + `Find*` + all three drivers + `ON CONFLICT` key change. |
| C4 | `audienceFiles` type unspecified | **Fixed** | §3 defines `fileEntry` struct and the new `map[string]map[string]fileEntry` type. Applied to all three reindex functions. |
| S1 | `DeleteAllAudiences` dead code | **Fixed** | §2 explicitly removes it from the plan. |
| S2 | `ReindexAllContent` not addressed | **Fixed** | Locked decision #6 + §3 applies version threading to all three functions. |
| S3 | `vectordb_nolance.go` / pool stubs | **Fixed** | §2 explicitly calls out both files. |
| S4 | Cutover `source_version` null/0/1 | **Fixed** | §3 cutover filter: `source_version IS NULL OR source_version = 0 OR source_version = 1`. |
| S5 | `ResolveQueryVersion` no-source-files edge | **Fixed** | §4 fallback #4 returns `0` → empty result set. |
| N1 | Line-number anchoring | **Fixed** | Uses function/struct names throughout. |
| N2 | `buildFilter` pre-existing injection | **Acknowledged** | §2 notes it; `source_version` uses `%d` (safe). |
| N3 | Verification missing failure modes | **Fixed** | §7 adds resume, concurrent, no-version, and cutover tests. |
| N4 | Version dropdown semantics | **Fixed** | §6 adds `/rag/indexed-versions` endpoint + "not indexed" badge cross-reference. |

---

## NEW ISSUES

### Nit 1: `ReindexCheckpoint` ON CONFLICT key migration is underspecified

The current SQLite upsert (`store/db/sqlite/agent.go`, `UpsertReindexCheckpoint`) uses:

```sql
ON CONFLICT(tenant_id, audience) DO UPDATE SET ...
```

Plan2 says to add `FileType`/`Version` fields and "key on `audience+fileType+version`", but doesn't specify:
- The new `ON CONFLICT` clause: `ON CONFLICT(tenant_id, audience, file_type, version)`?
- A migration to **drop the old unique constraint** on `(tenant_id, audience)` and create the new composite one.
- Whether existing checkpoint rows need backfilling (they have NULL `file_type`/`version` from the old schema).

This is a non-trivial migration that needs an explicit SQL statement in the migration file, not just "add fields." The Postgres and MySQL drivers have analogous unique constraints that also need migration.

**Recommendation:** Add to §3 a concrete migration fragment:

```sql
-- SQLite example
DROP INDEX IF EXISTS idx_reindex_checkpoint_tenant_audience;
CREATE UNIQUE INDEX idx_reindex_checkpoint_tenant_audience_filetype_version
    ON agent_reindex_checkpoints(tenant_id, audience, file_type, version);
```

### Nit 2: `resolveQueryVersion` step 3 queries LanceDB — but the service helper doesn't have VectorDB access

§4 says:

> 3. else query the **distinct indexed `source_version`** values in LanceDB for that `(tenant, audience, fileType)` and use the max;

This requires a new VectorDB method (e.g., `ListIndexedVersions(ctx, tenantID, audience, fileType) ([]int32, error)`). The plan mentions this step but doesn't add the method to the `VectorDB` interface or its implementations. Without it, `resolveQueryVersion` can't perform step 3.

**Recommendation:** Add to §2:

```
- Add to VectorDB interface:
  `ListIndexedVersions(ctx, tenantID int32, audienceType, fileType string) ([]int32, error)`.
  Lance impl: `SELECT DISTINCT source_version FROM <table> WHERE tenant_id=? AND audience_type=? AND content_type=?`.
  Memory/NoOp: scan in-memory chunks; NoOp returns empty.
```

### Nit 3: `POST /rag/active-version` needs auth/permission spec

§5 lists two new endpoints but doesn't specify their auth model. Given the existing RBAC system (`AGENTS.md` permission model), the plan should state:

- `POST /rag/active-version` → requires `api:config` (same as reindex — this modifies tenant state).
- `GET /rag/active-versions` → requires `tenant:read` (read-only, safe).
- `GET /rag/indexed-versions` → requires `tenant:read`.

Without this, the implementer has to guess or ask.

---

## VERIFICATION CHECKLIST

| Test | Covered in §7? |
|------|---------------|
| Happy path v1→v2 isolation | ✅ |
| Cutover NULL/0/1 purge | ✅ |
| Retention (6th version purges oldest) | ✅ |
| Rollback via active-version switch | ✅ |
| Resume after mid-flight failure | ✅ |
| Concurrent reindex no duplicates | ✅ |
| No-version fallback → empty results | ✅ |
| All three drivers compile + migrate | ✅ |
| `go build ./...` (non-rag + rag) | ✅ |
| Lint/typecheck | ✅ |

---

## RECOMMENDATION

Plan v2 is **approved for implementation**. All eight previous critical/serious issues are resolved with appropriate specificity. The three nits above are minor and can be addressed inline during implementation without a plan revision:

1. Write the `ON CONFLICT` migration SQL for all three drivers.
2. Add `ListIndexedVersions` to the `VectorDB` interface.
3. Add auth/permission notes to the three new endpoints.

No further plan revision needed. Ready to proceed.
