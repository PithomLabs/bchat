# LanceDB Versioning Architecture

**Status:** Reference Documentation
**Created:** 2026-07-15
**Related:** `docs/bugs/038/plan.md`

---

## 1. Overview

The RAG (Retrieval-Augmented Generation) pipeline uses LanceDB as its vector database. Each tenant's vectors are stored in an isolated LanceDB instance. The system implements a multi-layer versioning architecture that allows:

- Multiple versions of indexed content to coexist in LanceDB
- Instant rollback to any prior indexed version without re-embedding
- Retention policies to manage storage growth
- Atomic pointer updates for version cutover

---

## 2. Tenant-Isolated Storage

Each tenant gets its own LanceDB instance with a dedicated directory or S3 prefix.

### Local Storage

```
build/data/lancedb/<tenantID>/
```

Resolved by `resolveLocalTarget()` in `vectordb.go:251-253`:

```go
func resolveLocalTarget(config *VectorDBConfig, tenantID int32) string {
    return fmt.Sprintf("%s/%d", config.LocalPath, tenantID)
}
```

### S3 Storage

```
s3://<bucket>/<prefix>/<tenantID>/
```

Resolved by `resolveStorageTarget()` in `vectordb.go:237-247`:

```go
prefix := global.S3Prefix
if prefix == "" {
    prefix = "lancedb"
}
if override != nil && override.Prefix != "" {
    prefix = override.Prefix
}
uri = fmt.Sprintf("s3://%s/%s/%d", resolved.S3Bucket, prefix, tenantID)
```

### Per-Tenant S3 Override

Tenants can have their own S3 bucket/prefix/credentials via `TenantConfig.VectorDBS3Override` (JSON in the database), enabling complete storage isolation from the global S3 configuration.

### Connection Pool

The `TenantVectorDBPool` (`vectordb_pool.go:15-98`) maintains a map of `int32 → VectorDB` (tenant ID to VectorDB instance). It lazily creates per-tenant LanceDB connections via `getOrCreate()`.

---

## 3. LanceDB Table Structure

Within a tenant's LanceDB database, data is stored in tables named by embedding dimension:

```go
func getTableNameForDimension(dim int) string {
    return fmt.Sprintf("kb_documents_%d", dim)
}
```

Examples:
- `kb_documents_1536` — OpenRouter text-embedding-3-small
- `kb_documents_384` — local sentence-transformers

### Arrow Schema

Each table stores `DocumentChunk` rows with these key columns:

| Column | Type | Purpose |
|--------|------|---------|
| `tenant_id` | Int32 | Tenant identifier (redundant but defense-in-depth) |
| `audience_type` | String | "internal" or "external" |
| `content_type` | String | "kb", "policy", or "script" |
| `source_version` | Int32 | Version of the source file this chunk was created from |
| `content` | String | The chunk text |
| `embedding` | List(Float) | Vector embedding |

### Query Filtering

Searches filter by `tenant_id`, `audience_type`, `content_type`, and optionally `source_version`:

```go
filter := fmt.Sprintf("tenant_id = %d AND audience_type = '%s' AND content_type = '%s'",
    query.TenantID, query.AudienceType, query.ContentType)
```

---

## 4. Four-Layer Version Tracking

### Layer 1: Source File Versions (SQLite)

**Table:** `agent_source_files`

Every upload creates a new row with an auto-incrementing version number per `(tenant_id, audience_type, file_type)` partition.

**Version assignment** (`store/db/sqlite/agent.go:1116-1136`):

