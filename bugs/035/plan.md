# Plan 035 — Versioned RAG Index (per-file-type, active-pointer + retention)

## Problem / Context (why this exists)

The bchat RAG pipeline lets a tenant upload `KB.MD`, `POLICY.MD`, and `SCRIPT.MD`
files that are chunked and embedded into a LanceDB vector index, then queried by the
Agent Admin **RAG Search Explorer** and the RAG Stats **Search Testing** surfaces.

Today the documents themselves are versioned in the database
(`agent_source_files.version` auto-increments per `tenant+audience+file_type`, with a
version-history + restore UI), but **the vector index is not version-aware**:

- `SearchQuery` (`server/router/api/v1/agent/vectordb.go:134`) and `buildFilter`
  (`server/router/api/v1/agent/vectordb_lance.go:1045`) have **no version filter** —
  a search returns the union of *all* chunks for an audience, regardless of which
  document version produced them.
- The chunk schema does carry `source_version` (`vectordb_lance.go:211`), and the
  chunker sets it (`chunker.go:374`), **but reindex hardcodes `sourceVersion=1`** at all
  six `ChunkMarkdownContent` call sites (`service.go:350,354,479,483,807,811`), so the
  real document version is never recorded.
- Reindex **wipes then replaces** the whole audience: `Delete(ctx, tenantID, audienceType)`
  (`service.go:828`). When the audience is `"all"`, the delete builds
  `audience_type='all'` which matches zero rows, so reindexing **appends duplicates**
  instead of replacing (the "Rebuild Index → All" bug).
- The two search handlers (`HandleTestRAGSearch` `handlers.go:4744`,
  `HandleTenantRAGSearch` `handlers.go:4880`) and their UIs
  (`agentAdmin.ts:1071`, `ragStats.ts:141`) send only `query/audience/top_k` — there is
  no way to target a specific document version.

### Consequence

A user cannot reliably ask "what does KB.md **version 3** say about X?" nor compare
retrieval across KB versions, nor roll back the *index* to a prior KB without
re-embedding. The versioning that exists at the document layer is invisible at query
time, and reindexing destroys history.

### Goal

Make the RAG index **versioned** so the Explorer and Search Testing query against a
specific versioned document (e.g. `KB.md` v3), with retained history, an explicit
"active version" pointer (for instant rollback without re-embedding), and automatic
retention to bound index size.

## Locked design decisions (from Q&A)

1. **Granularity** — versioning is **per file type** (`kb` / `policy` / `script`),
   matching `agent_source_files.version` (auto-incremented per `tenant+audience+file_type`).
2. **Selection model** — a **stored "active version" pointer** per
   `(tenant, audience, file_type)`, defaulting to latest on (re)index; the Explorer /
   Testing UIs may **override per query** with an explicit version.
3. **Retention** — keep the **last N = 5** versions; automatically purge older ones.
4. **Scope** — **full**: backend version filter + UI picker + active-pointer/rollback
   action + retention cleanup.

## Current-state gaps (grounded in code)

| Area | Location | Issue |
|------|----------|-------|
| Search filter | `vectordb.go:134`, `vectordb_lance.go:1045` | No `source_version` filter (vector + FTS both use `buildFilter`) |
| Version recording | `service.go:350,354,479,483,807,811` | `ChunkMarkdownContent` called with hardcoded `1` |
| Reindex semantics | `service.go:828` | Audience `"all"` → no-op delete → duplicates; whole-audience wipe loses history |
| Active-version storage | (none) | No table/field for the active pointer |
| Search requests | `handlers.go:4535`, `handlers.go:4881` | No `version` / `file_type` fields |
| UI | `agentAdmin.ts:1071`, `ragStats.ts:141` | Send only `query/audience/top_k` |

## Proposed changes

### 1. Data model
- **New table** `agent_rag_active_versions(tenant_id, audience_type, file_type, version)`
  + index on `(tenant_id, audience_type, file_type)`.
  Migration: `store/migration/sqlite/NN__rag_active_versions.sql`.
