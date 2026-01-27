# Chat Transcript Recording Feature

**Status:** Planned
**Date:** 2026-01-27
**Priority:** Medium

---

## Overview

Add transcript recording capability to all chat conversations with a tenant-level setting to enable/disable recording. The checkbox defaults to `true` (recording enabled).

### Key Features
- Record all external chat conversations to database
- Tenant-level toggle to enable/disable recording (default: enabled)
- View and manage transcripts in Agent Admin
- Capture metadata: client IP, user agent, customer info, intent

---

## Current State Analysis

### Internal Sessions (Already Persisted)
| Aspect | Status |
|--------|--------|
| Storage | `agent_sessions` table |
| Messages | JSON column `messages` |
| Persistence | On every message exchange |
| Access | `GET /api/v1/agent/:slug/sessions/:sessionId` |

### External Sessions (NOT Persisted) - Gap to Fill
| Aspect | Status |
|--------|--------|
| Storage | `MemorySessionStore` (in-memory only) |
| TTL | 30 minutes, lost on server restart |
| Persistence | **None** |
| Access | **None** |

**This feature fills the gap by persisting external chat sessions.**

---

## Implementation Plan

### Phase 1: Database Schema

#### 1.1 New Table: `agent_transcripts`

```sql
-- store/migration/sqlite/0.25/18__agent_transcripts.sql

CREATE TABLE IF NOT EXISTS agent_transcripts (
    id TEXT PRIMARY KEY,                              -- UUID
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,                         -- Original session ID
    audience_type TEXT NOT NULL,                      -- "external" or "internal"

    -- Conversation data
    messages TEXT NOT NULL DEFAULT '[]',              -- JSON array of messages
    message_count INTEGER DEFAULT 0,

    -- Metadata
    client_ip TEXT,                                   -- For external sessions
    user_agent TEXT,                                  -- Browser info

    -- Extracted info
    customer_name TEXT,
    customer_phone TEXT,
    customer_email TEXT,
    customer_location TEXT,
    detected_intent TEXT,

    -- Timestamps
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    last_message_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Completion
    is_completed INTEGER DEFAULT 0,
    completion_reason TEXT,                           -- "resolved", "abandoned", "escalated"

    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_transcripts_tenant ON agent_transcripts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_transcripts_started ON agent_transcripts(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_transcripts_audience ON agent_transcripts(tenant_id, audience_type);
```

#### 1.2 Add Setting Column to TenantConfig

```sql
-- store/migration/sqlite/0.25/18__agent_transcripts.sql (continued)

ALTER TABLE tenant_config ADD COLUMN record_transcripts INTEGER DEFAULT 1;
```

---

### Phase 2: Backend Changes

#### 2.1 Store Layer Types

**store/agent.go** - Add transcript types:

```go
// AgentTranscript represents a recorded chat conversation
type AgentTranscript struct {
    ID               string          `json:"id"`
    TenantID         int32           `json:"tenant_id"`
    SessionID        string          `json:"session_id"`
    AudienceType     string          `json:"audience_type"`
    Messages         []AgentMessage  `json:"messages"`
    MessageCount     int             `json:"message_count"`
    ClientIP         string          `json:"client_ip,omitempty"`
    UserAgent        string          `json:"user_agent,omitempty"`
    CustomerName     string          `json:"customer_name,omitempty"`
    CustomerPhone    string          `json:"customer_phone,omitempty"`
    CustomerEmail    string          `json:"customer_email,omitempty"`
    CustomerLocation string          `json:"customer_location,omitempty"`
    DetectedIntent   string          `json:"detected_intent,omitempty"`
    StartedAt        time.Time       `json:"started_at"`
    EndedAt          *time.Time      `json:"ended_at,omitempty"`
    LastMessageAt    time.Time       `json:"last_message_at"`
    IsCompleted      bool            `json:"is_completed"`
    CompletionReason string          `json:"completion_reason,omitempty"`
}

// FindAgentTranscript specifies search criteria for transcripts
type FindAgentTranscript struct {
    ID           *string
    TenantID     *int32
    SessionID    *string
    AudienceType *string
    Limit        int
    Offset       int
}
```

#### 2.2 Driver Interface

**store/driver.go** - Add interface methods:

```go
// Transcript methods
CreateAgentTranscript(ctx context.Context, transcript *AgentTranscript) (*AgentTranscript, error)
GetAgentTranscript(ctx context.Context, find *FindAgentTranscript) (*AgentTranscript, error)
ListAgentTranscripts(ctx context.Context, find *FindAgentTranscript) ([]*AgentTranscript, error)
UpdateAgentTranscript(ctx context.Context, transcript *AgentTranscript) error
DeleteAgentTranscript(ctx context.Context, id string) error
```

#### 2.3 SQLite Implementation

