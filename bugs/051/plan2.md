# Plan2: Internal Notes + RAG-Based Bug Inference (Revised)

**Bug ID:** 051
**Date:** 2026-07-30
**Status:** Revised — Post-Adversarial Review
**Revision:** plan2.md (incorporates all valid findings from plan_review.md)

---

## Changelog: plan.md → plan2.md

| Finding | Source | Resolution |
|---------|--------|------------|
| 1. Postgres Migration Missing | plan_review.md #1 | **Added** — Full Postgres parity with CockroachDB compatibility |
| 2. `CheckUserPermission` doesn't exist | plan_review.md #2 | **Fixed** — Use `agent.ResolveEffectivePermissions()` |
| 3. `convertTicketFromStore` signature breaks compilation | plan_review.md #3 | **Fixed** — Keep single-arg, filter in each handler |
| 4. `cmd/seed/` package conflict | plan_review.md #4 | **Fixed** — Move to `cmd/import-bugs/main.go` |
| 5. LLM dependency during import | plan_review.md #5 | **Fixed** — Async worker pool with fallback |
| 6. Cross-tenant vector search isolation | plan_review.md #6 | **Fixed** — Add `tenant_id` filter to vector search |
| 7. `AllPermissions`/`PermissionPresets` not updated | plan_review.md #7 | **Fixed** — Add to both |
| 8. RBAC check in ListTickets loop | plan_review.md #8 | **Fixed** — Resolve once before loop |
| 9. `internalNotes` sensitivity in embeddings | plan_review.md #9 | **Documented** — Trade-off note added |
| 10. No tests mentioned | plan_review.md #10 | **Added** — Test plan section |
| 11. Postgres migration validation | plan_review.md #11 | **Added** — `task validate:schema` step |
| 12. Import script versioning | plan_review.md #12 | **No change** — 0.35 is correct |
| 13. `discovery_context` relationship | plan_review.md #13 | **Clarified** — Complementary fields |

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

**SQLite Migration:** `store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql`

```sql
ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';
```

**Postgres Migration:** `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql`

```sql
ALTER TABLE tickets ADD COLUMN internal_notes TEXT DEFAULT '';
```

**CockroachDB Compatibility:** `ALTER TABLE ... ADD COLUMN` is standard SQL supported by CockroachDB. No extensions or special syntax needed. This migration works identically on SQLite, Postgres, and CockroachDB.

**LATEST.sql updates:**

SQLite (`store/migration/sqlite/LATEST.sql`, line 171):
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

Postgres (`store/migration/postgres/LATEST.sql`, line 660):
```sql
CREATE TABLE tickets (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'OPEN',
    priority TEXT NOT NULL DEFAULT 'MEDIUM',
    creator_id INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    assignee_id INTEGER REFERENCES "user"(id) ON DELETE SET NULL,
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

**Relationship: `discovery_context` vs `internal_notes`**

These are complementary fields, not replacements:
- `discovery_context` — System-generated context about how a ticket was discovered (e.g., "Auto-created from chat escalation")
- `internal_notes` — Human-written or AI-generated annotations about resolution steps, root cause, lessons learned

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

#### Postgres Driver Changes

**`store/db/postgres/ticket.go`:**

Same changes as SQLite, using `$N` parameter syntax:

**CreateTicket (lines 12-52):**
- Add `internal_notes` to INSERT column list
- Change `$11` to `$12` for `tenant_id`, add `$12` for `internal_notes`
- Bind `create.InternalNotes` after `create.TenantID`

**ListTickets (lines 96-151):**
- Add `internal_notes` to SELECT column list
- Add `&ticket.InternalNotes` to Scan call

**UpdateTicket (lines 165-244):**
- Add `if update.InternalNotes != nil` block with `fmt.Sprintf("internal_notes = $%d", argCounter)`
- Add `internal_notes` to RETURNING column list
- Add `&ticket.InternalNotes` to Scan call

### 2. RBAC: `ticket:internal_notes` Permission

#### Permission Constant

**`server/router/api/v1/agent/permissions.go`:**

```go
const (
    // ... existing constants ...
    PermTicketInternalNotes = "ticket:internal_notes"
)
```

#### AllPermissions Update

```go
var AllPermissions = []string{
    PermTenantAdmin, PermTenantRead, PermTenantWrite,
    PermChatTest, PermChatLogs,
    PermFilesUpload, PermFilesRestore, PermAPIConfig,
    PermTicketInternalNotes,  // NEW
}
```

#### PermissionPresets Update

```go
var PermissionPresets = map[string][]string{
    "viewer":       {PermTenantRead},
    "tester":       {PermTenantRead, PermChatTest},
    "analyst":      {PermTenantRead, PermChatLogs, PermTicketInternalNotes},  // Added
    "editor":       {PermTenantRead, PermTenantWrite, PermFilesUpload, PermTicketInternalNotes},  // Added
    "tenant_admin": {PermTenantAdmin, PermTicketInternalNotes},  // Added
}
```

#### Visibility Rules

Internal notes are visible to:
1. **HOST/ADMIN** (superusers) — all tickets
2. **Ticket creator** — their own tickets
3. **Assigned users** (`assignee_id`) — tickets assigned to them
4. **Users with `ticket:internal_notes` permission** — all tickets in their tenant

All other users see empty string for `internal_notes`.

#### Permission Check Mechanism

**NOT** `CheckUserPermission` (doesn't exist). Use:

```go
import "github.com/usememos/memos/server/router/api/v1/agent"

