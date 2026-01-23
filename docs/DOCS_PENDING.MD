# Pending Work and Known Limitations

**Purpose:** Track outstanding issues, limitations, and planned improvements.
**Format:** Version-controlled by date with newest entries at the top.

---

## 2026-01-23

### Pending Features

| Feature | Description | Priority |
|---------|-------------|----------|
| Comprehensive test suite | Unit and integration tests for Go backend and React frontend | High |
| API documentation | OpenAPI/Swagger specification for all endpoints | Medium |
| Batch simulation runs | Run multiple simulations in parallel with different scenarios | Medium |
| Analysis history dashboard | Historical trends and aggregate metrics across transcripts | Low |

---

### Technical Limitations

#### LLM Provider
- **Single provider support** - Only OpenRouter is supported
- No direct OpenAI, Anthropic, or local LLM integration
- Workaround: Use OpenRouter as proxy for other providers

#### Database
- **SQLite is primary** - PostgreSQL and MySQL drivers exist but are stubs
- Stub implementations return `errNotImplemented`
- Files: `store/db/mysql/agent.go`, `store/db/postgres/agent.go`

#### External Widget
- **No SSE streaming** - Uses request/response pattern
- **Sessions not persisted** - Lost on server restart (in-memory only)
- **Limited customization** - Primary color hardcoded (#0d9488)
- **No file uploads** - Text messages only
- **No typing indicators** - No "user is typing" to agent

#### Internationalization
- **English only** - i18n infrastructure exists but only English translations complete
- Translation file: `web/src/locales/en.json`

---

### Known Issues

#### LanceDB on Linux
- **Requires shared library (.so)**, not static library (.a)
- Caused by BSD/GNU ar incompatibility
- Solution: Build with `-tags rag` uses shared library correctly
- Reference: `Taskfile.yml` build configuration

#### Mock Embeddings
- **Semantically meaningless** - Generates deterministic but random vectors
- Only suitable for pipeline testing, not semantic search quality
- Use `EMBEDDING_PROVIDER=openrouter` for production

#### SCRIPT.MD Not RAG-Indexed
- **By design** - SCRIPT.MD is included fully in system prompt
- Not chunked or embedded like KB.MD and POLICY.MD
- Rationale: Conversation flow guide needs complete context

---

### Technical Debt

#### Code TODOs
| File | Line | Description |
|------|------|-------------|
| `learning.go` | 105 | `TrendPercent: 0, // TODO: calculate trend from historical data` |

#### Stub Implementations
- `store/db/mysql/agent.go` - All agent methods return `errNotImplemented`
- `store/db/postgres/agent.go` - All agent methods return `errNotImplemented`

#### Hardcoded Values
| Value | Location | Description |
|-------|----------|-------------|
| Widget color `#0d9488` | `handlers.go:1104` | Primary color for widget UI |
| Rate limit 60s | `handlers.go:113-117` | Cooldown period for rate limiting |
| Widget dimensions 350x500px | `handlers.go:1273` | Chat panel size |

#### Missing Configuration APIs
- No endpoint to customize widget appearance (color, position, size)
- No endpoint to configure rate limiting per tenant
- No endpoint to set session persistence strategy

---

### Security Considerations

#### API Key Management
- `OPENROUTER_API_KEY` must never be committed to version control
- Tenant-specific keys can be encrypted with `ENCRYPTION_MASTER_KEY`
- If `ENCRYPTION_MASTER_KEY` not set, tenant keys stored unencrypted

#### External Chat Endpoint
- `POST /api/v1/agent/:slug/chat/ext` requires **no authentication**
- Designed for anonymous public users on customer websites
- Protection via IP-based rate limiting only

#### Session Security
- External sessions stored in memory only
- No session token validation beyond `session_id` matching
- Sessions lost on server restart (no persistence for external users)

---

### Future Enhancements

#### RAG Pipeline Improvements
From `DOCS_LANCEDB_PHASE3.MD`:

| Enhancement | Description | Benefit |
|-------------|-------------|---------|
| Hybrid search | Combine vector + BM25 keyword search | Better recall for exact matches |
| Re-ranking | Use cross-encoder for more accurate ranking | Higher precision |
| Query expansion | Expand user queries for better recall | Catch paraphrased queries |
| Caching | Cache frequent queries and embeddings | Reduced latency and cost |
| Analytics | Track retrieval quality metrics | Monitor and improve performance |

#### Widget Enhancements
- SSE streaming for real-time responses
- Customizable appearance via API
- File upload support
- Persistent session storage option
- Multi-language support

#### Platform Improvements
- PostgreSQL/MySQL production support
- Direct LLM provider integrations (bypass OpenRouter)
- Webhook notifications for chat events
- Analytics and reporting dashboard

---

### References

| Document | Description |
|----------|-------------|
| [DOCS_README.MD](./DOCS_README.MD) | Project overview and limitations |
| [DOCS_WIDGET.MD](./DOCS_WIDGET.MD) | Widget limitations and troubleshooting |
| [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) | RAG architecture and future enhancements |
| [CLAUDE.md](../CLAUDE.md) | Development guidelines and pending work |

---

*Last Updated: 2026-01-23*
