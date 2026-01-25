# Debugging "Rebuild Index" - Zero Chunks Issue

## Overview

This guide helps diagnose why clicking "Rebuild Index" in Agent Admin results in zero chunks appearing in RAG Stats.

---

## Diagnostic Flowchart

```
"Rebuild Index" clicked
         │
         ▼
┌─────────────────────────────────────┐
│ What did the toast message show?    │
└─────────────────────────────────────┘
         │
    ┌────┴────┬─────────────┐
    ▼         ▼             ▼
 Error    Success,       Success,
message   chunks: 0     chunks: N>0
    │         │             │
    ▼         ▼             ▼
  See       See          See
Cause 5   Cause 1-3    Cause 4
```

---

## Root Causes

### Cause 1: No Source Files in Database

**Symptom:** `Successfully reindexed 0 chunks`

The reindex function loads KB/Policy files from the database:

```go
// server/router/api/v1/agent/service.go:218-224
files, err := s.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
    TenantID: &tenantID,
})
```

If the tenant has no uploaded KB.MD or POLICY.MD files, `files` is empty → 0 chunks.

**Verification:**
```bash
sqlite3 build/data/memos_dev.db "
SELECT
  t.slug,
  sf.audience_type,
  sf.file_type,
  LENGTH(sf.content) as content_length
FROM agent_source_files sf
JOIN agent_tenant t ON sf.tenant_id = t.id
WHERE t.slug = 'iglesia-ni-cristo';
"
```

**Expected output (if files exist):**
```
iglesia-ni-cristo|external|kb|15234
iglesia-ni-cristo|external|policy|8456
```

**If empty:** Upload KB.MD and POLICY.MD via Agent Admin "Import" tab.

---

### Cause 2: Parsing Fails Silently

**Symptom:** `Successfully reindexed 0 chunks` (files exist but 0 chunks)

Parser errors are **silently ignored** in the reindex flow:

```go
// server/router/api/v1/agent/service.go:242-246
if kbContent, ok := fileMap["kb"]; ok && kbContent != "" {
    parsedKB, _ = s.parser.ParseKB(kbContent, tenantID, audience)  // Error IGNORED
}
if policyContent, ok := fileMap["policy"]; ok && policyContent != "" {
    parsedPolicy, _ = s.parser.ParsePolicy(policyContent, tenantID, audience)  // Error IGNORED
}
```

If KB.MD has invalid format, `parsedKB` will be `nil` → no chunks created.

**Verification:**
```bash
# Extract KB content and check format
sqlite3 build/data/memos_dev.db "
SELECT content FROM agent_source_files
WHERE tenant_id = (SELECT id FROM agent_tenant WHERE slug = 'iglesia-ni-cristo')
AND file_type = 'kb' AND audience_type = 'external';
" > /tmp/kb_content.md

# Check for required section headers
grep -E "^## (SERVICES|FAQs|EXCLUSIONS|COVERAGE)" /tmp/kb_content.md
```

**Required KB.MD format:**
```markdown
## SERVICES
### SERVICE_CODE_1
Name: Service Name
Description: Service description
...

## FAQs
### FAQ: Question here?
Answer here

## EXCLUSIONS
- Exclusion item 1
- Exclusion item 2
```

---

### Cause 3: Empty Parsed Content

**Symptom:** `Successfully reindexed 0 chunks` (files valid but empty)

Even if parsing succeeds, if the KB has no services, FAQs, exclusions, etc., chunking produces 0 chunks:

```go
// server/router/api/v1/agent/chunker.go:63-90
func (c *Chunker) ChunkKBContent(kb *ParsedKB, ...) []DocumentChunk {
    var chunks []DocumentChunk

    // Chunks are created from these arrays:
    // - kb.Services
    // - kb.FAQs
    // - kb.Exclusions
    // - kb.CoverageAreas

    // If all are empty, chunks slice is empty
    return chunks
}
```

**Verification:**
```bash
# Check what sections exist in KB
sqlite3 build/data/memos_dev.db "
SELECT content FROM agent_source_files
WHERE tenant_id = (SELECT id FROM agent_tenant WHERE slug = 'iglesia-ni-cristo')
AND file_type = 'kb';
" | head -100
```

