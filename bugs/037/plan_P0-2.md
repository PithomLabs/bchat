# Plan P0-2 — `forceRAG` Overrides Tenant's Explicit `RetrievalMode`

**Date:** 2026-07-14
**Status:** Ready for approval — no coding until explicit go-ahead
**Scope:** Chat flow mode-selection fix + upload-time token recalculation

---

## Problem Statement

The evpn tenant has `retrieval_mode = "long_context"` but the agent runs in RAG mode
during chat. Since no RAG index was built (reindex correctly skipped), the vector DB
returns nothing and the agent says "retrieved context does not provide."

**Reproduction:**
1. evpn tenant: 363K tokens of plain ExpressVPN markdown (no `@service`/`@faq` annotations)
2. `content_tokens = 0` in DB → mode set to `long_context` at upload time
3. User asks "what is ISP throttling"
4. Agent responds: "the specific text you quoted does not appear in the retrieved context"

---

## Root Cause Analysis

### Bug 1: `forceRAG` overrides `RetrievalMode` (the chat bug)

**File:** `server/router/api/v1/agent/service.go:2167-2181`

```go
forceRAG := !config.HasStructuredContent && s.UseRAGPipeline()  // LINE 2168
if forceRAG {
    useRAG = true  // RetrieverMode NEVER consulted
} else if s.UseRAGPipeline() {
    tenantConfig, _ := s.store.GetTenantConfig(...)
    if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
        useRAG = true  // only path that checks RetrieverMode
    }
}
```

**Chain:**
1. `HasStructuredContent = false` (plain markdown, no annotations)
2. `forceRAG = true`
3. `RetrievalMode` check at line 2178 is **dead code** — never reached when `forceRAG = true`
4. Agent enters RAG mode → empty retrieval → confused response

### Bug 2: `content_tokens = 0` at upload time (misconfiguration)

**File:** `server/router/api/v1/agent/handlers.go:1123-1135`

The upload handler correctly calculates tokens and sets mode:
```go
totalTokens := EstimateTokens(kbContent) + EstimateTokens(policyContent)
retrievalMode := "long_context"
if totalTokens >= DefaultTokenThreshold {
    retrievalMode = "rag"
}
tenantConfig.ContentTokens = int32(totalTokens)
```

But evpn's `content_tokens = 0` means this code path never executed for this tenant.
The token calculation ran but the result was not persisted (possibly an earlier version
of the upload handler, or a direct DB insert that bypassed the handler).

---

## Design Principles

1. **Explicit > Implicit** — If a tenant has an explicit `RetrievalMode`, respect it
2. **Unstructured content with RAG enabled defaults to RAG** — The `forceRAG` fallback
   is correct for tenants with no explicit mode preference
3. **Long context mode should work even for large KBs** — If the user explicitly chose
   long_context, the system should attempt it (with appropriate warnings)

---

## Changes

### Change 1: Respect explicit `RetrievalMode` in `processChat`

**File:** `server/router/api/v1/agent/service.go`
**Location:** Lines 2163-2181

**Current logic (broken):**
```
1. forceRAG = !HasStructuredContent && UseRAGPipeline()  → overrides everything
2. else if UseRAGPipeline() && RetrieverMode == "rag"    → only checked if !forceRAG
3. else → long_context
```

**Proposed logic:**
```go
useRAG := false
if s.UseRAGPipeline() {
    tenantConfig, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &config.TenantID})
    if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
        // Explicit RAG — respect it
        useRAG = true
    } else if tenantConfig != nil && tenantConfig.RetrievalMode == "long_context" {
        // Explicit long_context — respect it (user chose this)
        useRAG = false
    } else if !config.HasStructuredContent {
        // No explicit mode + unstructured content → force RAG
        // (existing behavior for tenants without RetrieverMode)
        useRAG = true
        slog.Debug("forcing RAG mode for unstructured content (no explicit retrieval mode)",
            "tenant_slug", config.TenantSlug,
            "session_id", session.ID)
    }
}
```

**Decision tree after fix:**

| RetrieverMode | HasStructuredContent | RAG Enabled | Result |
|---------------|---------------------|-------------|--------|
| `"rag"` | any | yes | RAG |
| `"rag"` | any | no | long_context (RAG unavailable) |
| `"long_context"` | any | any | long_context |
| `""` (unset) | true | yes | long_context |
| `""` (unset) | true | no | long_context |
| `""` (unset) | false | yes | RAG (forceRAG) |
| `""` (unset) | false | no | long_context |

**Also update the fallback logic (lines 2187-2198):**

The `forceRAG` variable is no longer used for the `RetrievalMode` check, but it's still
used in the error path to decide whether fallback is allowed. Replace with a new variable:

```go
canFallback := !useRAG || (tenantConfig != nil && tenantConfig.RetrievalMode == "long_context")
```

Actually, simpler: `canFallback` should be true when the mode was NOT explicitly forced
to RAG by the tenant config. Let me rethink:

```go
if useRAG {
    response, genErr = s.generateRAGResponse(...)
    if genErr != nil {
        // Can only fall back to long_context if:
        // 1. We're not in explicit RAG mode, OR
        // 2. We entered RAG due to forceRAG (unstructured content)
        if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
            // Explicit RAG — no fallback
            slog.Error("RAG generation failed for explicit rag mode, no fallback",
                "error", genErr, "session_id", session.ID)
            return nil, fmt.Errorf("failed to generate response: %w", genErr)
        }
        // forceRAG or unset — fallback to long_context
        slog.Warn("RAG generation failed, falling back to long context",
            "error", genErr, "session_id", session.ID)
        response, genErr = s.generateResponse(ctx, config, session, classification, decision)
    }
} else {
    response, genErr = s.generateResponse(...)
}
```

