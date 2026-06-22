# Adversarial Review of plan4.md

## Verdict

**accept_with_repairs**

The plan is architecturally sound and well-structured, but it contains a critical SQLite schema bug, several missing transaction boundaries, underspecified HMAC signature base, no body size limits, a stream-token design that needs hardening, and at least one incorrect assumption about the existing escalation code path. None of these are architectural blockers, but all must be fixed before implementation begins. The plan's core insight — durable event sourcing with monotonic seq, outbox pattern, and typed bridge state machine — is correct and well-suited to bchat.

---

## Critical Blockers

### 1. Composite Foreign Key in `bridge_messages` Is Broken

```sql
FOREIGN KEY (tenant_id, session_id) REFERENCES bridge_session_state(tenant_id, session_id)
```

`bridge_session_state` has `UNIQUE(tenant_id, session_id)` but **no explicit composite primary key**. SQLite foreign key resolution requires the referenced columns to be either the primary key or a unique constraint. While the UNIQUE constraint *should* work, the referenced columns are `(tenant_id, session_id)` while the actual primary key is `id INTEGER PRIMARY KEY AUTOINCREMENT`. This creates a fragile dependency: if anyone ever drops or alters the UNIQUE constraint, cascading deletes silently break. Worse, `bridge_events` has **no such foreign key at all** — events can become orphaned when a session is deleted.

**Fix**: Add an explicit composite foreign key reference or, better, use `bridge_session_state.id` as the canonical FK in all child tables:

```sql
CREATE TABLE bridge_session_state (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    -- ...
    UNIQUE(tenant_id, session_id)
);

CREATE TABLE bridge_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bridge_session_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (bridge_session_id) REFERENCES bridge_session_state(id) ON DELETE CASCADE
);
```

Apply the same pattern to `bridge_events`, `bridge_stream_tokens`, and `bridge_outbox`. This eliminates the composite FK fragility entirely.

### 2. `bridge_events` Has No Foreign Key to `bridge_session_state`

`bridge_events` stores `tenant_id` and `session_id` but has **no foreign key** referencing `bridge_session_state`. This means:
- Events can reference non-existent sessions
- Deleting a session leaves orphaned events in the table
- The SSE replay query can return events for sessions that no longer exist

**Fix**: Add `bridge_session_id INTEGER NOT NULL` with `FOREIGN KEY REFERENCES bridge_session_state(id) ON DELETE CASCADE`.

### 3. `bridge_outbox` Has No Unique Constraint on `event_id`

```sql
CREATE TABLE bridge_outbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL,
    -- ...
    FOREIGN KEY (event_id) REFERENCES bridge_events(event_id) ON DELETE CASCADE
);
```

There is no `UNIQUE(event_id)` constraint. If the outbox processor crashes between inserting the outbox row and marking it as delivered, a retry of the same event will create a **duplicate outbox row**, causing duplicate delivery to Hermes. The Hermes side will receive the same event twice.

**Fix**: Add `UNIQUE(event_id)` to `bridge_outbox`, or use `INSERT OR IGNORE` / `ON CONFLICT` logic when enqueueing.

### 4. Missing `Content-Length` / Body Size Limit on HMAC Endpoints

The plan specifies HMAC over `raw_body` but sets **no maximum body size**. An attacker who discovers a valid `key_id` (even without the secret) can send a multi-gigabyte body to the `/bridge/reply` endpoint, causing memory exhaustion. The HMAC verification reads the entire body into memory before checking the signature.

**Fix**: Add a hard body size limit (e.g., 1MB for reply payloads) **before** reading the body. In Go: `r.Body = http.MaxBytesReader(w, r.Body, maxSize)`. Reject with 413 if exceeded. Document the limit.

### 5. HMAC Signature Base Is Insufficient

The plan specifies:
```
signature_base = timestamp + "." + nonce + "." + raw_body
```

This is **missing the HTTP method and request path**. Without binding method and path, a valid signature for `POST /bridge/reply` can be replayed against `POST /bridge/takeover` or `POST /bridge/release` with the same timestamp+nonce+body. An attacker who captures a legitimate reply request can replay it as a takeover or release.

**Fix**: The signature base must include method, path, and content-type:
```
signature_base = method + "\n" + path + "\n" + content_type + "\n" + timestamp + "\n" + nonce + "\n" + raw_body_hash
```

Use `SHA256(raw_body)` in the signature base instead of the raw body itself to keep the signature input bounded and deterministic. The verifier independently computes the body hash from the received body.

### 6. `bridge_session_state.ticket_id` Type Mismatch Risk

The plan declares `ticket_id INTEGER` in `bridge_session_state`. The actual bchat `store.Ticket.ID` is `int32`. In SQLite this works (INTEGER can hold int32), but the plan's `TakeoverAcceptedPayload.TicketID` is `string`. This type mismatch between the DB layer (integer), the Go store layer (int32), and the event payload (string) will cause silent bugs.

**Fix**: Standardize on `int32` in Go, `INTEGER` in SQLite, and document that event payloads serialize the integer as a number, not a string. Or if string is preferred for event payloads, add explicit conversion at the event creation boundary.

---

## High-Priority Repairs

### 7. No Transaction Boundary Around Takeover + Ticket Creation

The takeover flow does:
1. Insert/update `bridge_session_state` to `human_active`
2. Call `CreateEscalationTicket()` (which creates a Memo AND a Ticket in separate DB operations)
3. Update `bridge_session_state.ticket_id`

If step 2 succeeds but step 3 crashes, the session is `human_active` with no linked ticket. The audit trail is broken. If step 2's memo creation succeeds but ticket creation fails, `CreateEscalationTicket` has its own fallback — but the bridge state machine doesn't know about it.

