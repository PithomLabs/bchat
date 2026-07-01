## Comprehensive RAG Search Report: Chat Widget vs Admin Search Testing

### 1. Chat Widget Audience: **EXTERNAL**

The chat widget is **hardcoded to use the `external` audience**. In `/home/chaschel/Documents/go/extract.github.io/layouts/partials/custom/head-end-prod.html`, the widget is initialized with tenant `"rizal"` and calls the **external** chat endpoint:

```html
<script src="http://localhost:8081/widget/rizal/embed.js"></script>
<script>
  AgentChatWidget.init({
    tenant: "rizal",
    baseUrl: "http://localhost:8081",
    ...
  });
</script>
```

The widget's `sendMessage()` function (in `widget/src/core/api.ts`) posts to:
```
/api/v1/agent/{tenant}/chat/ext
```

This hits `HandleChatExternal`, which loads config with `s.LoadConfig(ctx, tenantSlug, "external")` and then calls `processChat()` with that external config.

---

### 2. Why Internal Returns 0 Results in RAG Stats Search Testing

There are **three possible root causes**, and they are all data/configuration-related rather than a code bug in `SearchQuery` itself.

#### Cause A: Internal content was never indexed for tenant `rizal`

`importFiles()` does call `indexContentForRAG()` for EACH audience type, so in theory both `external` and `internal` content get indexed when files are uploaded. However, the internal index only exists if:
- internal `KB.MD` was actually uploaded through the Agent Admin UI for tenant `rizal`, AND
- `indexContentForRAG()` completed successfully for the `internal` audience.

If you only uploaded files under the **external** audience, there will be zero `internal` chunks. The admin search tester filters strictly by `audience_type`, so it correctly returns 0 hits when nothing is stored under `internal`.

#### Cause B: Reindex endpoint default behavior / comment vs implementation mismatch

`HandleReindexTenant` has a comment stating:

> "Note: Only indexes internal audience content. External audience is never indexed."

That comment is **stale and wrong**. The actual implementation of `ReindexTenantContentWithResume` groups files by `audience_type` and reinserts whatever source files exist. If you triggered reindex with `audience_type=all`, it should process both audiences. If you triggered it without specifying audience, it defaults to `all`, but if `internal` source files are missing, still nothing gets indexed.

#### Cause C: The widget's external chat does not use RAG for this tenant

`processChat()` determines whether to use RAG like this:

```go
forceRAG := !config.HasStructuredContent && s.UseRAGPipeline()
if forceRAG {
    useRAG = true
} else if s.UseRAGPipeline() {
    tenantConfig, _ := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &config.TenantID})
    if tenantConfig != nil && tenantConfig.RetrievalMode == "rag" {
        useRAG = true
    }
}
```

For unstructured content (no structured services/FAQs/etc.), `HasStructuredContent` is `false`, so `forceRAG = true` whenever the global `RAG_PIPELINE_ENABLED=true` is set. This means the widget **should** use RAG. But if the `external` index is also empty or the query's `MinScore` threshold filters everything out, the LLM falls back to the "I don't have information..." response.

---

### 3. Complete RAG Flow for the Chat Widget

```
User types message in widget
    ↓
POST /api/v1/agent/rizal/chat/ext
    ↓
HandleChatExternal
    ↓ LoadConfig(ctx, "rizal", "external")
    ↓ processChat()
        ↓ [audience = "external"]
        ↓ useRAG = true (unstructured + RAG enabled)
        ↓ generateRAGResponse()
            ↓ RetrieveContextForQuery(ctx, db, query, intent, tenantID, "external", hybridOpts)
                ↓ SearchQuery{TenantID, AudienceType: "external", ActiveOnly: true, TopK: 10, MinScore: 0.25}
                ↓ db.Search(ctx, searchQuery)
                    ↓ buildFilter() -> "tenant_id = X AND is_active = true"
                    ↓ [hybrid or vector-only search]
                ↓ Returns chunks where audience_type = "external"
            ↓ Build system prompt with retrieved chunks
            ↓ Call LLM
    ↓ Return response to widget
```

---

### 4. Key Code Locations

| File | Relevant Code | Purpose |
|------|---------------|---------|
| `widget/src/core/api.ts` | `sendMessage()` | Widget POSTs to `/chat/ext` |
| `handlers.go:1467` | `HandleChatExternal()` | External chat handler |
| `service.go:1468` | `LoadConfig(ctx, tenantSlug, "external")` | Widget always loads **external** config |
| `service.go:1814-1828` | `useRAG` logic | Determines if RAG is used |
| `service.go:2543-2585` | `generateRAGResponse()` | RAG response generation |
| `vectordb.go:808-857` | `RetrieveContextForQuery()` | Builds `SearchQuery` for retrieval |
| `vectordb.go:870-891` | `buildFilter()` | Constructs LanceDB filter string |
| `handlers.go:4350-4375` | `HandleTestRAGSearch()` | Admin search tester — respects `AudienceType` exactly |
| `handlers.go:1086-1112` | `HandleReindexTenant()` | Reindexes by audience |
| `service.go:657-856` | `ReindexTenantContentWithResume()` | Groups by audience and reinserts |

---

### 5. Why the Same Query Returns Results for External but Not Internal

**The admin search tester correctly filters by exact `audience_type`.** The zero internal results mean one or more of:

1. **No internal KB/policy files uploaded** for tenant `rizal`. The internal audience may simply not exist in the database.
2. **Internal files exist but were never indexed** — `indexContentForRAG()` writes chunks tagged with `audience_type = "internal"`. If indexing failed silently (network error, embedding API issue), no internal chunks exist.
3. **Internal index was deleted** — if `ReindexTenantContent` was called with `audience_type=external`, it runs `vectorDB.Delete(ctx, tenantID, "external")` for external only; it does not delete internal. But if a full reindex targeted only external, internal stays untouched.

**To diagnose**, run:

```bash
# Check what audiences/chunks exist in LanceDB
sqlite3 build/data/memos_dev.db "SELECT audience_type, content_type, COUNT(*) FROM agent_source_files WHERE tenant_id = <RIZAL_TENANT_ID> GROUP BY audience_type, content_type;"

# Or in LanceDB directly if using local storage
ls build/data/lancedb/
```

Then in the RAG Stats page, click the tenant row to see **Chunks by Audience** — if `internal` shows 0 chunks, that is the definitive answer.

---

### 6. Important Behavioral Note: External Chat vs Internal Search

The chat widget uses `external` audience. When you test in the admin panel with `audience=external` and get 5 results, those are the **exact same chunks** the widget retrieves at query time. The widget's "I don't have information..." response happens when the RAG search in `generateRAGResponse()` returns **no chunks above `MinScore=0.25`**, OR when the LLM decides the retrieved context doesn't contain the answer. The admin tester uses `MinScore=0.0` by default (`HandleTenantRAGSearch`), so it shows more permissive results than the live chat path.