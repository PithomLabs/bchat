# bchat Flows — Implementation Plan

**Date:** 2026-08-05
**Author:** opencode (senior architect pass, with chaschel)
**Status:** DESIGN — pending approval. No code until this document is approved.
**Target:** CockroachDB × AWS Hackathon — deadline **Aug 19, 2026 05:00 GMT+8** (~14 days)
**Folder:** all working docs for this effort live in `bugs/059/`

---

## 1. Executive Summary

Turn bchat from a chat agent into an **intent-driven automation runtime**: given only
`KB.md`, `POLICY.md`, and `SCRIPT.md`, a tenant's agent can autonomously *do things* —
query databases, call webhooks, send email, schedule follow-ups — subject to its
persona, a per-tenant skill allowlist, risk-based approval gates, and hard budgets.

**Thesis (the n8n kill-shot):** n8n automates what you can *diagram* — one static node
graph per workflow, no judgment at runtime. bchat automates what you can *describe* —
one persona (3 markdown files) handles arbitrarily many workflows, adapts at runtime,
and remembers everything in CockroachDB.

> "n8n needed three node graphs. bchat needed three markdown files."

**Hackathon fit:** CockroachDB is the agent's durable memory — run state, step audit
log, vector-indexed artifacts, observation logs — globally consistent, always on.
Demo closes with a judge's own Claude/Cursor connecting via the **CRDB Cloud Managed
MCP Server** and querying the agent's memory read-only.

### Locked decisions (from design Q&A, 2026-08-05)

| # | Decision | Choice |
|---|----------|--------|
| D1 | Where automation is declared | Keep **3 files**; extend POLICY.md (`@skill`, `@automation`) and SCRIPT.md (`@stage` signals) |
| D2 | Extension model | **Compiled-in Go skills only**; tenants enable/constrain, never invent |
| D3 | Demo scenarios | Lead-to-booking (chat intent) + Support-ops triage (webhook) + Scheduled back-office (cron) — one tenant, three triggers |
| D4 | CRDB tools | Distributed Vector Indexing + Agent Skills Repo + Managed MCP Server (3 of 4; requirement is ≥2) |
| D5 | AWS service | ECS Fargate + S3 |
| D6 | Chat-triggered runs | **Async**: immediate ack, background run, result delivered back into session |
| D7 | Sandbox | Risk levels (read/write/external) + human **approval gate** + budgets (steps/tokens/wall-clock) |

---

## 2. Background: Deep-Dive Investigation Findings

Verified against the codebase 2026-08-05. File:line references are anchors for
implementation, not guarantees — re-verify before editing.

### 2.1 What already exists (reuse, don't rebuild)

| Capability | Location | Notes |
|---|---|---|
| Annotation grammar | `server/router/api/v1/agent/parser.go:100-200` | Regex `<!--\s*@(\w+)(?::\s*([^>]*))?\s*-->`; first keyless param becomes `code`. New annotation types = new `case` in `ParseKB`/`ParsePolicy` switches + store struct. Round-trip exporters `ExportKB`/`ExportPolicy` (`parser.go:644-862`) need matching cases |
| Intent → action seam | `parser.go:437-525` (`@intent`), `service.go:2585` (`classifyIntent`), `service.go:2929` (`evaluatePolicy`) | `@intent` already has an `action` param (default `standard_flow`, `parser.go:461-463`). `PolicyDecision{Action, Phase, SafetyTrigger, AppliedRules}` (`service.go:2921-2926`) is the launch point for runs |
| Start/pause/resume/stop lifecycle | `simulation.go:78-192` | `SimulationState` with `pauseCh/resumeCh/stopCh` (`:93-95`), in-memory `SimulationSessionStore` with TTL cleanup (`:105-181`). Clone this pattern for runs |
| Lifecycle HTTP | `handlers.go:3466-3536` (start), `:3541-3624` (SSE stream), `:3629-3678` (control) | Blueprint for run endpoints |
| Claim/lease job queue | `store/agent.go:1270-1282` (`AgentEvent`), `ClaimPendingEvents` (`store/driver.go:298`), poller `service.go:5496-5551` | Status machine pending/processing/delivered/failed, 300s lease reclaim, max 5 attempts, idempotency key. Template for `ClaimQueuedRuns` |
| Outbound webhooks (SSRF-safe) | `integrations.go:28-190` | `isInternalIP`, `validateAndResolveWebhookURL`, `buildSecureHTTPClient`, HMAC `signPayload`, `deliverWebhook`. Reuse inside `http.request`/`webhook.send` skills |
| Inbound machine auth | `bridge_middleware.go:29-234` (`RequireBridgeHMAC`) | Bearer key-id + timestamp ±5min + nonce replay protection + HMAC-SHA256. Reuse verbatim for webhook triggers |
| Async delivery into chat sessions | `delivery.go:15-201` | `DeliverWebChatReply`: claim outbox row → append to session transcript → persist → settle. `rebuildMemorySession` (`:166-201`). Template for run-result delivery |
| Cron library | `plugin/cron/` | Full vendored robfig/cron fork (spec parser, `AddFunc`, chains). **Imported by nothing today** — free to wire in |
| External cron fallback | `integrations.go:197-210` (`HandleTriggerCron`), `v1.go:457`, supercronic in both Fly Dockerfiles | Keep as fallback trigger tick |
| Tool calling | pinned `revrost/go-openrouter v1.1.5` | `ChatCompletionRequest.Tools/ToolChoice` + `ToolCall` types exist in the lib; **zero usages in bchat today**. No dependency change needed |
| Per-tenant feature flags | `TenantConfig.Features map[string]interface{}` (`store/rbac.go:36`) | Zero-migration gating: `features.flows = true` |
| S3 storage | `plugin/storage/s3/` | Used by resource service; reuse for artifact spill |
| CRDB vector index | `vectordb_cockroach.go:112` | `CREATE VECTOR INDEX IF NOT EXISTS` (bug 058 fix); `LANCEDB_STORAGE_PROVIDER=cockroach` swaps the whole VectorDB interface |
| Playground seeding | `playground.go` (`StartupSeedPlaygroundDemos`, `HandlePlaygroundRun:522`) | Extend to seed the demo tenant |
| LLM mock for tests | `llm_mock_test.go` + `OPENROUTER_API_BASE_URL` override (`service.go:58-67`) | Engine tests run without real LLM |