- `SearchQuery` (`vectordb.go:134`): add `SourceVersion *int32`.
- `ChunkInfo` (`handlers.go:4521`): add `SourceVersion int32` (provenance in results).
- `RAGSearchRequest` (`handlers.go:4535`): add `SourceVersion *int32` + optional
  `FileType string`. Agent-Admin search body (`handlers.go:4881`): add `version` +
  `file_type`.

### 2. Vector DB layer (`vectordb.go` + `vectordb_lance.go`)
- Add to `VectorDB` interface (`vectordb.go:30`):
  `DeleteByVersion(ctx, tenantID int32, audienceType, fileType string, version int32) error`.
  Implement in `LanceVectorDB`, `MemoryVectorDB` (`:348`), `NoOpVectorDB` (`:640`).
  - Lance impl: `DELETE` where `tenant_id=? AND audience_type=? AND content_type=? AND source_version=?`.
- `buildFilter` (`vectordb_lance.go:1045`): append `source_version = N` when
  `query.SourceVersion != nil`. Feeds both vector and FTS paths — no separate FTS change.
- Fix the `"all"` delete: add `DeleteAllAudiences(ctx, tenantID)` (loops `internal` /
  `external`). Keep `Delete(ctx, tenantID, audienceType)` for single-audience surgical
  deletes (used by retention / rollback).

### 3. Reindex (`service.go`)
- Thread the **real version** at the six `ChunkMarkdownContent` sites: pass `f.Version`
  instead of `1`. Restructure the `audienceFiles` grouping (`service.go:793-799`) to
  preserve `f.Version` per `(audience, fileType)`.
- **Change semantics**: do **not** wipe the whole audience. Insert the new versioned
  chunks (tagged with `f.Version`); after insert, **set the active pointer** = new
  version for that `(tenant, audience, fileType)`.
- **Retention** (after insert): list versions for `(tenant, audience, fileType)`; if
  count > 5, `DeleteByVersion` the oldest beyond 5.
- **Cutover**: on the first versioned reindex for a `(tenant, audience, fileType)`,
  delete existing stale `source_version=1` chunks for that key (one-time clean slate),
  then insert as the current version. Subsequent reindexes append.
- Update checkpoint logic (`ReindexCheckpoint`) to key on `audience+fileType+version` so
  resume stays correct.

### 4. Active-version store + helper
- Store interface (`driver.go`): `UpsertAgentRAGActiveVersion`,
  `GetAgentRAGActiveVersion(ctx, tenantID, audience, fileType) (*int32, error)`,
  `ListAgentRAGActiveVersions(tenantID)`, `DeleteAgentRAGActiveVersion(...)`.
- Service helper `resolveQueryVersion(tenantID, audience, fileType, requested *int32) int32`:
  if `requested` set → use it; else look up active pointer; else fall back to latest
  `agent_source_files.version`.

### 5. Endpoints (`handlers.go` + `v1.go`)
- Existing search handlers (`HandleTestRAGSearch` `:4744`, `HandleTenantRAGSearch` `:4880`):
  read `version` / `file_type`; set `SearchQuery.ContentTypes=[fileType]` (optional) and
  `SearchQuery.SourceVersion = resolveQueryVersion(...)`. Add `sourceVersion` to the
  `ChunkInfo` built in both response builders.
- New: `POST /api/v1/agent/:slug/rag/active-version` (body `audience_type, file_type,
  version`) → `UpsertAgentRAGActiveVersion` (rollback without re-embedding).
  `GET /api/v1/agent/:slug/rag/active-versions` → list.
  Register both in `server/router/api/v1/v1.go`.

### 6. UI
- **AgentAdmin RAG Search Explorer** (`agentAdmin.ts:1071`, `AgentAdmin.tsx`): add a
  **file-type selector** (`kb`/`policy`/`script`) + **version dropdown** (populated from
  the existing `/files/{audience}/{fileType}/versions` endpoint at `agentAdmin.ts:582`);
  send `file_type` + `version` to `/rag/search`. Show `sourceVersion` on each result.
- **RAG Stats → Search Testing** (`ragStats.ts:141`, `RagStats.tsx`): add the same
  version selector; send `version` to `/admin/rag/search`.
- **Active-version management**: a small control to set the active version per file type
  (calls the new endpoint) — enables rollback.