**Fix**: Wrap the entire takeover operation (bridge state update + ticket creation + bridge state ticket_id update) in a single SQLite transaction using `BEGIN IMMEDIATE`. If ticket creation fails, roll back the state transition and return an error to the harness.

### 8. Stream Token Returned on Every `human_active` Message Creates Token Proliferation

The plan says: "Return a newly minted `stream_token`" on every customer message while `human_active`. This means every message from the widget creates a new token. With rapid typing, you could accumulate dozens of valid tokens for one session. Each token is a valid SSE credential until its TTL expires.

**Fix**: Return the **same** stream token for the entire `human_active` duration. Only mint a new token if the existing one is expired or revoked. Store a single active token per session in `bridge_stream_tokens` with `UNIQUE(tenant_id, session_id)` or check for existing valid tokens before inserting.

### 9. `bridge_stream_tokens` Has No Index on `session_id` for Revocation

When releasing a session, the code must revoke all tokens for that session. Without an index on `(tenant_id, session_id)`, revocation requires a full table scan. With the proposed schema, the primary key is `token_hash`, so lookup by session requires scanning all tokens.

**Fix**: Add `CREATE INDEX idx_bridge_stream_tokens_session ON bridge_stream_tokens(tenant_id, session_id)`.

### 10. `bridge_events.seq` Is Globally Monotonic, Not Per-Session

`seq INTEGER PRIMARY KEY AUTOINCREMENT` is global across all sessions. The SSE query uses `seq > ?` with `session_id = ?`. This works correctly for replay **only if** the `seq` values for a given session are monotonically increasing — which they are, since `AUTOINCREMENT` is global. However, the `Last-Event-ID` header from the widget is a global seq number. A malicious widget could send `Last-Event-ID: 0` and receive **all** events for **all sessions** that have `seq > 0` and `visible_to_widget = 1` — except the `WHERE session_id = ?` clause prevents cross-session leakage.

**Verdict**: The per-session `WHERE` clause provides tenant isolation. But the global seq means a widget can infer the total event volume across all sessions by observing seq gaps. This is a low-severity information leak.

**Acceptable for v1**. For hardening, consider per-session monotonic counters.

### 11. `bridge_nonces` Purge Can Race with Verification

The plan says "Old nonces are purged after the timestamp skew window (e.g., 5 minutes)." If the purge runs while a request is being verified, a valid nonce could be deleted between the check and the insert, allowing a replay within the race window.

**Fix**: Use `INSERT OR IGNORE` into `bridge_nonces` as the atomic check-and-record operation. If the insert succeeds, the nonce is new. If it fails (duplicate), the nonce was already used. The purge only deletes rows older than the skew window. No race is possible because the INSERT itself is the check.

### 12. No `Content-Type` Enforcement on Bridge Endpoints

The HMAC endpoints don't require `Content-Type: application/json`. Without this, a client could send `application/x-www-form-urlencoded` data that parses differently, potentially causing signature verification bypass if the server and client disagree on what constitutes the "raw body."

**Fix**: Require `Content-Type: application/json` on all bridge endpoints. Reject with 415 if missing or incorrect.

### 13. `bridge_config.harness_webhook_url` Has No URL Validation

The column is `TEXT` with no CHECK constraint. An invalid URL will cause the outbox processor to fail repeatedly, filling the outbox with permanently failing events.

**Fix**: Add application-level URL validation on write. Optionally add a CHECK constraint: `CHECK(harness_webhook_url IS NULL OR harness_webhook_url LIKE 'http%')`.

### 14. `bridge_idempotency_keys` Has No TTL / Expiration

Idempotency keys accumulate forever. A long-running system will eventually fill the table with stale keys, causing unbounded growth.

**Fix**: Add a `expires_at INTEGER` column and a background worker that deletes expired rows. Set TTL to 24 hours (or configurable). Add `CREATE INDEX idx_bridge_idempotency_expires ON bridge_idempotency_keys(expires_at)`.

### 15. `bridge_messages` Lacks `event_id` Correlation

Human replies are stored in `bridge_messages` but have no `event_id` linking them to the `bridge_events` row that delivered them. This makes it impossible to trace which event triggered which message, complicating debugging and audit.

**Fix**: Add `event_id TEXT` column to `bridge_messages` with a foreign key to `bridge_events(event_id)`.

---

## Medium-Priority Repairs

### 16. `bridge_session_state` Missing `version` Column for Optimistic Locking

Concurrent takeover and release requests can race. Without optimistic locking, two operators could both believe they've taken over the same session.

**Fix**: Add `version INTEGER NOT NULL DEFAULT 1` to `bridge_session_state`. Increment on every UPDATE. Use `WHERE version = ?` in UPDATE statements and check rows affected.

### 17. `bridge_session_state` Missing `closed` State Transition Path

The plan defines `closed` as a state but lists no transitions into it. How does a session reach `closed`? From `released`? From `timeout_released`? From any state?

**Fix**: Add explicit transitions:
- `released` → `closed` (admin action or auto-close after transcript retention period)
- `timeout_released` → `closed`
- `human_active` → `closed` (force close by admin)
- Document that `closed` is terminal and prevents re-takeover.

### 18. `bridge_outbox` Missing `last_error` and `last_attempt_at` Columns

When delivery fails permanently, there's no record of why. The `attempts` counter increments but the error message is lost.

**Fix**: Add `last_error TEXT` and `last_attempt_at INTEGER` columns.

### 19. `bridge_events` Missing Index for SSE Replay

