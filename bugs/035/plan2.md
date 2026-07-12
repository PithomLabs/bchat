# Plan 035 (v2) — Versioned RAG Index (per-file-type, active-pointer + retention)

> Revised per `plan_review.md` (adversarial review) and the two additional changes
> (remove "All" Rebuild-Index option; remove auto-reindex on upload). This version
> addresses all CRITICAL/SERIOUS/NIT items from the review. Anchors use function names
> (line numbers are辅助 and will shift as code changes).

## Problem / Context (why this exists)

The bchat RAG pipeline lets a tenant upload `KB.MD`, `POLICY.MD`, and `SCRIPT.MD` that
are chunked and embedded into a LanceDB vector index, then queried by the Agent Admin
**RAG Search Explorer** and the RAG Stats **Search Testing** surfaces.

Today the documents themselves are versioned in the database (`agent_source_files.version`,
auto-incremented per `tenant+audience+file_type`, with a version-history + restore UI),
but **the vector index is not version-aware**:

- `SearchQuery` (in `vectordb.go`, `type SearchQuery struct`) and `buildFilter` (in
  `vectordb_lance.go`, `func (db *LanceVectorDB) buildFilter`) have **no version filter** —
  a search returns the union of *all* chunks for an audience regardless of document version.
- The Lance schema stores `source_version` (`vectordb_lance.go` `AddInt32Field("source_version", true)`)
  and `DocumentChunk.SourceVersion` exists (`chunker.go`, `type DocumentChunk struct`),
  but reindex **hardcodes `sourceVersion=1`** at all six `ChunkMarkdownContent` call sites
  (in `ReindexAllContent`, `ReindexTenantContent`, and `ReindexTenantContentWithResume`),
  so the real document version is never recorded.
- Reindex **wipes then replaces** the audience: `LanceVectorDB.Delete` (in `vectordb_lance.go`)
  builds `audience_type = '<value>'`; when the value is `"all"` it matches zero rows, so
  `ReindexTenantContentWithResume` calling `Delete(ctx, tenantID, "all")` deletes nothing
  then re-inserts → **duplicates** (the "Rebuild Index → All" bug).
- The search handlers (`HandleTestRAGSearch`, `HandleTenantRAGSearch`, and `HandleRAGSearch`)
  and their UIs send only `query/audience/top_k` — no way to target a specific document version.

### Consequence
A user cannot ask "what does KB.md **version 3** say about X?", cannot compare retrieval
across KB versions, and cannot roll back the *index* to a prior KB without re-embedding.
Document-level versioning is invisible at query time, and reindexing destroys history.

### Goal
Make the RAG index **versioned** so Explorer/Testing query a specific versioned document
(e.g. `KB.md` v3), with retained history, an explicit "active version" pointer (instant
rollback without re-embedding), and automatic retention to bound index size.

## Locked design decisions (from Q&A)
1. **Granularity** — versioning is **per file type** (`kb` / `policy` / `script`), matching
   `agent_source_files.version` (per `tenant+audience+file_type`).
2. **Selection** — a **stored "active version" pointer** per `(tenant, audience, file_type)`,
   defaulting to latest on (re)index; Explorer/Testing may **override per query**.
3. **Retention** — keep the **last N = 5** versions; automatically purge older ones.
4. **Scope** — **full** (backend filter + UI picker + active-pointer/rollback + retention).
5. **Drivers** — new store methods + migrations implemented for **all three** DB drivers
   (SQLite, Postgres, MySQL), following the `ReindexCheckpoint` precedent.
6. **`ReindexAllContent`** — **updated** with version threading (version-safe), not deprecated.
7. **No-version fallback** — resolve to **latest indexed `source_version`**; if none indexed,
   return empty results.

## Addendum changes (A & B)
- **Change A** — Remove the "All" option under Rebuild Index (Agent Admin); default the
  dropdown to `internal`; backend `audience_type=all` kept but made safe via per-audience
  append (no broad wipe).
- **Change B** — Remove auto-reindex on upload (`importFiles`); upload creates a new document
  version but the index only changes on manual Rebuild Index; add a post-upload reminder hint.

