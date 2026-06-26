# Beta Foundation Review Fixes

Resolves the defects surfaced by the adversarial code review of the 0.26 foundation slice:
idempotency race + durability, widget pending-message overwrite + insecure-context UUID crash,
Postgres bridge error surfacing, and RBAC permissions round-trip fidelity.

User decisions locked:
- Durable idempotency via a **new `agent_messages` table** + migration (not reusing sessions JSON).
- **Coarse per-session mutex** held from idempotency check through assistant-message append.
- **Amend into the existing 0.26 slice** (new `01__agent_messages.sql` inside the existing
  `0.26/` folders, `version.go` stays `0.26.0`).
- Postgres bridge remains **stubbed**; only surface `ErrBridgeUnsupportedDatabase` as `501`.

---

## 1. New `agent_messages` table (durable idempotency)

### [ADD] `store/migration/sqlite/0.26/01__agent_messages.sql`
```sql
CREATE TABLE IF NOT EXISTS agent_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_source_lookup
    ON agent_messages(session_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_agent_messages_tenant ON agent_messages(tenant_id);
```

### [ADD] `store/migration/postgres/0.26/01__agent_messages.sql`
```sql
CREATE TABLE IF NOT EXISTS agent_messages (
    id SERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_source_lookup
    ON agent_messages(session_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_agent_messages_tenant ON agent_messages(tenant_id);
```

### [MODIFY] `store/migration/sqlite/LATEST.sql` and `store/migration/postgres/LATEST.sql`
Append the same `agent_messages` table + indexes at the end of each file so fresh-install
`preMigrate` (LATEST.sql path) gets the table too.

---

## 2. Store interface + driver bindings

### [MODIFY] `store/agent.go`
Add a `FindAgentMessage` filter and a new storage-methodgroup:

```go
type FindAgentMessage struct {
    SessionID *string
    Source    *string
    SourceID  *string
    TenantID  *int32
}

type AgentMessageRecord struct {
    ID        int32
    SessionID string
    TenantID  int32
    Source    string
    SourceID  string
    Role      string
    Content   string
    CreatedAt time.Time
}
```

Store wrapper methods:

```go
func (s *Store) CreateAgentMessages(ctx context.Context, messages []*AgentMessageRecord) error
func (s *Store) GetAssistantMessageBySourceID(ctx context.Context, sessionID, sourceID string) (*AgentMessageRecord, error)
```

The getter returns the assistant message (and its content/timestamp) whose `source_id`
matches the client message ID — or nil if absent. This is the durable idempotency lookup.

### [MODIFY] `store/driver.go`
Add to the `Driver` interface (under Agent section):

```go
CreateAgentMessages(ctx context.Context, messages []*AgentMessageRecord) error
GetAssistantMessageBySourceID(ctx context.Context, sessionID, sourceID string) (*AgentMessageRecord, error)
```

### [MODIFY] `store/db/sqlite/agent.go`
Implement both methods. `GetAssistantMessageBySourceID`:

```sql
SELECT id, session_id, tenant_id, source, source_id, role, content, created_at
FROM agent_messages
WHERE session_id = ? AND source = 'external_response' AND source_id = ?
LIMIT 1
```

`CreateAgentMessages`: loop over the slice, inserting each row (one INSERT per row;
transaction optional — keep it simple and explicit).

### [MODIFY] `store/db/postgres/agent.go`
Same logic with `$N` parameters. Note: `agent_messages` is referenced by Sessions but
the new table also needs its own `tenant_id` column — already present in the schema above.

### [ADD] stub implementations in `store/db/postgres/agent.go` for these exact two methods
Postgres `getSystemSecret`/`agent` methods are partially implemented, so these should
also be real — not stubbed — since the table now exists in Postgres too.

---

## 3. Per-session idempotency mutex + durable lookup in `ChatExternal`

### [MODIFY] `store/agent.go` — `AgentSession` struct
Add an unexported mutex alongside existing in-memory-only fields:

```go
// Not persisted; guards concurrent ChatExternal calls for the same session.
IdempotencyMu sync.Mutex `json:"-"`
```

### [MODIFY] `server/router/api/v1/agent/service.go` — `ChatExternal`
Replace the existing in-memory-only idempotency block (lines ~1439-1458) with:

```go
session := s.memorySessions.GetOrCreate(config.TenantID, sessionID)

// Durable idempotency check first. Survives process restart and multi-instance
// deployments because the lookup hits the database, not in-memory state.
if req.ClientMessageID != "" {
    if cached, err := s.store.GetAssistantMessageBySourceID(ctx, session.ID, req.ClientMessageID); err == nil && cached != nil {
        // Strict content matching: if the client reused the same ID with different
        // text, treat as a new message rather than returning a stale response.
        // (Verification happens by re-checking the user message content below.)
        _ = cached // content check handled by comparing against persisted user row
    }
}
```