**store/db/sqlite/agent.go** - Implement CRUD operations:

```go
func (d *DB) CreateAgentTranscript(ctx context.Context, transcript *store.AgentTranscript) (*store.AgentTranscript, error) {
    messagesJSON, _ := json.Marshal(transcript.Messages)

    _, err := d.db.ExecContext(ctx, `
        INSERT INTO agent_transcripts (
            id, tenant_id, session_id, audience_type, messages, message_count,
            client_ip, user_agent, customer_name, customer_phone, customer_email,
            customer_location, detected_intent, started_at, last_message_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, transcript.ID, transcript.TenantID, transcript.SessionID, transcript.AudienceType,
       string(messagesJSON), transcript.MessageCount, transcript.ClientIP, transcript.UserAgent,
       transcript.CustomerName, transcript.CustomerPhone, transcript.CustomerEmail,
       transcript.CustomerLocation, transcript.DetectedIntent, transcript.StartedAt,
       transcript.LastMessageAt)

    return transcript, err
}

// GetAgentTranscript, ListAgentTranscripts, UpdateAgentTranscript, DeleteAgentTranscript...
```

#### 2.4 Service Layer

**service.go** - Add transcript recording:

```go
// shouldRecordTranscript checks tenant config for recording setting
func (s *Service) shouldRecordTranscript(ctx context.Context, tenantID int32) bool {
    config, err := s.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenantID})
    if err != nil || config == nil {
        return true // Default to recording
    }
    return config.RecordTranscripts
}

// saveTranscript persists the chat session to the transcripts table
func (s *Service) saveTranscript(ctx context.Context, session *store.AgentSession, clientIP, userAgent string) error {
    transcript := &store.AgentTranscript{
        ID:               session.ID,
        TenantID:         session.TenantID,
        SessionID:        session.ID,
        AudienceType:     session.AudienceType,
        Messages:         session.Messages,
        MessageCount:     session.MessageCount,
        ClientIP:         clientIP,
        UserAgent:        userAgent,
        CustomerName:     session.CustomerName,
        CustomerPhone:    session.CustomerPhone,
        CustomerLocation: session.CustomerLocation,
        DetectedIntent:   session.CurrentIntent,
        LastMessageAt:    time.Now(),
        StartedAt:        session.CreatedAt,
    }

    // Upsert - create or update existing transcript
    existing, _ := s.store.GetAgentTranscript(ctx, &store.FindAgentTranscript{SessionID: &session.ID})
    if existing != nil {
        return s.store.UpdateAgentTranscript(ctx, transcript)
    }
    _, err := s.store.CreateAgentTranscript(ctx, transcript)
    return err
}
```

**Modify ChatExternal():**

```go
func (s *Service) ChatExternal(ctx context.Context, tenantSlug, clientIP string, req *ChatRequest) (*ChatResponse, error) {
    // ... existing code ...

    // Get user agent from context if available
    userAgent := "" // Extract from request headers

    // After processing chat, save transcript if enabled
    if s.shouldRecordTranscript(ctx, config.TenantID) {
        if err := s.saveTranscript(ctx, session, clientIP, userAgent); err != nil {
            slog.Warn("Failed to save transcript", "sessionID", session.ID, "error", err)
            // Don't fail the request, just log the error
        }
    }

    return response, nil
}
```

#### 2.5 Handler Layer

**handlers.go** - Add transcript endpoints:

```go
// HandleListTranscripts returns all transcripts for a tenant
// GET /api/v1/agent/:slug/transcripts
func (h *Handler) HandleListTranscripts(c echo.Context) error {
    slug := c.Param("slug")

    // Verify admin access
    tenant, err := h.store.GetAgentTenantBySlug(c.Request().Context(), slug)
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
    }

    transcripts, err := h.store.ListAgentTranscripts(c.Request().Context(), &store.FindAgentTranscript{
        TenantID: &tenant.ID,
        Limit:    100,
    })
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list transcripts")
    }

    return c.JSON(http.StatusOK, map[string]interface{}{
        "transcripts": transcripts,
        "total":       len(transcripts),
    })
}

// HandleGetTranscript returns a single transcript with full messages
// GET /api/v1/agent/:slug/transcripts/:id
func (h *Handler) HandleGetTranscript(c echo.Context) error {
    // ... implementation
}

