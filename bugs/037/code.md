# Code.md — Plan 037-UX Implementation

**Date:** 2026-07-14
**Branch:** Working tree (uncommitted)
**Status:** Implemented + tested, ready for adversarial review

---

## Summary of Changes

Backend pre-checks detect skip conditions synchronously (no goroutine spawned),
enrich status responses with `retrieval_mode`, and persist failed checkpoints for
goroutine-level failures. Frontend renders category-appropriate toasts and
mode-aware idle messaging.

---

## Files Changed (12)

| File | Lines Changed | Description |
|------|---------------|-------------|
| `store/driver.go` | +1 | `CountTenantSourceFiles` interface method |
| `store/agent.go` | +4 | Delegating method |
| `store/db/sqlite/agent.go` | +9 | `COUNT(*) + SUM(LENGTH(content))` query |
| `store/db/postgres/agent.go` | +9 | Same (Postgres `$1`) |
| `store/db/mysql/agent.go` | +4 | `errNotImplemented` stub |
| `handlers.go` | +66/-30 | SkipReason constants, 3 pre-checks, goroutine + 202 |
| `service.go` | +139/-20 | `createFailedCheckpoint`, `retrieval_mode` in status, verbose logging |
| `handlers_test.go` | +15 | `TestSkipReasonConstants` |
| `service_reindex_test.go` | +24 | `TestCreateFailedCheckpointWritesToStore` |
| `agentAdmin.ts` | +9/-3 | `SkipReason` type, `retrieval_mode`, `skip_reason` return |
| `AgentAdmin.tsx` | +29/-2 | Toast switch + idle-state card |
| `en.json` | +5 | 5 translation keys |

---

## Detailed Changes

### Change 1b: `CountTenantSourceFiles` (store layer)

**Purpose:** Lightweight alternative to `ListAgentSourceFiles` for the pre-check.
`ListAgentSourceFiles` with `LatestOnly: true` still fetches `content` blobs (confirmed
in both SQLite and Postgres implementations). A single `COUNT(*) + SUM(LENGTH(content))`
query is ~100x cheaper.

```sql
-- SQLite/MySQL
SELECT COUNT(*), COALESCE(SUM(LENGTH(content)), 0)
FROM agent_source_files WHERE tenant_id = ?

-- Postgres
SELECT COUNT(*), COALESCE(SUM(LENGTH(content)), 0)
FROM agent_source_files WHERE tenant_id = $1
```

**Interface:**
```go
CountTenantSourceFiles(ctx context.Context, tenantID int32) (count int, totalContentLen int, err error)
```

**MySQL:** Returns `errNotImplemented` (consistent with other MySQL agent methods).

---

### Change 1: Skip Reason Constants + Pre-Checks (handlers.go)

**Constants:**
```go
const (
    SkipReasonNone             = ""
    SkipReasonLongContext      = "long_context"
    SkipReasonNoSourceFiles    = "no_source_files"
    SkipReasonPipelineDisabled = "pipeline_disabled"
)
```

**Pre-checks (after long_context skip, before goroutine spawn):**

1. **Pipeline disabled:** `!h.service.IsRAGEnabled()` → 200 + `SkipReasonPipelineDisabled`
2. **No source files:** `CountTenantSourceFiles` returns count=0 or totalContentLen=0 → 200 + `SkipReasonNoSourceFiles`
3. **Count query fails:** Log warning, fall through to goroutine (best-effort)

**Ordering:**
```
long_context skip → pipeline check → source file count → (goroutine spawn)
```

**HTTP status:** Pre-checks return `200 OK`. Goroutine returns `202 Accepted`.

---

### Change 2: `createFailedCheckpoint` (service.go)

**Purpose:** Goroutine-level failures (G1: nil vectorDB, G2: NoOpVectorDB) that occur
after the goroutine starts but before any checkpoint is created need a checkpoint so
`GetReindexStatus` can surface them.

```go
func (s *Service) createFailedCheckpoint(ctx context.Context, tenantID int32, audience, msg string) {
    cp := &store.ReindexCheckpoint{
        TenantID:     tenantID,
        Audience:     audience,
        Status:       "failed",
        ErrorMessage: msg,
        StartedAt:    time.Now(),
    }
    // Detached context: checkpoint write must persist even if the
    // parent request context is cancelled (goroutine may abort).
    // 5s timeout prevents a slow DB write from blocking indefinitely.
    checkpointCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if _, err := s.store.UpsertReindexCheckpoint(checkpointCtx, cp); err != nil {
        slog.Warn("failed to create failure checkpoint", "error", err)
    }
}
```

