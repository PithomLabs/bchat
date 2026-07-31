# bugs/052 Summary: Per-Ticket RAG Indexing Implementation

## Overview
Per-ticket RAG indexing for Jira-style "Ask Rovo" feature. Each ticket's content is indexed into the vector DB so that `InferResolutionForNewTicket` can surface similar historical tickets and bug sections when a new ticket is created.

## Architecture

```
Ticket Created/Updated/Commented
  └── IndexTicketContent (background goroutine)
        ├── Build content blob: title + description + comments
        ├── Per-ticket mutex + content-hash dedup
        ├── UpsertAgentSourceFile (file_type="ticket")
        ├── ReindexFileVersion (chunk + embed + LanceDB insert)
        └── triggerInference? → InferResolutionForNewTicket
```

## Entry Points / Hooks

| Event | File | Line | triggerInference |
|-------|------|------|-----------------|
| Ticket created | `ticket_service.go` | ~177 | `true` |
| Ticket deduped | `ticket_service.go` | ~156 | `false` |
| Ticket updated | `ticket_service.go` | ~379 | `false` |
| Comment created | `memo_service.go` | ~665 | `false` |
| Comment edited | `memo_service.go` | ~441 | `false` |

All hooks:
- Run in a `go func()` with `context.WithoutCancel(ctx)` (fire-and-forget).
- Guarded by `s.agentHandler != nil` and `TenantID != nil`.
- Extract tenant via context helper; skip indexing for unscoped/nil tenants.

## Key Implementation Details

- **`IndexTicketContent`** (`service.go:5708`): builds a markdown blob, computes `ContentHash`, queries latest `AgentSourceFile` by `(tenantID, file_type="ticket", LatestOnly=true)`, skips upsert if hash matches, otherwise upserts and calls `ReindexFileVersion`.
- **Content-hash dedup**: prevents version bloat when re-indexing unchanged content.
- **Per-ticket mutex** (`ticketIndexMu sync.Map`): serializes check+upsert per ticket to prevent TOCTOU races.
- **Chained inference**: `InferResolutionForNewTicket` is called *after* successful indexing, not in parallel.
- **`getTicketComments`** (`ticket_service.go:549`): resolves parent memo from `ticket.Description = "/m/<uid>"`, fetches `COMMENT` relations, returns comment memos.

## What Gets Indexed

```
# {ticket.Title}

{ticket.Description}

## Comments

---

{comment1.Content}

---

{comment2.Content}
```

## What Gets Written Back

`InferResolutionForNewTicket` merges chunks from:
- `ContentTypes: ["ticket"]` — similar tickets in same tenant
- `ContentTypes: ["bug_section"]` — bug history corpus

It writes the merged suggestion into `ticket.InternalNotes`.

## Importantly: internal_notes Update Behavior

- **Ticket creation**: triggers inference → `internal_notes` updated with suggestions.
- **Ticket update / comment created / comment edited**: only re-indexes RAG; **does NOT update `internal_notes`**.

This is by design — `triggerInference` is `false` on all edit hooks to avoid overwriting human notes and to prevent feedback loops.

Tradeoff: `internal_notes` can become stale after edits. A future periodic sweep or edit-time re-inference could refresh it.

## Tenant Isolation

Every new store query passes `TenantID` explicitly. Nil-tenant requests are skipped entirely.

## Rollback

```sql
DELETE FROM agent_source_files WHERE file_type='ticket' AND tenant_id=19;
git checkout server/router/api/v1/agent/service.go \
  server/router/api/v1/ticket_service.go \
  server/router/api/v1/memo_service.go
```

## Status

- Build: passes
- Tests: pass
- Approved with 1 nit: `memo_service_test.go` claimed in docs but not present in repo.