The SSE replay query filters on `(tenant_id, session_id, seq)`. Without a composite index, this requires a full table scan or index merge.

**Fix**: Add `CREATE INDEX idx_bridge_events_replay ON bridge_events(tenant_id, session_id, seq)`.

### 20. `bridge_events` Missing Index for Outbox Visibility

The outbox processor queries `visible_to_harness = 1` events. Without an index, this scans all events.

**Fix**: Add `CREATE INDEX idx_bridge_events_harness ON bridge_events(visible_to_harness, created_at) WHERE visible_to_harness = 1`. (Partial index if SQLite version supports it; otherwise a regular index.)

### 21. No Dead-Letter Queue for Permanently Failed Outbox Events

After max retries, failed outbox events are just stuck with `status = 'failed'`. There's no mechanism to alert operators or move them to a dead-letter table.

**Fix**: Add `max_attempts INTEGER DEFAULT 10` to `bridge_config`. After exceeding max, set status to `dead_letter` and generate a `delivery.permanently_failed` event. Optionally notify via existing bchat notification system.

### 22. `bridge_config` Missing `idempotency_ttl_mins` Configuration

The idempotency key TTL is hardcoded. Different tenants may need different windows.

**Fix**: Add `idempotency_ttl_mins INTEGER DEFAULT 1440` to `bridge_config`.

### 23. `bridge_config` Missing `max_stream_token_ttl_mins` Configuration

Stream token TTL is hardcoded at 10-30 minutes. This should be tenant-configurable.

**Fix**: Add `stream_token_ttl_mins INTEGER DEFAULT 30` to `bridge_config`.

### 24. No `created_by` / `updated_by` Audit Columns on `bridge_session_state`

The `operator_id` field tracks who took over, but there's no audit trail for who created the bridge session or who last modified it.

**Fix**: Add `created_by INTEGER` and `updated_by INTEGER` columns (or reuse `operator_id` for updates and add `initiated_by`).

### 25. `bridge_api_keys.secret_hash` Purpose Is Unclear

The plan says `secret_hash` is "only for audit and non-secret metadata." But what is it a hash of? If it's a hash of the plaintext secret, it could be used to verify a secret without decrypting — which means it's a password-equivalent and must be protected. If it's a hash of the encrypted blob, it's useless for verification.

**Fix**: Document the exact purpose. If it's for key identification (e.g., showing "key ending in ...abc" in the admin UI), use the last 8 characters of the key_id, not a hash of the secret. If it's for verification, use HMAC, not a plain hash.

### 26. `bridge_api_keys` Missing `name` / `description` Column

Operators need to identify keys. A bare `key_id` is not user-friendly.

**Fix**: Add `name TEXT` column.

### 27. No Rate Limiting on Bridge Endpoints

The plan mentions rate limiting for the Hermes reply endpoint but doesn't specify how. The existing `CheckRateLimit` uses `(tenantID, audienceType, clientIP)` — but bridge endpoints are service-to-service, not widget-to-service. The `clientIP` will be the Hermes server's IP, not the end user.

**Fix**: Add a separate rate limiter for bridge endpoints keyed on `(tenant_id, key_id)` with a higher limit (e.g., 100 req/min). Document that this protects against runaway Hermes instances, not end-user abuse.

### 28. `ChatExternal` Integration: `last_customer_msg_at` Updated in All Modes

The plan says: "Preamble: Check `bridge_session_state`. Updates `last_customer_msg_at` and `last_activity_at`." This implies updating timestamps even in `ai` mode. But `bridge_session_state` rows are only created when monitoring is enabled or takeover occurs. In pure `ai` mode with no monitoring, there's no `bridge_session_state` row to update.

**Fix**: Clarify that `last_customer_msg_at` is only updated when a `bridge_session_state` row exists. In pure `ai` mode with no bridge state, the preamble is a no-op. Document this clearly.

### 29. Escalation Detection Location Is Wrong in the Plan

The plan says: "If escalation intent detected (`classification.PrimaryIntent == "escalation"`)" in the context of `evaluatePolicy()`. The actual code checks this in `processChat()` directly (service.go line 1508), not in `evaluatePolicy()`. The plan's Step 10 references the wrong function.

**Fix**: Update the plan to reference the correct location in `processChat()`.

### 30. Widget `ChatResponse.metadata` Doesn't Have `stream_token` Field

The plan says: "In `api.ts`, watch for `metadata.stream_token` in the `ChatResponse`." The current `ChatResponse` type has `metadata?: { intent?: string; confidence?: number }`. There's no `stream_token` field.

**Fix**: The Go backend's `ChatResponse` struct and the widget's TypeScript `ChatResponse` interface must both be updated to include `stream_token?: string` in metadata. This is a two-file change that the plan doesn't explicitly call out.

---

## Security Review: Widget SSE Authentication

### Judgment: **accept_with_repairs**

The stream-token-only model is acceptable for anonymous widgets, but the plan needs the following mandatory safeguards:

### Mandatory Safeguards

1. **Token generation**: Use `crypto/rand` with at least 256 bits (32 bytes), encoded as hex or base64url. Do NOT use `uuid.New()` — while UUIDv4 is random, it's only 122 bits of entropy and has fixed structure. Use `rand.Read()` directly.

2. **Token storage**: Store `SHA-256(token)` as `token_hash`. Never store the raw token. Use `crypto/subtle.ConstantTimeCompare` for verification.

3. **Token scope**: Bind to `(tenant_id, session_id)`. The SSE handler must verify that the token's tenant matches the `:slug` in the URL. Cross-tenant token use must return 403.

