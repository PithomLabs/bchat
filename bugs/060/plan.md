# Bug 060 — CRDB Vector Search Returns Zero Results in Chat Flow

**Status:** In progress — plan approved (GO-WITH-CHANGES), implementation underway
**Milestone:** bchat-crdb RAG answers are grounded in indexed KB chunks
**Affects:** CockroachDB vector backend only (app `bchat-crdb`)
**No impact:** Postgres / Neon / SQLite / S3-LanceDB deployments

---

## 1. Symptom

In the widget chat for tenant `rgresidences` on `bchat-crdb.fly.dev`, the question
was:

> "what does maria clara give to the leper"

Reply:

> "I don't have the specific scene where Maria Clara gives something to the leper
> (Elias) in the retrieved context provided. The text available only covers the
> Translator's Introduction and early background of the novel..."

Expected: a grounded answer from the indexed chapters (e.g., the scene mid-book).
Observed: the model only "sees" the Translator's Introduction and then moves to
lead capture.

This is not a model quality issue — the model literally only received the first
~6,000 tokens of the Knowledge Base as context (see §3).

## 2. Investigation / Background

### 2.1 How chunks reach CockroachDB

Reindex flow (`POST /api/v1/agent/:slug/reindex` → `HandleReindexTenant`
→ `Service.ReindexTenantContentWithResume`, `service.go:1102`) chunkifies KB /
policy / script files via `ChunkMarkdownContent`.

`ChunkMarkdownContent` stores chunks with:

```go
ContentType: fileType + "_section",   // "kb_section", "policy_section", "bug_section"
AudienceType: audience,               // not persisted in CRDB (no column)
```
> — `chunker.go:394` (see also guard split `chunker.go:439-449`)

So every row in `agent_vectors.content_type` is the string `<fileType>_section`
(in practice `kb_section`), never a bare `kb` or `policy`.

Schema (`vectordb_cockroach.go:104-116`):

```
agent_vectors(id STRING PK, tenant_id INT, content_type STRING,
              title STRING, content TEXT, embedding VECTOR(1536),
              metadata JSONB, source_version INT, created_at TIMESTAMPTZ)
```

### 2.2 How the chat/retrieval searches

The widget chat path calls `generateRAGResponse` (`service.go:3686`) which calls
`RetrieveContextForQuery` (`vectordb.go:1055`). This path intentionally searches
*all* content types:

```go
ContentTypes: []string{},   // comment: empty = search all types
AudienceType: audience,     // "" when internal (search both)
TopK:         10,
MinScore:     0.25,
```
> — `vectordb.go:1077-1085`

### 2.3 The bug: from "empty list" to `content_type IN ('')`

`CockroachVectorDB.Search` builds its WHERE clause like this
(`vectordb_cockroach.go:387-405`):

```go
contentTypeFilter := "''"                       // default when list empty
if len(query.ContentTypes) > 0 { ... }          // build "a","b",...

sqlQuery := `
   SELECT id, title, content, content_type, metadata, source_version, created_at,
          1 - (embedding <=> $1::VECTOR) AS similarity
   FROM agent_vectors
   WHERE tenant_id = $2 AND content_type IN ('' )          -- <-- BUG
     AND (embedding <=> $1::VECTOR) <= 1 - $4
   ORDER BY embedding <=> $1::VECTOR
   LIMIT $3`
```

Because chunks store `content_type = 'kb_section'`, the predicate
`content_type IN ('')` matches **zero rows**. Every chat/queries which searches
all types therefore returns `TotalResults: 0` on the CRDB backend.

### 2.4 Why the failure is silent (the observed symptom)

In `generateRAGResponse` (`service.go:3733-3758`):

```go
needsFallback := len(retrieved.KBSections) == 0 || retrieved.topScore() < 0.25
...
fallbackKB := truncateToTokenBudget(config.RawKB, 6000) // ragFallbackTokenBudget
```

A zero-hit search → fallback is enabled → the system prompt is built from the
**raw KB truncated to 6,000 tokens**. For a novel-length KB, 6,000 tokens covers
roughly the Translator's Introduction and early chapters — exactly the text the
model reports having. The answer is an honest but useless "I only have the
introduction."

### 2.5 Why the widget was "not showing up" (separate)

`bchat-crdb.fly.dev/widget/<slug>/embed.js` returns 404 for every slug: the
CockroachDB cluster was fresh and `agent_tenants` had no rows, so
`ResolveSlugTenantMiddleware` (`tenant_resolver.go:26-35`) → "Agent not found"
→ `embed.js` never loads → widget never render.

**Out of scope of this bug.** Closing note: agents must exist and be re-indexed
on the CRDB backend (via Agent Admin) for a widget to load at all. Bug 060 is
about searches returning zero *even when* the index is populated.

### 2.6 Latent defects uncovered during investigation (NOT fixed here)

1. **CRDB Search ignores `SourceVersion` / `ActiveOnly`.** `SearchQuery` carries
   `SourceVersion`, `AudienceType`, `ActiveOnly`, but the SQL in
   `vectordb_cockroach.go` only uses `tenant_id`, `content_type`, distance. A
   versioned query therefore can't restrict to the active/versioned rounds.
2. **CRDB has no `audience` column.** `agent_vectors` stores only `content_type`
   (which is `fileType`). `AuditType` filter for external vs internal is
   impossible at the DB level today. `ReleaseVersion`/`resume` checkpointing
   works, but audience-level isolation does not.
3. **content_type CRUD mismatch (both CRDB AND LanceDB)**: deletion/purge/
   version-list helpers pass `fileType` (`kb`) but chunks store `fileType_section`.

Because fixing those changes shared code paths and behavior reachable by
PG/LanceDB deployments, they are deferred — see §5.

---

## 3. Root cause (one line)

`CockroachVectorDB.Search` renders an empty `ContentTypes` list as the SQL
clause `content_type IN ('')`, the `agent_vectors` rows never match, so every
chat/queries retrieves nothing and the caller silently falls back to a 6,000-token
raw-KB prompt.

## 4. Why this is CRDB-specific (backend isolation)

`NewVectorDB` (`vectordb.go:259-293`) routes purely on
`LANCEDB_STORAGE_PROVIDER`:

| Provider | Class | Build tag | Search works? |
|---|---|---|---|
| `cockroach` | `CockroachVectorDB` | `cockroach` | **Broken** (this fix) |
| `s3` / `local` | `LanceVectorDB` | `rag` | OK — `buildFilter` omits content_type when list empty (`vectordb_lance.go:1198`) |
| `memory` | `MemoryVectorDB` | — | OK — hard-coded content filter per row |
| none / `default` | `NoOpVectorDB` | — | n/a |

The app datastore drivers (Postgres/Neon `store/db/postgres/agent.go`, SQLite
`store/db/sqlite/agent.go`, and CRDB via the postgres wire protocol) are
untouched by this change; they handle the app data (tenants, KB files,
reindex checkpoints) and play no part in the vector-search SQL.

**Residual:** All deploy Docker images build with `-tags cockroach` only for
`Dockerfile.cockroach.fly` (`bchat-crdb`); the PG and S3 images build with
`-tags rag`.

---

## 5. Implementation plan

### Fix 1 — CockroachVectorDB.Search filter builder (REQUIRED)

File: `server/.../agent/vectordb_cockroach.go` lines ~387-405.

Adopts plan-review findings (§5 of plan_review.md). Extract a testable helper:

```go
func buildCockroachSearchQuery(query SearchQuery, vecStr string) (string, []interface{}, error)
```

Implemented rules inside the builder:
- `len(query.ContentTypes) == 0` → **no** `content_type IN (...)` clause at all
  (search all types — the documented intent of the chat flow).
- Non-empty → validate each entry against `^[A-Za-z0-9_-]+$` (allowlist; rejects
  admin-supplied `fileType`/`AudienceType` injection), expand bare types to
  `<ct>` + `<ct>_section`, dedupe, emit `content_type IN (...)`.
- `query.TopK <= 0` → default `10` (mirrors LanceDB).
- Keep `tenant_id`, distance filter, `LIMIT`; maintain
  `formatVectorLiteral` text-format for `VECTOR` params.

### Fix 2 — content_type CRUD mismatch (REQUIRED, same PR)

File: `vectordb_cockroach.go`.

All three helpers match `content_type IN ($fileType, $fileType_section)` so
version-purge and reindex actually delete/purge the rows the chunker writes:

- `ListIndexedVersions` — `content_type IN ($2, $3)` both variants;
- `PurgePreVersionedChunks` — `content_type IN ($2, $3)` +
  `(source_version IS NULL OR source_version <= 1)`;
- `DeleteByVersion` — `content_type IN ($2, $3)` + `source_version = $4`.

Plain `Delete()` left unchanged (verified unused on the CRDB path — it is only
referenced by `TenantVectorDBPool`, which serves Lance backends).

Explicitly do NOT touch `LanceVectorDB` in this change. The identical Lance
mismatch (`vectordb_lance.go:1049,1066` filter `content_type = <fileType>` but
rows store `<fileType>_section`) is logged as a follow-up (bugs/061) so PG/S3
deployments are not disturbed.

### Fix 3 — `AudienceType`/`ActiveOnly` in CRDB (DEFERRED — document only)

- CRDB `agent_vectors` lacks an `audience` column; to honor audience filters the
  vector row must persist the audience into `metadata` JSONB instead of
  `'{}'`, or add a column. Requires a column migration + reindex; must remain
  outside this fix.

## 6. Tests

- SQL shape unit tests for the query builder (`buildCockroachSearchQuery`):
  - `ContentTypes = []` → no `content_type` predicate in the emitted SQL;
  - `ContentTypes = ["kb_section","kb"]` → `content_type IN ('kb','kb_section')`
    (deduplicated);
  - `ContentTypes = ["kb"]` → expands to both `kb` and `kb_section`;
  - invalid `ContentTypes` entry (e.g. `kb'; DROP`) → returns error, no SQL
    interpolation;
  - `TopK <= 0` → defaults to 10.
- New file `vectordb_cockroach_test.go` (`//go:build cockroach`) — pure string
  comparison, no live CRDB required.
- Full CI run for `cockroach` tag and for `rag` tag to prove no cross-contamination.

## 7. Verification

Local (repo `/home/.../bchat`):

```bash
COCKROACH_DSN=... task build:backend:cockroach
COCKROACH_DSN=... task run   # or run binary
# 1) create/seed a test tenant + KB
# 2) Admin → Rebuild Index
# 3) Agent Admin → RAG Search Explorer, query "what does Maria Clara give the
#    leper", with file-type selection EMPTY → expects `kb_section` chunks
#    (pre-fix: 0 rows)
# 4) chat the question in the widget → correct answer
# 5) server logs must NOT print `RAG fallback activated`
```

Deployed:

```bash
# cd /home/chaschel/Documents/go/bchat
# fly deploy --config fly_cockroach.toml   (or scripts/crdb-deploy.sh)
# re-run Rebuild Index in Agent Admin on bchat-crdb
# repeat steps 3–4 against bchat-crdb.fly.dev
```

Rollback path: redeploy the previous known-good image; the SQL change is
read-only and additive.

## 8. Risks / open questions

- **Deployed binary vs repo HEAD** — the running `bchat-crdb` image may lag this
  source; confirm prod behavior/confirm before shipping (log line check).
- **KB coverage on CRDB** — if the uploaded KB file on the CRD instance itself
  only contains the Translator's Introduction chunk, Fix 1 alone won't answer.
  Validate via RAG Search Explorer after fix.
- **Regressions in other backends** — by keeping all edits inside
  `vectordb_cockroach.go`, Lance/S3/PG/SQLite are untouched; verify with a full
  build of `-tags rag`.
- Distraction: `content_type IN ('')` might have legacy bare `kb` rows from
  older importers (see vectordb_test.go:473 for old/new format). Fix may need
  to accept `*_section` variants when a caller supplies bare types
  (defensive; normalize both).
- **Local CRDB shell gotcha** — the repo's bundled
  `cockroach-sql-v22.1.9.linux-amd64` predates VECTOR support (v25.2+).
  Local verification needs a real CRDB 25.2+ instance / serverless DSN.

---

## 9. CockroachDB MCP Validation (2026-08-06)

Reviewed the plan against CockroachDB official knowledge sources:

### Confirmed (no change required)

1. **`<=>` is cosine distance; similarity = `1 - distance`.** The app's
   `1 - (embedding <=> $1::VECTOR) AS similarity` and filter
   `(embedding <=> $1::VECTOR) <= 1 - $4` (= similarity ≥ 0.25) match CRDB
   semantics exactly.
2. **Vector literal text format.** `'[1.0, 0.0, 0.0]'` cast `::VECTOR` is the
   documented input format — `formatVectorLiteral` approach is correct.
3. **`CREATE VECTOR INDEX` syntax** is valid (v25.2+); the cluster setting
   `feature.vector_index.enabled = true` is already applied via
   `scripts/crdb-init.sql` and the `Validate()` SQLSTATE fallback remains.

### Finding A — search is NOT accelerated by the current vector index (perf)

The index `idx_agent_vectors_embedding` is created without an opclass →
default `vector_l2_ops` (accelerates only `<->`). The query orders by `<=>`
(cosine), so CRDB brute-force scans. Correctness unaffected; results are
complete, just slower.

**Recommendation (deferred, NOT part of this fix):** drop and recreate the
index with prefix column + cosine opclass:

```sql
DROP INDEX agent_vectors@idx_agent_vectors_embedding;
CREATE VECTOR INDEX idx_agent_vectors_embedding
  ON agent_vectors (tenant_id, embedding vector_cosine_ops);
```

Prefix columns are used by the optimizer only when constrained — `tenant_id`
is always constrained by the search query. This is a schema-mutating prod
change with index rebuild cost; track separately (bugs/061).

### Finding B — large batch VECTOR inserts discouraged (info)

CRDB docs warn that large batch inserts of VECTOR degrade performance
(batching should be avoided). Current reindex batches via
`EMBEDDING_BATCH_SIZE` (10 on bchat-crdb). Out of scope; noted for the
reindex-tune follow-up.

### Finding C — local verification version gap (process)

Repo's bundled `cockroach-sql-v22.1.9` shell predates VECTOR (v25.2+). Local
testing must target CRDB 25.2+ / serverless; see §8.

---

## 9. Adversarial Review (to be executed before implementing)

Prompt for a hostile reviewer LLM/agent:

> Act as a hostile engineering reviewer for bugs/060 in this repo. Assume the
> plan author is competent but biased. Attack these surfaces and produce a
> verdict — `GO` / `GO-WITH-CHANGES` / `NO-GO` — with a numbered list of
> concrete issues:
>
> 1. **Root-cause validity** — Re-derive the failure from source: confirm
>    `RetrieveContextForQuery` sends `ContentTypes: []`, that
>    `CockroachVectorDB.Search` renders it as `content_type IN ('')`, and
>    that chunks are stored as `content_type = '<fileType>_section'`
>    (chunker.go:394). Cite inconsistencies. Can the new SQL be tested without a
>    live CRDB?
> 2. **Broken assumptions** — Is `content_type IN ('')` truly match-zero? Are
>    there legacy rows with `content_type` `''` or bare `kb`?  Does the deployed
>    bchat-crdb binary match repo HEAD — could prod behavior differ?
> 3. **Fix risk** — With `ContentTypes` empty the new query drops the clause;
>    can it regress LanceDB/PG/S3/SQLite? Compiles under both `-tags cockroach`
>    and `-tags rag`? Does dropping the filter break tenant isolation / the
>    `tenant_id` guard?
> 4. **Latent bugs** — CRDB Search also ignores `SourceVersion`/`ActiveOnly` and
>    there's no audience column. If the fix lands alone, what visible failure
>    remains (stale-version mixing)? Classify as must-fix vs defer.
> 5. **Scope discipline** — verify the plan touches ONLY `vectordb_cockroach.go`
>    (CRDB-only) and no postgres/sqlite/Lance files; flag any drift.
> 6. **Test design** — are the SQL-shape unit tests meaningful? Can they run in
>    CI without a CRDB server? Flake risk?
> 7. **Deploy** — rebuild/redeploy steps, downtime, reindex requirement, rollback
>    path.
> 8. **Edge cases** — empty `QueryText`, `TopK<=0`, MinScore distance/similarity
>    sign in CRDB, content_type values written by legacy importers
>    (`kb` vs `kb_section`, see vectordb_test.go:473-480).
> 9. **Missed anything?** — the widget-404 issue, the fallback-token-budget
>    artifact, embed.js caching.
>
> Rate confidence that Fix 1 alone restores correct answers on bchat-crdb.