### 2.2 Chat flow today (where we hook in)

Real routes are `/chat/ext` (public) and `/chat/int` (authenticated) — AGENTS.md's
`/chat` + `/chat/stream` are stale; **chat has no SSE today** (only simulation +
notification hub do).

```
POST /api/v1/agent/:slug/chat/ext                     v1.go:290-301
 → HandleChatExternal                                 handlers.go:386-462
 → Service.ChatExternal                               service.go:2150-2378
    LoadConfig (5-min ConfigCache)                    service.go:1845-1960
    rate limits → memory session → idempotency
    → processChat                                     service.go:2532-2814
        sanitize → score → classifyIntent (LLM #1)    service.go:2585
        → evaluatePolicy                              service.go:2637
        → RAG-vs-longcontext decision                 service.go:2643-2705
        → generate(RAG)Response (LLM #2)              service.go:3011 / 3601
        → sanitize → verify → persist
    → transcript → lead capture → dispatchEvent("lead.captured")
```

### 2.3 Constraints & gotchas discovered

1. **Name collision:** `agent_workflows` / `AgentWorkflow` is taken (beads task log,
   `store/agent_workflow.go:9`, `LATEST.sql:602`). Feature is named **Flows**; tables
   are `agent_runs`, `agent_run_steps`, `agent_run_artifacts`.
2. **Migration parity is enforced at build:** sqlite + postgres files via
   `task migrate:new`; `validate:parity` + `validate:migrations` are build deps.
   Cockroach boots from `LATEST.sql` (must be updated too). Latest version today:
   **0.35** → new migration is **0.36**.
3. **CRDB Cloud Basic slow DDL:** first-boot LATEST.sql backfill takes 25–60 min;
   indexes can time out silently (`verifyCockroachIndexes` repairs at startup).
   Mitigation: keep 0.36 DDL tiny (3 tables, 4 indexes); pre-warm cluster; prefer
   Cloud Standard for the demo cluster.
4. **No graceful shutdown plumbing** for background goroutines — the run engine must
   own a cancellable `context.Context` rooted in `NewService` (`service.go:89-271`).
5. **Widget is request/response** — no push channel to the browser widget. Async
   results need a lightweight poll endpoint (see §6.4) or are visible on next
   interaction. Admin UI gets true SSE.
6. **Fly auto-stop** can kill long runs on the current deploy — the hackathon deploy
   target is **AWS ECS Fargate** (D5), which sidesteps this. Keep Fly profiles working.
7. **MySQL driver is stubbed** for agent features (existing pattern) — flows follow
   suit: sqlite + postgres/CRDB parity only.
8. **OpenRouter tool-calling reliability varies by model** — runs pin a known-good
   tool model via `TenantConfig` (new optional field `FlowModel`, fallback
   `LLM_MODEL_REASONING` env).

---

## 3. Goals / Non-Goals

### Goals
- G1: General-purpose automation runtime driven entirely by the 3 MD files + built-in Go skills.
- G2: Explicit **start/stop signals**: triggers start runs; SCRIPT.md terminal stages, budgets, operators, and policy violations stop them.
- G3: Production-grade safety: tenant skill allowlists, risk levels, approval gates, budgets, full step audit.
- G4: CockroachDB as the single durable memory layer (state, audit, vectors) — the hackathon story.
- G5: Win the hackathon: 3-scenario live demo + MCP finale + <3-min video.

### Non-Goals (explicitly out of scope)
- Visual node/graph editor (that's n8n's game; we refuse it).
- Dynamic code loading, WASM, go-plugin sidecars, scripting languages (D2: Go-only, compiled-in).
- Tenant-authored skills. Tenants configure; developers extend.
- Cross-tenant workflows. Everything is tenant-scoped, no exceptions.
- MySQL parity for flows (stub only, matches existing agent-feature pattern).

---

## 4. The Killer Demo Narrative

**Setting:** one demo tenant, **"Field Services Co"** (slug `field-services`), seeded
by playground startup. Three markdown files define the entire agent. CRDB Cloud holds:
appointments, customers, tickets, inventory tables (demo data) + all agent memory.