## Grounded code gaps
| Area | Anchor | Issue |
|------|--------|-------|
| Search filter | `SearchQuery` / `buildFilter` | No `source_version` filter (vector + FTS both use `buildFilter`) |
| Version recording | `ChunkMarkdownContent` (6 call sites in `ReindexAllContent`, `ReindexTenantContent`, `ReindexTenantContentWithResume`) | hardcoded `1` |
| Reindex semantics | `LanceVectorDB.Delete` + `ReindexTenantContentWithResume` | `"all"` → no-op delete → duplicates; whole-audience wipe loses history |
| Active-version storage | (none) | No table/field for the active pointer |
| Search handlers | `HandleTestRAGSearch`, `HandleTenantRAGSearch`, `HandleRAGSearch` | No `version` / `file_type` |
| `SearchVectorDB` | `func (s *Service) SearchVectorDB` (service.go) | Builds its own `SearchQuery`; needs `SourceVersion` param |
| Auto-reindex | `importFiles` (calls `ReindexTenantContentWithResume`) | Reindexes on every upload |
| "All" UI option | `AgentAdmin.tsx` (Rebuild Index `Select`) | `<Option value="all">All</Option>`; default `"all"` |

## Implementation plan (revised)

### 1. Data model
- **New table** `agent_rag_active_versions(tenant_id, audience_type, file_type, version)`
  + index on `(tenant_id, audience_type, file_type)`.
  - Migrations in **all three** drivers: `store/migration/sqlite/NN__rag_active_versions.sql`,
    the Postgres migration dir, and the MySQL migration dir (mirror the `NN__snake_case.sql`
    convention and the `ReindexCheckpoint` migration pattern).
- `SearchQuery` (`type SearchQuery struct`, vectordb.go): add `SourceVersion *int32`.
- `ChunkInfo` (handlers.go, `type ChunkInfo struct`): add `SourceVersion int32` (provenance).
- `RAGSearchRequest` (handlers.go, `type RAGSearchRequest struct`): add `SourceVersion *int32`
  + optional `FileType string`. The inline request structs in `HandleTenantRAGSearch` and
  `HandleRAGSearch` gain `version` + `file_type`.

### 2. Vector DB layer (`vectordb.go` + `vectordb_lance.go` + stubs)
- Add to `VectorDB` interface (`type VectorDB interface`, vectordb.go):
  `DeleteByVersion(ctx, tenantID int32, audienceType, fileType string, version int32) error`.
  Implement in `LanceVectorDB` (real `DELETE` where
  `tenant_id=? AND audience_type=? AND content_type=? AND source_version=?`),
  `MemoryVectorDB`, and `NoOpVectorDB`.
- Add stub in **`vectordb_nolance.go`** (`type NoOpVectorDB` / non-rag build) and delegation in
  **`vectordb_pool.go`** (`TenantVectorDBPool`, alongside its existing `Delete`/`Search`
  delegations) so non-rag builds and the pool compile.
- `buildFilter` (`func (db *LanceVectorDB) buildFilter`): append `source_version = N` when
  `query.SourceVersion != nil`. Feeds both vector and FTS paths — no separate FTS change.
  (Pre-existing `fmt.Sprintf` injection in `buildFilter` is noted as a pre-existing issue, not
  introduced here; `source_version` is an `int32` via `%d`, safe.)
- **Remove `DeleteAllAudiences`** from the plan — the new flow never wipes a whole audience;
  the only deletions are `DeleteByVersion` (retention + cutover). The `"all"` reindex path is
  kept but made safe: it loops `internal`/`external` and appends a new versioned set per
  audience (no broad `Delete`).

### 3. Reindex (`service.go`) — versioned, append-only, no wipe
- Thread the **real version** at all six `ChunkMarkdownContent` call sites: pass `f.Version`
  instead of `1`. Applies to `ReindexAllContent`, `ReindexTenantContent`, and
  `ReindexTenantContentWithResume`.
- **Restructure `audienceFiles` grouping** (in all three reindex functions) from
  `map[string]map[string]string` to:
  ```go
  type fileEntry struct {
      content string
      version int32
  }
  audienceFiles := make(map[string]map[string]fileEntry) // audience -> fileType -> fileEntry
  ```
