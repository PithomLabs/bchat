# plan_P0-2_v2 — `forceRAG` Overrides `RetrievalMode` (incorporated code3_review.md)

**Date:** 2026-07-14
**Status:** Ready for implementation

---

## Bugs Fixed

### Bug 1: `forceRAG` overrides explicit `RetrievalMode` (chat flow)
**File:** `service.go:2167-2181`

### Bug 2: `content_tokens = 0` at upload time (misconfiguration)
**File:** `service.go` — in `processChat` after `tenantConfig` fetch (line 2177)

---

## Changes

### Change 1: Rewrite mode decision in `processChat`

**Location:** `service.go:2163-2202`

**Current (broken):**
```go
forceRAG := !config.HasStructuredContent && s.UseRAGPipeline()
if forceRAG {
    useRAG = true
} else if s.UseRAGPipeline() {
    tenantConfig, _ := s.store.GetTenantConfig(...)
    if tenantConfig.RetrievalMode == "rag" { useRAG = true }
}
```

**Proposed:**
```go
useRAG := false
if s.UseRAGPipeline() {
    tenantConfig, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &config.TenantID})

    // Fix misconfigured content_tokens (one-time recalculation)
    if tenantConfig != nil && tenantConfig.ContentTokens == 0 {
        s.recalcContentTokens(ctx, tenantConfig)
    }

    if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
        useRAG = true
    } else if tenantConfig != nil && tenantConfig.RetrievalMode == "long_context" {
        useRAG = false
    } else if !config.HasStructuredContent {
        useRAG = true
    }
}
```

**Error fallback update:**
```go
if useRAG {
    response, genErr = s.generateRAGResponse(...)
    if genErr != nil {
        if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
            return nil, fmt.Errorf("failed to generate response: %w", genErr)
        }
        slog.Warn("RAG generation failed, falling back to long context", ...)
        response, genErr = s.generateResponse(...)
    }
} else {
    response, genErr = s.generateResponse(...)
}
```

Remove `forceRAG` variable entirely.

### Change 2: Add `recalcContentTokens` helper

**File:** `service.go`

```go
func (s *Service) recalcContentTokens(ctx context.Context, tc *store.TenantConfig) {
    files, err := s.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
        TenantID:   &tc.TenantID,
        LatestOnly: true,
    })
    if err != nil || len(files) == 0 {
        return
    }
    var totalTokens int
    for _, f := range files {
        totalTokens += EstimateTokens(f.Content)
    }
    if totalTokens == 0 {
        return
    }
    tc.ContentTokens = int32(totalTokens)
    if totalTokens >= DefaultTokenThreshold {
        tc.RetrievalMode = "rag"
    } else {
        tc.RetrievalMode = "long_context"
    }
    if _, err := s.store.UpsertTenantConfig(ctx, tc); err != nil {
        slog.Warn("failed to persist content_tokens correction", "error", err)
    }
}
```

### Change 3: Add mode decision logging

After the decision block, log the decision for debugging.

### Change 4: Add `TestModeDecision` unit test

**File:** `service_mode_test.go` (new)

7 test cases covering the decision tree.

### code3.md fixes: COALESCE + dead code cleanup

- SQLite/Postgres: `COALESCE(MAX(LENGTH(TRIM(content))), 0)`
- Remove duplicate `audienceType` block in handlers.go

---

## Verification

```bash
go build ./...
go test ./server/router/api/v1/agent/... -count=1
```

Manual:
1. evpn chat → should use long_context (or corrected RAG)
2. RAG tenant → should use RAG
3. No explicit mode + unstructured → should force RAG
