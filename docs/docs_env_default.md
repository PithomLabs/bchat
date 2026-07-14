# Reindex Stuck Investigation: EMBEDDING_BATCH_SIZE=1 + EMBEDDING_TIMEOUT=10m

## Symptoms

A reindex of the **evpn** tenant (1.4MB KB file, `internal` audience) appeared stuck — the API request never returned, and no vector index was built.

## Evidence (from DB + process inspection)

| Check | Result |
|-------|--------|
| `agent_reindex_checkpoints` for tenant 13 | **Empty** — no checkpoint created |
| `agent_rag_active_versions` for tenant 13 | **Empty** — no successful index |
| `build/data/lancedb/13*` | **Does not exist** — no chunks stored |
| Other tenant LanceDB dirs (7-12) | Exist and populated |
| Server responsiveness (port 8081) | Responsive to new requests |
| Process state (PID 31347) | `futex_wait_queue` (sleeping on API call) |
| Process uptime | ~11 minutes at time of inspection |

## Root Cause

Two environment variables combined to stall the reindex:

### 1. `EMBEDDING_BATCH_SIZE=1`

The default is **200**, but the `run:rag` task in `Taskfile.yml` line 109 **overrides** it to `1` via inline env var. This overrides the `.env` value of `200` because inline vars take precedence over sourced `.env`.

**Impact:** Each batch contains only 1 chunk. With ~183 chunks (from a 1.4MB file at 4096 tokens/chunk), this means **183 sequential API calls** instead of 1-2 calls.

### 2. `EMBEDDING_TIMEOUT=10m`

The default is `180s`, but `.env` line 63 sets it to `10m` (10 minutes).

**Impact:** Each individual embedding API call waits **10 minutes** before timing out. Combined with batch size 1, worst case: 183 × 10 min = **30 hours** to complete.

### Why no checkpoint?

The reindex flow creates a checkpoint **after** the preflight validation step and chunking, but **before** the batched insert. The validation step (`Validate()` in `vectordb_lance.go:600`) makes a single preflight embedding call. With `EMBEDDING_TIMEOUT=10m`, this call hung for 10 minutes. No checkpoint was ever created because the reindex never progressed past validation.

## Fix Locations

| File | Line | Current | Change to |
|------|------|---------|-----------|
| `Taskfile.yml` | 109 | `EMBEDDING_BATCH_SIZE=1` | Remove the inline override entirely (`.env` has `200`) or change to `200` |
| `.env` | 63 | `EMBEDDING_TIMEOUT=10m` | `EMBEDDING_TIMEOUT=180s` |

Also apply the same `Taskfile.yml` fix to variant Taskfiles that have the same override:

| File | Line | Change |
|------|------|--------|
| `Taskfile_pg.yml` | 89 | Remove `EMBEDDING_BATCH_SIZE=1` inline override |
| `Taskfile1.yml` | 109 | Same |
| `Taskfile2.yml` | 109 | Same |
| `Taskfile_nub.yml` | 109 | Same |

## Recovery Steps

1. Apply the env var fixes above
2. Kill the stuck process (`kill <PID>`)
3. Restart: `task run:rag`
4. Trigger reindex via UI or API
5. The timed-out attempt will eventually persist a `failed` checkpoint via the detached 5s context in `ReindexTenantContentWithResume` — subsequent attempts can use `?resume=true`

## Environment Data (from running process)

```
EMBEDDING_BATCH_SIZE=1       # from Taskfile.yml inline override
EMBEDDING_TIMEOUT=10m        # from .env
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
RAG_MAX_CHUNK_TOKENS=4096
RAG_PIPELINE_ENABLED=true
RAG_STARTUP_REINDEX_DISABLED=true
FORCE_REINDEX_ON_STARTUP=false
LANCEDB_STORAGE_PROVIDER=local
OPENROUTER_API_KEY=sk-or-v1-...
```

This combination made the reindex process **~180× slower** than with default settings (batch size 200 ÷ batch size 1 = 200× more API calls, each with a 3.3× longer timeout).