### 7. Verification
- `task build:rag`; reindex tenant 12 (`bchat`) external `kb` twice to create v1 → v2.
- Query v1 vs v2 via Explorer; assert results differ and are isolated by `source_version`.
- Confirm retention purges v1 once a 6th version is indexed.
- Confirm `active-version` set/rollback switches Explorer results without re-embedding.
- Run `go build ./...` plus existing lint/typecheck before finishing.

## Risks / notes
- The local index only has 30 internal chunks; the external 14MB KB lives in S3 and is
  unreachable locally (`LANCEDB_STORAGE_PROVIDER=s3` in `.env`). Cutover/verification of
  the full external corpus requires either local S3 access or pointing
  `LANCEDB_STORAGE_PROVIDER=local` + reindex (the earlier abandoned step). Plan assumes
  reindex runs where storage is reachable.
- Per-content-type version divergence: if KB and POLICY versions differ and a query spans
  both without a file-type selector, the same `SourceVersion` applies to all selected
  types. The UI file-type selector (single type per query) avoids this; multi-type
  mixed-version queries are out of scope for v1.

## Addendum — two additional changes (agreed via Q&A)

These are folded into the versioned RAG plan above. All four Q&A decisions were taken
with the recommended options.

### Change A — Remove the "All" option under Rebuild Index (Agent Admin)

**Decisions:**
- The Rebuild Index dropdown **defaults to `internal`** (matches `service.go:729` fallback).
- The backend `audience_type=all` on `POST /:slug/reindex` is **kept but made safe** — under
  the versioned reindex (Plan §3) it loops audiences and appends a new versioned index for each,
  with no broad `Delete`. No separate backend removal.

**Files / lines:**
- `web/src/pages/AgentAdmin.tsx:1208` — delete `<Option value="all">All</Option>`.
- `web/src/pages/AgentAdmin.tsx:136` — change `useState<string>("all")` → `useState<string>("internal")`.
- `server/router/api/v1/agent/handlers.go:1184-1187` — keep the `"all"` default; it becomes
  safe once Plan §3's per-audience append (no broad wipe) lands. No code change required here.

### Change B — Remove auto-reindex on file upload

**Decisions:**
- **Fully manual:** an upload creates a new document version but does **not** touch the RAG
  index. The index only changes when the user manually clicks **Rebuild Index** (which, per
  Plan §3, creates a new index version and updates the active pointer).
- Add a **non-blocking reminder hint** after a successful upload.

**Files / lines:**
- `server/router/api/v1/agent/handlers.go:1559-1565` — remove the
  `ReindexTenantContentWithResume(...)` block inside `importFiles`. This single removal
  disables auto-reindex for **both** `HandleImport` (`:1402`) and `HandleImportSingleFile`
  (`:1038`), since both funnel through `importFiles`. (`HandleImportScript` `:3922` is
  unaffected — it has no RAG reindex.)
- **UI hint:** in the Agent Admin upload-success path (`web/src/store/v2/agentAdmin.ts` upload
  method + `web/src/pages/AgentAdmin.tsx` upload handler), show a non-blocking toast such as
  *"Upload saved. Click Rebuild Index to update the RAG index."* (optionally include the new `vN`).

### How Changes A & B integrate with the versioned RAG plan
- Change B makes **manual Rebuild Index the sole trigger for a new index version** — exactly the
  versioned, history-preserving flow in Plan §3.
- Change A's "All" removal + safe backend keeps reindex **per-audience**, which is required for
  per-file-type versioning (Plan §1–§3).

### Files touched (summary)
| File | Change |
|------|--------|
| `web/src/pages/AgentAdmin.tsx:136,1208` | Remove "All" option; default `internal` |
| `server/router/api/v1/agent/handlers.go:1559-1565` | Remove auto-reindex in `importFiles` |
| `web/src/store/v2/agentAdmin.ts` + `AgentAdmin.tsx` upload path | Add post-upload "rebuild" reminder toast |
| `handlers.go:1184-1187` | (no code change; relies on Plan §3 safety) |

## Status
Plan complete (versioned RAG + Changes A & B). **No code changes yet** — awaiting approval to implement.
