# Plan: Internal Notes + RAG-Based Bug Inference

**Bug ID:** 051
**Date:** 2026-07-30
**Status:** Draft — Awaiting Adversarial Review

---

## Background: The Story Behind This Feature

### The Problem

bchat is a multi-tenant AI chat agent platform built on top of Memos. Over 50 bugs (001-050) have been resolved through an intensive iterative process involving plan → adversarial review → code → adversarial review → signoff cycles. Each bug folder contains rich history: root cause analyses, implementation decisions, adversarial findings, and resolution patterns.

However, this knowledge is trapped in local markdown files. When a new bug appears, the agent cannot learn from past resolutions. The same issues (RAG indexing failures, permission errors, migration conflicts) recur because there is no mechanism for the agent to access historical resolution data.

### The Vision

Transform the bug folder history into a living knowledge base that the agent can query. When a new ticket is created, the agent should:

1. Search CockroachDB's distributed vector index for similar past bugs
2. Extract resolution patterns from internal notes
3. Auto-suggest resolution steps based on what worked before
4. Learn from every resolution, improving over time

This is the "agentic memory" that the CockroachDB × AWS Hackathon demands — not toy data, but real production knowledge that makes the agent genuinely useful.

### Why This Matters

The 50 bugs under `bugs/` represent hundreds of hours of adversarial review across multiple AI reviewers (StepFun, DeepSeek, Mimo, hy3, Fable, Kimi, Nemotron, OWL, OpenCode). Each bug went through 2-4 plan iterations and 1-3 code iterations. The resolution patterns are valuable institutional knowledge — but they're locked in local files.

By importing this history as tickets with internal notes, and using CockroachDB vector search for inference, we create a system where:

- New bugs benefit from past resolutions immediately
- Cross-bug patterns are discovered automatically
- The agent becomes more valuable with every bug resolved
- The hackathon submission demonstrates real-world agentic memory at scale

---

## Requirements

### Hackathon Criteria Alignment

| Criterion | Our Approach |
|-----------|-------------|
| **Agentic Memory Design** | CockroachDB stores 130+ tickets with embeddings as persistent agent memory. Vector search finds similar bugs in <200ms. |
| **Technical Implementation** | Distributed Vector Indexing + ccloud CLI. Clean RBAC with internal_notes visibility control. |
| **Real-World Impact** | 50 real bugs with real resolutions. Agent infers solutions from past tickets. Not toy data. |
| **Production Readiness** | Multi-tenant isolation, RBAC, observability, resilience via crdb.ExecuteTx retry. |
| **Creativity & Originality** | Cross-bug pattern analysis. Agent discovers recurring issues across categories. |

### CockroachDB Tools (2 required)

1. **Distributed Vector Indexing** — Store ticket embeddings, semantic search for similar bugs
2. **ccloud CLI** — Cluster provisioning, management, monitoring

### AWS Services (1 required)

1. **Amazon ECS Fargate** — Containerized bchat deployment
2. **Amazon Bedrock** — LLM inference for summary generation
3. **Amazon S3** — Tenant document storage

---

## Technical Design

### 1. Internal Notes Field

#### Database Schema

**Migration:** `store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql`

```sql
ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';
```

**LATEST.sql update:**
```sql
CREATE TABLE tickets (
   id INTEGER PRIMARY KEY AUTOINCREMENT,
   title TEXT NOT NULL,
   description TEXT NOT NULL DEFAULT '',
   status TEXT NOT NULL DEFAULT 'OPEN',
   priority TEXT NOT NULL DEFAULT 'MEDIUM',
   creator_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
   assignee_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
   created_ts BIGINT NOT NULL,
   updated_ts BIGINT NOT NULL,
   type TEXT NOT NULL DEFAULT 'TASK',
   tags TEXT NOT NULL DEFAULT '[]',
   beads_id TEXT UNIQUE,
   parent_id INTEGER REFERENCES tickets(id) ON DELETE CASCADE,
   labels TEXT DEFAULT '[]',
   dependencies TEXT DEFAULT '[]',
   discovery_context TEXT,
   closed_reason TEXT,
   issue_type TEXT,
   tenant_id INTEGER DEFAULT NULL,
   internal_notes TEXT DEFAULT ''  -- NEW
);
```

#### Store Types

**`store/ticket.go`:**