- **Change semantics**: do **not** call the broad `Delete(ctx, tenantID, audienceType)`.
  Insert the new versioned chunks (tagged with `f.Version`); after insert, **set the active
  pointer** = new version for that `(tenant, audience, fileType)`.
- **Retention** (after insert): list indexed versions for `(tenant, audience, fileType)`;
  if count > 5, `DeleteByVersion` the oldest beyond 5.
- **Cutover**: on the first versioned reindex for a `(tenant, audience, fileType)`, delete
  pre-versioning chunks via
  `source_version IS NULL OR source_version = 0 OR source_version = 1`
  (schema is nullable; old entries are `NULL`/`0`/`1`), then insert as the current version.
  Subsequent reindexes append.
- **`ReindexCheckpoint` schema** (`store/agent.go`, `type ReindexCheckpoint struct` +
  `FindReindexCheckpoint`): add `FileType *string` and `Version *int32` fields; extend the
  `Find*` filter; update the SQL schema and **all three driver** implementations
  (`store/db/sqlite/agent.go`, `store/db/postgres/agent.go`, `store/db/mysql/agent.go`).
  Checkpoint keys on `audience+fileType+version` so resume stays correct.

### 4. Active-version store + helper
- Store interface (`store/driver.go`): add
  `UpsertAgentRAGActiveVersion`, `GetAgentRAGActiveVersion(ctx, tenantID, audience, fileType) (*int32, error)`,
  `ListAgentRAGActiveVersions(tenantID)`, `DeleteAgentRAGActiveVersion(...)`.
  Implement in **all three** drivers (model on `ReindexCheckpoint` methods).
- Service helper `resolveQueryVersion(tenantID, audience, fileType, requested *int32) int32`:
  1. if `requested` set → use it;
  2. else look up the active pointer → use it;
  3. else query the **distinct indexed `source_version`** values in LanceDB for that
     `(tenant, audience, fileType)` and use the max;
  4. if none indexed → return `0` (search applies **no version filter → empty result set**,
     i.e. the query returns nothing rather than stale chunks).

### 5. Endpoints (`handlers.go` + `v1.go`)
- Update **all three** search handlers:
  - `HandleTestRAGSearch` (uses `RAGSearchRequest`): read `version`/`file_type`; set
    `SearchQuery.ContentTypes=[fileType]` (optional) and `SearchQuery.SourceVersion = resolveQueryVersion(...)`.
  - `HandleTenantRAGSearch` (inline struct): same.
  - `HandleRAGSearch` (inline struct): same, and thread `SourceVersion` into
    `service.SearchVectorDB` (add a `sourceVersion *int32` parameter to `SearchVectorDB`;
    set `SearchQuery.SourceVersion` inside it).
  - Add `sourceVersion` to the `ChunkInfo` built in all three response builders.
- New endpoints:
  - `POST /api/v1/agent/:slug/rag/active-version` (body `audience_type, file_type, version`)
    → `UpsertAgentRAGActiveVersion` (rollback without re-embedding).
  - `GET /api/v1/agent/:slug/rag/active-versions` → list active pointers.
  - `GET /api/v1/agent/:slug/rag/indexed-versions?audience_type=&file_type=` → distinct
    indexed `source_version` values (used by the UI to flag "not indexed" versions).
  - Register all in `server/router/api/v1/v1.go`.

### 6. UI
- **AgentAdmin RAG Search Explorer** (`agentAdmin.ts` `searchRAG`, `AgentAdmin.tsx`): add a
  **file-type selector** (`kb`/`policy`/`script`) + **version dropdown** (populated from the
  existing `/files/{audience}/{fileType}/versions` endpoint, cross-referenced with
  `/rag/indexed-versions` to show a **"not indexed" badge** for versions lacking chunks);
  send `file_type` + `version` to `/rag/search`. Show `sourceVersion` on each result.
- **RAG Stats → Search Testing** (`ragStats.ts`, `RagStats.tsx`): same version selector;
  send `version` to `/admin/rag/search`.
- **Active-version management**: a control to set the active version per file type (calls the
  new endpoint) — enables rollback.
