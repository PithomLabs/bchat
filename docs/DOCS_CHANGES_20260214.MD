# RAG Indexing Mechanism Fix & Progress Reporting Plan

## Problem Analysis

1.  **Audience Mismatch**: Uploading a KB file works because `HandleImportSingleFile` uses the explicit `audience_type` from the form. The "Rebuild Index" button was previously hardcoded to `internal` and didn't allow selecting `all`.
2.  **Lack of Visibility**: The reindex process (especially with resume support) can be slow for large KBs, but the UI provides no detailed progress beyond a generic loading state.
3.  **Synchronous vs Asynchronous**: File upload performs a synchronous `indexContentForRAG`, while the reindex button uses `ReindexTenantContentWithResume` which processes in batches but lacks a polling mechanism in the frontend.

## Proposed Changes

### Backend

#### 1. Data Store Enhancements
Modify `store/agent.go` and `store/db/sqlite/agent.go`:
- Add `LastMessage` (string) to `ReindexCheckpoint` struct.
- Update SQLite schema and `UpsertReindexCheckpoint` to persist this field.

#### 2. Service Logic
Modify `server/router/api/v1/agent/service.go`:
- Add `LastMessage` to `ReindexStatus` struct.
- Update `ReindexTenantContentWithResume` to set `LastMessage` at the start of each batch with the format: `Processing batch batch=12 totalBatches=29 chunksInBatch=32 progress=384/918...`.
- Update `GetReindexStatus` to include the `LastMessage` in the response.

### Frontend

#### 1. Store Updates
Modify `web/src/store/v2/agentAdmin.ts`:
- Add `fetchReindexStatus` action to call `/api/v1/agent/:slug/reindex/status`.
- Add `reindexStatus` state to `LocalState`.

#### 2. UI Enhancements
Modify `web/src/pages/AgentAdmin.tsx`:
- Add an `audienceType` selector for the "Rebuild Index" button (options: "All", "Internal", "External").
- Implement polling for reindex status when a process is active.
- Display the `lastMessage` from the status response in the UI (e.g., in a status bar or log preview).

## Verification Plan

### Automated
- Unit test for `ReindexTenantContentWithResume` to ensure `LastMessage` is correctly formatted and persisted.
- Verify API response of `/reindex/status` contains the new field.

### Manual
- Upload a file and verify it still indexes correctly.
- Use the "Rebuild Index" button with "All" audiences and verify that progress "logs" appear in the UI.