**Act 1 — Intent-triggered run (chat).**
Judge opens the embedded widget on the demo page and types:
*"My basement is flooding, can someone come tomorrow morning?"*
The agent (persona: emergency dispatcher) classifies intent `schedule_service`, which
POLICY.md binds to automation `booking_flow`. Chat replies instantly:
*"I'm on it — checking crews for tomorrow morning."* In the background a **run**
starts: `crdb.query` (available crews/regions) → `memory.recall` (customer history) →
`crdb.exec` (INSERT appointment) → `webhook.send` (dispatch notification) →
`flow.control(complete)`. The confirmation is **delivered back into the same chat
session**: *"Booked: tomorrow 8–10am, crew #3. Ref FS-1042."*
Admin panel shows the run's live step timeline via SSE.

**Act 2 — Webhook-triggered run (support-ops).**
An external system (simulated by curl in the video) POSTs an HMAC-signed webhook:
new complaint ticket. Automation `ticket_triage` runs: `memory.recall` (past issues
for this customer, vector search) → `crdb.query` (order/warranty status) → drafts
reply → `email.send` — **risk: external → the run pauses at `awaiting_approval`**.
Judge clicks **Approve** in the admin UI; email sends (mock SMTP, logged). Every step
is audited in `agent_run_steps`.

**Act 3 — Cron-triggered run (back-office).**
*"Every agent needs a night shift."* Automation `nightly_ops` (`cron:0 9 * * *`,
manually fired for the demo): scans CRDB for unconfirmed appointments and stale leads
→ sends reminders (`webhook.send`, budgeted) → writes a digest via `memory.remember`
→ posts digest to ops webhook. The digest artifact is vector-indexed.

**Finale — Memory that never goes down (MCP).**
Open Claude Code on the judge's machine with the CRDB Cloud **Managed MCP Server**
config snippet (read-only). Ask: *"What did the field-services agent do today and
what does it remember about customer Acme?"* Claude queries curated views over
`agent_runs`, `agent_run_steps`, `agent_run_artifacts` — the agent's entire day,
its audit trail, and its semantic memory, from outside the app.

**Closing line:** three triggers, one persona, three markdown files, zero node graphs.
All state survived in CockroachDB — kill the ECS task mid-demo, restart, and the
queued/paused runs resume (claim-lease queue). *That* is agentic memory.

---

## 5. Design

### 5.1 Skill framework (new package `server/router/api/v1/agent/skill/`)

```go
package skill

type Risk string

const (
    RiskRead     Risk = "read"     // no side effects
    RiskWrite    Risk = "write"    // mutates our own DB/state
    RiskExternal Risk = "external" // leaves the building (email, http, webhook)
)

type Spec struct {
    Name        string         // "crdb.query"
    Title       string         // human label for UI
    Description string         // shown to the LLM (tool description)
    Risk        Risk
    InputSchema map[string]any // JSON Schema → go-openrouter Tool
}

type Context struct {
    context.Context
    TenantID int32
    RunID    int64
    Store    Store            // narrow interface over *store.Store (avoids cycle)
    Vectors  VectorStore      // narrow interface over agent VectorDB
    HTTP     *http.Client     // SSRF-safe client from integrations.go
    Budget   *BudgetState     // atomic counters: steps, llm calls, deadline
    Grants   GrantSet         // tenant constraints for THIS skill (tables, caps)
    Counters *SkillCounters   // per-run per-skill invocation counts
    Logger   *slog.Logger
}

type Result struct {
    Output   any       // JSON-serializable; fed back to LLM as tool result
    Artifact *Artifact // optional: persisted + embedded (memory.remember)
    Log      string    // one-line human audit entry
}

type Skill interface {
    Spec() Spec
    Execute(ctx *Context, input json.RawMessage) (*Result, error)
}
```

**Registry** (same package): `Register(Skill)` called from each skill file's `init()`;
`Registry.All()`, `Registry.Get(name)`. Compile-time only (D2).

**Hackathon skill set** (build order = phase order):

| Skill | Risk | Purpose | Constraints enforced |
|---|---|---|---|
| `crdb.query` | read | SELECT against tenant DB | read-only txn, table allowlist (grant `tables:`), row cap, statement timeout |
| `crdb.exec` | write | parameterized INSERT/UPDATE | table allowlist, no DDL/TRUNCATE, rows-affected report |
| `memory.remember` | write | persist artifact + embed | size cap; S3 spill >64KB |
| `memory.recall` | read | vector search over artifacts + observations | tenant-scoped, top-k cap |
| `flow.control` | write | `advance_stage` / `complete` / `escalate` | engine pseudo-skill; the stop-signal mechanism |
| `http.request` | external | outbound HTTP | SSRF-safe client, method allowlist, resp truncation |
| `webhook.send` | external | notify via tenant integration | reuses outbox (`dispatchEvent` path) |
| `email.send` | external | SMTP send | `require_approval: true` default; mock-transport mode for demo |
| `ticket.create` | write | create internal ticket | existing ticket store |
| `schedule.followup` | write | enqueue delayed run | `scheduled_at`, dedupe key |
| `crdb.expertise` | read | **Agent Skills Repo pack**: answer CRDB schema/ops/perf questions from embedded `cockroachdb-skills-main` docs (`go:embed` + keyword/embedding lookup) | read-only corpus |