- **Change A (UI)**: remove `<Option value="all">All</Option>` from the Rebuild Index `Select`;
  change its default state from `"all"` to `"internal"`.
- **Change B (UI)**: in the Agent Admin upload-success path (`agentAdmin.ts` upload method +
  `AgentAdmin.tsx` upload handler), show a non-blocking toast such as
  *"Upload saved. Click Rebuild Index to update the RAG index."* (optionally include the new `vN`).

### 7. Verification (expanded for edge cases)
- `task build:rag` (and a `go build ./...` covering non-rag + Postgres/MySQL stubs) plus
  existing lint/typecheck.
- Reindex tenant 12 (`bchat`) external `kb` twice → v1 → v2; query v1 vs v2 via Explorer;
  assert results differ and are isolated by `source_version`.
- **Cutover**: confirm pre-versioning chunks (`NULL`/`0`/`1`) are purged on first versioned
  reindex and not mixed into v1.
- **Retention**: index a 6th version and confirm v1 is purged via `DeleteByVersion`.
- **Rollback**: `POST /rag/active-version` to an older version switches Explorer results
  without re-embedding.
- **Resume**: kill a reindex mid-flight and confirm `ReindexCheckpoint` (now keyed by
  `audience+fileType+version`) resumes correctly without duplicating the in-flight version.
- **Concurrent reindex** of the same `(tenant, audience, fileType)` does not produce duplicate
  versioned chunks (append-only + checkpoint guarding).
- **No-version fallback**: for a `(tenant, audience, fileType)` with no indexed version, a
  search returns an empty result set (not stale chunks).
- **All three drivers**: run the migration + a smoke test on SQLite (dev) and confirm
  Postgres/MySQL compile and the new methods are implemented.

## Files touched (summary)
| File | Change |
|------|--------|
| `store/agent.go` | `ReindexCheckpoint` + `FindReindexCheckpoint` add `FileType`/`Version`; (new) active-version types |
| `store/driver.go` | Interface: active-version methods + (existing) `ReindexCheckpoint` methods |
| `store/db/sqlite/agent.go`, `store/db/postgres/agent.go`, `store/db/mysql/agent.go` | Implement active-version methods + `ReindexCheckpoint` field changes |
| `store/migration/{sqlite,postgres,mysql}/NN__rag_active_versions.sql` | New table + index |
| `server/router/api/v1/agent/vectordb.go` | `SearchQuery.SourceVersion`; `VectorDB` interface `DeleteByVersion` |
| `server/router/api/v1/agent/vectordb_lance.go` | `buildFilter` add `source_version`; `LanceVectorDB.DeleteByVersion`; cutover filter `NULL/0/1` |
| `server/router/api/v1/agent/vectordb_nolance.go` | `DeleteByVersion` stub |
| `server/router/api/v1/agent/vectordb_pool.go` | `DeleteByVersion` delegation on `TenantVectorDBPool` |
| `server/router/api/v1/agent/service.go` | `ReindexAllContent`/`ReindexTenantContent`/`ReindexTenantContentWithResume` version threading + append-only + retention + cutover; `ReindexCheckpoint` fields; `resolveQueryVersion`; `SearchVectorDB` `sourceVersion` param |
| `server/router/api/v1/agent/handlers.go` | Update `HandleTestRAGSearch`, `HandleTenantRAGSearch`, `HandleRAGSearch`; remove auto-reindex in `importFiles`; add active-version + indexed-versions endpoints; `RAGSearchRequest` + `ChunkInfo` fields |
| `server/router/api/v1/v1.go` | Register new endpoints |
| `web/src/store/v2/agentAdmin.ts`, `web/src/pages/AgentAdmin.tsx` | File-type + version picker; remove "All" option + default `internal`; post-upload reminder; active-version control |
| `web/src/store/v2/ragStats.ts`, `web/src/pages/RagStats.tsx` | Version selector for Search Testing |

## Status
Plan v2 written, addressing `plan_review.md` (C1–C4, S1–S5, N1–N4) plus Changes A & B.
**No application code has been written yet** — awaiting approval to implement.