**Note:** The `forceRAG` variable is removed entirely. The decision is now based on
`tenantConfig.RetrievalMode` and `HasStructuredContent` as fallback.

---

### Change 2: Recalculate tokens when `content_tokens = 0`

**File:** `server/router/api/v1/agent/service.go`
**Location:** Inside `LoadConfig()` (around line 1608)

**Why:** The evpn tenant has `content_tokens = 0` because the upload handler's token
calculation never ran. This means `RetrievalMode = "long_context"` was set by default
(0 < threshold), not by actual content size. For the evpn KB (363K tokens), RAG is
the correct mode.

**Proposed:** Add a recalculation check at the start of `LoadConfig()`:

```go
// Fix misconfigured content_tokens: recalculate if zero but files exist.
// This handles cases where the upload handler's token calculation was bypassed.
if tenantConfig.ContentTokens == 0 {
    files, err := s.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
        TenantID:   &tenant.ID,
        AudienceType: &audienceType,
        LatestOnly: true,
    })
    if err == nil && len(files) > 0 {
        var totalTokens int
        for _, f := range files {
            totalTokens += EstimateTokens(f.Content)
        }
        if totalTokens > 0 {
            slog.Warn("recalculating content_tokens (was 0, now correcting)",
                "tenant_id", tenant.ID,
                "old_tokens", tenantConfig.ContentTokens,
                "new_tokens", totalTokens,
            )
            tenantConfig.ContentTokens = int32(totalTokens)
            // Update mode based on actual content size
            if totalTokens >= DefaultTokenThreshold {
                tenantConfig.RetrievalMode = "rag"
            } else {
                tenantConfig.RetrievalMode = "long_context"
            }
            // Persist the correction
            if _, err := s.store.UpsertTenantConfig(ctx, tenantConfig); err != nil {
                slog.Warn("failed to persist content_tokens correction", "error", err)
            }
        }
    }
}
```

**Note:** This is a one-time correction. Once `content_tokens > 0`, the recalculation
never runs again. The fix is idempotent.

---

### Change 3: Log the mode decision in processChat

**File:** `server/router/api/v1/agent/service.go`
**Location:** After the mode decision block

```go
slog.Info("chat mode decision",
    "tenant_id", config.TenantID,
    "retrieval_mode", func() string {
        if tc, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &config.TenantID}); tc != nil {
            return tc.RetrievalMode
        }
        return ""
    }(),
    "has_structured_content", config.HasStructuredContent,
    "use_rag", useRAG,
    "rag_enabled", s.UseRAGPipeline(),
    "session_id", session.ID,
)
```

**Note:** This is a debug aid. The `GetTenantConfig` call is redundant with the one
already made in the decision block, but the log is valuable for diagnosing mode issues.

---

## Files to Modify

| File | Changes |
|------|---------|
| `service.go:2167-2181` | Rewrite mode decision: check `RetrievalMode` first, then `HasStructuredContent` fallback |
| `service.go:2187-2198` | Update error fallback logic: explicit RAG = no fallback |
| `service.go:LoadConfig()` | Add `content_tokens = 0` recalculation |
| `service.go:~2202` | Add mode decision logging |

---

## Verification

### Step 1: Compile and test
```bash
go build ./...
go test ./server/router/api/v1/agent/... -count=1
```

### Step 2: Manual test — evpn tenant (long_context)
```bash
task run:rag
# Select evpn tenant in Agent Admin
# Send: "What is ISP throttling?"
# Expect: Agent responds with content from the KB (not "retrieved context does not provide")
# Verify in logs: "chat mode decision" shows use_rag=false
```

### Step 3: Manual test — RAG tenant
```bash
# Select a tenant with rag mode and indexed content
# Send a question about the KB
# Expect: Agent responds using RAG retrieval
# Verify in logs: "chat mode decision" shows use_rag=true
```

### Step 4: Manual test — no explicit mode (forceRAG)
```bash
# Select a tenant with no RetrieverMode set and unstructured content
# Expect: forceRAG activates (existing behavior preserved)
```

### Step 5: Verify content_tokens correction
```bash
# After evpn chat test, check DB:
sqlite3 build/data/memos_dev.db "SELECT content_tokens, retrieval_mode FROM tenant_config WHERE tenant_id=13;"
# Expect: content_tokens > 0, retrieval_mode = "rag" (corrected from 363K tokens)
```

---

## Risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Existing tenants with `RetrievalMode = "long_context"` may get long_context even for large KBs | Medium — may hit token limits | The mode was explicitly chosen; add warning in logs |
| `content_tokens` recalculation runs on every chat request for misconfigured tenants | Low — only runs once, then persists | Idempotent: `content_tokens > 0` skips the block |
| Removing `forceRAG` variable changes error fallback behavior | Low — fallback is now based on explicit mode | Explicit RAG = no fallback (correct); forceRAG/unset = fallback (preserved) |
| Extra `GetTenantConfig` call in mode decision | Low — cached by store | Single DB call per request |

---

## Rollback

Revert the 4 change locations in `service.go`. No data model changes. No migrations needed.