**Tenant gating:** effective toolset for a run = `registry ∩ POLICY.md grants`.
A skill with no grant is invisible to the LLM. Grants carry constraints
(`tables:`, `max_per_run:`, `require_approval:`) enforced by the engine, not by
trusting the LLM.

### 5.2 Annotation extensions (parser.go)

POLICY.md — new cases in `ParsePolicy`:

```markdown
<!-- @skill: crdb.query, tables: appointments customers crews, max_per_run: 50 -->
<!-- @skill: crdb.exec, tables: appointments, max_per_run: 10 -->
<!-- @skill: email.send, require_approval: true, max_per_run: 3 -->
<!-- @skill: webhook.send -->
<!-- @skill: memory.remember -->
<!-- @skill: memory.recall -->

<!-- @automation: booking_flow, trigger: intent:schedule_service, budget_steps: 20, budget_minutes: 5 -->
## Booking Flow
Triggered when a customer wants to book a service. Check availability, book, confirm.

<!-- @automation: ticket_triage, trigger: webhook, budget_steps: 30, budget_minutes: 10 -->
<!-- @automation: nightly_ops, trigger: cron:0 9 * * *, budget_steps: 50, budget_minutes: 15 -->
```

SCRIPT.md — stage annotations attach to the section they precede:

```markdown
<!-- @stage: intake, signal: start -->
## Stage: Intake
...
<!-- @stage: done, signal: stop -->
## Stage: Done
...
```

New store types (`store/agent.go` or new `store/flows.go`):

```go
type AgentSkillGrant struct {
    TenantID  int32
    SkillName string
    Config    map[string]string // tables, max_per_run, require_approval, ...
}

type AgentAutomation struct {
    ID            int32
    TenantID      int32
    Code          string // "booking_flow"
    Description   string // block content under the annotation
    TriggerType   string // intent | webhook | cron | manual | scheduled
    TriggerRef    string // intent code | cron spec | ""
    BudgetSteps   int    // default 20
    BudgetMinutes int    // default 5
    IsActive      bool
    LastRunAt     *time.Time // cron scheduling state
}
```

`ScriptSection` gains `Annotations map[string]string` (`signal: start|stop`,
future `emit:`). `ParsedPolicy`/`ParsedScript` gain the new slices. Persisted in the
existing file-processing path alongside other parsed entities; `ExportPolicy`
round-trips the new annotations.

**Intent binding:** existing `@intent` gains meaning for `action: run:<code>`
(e.g. `<!-- @intent: schedule_service, action: run:booking_flow -->`). `classifyIntent`
already returns the intent code; the engine maps `run:<code>` → enqueue.

### 5.3 Run engine

**State machine** (`agent_runs.status`):

```
scheduled ─► queued ─► running ─► completed
                          │  ▲
                          │  └── resume
                          ├─► paused
                          ├─► awaiting_approval ─► running   (approve)
                          │                     └► cancelled (deny / timeout)
                          ├─► failed    (error | budget | policy violation)
                          ├─► stopped   (operator stop signal)
                          └─► cancelled
```

**Engine loop** (goroutine per claimed run; channels cloned from `simulation.go`):

```
1. Load tenant config (LoadConfig) + grants + automation + script
2. Build system prompt:
   identity + security constraints (existing buildSystemPrompt sections)
   + AUTOMATION brief + SCRIPT stages (current highlighted)
   + KB context via RAG over trigger input (CRDB vector search)
   + AVAILABLE SKILLS (from grants) + budgets + guardrails
3. Loop:
   a. select stop/pause channels (non-blocking; pause blocks on resume/stop)
   b. budget check: steps_used, wall-clock deadline, llm_calls → fail with reason
   c. LLM call with Tools (go-openrouter), FlowModel, low temperature
   d. For each ToolCall:
      - resolve skill; verify grant exists (else log + refuse to LLM)
      - risk check: external && grant.require_approval →
          persist approval step, status=awaiting_approval, SSE event,
          block on approvalCh (cap: approval_timeout, default 10m → cancelled)
      - execute skill.Execute with Context (counters, budget decrement)
      - persist agent_run_steps row (kind=skill, input, output, duration)
      - emit SSE event; append tool message
   e. flow.control handling: advance_stage → update current_stage + step row
      (kind=stage); complete → break if stage has signal:stop (else warn LLM);
      escalate → create ticket + stop
   f. No tool calls → final answer; if current stage is terminal → complete
4. Finalize: status + result JSON; dispatchEvent("run.completed"/"run.failed")
5. If session_id != "" → deliver result into chat session (§5.5)
```

**Runtime registry:** in-memory `map[int64]*RunState` with TTL cleanup — clone of
`SimulationSessionStore` (`simulation.go:105-181`). Survives restarts only as DB
state: on startup, runs stuck in `running`/`paused` with stale leases are requeued
(same reclaim semantics as `ClaimPendingEvents`).

**Worker:** ticker loop started in `NewService` (pattern: `service.go:206-217`),
gated by `FLOWS_ENABLED=true`. Each tick: (1) enqueue due cron automations,
(2) promote due `scheduled` runs, (3) `ClaimQueuedRuns(5)` → launch goroutines.
Claim is atomic (UPDATE-claim), so multiple ECS tasks are safe.

### 5.4 Triggers