// In handler:
resolvedPerms, err := agent.ResolveEffectivePermissions(ctx, s.Store, tenantID, userID)
hasInternalNotesPerm := agent.ContainsPermission(resolvedPerms, agent.PermTicketInternalNotes)
```

**Note:** `ContainsPermission` takes `[]ResolvedPermission` (not `[]string`). The existing `ContainsPermission` function (line 65) is deprecated but works. For new code, use `containsResolvedPermission` (line 84) — but it's unexported. Either export it or use the deprecated version.

**Recommended approach:** Add a public helper:

```go
// In permissions.go:
func HasPermission(permissions []ResolvedPermission, required string) bool {
    return containsResolvedPermission(permissions, required)
}
```

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

**`convertTicketFromStore` — KEEP ORIGINAL SIGNATURE:**
```go
func convertTicketFromStore(ticket *store.Ticket) *Ticket {
    return &Ticket{
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
        InternalNotes: ticket.InternalNotes,  // Always include; filter in handler
    }
}
```

**RBAC filtering in each handler (after conversion):**

Helper function:
```go
func filterInternalNotes(resp *Ticket, ticket *store.Ticket, user *store.User, hasPerm bool) {
    if isSuperUser(user) || ticket.CreatorID == user.ID ||
        (ticket.AssigneeID != nil && *ticket.AssigneeID == user.ID) || hasPerm {
        return  // Keep internal notes
    }
    resp.InternalNotes = ""  // Hide
}
```

**GetTicket handler (line 358):**
```go
// After fetching ticket:
resolvedPerms, _ := agent.ResolveEffectivePermissions(ctx, s.Store, *tenantID, userID)
hasPerm := agent.HasPermission(resolvedPerms, agent.PermTicketInternalNotes)
resp := convertTicketFromStore(ticket)
filterInternalNotes(resp, ticket, user, hasPerm)
return c.JSON(http.StatusOK, resp)
```

**ListTickets handler (line 165):**
```go
// Resolve permission ONCE before loop:
resolvedPerms, _ := agent.ResolveEffectivePermissions(ctx, s.Store, *tenantID, userID)
hasPerm := agent.HasPermission(resolvedPerms, agent.PermTicketInternalNotes)

for _, t := range list {
    resp := convertTicketFromStore(t)
    filterInternalNotes(resp, t, user, hasPerm)
    result = append(result, resp)
}
```

**UpdateTicket handler (line 252):**
```go
// Only allow internal_notes update if user has permission:
if request.InternalNotes != nil {
    resolvedPerms, _ := agent.ResolveEffectivePermissions(ctx, s.Store, *tenantID, userID)
    hasPerm := agent.HasPermission(resolvedPerms, agent.PermTicketInternalNotes)
    if isSuperUser(user) || hasPerm {
        update.InternalNotes = request.InternalNotes
    }
}
```

**CreateTicket handler (line 57):**
- `internal_notes` defaults to `""`, not settable via create API

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

**Script:** `cmd/import-bugs/main.go` (separate package, no conflict with `cmd/seed/`)

#### Import Flow (Async Design)

```
Phase 1: Create Tickets (synchronous)
  For each bug folder (001-050):
    1. Read all .md files
    2. Parse for topic, status, category
    3. For each phase (plan, code, testing):
       - Create memo with phase content
       - Create ticket linked to memo
       - Set internal_notes = "Pending summary..."
    4. Store bug metadata for meta-tickets