4. **Token in URL is acceptable** for `EventSource` because:
   - `EventSource` API does not support custom headers or cookies
   - `fetch()` + `ReadableStream` could use headers but adds significant complexity
   - The token is high-entropy, short-lived, and revocable
   - This is the same pattern used by many production SSE services (e.g., Stripe, OpenAI)

5. **Referrer Policy**: The widget page should include `<meta name="referrer" content="no-referrer">` or at minimum `strict-origin-when-cross-origin` to prevent token leakage via Referer headers to third-party resources.

6. **CORS**: The SSE endpoint must validate the `Origin` header against the tenant's `AllowedDomains` (reuse the existing domain allowlist pattern from `HandleWidgetEmbed`). Without this, any website can open an SSE connection if they know a valid token.

7. **Token in browser history**: Since the token is in the URL query string, it will appear in browser history. Mitigation: After establishing the SSE connection, use `history.replaceState()` to remove the token from the URL. The `EventSource` object maintains the connection independently of the URL bar.

8. **Multi-tab behavior**: Multiple tabs with the same session will share the same token. This is acceptable — they're the same user. The SSE handler should allow multiple concurrent connections per token.

9. **Token renewal**: When a token expires mid-connection, the SSE handler should return a special `event: token_expired` message. The widget then POSTs a new message to `/chat/ext` to get a fresh token and reconnects.

10. **Log redaction**: The token appears in the URL. bchat's access logs will capture it. Implement log middleware that strips query parameters from SSE endpoint log lines, or at minimum strips the `token` parameter specifically.

11. **Connection limits**: Add a max concurrent SSE connections per session (e.g., 5) to prevent DoS. Return 429 if exceeded.

### Residual Risk

If a token is leaked (e.g., via browser extension, proxy logs, or shared computer), an attacker can read the human-agent conversation for the token's TTL. This is acceptable for v1 given the short TTL (10-30 min) and session scoping. For higher-security deployments, add client fingerprint binding (IP + User-Agent hash) as an optional hardening.

---

## Schema and Transaction Review

### SQLite-Specific Issues

1. **Foreign key enforcement**: SQLite defaults to `PRAGMA foreign_keys = OFF`. The plan doesn't mention enabling it. bchat must execute `PRAGMA foreign_keys = ON` on every connection. **Verify this in the codebase** — if bchat doesn't enable it, all FK constraints are no-ops.

   `needs_codebase_review`: Check if bchat's SQLite driver enables foreign keys.

2. **WAL mode**: For the outbox processor and SSE readers to work concurrently without locking, SQLite should use WAL mode. **Verify** bchat enables this.

   `needs_codebase_review`: Check if bchat uses WAL mode.

3. **`BOOLEAN` type**: SQLite doesn't have a native BOOLEAN type. `BOOLEAN` is stored as INTEGER (0/1). This is fine but the plan should note that Go's `bool` maps to `INTEGER` in SQLite.

4. **`AUTOINCREMENT` semantics**: `AUTOINCREMENT` in SQLite prevents reuse of deleted IDs. This is correct for `seq` (monotonic, never reuse) but unnecessary for `id` columns where regular `INTEGER PRIMARY KEY` suffices. The plan uses `AUTOINCREMENT` on `bridge_events.seq` (correct) and `bridge_outbox.id` (unnecessary but harmless).

5. **Missing indexes** (consolidated):
   - `bridge_events(tenant_id, session_id, seq)` — for SSE replay
   - `bridge_events(visible_to_harness, created_at)` — for outbox processor
   - `bridge_stream_tokens(tenant_id, session_id)` — for revocation
   - `bridge_outbox(status, next_retry_at)` — for outbox polling
   - `bridge_nonces(created_at)` — for purge worker
   - `bridge_idempotency_keys(expires_at)` — for TTL cleanup
   - `bridge_session_state(tenant_id, mode)` — for timeout checker scanning `human_active` sessions

### Transaction Rules

1. **Takeover**: `BEGIN IMMEDIATE` → insert/update `bridge_session_state` → call `CreateEscalationTicket` → update `ticket_id` → insert `bridge_events` → insert `bridge_outbox` → `COMMIT`. Rollback on any failure.

2. **Reply**: `BEGIN IMMEDIATE` → verify state is `human_active` → insert `bridge_messages` → insert `bridge_events` → insert `bridge_outbox` → update `last_agent_msg_at` → `COMMIT`. The SSE push happens AFTER commit.

3. **Release**: `BEGIN IMMEDIATE` → update `bridge_session_state` → revoke tokens → update ticket → insert `bridge_events` → insert `bridge_outbox` → `COMMIT`.

4. **Customer message in human mode**: `BEGIN IMMEDIATE` → insert `bridge_messages` → insert `bridge_events` → insert `bridge_outbox` → update `last_customer_msg_at` → `COMMIT`. Return stream token if needed.