| Trigger | Entry point | Auth |
|---|---|---|
| Chat intent | `processChat` after `classifyIntent`/`evaluatePolicy` (`service.go:2585-2637`); `PolicyDecision.Action == "run:<code>"` → enqueue with `session_id`, input = message + extracted facts; generated reply acks | existing chat auth |
| Webhook | `POST /api/v1/agent/:slug/flows/:code/trigger` | `RequireBridgeHMAC` (reuse verbatim) |
| Cron | worker tick scans active `trigger_type=cron` automations; due = cron spec matches since `LastRunAt`; idempotency key `automation:<code>:<slot>` | internal |
| Manual | `POST /:slug/flows/:code/run` (admin) | AuthMiddleware + `api:config` permission |
| Scheduled | `schedule.followup` skill inserts run with `scheduled_at` | internal |

All enqueue paths write one `agent_runs` row (status `queued`) and emit SSE/event.

### 5.5 Async delivery back into chat (D6)

On completion with non-empty `session_id`:
1. Build delivery message from `result` (LLM-summarized if long, template fallback).
2. Append as assistant message via the `delivery.go` pattern
   (rebuild memory session → append → persist transcript).
3. Widget visibility: new lightweight poll endpoint
   `GET /api/v1/agent/:slug/chat/ext/poll?session_id=&after=<msg_id>` returning
   messages delivered since `after` (public CORS + widget-key gate, same as chat/ext).
   Widget polls every 2s while a run is pending (signaled in the ack payload:
   `"run_started": true`). Stretch if time is tight — fallback: confirmation also
   goes out via webhook/email and appears on next user message.

### 5.6 Approval gate (D7)

- Pending approval = `agent_run_steps` row `kind=approval, status=pending`
  (input = skill name + args summary + risk rationale).
- Run parks in `awaiting_approval`; SSE event `approval_requested`.
- `POST /runs/:id/control {"action":"approve"|"deny","step_id":N}` resolves it;
  decision recorded (who/when) on the step row.
- Timeout (`approval_timeout`, default 10 min) → run `cancelled` + notification event.
- UI: pending-approvals banner in RunsSection (admin). Chat-based approval: post-MVP.

### 5.7 Memory layer on CockroachDB (hackathon core)

| Memory kind | Storage | Vector-indexed |
|---|---|---|
| Run state + audit | `agent_runs` / `agent_run_steps` (CRDB, multi-region) | — |
| Artifacts (digests, notes, docs) | `agent_run_artifacts` (content or S3 key) | yes — embedding via existing VectorDB interface, `source_ref="artifact:<id>"` (works on Lance for sqlite dev, CRDB HNSW for the demo) |
| Conversations | existing sessions/transcripts | existing ticket embedder pattern |
| Observations | existing OM `observation_logs` | readable by `memory.recall` |
| External audit | curated views `v_agent_runs_summary`, `v_agent_memory` exposed via **CRDB Cloud Managed MCP** (read-only) | — |

No new vector columns needed — reuse `agent_vectors` through the VectorDB interface,
which keeps sqlite/postgres/CRDB parity intact.

---

## 6. Data Model & API

### 6.1 Migration 0.36 (`task migrate:new NAME=add_flows`)

sqlite + postgres parity (cockroach via LATEST.sql). DDL sketch (types per
`docs/TYPE_MAPPING.md`):

```sql
CREATE TABLE IF NOT EXISTS agent_runs (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,   -- pg: BIGSERIAL
  tenant_id       INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
  automation_code TEXT NOT NULL DEFAULT '',
  trigger_type    TEXT NOT NULL,          -- chat_intent|webhook|cron|manual|scheduled
  trigger_ref     TEXT NOT NULL DEFAULT '',
  session_id      TEXT NOT NULL DEFAULT '',
  status          TEXT NOT NULL DEFAULT 'queued',
  current_stage   TEXT NOT NULL DEFAULT '',
  intent          TEXT NOT NULL DEFAULT '',
  input           TEXT NOT NULL DEFAULT '',   -- JSON (pg: JSONB)
  result          TEXT NOT NULL DEFAULT '',   -- JSON
  budget_steps    INTEGER NOT NULL DEFAULT 20,
  budget_seconds  INTEGER NOT NULL DEFAULT 300,
  steps_used      INTEGER NOT NULL DEFAULT 0,
  llm_calls       INTEGER NOT NULL DEFAULT 0,
  error           TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  scheduled_at    TIMESTAMP NULL,
  claimed_at      TIMESTAMP NULL,
  started_at      TIMESTAMP NULL,
  finished_at     TIMESTAMP NULL,
  created_ts      ... , updated_ts ...
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_runs_idem
  ON agent_runs(tenant_id, idempotency_key) WHERE idempotency_key != '';  -- pg partial; sqlite: plain unique on (tenant_id,idempotency_key) with '' tolerated via expression or app-side dedupe
CREATE INDEX IF NOT EXISTS idx_agent_runs_tenant_status ON agent_runs(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_agent_runs_claim ON agent_runs(status, scheduled_at);

CREATE TABLE IF NOT EXISTS agent_run_steps (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id      INTEGER NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  tenant_id   INTEGER NOT NULL,
  seq         INTEGER NOT NULL,
  kind        TEXT NOT NULL,              -- llm|skill|stage|approval|signal
  skill_name  TEXT NOT NULL DEFAULT '',
  risk        TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL DEFAULT 'ok', -- ok|error|denied|timeout|pending|approved
  input       TEXT NOT NULL DEFAULT '',   -- JSON
  output      TEXT NOT NULL DEFAULT '',   -- JSON
  error       TEXT NOT NULL DEFAULT '',
  actor       TEXT NOT NULL DEFAULT '',   -- approver user id, if any
  duration_ms INTEGER NOT NULL DEFAULT 0,
  created_ts  ...
);
CREATE INDEX IF NOT EXISTS idx_agent_run_steps_run ON agent_run_steps(run_id, seq);

CREATE TABLE IF NOT EXISTS agent_run_artifacts (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id     INTEGER NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  tenant_id  INTEGER NOT NULL,
  kind       TEXT NOT NULL,               -- memory|digest|document
  name       TEXT NOT NULL DEFAULT '',
  content    TEXT NOT NULL DEFAULT '',
  s3_key     TEXT NOT NULL DEFAULT '',
  source_ref TEXT NOT NULL DEFAULT '',    -- "artifact:<id>" vector linkage
  created_ts ...
);
CREATE INDEX IF NOT EXISTS idx_agent_run_artifacts_tenant ON agent_run_artifacts(tenant_id);

CREATE TABLE IF NOT EXISTS agent_automations (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id      INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
  code           TEXT NOT NULL,
  description    TEXT NOT NULL DEFAULT '',
  trigger_type   TEXT NOT NULL,
  trigger_ref    TEXT NOT NULL DEFAULT '',
  budget_steps   INTEGER NOT NULL DEFAULT 20,
  budget_minutes INTEGER NOT NULL DEFAULT 5,
  is_active      INTEGER NOT NULL DEFAULT 1,
  last_run_at    TIMESTAMP NULL,
  created_ts ..., updated_ts ...,
  UNIQUE(tenant_id, code)
);

CREATE TABLE IF NOT EXISTS agent_skill_grants (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id  INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
  skill_name TEXT NOT NULL,
  config     TEXT NOT NULL DEFAULT '',    -- JSON
  UNIQUE(tenant_id, skill_name)
);
```