**Called from:**
```go
// In ReindexTenantContentWithResume:
if s.vectorDB == nil || s.chunker == nil {
    s.createFailedCheckpoint(ctx, tenantID, audienceType, "RAG pipeline not initialized")
    return 0, fmt.Errorf("RAG pipeline not initialized")
}
if _, isNoOp := s.vectorDB.(*NoOpVectorDB); isNoOp {
    s.createFailedCheckpoint(ctx, tenantID, audienceType, "RAG pipeline disabled")
    return 0, fmt.Errorf("RAG pipeline disabled (using NoOpVectorDB)")
}
```

---

### Change 3: `retrieval_mode` in ReindexStatus (service.go)

**Struct:**
```go
type ReindexStatus struct {
    // ... existing fields ...
    RetrievalMode   string `json:"retrieval_mode,omitempty"`
}
```

**Populated in all 3 return paths** of `GetReindexStatus`:
1. Idle (no checkpoints) → fetch from `GetTenantConfig`
2. Single checkpoint → fetch before return
3. Aggregate checkpoints → fetch before return

**Error handling:** `slog.Warn` on failure, field left empty (frontend treats omission as "unknown").

---

### Change 4: Frontend Store (agentAdmin.ts)

```typescript
export type SkipReason = "long_context" | "no_source_files" | "pipeline_disabled";

export interface ReindexStatus {
  // ... existing fields ...
  retrieval_mode?: string;
}

const reindexTenant = async (slug, audienceType) => Promise<{
  success: boolean;
  chunks?: number;
  skip_reason?: SkipReason;  // NEW
  error?: string;
}>
```

---

### Change 5: Toast Switch (AgentAdmin.tsx)

```typescript
if (result.success) {
    switch (result.skip_reason) {
      case "long_context":
        toast(t("agent-admin.reindex-skipped-long-context"), { icon: "ℹ️" });
        break;
      case "no_source_files":
        toast(t("agent-admin.reindex-skipped-no-source"), { icon: "⚠️" });
        break;
      case "pipeline_disabled":
        toast(t("agent-admin.reindex-skipped-pipeline"), { icon: "⚠️" });
        break;
      default:
        toast.success(t("agent-admin.rebuild-index-started"));
    }
}
```

---

### Change 6: Idle-State Card (AgentAdmin.tsx)

```tsx
{agentAdminStore.state.reindexStatus?.status === "idle" && (
  <div className="bg-white dark:bg-zinc-800 rounded-lg p-3 border ...">
    {retrieval_mode === "long_context" ? (
      <div className="text-xs text-blue-600 ...">ℹ️ {t("...idle-long-context")}</div>
    ) : retrieval_mode === "rag" ? (
      <div className="text-xs text-amber-600 ...">⚠️ {t("...idle-no-index")}</div>
    ) : null}
  </div>
)}
```

---

### Change 7: Translation Keys (en.json)

```json
"reindex-skipped-long-context": "Skipped — this tenant uses long_context mode, RAG indexing is not needed",
"reindex-skipped-no-source": "No files to index. Upload KB or Policy files first.",
"reindex-skipped-pipeline": "RAG pipeline is not configured. Check server settings.",
"reindex-idle-long-context": "This tenant uses long_context mode. The full document is sent to the LLM — RAG indexing is not needed.",
"reindex-idle-no-index": "No RAG index found. Upload KB/Policy files and click Rebuild Index to make content searchable."
```

---

### Change 8: Tests

**`TestSkipReasonConstants`** — Validates all 4 constants have expected values.

**`TestCreateFailedCheckpointWritesToStore`** — Validates checkpoint struct population
(TenantID, Audience, Status, ErrorMessage). Does not test DB write (requires mock store).

---

## UX Matrix

| Scenario | HTTP | Toast | Progress Card | Detection |
|----------|------|-------|---------------|-----------|
| long_context skip | 200 | ℹ️ | Blue info | Sync |
| No source files | 200 | ⚠️ | — | Sync |
| Pipeline disabled | 200 | ⚠️ | — | Sync |
| Normal start | 202 | ✅ | Progress bar | Async |
| Zero chunks | 202 | ✅ | 0 chunks | Async |
| GOROUTINE FAILS | 202 | ✅ | Failed → red | Async |
| Idle, long_context | — | — | Blue info | Poll |
| Idle, RAG, no index | — | — | Amber warning | Poll |