Better approach (combines durability + content match + concurrency):

```go
if req.ClientMessageID != "" {
    session.IdempotencyMu.Lock()
    defer session.IdempotencyMu.Unlock()

    // Check durable store first.
    if cached, derr := s.store.GetAssistantMessageBySourceID(ctx, session.ID, req.ClientMessageID); derr == nil && cached != nil {
        // Verify the persisted user message content matches the retry exactly.
        if usr, uerr := s.store.GetUserMessageBySourceID(ctx, session.ID, req.ClientMessageID); uerr == nil && usr != nil && usr.Content == req.Message {
            return &ChatResponse{
                SessionID: session.ID,
                Message: ResponseMessage{
                    Role:      cached.Role,
                    Content:   cached.Content,
                    Timestamp: cached.CreatedAt,
                },
                Metadata: ChatMetadata{Phase: session.Phase},
            }, nil
        }
    }

    // In-memory fallback for hot path (avoids a DB round-trip within the lock).
    for i, message := range session.Messages {
        if message.Role != "user" || message.Source != "external_client_message" || message.SourceID != req.ClientMessageID {
            continue
        }
        if message.Content != req.Message {
            break // key reuse with different text → treat as new
        }
        for _, candidate := range session.Messages[i+1:] {
            if candidate.Role == "assistant" && candidate.Source == "external_response" && candidate.SourceID == req.ClientMessageID {
                return &ChatResponse{ /* same shape as original */ }, nil
            }
        }
    }
}
```

Where `GetUserMessageBySourceID` narrowly fetches the `external_client_message` row:

```sql
SELECT id, session_id, tenant_id, source, source_id, role, content, created_at
FROM agent_messages
WHERE session_id = ? AND source = 'external_client_message' AND source_id = ?
LIMIT 1
```

Add `GetUserMessageBySourceID` to `Driver`, `store/agent.go`, SQLite, and Postgres
alongside the assistant getter.

### Persist user + assistant rows for future dedup
After `processChat` succeeds and the assistant message is appended (lines 1519-1526),
batch-insert the user and assistant `AgentMessageRecord`s:

```go
if req.ClientMessageID != "" {
    records := []*store.AgentMessageRecord{
        {SessionID: session.ID, TenantID: config.TenantID, Source: "external_client_message", SourceID: req.ClientMessageID, Role: "user", Content: req.Message},
        {SessionID: session.ID, TenantID: config.TenantID, Source: "external_response", SourceID: req.ClientMessageID, Role: "assistant", Content: assistantContent},
    }
    if perr := s.store.CreateAgentMessages(ctx, records); perr != nil {
        slog.Warn("failed to persist agent_messages", "error", perr)
    }
}
```

Because the lookup and append happen under `session.IdempotencyMu`, the persist also races
no other request for the same session — the persisted row is committed before the next
request's durable lookup runs.

---

## 4. Widget pending-message queue + insecure-context UUID fallback

### [MODIFY] `widget/src/core/state.ts`

Replace single-object storage with a dictionary keyed by message text, and wrap the UUID
generator.

LocalStorage value shape: `JSON.stringify({ [message]: uuid, ... })`.

```typescript
private generateUUID(): string {
    try {
        if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
            return crypto.randomUUID();
        }
    } catch { /* fall through */ }
    // RFC4122 v4-ish fallback using getRandomValues when available, else Math.random.
    const rnds = new Uint8Array(16);
    if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
        crypto.getRandomValues(rnds);
    } else {
        for (let i = 0; i < 16; i++) rnds[i] = Math.floor(Math.random() * 256);
    }
    rnds[6] = (rnds[6] & 0x0f) | 0x40;
    rnds[8] = (rnds[8] & 0x3f) | 0x80;
    const hex = [...rnds].map(b => b.toString(16).padStart(2, "0")).join("");
    return `${hex.slice(0,8)}-${hex.slice(8,12)}-4${hex.slice(13,16)}-${hex.slice(16,20)}-${hex.slice(20)}`;
}

getOrCreatePendingMessageID(message: string): string {
    const key = `bchat_pending:${this.tenantSlug}`;
    let map: Record<string, string> = {};
    try {
        const stored = localStorage.getItem(key);
        if (stored) {
            const parsed = JSON.parse(stored);
            if (parsed && typeof parsed === "object") map = parsed;
        }
    } catch { localStorage.removeItem(key); }

    if (map[message]) return map[message];
    const id = this.generateUUID();
    map[message] = id;
    try { localStorage.setItem(key, JSON.stringify(map)); }
    catch { /* quota exceeded; proceed without persistence */ }
    return id;
}

acknowledgePendingMessage(message: string, clientMessageId: string): void {
    const key = `bchat_pending:${this.tenantSlug}`;
    let map: Record<string, string> = {};
    try {
        const stored = localStorage.getItem(key);
        if (stored) map = JSON.parse(stored) || {};
    } catch { localStorage.removeItem(key); return; }
    if (map[message] === clientMessageId) {
        delete map[message];
        try {
            if (Object.keys(map).length === 0) localStorage.removeItem(key);
            else localStorage.setItem(key, JSON.stringify(map));
        } catch { /* ignore */ }
    }
}
```