Phase 2: Generate LLM Summaries (async background)
  Worker pool (5 goroutines):
    1. Pull ticket from queue
    2. Generate summary via LLM with template
    3. Update ticket's internal_notes
    4. On failure: set internal_notes = "Summary generation failed"
  
  Timeout: 30 seconds per ticket
  Retry: 2 attempts with exponential backoff

Phase 3: Create Meta-Tickets (after Phase 2 completes)
  For each category:
    1. Aggregate summaries from all bugs in category
    2. Generate cross-bug pattern summary via LLM
    3. Create meta-ticket with summary
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
    // 1. Check if CockroachDB is available
    if s.vectorDB == nil {
        return
    }
    
    // 2. Embed ticket description + title
    // 3. Search CockroachDB for top-5 similar tickets (similarity > 0.7)
    //    MUST filter by tenant_id for isolation
    // 4. Extract internal_notes from matches
    // 5. Format suggested resolution
    // 6. Update ticket's internal_notes
}
```

#### Vector Search with Tenant Isolation

```sql
SELECT id, title, content, content_type, metadata, source_version, created_at,
       1 - (embedding <=> $1::VECTOR) AS similarity
FROM agent_vectors
WHERE tenant_id = $2 AND content_type = ANY($3)
  AND (1 - (embedding <=> $1::VECTOR)) > 0.7