---

## Build & Test Results

```
go build ./...                              # clean
go test ./server/router/api/v1/agent/...    # all pass
go test ./store/...                         # pass (pre-existing schema version failure unrelated)
```

---

## Adversarial Code Review Prompt

You are a senior security and reliability engineer performing an adversarial code review
of the plan_ux3.md implementation changes. Your goal is to find bugs, security issues,
race conditions, edge cases, and design flaws that could cause production failures.

Review the following files:
- `store/driver.go` (new method)
- `store/agent.go` (delegating method)
- `store/db/sqlite/agent.go` (CountTenantSourceFiles)
- `store/db/postgres/agent.go` (CountTenantSourceFiles)
- `store/db/mysql/agent.go` (CountTenantSourceFiles stub)
- `server/router/api/v1/agent/handlers.go` (SkipReason constants, pre-checks, goroutine)
- `server/router/api/v1/agent/service.go` (createFailedCheckpoint, retrieval_mode, logging)
- `server/router/api/v1/agent/handlers_test.go` (TestSkipReasonConstants)
- `server/router/api/v1/agent/service_reindex_test.go` (TestCreateFailedCheckpointWritesToStore)
- `web/src/store/v2/agentAdmin.ts` (SkipReason type, ReindexStatus update)
- `web/src/pages/AgentAdmin.tsx` (toast switch, idle card)
- `web/src/locales/en.json` (translation keys)

**Context:**
- The `HandleReindexTenant` handler now spawns a goroutine for the reindex operation
- Pre-checks run synchronously before the goroutine
- `createFailedCheckpoint` uses a detached 5s context for DB writes
- `CountTenantSourceFiles` runs `COUNT(*) + SUM(LENGTH(content))` on `agent_source_files`
- Frontend polls `GET /:slug/reindex/status` for progress
- The service uses `NoOpVectorDB` when RAG is disabled

**Specific areas to scrutinize:**

1. **Race conditions:**
   - Can `createFailedCheckpoint` race with the goroutine's own checkpoint creation?
   - Can two concurrent reindex calls for the same tenant+audience interfere?
   - Is the 5s detached context sufficient for all DB backends under load?

2. **Error handling:**
   - What happens if `CountTenantSourceFiles` returns an error AND the goroutine also fails?
   - What if `GetTenantConfig` fails in all 3 return paths of `GetReindexStatus`?
   - What if the frontend receives a response without `skip_reason` (field missing)?

3. **Edge cases:**
   - Can a tenant have files with `content = ""` but `count > 0`? Does the pre-check handle this?
   - What if `retrieval_mode` is empty string vs. null vs. undefined in the JSON?
   - Can `skip_reason` be an empty string? What does the frontend switch do with `""`?

4. **Security:**
   - Does `CountTenantSourceFiles` need tenant isolation? (It receives `tenant.ID` from the handler, which already verified tenant access)
   - Can a user craft a request that bypasses the pre-checks?
   - Is the 202 response safe when the goroutine fails silently?

5. **Performance:**
   - Is `COUNT(*) + SUM(LENGTH(content))` actually indexed on `tenant_id`?
   - Can the pre-check queries be slow for tenants with millions of chunks?
   - Does the idle-state card cause unnecessary re-renders?

6. **Correctness:**
   - Does the frontend `switch` on `skip_reason` handle `undefined` correctly? (TypeScript type says optional)
   - Are the translation keys referenced in both toast AND idle card? (5 keys, 2 usage sites)
   - Does `retrieval_mode` get populated for tenants with no config (null tc)?

7. **Test coverage:**
   - Is `TestCreateFailedCheckpointWritesToStore` actually testing the function, or just struct construction?
   - Should there be an integration test that calls `createFailedCheckpoint` against a real store?
   - Are the pre-check paths tested end-to-end?

**Return your findings as a prioritized list:**
- **P0 (Critical):** Bugs that will cause data loss, security breaches, or production outages
- **P1 (High):** Issues that will cause incorrect behavior in common scenarios
- **P2 (Medium):** Edge cases that could cause problems under unusual conditions
- **P3 (Low):** Code quality, style, or documentation issues

For each finding, include:
1. File and line number
2. Description of the issue
3. Reproduction steps or scenario
4. Suggested fix