### [MODIFY] `widget/src/ui/Widget.ts`
Update the call site: pass `message` to `acknowledgePendingMessage` so removal is keyed
on the same text used for storage:

```typescript
const clientMessageId = this.state.getOrCreatePendingMessageID(message);
try {
    const response = await sendMessage(..., clientMessageId);
    this.state.acknowledgePendingMessage(message, clientMessageId);
    ...
```

Keep `clientMessageId` as the final idempotency key sent to the server.

---

## 5. Postgres bridge error surfacing (stubs + 501)

### [MODIFY] `server/router/api/v1/agent/delivery.go`
At the top of `DeliverWebChatReply`, before the claim attempt:

```go
if !s.store.SupportsBridgeDelivery() {
    slog.Warn("bridge delivery not supported by database driver", "tenant_id", tenantID, "outbox_id", outboxID)
    return store.ErrBridgeUnsupportedDatabase
}
```

### [ADD] `store/agent.go` helper
```go
func (s *Store) SupportsBridgeDelivery() bool {
    _, err := s.driver.ClaimBridgeReplyOutboxByOutboxID(context.Background(), 0, "", "", time.Time{}, 0)
    return !errors.Is(err, ErrBridgeUnsupportedDatabase)
}
```

This is a cheap probe: the stub returns `ErrBridgeUnsupportedDatabase` immediately
without touching the DB. Real drivers will return a different error (e.g. not-found)
for the zero-tenant probe, which is also non-`ErrBridgeUnsupportedDatabase`.

### [MODIFY] `server/router/api/v1/agent/handlers.go`
In the handler that calls `DeliverWebChatReply` (around line 268), add an explicit branch:

```go
} else if errors.Is(dErr, store.ErrBridgeUnsupportedDatabase) {
    deliveryStatus.Status = "skipped_unsupported_driver"
    slog.Warn("bridge delivery skipped: unsupported database", "outbox_id", result.Outbox.OutboxID)
```

This surfaces the limitation cleanly instead of a generic "failed".

---

## 6. RBAC permissions round-trip fidelity (Postgres)

### [MODIFY] `store/db/postgres/rbac.go` — `ListUserTenantPermissions`
After scanning the `permissions` string:

```go
if permissions == "" {
    perm.Permissions = []string{}
} else {
    perm.Permissions = strings.Split(permissions, ",")
}
```

This guarantees an empty slice (not nil) on round-trip, matching SQLite behavior.

---

## Verification Plan

### Automated
1. **Race test** — `TestChatExternalClientMessageIDIsIdempotent_Concurrent`: spawn N goroutines
   with identical `ClientMessageID` + `SessionID`, assert exactly one LLM call and one
   assistant message. Run with `-race`.
2. **Durability test** — `TestChatExternalClientMessageIDIsIdempotent_Restart`: first call
   persists to DB, then construct a **new** `Service` (fresh `MemorySessionStore`) against
   the same DB; second call must return the cached response without an LLM call.
3. **Content-mismatch test** — same `ClientMessageID` with different `message` text must
   produce a new LLM call and a new assistant message.
4. **Widget unit test** (if a test harness exists; otherwise manual) — send three rapid
   messages offline, verify three distinct pending IDs; reconnect, verify three distinct
   server-side `client_message_id` values.
5. **Postgres migration** — `TestPostgresMigrationFromV25ToV26` continues to pass; new
   `01__agent_messages.sql` applies cleanly on top of a 0.25 baseline.
6. **RBAC round-trip** — create a `UserTenantPermission` with empty permissions on Postgres,
   read back, assert `Permissions` is `[]string{}` not nil.

### Manual
1. Render the widget over plain `http://` (no TLS). Verify messages send successfully using
   the UUID fallback (log the generated ID format to confirm the polyfill path was taken).
2. In DevTools, disable network. Type three rapid messages. Reconnect. Verify all three
   transmit with distinct IDs and each receives its own response.