```go
func (d *DB) UpsertAgentSourceFile(ctx context.Context, file *store.AgentSourceFile) (*store.AgentSourceFile, error) {
    var nextVersion int32 = 1
    err := d.db.QueryRowContext(ctx, `
        SELECT COALESCE(MAX(version), 0) + 1
        FROM agent_source_files
        WHERE tenant_id = ? AND audience_type = ? AND file_type = ?
    `, file.TenantID, file.AudienceType, file.FileType).Scan(&nextVersion)

    // Always INSERT (never UPDATE) — creates new version row
    _, err = d.db.ExecContext(ctx, `
        INSERT INTO agent_source_files (tenant_id, audience_type, file_type, content, content_hash, version, imported_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `, ...)
}
```

Key properties:
- Pure append-only — never updates existing rows
- `ContentHash` is stored but NOT used for deduplication
- Identical content still gets a new version

### Layer 2: Indexed Source Version (LanceDB Metadata)

Each `DocumentChunk` carries a `SourceVersion int32` field (`chunker.go:31`). This is set from the source file version during chunking and stored as the `source_version` Arrow column in LanceDB (`vectordb_lance.go:882`).

This allows multiple versions to coexist in the same LanceDB table — chunks from version 3 and version 5 can both exist, differentiated by `source_version`.

### Layer 3: Active Version Pointer (SQLite)

**Table:** `agent_rag_active_versions`

**Schema** (`store/migration/sqlite/0.31/02__rag_active_versions.sql`):

```sql
CREATE TABLE IF NOT EXISTS agent_rag_active_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    audience_type TEXT NOT NULL,
    file_type TEXT NOT NULL,
    version INTEGER NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rag_active_version_lookup
ON agent_rag_active_versions(tenant_id, audience_type, file_type);
```

**Purpose:** Points to which indexed version is "current" for query purposes per `(tenant_id, audience_type, file_type)`.

**Updated by:**
- Automatic update after successful reindex (`service.go:317-323`, `1075-1082`)
- Manual rollback via admin API (`handlers.go:5994-6057`)

### Layer 4: Query Version Resolution

`resolveQueryVersion` (`service.go:4381-4421`) resolves which version to query:

```
1. Explicit request parameter (if provided, use directly)
   ↓ (if nil)
2. Active-version pointer (from agent_rag_active_versions)
   ↓ (if no pointer set)
3. Latest indexed version from LanceDB (fallback)
   ↓ (if no data)
4. nil (no data exists)
```

This ensures queries always use the correct version, even if no explicit pointer has been set.

---

## 5. Upload Flow

### What Happens on File Upload

1. Admin uploads KB or POLICY file via `HandleImportSingleFile` (`handlers.go:1045`)
2. `importFiles()` is called, which calls `UpsertAgentSourceFile()`
3. A new row is created in `agent_source_files` with version N
4. **RAG reindex is NOT triggered** — explicit comment at `handlers.go:1601-1602`:
   ```go
   // RAG reindex is intentionally NOT triggered on upload. The admin must rebuild
   // the index manually (Rebuild Index) so a new source-file version becomes searchable.
   ```
5. The active-version pointer remains pointing to version N-1
6. Queries continue to use version N-1 until reindex is triggered

### Design Rationale

This two-step process (upload → manual reindex) gives admins control over when the RAG index is updated. They can:
- Upload multiple files before reindexing
- Review content before it becomes searchable
- Choose the audience scope for reindex

---

## 6. Rebuild Index Flow

### What Happens on Rebuild Index

When admin clicks "Rebuild Index", the system calls `ReindexTenantContentWithResume` (`service.go:747`):

1. **Read latest source files:**
   ```go
   files, err := s.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
       TenantID:     &tenantID,
       LatestOnly:   boolPtr(true),
   })
   ```
   This always returns the latest version per `(audience, file_type)`.

2. **Chunk content with version:**
   ```go
   chunks := s.chunker.ChunkMarkdownContent(content, tenantID, audience, fileType, version, maxChunkTokens)
   ```
   Each chunk gets `source_version = version` embedded in its metadata.

3. **Append to LanceDB:**
   ```go
   s.vectorDB.Insert(ctx, chunks)
   ```
   New versioned chunks are appended to the existing table. Old version data coexists.

4. **Update active-version pointer:**
   ```go
   s.store.UpsertAgentRAGActiveVersion(ctx, &store.AgentRAGActiveVersion{
       TenantID:     tenantID,
       AudienceType: audience,
       FileType:     fileType,
       Version:      version,
   })
   ```

5. **Retention cleanup:**
   ```go
   if len(versions) > 5 {
       for _, v := range versions[:len(versions)-5] {
           s.vectorDB.DeleteByVersion(ctx, tenantID, audience, fileType, v)
       }
   }
   ```
   Keeps only the last 5 indexed versions. Older versions are deleted from LanceDB.

### Key Properties

- **Append-only:** Old version data coexists with new until retention prunes it
- **Atomic pointer:** The active-version pointer update is a single SQLite upsert
- **Idempotent:** Re-running reindex for the same version overwrites with identical data
- **Resumable:** Checkpoints allow resuming from failure without re-embedding completed chunks

---

## 7. Version Rollback (Instant, No Re-embedding)

### How It Works

Rollback is a simple pointer flip in SQLite. No re-embedding is needed because old version data still exists in LanceDB (within the 5-version retention window).

### Backend Endpoints

**List active versions:**
```
GET /api/v1/agent/:slug/rag/active-versions

Response:
{
  "tenantId": 4,
  "activeVersions": [
    {"audience_type": "internal", "file_type": "kb", "version": 3},
    {"audience_type": "internal", "file_type": "policy", "version": 2}
  ]
}
```

**List indexed versions (available for rollback):**
```
GET /api/v1/agent/:slug/rag/indexed-versions

Response:
{
  "tenantId": 4,
  "groups": [
    {"audience_type": "internal", "file_type": "kb", "versions": [1, 2, 3]},
    {"audience_type": "internal", "file_type": "policy", "versions": [1, 2]}
  ]
}
```

**Set active version (rollback):**
```
POST /api/v1/agent/:slug/rag/active-version

Body:
{
  "audience_type": "internal",
  "file_type": "kb",
  "version": 2
}

Response:
{
  "audience_type": "internal",
  "file_type": "kb",
  "version": 2,
  "status": "active"
}
```

### Validation

The backend validates that the requested version actually exists in LanceDB before accepting:

```go
indexed, lerr := h.service.vectorDB.ListIndexedVersions(ctx, tenant.ID, req.AudienceType, req.FileType)
found := false
for _, v := range indexed {
    if v == req.Version {
        found = true
        break
    }
}
if !found {
    return echo.NewHTTPError(http.StatusBadRequest, "version not found in indexed versions")
}
```

### Limitations

- **Retention window:** Rollback only works for versions within the last 5 indexed versions. Older versions are pruned.
- **No cross-type rollback:** You can only rollback within the same (audience, file_type) combination. Rolling back KB does not affect Policy.
- **Pointer-only:** The rollback only flips the active-version pointer. It does not delete or modify any LanceDB data.

---

## 8. Retention Policy

### How It Works

After each reindex, the system enforces a maximum of 5 retained versions per `(tenant_id, audience_type, file_type)`:

```go
// service.go:326-333
if versions, lerr := s.vectorDB.ListIndexedVersions(ctx, tenantID, audience, fileType); lerr == nil && len(versions) > 5 {
    sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
    for _, v := range versions[:len(versions)-5] {
        s.vectorDB.DeleteByVersion(ctx, tenantID, audience, fileType, v)
    }
}
```

### Deletion

Old versions are deleted from LanceDB using a filter:

```go
func (db *LanceVectorDB) DeleteByVersion(ctx context.Context, tenantID int32, audienceType, fileType string, version int32) error {
    filter := fmt.Sprintf("tenant_id = %d AND audience_type = '%s' AND content_type = '%s' AND source_version = %d",
        tenantID, audienceType, fileType, version)
    db.table.Delete(ctx, filter)
}
```

### Implications

- If an admin uploads 6 versions without reindexing, then does a full reindex, only the latest 5 indexed versions will be available for rollback
- The active-version pointer is NOT automatically cleaned up when a version is pruned — it may point to a non-existent version, in which case `resolveQueryVersion` falls through to the LanceDB fallback

---

## 9. Potential Issues

### Issue A: Upload/Reindex Race Condition (Benign)

**Scenario:** Admin uploads version 3, triggers reindex, then uploads version 4 while reindex is running.

**Result:** Reindex completes with version 3, sets active pointer to 3. Version 4 exists in `agent_source_files` but is not indexed.

**Mitigation:** By design — admin must trigger another reindex. The system is explicitly designed for manual reindex workflow.

### Issue B: Dual Upload Race (Minor)

**Scenario:** Two rapid uploads for the same `(tenant, audience, filetype)` could both read the same `MAX(version)`.

**Result:** Two rows with the same version number could exist in `agent_source_files`.

**Mitigation:** No UNIQUE constraint on version. `MAX(version)` still returns correct latest. Deterministic tiebreaking via `imported_at`. No data corruption.

### Issue C: Active Version Pointer Not Auto-Cleaned on Retention Prune

**Scenario:** If retention prunes version 3 but the active pointer still points to version 3.

**Result:** `resolveQueryVersion` falls through to the LanceDB fallback (latest indexed), which is correct. The stale pointer doesn't cause errors.

**Severity:** Very low — self-healing via fallback logic.

---

## 10. Key Files Reference

| File | Line | Purpose |
|------|------|---------|
| `server/router/api/v1/agent/vectordb.go:237-253` | Storage path resolution (S3 + local) |
| `server/router/api/v1/agent/vectordb_pool.go:15-98` | Per-tenant connection pool |
| `server/router/api/v1/agent/vectordb_lance.go:29-32` | Table naming by dimension |
| `server/router/api/v1/agent/vectordb_lance.go:882` | Arrow schema with `source_version` |
| `server/router/api/v1/agent/vectordb_lance.go:1025-1038` | `DeleteByVersion` implementation |
| `server/router/api/v1/agent/vectordb_lance.go:1167-1169` | Query filtering by `tenant_id` |
| `server/router/api/v1/agent/chunker.go:31` | `SourceVersion` field on `DocumentChunk` |
| `server/router/api/v1/agent/service.go:291-337` | `reindexFileVersion` — append-only versioned indexing |
| `server/router/api/v1/agent/service.go:747` | `ReindexTenantContentWithResume` — full reindex flow |
| `server/router/api/v1/agent/service.go:4381-4421` | `resolveQueryVersion` — version selection algorithm |
| `server/router/api/v1/agent/handlers.go:1045` | `HandleImportSingleFile` — upload handler |
| `server/router/api/v1/agent/handlers.go:1601-1602` | Comment: RAG reindex not triggered on upload |
| `server/router/api/v1/agent/handlers.go:5994-6057` | `HandleSetActiveVersion` — rollback endpoint |
| `server/router/api/v1/agent/handlers.go:6061-6093` | `HandleListActiveVersions` — list active versions |
| `server/router/api/v1/agent/handlers.go:6097-6140` | `HandleListIndexedVersions` — list indexed versions |
| `store/agent.go:349-367` | `AgentRAGActiveVersion` struct and filter |
| `store/db/sqlite/agent.go:1116-1136` | `UpsertAgentSourceFile` — version assignment |
| `store/db/sqlite/agent.go:1244-1327` | Active version CRUD (SQLite) |
| `store/db/postgres/agent.go:1154-1232` | Active version CRUD (Postgres) |
| `store/migration/sqlite/0.31/02__rag_active_versions.sql` | Migration for active version table |