// HandleDeleteTranscript deletes a transcript
// DELETE /api/v1/agent/:slug/transcripts/:id
func (h *Handler) HandleDeleteTranscript(c echo.Context) error {
    // ... implementation
}
```

#### 2.6 Route Registration

**v1.go** - Register routes:

```go
// Transcript endpoints (admin only)
adminGroup.GET("/:slug/transcripts", s.agentHandler.HandleListTranscripts)
adminGroup.GET("/:slug/transcripts/:id", s.agentHandler.HandleGetTranscript)
adminGroup.DELETE("/:slug/transcripts/:id", s.agentHandler.HandleDeleteTranscript)
```

---

### Phase 3: Frontend Changes

#### 3.1 Setting Checkbox

**web/src/pages/AgentAdmin.tsx:**

```tsx
// Add in settings section
<Box sx={{ mt: 2 }}>
  <Checkbox
    label={t("agent-admin.record-transcripts")}
    checked={tenantConfig?.recordTranscripts ?? true}
    onChange={(e) => handleUpdateTenantConfig("recordTranscripts", e.target.checked)}
    size="sm"
  />
  <Typography level="body-xs" sx={{ ml: 3, color: "text.tertiary" }}>
    {t("agent-admin.record-transcripts-description")}
  </Typography>
</Box>
```

#### 3.2 Transcripts Viewer Section

**web/src/pages/AgentAdmin.tsx:**

```tsx
// New collapsible section for transcripts
<Accordion defaultExpanded={false}>
  <AccordionSummary>
    <Typography level="title-md" startDecorator={<ChatIcon />}>
      {t("agent-admin.transcripts")}
      {transcripts.length > 0 && ` (${transcripts.length})`}
    </Typography>
  </AccordionSummary>
  <AccordionDetails>
    {transcripts.length === 0 ? (
      <Typography level="body-sm" color="neutral">
        {t("agent-admin.transcripts-empty")}
      </Typography>
    ) : (
      <Table size="sm">
        <thead>
          <tr>
            <th>Started</th>
            <th>Customer</th>
            <th>Intent</th>
            <th>Messages</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {transcripts.map((t) => (
            <tr key={t.id}>
              <td>{formatDate(t.startedAt)}</td>
              <td>{t.customerName || "Anonymous"}</td>
              <td>{t.detectedIntent || "-"}</td>
              <td>{t.messageCount}</td>
              <td>
                <IconButton size="sm" onClick={() => viewTranscript(t.id)}>
                  <VisibilityIcon />
                </IconButton>
                <IconButton size="sm" color="danger" onClick={() => deleteTranscript(t.id)}>
                  <DeleteIcon />
                </IconButton>
              </td>
            </tr>
          ))}
        </tbody>
      </Table>
    )}
  </AccordionDetails>
</Accordion>
```

#### 3.3 MobX Store

**web/src/store/v2/agentAdmin.ts:**

```typescript
// Add Transcript interface
export interface Transcript {
  id: string;
  sessionId: string;
  audienceType: string;
  messages: AgentMessage[];
  messageCount: number;
  clientIp?: string;
  userAgent?: string;
  customerName?: string;
  customerPhone?: string;
  customerEmail?: string;
  customerLocation?: string;
  detectedIntent?: string;
  startedAt: string;
  endedAt?: string;
  lastMessageAt: string;
  isCompleted: boolean;
  completionReason?: string;
}

// Add to LocalState
transcripts: Transcript[] = [];
isLoadingTranscripts: boolean = false;
selectedTranscript: Transcript | null = null;

// Add methods
const fetchTranscripts = async (slug: string) => {
  state.setPartial({ isLoadingTranscripts: true });
  try {
    const response = await axios.get<{ transcripts: Transcript[] }>(
      `/api/v1/agent/${slug}/transcripts`
    );
    runInAction(() => {
      state.transcripts = response.data.transcripts;
      state.isLoadingTranscripts = false;
    });
  } catch (error) {
    runInAction(() => {
      state.isLoadingTranscripts = false;
    });
  }
};

const deleteTranscript = async (slug: string, id: string): Promise<boolean> => {
  try {
    await axios.delete(`/api/v1/agent/${slug}/transcripts/${id}`);
    runInAction(() => {
      state.transcripts = state.transcripts.filter((t) => t.id !== id);
    });
    return true;
  } catch (error) {
    return false;
  }
};
```

#### 3.4 Translations

**web/src/locales/en.json:**

```json
{
  "agent-admin": {
    "record-transcripts": "Record transcript of chat conversations",
    "record-transcripts-description": "Save all external chat conversations for review and analysis. Enabled by default.",
    "transcripts": "Chat Transcripts",
    "transcripts-empty": "No transcripts recorded yet. Start a chat conversation to see transcripts here.",
    "transcript-view": "View Transcript",
    "transcript-delete": "Delete Transcript",
    "transcript-delete-confirm": "Are you sure you want to delete this transcript? This action cannot be undone.",
    "transcript-customer": "Customer",
    "transcript-anonymous": "Anonymous",
    "transcript-started": "Started",
    "transcript-messages": "Messages",
    "transcript-intent": "Detected Intent"
  }
}
```

---

## Data Flow

```
External Chat Request (POST /api/v1/agent/:slug/chat/ext)
        │
        ▼