3. On a Postgres-backed instance, trigger a bridge handoff reply delivery and confirm the
   operator sees `skipped_unsupported_driver` (not a generic failure) in the delivery status.

---

## Adversarial Review Prompt for Another Agent

Use the following prompt to have a fresh AI agent independently review the
implementation. It does not reference this plan and states only the areas to inspect.

---

Perform a focused adversarial code review of the uncommitted changes in
`/home/chaschel/Documents/go/bchat`. Review only the implementation of the
beta-foundation-review fixes below. Inspect the actual git diff and surrounding code.
Do not review unrelated code or demand features outside this slice.

Scope:

- `ChatExternal` durable idempotency via the new `agent_messages` table.
- Per-session concurrency mutex (`IdempotencyMu`) guarding the idempotency check,
  the LLM call path, and the persistence of user/assistant rows.
- Widget pending-message dictionary storage and insecure-context UUID fallback.
- `SupportsBridgeDelivery` guard in `DeliverWebChatReply` and the `501`
  (`skipped_unsupported_driver`) branch in the bridge-reply handler.
- RBAC empty-permission round-trip fix on PostgreSQL.
- MySQL stub parity (`CreateAgentMessages`, `GetAssistantMessageBySourceID`,
  `GetUserMessageBySourceID`, `SupportsBridgeDelivery`).
- Two new `0.26` migration files and the appended entries in both `LATEST.sql`
  files.
- Four new tests in `bridge_foundation_test.go`.

Prioritize fatal or release-blocking issues. For every finding include: severity
(Critical / High / Medium / Low), exact file and line, concrete failure or exploit
scenario, root cause, minimum safe fix, and required regression test.

Explicitly answer these questions:

1. Can two concurrent requests with the same `client_message_id` and `session_id`
   still both call the LLM? Walk through the lock acquisition and DB-lookup ordering.
2. After a process restart, does a retry with the same `client_message_id` reliably
   return the cached response without calling the LLM? What if the in-memory state
   was partially flushed?
3. Can reusing one `client_message_id` with different message text return an unrelated
   prior response? Verify the server-side content comparison in both the DB path and
   the in-memory fallback.
4. Are all four new tests actually executing the repaired logic, or could any pass
   for the wrong reason (e.g. caching in the old code path)?
5. Does the new `agent_messages` lookup use the index you expect? Is the query on
   `(session_id, source, source_id)` with `LIMIT 1`?
6. Can `CreateAgentMessages` be called with a nil or empty slice without error?
7. Does the widget dictionary migration from the old single-object format cause data
   loss or stale entries across browser reloads?
8. Does `generateUUID()` produce lexically valid UUID v4 strings in all three
   branches (`randomUUID`, `getRandomValues`, `Math.random`)?
9. Does `SupportsBridgeDelivery` for SQLite ever return a false negative (e.g. a driver
   error during the probe) and silently disable bridge delivery in production?
10. Any SQL injection or type-scanning mismatch in the new SQLite / Postgres /
    MySQL agent-message queries?

Verify with the race detector (`-race`), browser/console checks for the widget,
and, if feasible, a real Postgres run before merging. Distinguish verified defects
from risks needing integration testing. End with one verdict: APPROVE, APPROVE WITH
NON-BLOCKING FIXES, or REQUEST CHANGES.

---

## Files Touched

- `store/agent.go` — `FindAgentMessage`, `AgentMessageRecord`, store wrappers,
  `SupportsBridgeDelivery`, `AgentSession.IdempotencyMu`.
- `store/driver.go` — `CreateAgentMessages`, `GetAssistantMessageBySourceID`,
  `GetUserMessageBySourceID` interface methods.
- `store/db/sqlite/agent.go` — SQLite implementations.
- `store/db/postgres/agent.go` — Postgres implementations.
- `store/db/postgres/rbac.go` — empty-permission fix.
- `store/migration/sqlite/0.26/01__agent_messages.sql` — new.
- `store/migration/postgres/0.26/01__agent_messages.sql` — new.
- `store/migration/sqlite/LATEST.sql` — append table + indexes.
- `store/migration/postgres/LATEST.sql` — append table + indexes.
- `server/router/api/v1/agent/service.go` — `ChatExternal` durable lookup + mutex + persist.
- `server/router/api/v1/agent/delivery.go` — `SupportsBridgeDelivery` guard.
- `server/router/api/v1/agent/handlers.go` — `skipped_unsupported_driver` branch.
- `widget/src/core/state.ts` — pending queue + UUID polyfill.
- `widget/src/ui/Widget.ts` — pass `message` to `acknowledgePendingMessage`.
