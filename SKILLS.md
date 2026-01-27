# Skills Quick Reference

Quick reference for common tasks in this codebase. Use this for fast execution.

---

## Quick Commands

```bash
# Development (recommended)
task run:rag                     # OpenRouter embeddings (needs OPENROUTER_API_KEY)

# Build
task build                       # Frontend + backend
task build:rag                   # With LanceDB support

# Other run options
task run:rag:mock                # Pipeline testing only (not semantically accurate)
```

---

## File Locations

| Task | Files |
|------|-------|
| API handlers | `server/router/api/v1/agent/handlers.go` |
| Business logic | `server/router/api/v1/agent/service.go` |
| Routes | `server/router/api/v1/v1.go` |
| DB types | `store/agent.go` |
| DB interface | `store/driver.go` |
| SQLite impl | `store/db/sqlite/agent.go` |
| Migrations | `store/migration/sqlite/0.25/` |
| Frontend store | `web/src/store/v2/agentAdmin.ts` |
| Frontend page | `web/src/pages/AgentAdmin.tsx` |
| Translations | `web/src/locales/en.json` |
| Embeddings | `server/router/api/v1/agent/embedding.go` |
| Chunking | `server/router/api/v1/agent/chunker.go` |
| Vector DB | `server/router/api/v1/agent/vectordb.go` |
| Processing | `server/router/api/v1/agent/processor.go` |

---

## Database Queries

```bash
# List tenants
sqlite3 build/data/memos_dev.db "SELECT id, slug, company_name FROM agent_tenants;"

# Get tenant by slug
sqlite3 build/data/memos_dev.db "SELECT * FROM agent_tenants WHERE slug='inc';"

# List source files for tenant
sqlite3 build/data/memos_dev.db "SELECT id, audience_type, file_type, length(content), version FROM agent_source_files WHERE tenant_id=4 ORDER BY file_type, version DESC;"

# Get latest KB content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_source_files WHERE tenant_id=<ID> AND file_type='kb' ORDER BY id DESC LIMIT 1;"

# Get latest POLICY content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_source_files WHERE tenant_id=<ID> AND file_type='policy' ORDER BY id DESC LIMIT 1;"

# Get SCRIPT content
sqlite3 build/data/memos_dev.db "SELECT content FROM agent_tenant_scripts WHERE tenant_id=<ID> ORDER BY id DESC LIMIT 1;"

# List all tables
sqlite3 build/data/memos_dev.db ".tables"

# Show table schema
sqlite3 build/data/memos_dev.db ".schema agent_tenants"
```

---

## Add New API Endpoint

1. **Handler** (`handlers.go`):
```go
func (h *Handler) HandleMyEndpoint(c echo.Context) error {
    ctx := c.Request().Context()
    slug := c.Param("slug")

    if !h.isAdmin(c) {
        return echo.NewHTTPError(http.StatusForbidden, "Admin required")
    }

    tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
    if err != nil || tenant == nil {
        return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
    }

    // Logic here
    return c.JSON(http.StatusOK, map[string]interface{}{"success": true})
}
```

2. **Route** (`v1.go`):
```go
adminGroup.POST("/:slug/my-endpoint", s.agentHandler.HandleMyEndpoint)
adminGroup.GET("/:slug/my-endpoint", s.agentHandler.HandleMyEndpoint)
```

---

## Add New Database Table

1. **Migration** (`store/migration/sqlite/0.25/NN__name.sql`):
```sql
CREATE TABLE IF NOT EXISTS my_table (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    content TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_my_table_tenant ON my_table(tenant_id);
```

2. **Types** (`store/agent.go`):
```go
type MyTable struct {
    ID        int32
    TenantID  int32
    Content   string
    CreatedAt time.Time
}

type FindMyTable struct {
    ID       *int32
    TenantID *int32
}
```

3. **Interface** (`store/driver.go`):
```go
CreateMyTable(ctx context.Context, item *MyTable) (*MyTable, error)
GetMyTable(ctx context.Context, find *FindMyTable) (*MyTable, error)
```

4. **SQLite** (`store/db/sqlite/agent.go`):
```go
func (d *DB) CreateMyTable(ctx context.Context, item *store.MyTable) (*store.MyTable, error) {
    // Implementation
}
```

