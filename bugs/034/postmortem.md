# Post-Mortem: bchat external reindex — HTTP 400 + missing `agent_integrations`

**Date:** 2026-07-11
**Status:** RESOLVED (both root causes fixed, reindex progressing)
**Components:** `server/router/api/v1/agent` (embedding + LanceDB), `store` (migration), `internal/version`
**Tenant:** `bchat` (id 12), external-audience KB (`combined_files.txt`, ~14.4 MB)

---

## 1. Summary

After running `task build:rag:all` and `task run:rag`, then rebuilding the RAG index for the
external audience of the `bchat` tenant, two distinct failures surfaced during reindex:

1. `HTTP 400: Invalid 'input[0]': maximum input length is 8192 tokens.` — emitted by the
   OpenRouter embedding API.
2. `no such table: agent_integrations` — a SQLite error when the agent code queried a table that
   did not exist.

Both were investigated through source tracing and in-process self-testing, root-caused, and fixed.
The `HTTP 400` was **not a code bug in the current tree** — it came from a **stale in-memory server
process** running an older `build/memos`. The missing table was a **real, separate bug**: the
version constant was never bumped to `0.31`, so the `0.31/` migration directory was never applied.
After the fixes, the external reindex is progressing normally (`10000 / 13402` chunks).

---

## 2. Trigger & Environment

| Item | Value |
|------|-------|
| Build command | `task build:rag:all` |
| Run command | `task run:rag` |
| Action | Rebuilt RAG index for external audience of tenant `bchat` (id 12) |
| Source KB | `combined_files.txt`, ~14,398,036 bytes (~14.4 MB) |
| Embedding provider | OpenRouter `text-embedding-3-small` |
| Model hard input limit | 8192 tokens |
| Run-time env | `EMBEDDING_BATCH_SIZE=1`, `RAG_PIPELINE_ENABLED=true` |
| Database | SQLite `build/data/memos_dev.db` |

---

## 3. Symptoms

### 3.1 The `HTTP 400` error

```
failed to generate embeddings for batch %d: ... embedding provider unavailable:
OpenRouter API error: HTTP 400: {
  "error": {
    "message": "Invalid 'input[0]': maximum input length is 8192 tokens."
  }
}
```

This string was surfaced during the reindex of the external bchat KB.

### 3.2 The missing-table error

```
no such table: agent_integrations
```

Surfaced when the agent code attempted to read/write integration rows for the tenant.

---

## 4. Investigation (debugging narrative)

### 4.1 Confirming the on-disk binary was current

The binary on disk (`build/memos`, mtime `2026-07-11 08:57`) was inspected:

```
strings build/memos | grep -c expandAndValidateBatch   # 2
strings build/memos | grep -c embedWithIsolation       # 3
```

Both Plan 8 symbols were present, so the **on-disk binary was up to date** with the source tree.

### 4.2 Tracing the reindex call path

The external reindex path flows through:

```
HandleReindexTenant            (handlers.go:1157)
  -> ReindexTenantContentWithResume   (service.go:715)   [also service.go:363 / :503 use Insert]
    -> InsertWithCheckpoint           (service.go:884)
      -> processSingleBatch           (vectordb_lance.go:677)
        -> embedWithIsolation         (vectordb_lance.go:695)
          -> doEmbed                  (embedding.go)
```

`embedWithIsolation` **never returns an error** — it skips a bad chunk and continues. Therefore the
current code path **cannot emit the `HTTP 400` string**, because that string is produced only by
`db.embedSvc.Embed(...)` returning an error and the caller wrapping it.

### 4.3 Locating the exact error source

A grep for the error format showed it is emitted at exactly one place:

```
vectordb_lance.go:442   (inside the unguarded Insert, lines 391–473)
```

`Insert` (the unguarded path) calls `db.embedSvc.Embed(ctx, textsToEmbed)` directly and returns:

```go
return fmt.Errorf("failed to generate embeddings for batch %d: %w", batchNum, err)
```

Plan 8 removed `Insert` from the reindex path in favor of `InsertWithCheckpoint` →
`processSingleBatch` → `embedWithIsolation`. So the **only** code that can produce this exact 400 is
the *old* `Insert` — meaning **the running server process was a stale binary** built before Plan 8
landed, still serving requests in memory.

### 4.4 Discovering the missing table

A direct DB inspection confirmed `agent_integrations` was absent:

```
sqlite3 build/data/memos_dev.db ".tables"   # no agent_integrations
sqlite3 build/data/memos_dev.db "SELECT version FROM migration_history ORDER BY rowid DESC LIMIT 1;"  # 0.30.2
```

Yet the migration file existed:

```
store/migration/sqlite/0.31/00__agent_integrations.sql
store/migration/sqlite/0.31/01__agent_events.sql
```

---

## 5. Self-Testing

Because the user instructed "test it yourself before implementing," an in-process harness was used
to exercise the real reindex path with the real OpenRouter API and the real LanceDB instance,
rather than only unit mocks.

### 5.1 In-process reindex harness (throwaway, later deleted)

- **`TestReindexBoundaryRealOpenRouter`** — pushed a single **78,000-token** chunk through the real
  `InsertWithCheckpoint` + real OpenRouter + real LanceDB. Result:

  ```
  Oversized embedding input detected and split tokens=60004 limit=8176
  Embedding input exceeded model limit; splitting origIndex=1 tokens=8179 limit=8176 parts=2
  BOUNDARY OK: InsertWithCheckpoint returned nil despite oversized chunk
  ```

  The oversized chunk was split and embedded; the reindex did **not** abort.

- **`TestReindexExternalBchat`** — ran the real external reindex of tenant `bchat` (id 12) through
  the full pipeline. Result: **41 batches / 1341 chunks, 0 errors, no 400.**

