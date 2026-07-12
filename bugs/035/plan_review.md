# Adversarial Plan Review: Plan 035 — Versioned RAG Index

**Reviewer:** Senior Go Architect  
**Date:** 2026-07-12  
**Verdict:** **Rework Required** — Solid design direction, but several critical gaps and a few incorrect assumptions.

---

## CONFIRMED (grounded in code)

The core problem analysis is accurate:

1. **`source_version=1` hardcoded** — Verified at `service.go:350,354,479,483,807,811` — all six `ChunkMarkdownContent` calls pass `1` as the version argument. The `DocumentChunk.SourceVersion` field exists (`chunker.go:31`) and LanceDB stores it (`vectordb_lance.go:211`), but it's always 1.

2. **No version filter in search** — `SearchQuery` (`vectordb.go:134`) has no `SourceVersion` field. `buildFilter` (`vectordb_lance.go:1045-1067`) filters on `tenant_id`, `audience_type`, `is_active`, `content_type` — but not `source_version`.

3. **"all" delete is a no-op** — `LanceVectorDB.Delete` (`vectordb_lance.go:941`) builds `audience_type = 'all'`, which matches zero rows (actual values are `"internal"`/`"external"`). So `ReindexTenantContentWithResume` with `audienceType="all"` at `service.go:828` deletes nothing then appends, creating duplicates.

4. **Search handlers lack version/file_type fields** — `HandleTenantRAGSearch` (`handlers.go:4881`) uses an inline struct with only `query`, `audience_type`, `top_k`. `HandleTestRAGSearch` (`handlers.go:4744`) uses `RAGSearchRequest` (`handlers.go:4535`) which also lacks `version`/`file_type`. `HandleRAGSearch` (`handlers.go:5818`) — a **third** search handler the plan doesn't mention — also lacks these fields.

5. **Auto-reindex on upload** — `importFiles` at `handlers.go:1559-1565` calls `ReindexTenantContentWithResume` after every upload. Verified.

6. **"All" option in UI** — `AgentAdmin.tsx:1208` has `<Option value="all">All</Option>`, default state is `"all"` at line 136.

7. **File versions endpoint exists** — `GET /api/v1/agent/:slug/files/:audienceType/:fileType/versions` at `handlers.go:855`, called from `agentAdmin.ts:580-592`.

---

## CRITICAL ISSUES

### C1. Missing `HandleRAGSearch` — there are THREE search handlers, not two

The plan references `HandleTestRAGSearch` (`:4744`) and `HandleTenantRAGSearch` (`:4880`), but there is also `HandleRAGSearch` (`:5818`) which is the **actual** handler registered at `v1.go:372` for `adminGroup.POST("/:slug/rag/search")`. This is the one called by the AgentAdmin UI (`agentAdmin.ts:1080`). The plan's §5 must update **all three** handlers, not two.

### C2. Postgres and MySQL driver stubs required

The plan only specifies `store/migration/sqlite/NN__rag_active_versions.sql`. But the codebase has three drivers: SQLite (`store/db/sqlite/agent.go`), Postgres (`store/db/postgres/agent.go`), and MySQL (`store/db/mysql/agent.go`). The new `UpsertAgentRAGActiveVersion` / `GetAgentRAGActiveVersion` / `ListAgentRAGActiveVersions` / `DeleteAgentRAGActiveVersion` methods must be implemented in all three, plus migration SQL for each. The `ReindexCheckpoint` pattern (all three drivers at `driver.go:253-255`) is the precedent — follow it exactly.

### C3. `ReindexCheckpoint` schema needs version/file_type for resume correctness

The plan says "Update checkpoint logic (`ReindexCheckpoint`) to key on `audience+fileType+version`" but the current `ReindexCheckpoint` struct (`store/agent.go:1147-1163`) has `TenantID` + `Audience` only — no `FileType` or `Version` fields. `FindReindexCheckpoint` (`store/agent.go:1165-1170`) also lacks these. The plan must specify adding these fields to the struct, the `Find*` filter, the SQL schema, and all three driver implementations. This is a non-trivial change that affects resume behavior.

### C4. `audienceFiles` grouping loses version — data structure change unspecified

Current: `audienceFiles := make(map[string]map[string]string)` at `service.go:793` — maps `audience → fileType → content`. Version is discarded. The plan says "restructure to preserve `f.Version` per `(audience, fileType)`" but doesn't specify the new type. This needs to be explicit, e.g.:

```go
type fileEntry struct {
    content string
    version int32
}
audienceFiles := make(map[string]map[string]fileEntry)
```

This same pattern exists in `ReindexAllContent` at `service.go:318` and `ReindexTenantContent` at `service.go:463` — all three functions need the same restructuring.

---

## SERIOUS ISSUES

### S1. `DeleteAllAudiences` is dead code under the new design