HandleChatExternal(c)
        │
        ├─► Extract clientIP, userAgent from request
        │
        ▼
ChatExternal(ctx, tenantSlug, clientIP, req)
        │
        ├─► processChat() → Updates in-memory session
        │
        ▼
shouldRecordTranscript(tenantID)?
        │
   YES  │  NO
        ▼   └──────────────────────────────────────┐
saveTranscript(session, clientIP, userAgent)       │
        │                                          │
        ▼                                          │
GetAgentTranscript(sessionID)                      │
        │                                          │
        ├─► EXISTS: UpdateAgentTranscript()        │
        │                                          │
        └─► NEW: CreateAgentTranscript()           │
                │                                  │
                ▼                                  ▼
        SQLite: agent_transcripts          Return response
                │                          (no persistence)
                ▼
        Return response
```

---

## Files to Modify

| File | Change | Priority |
|------|--------|----------|
| `store/migration/sqlite/0.25/18__agent_transcripts.sql` | **NEW** - Create table + setting column | High |
| `store/agent.go` | Add `AgentTranscript`, `FindAgentTranscript` types | High |
| `store/driver.go` | Add interface methods | High |
| `store/db/sqlite/agent.go` | Implement CRUD | High |
| `store/db/mysql/agent.go` | Add stubs (return `errNotImplemented`) | Medium |
| `store/db/postgres/agent.go` | Add stubs (return `errNotImplemented`) | Medium |
| `server/router/api/v1/agent/service.go` | Add `shouldRecordTranscript`, `saveTranscript` | High |
| `server/router/api/v1/agent/handlers.go` | Add transcript endpoints | High |
| `server/router/api/v1/v1.go` | Register routes | High |
| `web/src/pages/AgentAdmin.tsx` | Add checkbox + transcripts viewer | Medium |
| `web/src/store/v2/agentAdmin.ts` | Add transcript state/methods | Medium |
| `web/src/locales/en.json` | Add translations | Low |

---

## API Reference

### List Transcripts
```
GET /api/v1/agent/:slug/transcripts
Authorization: Required (Admin)

Response:
{
  "transcripts": [
    {
      "id": "uuid",
      "sessionId": "uuid",
      "audienceType": "external",
      "messageCount": 5,
      "customerName": "John Doe",
      "detectedIntent": "service_inquiry",
      "startedAt": "2026-01-27T10:00:00Z",
      "lastMessageAt": "2026-01-27T10:05:00Z"
    }
  ],
  "total": 1
}
```

### Get Transcript
```
GET /api/v1/agent/:slug/transcripts/:id
Authorization: Required (Admin)

Response:
{
  "id": "uuid",
  "messages": [
    {"role": "user", "content": "Hello", "timestamp": "..."},
    {"role": "agent", "content": "Hi! How can I help?", "timestamp": "..."}
  ],
  ...
}
```

### Delete Transcript
```
DELETE /api/v1/agent/:slug/transcripts/:id
Authorization: Required (Admin)

Response: 204 No Content
```

---

## Verification Checklist

### 1. Recording Enabled (Default)
- [ ] Start server: `task run:rag:cybertron`
- [ ] Send external chat message via widget or API
- [ ] Check database: `sqlite3 build/data/memos_dev.db "SELECT id, message_count FROM agent_transcripts;"`
- [ ] Should see transcript with messages

### 2. Recording Disabled
- [ ] In Agent Admin, uncheck "Record transcript of chat conversations"
- [ ] Save settings
- [ ] Send external chat message
- [ ] Check database - no new transcript should be created

### 3. View Transcripts
- [ ] Open Agent Admin for tenant
- [ ] Expand "Chat Transcripts" section
- [ ] Should see list of recorded conversations
- [ ] Click view icon to see full transcript

### 4. Delete Transcript
- [ ] Click delete icon on transcript
- [ ] Confirm deletion
- [ ] Transcript removed from list
- [ ] Verify removed from database

### 5. Transcript Updates
- [ ] Continue existing conversation (same session ID)
- [ ] Verify transcript updates with new messages
- [ ] Message count should increase

---

## Privacy Considerations

1. **Data Retention**: Consider adding automatic cleanup of old transcripts
2. **PII Handling**: Customer name, phone, email are stored - ensure compliance with privacy policies
3. **IP Address**: Client IP is logged - may need consent notice
4. **Opt-out**: Checkbox allows tenants to disable recording entirely

---

## Future Enhancements

1. **Export transcripts** - CSV/JSON export functionality
2. **Search transcripts** - Full-text search across conversations
3. **Analytics dashboard** - Conversation metrics, popular intents
4. **Auto-cleanup** - Configurable retention period (30/60/90 days)
5. **Transcript tags** - Manual tagging for categorization

---

*Document Version: 1.0*
*Last Updated: 2026-01-27*
