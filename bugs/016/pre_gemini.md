Based on a deep dive into the `bchat` backend, the `widget` source, and your `extract.github.io` Hugo codebase, I can explain exactly why you are seeing this behavior. 

Here is a comprehensive report on the RAG search, audience filtering, and the chat widget integration.

### 1. Why did the Chat Widget say it doesn't have the information?

When you tested the chat widget via `http://localhost:1313/` and asked about Maria Clara, the widget did not query your local `rizal` tenant at all.

In your Hugo site (`extract.github.io`), the active widget configuration is located in [`layouts/partials/custom/head-end.html`](file:///home/chaschel/Documents/go/extract.github.io/layouts/partials/custom/head-end.html). If we look at this file, it is hardcoded to point to the production server and a different tenant:

```html
<script src="https://bchat.fly.dev/widget/scraper/embed.js"></script>
<script>
  AgentChatWidget.init({
    tenant: "scraper",
    baseUrl: "https://bchat.fly.dev",
    // ...
  });
</script>
```

**The issue:** Your local Hugo site is talking to the remote production `bchat.fly.dev` server, querying the **`scraper`** tenant instead of the **`rizal`** tenant on your local machine (`http://localhost:8081`). Because the `scraper` tenant's knowledge base doesn't contain information about Noli Me Tangere / Maria Clara, the LLM correctly replies that it doesn't have that information.

> [!TIP] 
> **How to fix this:** To test your local `rizal` agent in the widget, you need to either temporarily change `head-end.html` or configure your Hugo theme to use `head-end-local.html`, which should be updated to point to `baseUrl: "http://localhost:8081"` and `tenant: "rizal"`.

---

### 2. Is the Chat Widget audience set to "Internal" or "External"?

**The chat widget ALWAYS uses the `external` audience.**

When the widget sends a message, it calls the `sendMessage` function in [`widget/src/core/api.ts`](file:///home/chaschel/Documents/go/bchat/widget/src/core/api.ts), which hits the following endpoint:
`POST /api/v1/agent/:slug/chat/ext`

If we look at how the backend handles this in `HandleChatExternal` and the `ChatExternal` service method in [`server/router/api/v1/agent/service.go`](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go), the audience type is strictly hardcoded to `"external"`:

```go
// ChatExternal handles chat for external (anonymous) users.
func (s *Service) ChatExternal(ctx context.Context, tenantSlug, clientIP, userAgent string, req ChatRequest) (*ChatResponse, error) {
	config, err := s.LoadConfig(ctx, tenantSlug, "external") // <--- Hardcoded here
    // ...
```

---

### 3. Why does the Admin RAG Stats page show 0 results for "Internal" audience?

When you uploaded the KB files for the `rizal` tenant, you uploaded them under the `external` audience tab. During the indexing phase (`ReindexTenantContent`), the chunks are saved into LanceDB with a strict `audience_type` column (e.g., `audience_type = 'external'`).

When you use the "Search Testing" tool in the RAG Stats UI (`HandleTenantRAGSearch`), it performs a strict filter against the vector database based on the dropdown selection:

```go
// In vectordb_lance.go
if query.AudienceType != "" {
    filterParts = append(filterParts, fmt.Sprintf("audience_type = '%s'", query.AudienceType))
}
```

Because you selected "internal" in the dropdown, LanceDB looks for chunks where `audience_type = 'internal'`. Since the documents were only uploaded and indexed for the `external` audience, it returns exactly **0 results**. When you switch the dropdown to "external", the filter matches the `external` chunks, returning the **5 results**.

> [!NOTE]
> **Production Chat Behavior vs. RAG Stats UI**
> Interestingly, the actual internal chat endpoint (`/chat/int` used by human agents in the dashboard) behaves slightly differently than the RAG Stats testing UI. When a human operator uses the internal chat, the RAG retrieval function (`RetrieveContextForQuery`) explicitly removes the audience filter to search *both* internal and external content:
> ```go
> searchAudience := audienceType
> if audienceType == "internal" {
>     searchAudience = "" // Search both to find any production-ready content
> }
> ```
> However, the RAG Stats testing UI explicitly tests the strict isolation of the vector index, which is why it returned 0 results.