Plan §2 proposes `DeleteAllAudiences(ctx, tenantID)` to fix the "all" delete bug. But §3 says the new reindex **never wipes the whole audience** — it inserts a new versioned set and updates the active pointer. The only deletion in the new flow is `DeleteByVersion` for retention cleanup and cutover. `DeleteAllAudiences` would only be useful if someone explicitly wants to nuke all RAG data for a tenant, which isn't part of the reindex flow. Either:
- Remove `DeleteAllAudiences` from the plan, or
- Explicitly scope it to a separate "purge all RAG data" admin action.

### S2. `ReindexAllContent` (`service.go:285`) is not addressed

This function iterates ALL tenants, calls `ChunkMarkdownContent` with hardcoded `1` at lines 350/354, and does `Delete(ctx, tenantID, audience)` at line 472. The plan only addresses `ReindexTenantContentWithResume`. Either:
- The plan should explicitly deprecate `ReindexAllContent` (and document why it's safe to leave as-is), or
- It needs the same version threading applied.

### S3. `vectordb_nolance.go` stub needs `DeleteByVersion`

The non-rag build stub (`vectordb_nolance.go`) has stubs for every `VectorDB` method. Adding `DeleteByVersion` to the interface will break non-rag builds unless a stub is added here. Similarly, `vectordb_pool.go` (rag build) needs the delegation.

### S4. Cutover logic edge case — which `source_version` to delete?

Plan §3 says "on the first versioned reindex, delete existing stale `source_version=1` chunks." But:
- Old chunks from the original (pre-versioning) code have `source_version=1` (hardcoded).
- The LanceDB schema has `source_version` as **nullable** (`AddInt32Field("source_version", true)` at `vectordb_lance.go:211`). Null/default would be `0` for Go `int32`.
- The `rowToDocumentChunk` at `vectordb_lance.go:1257-1260` handles both `int32` and `float64` but not null.

The cutover filter must match ALL pre-versioned chunks, not just `source_version=1`. Consider: `source_version IS NULL OR source_version = 0 OR source_version = 1`.

### S5. `ResolveQueryVersion` fallback when no source files exist

The plan's §4 helper falls back to `latest agent_source_files.version`. But what if a tenant has never uploaded any source files for that `(audience, fileType)`? The function needs an explicit "not found" path (return 0, or error).

---

## NITS

### N1. The plan references line numbers that will shift

Line numbers like `handlers.go:4535`, `service.go:350` etc. are fragile. Recommend using function names as anchors instead (e.g., "the `RAGSearchRequest` struct" rather than "`handlers.go:4535`").

### N2. `buildFilter` SQL injection — existing but worth noting

`buildFilter` at `vectordb_lance.go:1045` uses `fmt.Sprintf` for all filter values including `AudienceType` (a string from user input). Adding `source_version` as an integer via `%d` is safe, but the existing pattern is fragile. This is a pre-existing issue, not caused by this plan, but worth flagging.

### N3. Plan §7 verification is light on failure modes

The verification plan covers the happy path (v1→v2, retention, rollback) but doesn't test:
- Resume after mid-reindex failure with versioned checkpoint
- Concurrent reindex of the same `(tenant, audience, fileType)`
- Migration of existing `source_version=1` chunks to the versioned model

### N4. Version dropdown data semantics need clarity

Plan §6 says the version dropdown uses "the existing `/files/{audience}/{fileType}/versions` endpoint at `agentAdmin.ts:582`". This endpoint exists (`handlers.go:855`). However, the response returns `AgentSourceFile[]` — document versions, not index versions. If a document version hasn't been indexed yet (no corresponding chunks in LanceDB), the dropdown would show versions that can't actually be queried. The UI should either:
- Cross-reference with indexed versions, or
- Add visual indicators (e.g., "not indexed" badge) for versions without chunks.

---

## RECOMMENDATION

The plan is **directionally correct** and the locked design decisions are sound. The core architecture (per-file-type versioning, active pointer, retention) is well-conceived. However, the plan needs:

1. **Add `HandleRAGSearch`** (`:5818`) to the search handler update list.
2. **Expand §1** to include Postgres/MySQL migrations and driver implementations.
3. **Expand §3** with explicit `ReindexCheckpoint` schema changes and `audienceFiles` restructuring types.
4. **Remove or scope `DeleteAllAudiences`** — it's not needed by the new flow.
5. **Address `ReindexAllContent`** explicitly (deprecate or update).
6. **Specify cutover filter** to handle null/0/1 `source_version` values.
7. **Add `vectordb_nolance.go` and `vectordb_pool.go` stubs** for `DeleteByVersion`.
8. **Expand §7** with edge case tests (resume, concurrent reindex, migration).

Once these gaps are filled, the plan is ready for implementation.