ORDER BY embedding <=> $1::VECTOR
LIMIT $4
```

**Critical:** The `tenant_id = $2` filter prevents cross-tenant data leakage.

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

**Trade-off Note (Finding #9):** Including `internalNotes` in embedding content makes resolution patterns searchable via vector search. This increases semantic search quality but means internal notes content is discoverable through vector queries. Since the vector DB doesn't enforce RBAC at search time, any user who can query the vector index could discover internal notes content through semantic search. This is acceptable for the hackathon demo because:
1. The vector search is tenant-scoped (tenant_id filter)
2. The internal notes themselves are still RBAC-gated in the API response
3. The semantic search doesn't return the full internal notes text, just similarity scores

---

## Implementation Order

| Step | Task | Files | Est. Time |
|------|------|-------|-----------|
| 1 | SQLite migration | `store/migration/sqlite/0.35/00__tickets_add_internal_notes.sql`, `LATEST.sql` | 10 min |
| 2 | Postgres migration | `store/migration/postgres/0.35/00__tickets_add_internal_notes.sql`, `LATEST.sql` | 10 min |
| 3 | Store types | `store/ticket.go` | 10 min |
| 4 | SQLite driver | `store/db/sqlite/ticket.go` | 30 min |
| 5 | Postgres driver | `store/db/postgres/ticket.go` | 30 min |
| 6 | Permission constant + presets | `server/router/api/v1/agent/permissions.go` | 10 min |
| 7 | Ticket service RBAC | `server/router/api/v1/ticket_service.go` | 45 min |
| 8 | Frontend display | `web/src/pages/TicketDetail.tsx` | 15 min |
| 9 | Bug import script | `cmd/import-bugs/main.go` | 2-3 hours |
| 10 | Ticket embedder | `server/router/api/v1/agent/ticket_embedder.go` | 15 min |
| 11 | Resolution inference | `server/router/api/v1/agent/service.go` | 1-2 hours |
| 12 | Run import + verify | Manual testing | 1 hour |
| **Total** | | | **7-9 hours** |

---

## Test Plan

### Unit Tests

| Test | File | Purpose |
|------|------|---------|
| `TestConvertTicketFromStore_InternalNotes` | `ticket_service_test.go` | Verify RBAC filtering logic |
| `TestFilterInternalNotes_SuperUser` | `ticket_service_test.go` | Superuser sees all |
| `TestFilterInternalNotes_Creator` | `ticket_service_test.go` | Creator sees own |
| `TestFilterInternalNotes_Assignee` | `ticket_service_test.go` | Assignee sees assigned |
| `TestFilterInternalNotes_Unauthorized` | `ticket_service_test.go` | Unauthorized sees empty |

### Integration Tests

| Test | File | Purpose |
|------|------|---------|
| `TestMigration_InternalNotes` | `migration_test.go` | Verify migration applies cleanly |
| `TestCreateTicket_InternalNotes` | `ticket_integration_test.go` | Create ticket with internal notes |
| `TestUpdateTicket_InternalNotes` | `ticket_integration_test.go` | Update internal notes |
| `TestGetTicket_InternalNotes_Visibility` | `ticket_integration_test.go` | RBAC enforcement |

### Import Script Tests

| Test | File | Purpose |
|------|------|---------|
| `TestParseBugFolder` | `import_bugs_test.go` | Parse bug folder structure |
| `TestParseBugFolder_Empty` | `import_bugs_test.go` | Handle empty folder (007) |
| `TestParseBugFolder_Malformed` | `import_bugs_test.go` | Handle malformed .md files |
| `TestGenerateSummary` | `import_bugs_test.go` | LLM summary generation |
| `TestGenerateSummary_Fallback` | `import_bugs_test.go` | LLM failure fallback |

### Inference Tests

| Test | File | Purpose |
|------|------|---------|
| `TestInferResolution_TenantIsolation` | `inference_test.go` | Cross-tenant data not leaked |
| `TestInferResolution_Fallback` | `inference_test.go` | CockroachDB unavailable |
| `TestInferResolution_NoMatches` | `inference_test.go` | No similar tickets found |

---

## Verification Plan

| Step | Command | Expected |
|------|---------|----------|
| Build | `go build ./bin/memos/main.go` | Compiles |
| Build (cockroach) | `go build -tags cockroach ./bin/memos/main.go` | Compiles |
| Test | `go test ./store/... -v` | Pass |
| Test tickets | `go test ./server/router/api/v1/... -run TestTicket -v` | Pass |
| Validate schema | `task validate:schema` | Pass |
| Validate parity | `task validate:parity` | Pass |
| Run locally | `task run` | Server starts, migration applies |
| Import bugs | `go run ./cmd/import-bugs` | ~130 tickets created |
| Verify RBAC | Create ticket as customer, view as admin | Internal notes visible to admin only |
| Verify inference | Create new ticket, check internal_notes | Auto-populated |

---

## CockroachDB Compatibility Notes

| Feature | SQLite | Postgres | CockroachDB |
|---------|--------|----------|-------------|
| `ALTER TABLE ADD COLUMN` | ✅ | ✅ | ✅ |
| `TEXT DEFAULT ''` | ✅ | ✅ | ✅ |
| `RETURNING` clause | ✅ | ✅ | ✅ |
| `VECTOR(1536)` type | ❌ N/A | Requires pgvector | ✅ Native |
| `CREATE VECTOR INDEX` | ❌ N/A | `USING hnsw` | `vector_ip_ops` |
| `<=>` cosine operator | ❌ N/A | ✅ | ✅ |

**This migration (`internal_notes`) uses only standard SQL that works on all three databases.** No CockroachDB-specific syntax needed.

**For vector search (Phase 5):** CockroachDB uses native `VECTOR(1536)` type and `CREATE VECTOR INDEX ... vector_ip_ops`. Postgres requires pgvector extension and `USING hnsw`. The existing `vectordb_cockroach.go` handles this difference via `Validate()` method.

---

## Adversarial Review Prompt

Please review this revised plan for:

1. **Correctness**: Are the SQL queries correct for both SQLite and Postgres? Will the migration work on existing databases?
2. **RBAC**: Are the visibility rules properly enforced using `agent.ResolveEffectivePermissions()`? Any bypass vectors?
3. **Performance**: Will the async import flow with worker pool be efficient? Any bottlenecks?
4. **Error handling**: What happens if LLM fails during import? If CockroachDB is unavailable?
5. **Edge cases**: What if a bug folder is empty (007)? What if files are malformed?
6. **Security**: Can non-admin users see internal notes through API manipulation? Is tenant isolation enforced in vector search?
7. **Scalability**: Will vector search scale with 130+ tickets? Any indexing concerns?
8. **Completeness**: Are we missing any files that need modification?
9. **CockroachDB**: Does the migration work on CockroachDB? Any compatibility issues?

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
- ResolveEffectivePermissions: `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/permissions.go:138`
- Ticket service: `/home/chaschel/Documents/go/bchat/server/router/api/v1/ticket_service.go`
- SQLite ticket driver: `/home/chaschel/Documents/go/bchat/store/db/sqlite/ticket.go`
- Postgres ticket driver: `/home/chaschel/Documents/go/bchat/store/db/postgres/ticket.go`
- Ticket embedder: `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/ticket_embedder.go`
- Frontend ticket detail: `/home/chaschel/Documents/go/bchat/web/src/pages/TicketDetail.tsx`
- CockroachDB vector store: `/home/chaschel/Documents/go/bchat/server/router/api/v1/agent/vectordb_cockroach.go`