Driver methods appended to `store/driver.go` (after `:299`) + delegating `Store`
methods, implemented sqlite + postgres (mysql stub): `CreateAgentRun`,
`UpdateAgentRun`, `GetAgentRun`, `ListAgentRuns`, `ClaimQueuedRuns(limit)`,
`RequeueStaleRuns(leaseAge)`, `CreateAgentRunStep`, `ListAgentRunSteps`,
`CreateAgentRunArtifact`, `ListAgentRunArtifacts`, automation + grant CRUD.

### 6.2 API surface (registered in `v1.go` `RegisterAgentRoutes`)

| Method | Path | Group/Auth | Purpose |
|---|---|---|---|
| GET | `/:slug/flows` | authGroup, `tenant:read` | automations + grants (parsed config) |
| POST | `/:slug/flows/:code/run` | adminGroup, `api:config` | manual trigger (optional JSON input) |
| GET | `/:slug/runs` | authGroup, `chat:logs` | list runs (status filter, paging) |
| GET | `/:slug/runs/:id` | authGroup, `chat:logs` | run detail + steps |
| GET | `/:slug/runs/:id/stream` | authGroup, `chat:logs` | SSE live step events (simulation-stream clone) |
| POST | `/:slug/runs/:id/control` | adminGroup, `api:config` | `{action: pause\|resume\|stop\|approve\|deny}` |
| POST | `/:slug/flows/:code/trigger` | **new flowsGroup w/ RequireBridgeHMAC** | inbound webhook trigger |
| GET | `/:slug/chat/ext/poll` | publicGroup + widget-key gate | delivered messages since `after` (§5.5) |

Permission checks via `service.CheckUserPermission` (existing pattern). Tenant
isolation per AGENTS.md checklist: context tenant id on every query, ownership check
on control actions, superuser bypass, no tenant ids in errors.

### 6.3 Frontend (web/)

New `web/src/pages/AgentAdminSections/RunsSection.tsx` (follow
`IntegrationsSection.tsx` extraction pattern), rendered in `AgentAdmin.tsx` next to
Integrations:
- Automations list: code, trigger badge (intent/webhook/cron), budgets, active toggle, **Run now**.
- Pending-approvals banner (poll `runs?status=awaiting_approval`).
- Runs table: status chips, trigger, duration, steps used; click → detail drawer.
- Detail drawer: step timeline (SSE live), input/output JSON viewers, Approve/Deny/Stop/Pause buttons.
- Skill grants viewer (read-only table of what this tenant may use).

Store methods in `web/src/store/v2/agentAdmin.ts` (export in return object ~`:1751`);
i18n keys under `agent-admin.flows.*` in `web/src/locales/en.json`.
Widget: pending-run indicator + poll loop (stretch, §5.5).

### 6.4 Config & flags

- Env: `FLOWS_ENABLED` (master switch, default false), `FLOWS_WORKER_INTERVAL=10s`,
  `FLOWS_APPROVAL_TIMEOUT=10m`, `FLOW_MODEL` (fallback tool-calling model).
- Tenant: `TenantConfig.Features["flows"]=true` + optional `FlowModel`.
- Demo tenant seeded via playground extension with all three automations.

---

## 7. File-by-File Implementation Guide