---

### Cause 4: Memory Storage Lost

**Symptom:** Reindex shows `chunks: N` (N > 0), but RAG Stats shows 0

If using `LANCEDB_STORAGE_PROVIDER=memory` (the default), data is volatile.

**Verification:**
Check RAG Stats page header → "Storage Provider" value.

If `memory`:
- Data exists only in server RAM
- Lost on server restart
- Could be lost if multiple server instances

**Fix:** Use persistent storage:
```bash
export LANCEDB_STORAGE_PROVIDER=local
# Restart server, then reindex again
```

---

### Cause 5: RAG Pipeline Disabled (NoOpVectorDB)

**Symptom:** Error message: `Reindex failed: RAG pipeline disabled (using NoOpVectorDB)`

If `RAG_PIPELINE_ENABLED` was not set to `"true"` at server startup:

```go
// server/router/api/v1/agent/vectordb.go:136-138
if !config.Enabled {
    slog.Info("RAG pipeline disabled, using no-op vector database")
    return NewNoOpVectorDB(), nil
}
```

The reindex function explicitly checks for this:

```go
// server/router/api/v1/agent/service.go:206-208
if _, isNoOp := s.vectorDB.(*NoOpVectorDB); isNoOp {
    return 0, fmt.Errorf("RAG pipeline disabled (using NoOpVectorDB)")
}
```

**Fix:**
```bash
export RAG_PIPELINE_ENABLED=true
# Restart server
task run:rag
# Then reindex
```

---

## Step-by-Step Debugging Guide

### Step 1: Check RAG Stats Page Status

Navigate to RAG Stats page and check the header:

| Field | Expected Value | Problem if Different |
|-------|----------------|---------------------|
| Enabled | Yes | RAG disabled, restart with `RAG_PIPELINE_ENABLED=true` |
| Storage Provider | local or s3 | `memory` loses data on restart |
| Embedding Provider | mock, local, or openrouter | Configuration issue |

### Step 2: Check Browser Network Tab

1. Open DevTools (F12) → Network tab
2. Click "Rebuild Index"
3. Find the POST request to `/api/v1/agent/{slug}/reindex`
4. Check the response:

```json
// Success with chunks:
{"success":true,"chunks":45,"message":"Successfully reindexed 45 chunks"}

// Success but no content:
{"success":true,"chunks":0,"message":"Successfully reindexed 0 chunks"}

// RAG disabled:
{"message":"Reindex failed: RAG pipeline disabled (using NoOpVectorDB)"}
```

### Step 3: Check Server Logs

Watch server console during reindex:

```bash
# Good - RAG enabled, files found:
INFO Starting RAG reindex for tenant tenantID=5 tenant=iglesia-ni-cristo
INFO Reindexed content for tenant tenantID=5 tenant=iglesia-ni-cristo audience=external chunks=45
INFO RAG reindex completed for tenant tenantID=5 tenant=iglesia-ni-cristo totalChunks=45

# Bad - No "Reindexed content" log:
INFO Starting RAG reindex for tenant tenantID=5 tenant=iglesia-ni-cristo
INFO RAG reindex completed for tenant tenantID=5 tenant=iglesia-ni-cristo totalChunks=0
# ↑ No "Reindexed content" means no files or parsing failed
```

### Step 4: Database Verification

```bash
# Check tenant exists
sqlite3 build/data/memos_dev.db "SELECT id, slug, company_name FROM agent_tenant WHERE slug = 'iglesia-ni-cristo';"

# Check source files exist
sqlite3 build/data/memos_dev.db "
SELECT sf.id, sf.audience_type, sf.file_type, LENGTH(sf.content) as size
FROM agent_source_files sf
JOIN agent_tenant t ON sf.tenant_id = t.id
WHERE t.slug = 'iglesia-ni-cristo';
"

# If no rows returned, files need to be uploaded
```

### Step 5: Validate File Content

```bash
# Export KB content
sqlite3 build/data/memos_dev.db "
SELECT content FROM agent_source_files
WHERE tenant_id = (SELECT id FROM agent_tenant WHERE slug = 'iglesia-ni-cristo')
AND file_type = 'kb' LIMIT 1;
" > /tmp/kb.md

# Check structure
head -50 /tmp/kb.md

# Look for section headers
grep -n "^##" /tmp/kb.md
```

