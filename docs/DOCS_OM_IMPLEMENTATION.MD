# Implementation Plan: Observational Memory in bchat

**Date:** 2026-02-13
**Status:** DRAFT

## Goal Description
Implement "Observational Memory" (OM) to give bchat agents infinite effective memory by compressing conversation history into a concise observation log, replacing the current 10-message limit.

## User Review Required
> [!IMPORTANT]
> This introduces a new database table `agent_observations`.
> The "Observer" logic will run as a background goroutine, potentially increasing LLM costs (though offset by smaller prompt context).

## Proposed Changes

### Database Layer (`store/`)

#### [NEW] `store/migration/sqlite/0.25/24__agent_observations.sql`
- Create `agent_observations` table linked to `agent_sessions`.
- Columns: `session_id` (PK, TEXT), `tenant_id` (INT), `observation_log` (TEXT), `last_observed_msg_index` (INT), `tokens_in_log` (INT), `last_updated_at` (TIMESTAMP).

#### [MODIFY] `store/agent.go`
- Add `ObservationLog` struct definition.
- Add store interface methods: `GetObservationLog`, `UpsertObservationLog`.

#### [MODIFY] `store/db/sqlite/agent.go`
- Implement SQLite CRUD for `ObservationLog`.

### Service Layer (`server/router/api/v1/agent/`)

#### [NEW] `prompts/observer.txt`
- Port of `observer-agent.ts` system prompt to Go template format.

#### [NEW] `prompts/reflector.txt`
- Port of `reflector-agent.ts` system prompt to Go template format.

#### [NEW] `observer.go`
- `RunObserver` method: Background worker logic.
- Triggers LLM call to generate observations from unobserved messages.
- Updates `agent_observations` table.

#### [MODIFY] `service.go`
- Update `CreateChatCompletion` to:
    1. Fetch existing observations for the session.
    2. Inject observations into the System Prompt.
    3. Trigger `RunObserver` asynchronously if unobserved message count > threshold (e.g., 10).

## Verification Plan

### Automated Tests
- N/A (Manual verification preferred for LLM logic).

### Manual Verification
1.  **Schema Check**: Verify migration applies successfully on startup.
2.  **Observation Generation**:
    - Start a chat session.
    - Send > 10 messages.
    - Verify `agent_observations` table populates.
    - Check log file for "Observer triggered" messages.
3.  **Context Injection**:
    - Ask the agent about something from early in the conversation (message 1-5).
    - Verify it answers correctly using the observation log (since raw history is truncated).