### New files
| File | Contents |
|---|---|
| `server/router/api/v1/agent/skill/skill.go` | Risk, Spec, Context, Result, Skill interface, Registry |
| `server/router/api/v1/agent/skill/budget.go` | BudgetState, SkillCounters, GrantSet parsing/enforcement |
| `server/router/api/v1/agent/skill/crdb.go` | `crdb.query`, `crdb.exec` (allowlist enforcement, read-only txn) |
| `server/router/api/v1/agent/skill/memory.go` | `memory.remember`, `memory.recall` |
| `server/router/api/v1/agent/skill/flow_control.go` | `flow.control` |
| `server/router/api/v1/agent/skill/http.go` | `http.request`, `webhook.send` (reuse integrations.go helpers — export them) |
| `server/router/api/v1/agent/skill/email.go` | `email.send` (mock transport + optional SMTP) |
| `server/router/api/v1/agent/skill/misc.go` | `ticket.create`, `schedule.followup` |
| `server/router/api/v1/agent/skill/crdb_expertise.go` + `skill/testdata/cockroachdb-skills/` | vendored Agent Skills Repo pack, `go:embed` |
| `server/router/api/v1/agent/run_engine.go` | RunState, runtime registry, agent loop, approval wait, finalize |
| `server/router/api/v1/agent/runs.go` | worker loop, claim, cron scan, enqueue paths, delivery |
| `server/router/api/v1/agent/run_handlers.go` | HTTP handlers for §6.2 |
| `store/flows.go` | AgentRun, AgentRunStep, AgentRunArtifact, AgentAutomation, AgentSkillGrant + Find types |
| `store/migration/{sqlite,postgres}/0.36/00__add_flows.sql` | §6.1 DDL (per-driver syntax) |
| `web/src/pages/AgentAdminSections/RunsSection.tsx` | §6.3 UI |
| `scripts/aws/` | ECS task definition, deploy script, S3 bucket bootstrap |
| `bugs/059/e2e/*.sh` | curl-based scenario scripts for the demo/video |

### Modified files (exact seams)
| File | Change |
|---|---|
| `server/router/api/v1/agent/parser.go` | `@skill` + `@automation` cases in `ParsePolicy`; `@stage` annotation attach in `ParseScript`; `ParsedPolicy`/`ParsedScript` fields; `ExportPolicy` round-trip |
| `server/router/api/v1/agent/service.go` | NewService: start flows worker (`:206-217` pattern); `processChat` (`:2585-2637`): detect `run:<code>` action → enqueue + ack flag; `dispatchEvent` new event types `run.started/completed/failed`; export SSRF helpers if moved |
| `server/router/api/v1/agent/handlers.go` | none directly (new run_handlers.go); possibly export helpers |
| `server/router/api/v1/v1.go` | register flows routes: authGroup/adminGroup additions + flowsGroup with `RequireBridgeHMAC` + poll endpoint in publicGroup |
| `store/driver.go` | new interface methods (§6.1) |
| `store/db/sqlite/flows.go`, `store/db/postgres/flows.go` | implementations; mysql stub |
| `store/migration/{sqlite,postgres}/LATEST.sql` | full-schema parity for 0.36 |
| `store/rbac.go` | new permission `flows:run` (or reuse `api:config` — decide in Phase 1; default: reuse) |
| `playground.go` | seed `field-services` demo tenant (KB/POLICY/SCRIPT + demo CRDB tables) |
| `web/src/pages/AgentAdmin.tsx` | render RunsSection |
| `web/src/store/v2/agentAdmin.ts` | flows store methods |
| `web/src/locales/en.json` | `agent-admin.flows.*` |
| `widget/src/core/api.ts` + state | poll loop (stretch) |
| `Taskfile.yml` | `run:flows` (FLOWS_ENABLED=true + cockroach env), `deploy:aws` |

---

## 8. Phases & Acceptance Criteria

**P0 — Scaffolding (day 1, ~half day)**
Branch, `task migrate:new NAME=add_flows`, `store/flows.go` types, driver stubs,
`FLOWS_ENABLED` flag, feature-flag plumbing.
✔ `task validate:parity` + `validate:migrations` pass; build green on sqlite+pg.

**P1 — Skill framework + data layer (days 1–3)**
skill package (interface, registry, budget/grants), `crdb.query/exec`,
`memory.remember/recall`, `flow.control`; parser annotations + persistence of
automations/grants; migrations finalized.
✔ Unit tests: annotation parsing, grant enforcement (table allowlist rejects,
counters cap), registry. ✔ crdb skills pass integration tests vs `task crdb:up`
local cluster. ✔ Parity green.

**P2 — Run engine + lifecycle (days 4–6)**
run_engine.go + runs.go worker; state machine; SSE stream + control endpoints;
approval gate; manual trigger endpoint; RunsSection read-only MVP.
✔ Manual run on dev tenant executes ≥3-step tool loop (LLM mock acceptable for
tests; real model manually). ✔ pause/resume/stop verified. ✔ external skill parks
run at awaiting_approval; approve continues, deny cancels, timeout cancels.
✔ Restart-recovery: kill process mid-run → stale lease requeued on boot.

**P3 — Triggers + async delivery + booking E2E (days 7–9)**
Chat-intent trigger wiring; webhook trigger endpoint (HMAC); poll endpoint;
delivery into session; widget pending indicator (stretch).
✔ E2E: chat "flooding, come tomorrow" → run → appointment row in CRDB →
confirmation visible in same session. ✔ Webhook curl with valid HMAC enqueues;
invalid HMAC rejected (nonce replay too). ✔ Idempotency: duplicate webhook ≠ 2 runs.

**P4 — Scenarios 2+3 + skills pack (days 10–11)**
ticket_triage (approval + email mock), nightly_ops (cron scan + digest + webhook
out); `crdb.expertise` with vendored skills repo; seed demo tenant complete.
✔ All three scenarios run unattended via `bugs/059/e2e/*.sh`. ✔ cron fires on
schedule locally; due-slot idempotent across 2 worker instances.

**P5 — Hackathon layer (days 12–13)**
CRDB Cloud Standard cluster (pre-warmed); curated MCP views + console MCP snippet;
ECS Fargate deploy + S3 artifact spill; RunsSection polish (approval banner,
timeline); event types in Integrations event log.
✔ Claude Code connects via MCP read-only and answers "what did the agent do today".
✔ bchat on ECS vs CRDB Cloud; kill-task resume demo works.

**P6 — Submission (day 14)**
Video <3 min (script in §4), README (setup/run/license MIT), architecture diagram,
Devpost form (tools used + how). Buffer for P3–P5 slip lives here.

---

## 9. Hackathon Compliance Matrix

| Requirement | Our usage |
|---|---|
| **Distributed Vector Indexing** | RAG + artifact embeddings in CRDB HNSW indexes via `vectordb_cockroach.go`; `memory.recall` semantic search over agent memory |
| **Agent Skills Repo** | vendored into `crdb.expertise` skill — the agent *uses* CockroachDB expertise skills at runtime |
| **Managed MCP Server** | read-only MCP endpoint on the bchat cluster; demo finale: external agent audits bchat memory |
| **AWS** | ECS Fargate (agent runtime) + S3 (artifact storage) |
| Judging: Agentic Memory Design | CRDB = run state + audit + vectors + OM; restart/kill resilience demo |
| Judging: Technical Implementation | claim-lease queue, HMAC auth, SSRF guards, approval gates, migration parity |
| Judging: Real-World Impact | field-services scenario = real SMB operations automation |
| Judging: Production Readiness | budgets, risk levels, audit trail, multi-instance safety, observability (SSE + event log) |
| Judging: Creativity | "automation by persona, not by diagram" — 3 MD files replace node graphs |

---

## 10. Risks & Mitigations

| Risk | P | Mitigation |
|---|---|---|
| CRDB Cloud slow DDL / index timeouts (Basic) | H | Cloud **Standard** for demo cluster; tiny 0.36 DDL; pre-warm before video; `verifyCockroachIndexes` already repairs |
| Tool-calling flakiness per model | H | pin `FlowModel` to a proven tool model; strict JSON schemas; low temp; verifier optional; LLM-mock regression tests |
| Scope creep (3 scenarios) | H | one engine — scenarios are config; cut order if slipping: widget poll → email SMTP (keep mock) → crdb.expertise |
| Async delivery UX in widget | M | poll endpoint is small; fallback = webhook/email confirmation + visible on next message |
| Runaway LLM loops / cost | M | hard budgets (steps/wall-clock/llm_calls), per-skill caps, global concurrent-run cap (worker claim limit) |
| Multi-instance double execution | M | atomic claim + idempotency keys + lease reclaim (proven outbox pattern) |
| Approval stalls block runs forever | L | approval timeout → cancel + notify |
| Fly auto-stop kills runs (current deploy) | L | demo on ECS; document for Fly users |
| Video/demo-day failures | M | scripted e2e under bugs/059; pre-record acts separately; local CRDB fallback recording |

---

## 11. Verification Strategy

- **Unit:** parser annotations (incl. export round-trip), grant/budget enforcement,
  state-machine transitions, allowlist SQL guards (injection attempts), claim query.
- **Integration:** full engine vs LLM mock (`OPENROUTER_API_BASE_URL`, existing
  `llm_mock_test.go` pattern) — deterministic tool-call scripts.
- **E2E:** `task crdb:up` local cluster + seeded demo tenant; curl scripts per
  scenario (booking, webhook triage w/ approval, cron sweep); happy + sad paths
  (bad HMAC, nonce replay, budget exhaustion, deny approval).
- **Parity/CI:** `task validate:parity`, `validate:migrations`, `go vet`, existing
  test suites must stay green (they're build deps).
- **Resilience:** kill process mid-run → reclaim; two workers, one queue, zero dupes;
  10 concurrent runs smoke.

---

## 12. Open Items (resolve during P1)

1. Permission: reuse `api:config` for run control vs new `flows:run` — default reuse.
2. `email.send` transport for demo: mock-log (default) vs real SMTP via SES — decide P4.
3. Partial unique index syntax for sqlite idempotency (expression index fallback).
4. Whether `crdb.query/exec` target the app's own CRDB database (demo data lives in
   bchat's cluster, separate schema `demo`) — recommended yes, single cluster story.
5. MCP views column set — finalize with video script in P5.

---

**Approval gate:** implementation starts only after this document is approved.
Approval = comment in this file's header changing Status to `APPROVED` + date.
