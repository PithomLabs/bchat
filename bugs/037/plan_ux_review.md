# Review: plan_ux.md (037-UX)

**Verdict: APPROVED WITH NITS**

---

## ✅ Strengths

- **Clear 3-category UX model** (by-design / limitation / bug) maps directly to info/warning/error primitives — no guesswork for implementors.
- **Backend-owns-message principle** is correct. Frontend should render, not infer.
- **UX matrix** is a single source of truth covering all known paths — should be kept in sync as new paths emerge.
- **`createFailedCheckpoint` helper** solves the core silent-failure problem soundly.

---

## 🔴 Nits (must rework before implementation)

### 1. Misleading success toast for pre-checkpoint failures

**Lines referenced:** UX Matrix rows 4-6 ("No source files", "Pipeline disabled", "Goroutine crashes early")

The matrix shows these returning **202 Accepted** with a **✅ "Reindexing started"** toast, only for polling to later reveal a failure. The user gets a moment of false success.

**Fix:** For failures detectable *before* spawning the goroutine (no source files, pipeline disabled, NoOpVectorDB), return **200 OK** synchronously with a `skip_reason` and an appropriate toast:

| Scenario | HTTP Status | Toast |
|----------|-------------|-------|
| No source files | 200 + `skip_reason: "no_source_files"` | ⚠️ "No files to index" |
| Pipeline disabled | 200 + `skip_reason: "pipeline_disabled"` | ⚠️ "Pipeline not configured" |
| NoOpVectorDB | 200 + `skip_reason: "pipeline_disabled"` | ⚠️ "Pipeline not configured" |

Only spawn a goroutine when reindex work is actually needed (actual files, pipeline ready). This eliminates the 2-step UX and avoids goroutines that immediately fail.

---

### 2. `skip_reason` is an untyped magic string

**Change 1** introduces `skip_reason: "long_context"` as a raw string. A single-value enum invites drift between backend and frontend as more reasons are added.

**Fix (backend):** Define a const group:
```go
const (
    SkipReasonNone              = ""
    SkipReasonLongContext       = "long_context"
    SkipReasonNoSourceFiles     = "no_source_files"
    SkipReasonPipelineDisabled  = "pipeline_disabled"
)
```

**Fix (frontend):** Use a union type instead of a raw string comparison:
```typescript
type SkipReason = "long_context" | "no_source_files" | "pipeline_disabled";
```

---

### 3. `retrieval_mode` fetch failures are silently swallowed

**Change 3, populate code:**
```go
if tc, err := s.store.GetTenantConfig(ctx, ...); err == nil && tc != nil {
    status.RetrievalMode = tc.RetrievalMode
}
```

If `GetTenantConfig` fails, `retrieval_mode` is empty — the frontend shows nothing. This could mask a backend connectivity problem.

**Fix:** Log the error. Decide on a default: either omit the field on error (frontend treats omission as `"unknown"`) or return `"unknown"` explicitly so the frontend can show a generic message.

---

### 4. Unused translation keys

**Change 7** adds `reindex-failed-pipeline` and `reindex-failed-no-source` to `en.json`, but neither key is referenced by any frontend change. If these are meant for failed-checkpoint messages, the backend must send a *message code* (not raw English text) so the frontend can look up the locale key. If they're not needed, remove them.

---

### 5. No test coverage for `createFailedCheckpoint`

**Verification step 1** only runs existing tests. The new early-failure paths (nil vectorDB, NoOpVectorDB, zero source files) should have explicit unit tests.

**Fix:** Add test cases:
- `createFailedCheckpoint` writes a checkpoint with `status: "failed"` and the correct message
- Early return before goroutine when pipeline is disabled
- Early return when no source files exist

---

## 🟡 Minor Concerns

| Concern | Suggestion |
|---------|------------|
| `createFailedCheckpoint` uses `context.WithTimeout(context.Background(), 5s)` — if DB is slow, the checkpoint is silently lost | Use `context.WithoutCancel(ctx)` (Go 1.21+) to inherit parent context without cancellation; the checkpoint write should respect a reasonable DB timeout |
| Idle-state JSX uses `bg-white dark:bg-zinc-800` — duplicates parent styling | Use Joy UI `Alert` or `Sheet` component, or just rely on parent container styling |
| Zero chunks → green "success" rendering may confuse users who expected content | Treat 0 chunks as a warning unless it's a no-source-files scenario (by-design) |
| No frontend component tests for the `handleRebuildIndex` branching logic | At minimum unit-test the skip_reason → toast mapping and the idle-state conditional rendering |
| `retrieval_mode` fetched on every status poll — extra query per request | Cache it in the checkpoint record, or batch it into the existing polling query via a join |

---

## Summary

The plan is structurally sound and the category-based UX model is the right approach. The main rework items converge on a single theme: **don't pretend work started when it hasn't.** Detect pre-checkpoint failures synchronously, formalize the `skip_reason` contract, and make sure every code path is tested.

Address the 5 nits, then proceed.