---

## Common Scenarios and Fixes

### Scenario A: New Tenant, Never Uploaded Files

**Symptoms:**
- Tenant appears in list
- No KB/Policy files in database
- Reindex returns 0 chunks

**Fix:**
1. Go to Agent Admin → select tenant → Import tab
2. Upload KB.MD and POLICY.MD files
3. Click "Rebuild Index"

### Scenario B: Files Uploaded Before RAG Enabled

**Symptoms:**
- Files exist in database
- Server was started without `RAG_PIPELINE_ENABLED=true`
- Reindex returns error about NoOpVectorDB

**Fix:**
```bash
# Restart with RAG enabled
export RAG_PIPELINE_ENABLED=true
task run:rag

# Then reindex
```

### Scenario C: Server Restarted, Lost Memory Storage

**Symptoms:**
- Reindex previously worked
- After server restart, chunks are 0

**Fix:**
```bash
# Use persistent storage
export LANCEDB_STORAGE_PROVIDER=local
task run:rag

# Reindex all tenants
export REINDEX_RAG=true
task run:rag
```

### Scenario D: Invalid KB/Policy Format

**Symptoms:**
- Files exist with content
- Reindex returns 0 chunks
- No parse errors in logs (silent failure)

**Fix:**
1. Export and examine files (see Step 5 above)
2. Fix format to match expected structure
3. Re-upload via Agent Admin
4. Reindex

---

## Code Reference

### Key Files

| File | Line | Function |
|------|------|----------|
| `service.go` | 198-288 | `ReindexTenantContent()` |
| `service.go` | 206 | NoOpVectorDB check |
| `service.go` | 218 | Load source files |
| `service.go` | 242-246 | Parse KB/Policy (errors ignored) |
| `chunker.go` | 63-90 | `ChunkKBContent()` |
| `vectordb.go` | 136 | NoOpVectorDB creation |
| `vectordb.go` | 404-421 | `MemoryVectorDB.Stats()` |

### Reindex Flow

```
HandleReindexTenant (handlers.go:555)
    │
    ▼
ReindexTenantContent (service.go:200)
    │
    ├─ Check vectorDB != nil (line 201)
    ├─ Check NOT NoOpVectorDB (line 206)
    │
    ▼
ListAgentSourceFiles (line 219)
    │
    ├─ If empty → return 0 chunks
    │
    ▼
Group files by audience (line 227-233)
    │
    ▼
For each audience:
    │
    ├─ ParseKB (line 243) - errors ignored!
    ├─ ParsePolicy (line 246) - errors ignored!
    │
    ├─ If both nil → continue (no chunks)
    │
    ▼
    vectorDB.Delete (line 254)
    │
    ▼
    ChunkKBContent + ChunkPolicyContent (lines 261-266)
    │
    ├─ If chunks empty → continue
    │
    ▼
    vectorDB.Insert (line 274)
    │
    ▼
Return totalChunks
```

---

## Recommendations

### 1. Add Parse Error Logging

The current code silently ignores parse errors. Consider adding logging:

```go
// Instead of:
parsedKB, _ = s.parser.ParseKB(kbContent, tenantID, audience)

// Use:
parsedKB, err := s.parser.ParseKB(kbContent, tenantID, audience)
if err != nil {
    slog.Warn("Failed to parse KB", "tenantID", tenantID, "audience", audience, "error", err)
}
```

### 2. Return Parse Errors in API Response

Modify reindex to report parsing issues:

```go
return c.JSON(http.StatusOK, map[string]interface{}{
    "success":      true,
    "chunks":       chunks,
    "parseErrors":  parseErrors,  // Add this
    "message":      fmt.Sprintf("Successfully reindexed %d chunks", chunks),
})
```

### 3. Validate Files Before Reindex

Add a validation endpoint that checks file format without reindexing.

---

## Related Documentation

- [DOCS_RAG_REINDEX.MD](./DOCS_RAG_REINDEX.MD) - General reindexing guide
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables
