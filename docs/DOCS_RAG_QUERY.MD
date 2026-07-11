# RAG Query Tool — `scripts/rag_query.sh`

**Purpose:** Multi-mode CLI for querying the bchat RAG pipeline (LanceDB vector search), browsing source files in SQLite, and grepping code — all from one shell script. Designed to give AI agents a grounded, no-hallucination answer path.

**File:** `scripts/rag_query.sh` (387 lines, bash, MIT-style)

---

## Modes

| Command | What it does | Auth needed? |
|---------|-------------|-------------|
| `rag_query.sh search "query" [tenant_id] [top_k]` | Semantic search via `/api/v1/admin/rag/search` | JWT in `/tmp/bchat_token` |
| `rag_query.sh source [tenant_id] [file_type]` | Lists `agent_source_files` from SQLite | No (local DB) |
| `rag_query.sh read <file_id>` | Dumps full content of one source file | No (local DB) |
| `rag_query.sh grep "pattern"` | ripgrep across `*.go`, `*.md`, `*.ts`, `*.tsx`, `*.sql`, `*.yaml`, `*.json` | No |
| `rag_query.sh all "query" [tenant_id]` | RAG search → fallback to grep if API unavailable | Optional |
| `rag_query.sh help` | Prints usage from script header | No |

---

## Setup

### Dependencies

```bash
sudo apt install curl sqlite3 ripgrep
# python3 is used for JSON payload building and JWT expiry parsing
```

### JWT Token (for `search` mode only)

The `search` mode hits the authenticated admin RAG API. You need a valid JWT:

```bash
# Once, after logging into bchat at http://localhost:5230:
# Browser dev tools → Application → Cookies → memos.access-token
# Copy the token value:
echo "eyJhbGciOiJIUzI1NiIsImtpZCI6InYxIn0..." > /tmp/bchat_token
```

**Fallback chain:** If the server is unreachable, or the token is missing/expired, `search` and `all` modes degrade to `grep` automatically. No silent failures.

---

## Architecture

### Data Flow

```
┌─ User command ──────────────────────────────────┐
│ ./rag_query.sh search "observer buffer" 1 5     │
└───────────────────┬─────────────────────────────┘
                    │
                    ▼
      ┌──────────────────────────┐
      │       main()             │
      │  check_deps → dispatch   │
      └──────────┬───────────────┘
                 │
        ┌────────┴────────┐
        ▼                  ▼
cmd_search()        cmd_grep() (fallback)
        │
        ├─ health-check: curl /api/v1/admin/rag/stats
        ├─ token check: read /tmp/bchat_token, decode JWT expiry
        ├─ API call: POST /api/v1/admin/rag/search
        │   body: {"query", "tenantId", "topK", "audienceType"}
        └─ parser: inline python3 script formats results
```

### File Structure

```
scripts/rag_query.sh          # Executable entry point
```

**Config constants** (lines 22-27):
| Variable | Default | Purpose |
|----------|---------|---------|
| `BCHAT_DIR` | `$(dirname $0)/..` | Project root (derived from script location) |
| `DB` | `build/data/memos_dev.db` | SQLite database for `source` and `read` modes |
| `API_BASE` | `http://localhost:5230` | bchat server for RAG API |
| `TOKEN_FILE` | `/tmp/bchat_token` | JWT token cache |
| `DEFAULT_TENANT_ID` | `1` | Default tenant if none specified |
| `DEFAULT_TOP_K` | `5` | Default number of RAG results |

### Capabilities

| Feature | How it works |
|---------|-------------|
| **Health-check before RAG** | Hits `/api/v1/admin/rag/stats` — if the server isn't running, skips RAG, goes straight to grep |
| **JWT expiry validation** | Decodes JWT payload (base64 → json → exp field), compares with `date +%s` |
| **Python payload builder** | Uses `python3 -c` with `json.dumps()` to avoid shell quoting issues in the POST body |
| **Inline result formatter** | Pipeline output through a second `python3 -c` to pretty-print chunks with scores, metadata, and truncated content |
| **Graceful fallback chain** | `search` and `all` never fail with an API error — they always fall through to `grep` |
| **Source file listing** | SQLite join on `agent_source_files` + `agent_tenants` with optional `tenant_id` and `file_type` filters |
| **Full content dump** | Read mode is a single `SELECT content FROM agent_source_files` — raw markdown output |

### Graceful Fallback Chain

```
cmd_search()
    │
    ├─ Server reachable? ──NO──► cmd_grep()
    │
    ├─ Token valid? ──────NO──► cmd_grep()
    │
    ├─ Payload build ok? ──NO──► cmd_grep()
    │
    ├─ API call ok? ───────NO──► cmd_grep()
    │
    ▼
Format & display RAG results
```

This means `search` mode **never returns empty-handed**. If RAG is unavailable, you still get ripgrep results.

---

## Response Format

### Search Mode Output

```
[MODE: search]
[QUERY: observer buffer size]
[SOURCE: rag-api]
[TENANT: 1] [K: 5]
[ENGINE: hybrid] [LATENCY: 142ms]
[TOTAL: 3]
---

Chunk 1 (score: 0.871 | vector=0.923 bm25=0.342)
  ID: chunk_abc123
  Title: Observation Memory Configuration
  Type: documentation  Audience: external
  Code: OM_BUFFER_SIZE=100
  Content:
  The observation buffer collects raw events...
```

**Fields explained:**
| Field | Source | Meaning |
|-------|--------|---------|
| `ENGINE` | `response.searchMode` | `vector`, `hybrid`, or `bm25` |
| `LATENCY` | `response.latencyMs` | Server-side RAG round-trip time |
| `score` | `result.score` | Combined relevance (0-1) |
| `vectorScore` | `result.vectorScore` | Semantic similarity component (hybrid only) |
| `bm25Score` | `result.bm25Score` | Keyword match component (hybrid only) |
| `Content` | `result.chunk.content` | Truncated at 600 chars |

### Source Mode Output

```
[MODE: source]
[SOURCE: sqlite]
[DB: .../build/data/memos_dev.db]
---
id  slug  tenant_id  audience_type  file_type  version  bytes  imported_at
--  ----  ---------  -------------  ---------  -------  -----  -----------
12  inc   4          external       kb         2        14321  2026-07-10 18:22:31
```

Columns: `id`, `slug`, `tenant_id`, `audience_type` (external/internal), `file_type` (kb/policy/script), `version`, `bytes`, `imported_at`.

---

## Agent Usage

When answering a user's question about the bchat codebase:

### 1. If JWT is available (RAG online)

```bash
./scripts/rag_query.sh search "how does the observer pipeline work?" 1 5
```

→ Returns semantically relevant chunks from the KB.MD, POLICY.MD, or SCRIPT.MD files that were embedded into LanceDB.

→ I answer with **direct quotes + file references**, no hallucination.

### 2. If no JWT or RAG offline

```bash
./scripts/rag_query.sh grep "RunObserver"
```

→ Returns ripgrep matches with line numbers from `*.go` and related files.

→ I answer with **code location + surrounding context**.

### 3. To inspect source file contents

```bash
./scripts/rag_query.sh read 12
```

→ Returns the full markdown text of a tenant's uploaded KB, policy, or script.

### 4. Combined mode (try RAG, fall back to grep)

```bash
./scripts/rag_query.sh all "event claiming water extraction"
```

---

## File Layout Reference

```
scripts/
└── rag_query.sh        # This script (387 lines)
    ├── show_help()             # Lines 43-46
    ├── check_deps()            # Lines 49-61
    ├── read_token()            # Lines 64-68
    ├── token_valid()           # Lines 70-87
    ├── cmd_search()            # Lines 90-204
    │   ├── health check        # Line 100
    │   ├── token validation    # Lines 110-117
    │   ├── payload build       # Lines 122-135
    │   ├── API call            # Lines 137-148
    │   └── result formatter    # Lines 156-204
    ├── cmd_source()            # Lines 208-258
    ├── cmd_read()              # Lines 262-301
    ├── cmd_grep()              # Lines 305-334
    ├── cmd_combined()          # Lines 338-350
    └── main()                  # Lines 354-386
```

---

## Comparison: Chatbot vs Script

| Aspect | Chatbot (web UI) | `rag_query.sh` |
|--------|-----------------|-----------------|
| Granularity | Full conversation | Targeted queries |
| Grounding | Internal agent memory | Direct LanceDB chunks |
| Response time | Seconds (LLM generation) | <200ms per query |
| Auth | Hidden (cookie) | Explicit `/tmp/bchat_token` |
| Exploration | Interactive | Scriptable, pipeable |

The script is the **surgical tool** — fast, grounded, composable. The chatbot is the **experience layer** — conversational, patient, explanatory.

---

## Security

- **Token lives in `/tmp/bchat_token`** — readable only by the current user (assuming standard umask). Never stored in the repo.
- **Token expiry validated client-side** — expired tokens trigger fallback to grep, never a "try harder" loop.
- **SQLite queries are read-only** — `cmd_source` and `cmd_read` do `SELECT` only. No writes.
- **RAG API is admin-only** — the `/api/v1/admin/rag/search` endpoint requires an authenticated session. The script does not bypass auth; it uses the same JWT the web UI does.

---

## Limitations

| Limitation | Impact |
|-----------|--------|
| JWT must be manually copied | Single-user scripts are fine; teams need a shared token file or a wrapper that logs in programmatically |
| Content truncated at 600 chars | Full content available via `read` mode |
| SQLite path hardcoded | Only works with default dev database; overridable via `DB` env var (not yet exposed) |
| No paging on `source` | Limited to 50 rows |
| Python3 dependency for JWT parsing and JSON building | Already a dev dependency (used by bchat build); small burden |

---

## Future

- **`DB` env var** — for pointing at production DBs or custom paths
- **`--no-fallback` flag** — to return non-zero exit code instead of degrading to grep
- **`--json` output** — for piping into `jq` or other tools
- **`--tenant-slug`** — resolve slug → ID via `agent_tenants` table
- **Programmatic auth** — `rag_query.sh login` that does the multi-tenant REST flow and writes `/tmp/bchat_token` directly