5. **Outbox delivery**: `BEGIN IMMEDIATE` → select pending events → mark as `delivering` → `COMMIT` → deliver to Hermes → `BEGIN IMMEDIATE` → mark as `delivered` or `failed` → `COMMIT`. Use `SELECT ... FOR UPDATE` equivalent (SQLite doesn't support it; use `BEGIN IMMEDIATE` to lock the DB).

---

## State Machine and Race Review

### Missing States

The plan is missing a `failed` state for when ticket creation fails during takeover. Without it, the session is stuck in `handoff_queued` forever.

**Fix**: Add transition `handoff_queued` → `failed` (ticket creation error). From `failed`, allow retry to `human_active` or fallback to `ai`.

### Concurrency Hazards

1. **Two simultaneous takeover requests**: Both read state as `handoff_queued`, both try to transition to `human_active`. Without optimistic locking, both succeed, creating two tickets and two operator assignments.

   **Fix**: Use `version` column + `WHERE version = ?` in UPDATE. First updater wins; second gets 0 rows affected and returns 409.

2. **Release while reply is in flight**: The harness sends a reply, which begins processing. Simultaneously, the operator sends release. The reply might be processed against a `released` state, becoming a `late_agent_reply`.

   **Fix**: The plan already handles this with `late_agent_reply`. Ensure the reply handler checks state AFTER acquiring the DB lock, not before.

3. **Timeout checker races with agent reply**: The timeout checker scans `last_activity_at` and decides to auto-release. At the same time, an agent reply arrives and updates `last_activity_at`. Without proper locking, the auto-release could proceed despite the fresh activity.

   **Fix**: The timeout checker should use `BEGIN IMMEDIATE` to read-and-update atomically. Or use a `last_heartbeat_at` field that's updated by a separate heartbeat mechanism, not by message activity.

4. **Customer message during takeover transition**: A customer message arrives while the takeover transaction is in progress (state is being updated from `handoff_queued` to `human_active`). The message handler reads the old state (`handoff_queued`) and doesn't know what to do — it's not `ai` (should it call LLM?) and not `human_active` (should it bypass?).

   **Fix**: Define `handoff_queued` behavior: messages are accepted and stored but the AI still generates a response (since no human is active yet). The takeover transaction should complete before the next customer message is processed. Use `BEGIN IMMEDIATE` to serialize.

### Recommended State Machine Revision

```
ai → handoff_queued (escalation detected)
ai → human_active (explicit harness takeover)
handoff_queued → human_active (harness accepts)
handoff_queued → failed (ticket creation error)
failed → human_active (retry takeover)
failed → ai (abandon handoff)
human_active → released (operator handback)
human_active → timeout_released (inactivity)
human_active → closed (admin force close)
released → human_active (re-takeover)
timeout_released → human_active (re-takeover)
released → closed (auto-close / admin)
timeout_released → closed (auto-close / admin)
```

Add `version INTEGER NOT NULL DEFAULT 1` to `bridge_session_state`. Every state transition must increment version and check the expected current version.

---

## HMAC, Nonce, and Idempotency Review

### HMAC Signature Base — Final Recommendation

The plan's `timestamp.nonce.raw_body` is insufficient. Recommend:

```
signature_base = "POST\n" +
                 "/api/v1/agent/:slug/bridge/reply\n" +
                 "application/json\n" +
                 timestamp + "\n" +
                 nonce + "\n" +
                 SHA256(raw_body)
```

The verifier:
1. Rejects if `Content-Type != application/json`
2. Rejects if `|current_time - timestamp| > 300` seconds (configurable)
3. Rejects if nonce already exists in `bridge_nonces` (atomic INSERT)
4. Computes `expected_sig = HMAC-SHA256(secret, signature_base)`
5. Compares `expected_sig` with provided signature using `crypto/subtle.ConstantTimeCompare`
6. Stores nonce in `bridge_nonces` (same transaction as request processing)

### Nonce Table — Revised Schema

```sql
CREATE TABLE bridge_nonces (
    tenant_id INTEGER NOT NULL,
    key_id TEXT NOT NULL,
    nonce TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, key_id, nonce)
);
```

Purge: `DELETE FROM bridge_nonces WHERE created_at < ?` (run every 5 minutes, delete older than 10 minutes — 2x the skew window).

### Idempotency — Revised Semantics

The plan's `(tenant_id, key, endpoint)` primary key is correct, but:

1. `endpoint` must include the HTTP method: `"POST /api/v1/agent/:slug/bridge/reply"`
2. `request_hash` must be `SHA256(raw_body)` — not the body itself (bounded size)
3. Cache response for 24 hours minimum (add `expires_at`)
4. On conflict (same key, different hash): return 409 with `{"error": "idempotency_conflict"}`
5. On match (same key, same hash): return cached `response_status` and `response_body`
6. **Partial failure handling**: If the handler begins processing but crashes before storing the idempotency response, the next retry with the same key will re-execute. The handler must be idempotent at the application level (e.g., check if the message already exists before inserting).

### API Key Enumeration

The `Authorization: Bearer <key_id>` header reveals the key_id. An attacker can enumerate valid key_ids by observing which ones get past the key lookup stage (before HMAC verification). This is acceptable — key_ids are not secrets, they're identifiers. The secret is what provides security. However, the error message for "unknown key_id" vs "invalid signature" must be identical (e.g., always "401 unauthorized") to prevent enumeration.

---

## SSE Replay and Event Visibility Review

### Replay Correctness

The SSE replay query:
```sql
WHERE tenant_id = ?
  AND session_id = ?
  AND seq > ?
  AND visible_to_widget = 1
ORDER BY seq ASC
```

This is correct **if**:
1. `seq` is globally monotonic (it is, via `AUTOINCREMENT`)
2. The `Last-Event-ID` from the widget maps to `seq` (the plan must ensure the SSE handler sends `id: <seq>` in the SSE data)
3. The widget sends `Last-Event-ID` as the last received `seq` value

**Risk**: If the widget reconnects with a stale `Last-Event-ID`, it will receive all events since that seq. This is correct behavior (no messages lost) but could deliver a large batch on reconnect. Consider adding a max batch size or a "catchup complete" event.

### Event Visibility Table

| Event Type | Widget | Harness | Notes |
|---|---|---|---|
| `agent.reply` | ✅ | ✅ | Human's reply to customer |
| `takeover.accepted` | ✅ | ✅ | Human joined |
| `release.accepted` | ✅ | ✅ | Human left |
| `handoff.resolved` | ✅ | ✅ | Escalation handled |
| `session.timeout_released` | ✅ | ✅ | Auto-released |
| `ticket.created` | ❌ | ✅ | Internal audit |
| `ticket.updated` | ❌ | ✅ | Internal audit |
| `takeover.rejected` | ❌ | ✅ | Harness needs to know |
| `late_agent_reply` | ❌ | ✅ | Harness needs to know |
| `ai.message` | ❌ | ✅ | Monitoring |
| `customer.message` | ❌ | ✅ | Forward to Telegram |
| `delivery.failed` | ❌ | ✅ | Outbox alert |
| `delivery.succeeded` | ❌ | ✅ | Outbox confirmation |
| `session.closed` | ❌ | ✅ | Terminal state |

**Concern**: `customer.message` is visible to harness but not widget. The widget already receives the customer's own message (they typed it). But the harness needs `customer.message` to forward to Telegram. This is correct.

**Concern**: `ai.message` is visible to harness. If `monitoring_scope = 'all_sessions'`, every AI response is sent to Hermes. This could leak PII. The redaction pass must happen before outbox insertion.

### `late_agent_reply` Visibility

The plan says `late_agent_reply` is widget-hidden. This is correct — delivering a stale human reply after AI has resumed would confuse the customer. But the harness needs to know the reply was late (not delivered). The plan should specify that the bridge returns `202 Accepted` with a body indicating `late: true` when a reply is persisted but not delivered.

---

## Outbox and Delivery Review

### Outbox Status Lifecycle

```
pending → delivering → delivered
                  ↘
                    failed → pending (retry)
                         ↘
                           dead_letter (max retries exceeded)
```

### Revised `bridge_outbox` Schema

```sql
CREATE TABLE bridge_outbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    tenant_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK(status IN ('pending', 'delivering', 'delivered', 'failed', 'dead_letter')),
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 10,
    next_retry_at INTEGER,
    last_attempt_at INTEGER,
    last_error TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (event_id) REFERENCES bridge_events(event_id) ON DELETE CASCADE
);

CREATE INDEX idx_bridge_outbox_poll
    ON bridge_outbox(status, next_retry_at)
    WHERE status IN ('pending', 'failed');
```

### Outbox Processor Rules

1. Poll interval: 5 seconds
2. Batch size: 10 events per poll
3. Backoff: `next_retry_at = now + (attempts * attempts * 5)` seconds (quadratic backoff)
4. Max attempts: 10 (configurable via `bridge_config`)
5. On permanent failure: set status to `dead_letter`, generate `delivery.permanently_failed` event
6. **No recursive outbox events**: `delivery.failed` and `delivery.succeeded` events are NOT inserted into `bridge_outbox`. They are internal state changes only. Otherwise, delivery confirmations would create infinite outbox entries.

### Delivery Idempotency

Hermes may receive duplicate events if the outbox processor crashes after delivery but before marking as `delivered`. Hermes must handle duplicates. The `event_id` field serves as the deduplication key. Hermes should track processed event_ids and skip duplicates.

---

## Ticket/Kanban Review

### Verified Facts

From the codebase investigation:

1. `store.Ticket` exists with `ID int32`, `Status TicketStatus` (OPEN/IN_PROGRESS/CLOSED), `Description` must start with `/m/`
2. `CreateEscalationTicket(ctx, tenantID, ticketType, customerInfo, issue)` exists and creates both a Memo (Protected visibility) and a Ticket
3. The ticket's `Description` is `"/m/" + memoUID`
4. `Ticket.Type` is set to `"agent_escalation"`
5. `Ticket.CreatorID` is `1` (system user)

### Issues

1. **`bridge_session_state.ticket_id` should be `INTEGER` referencing `tickets(id)`**. The plan has this right. But the event payload uses `TicketID string` — this must be cast from int32.

2. **Ticket creation failure handling**: `CreateEscalationTicket` has a fallback that creates a ticket with `/m/agent-escalation` prefix if memo creation fails. The bridge must handle this — the ticket is still created, just with a different description format.

3. **One ticket per active handoff**: The plan states this as an invariant but doesn't enforce it. If two takeovers happen for the same session (e.g., after `released` → `human_active`), a second ticket is created. This is actually correct behavior — each handoff gets its own ticket. The invariant should be: "one **active** ticket per active handoff."

4. **Ticket status on release**: The plan says "updates the ticket status to `Closed` or `Resolved`." bchat's `TicketStatus` only has `OPEN`, `IN_PROGRESS`, and `CLOSED`. There is no `RESOLVED`. Use `CLOSED`.

5. **Reopened handoffs**: If a session goes `human_active` → `released` → `human_active`, the second takeover creates a new ticket. The first ticket should be `CLOSED` before the second is created. This must be enforced in the takeover handler.

### Recommendation

```go
// In takeover handler:
// 1. Check for existing active ticket for this session
// 2. If exists and status != CLOSED, reject takeover (or close the old ticket first)
// 3. Create new ticket via CreateEscalationTicket
// 4. Link ticket_id in bridge_session_state
```

---

## Privacy Review

### PII Leakage Vectors

1. **Customer messages to Hermes**: The `customer.message` event contains the raw customer message. If the customer shared a phone number, email, or address, this is sent to Hermes and then to Telegram. The plan mentions "redaction pass before inserting into `bridge_outbox`" but doesn't specify what gets redacted.

   **Recommendation**: Redact phone numbers, email addresses, and payment card numbers from the `customer.message` event payload before outbox insertion. Use the same regex patterns bchat already uses for customer info extraction. The full unredacted transcript stays in `bridge_messages` inside bchat.

2. **Chat history in takeover payload**: When the harness takes over, it receives the full chat history. This includes all customer messages. If Hermes stores this data, it's a PII exposure.

   **Recommendation**: The takeover event should include a summary (last 5 messages, customer info) rather than the full transcript. The full transcript stays in bchat.

3. **SSE event payloads**: `agent.reply` events are visible to the widget. If the human agent includes internal notes or PII in their reply, it's shown to the customer. This is intentional (the human is talking to the customer), but the admin UI should warn operators.

4. **Log redaction**: The plan doesn't mention log redaction. Bridge events may contain PII in payloads. bchat's `slog` logs should never log full event payloads at INFO level. Log only event_type, session_id, and tenant_id.

5. **`monitoring_scope = 'all_sessions'`**: This sends every AI message to Hermes. For tenants with strict privacy requirements, this is a risk. The plan defaults to `escalated_only`, which is correct. Consider adding a warning in the admin UI when enabling `all_sessions`.

### Retention

The plan doesn't specify data retention for bridge tables. Recommend:
- `bridge_events`: 90 days (configurable)
- `bridge_messages`: 90 days
- `bridge_outbox`: 30 days after delivery
- `bridge_nonces`: 10 minutes (already covered by purge)
- `bridge_idempotency_keys`: 24 hours
- `bridge_stream_tokens`: until expiry + 1 hour

---

## Missing Tests

### Schema and Migration Tests
- `TestBridgeSchemaForeignKeysEnabled` — verify FK enforcement is ON
- `TestBridgeSchemaCascadeDelete` — verify cascading deletes work
- `TestBridgeSchemaUniqueConstraints` — verify UNIQUE constraints reject duplicates
- `TestBridgeSchemaCheckConstraints` — verify CHECK constraints reject invalid values

### Tenant Isolation Tests
- `TestBridgeCrossTenantSessionAccess` — token for tenant A cannot access tenant B's SSE
- `TestBridgeCrossTenantTakeover` — key for tenant A cannot takeover tenant B's session
- `TestBridgeCrossTenantReply` — reply for tenant A's session rejected with tenant B's key

### HMAC Tests
- `TestBridgeHMACValidSignatureAccepted`
- `TestBridgeHMACInvalidSignatureRejected`
- `TestBridgeHMACReplayRejected` — same nonce, same timestamp
- `TestBridgeHMACExpiredTimestampRejected` — timestamp > 5 min old
- `TestBridgeHMACFutureTimestampRejected` — timestamp > 5 min in future
- `TestBridgeHMACMethodBinding` — signature for POST cannot be reused for GET
- `TestBridgeHMACPathBinding` — signature for /reply cannot be reused for /takeover
- `TestBridgeHMACBodyTamperingDetected` — modified body fails verification
- `TestBridgeHMACRevokedKeyRejected`
- `TestBridgeHMACUnknownKeyReturnsSameError` — no enumeration
- `TestBridgeHMACConstantTimeComparison` — timing-independent verification

### Nonce Tests
- `TestBridgeNoncePurge` — old nonces are deleted
- `TestBridgeNonceConcurrentInsert` — concurrent requests with same nonce, only one succeeds

### Idempotency Tests
- `TestIdempotencySameKeySameBodyReturnsCachedResponse`
- `TestIdempotencySameKeyDifferentBodyReturns409`
- `TestIdempotencyExpiredKeyEvicted` — old idempotency keys are cleaned up
- `TestIdempotencyMissingKeyRejected` — mutating endpoints require idempotency key

### Stream Token Tests
- `TestStreamTokenCryptoRandom` — token has sufficient entropy
- `TestStreamTokenHashStored` — raw token not in DB
- `TestStreamTokenConstantTimeCompare` — comparison is timing-safe
- `TestStreamTokenScopedToSession` — token for session A cannot access session B
- `TestStreamTokenScopedToTenant` — token for tenant A cannot access tenant B
- `TestStreamTokenRevokedOnRelease`
- `TestStreamTokenRevokedOnTimeout`
- `TestStreamTokenExpiredRejected`
- `TestStreamTokenReusableForReconnect`
- `TestStreamTokenRenewedOnNewMessage`
- `TestStreamTokenNotLeakedInLogs`
- `TestStreamTokenMaxConnectionsEnforced`

### SSE Tests
- `TestSSEFiltersInternalEvents` — widget never receives `ai.message`, `customer.message`, etc.
- `TestSSEFiltersBySession` — widget only receives events for its session
- `TestSSEReplayFromLastEventID` — reconnect delivers missed events
- `TestSSEReplayDoesNotSkipMessages` — no gaps in delivery
- `TestSSEReplayDoesNotDeliverFutureMessages` — no time travel
- `TestSSEHeartbeatKeepAlive` — connection stays open
- `TestSSEDeliversAgentReply` — human reply reaches widget
- `TestSSEDoesNotDeliverLateAgentReply` — late reply not pushed
- `TestSSEEventIDIsMonotonicSeq` — event IDs are seq values, not random
- `TestSSECORSValidation` — cross-origin SSE rejected
- `TestSSEReferrerPolicy` — token not leaked via Referer

### Outbox Tests
- `TestOutboxDeliversPendingEvents`
- `TestOutboxRetryOnFailure`
- `TestOutboxDeadLetterAfterMaxAttempts`
- `TestOutboxNoDuplicateDelivery` — crash between delivery and mark-delivered
- `TestOutboxOrderingPreserved` — events delivered in seq order
- `TestOutboxNoRecursiveEvents` — delivery confirmations don't create new outbox entries

### State Machine Tests
- `TestStateMachineValidTransitions` — all valid transitions succeed
- `TestStateMachineInvalidTransitionsRejected` — e.g., `ai` → `released` rejected
- `TestStateMachineDuplicateTakeover` — second takeover gets 409
- `TestStateMachineOptimisticLocking` — concurrent updates, one fails
- `TestStateMachineTimeoutRelease` — inactivity triggers auto-release
- `TestStateMachineReleaseWhileReplyInFlight` — reply becomes late_agent_reply
- `TestStateMachineReTakeoverAfterRelease` — released → human_active works
- `TestStateMachineClosedIsTerminal` — no transitions out of closed

### Chat Integration Tests
- `TestChatExternalHumanModeBypassesLLM` — no OpenRouter call in human mode
- `TestChatExternalHumanModeReturnsHoldResponse`
- `TestChatExternalHumanModeReturnsStreamToken`
- `TestChatExternalEscalationCreatesHandoffQueued`
- `TestChatExternalEscalationInjectsTicketNumber`
- `TestChatExternalResumeAfterRelease` — AI resumes with transcript
- `TestChatExternalStreamTokenRenewedOnMessage`

### Ticket Integration Tests
- `TestTicketCreatedOnTakeover`
- `TestTicketClosedOnRelease`
- `TestTicketCreationFailureHandled` — takeover fails gracefully
- `TestTicketOnePerActiveHandoff` — no duplicate active tickets
- `TestTicketReopenedHandoffCreatesNewTicket`

### Privacy Tests
- `TestRedactionBeforeOutbox` — PII redacted from outbox events
- `TestFullTranscriptInBridgeMessages` — unredacted in bridge_messages
- `TestLogsDoNotContainPII` — log output checked for phone/email patterns
- `TestMonitoringScopeDefault` — default is escalated_only
- `TestMonitoringScopeAllSessionsWarning` — admin UI shows warning

---

## Revised Acceptance Criteria

The following must all pass before the plan is considered implementation-ready:

- [ ] All bridge tables use `bridge_session_id INTEGER` FK instead of composite `(tenant_id, session_id)` FK
- [ ] `bridge_events` has FK to `bridge_session_state(id)` with CASCADE
- [ ] `bridge_outbox` has `UNIQUE(event_id)` constraint
- [ ] HMAC signature base includes method, path, content-type, timestamp, nonce, and body hash
- [ ] Body size limit (1MB) enforced before HMAC verification on all bridge endpoints
- [ ] `Content-Type: application/json` required on all bridge endpoints
- [ ] Stream tokens use `crypto/rand` with 256+ bits, stored as SHA-256 hash, compared with `ConstantTimeCompare`
- [ ] Stream token is reused across reconnects (not regenerated per message)
- [ ] `bridge_stream_tokens` has index on `(tenant_id, session_id)` for revocation
- [ ] `bridge_nonces` uses atomic INSERT-or-ignore for replay protection
- [ ] `bridge_idempotency_keys` has TTL and expiration
- [ ] `bridge_session_state` has `version` column for optimistic locking
- [ ] State machine includes `failed` state and `closed` terminal transitions
- [ ] All state transitions wrapped in `BEGIN IMMEDIATE` transactions
- [ ] Outbox has `UNIQUE(event_id)`, status lifecycle, dead-letter handling, no recursive events
- [ ] Outbox processor uses quadratic backoff with configurable max attempts
- [ ] SSE endpoint validates Origin against tenant's AllowedDomains
- [ ] SSE endpoint enforces max concurrent connections per session
- [ ] SSE handler strips token from access logs
- [ ] Widget `ChatResponse` type updated with `stream_token` in metadata
- [ ] Escalation detection references correct location in `processChat()` (not `evaluatePolicy()`)
- [ ] Ticket creation uses `int32` consistently (not `string` in event payloads)
- [ ] Ticket status uses `CLOSED` (not `RESOLVED` — doesn't exist in bchat)
- [ ] PII redaction applied before outbox insertion
- [ ] `monitoring_scope` defaults to `escalated_only`
- [ ] All missing indexes created (SSE replay, outbox poll, nonce purge, etc.)
- [ ] SQLite foreign keys verified as enabled in bchat's connection setup
- [ ] Error messages for HMAC failures are identical (no key enumeration)
- [ ] All tests from the Missing Tests section pass

---

## Final Recommendation

**Revise first, then implement in phases.**

The plan's architecture is solid — the event-sourced bridge with outbox, monotonic seq replay, and state machine is the right pattern for this problem. But the schema has a critical composite FK bug, the HMAC signature base is insufficient, the stream token lifecycle needs hardening, and the transaction boundaries are unspecified.

**Recommended revision order:**

1. Fix the schema: replace composite FKs with `bridge_session_id` integer FKs, add missing indexes, add `version` column, add `UNIQUE(event_id)` to outbox.
2. Fix the HMAC: add method+path+content-type to signature base, use body hash instead of raw body, add content-type enforcement, add body size limit.
3. Fix the stream token: specify crypto/rand generation, hash storage, constant-time comparison, reuse across reconnects, CORS validation.
4. Fix the state machine: add `failed` state, `closed` transitions, optimistic locking, transaction boundaries.
5. Fix the outbox: add dead-letter handling, no recursive events, proper status lifecycle.
6. Fix the ticket integration: use `int32` consistently, use `CLOSED` not `RESOLVED`, handle creation failures.
7. Add PII redaction, log sanitization, and retention policies.

After these revisions, implement in the plan's suggested phases: schema → security → chat integration → SSE/outbox → widget → admin UI. Each phase should have tests from the Missing Tests section before moving to the next.