5. **Stubs** (`store/db/mysql/agent.go`, `store/db/postgres/agent.go`):
```go
func (d *DB) CreateMyTable(ctx context.Context, item *store.MyTable) (*store.MyTable, error) {
    return nil, errNotImplemented
}
```

---

## Add Frontend Store Method

```typescript
// In web/src/store/v2/agentAdmin.ts

const myNewMethod = async (slug: string, data: any): Promise<{ success: boolean; error?: string }> => {
  state.setPartial({ isSaving: true, error: null });
  try {
    const response = await axios.post(`/api/v1/agent/${slug}/my-endpoint`, data);
    runInAction(() => {
      state.isSaving = false;
    });
    return { success: true };
  } catch (error: any) {
    runInAction(() => {
      state.isSaving = false;
      state.error = error.response?.data?.message || "Failed";
    });
    return { success: false, error: error.response?.data?.message };
  }
};

// Add to return object:
return {
  state,
  myNewMethod,  // <-- Add here
  // ...
};
```

---

## Add Translation

```json
// web/src/locales/en.json
{
  "agent-admin": {
    "my-feature-title": "My Feature",
    "my-feature-desc": "Description here"
  }
}
```

Usage in React:
```tsx
const t = useTranslate();
<span>{t("agent-admin.my-feature-title")}</span>
```

---

## Common Patterns

### Check Admin Role
```go
if !h.isAdmin(c) {
    return echo.NewHTTPError(http.StatusForbidden, "Admin role required")
}
```

### Check Permission
```go
if !h.hasPermission(c, tenant.ID, "chat:test") {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
}
```

### Get Tenant from Slug
```go
tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
if err != nil || tenant == nil {
    return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
}
```

### Parse JSON Body
```go
var req MyRequest
if err := c.Bind(&req); err != nil {
    return echo.NewHTTPError(http.StatusBadRequest, "Invalid request: "+err.Error())
}
```

### Logging
```go
slog.Info("Operation completed", "tenant", slug, "count", count)
slog.Error("Operation failed", "error", err, "tenant", slug)
slog.Debug("Debug info", "data", data)
```

---

## Gotchas

| Issue | Solution |
|-------|----------|
| Table name wrong | Use **plural**: `agent_tenants` not `agent_tenant` |
| Env vars not working in Taskfile | Use inline: `VAR=value ./binary` not `env:` block |
| Frontend state not updating | Wrap in `runInAction()` |
| Store method not accessible | Add to return object |
| Mock embeddings not semantic | Use `openrouter` instead |
| Migration not running | Check filename: `NN__snake_case.sql` |
| CGO errors | Run `task setup:lancedb` first |

---

## Environment Variables (Quick)

```bash
# Minimum for development
OPENROUTER_API_KEY=sk-or-v1-xxx
EMBEDDING_PROVIDER=openrouter
RAG_PIPELINE_ENABLED=true

# Full production
OPENROUTER_API_KEY=sk-or-v1-xxx
LLM_MODEL=openai/gpt-4o-mini
EMBEDDING_PROVIDER=openrouter
EMBEDDING_MODEL=openai/text-embedding-3-small
RAG_PIPELINE_ENABLED=true
LANCEDB_STORAGE_PROVIDER=local
```

---

## Embedding Providers

| Provider | Command | Semantic | Cost |
|----------|---------|----------|------|
| `openrouter` | `task run:rag` | Yes | API |
| `mock` | `task run:rag:mock` | No | Free |
| `local` | Custom server required | Yes | Free |

---

## Useful Grep Patterns

```bash
# Find handler
grep -n "func.*Handle" server/router/api/v1/agent/handlers.go

# Find route registration
grep -n "adminGroup\|authGroup" server/router/api/v1/v1.go

# Find store method
grep -n "func.*Store" store/agent.go

# Find translation key usage
grep -rn "agent-admin.my-key" web/src/
```

---

## Testing Endpoints

```bash
# Validate tenant
curl http://localhost:5230/api/v1/agent/inc/validate

# Reindex tenant (needs auth cookie)
curl -X POST http://localhost:5230/api/v1/agent/inc/reindex

# RAG stats
curl http://localhost:5230/api/v1/admin/rag/stats

# Test RAG search
curl -X POST http://localhost:5230/api/v1/admin/rag/search \
  -H "Content-Type: application/json" \
  -d '{"query": "water damage", "tenant_id": 4, "limit": 5}'
```

---

*Quick reference for Claude Code context. See CLAUDE.md for full details.*