This empirically proved the disk binary (with Plan 8) does not produce the 400, confirming RC1.

### 5.2 Unit tests

- **`TestDoEmbedRecursiveResplit`** (new) — fake provider rejects any input >100 estimated tokens;
  asserts `doEmbedWith` recursively halves `splitLimit` and converges to 3 embeddings.
- **`TestEmbedWithIsolationSkipsFailedItem`** (existing, still green) — one failing chunk is skipped,
  the rest indexed.
- **`TestLanceVectorDBProcessBatchWithRetryShortCircuitsPermanentError`** (updated) — now asserts the
  new contract: when *every* chunk in a batch fails, the batch aborts with
  `ErrEmbeddingProviderUnavailable` (systemic failure), with the embed service called ≥1×.

Full package result:

```
ok  github.com/usememos/memos/server/router/api/v1/agent  6.895s
```

---

## 6. Root Causes

### RC1 — `HTTP 400`: stale server process (NOT a current-tree bug)

The 400 string is emitted exclusively from the unguarded `Insert` at `vectordb_lance.go:442`. Plan 8
removed that path from reindex; the current code uses `InsertWithCheckpoint` → `processSingleBatch`
→ `embedWithIsolation`, which isolates failures and never emits the 400. The error therefore came
from an **older `build/memos` still running in memory** — i.e. the server was not restarted after a
prior rebuild, so it served requests with pre-Plan-8 code.

### RC2 — missing `agent_integrations`: version constant never bumped

`internal/version/version.go` still declared `Version = "0.30.0"`. The migrator's
`GetCurrentSchemaVersion()` only globs `migration/sqlite/<current-minor>/*.sql`, i.e. the `0.30/`
directory. Because the DB's migration history was already at `0.30.2`, `Migrate` computed
`schemaVersion (0.30.2) > latestHistory (0.30.2)` = **false**, so it ran **no** migration. The
entire `0.31/` directory — which creates `agent_integrations` and `agent_events` — was **never
consulted**. The migration files had been added but the version constant was never bumped to match.

---

## 7. Fixes Applied

### 7.1 Recursive, API-authoritative re-split (robustness net) — `embedding.go`

Refactored `doEmbed` into a recursive `doEmbedWith(ctx, texts, splitLimit, depth, embedFunc)`. On an
`isMaxInputLengthError` response from the API, it halves `splitLimit` and recurses
(`doEmbedMaxDepth = 12`). Added `collapseEmbeddings`, the `embedGroup` type, and
`isMaxInputLengthError`. This guarantees no over-length input reaches the API even if the
estimate-based split lands a part just under the limit (the margin is tight: split parts land at
~8179 tokens vs the 8192 hard limit).

### 7.2 Systemically-aware isolation — `vectordb_lance.go`

Both `processSingleBatch` and `Insert` now embed via `embedWithIsolation` and:

- **Skip partial embedding failures** (one bad chunk is dropped, the rest of the batch is indexed)
  — so a single bad chunk can never abort a whole reindex.
- **Abort loudly if *all* chunks in a batch fail** (systemic failure — e.g. misconfigured/down
  provider), returning `ErrEmbeddingProviderUnavailable`. This prevents the silent failure mode
  where every chunk fails and the reindex "succeeds" with an empty index.

### 7.3 Bump version so `0.31` migrations apply — `internal/version/version.go`

```
var Version = "0.31.0"
var DevVersion = "0.31.0"
```

This makes `GetCurrentSchemaVersion()` glob the `0.31/` directory, compute `0.31.2`, which is now
greater than the DB history (`0.30.2`), triggering the incremental migration of `0.31/00` and
`0.31/01`.

### 7.4 Rebuild & restart

- Rebuilt `./build/memos` with `-tags rag` (mtime `2026-07-11 09:33`).
- Killed the stale server process so the new binary serves requests.

---

## 8. Verification

Migration applied on startup:

```
INFO start migration currentSchemaVersion=0.30.2 targetSchemaVersion=0.31.2
INFO end migrate
```

Table now exists:

```
sqlite3 build/data/memos_dev.db ".tables"   # agent_integrations, agent_events present
sqlite3 build/data/memos_dev.db ".schema agent_integrations"   # CREATE TABLE agent_integrations (...)
sqlite3 build/data/memos_dev.db "SELECT version FROM migration_history ORDER BY rowid DESC LIMIT 1;"  # 0.31.2
```

Reindex progress after fixes (external bchat tenant):

```
10000 / 13402 chunks indexed (in progress)
```

No `HTTP 400` and no `no such table: agent_integrations` observed since.

---

## 9. Lessons & Prevention

1. **Always restart the server after a rebuild.** A stale in-memory process masked the Plan 8 fix
   and produced a "bug" that did not exist in the source. Add a deploy step that stops the old
   process before starting the new binary (or rely on the process manager to do so).

2. **Pair every new `migration/<minor>/` directory with a `version.go` bump.** The migrator only
   scans the directory matching the current minor version, so a migration dir added without a
   matching version bump is silently never applied. Consider a CI check that fails if a new
   `migration/sqlite/X.Y/` dir exists without `version.go` being at `X.Y.0+`.

3. **Keep the embedding boundary API-authoritative + isolate failures.** Estimating token counts is
   heuristic; the only ground truth is the API's 400. Enforce the limit at `doEmbed` (recursive
   re-split) and isolate per-chunk failures so one bad chunk can never abort a reindex — while still
   aborting on a *systemic* failure so empty indexes are never silently produced.

4. **Prefer empirical self-testing.** Exercising the real reindex path against the real OpenRouter
   API + real LanceDB (a throwaway in-process harness) was decisive in proving RC1 and validating the
   fix, rather than reasoning about the code alone.