```go
type Ticket struct {
    ID            int32
    Title         string
    Description   string
    Status        TicketStatus
    Priority      TicketPriority
    CreatorID     int32
    AssigneeID    *int32
    CreatedTs     int64
    UpdatedTs     int64
    Type          string
    Tags          []string
    TenantID      *int32
    InternalNotes string  // NEW
}

type UpdateTicket struct {
    ID            int32
    Title         *string
    Description   *string
    Status        *TicketStatus
    Priority      *TicketPriority
    AssigneeID    *int32
    UpdatedTs     *int64
    Type          *string
    Tags          []string
    InternalNotes *string  // NEW
}
```

#### SQLite Driver Changes

**`store/db/sqlite/ticket.go`:**

**CreateTicket (lines 17-48):**
- Add `internal_notes` to INSERT column list (after `tenant_id`)
- Add 12th `?` placeholder
- Bind `create.InternalNotes` as 12th parameter

**ListTickets (lines 87-136):**
- Add `internal_notes` to SELECT column list (after `tenant_id`)
- Add `&ticket.InternalNotes` to Scan call (after `&ticket.TenantID`)

**UpdateTicket (lines 156-226):**
- Add `if update.InternalNotes != nil` block:
  ```go
  if update.InternalNotes != nil {
      set = append(set, "internal_notes = ?")
      args = append(args, *update.InternalNotes)
  }
  ```
- Add `internal_notes` to RETURNING column list
- Add `&ticket.InternalNotes` to Scan call

### 2. RBAC: `ticket:internal_notes` Permission

#### Permission Constant

**`server/router/api/v1/agent/permissions.go`:**

```go
PermTicketInternalNotes = "ticket:internal_notes"
```

#### Visibility Rules

Internal notes are visible to:
1. **HOST/ADMIN** (superusers) — all tickets
2. **Ticket creator** — their own tickets
3. **Assigned users** (`assignee_id`) — tickets assigned to them
4. **Users with `ticket:internal_notes` permission** — all tickets in their tenant

All other users see empty string for `internal_notes`.

#### Permission Preset Mapping

| Preset | `ticket:internal_notes` |
|--------|------------------------|
| Viewer | No |
| Tester | No |
| Analyst | Yes (read) |
| Editor | Yes |
| Tenant Admin | Yes |

#### Handler Changes

**`server/router/api/v1/ticket_service.go`:**

**Ticket response struct (line 14):**
```go
type Ticket struct {
    // ... existing fields ...
    InternalNotes string `json:"internalNotes"`
}
```

**UpdateTicketRequest (line 38):**
```go
type UpdateTicketRequest struct {
    // ... existing fields ...
    InternalNotes *string `json:"internalNotes"`
}
```

**convertTicketFromStore — new signature:**
```go
func convertTicketFromStore(ticket *store.Ticket, user *store.User, hasInternalNotesPerm bool) *Ticket {
    t := &Ticket{
        ID:          ticket.ID,
        Title:       ticket.Title,
        Description: ticket.Description,
        Status:      string(ticket.Status),
        Priority:    string(ticket.Priority),
        CreatorID:   ticket.CreatorID,
        AssigneeID:  ticket.AssigneeID,
        CreatedTs:   ticket.CreatedTs,
        UpdatedTs:   ticket.UpdatedTs,
        Type:        ticket.Type,
        Tags:        ticket.Tags,
    }
    
    // Internal notes visibility
    if isSuperUser(user) ||
        ticket.CreatorID == user.ID ||
        (ticket.AssigneeID != nil && *ticket.AssigneeID == user.ID) ||
        hasInternalNotesPerm {
        t.InternalNotes = ticket.InternalNotes
    }
    
    return t
}
```

**GetTicket handler (line 358):**
```go
// After fetching ticket:
hasPerm, _ := h.service.CheckUserPermission(ctx, userID, *tenantID, PermTicketInternalNotes)
return c.JSON(http.StatusOK, convertTicketFromStore(ticket, user, hasPerm))
```

**ListTickets handler (line 165):**
```go
// For each ticket in list:
hasPerm, _ := h.service.CheckUserPermission(ctx, userID, *tenantID, PermTicketInternalNotes)
result = append(result, convertTicketFromStore(t, user, hasPerm))
```

**UpdateTicket handler (line 252):**
```go
// Only allow internal_notes update if user has permission:
if request.InternalNotes != nil {
    hasPerm, _ := h.service.CheckUserPermission(ctx, userID, *tenantID, PermTicketInternalNotes)
    if isSuperUser(user) || hasPerm {
        update.InternalNotes = request.InternalNotes
    }
}
```

### 3. Frontend Display

**`web/src/pages/TicketDetail.tsx`:**

Add `internalNotes` to Ticket interface:
```typescript
interface Ticket {
    // ... existing fields ...
    internalNotes?: string;
}
```

Add display section below Description:
```tsx
{ticket.internalNotes && (
    <div className="mt-6 w-full">
        <p className="text-sm text-gray-500 mb-2">Internal Notes</p>
        <div className="p-4 border rounded-md whitespace-pre-wrap dark:border-gray-700 bg-yellow-50 dark:bg-yellow-900/20">
            {ticket.internalNotes}
        </div>
    </div>
)}
```

### 4. Bug Folder Import Script

**Script:** `cmd/seed/import_bugs.go`

#### Import Flow

```
For each bug folder (001-050):
  1. Read all .md files in folder
  2. Parse plan.md for:
     - Bug topic (from title or first heading)
     - Key decisions (from "Decisions" or "Proposed Changes" sections)
     - Resolution status (from "Resolved" markers or signoff files)
  3. Parse code.md for:
     - Implementation summary (from "Changes" or "What Was Implemented" sections)
     - Files modified
  4. Parse review.md for:
     - Adversarial findings
     - Resolution patterns
  
  For each phase present:
    - plan*.md → Create "Planning" ticket
    - code*.md → Create "Implementation" ticket
    - testing*.md / review*.md → Create "Testing" ticket
    
    Each ticket:
      1. Create memo with phase content
      2. Create ticket linked to memo
      3. Set status: resolved/in-progress/unknown
      4. Generate internal_notes via LLM with template

After all 50 bugs:
  Create category meta-tickets with cross-bug summaries
```

#### LLM Summary Template

**Per-bug summary:**
```
## Bug {ID}: {Topic}

**Status:** {resolved/in-progress/unknown}
**Files:** {count} files across {phase_count} phases
**Category:** {RAG/Security/Migration/etc}

### Key Decisions
{Extracted from plan.md - bullet list}

### Implementation Summary
{Extracted from code.md - what changed}

### Resolution
{For resolved: what fixed it. For in-progress: current state}

### Lessons Learned
{From review.md - adversarial findings}
```

**Meta-ticket (per category):**
```
## {Category} Patterns Across {count} Bugs

### Common Issues
- {pattern 1}: bugs {list}
- {pattern 2}: bugs {list}

### Resolution Patterns
- {resolution 1}: bugs {list}
- {resolution 2}: bugs {list}

### Recommendations
- {recommendation based on cross-bug analysis}
```

#### Data Volume

| Metric | Value |
|--------|-------|
| Bug folders | 50 |
| Avg phases per bug | 2-3 |
| Tickets per bug | 2-3 |
| **Total tickets** | **~120** |
| Category meta-tickets | 9 |
| **Grand total** | **~130 tickets** |

#### Bug Categories

| Category | Bug IDs | Count |
|----------|---------|-------|
| RAG Pipeline | 001, 004, 017, 032, 034, 035, 037, 038, 049 | 9 |
| Security & RBAC | 013, 018, 019, 021, 022, 025, 030, 043 | 8 |
| SQLite/Postgres Migration | 009, 020, 028, 029, 031, 036, 040, 044, 045, 046 | 10 |
| Deployment | 002, 003, 004, 024, 026, 031, 042 | 7 |
| Tenant Isolation | 010, 011, 021, 048 | 4 |
| Chat Widget & UI | 005, 006, 014, 042, 046 | 5 |
| LLM/Agent Behavior | 012, 015, 016, 017, 039 | 5 |
| Integrations | 033 | 1 |
| CockroachDB | 050 | 1 |
| Testing & QA | 041, 047 | 2 |

### 5. Ticket Resolution Inference (Synchronous)

#### Trigger Point

**`server/router/api/v1/ticket_service.go`:**

In `CreateTicket` handler, after successful creation:
```go
// After s.Store.CreateTicket(ctx, ticket) succeeds:
go s.inferResolutionForNewTicket(ctx, ticket)
```

#### Inference Function

**`server/router/api/v1/agent/service.go`:**

```go
func (s *Service) inferResolutionForNewTicket(ctx context.Context, ticket *store.Ticket) {
    // 1. Embed ticket description + title
    // 2. Search CockroachDB for top-5 similar tickets (similarity > 0.7)
    // 3. Extract internal_notes from matches
    // 4. Format suggested resolution
    // 5. Update ticket's internal_notes
}
```

#### Suggested Resolution Format

```
## Suggested Resolution (Auto-generated)
Based on {count} similar past tickets:

### Ticket #{id} ({similarity}% match)
{internal_notes from that ticket}

### Ticket #{id} ({similarity}% match)
{internal_notes from that ticket}

## Recommended Actions
1. {action from top match}
2. {action from second match}

---
*This suggestion was auto-generated. Please review and update.*
```

#### Fallback

If CockroachDB unavailable or no matches:
- Set `internal_notes` to `"No similar past tickets found. Manual review required."`
- Log warning, do not fail ticket creation

### 6. Ticket Embedder Enhancement

**`server/router/api/v1/agent/ticket_embedder.go`:**

Current:
```go
content := fmt.Sprintf("%s\n%s", ticket.Title, ticket.Description)
```

Enhanced:
```go
content := fmt.Sprintf("%s\n%s\n%s", ticket.Title, ticket.Description, ticket.InternalNotes)
```

---

## Implementation Order

| Step | Task | Files | Est. Time |
|------|------|-------|-----------|
| 1 | Migration | `store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql`, `LATEST.sql` | 15 min |
| 2 | Store types | `store/ticket.go` | 10 min |
| 3 | SQLite driver | `store/db/sqlite/ticket.go` | 30 min |
| 4 | Permission constant | `server/router/api/v1/agent/permissions.go` | 5 min |
| 5 | Ticket service RBAC | `server/router/api/v1/ticket_service.go` | 45 min |
| 6 | Frontend display | `web/src/pages/TicketDetail.tsx` | 15 min |
| 7 | Bug import script | `cmd/seed/import_bugs.go` | 2-3 hours |
| 8 | Ticket embedder | `server/router/api/v1/agent/ticket_embedder.go` | 15 min |
| 9 | Resolution inference | `server/router/api/v1/agent/service.go` | 1-2 hours |
| 10 | Run import + verify | Manual testing | 1 hour |
| **Total** | | | **6-8 hours** |

---

## Verification Plan

| Step | Command | Expected |
|------|---------|----------|
| Build | `go build ./bin/memos/main.go` | Compiles |
| Build (cockroach) | `go build -tags cockroach ./bin/memos/main.go` | Compiles |
| Test | `go test ./store/... -v` | Pass |
| Test tickets | `go test ./server/router/api/v1/... -run TestTicket -v` | Pass |
| Run locally | `task run` | Server starts, migration applies |
| Import bugs | `go run ./cmd/seed/import_bugs.go` | ~130 tickets created |
| Verify RBAC | Create ticket as customer, view as admin | Internal notes visible to admin only |
| Verify inference | Create new ticket, check internal_notes | Auto-populated |

---

## Adversarial Review Prompt

Please review this plan for:

1. **Correctness**: Are the SQL queries correct? Will the migration work on existing databases?
2. **RBAC**: Are the visibility rules properly enforced? Any bypass vectors?
3. **Performance**: Will importing 130 tickets with LLM summaries be too slow? Any bottlenecks?
4. **Error handling**: What happens if LLM fails during import? If CockroachDB is unavailable?
5. **Edge cases**: What if a bug folder is empty (007)? What if files are malformed?
6. **Security**: Can non-admin users see internal notes through API manipulation?
7. **Scalability**: Will vector search scale with 130+ tickets? Any indexing concerns?
8. **Completeness**: Are we missing any files that need modification?

---

## Open Questions

None. All decisions confirmed by user.

---

## References

- Hackathon criteria: `/home/chaschel/Desktop/crdb_hackathon.md`
- Bug folders: `/home/chaschel/Documents/go/bchat/bugs/001-050/`
- RBAC documentation: `/home/chaschel/Documents/go/bchat/docs/DOCS_RBAC_2.MD`
- Ticket schema: `/home/chaschel/Documents/go/bchat/store/migration/sqlite/LATEST.sql:152-172`
- Permission constants: `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/permissions.go`
- Ticket service: `/home/chaschel/Documents/go/bchat/server/router/api/v1/ticket_service.go`
- SQLite ticket driver: `/home/chaschel/Documents/go/bchat/store/db/sqlite/ticket.go`
- Postgres ticket driver: `/home/chaschel/Documents/go/bchat/store/db/postgres/ticket.go`
- Ticket embedder: `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/ticket_embedder.go`
- Frontend ticket detail: `/home/chaschel/Documents/go/bchat/web/src/pages/TicketDetail.tsx`
