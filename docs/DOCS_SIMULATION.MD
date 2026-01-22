# Agent Simulation Feature Specification

## Overview

The Agent Simulation feature enables automated testing of AI agent conversations by simulating realistic human interactions. This feature runs a back-and-forth conversation between:

- **Simulated Human**: An LLM (using the same model from environment variables) prompted to act as a realistic customer
- **AI Agent**: The existing tenant-configured agent with KB, Policy, and intents

Users provide an initial scenario, and the system automatically generates a natural conversation flow with color-coded messages to distinguish between participants.

## Key Requirements

| Requirement | Value |
|-------------|-------|
| Run Mode | Automatic (continuous back-and-forth) |
| Minimum Turns | 10 (always run at least 10 turns) |
| Maximum Turns | 50 (safety cap) |
| End Detection | LLM analysis + agent phase + keyword fallback |
| Transcript Storage | Save to database at end of conversation |
| Transcript Content | Messages + metadata (intent, phase, urgency) |
| Access Control | Requires `chat:test` permission (same as Internal Agent) |

---

## Architecture

### Flow Diagram

```
User provides initial prompt
        |
        v
[Frontend: AgentSimulation.tsx]
        |
        v
POST /api/v1/agent/:slug/simulate (start)
        |
        v
[Backend: RunSimulation()]
        |
        +---> Loop (10-50 turns):
        |        |
        |        v
        |    [Human Simulator LLM] ---> "Human" message
        |        |
        |        v
        |    [AI Agent processChat()] ---> "Agent" message
        |        |
        |        v
        |    [End Detection Check]
        |        |
        |        +-- If ended: Save transcript, close
        |        |
        |        +-- If continue: Next turn
        |        |
        |        v
        |    SSE: Stream messages to client
        |
        v
Return final conversation, save transcript
```

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| `AgentSimulation.tsx` | UI, user input, SSE connection, message display |
| `agentSimulation.ts` | MobX state management, API calls |
| `simulation.go` | Types, orchestration, human prompt, end detection |
| `handlers.go` | HTTP handlers, SSE streaming |
| `service.go` | Integration with existing chat infrastructure |
| `agent.go` (store) | Database operations for transcripts |

---

## Database Schema

### New Table: `agent_simulation_transcript`

```sql
CREATE TABLE agent_simulation_transcript (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    initial_prompt TEXT NOT NULL,
    persona_hint TEXT,
    total_turns INTEGER NOT NULL,
    end_reason TEXT NOT NULL,
    messages TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id)
);

CREATE INDEX idx_simulation_transcript_tenant ON agent_simulation_transcript(tenant_id);
CREATE INDEX idx_simulation_transcript_created ON agent_simulation_transcript(created_at);
```

### Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT | UUID for the simulation session |
| `tenant_id` | INTEGER | Foreign key to agent_tenant |
| `user_id` | INTEGER | User who ran the simulation |
| `initial_prompt` | TEXT | User-provided starting scenario |
| `persona_hint` | TEXT | Optional persona characteristics |
| `total_turns` | INTEGER | Number of complete turns |
| `end_reason` | TEXT | How simulation ended: `llm_detected`, `phase_closed`, `keyword_match`, `max_turns`, `user_stopped` |
| `messages` | TEXT | JSON array of messages with metadata |
| `created_at` | TIMESTAMP | When simulation was created |

### Messages JSON Structure

```json
[
  {
    "role": "human_sim",
    "content": "Hi, I just noticed water damage in my basement after the storm last night.",
    "turn_num": 1,
    "timestamp": "2026-01-20T10:30:15Z"
  },
  {
    "role": "agent",
    "content": "I'm sorry to hear about the water damage. Let me help you get this addressed quickly...",
    "turn_num": 1,
    "timestamp": "2026-01-20T10:30:18Z",
    "metadata": {
      "intent": "water_damage",
      "phase": "triage",
      "urgency": 2
    }
  }
]
```

---

## API Endpoints

### POST `/api/v1/agent/:slug/simulate`

Start a new simulation session.

**Authentication:** Required (user must have `chat:test` permission)

**Request:**
```json
{
  "initial_prompt": "I just noticed water damage in my basement after a storm",
  "persona_hint": "frustrated homeowner"
}
```

**Response:**
```json
{
  "session_id": "sim-550e8400-e29b-41d4-a716-446655440000",
  "status": "running",
  "stream_url": "/api/v1/agent/acme/simulate/sim-550e8400-e29b-41d4-a716-446655440000/stream"
}
```

### GET `/api/v1/agent/:slug/simulate/:sessionId/stream`

SSE stream for receiving simulation messages in real-time.

**Authentication:** Required

**SSE Events:**

```
event: message
data: {"role":"human_sim","content":"Hi, I just noticed...","turn_num":1,"timestamp":"2026-01-20T10:30:15Z"}

event: message
data: {"role":"agent","content":"I'm sorry to hear...","turn_num":1,"timestamp":"2026-01-20T10:30:18Z","metadata":{"intent":"water_damage","urgency":2,"phase":"triage"}}

event: status
data: {"status":"running","current_turn":2,"responding_role":"human_sim"}

event: complete
data: {"status":"completed","total_turns":12,"end_reason":"llm_detected"}

event: error
data: {"error":"LLM rate limit exceeded"}
```

### POST `/api/v1/agent/:slug/simulate/:sessionId/control`

Control an active simulation.

**Request:**
```json
{
  "action": "pause"
}
```

**Actions:**
- `pause` - Pause the simulation
- `resume` - Resume a paused simulation
- `stop` - Stop and save the simulation

**Response:**
```json
{
  "success": true,
  "status": "paused"
}
```

### GET `/api/v1/agent/:slug/simulations`

List saved simulation transcripts for a tenant.

**Query Parameters:**
- `limit` (default: 20, max: 100)
- `offset` (default: 0)

**Response:**
```json
{
  "simulations": [
    {
      "id": "sim-550e8400-e29b-41d4-a716-446655440000",
      "initial_prompt": "I just noticed water damage...",
      "total_turns": 12,
      "end_reason": "llm_detected",
      "created_at": "2026-01-20T10:30:00Z"
    }
  ],
  "total": 45
}
```

### GET `/api/v1/agent/:slug/simulations/:id`

Get a specific simulation transcript.

**Response:**
```json
{
  "id": "sim-550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": 1,
  "user_id": 5,
  "initial_prompt": "I just noticed water damage in my basement",
  "persona_hint": "frustrated homeowner",
  "total_turns": 12,
  "end_reason": "llm_detected",
  "messages": [...],
  "created_at": "2026-01-20T10:30:00Z"
}
```

---

## End Detection Logic

The simulation checks for conversation completion after each turn (minimum 10 turns). Detection methods are applied in order:

### 1. Agent Phase Detection (Primary)

Check if agent metadata indicates completion:
- `phase: "resolved"` - Issue has been resolved
- `phase: "closed"` - Conversation naturally closed
- `phase: "escalated"` - Handed off (counts as complete)

### 2. LLM Analysis (Secondary)

After each agent response, an LLM call evaluates conversation state:

```
Analyze this conversation and determine if it has reached a natural conclusion.

A conversation is complete if:
- The customer's issue has been resolved
- The customer has indicated they have no more questions
- A clear next step has been established (callback scheduled, etc.)
- The conversation has reached a natural goodbye

Conversation:
{{last_5_messages}}

Respond with JSON: {"is_complete": true/false, "reason": "brief explanation"}
```

### 3. Keyword Fallback (Tertiary)

Detect closing phrases in the last human or agent message:

**Human closing phrases:**
- "thank you, goodbye"
- "thanks, that's all"
- "I think that covers it"
- "no more questions"
- "I'll call back"

**Agent closing phrases:**
- "Is there anything else I can help you with?" + human "no"
- "Have a great day"
- "We'll be in touch"

### 4. Maximum Turns (Safety)

Hard stop at 50 turns to prevent runaway simulations.

---

## Human Simulator Prompt

The LLM generating human responses receives this system prompt:

```
You are simulating a realistic human customer/user interacting with {{companyName}}'s AI assistant.

CONTEXT:
- Company: {{companyName}}
- Industry: {{vertical}}
- Initial scenario: "{{initialPrompt}}"
{{#if personaHint}}
- Persona: {{personaHint}}
{{/if}}

YOUR ROLE:
You are playing the part of a customer. Your responses should be:

1. NATURAL AND HUMAN-LIKE:
   - Use casual language, contractions, sometimes informal grammar
   - Include occasional typos (rarely, realistically)
   - Express emotions naturally (frustration, relief, confusion, gratitude)
   - Ask follow-up questions when clarification is needed
   - Sometimes go slightly off-topic, then return to the main issue

2. REALISTIC BEHAVIOR:
   - Provide information gradually, not all at once
   - React to the agent's tone and helpfulness
   - If asked for details, provide them (but not always completely)
   - Express urgency appropriately based on the situation
   - Occasionally misunderstand or need clarification

3. STAY IN CHARACTER:
   - You are a real person with this problem, not an AI
   - Never mention being a simulation, test, or AI
   - Have realistic knowledge gaps about the company's services
   - React to wait times, policies, processes like a real customer would

4. CONVERSATION DYNAMICS:
   - If satisfied with an answer, express thanks and potentially ask a follow-up
   - If frustrated, express it but remain civil
   - Eventually reach a natural conclusion (resolved, need to call, will think about it)

CONVERSATION HISTORY:
{{conversationHistory}}

Generate the next human response. Keep it realistic - usually 1-3 sentences, occasionally longer for complex explanations. Do not include any prefix like "Human:" or quotation marks.
```

---

## UI Design

### Page Layout

```
+--------------------------------------------------+
| [Flask Icon] Agent Simulation                    |
|   [Tenant: ACME Corp] [Disconnect]    [Clear]    |
+--------------------------------------------------+
|                                                  |
| (If no tenant selected)                          |
|   +--------------------------------------------+ |
|   | Select a Tenant                            | |
|   | Enter tenant slug to test against          | |
|   | [________________] [Validate]              | |
|   +--------------------------------------------+ |
|                                                  |
+--------------------------------------------------+
| (If tenant selected, simulation not started)     |
|   +--------------------------------------------+ |
|   | Initial Scenario                           | |
|   | Describe the customer's situation...       | |
|   | [                                        ] | |
|   | [                                        ] | |
|   +--------------------------------------------+ |
|   | Persona (optional): [____________________] | |
|   +--------------------------------------------+ |
|   |            [Start Simulation]              | |
|   +--------------------------------------------+ |
|                                                  |
+--------------------------------------------------+
| (If simulation running)                          |
|   +--------------------------------------------+ |
|   | Status: Running     Turn 7/50              | |
|   | [Pause] [Stop]                             | |
|   +--------------------------------------------+ |
|   |                                            | |
|   | +----------------------------------------+ | |
|   | | HUMAN (Simulated)         Turn 1       | | <- Blue background
|   | | Hi, I just noticed water damage in my  | |
|   | | basement after the storm last night.   | |
|   | | 10:30:15 AM                            | |
|   | +----------------------------------------+ | |
|   |                                            | |
|   | +----------------------------------------+ | |
|   | | AI AGENT                    Turn 1     | | <- Teal background
|   | | I'm sorry to hear about the water      | |
|   | | damage. Let me help you...             | |
|   | | Intent: water_damage | Phase: triage   | |
|   | | 10:30:18 AM                            | |
|   | +----------------------------------------+ | |
|   |                                            | |
|   | +----------------------------------------+ | |
|   | | HUMAN (Simulated)         Turn 2       | | <- Blue, pulsing
|   | | Typing...                              | |
|   | +----------------------------------------+ | |
|   |                                            | |
|   +--------------------------------------------+ |
|                                                  |
+--------------------------------------------------+
```

### Color Scheme

| Element | Light Mode | Dark Mode |
|---------|------------|-----------|
| Human message bg | `bg-blue-100` | `bg-blue-900/30` |
| Human message text | `text-blue-900` | `text-blue-100` |
| Human badge | `bg-blue-200` | `bg-blue-800` |
| Agent message bg | `bg-teal-500` | `bg-teal-600` |
| Agent message text | `text-white` | `text-white` |
| Agent badge | `bg-teal-600` | `bg-teal-700` |

### Controls

| State | Available Controls |
|-------|-------------------|
| Idle (no tenant) | Tenant selection |
| Tenant selected | Initial prompt input, Start button |
| Running | Pause, Stop |
| Paused | Resume, Stop |
| Completed/Stopped | Clear, Start new |

---

## State Management

### MobX Store Structure

```typescript
interface SimulationMessage {
  role: "human_sim" | "agent";
  content: string;
  turnNum: number;
  timestamp: Date;
  metadata?: {
    intent?: string;
    phase?: string;
    urgency?: number;
  };
}

interface SimulationState {
  // Tenant
  tenantSlug: string | null;
  tenantCompanyName: string | null;
  isValidating: boolean;

  // Simulation
  sessionId: string | null;
  status: "idle" | "running" | "paused" | "completed" | "stopped";
  messages: SimulationMessage[];
  currentTurn: number;
  maxTurns: number;
  endReason: string | null;

  // Configuration
  initialPrompt: string;
  personaHint: string;

  // UI
  respondingRole: "human_sim" | "agent" | null;
  error: string | null;
}
```

### Store Methods

| Method | Description |
|--------|-------------|
| `validateAndSetTenant(slug)` | Validate tenant access |
| `startSimulation(prompt, persona?)` | Start new simulation |
| `pauseSimulation()` | Pause active simulation |
| `resumeSimulation()` | Resume paused simulation |
| `stopSimulation()` | Stop and save simulation |
| `clearSimulation()` | Reset all state |
| `disconnectTenant()` | Clear tenant selection |

---

## Error Handling

| Error | User Message | Recovery |
|-------|--------------|----------|
| Tenant not found | "Tenant not found" | Re-enter slug |
| Permission denied | "You don't have permission to access this tenant" | Contact admin |
| LLM rate limit | "Rate limit exceeded. Simulation paused." | Auto-resume after delay |
| SSE disconnected | "Connection lost. Reconnecting..." | Auto-reconnect with retry |
| Invalid response | "Failed to generate response. Retrying..." | Retry up to 3 times |

---

## Security Considerations

1. **Authentication**: All endpoints require valid user session
2. **Authorization**: User must have `chat:test` permission for the tenant
3. **Rate Limiting**: Simulations limited to 1 concurrent per user, 10 per hour per tenant
4. **Data Isolation**: Transcripts scoped to tenant, only visible to users with access
5. **Input Validation**: Initial prompt and persona hint sanitized and length-limited

---

## Performance Considerations

1. **SSE Connection**: Single connection per simulation, auto-cleanup on completion
2. **Memory**: Simulation state held in memory during run, persisted only at end
3. **LLM Calls**: Two calls per turn (human + agent), consider caching/batching
4. **Timeout**: Individual LLM calls timeout at 30s, simulation timeout at 10 minutes
5. **Cleanup**: Orphaned sessions cleaned up after 15 minutes of inactivity

---

## Future Enhancements

1. **Batch Simulations**: Run multiple scenarios automatically
2. **Scenario Library**: Pre-defined test scenarios per vertical
3. **Comparison View**: Compare agent versions side-by-side
4. **Analytics**: Track simulation outcomes, identify weak points
5. **Export**: Export transcripts as PDF/JSON for review

---

# Addendum: Conversation History Dropdown

## Overview

Add a dropdown to the Agent Simulation page that displays the **last 50 conversations** combining:
- **Simulation transcripts** (from `agent_simulation_transcripts` table)
- **Real chat sessions** (from `agent_sessions` table)

Users can select any past conversation to view its full transcript with color-coded messages.

## Permission Model

The existing permissions control visibility:

| Data Source | Table | Permission | Description |
|-------------|-------|------------|-------------|
| Simulations | `agent_simulation_transcripts` | `chat:test` | Users who can run simulations |
| Real Chats | `agent_sessions` | `chat:logs` | Users with analyst/logs access |

The dropdown dynamically shows:
- **Only simulations** if user has `chat:test` only
- **Only real chats** if user has `chat:logs` only
- **Both combined** if user has both permissions
- **Empty** if user has neither permission

---

## API Design

### GET `/api/v1/agent/:slug/conversations`

Unified endpoint that queries both tables and returns a merged, sorted list.

**Query Parameters:**
| Parameter | Default | Max | Description |
|-----------|---------|-----|-------------|
| `limit` | 50 | 50 | Maximum conversations to return |

**Backend Logic:**
1. Check user permissions (`chat:test`, `chat:logs`)
2. If `chat:test`: Query `agent_simulation_transcripts` (limit 50)
3. If `chat:logs`: Query `agent_sessions` (limit 50)
4. Merge results, sort by `created_at` DESC
5. Return top 50

**Response:**
```json
{
  "conversations": [
    {
      "id": "sim-550e8400-e29b-41d4-a716-446655440000",
      "type": "simulation",
      "summary": "Water damage in basement after storm...",
      "message_count": 24,
      "created_at": "2026-01-20T10:30:00Z",
      "end_reason": "llm_detected",
      "persona_hint": "frustrated homeowner"
    },
    {
      "id": "sess-7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "type": "chat",
      "summary": "John Smith - water_damage - resolved",
      "message_count": 15,
      "created_at": "2026-01-20T09:15:00Z",
      "phase": "resolved",
      "audience_type": "external",
      "customer_name": "John Smith"
    }
  ],
  "permissions": {
    "can_view_simulations": true,
    "can_view_chats": true
  },
  "total": 50
}
```

### GET `/api/v1/agent/:slug/conversations/:id`

Get full conversation details. Works for both types by checking prefix:
- `sim-*` → Query `agent_simulation_transcripts`
- `sess-*` → Query `agent_sessions`

**Response (Simulation):**
```json
{
  "id": "sim-550e8400-e29b-41d4-a716-446655440000",
  "type": "simulation",
  "initial_prompt": "I just noticed water damage in my basement after the storm last night.",
  "persona_hint": "frustrated homeowner",
  "total_turns": 12,
  "end_reason": "llm_detected",
  "messages": [
    {
      "role": "human_sim",
      "content": "Hi, I just noticed water damage in my basement after the storm last night.",
      "turn_num": 1,
      "timestamp": "2026-01-20T10:30:15Z"
    },
    {
      "role": "agent",
      "content": "I'm sorry to hear about the water damage. Let me help you get this addressed quickly...",
      "turn_num": 1,
      "timestamp": "2026-01-20T10:30:18Z",
      "metadata": {
        "intent": "water_damage",
        "phase": "triage",
        "urgency": 2
      }
    }
  ],
  "created_at": "2026-01-20T10:30:00Z"
}
```

**Response (Chat):**
```json
{
  "id": "sess-7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "type": "chat",
  "audience_type": "external",
  "customer_name": "John Smith",
  "customer_phone": "555-1234",
  "phase": "resolved",
  "current_intent": "water_damage",
  "messages": [
    {
      "role": "user",
      "content": "I need help with water damage",
      "timestamp": "2026-01-20T09:15:00Z"
    },
    {
      "role": "assistant",
      "content": "I'm sorry to hear about the water damage...",
      "timestamp": "2026-01-20T09:15:03Z"
    }
  ],
  "created_at": "2026-01-20T09:15:00Z"
}
```

---

## UI Design

### Dropdown Button Location

```
+------------------------------------------------------------------+
| [Flask Icon] Agent Simulation                                     |
|   [Tenant: ACME Corp] [📜 History (50) ▼]  [Disconnect]  [Clear] |
+------------------------------------------------------------------+
```

The "History" button appears after tenant selection, showing conversation count.

### Dropdown Content

```
+------------------------------------------------------+
| 📜 Recent Conversations                               |
+------------------------------------------------------+
| 🧪 Water damage in basement after...        24 msgs  |  <- Purple/violet badge
|    Simulation • Jan 20, 10:30 AM                     |
+------------------------------------------------------+
| 💬 John Smith - water_damage                 15 msgs |  <- Green badge
|    External Chat • Jan 20, 9:15 AM                   |
+------------------------------------------------------+
| 🧪 Insurance claim inquiry about...          18 msgs |
|    Simulation • Jan 19, 3:45 PM                      |
+------------------------------------------------------+
| 💬 Sarah Johnson - billing_question          8 msgs  |
|    Internal Chat • Jan 19, 2:00 PM                   |
+------------------------------------------------------+
| ... (scrollable, max 50)                             |
+------------------------------------------------------+
```

### Badge Colors

| Type | Badge | Icon |
|------|-------|------|
| Simulation | `bg-purple-100 text-purple-700` | 🧪 FlaskConical |
| External Chat | `bg-green-100 text-green-700` | 💬 MessageSquare |
| Internal Chat | `bg-blue-100 text-blue-700` | 💬 MessageSquare |

### Viewing Mode

When a conversation is selected from dropdown:

1. **Hide** the "Configure Simulation" section
2. **Show** a header bar with conversation info:
   ```
   +------------------------------------------------------------------+
   | ← Back to Simulation                                              |
   | Viewing: Simulation from Jan 20, 2026 at 10:30 AM                |
   | Initial prompt: "Water damage in basement after storm..."        |
   | Turns: 12 | End reason: Natural conclusion                       |
   +------------------------------------------------------------------+
   ```
3. **Display** messages in the same message container (read-only)
4. Messages use same color coding:
   - Human (blue) - works for both `human_sim` and `user` roles
   - Agent (teal) - works for both `agent` and `assistant` roles

---

## State Management Changes

### New State Fields (agentSimulation.ts)

```typescript
// Add to LocalState class
conversations: ConversationSummary[] = [];
selectedConversation: ConversationDetail | null = null;
viewMode: "simulation" | "viewing" = "simulation";
isLoadingHistory: boolean = false;

// New interfaces
interface ConversationSummary {
  id: string;
  type: "simulation" | "chat";
  summary: string;
  messageCount: number;
  createdAt: Date;
  // Simulation-specific
  endReason?: string;
  personaHint?: string;
  // Chat-specific
  phase?: string;
  audienceType?: string;
  customerName?: string;
}

interface ConversationDetail {
  id: string;
  type: "simulation" | "chat";
  messages: UnifiedMessage[];
  createdAt: Date;
  // ... other fields based on type
}

interface UnifiedMessage {
  role: "human" | "agent";  // Normalized
  content: string;
  timestamp: Date;
  turnNum?: number;
  metadata?: SimulationMetadata;
}
```

### New Methods

```typescript
fetchConversationHistory(): Promise<void>
// Calls GET /api/v1/agent/:slug/conversations
// Populates state.conversations

selectConversation(id: string): Promise<void>
// Calls GET /api/v1/agent/:slug/conversations/:id
// Sets state.selectedConversation and state.viewMode = "viewing"

closeConversationView(): void
// Clears state.selectedConversation
// Sets state.viewMode = "simulation"
```

---

## Files to Modify

### Backend

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/handlers.go` | Add `HandleGetConversations`, `HandleGetConversation` |
| `server/router/api/v1/v1.go` | Register new routes |

### Frontend

| File | Changes |
|------|---------|
| `web/src/store/v2/agentSimulation.ts` | Add history state, fetch methods |
| `web/src/pages/AgentSimulation.tsx` | Add dropdown UI, viewing mode |
| `web/src/locales/en.json` | Add translation keys |

---

## Translation Keys

```json
{
  "agent-simulation": {
    "history": "History",
    "recent-conversations": "Recent Conversations",
    "viewing-simulation": "Viewing: Simulation from {{date}}",
    "viewing-chat": "Viewing: Chat with {{name}}",
    "back-to-simulation": "Back to Simulation",
    "no-conversations": "No conversations yet",
    "conversation-type-simulation": "Simulation",
    "conversation-type-external": "External Chat",
    "conversation-type-internal": "Internal Chat",
    "messages-count": "{{count}} msgs"
  }
}
```

---

## Implementation Sequence

1. **Backend API**: Add conversation list and detail handlers
2. **Backend Routes**: Register in v1.go
3. **Frontend Store**: Add state and methods to agentSimulation.ts
4. **Frontend UI**: Add dropdown component and viewing mode
5. **Translations**: Add new keys to en.json

---

## Verification

1. Build: `task build`
2. Dev server: `task dev`
3. Log in as user with `chat:test` permission
4. Navigate to Agent Simulation
5. Select a tenant that has past simulations/chats
6. Click "History" dropdown
7. Verify:
   - Simulations shown (if user has `chat:test`)
   - Real chats shown (if user has `chat:logs`)
   - Both shown if user has both permissions
   - Sorted by date descending
   - Max 50 conversations
8. Click a conversation to view
9. Verify messages display with correct colors
10. Click "Back to Simulation" to return

---

# Addendum: Simulation Refinements v3

## Overview

This addendum documents four improvements implemented to refine the agent simulation and chat system:

| # | Change | Impact |
|---|--------|--------|
| 1 | Plain text responses (no markdown) | Backend: service.go |
| 2 | Remove Chat Session Logs from Agent Admin | Frontend: AgentAdmin.tsx, agentAdmin.ts |
| 3 | SCRIPT.MD Integration | New file type for conversation flow |
| 4 | Transcript Benchmark Analysis | New feature: Backend + Frontend |

---

## 1. Plain Text Responses (No Markdown)

### Problem
The LLM sometimes produces markdown formatting (`**bold**`, `*italic*`, `- bullets`) which appears as raw characters in the chat UI since we render with `whitespace-pre-wrap` only.

### Solution
Added explicit instruction to the system prompt in `service.go`:

```go
sb.WriteString("\nRESPONSE FORMAT: Use plain text only. NO markdown formatting (no **, no *, no # headers, no bullet symbols). ")
sb.WriteString("Use natural sentence structure. For lists, use numbered sentences or comma-separated items.\n\n")
```

### File Changed
- `server/router/api/v1/agent/service.go`

---

## 2. Remove Chat Session Logs from Agent Admin

### Rationale
The feature was redundant. Conversation history is now accessible via the Agent Simulation page dropdown, making the dedicated section in Agent Admin unnecessary.

### Changes Made

**Frontend Files Modified:**
| File | Changes |
|------|---------|
| `web/src/pages/AgentAdmin.tsx` | Removed `SessionLogsSection` component entirely |
| `web/src/store/v2/agentAdmin.ts` | Removed `chatSessions`, `selectedSession`, `isLoadingSessions` state and related methods |

**Backend (Unchanged):**
- `HandleListSessions` and `HandleGetSession` handlers retained for conversation history dropdown

---

## 3. SCRIPT.MD - Conversation Flow Guide

### Overview
SCRIPT.MD is a **tenant-level** file (one per tenant, not per-audience) that defines the expected conversation structure and flow for the AI agent.

### Architecture

```
Per-Tenant:
└── SCRIPT.MD (conversation structure - same for all audiences)

Per-Audience (external/internal):
├── KB.MD (knowledge content differs by audience)
└── POLICY.MD (rules/tone differs by audience)
```

### Database Schema

```sql
CREATE TABLE agent_tenant_script (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id)
);
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/agent/:slug/script` | Get current script |
| POST | `/api/v1/agent/:slug/script` | Upload/replace script (multipart form) |
| DELETE | `/api/v1/agent/:slug/script` | Delete script |

### Integration into System Prompt

When a script exists, it is condensed and added to the system prompt:

```go
// In buildSystemPrompt()
if config.Script != nil && config.Script.Summary != "" {
    sb.WriteString("\nCONVERSATION FLOW GUIDE:\n")
    sb.WriteString("Follow this general structure (adapt naturally to the conversation):\n")
    sb.WriteString(config.Script.Summary)
    sb.WriteString("\n\n")
}
```

### Parser

The `ParseScript` function in `parser.go` parses SCRIPT.MD into sections:

```go
type ParsedScript struct {
    Sections []ScriptSection
    Summary  string  // Condensed for system prompt
    Raw      string  // Original content
}

type ScriptSection struct {
    Name      string
    Questions []string
    Required  bool
}
```

### UI in Agent Admin

A new section appears in Agent Admin after selecting a tenant:

```
+------------------------------------------------------------------+
| Conversation Flow Guide (SCRIPT.MD)                    [Configured] |
| Defines the structure and flow of conversations                    |
+------------------------------------------------------------------+
| SCRIPT.MD                                                          |
| Imported: Jan 20, 2026, 10:30 AM                                   |
| Version: 1 • Hash: a1b2c3d4e5f6...                                 |
|                                                                     |
| [Preview] [Delete]                                                  |
|                                        [Replace Script]             |
+------------------------------------------------------------------+
```

### Files Changed

| File | Changes |
|------|---------|
| `store/agent.go` | Added `AgentTenantScript`, `FindAgentTenantScript` types |
| `store/driver.go` | Added interface methods |
| `store/db/sqlite/agent.go` | Added CRUD operations |
| `server/router/api/v1/agent/parser.go` | Added `ParseScript` function |
| `server/router/api/v1/agent/service.go` | Integrated script into system prompt |
| `server/router/api/v1/agent/handlers.go` | Added script handlers |
| `server/router/api/v1/v1.go` | Registered script routes |
| `web/src/store/v2/agentAdmin.ts` | Added script state and methods |
| `web/src/pages/AgentAdmin.tsx` | Added `ScriptSection` component |

---

## 4. Transcript Benchmark Analysis

### Overview
A feature to analyze conversation transcripts (simulations or real chats) against KB.MD, POLICY.MD, and SCRIPT.MD to evaluate how well the agent performed.

### Scoring Rubric

| Category | Max Points | What to Check | Source |
|----------|------------|---------------|--------|
| Intent Recognition | 15 | Did agent correctly identify customer intent? | POLICY.MD intents |
| Service Alignment | 15 | Did agent offer appropriate services? | KB.MD services |
| Policy Compliance | 20 | Did agent follow rules, avoid exclusions? | POLICY.MD rules, KB.MD exclusions |
| Conversation Flow | 20 | Did agent follow expected conversation structure? | SCRIPT.MD |
| Information Gathering | 15 | Did agent collect required info (name, phone, address)? | SCRIPT.MD |
| Tone & Resolution | 15 | Was tone appropriate? Proper conclusion? | POLICY.MD identity |
| **Total** | **100** | | |

### Grading Scale

| Score | Grade | Meaning |
|-------|-------|---------|
| 90-100 | A | Excellent - follows all best practices |
| 80-89 | B | Good - minor improvements possible |
| 70-79 | C | Acceptable - some issues to address |
| 60-69 | D | Needs Work - significant gaps |
| 0-59 | F | Failed - major compliance issues |

### API Endpoint

**POST `/api/v1/agent/:slug/analyze`**

Analyze a transcript against benchmarks.

**Request:**
```json
{
  "conversation_id": "sim-550e8400-e29b-41d4-a716-446655440000",
  "include_suggestions": true
}
```

**Response:**
```json
{
  "id": "ana-7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "conversation_id": "sim-550e8400-e29b-41d4-a716-446655440000",
  "conversation_type": "simulation",
  "score": 78,
  "grade": "B",
  "breakdown": {
    "intent_recognition": { "score": 14, "max": 15, "notes": "Correctly identified water_damage" },
    "service_alignment": { "score": 15, "max": 15, "notes": "Offered appropriate services" },
    "policy_compliance": { "score": 16, "max": 20, "notes": "Missed rule about insurance verification" },
    "conversation_flow": { "score": 15, "max": 20, "notes": "Skipped scheduling section" },
    "information_gathering": { "score": 10, "max": 15, "notes": "Missing phone number" },
    "tone_resolution": { "score": 8, "max": 15, "notes": "Resolution was abrupt" }
  },
  "issues": [
    { "severity": "warning", "turn": 5, "message": "Did not ask for customer phone number" },
    { "severity": "info", "turn": 3, "message": "Could acknowledge frustration more explicitly" }
  ],
  "suggestions": [
    "Consider asking for contact information earlier in conversation",
    "Add empathetic acknowledgment when customer expresses urgency"
  ],
  "benchmark_version": "2026-01-20",
  "created_at": "2026-01-20T11:45:00Z"
}
```

### Database Schema

```sql
CREATE TABLE agent_analysis_results (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    conversation_id TEXT NOT NULL,
    conversation_type TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    score INTEGER NOT NULL,
    grade TEXT NOT NULL,
    breakdown TEXT NOT NULL,       -- JSON
    issues TEXT NOT NULL,          -- JSON array
    suggestions TEXT,              -- JSON array (optional)
    benchmark_version TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id)
);

CREATE INDEX idx_analysis_tenant ON agent_analysis_results(tenant_id);
CREATE INDEX idx_analysis_conversation ON agent_analysis_results(conversation_id);
```

### UI in Agent Simulation

When viewing a conversation, an "Analyze" button appears:

```
+------------------------------------------------------------------+
| ← Back to Simulation                              [📊 Analyze]    |
| Viewing: Simulation from Jan 20, 2026                            |
+------------------------------------------------------------------+
```

Clicking "Analyze" opens a modal with results:

```
+------------------------------------------------------------------+
| Transcript Analysis                                         [X]   |
+------------------------------------------------------------------+
| Overall Score: 78/100                                             |
| Grade: B                                                          |
+------------------------------------------------------------------+
| Score Breakdown:                                                  |
| ████████████████░░ Intent Recognition       14/15                |
| █████████████████░ Service Alignment        15/15                |
| ████████████████░░░░ Policy Compliance      16/20                |
| ███████████████░░░░░ Conversation Flow      15/20                |
| ██████████░░░░░ Information Gathering       10/15                |
| ████████░░░░░░░ Tone & Resolution            8/15                |
+------------------------------------------------------------------+
| Issues Found:                                                     |
| ⚠️ Turn 5: Did not ask for customer phone number                  |
| ℹ️ Turn 3: Could acknowledge frustration more explicitly          |
+------------------------------------------------------------------+
| Suggestions:                                                      |
| ✓ Consider asking for contact information earlier                 |
| ✓ Add empathetic acknowledgment when customer expresses urgency   |
+------------------------------------------------------------------+
| Benchmark Version: 2026-01-20                                     |
+------------------------------------------------------------------+
```

### Files Changed

| File | Changes |
|------|---------|
| `store/agent.go` | Added `AgentAnalysisResult`, `AnalysisBreakdown`, `CategoryScore`, `AnalysisIssue` types |
| `store/driver.go` | Added interface methods |
| `store/db/sqlite/agent.go` | Added CRUD operations |
| `server/router/api/v1/agent/analysis.go` | **New file** - Analysis service logic |
| `server/router/api/v1/agent/handlers.go` | Added `HandleAnalyzeTranscript`, `HandleGetAnalysisHistory` |
| `server/router/api/v1/v1.go` | Registered analysis routes |
| `web/src/store/v2/agentSimulation.ts` | Added analysis state and methods |
| `web/src/pages/AgentSimulation.tsx` | Added Analyze button and results modal |
| `web/src/locales/en.json` | Added translation keys |

---

## Translation Keys Added

```json
{
  "agent-admin": {
    "script-title": "Conversation Flow Guide (SCRIPT.MD)",
    "script-description": "Defines the structure and flow of conversations for the AI agent",
    "script-uploaded": "Script uploaded successfully",
    "script-deleted": "Script deleted successfully",
    "script-delete-confirm": "Are you sure you want to delete the script?",
    "no-script": "No script configured. Upload SCRIPT.MD to guide conversation flow.",
    "upload-script": "Upload SCRIPT.MD",
    "replace-script": "Replace Script",
    "preview": "Preview",
    "configured": "Configured"
  },
  "agent-simulation": {
    "analyze": "Analyze",
    "analysis-title": "Transcript Analysis",
    "grade": "Grade",
    "breakdown": "Score Breakdown",
    "issues-found": "Issues Found",
    "suggestions": "Suggestions for Improvement",
    "benchmark-version": "Benchmark Version"
  },
  "common": {
    "hide": "Hide"
  }
}
```

---

## Verification

### Plain Text Responses
1. Run a simulation
2. Verify no markdown characters (`**`, `*`, `#`, `-` bullets) in agent responses
3. Verify responses read naturally as plain sentences

### Chat Session Logs Removal
1. Open Agent Admin
2. Verify "Chat Session Logs" section is completely gone
3. Verify conversation history still works in Agent Simulation page

### SCRIPT.MD Support
1. Navigate to Agent Admin
2. Select a tenant
3. Upload a SCRIPT.MD file
4. Verify upload success and preview works
5. Run simulation
6. Verify agent follows conversation structure

### Transcript Analysis
1. Run a simulation to completion
2. View the simulation in history dropdown
3. Click "Analyze" button
4. Verify:
   - Score and grade displayed
   - Breakdown shows all 6 categories with progress bars
   - Issues show severity icons and turn numbers
   - Suggestions are actionable
5. Close modal and verify state resets

---

# Addendum: Troubleshooting Guide

## Common Issues and Solutions

### Analyze Button Not Visible

**Symptom:** After selecting a conversation from history, the "Analyze" button doesn't appear.

**Possible Causes:**

1. **Browser cache serving old JavaScript**
   - Solution: Hard refresh with `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac)
   - Alternative: Open DevTools → Network tab → Check "Disable cache" → Reload

2. **Dev server not restarted after build**
   - Solution: Stop dev server, run `task build`, then `task dev`

3. **Not in viewing mode**
   - The Analyze button only appears when viewing a **historical conversation**
   - It does NOT appear during an active simulation
   - Steps to reach viewing mode:
     1. Connect to a tenant
     2. Click "History" dropdown button
     3. Select a past simulation or chat
     4. The "Back to Simulation" and "Analyze" buttons appear in the header

**Verification:**
```
Expected UI in viewing mode:
+------------------------------------------------------------------+
| ← Back to Simulation                              [📊 Analyze]    |
| Viewing: Simulation from Jan 20, 2026                            |
| Initial prompt: "Water damage in basement..."                     |
| 24 messages | Natural conclusion                                  |
+------------------------------------------------------------------+
```

### Script Upload Fails with "no such table"

**Symptom:** Error "SQL logic error: no such table: agent_tenant_scripts"

**Cause:** Database migration not applied.

**Solution:**
1. Ensure migration file exists: `store/migration/sqlite/0.25/09__agent_script_analysis.sql`
2. Restart the server - migrations auto-apply on startup
3. Or manually apply: `sqlite3 build/data/memos_dev.db < store/migration/sqlite/0.25/09__agent_script_analysis.sql`

### Analysis Returns Empty/Error

**Symptom:** Clicking Analyze shows error or empty result.

**Possible Causes:**

1. **Missing OPENROUTER_API_KEY**
   - Solution: Set environment variable before starting server
   ```bash
   export OPENROUTER_API_KEY=your-key
   task dev
   ```

2. **Conversation too short**
   - Minimum 2 messages required for analysis
   - Run a longer simulation

3. **LLM rate limit**
   - Wait and retry
   - Check OpenRouter dashboard for quota

### History Dropdown Empty

**Symptom:** "History" button shows (0) or dropdown is empty.

**Possible Causes:**

1. **No conversations exist yet**
   - Run a simulation first
   - Or have real chat sessions

2. **Permission issue**
   - `chat:test` permission needed to see simulations
   - `chat:logs` permission needed to see real chats
   - Check user permissions in Agent Admin

3. **API error**
   - Check browser DevTools console for errors
   - Check server logs

---

## Development Checklist

After making changes to the simulation/analysis feature, verify:

- [ ] Backend builds: `go build -o build/memos ./bin/memos/main.go`
- [ ] Frontend builds: `cd web && npm run build`
- [ ] Dev server restarted: `task dev`
- [ ] Browser hard-refreshed: `Ctrl+Shift+R`
- [ ] Database migrations applied (check server startup logs)
- [ ] Translation keys exist in `web/src/locales/en.json`
- [ ] Store methods exported in return object
- [ ] Routes registered in `v1.go`

---

## Debug Commands

**Check if table exists:**
```bash
sqlite3 build/data/memos_dev.db ".tables" | grep agent
```

**View recent migrations:**
```bash
sqlite3 build/data/memos_dev.db "SELECT * FROM migration_history ORDER BY created_ts DESC LIMIT 5;"
```

**Test API endpoint directly:**
```bash
# Get conversations
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8081/api/v1/agent/YOUR_SLUG/conversations

# Analyze a conversation
curl -X POST -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"conversation_id":"sim-xxx","include_suggestions":true}' \
  http://localhost:8081/api/v1/agent/YOUR_SLUG/analyze
```

**Check frontend chunk includes analyze code:**
```bash
grep -l "handleAnalyze\|BarChart2" web/dist/assets/*.js
```

---

# Addendum: Agent Self-Improvement Based on Analysis

**Date:** 2026-01-20
**Status:** Proposal (Not Yet Implemented)

## Current State

### Analysis Storage: YES

Analysis results **are** stored in the database. When you click "Analyze" on a conversation:

1. The LLM evaluates the transcript against KB.MD, POLICY.MD, and SCRIPT.MD
2. Results are stored in `agent_analysis_results` table:

```sql
-- Existing table
CREATE TABLE agent_analysis_results (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    conversation_id TEXT NOT NULL,
    conversation_type TEXT NOT NULL,  -- 'simulation' or 'chat'
    user_id INTEGER NOT NULL,
    score INTEGER NOT NULL,           -- 0-100
    grade TEXT NOT NULL,              -- A/B/C/D/F
    breakdown TEXT NOT NULL,          -- JSON: category scores
    issues TEXT NOT NULL,             -- JSON: array of issues
    suggestions TEXT,                 -- JSON: array of improvements
    benchmark_version TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

3. Results can be viewed in the modal after analysis
4. Historical analysis is accessible via `GET /api/v1/agent/:slug/analysis`

### Agent Self-Improvement: NOT YET IMPLEMENTED

Currently, analysis results are stored but **not used** to improve the agent. The agent's behavior is solely determined by:
- KB.MD (knowledge base)
- POLICY.MD (rules and identity)
- SCRIPT.MD (conversation flow)

**Gap:** There is no feedback loop from analysis results back to the agent's behavior.

---

## Proposed Solution: Agent Learning Memory

### Concept

Create a **tenant-level learning memory** that aggregates insights from analysis results and injects them into the system prompt as specific behavioral guidance.

### Architecture

```
Analysis Results (many)
        ↓
   Aggregation
        ↓
Agent Learning Memory (per tenant)
        ↓
   System Prompt Enhancement
        ↓
Improved Agent Behavior
```

### Data Model

**New Table: `agent_learning_memory`**

```sql
CREATE TABLE agent_learning_memory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE,

    -- Aggregated insights from analysis
    common_issues TEXT NOT NULL,         -- JSON: frequently occurring issues
    learned_behaviors TEXT NOT NULL,     -- JSON: specific behavioral guidance
    improvement_areas TEXT NOT NULL,     -- JSON: categories needing attention

    -- Metadata
    analysis_count INTEGER DEFAULT 0,    -- Number of analyses incorporated
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,

    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id) ON DELETE CASCADE
);
```

**Example `learned_behaviors` JSON:**

```json
[
  {
    "trigger": "when customer mentions urgency",
    "behavior": "Acknowledge their urgency explicitly before proceeding",
    "source": "Multiple analyses noted missing empathy",
    "added": "2026-01-20"
  },
  {
    "trigger": "information gathering phase",
    "behavior": "Ask for phone number within first 3 turns",
    "source": "5 analyses flagged missing contact info",
    "added": "2026-01-19"
  },
  {
    "trigger": "conversation closing",
    "behavior": "Summarize next steps before ending",
    "source": "3 analyses noted abrupt endings",
    "added": "2026-01-18"
  }
]
```

### Learning Process

#### Option A: Manual Curation (Recommended for v1)

Admin reviews analysis results and manually approves improvements:

1. View aggregated analysis trends in Agent Admin
2. See suggested behavioral improvements
3. Approve/reject/edit each suggestion
4. Approved items go into learning memory

**UI in Agent Admin:**

```
+------------------------------------------------------------------+
| Learning Insights                                    [12 analyses] |
| Based on transcript analysis results                              |
+------------------------------------------------------------------+
| Suggested Improvements:                                           |
|                                                                   |
| ⚠️ Information Gathering (flagged 5 times)                        |
| "Ask for customer phone number within first 3 exchanges"          |
| [Approve] [Edit] [Dismiss]                                        |
|                                                                   |
| ⚠️ Tone & Resolution (flagged 3 times)                            |
| "Summarize agreed next steps before closing conversation"         |
| [Approve] [Edit] [Dismiss]                                        |
|                                                                   |
| ℹ️ Intent Recognition (flagged 2 times)                           |
| "Confirm intent with customer if initial message is ambiguous"    |
| [Approve] [Edit] [Dismiss]                                        |
+------------------------------------------------------------------+
| Active Learned Behaviors: 4                    [View All] [Clear] |
+------------------------------------------------------------------+
```

#### Option B: Automatic Learning (Future Enhancement)

After configurable threshold (e.g., 5 similar issues), automatically add to learning memory:

1. After each analysis, check for patterns
2. If issue appears 5+ times with consistent suggestion, auto-add
3. Admin can still review/remove auto-learned behaviors
4. Requires higher confidence threshold to prevent drift

### Integration into System Prompt

Modify `buildSystemPrompt` in `service.go`:

```go
// After SCRIPT.MD section, before closing
if config.LearningMemory != nil && len(config.LearningMemory.LearnedBehaviors) > 0 {
    sb.WriteString("\nLEARNED IMPROVEMENTS:\n")
    sb.WriteString("Based on conversation analysis, apply these specific behaviors:\n\n")
    for _, behavior := range config.LearningMemory.LearnedBehaviors {
        sb.WriteString("• ")
        sb.WriteString(behavior.Trigger)
        sb.WriteString(": ")
        sb.WriteString(behavior.Behavior)
        sb.WriteString("\n")
    }
    sb.WriteString("\n")
}
```

**Example System Prompt Addition:**

```
LEARNED IMPROVEMENTS:
Based on conversation analysis, apply these specific behaviors:

• When customer mentions urgency: Acknowledge their urgency explicitly before proceeding
• During information gathering: Ask for phone number within first 3 turns
• Before closing conversation: Summarize agreed next steps
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/agent/:slug/learning` | Get learning memory and suggestions |
| POST | `/api/v1/agent/:slug/learning/approve` | Approve a suggested improvement |
| DELETE | `/api/v1/agent/:slug/learning/:id` | Remove a learned behavior |
| POST | `/api/v1/agent/:slug/learning/regenerate` | Re-aggregate from recent analyses |

### Safeguards

1. **Maximum behaviors:** Limit to 10 learned behaviors per tenant (prevent prompt bloat)
2. **Admin control:** All changes require admin approval in v1
3. **Audit trail:** Track when behaviors were added and from which analyses
4. **Rollback:** Ability to clear all learned behaviors and start fresh
5. **Versioning:** Learning memory has version number for tracking changes

---

## Implementation Plan

### Phase 1: Analysis Aggregation Dashboard (Backend + Frontend)

**Files to create/modify:**

| File | Changes |
|------|---------|
| `store/agent.go` | Add `AgentLearningMemory`, `LearnedBehavior`, `ImprovementSuggestion` types |
| `store/db/sqlite/agent.go` | Add CRUD for learning memory |
| `store/migration/sqlite/0.25/10__agent_learning_memory.sql` | New table |
| `server/router/api/v1/agent/learning.go` | New file: aggregation logic |
| `server/router/api/v1/agent/handlers.go` | Add learning handlers |
| `web/src/pages/AgentAdmin.tsx` | Add "Learning Insights" section |
| `web/src/store/v2/agentAdmin.ts` | Add learning state and methods |

### Phase 2: System Prompt Integration

**Files to modify:**

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/service.go` | Load learning memory in `LoadConfig`, add to `buildSystemPrompt` |

### Phase 3: Testing & Verification

1. Run 5+ simulations with a tenant
2. Analyze each simulation
3. View aggregated suggestions in Agent Admin
4. Approve 2-3 suggestions
5. Run new simulation
6. Verify agent follows learned behaviors
7. Analyze new simulation - score should improve

---

## Alternative Approaches Considered

### A. Direct KB/Policy Modification

**Concept:** Generate suggested edits to KB.MD or POLICY.MD files.

**Pros:**
- Changes persist in standard files
- Explicit, reviewable changes

**Cons:**
- Requires file re-parsing and re-upload
- More disruptive workflow
- Harder to roll back

**Verdict:** Not recommended for v1. Learning memory is lighter-weight.

### B. Per-Conversation Context

**Concept:** Store learned context per customer/session.

**Pros:**
- Highly personalized

**Cons:**
- Doesn't generalize across customers
- Requires session continuity
- More complex state management

**Verdict:** Could be future enhancement for VIP customers.

### C. Fine-Tuning

**Concept:** Fine-tune the LLM on good conversations.

**Pros:**
- Permanent behavioral change
- No prompt overhead

**Cons:**
- Requires significant data
- Model-specific
- Expensive and slow
- Hard to roll back

**Verdict:** Out of scope. Prompt-based learning is more practical.

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Score improvement | +10 points average | Compare analysis scores before/after learning |
| Issue recurrence | -50% | Same issues should appear less often |
| Admin engagement | 80% approval rate | Suggested improvements should be relevant |
| Prompt overhead | < 200 tokens | Learning section shouldn't bloat prompts |

---

## Timeline

| Phase | Scope | Priority |
|-------|-------|----------|
| Phase 1 | Aggregation dashboard | High |
| Phase 2 | System prompt integration | High |
| Phase 3 | Testing & tuning | High |
| Future | Automatic learning | Medium |
| Future | Per-conversation context | Low |

---

## Questions for Implementation

1. **Threshold for suggestions:** How many times should an issue appear before suggesting a behavior? (Proposed: 3)

2. **Behavior limit:** Maximum learned behaviors per tenant? (Proposed: 10)

3. **Expiration:** Should learned behaviors expire if the issue stops appearing? (Proposed: No expiration, manual removal)

4. **Audience scope:** Should learning memory be tenant-wide or per-audience? (Proposed: Tenant-wide, since most improvements apply to both)

---

## Summary

| Question | Answer |
|----------|--------|
| Is analysis stored in DB? | **Yes** - in `agent_analysis_results` table |
| Does agent improve from analysis? | **Not yet** - requires new feature |
| Proposed solution | Agent Learning Memory - aggregates insights, injects into prompt |
| Implementation approach | Manual curation (v1), then automatic learning (future) |

---
---

# Addendum: Conversation Control & Closure Logic

**Date:** 2026-01-20
**Status:** Proposal (Not Yet Implemented)
**Reference:** Analysis of distance.so/agent sophisticated patterns

## Executive Summary

This addendum proposes enhancements to tighten the chat agent's conversation control logic. The goal is to create a **general-purpose agent** (not tenant-specific) that:

1. Follows SCRIPT.MD protocol strictly
2. Adheres to KB.MD and POLICY.MD as hard constraints
3. Avoids endless conversations ("Always Be Closing")
4. Politely declines requests outside scope
5. Creates tickets with reference numbers for escalations

---

## Current State Analysis

### What Exists

| Feature | Status | Notes |
|---------|--------|-------|
| Phase tracking | Partial | Phases stored but not enforced |
| Intent classification | Good | LLM-based with urgency |
| KB/Policy context | Good | Injected into system prompt |
| SCRIPT.MD | Partial | Loaded as context, not executable |
| Session state | Good | Persistent tracking |
| Escalation | Missing | No triggers, no tickets |
| Closing logic | Missing | No proactive closure |

### Key Gaps

1. **Phases are passive descriptors**, not conversation drivers
2. **System prompt is advisory**, not enforcing constraints
3. **No closing protocol** - conversations can drift endlessly
4. **No escalation workflow** - no ticket creation
5. **SCRIPT.MD is context**, not a flowchart to execute

---

## Proposed Solution: Conversation Control Framework

### 1. Phase-Based State Machine

Replace passive phase tracking with an **explicit state machine**:

```
┌─────────┐     confidence > 0.7     ┌───────────┐
│  TRIAGE │ ────────────────────────▶│ GATHERING │
└─────────┘                          └───────────┘
     │                                     │
     │ emergency                           │ info complete
     ▼                                     ▼
┌─────────┐                          ┌───────────┐
│EMERGENCY│                          │ RESOLUTION│
└─────────┘                          └───────────┘
     │                                     │
     │ safety delivered                    │ resolved
     ▼                                     ▼
┌─────────┐     confirmation         ┌───────────┐
│ CLOSING │ ◀────────────────────────│  CLOSING  │
└─────────┘                          └───────────┘
     │                                     │
     │ explicit close                      │ explicit close
     ▼                                     ▼
┌─────────┐                          ┌───────────┐
│COMPLETED│                          │ COMPLETED │
└─────────┘                          └───────────┘

SPECIAL STATES:
┌─────────┐                          ┌───────────┐
│ESCALATED│ (requires ticket)        │ DECLINED  │ (out of scope)
└─────────┘                          └───────────┘
```

**Phase Definitions:**

| Phase | Purpose | Exit Condition |
|-------|---------|----------------|
| `triage` | Identify intent and urgency | Intent confidence > 0.7 |
| `gathering` | Collect required information | All required fields populated |
| `resolution` | Provide answer/service | Customer confirms understanding |
| `closing` | Confirm satisfaction, offer more help | Explicit "no" or closure keyword |
| `completed` | Terminal state | N/A |
| `escalated` | Handed to human + ticket | N/A |
| `declined` | Out of scope | N/A |

### 2. "Always Be Closing" Logic

Add explicit closing behavior to the agent:

**A. Turn-Based Closing Triggers:**

```go
const (
    MaxTurnsBeforeClose     = 15  // After 15 turns, start closing
    MaxTurnsAfterResolution = 3   // After resolution, max 3 more turns
    IdleTimeoutSeconds      = 300 // 5 min silence = soft close
)
```

**B. Closing Keywords Detection:**

```go
var closingKeywords = []string{
    "thank you", "thanks", "got it", "understand", "ok",
    "goodbye", "bye", "good night", "see you", "no",
    "nothing else", "that's all", "i'm good", "all set",
}
```

**C. Satisfaction Check Pattern:**

After resolution, inject into system prompt:
```
CLOSING PROTOCOL:
1. Summarize what was discussed/resolved
2. Ask: "Is there anything else I can help you with?"
3. If response is positive → re-enter GATHERING phase
4. If response contains closure keyword → transition to COMPLETED
5. If 3 turns without closure → gently prompt: "If you're all set, feel free to reach out anytime!"
```

**D. Escape Hatch (from distance.so/agent):**

If in `closing` phase and user sends revert keywords, return to `gathering`:
```go
var revertKeywords = []string{"wait", "actually", "one more", "also", "help", "?"}
```

### 3. Strict KB/Policy Enforcement

**A. Hard Constraint Injection in System Prompt:**

Add to `buildSystemPrompt()`:

```
CRITICAL CONSTRAINTS:
- You MUST ONLY offer services listed in "Services we offer" section
- You MUST NEVER offer services in "Services we DON'T provide" section
- If asked about something not in your knowledge base, say: "I don't have information about that. Would you like me to connect you with someone who can help?"
- If the request is clearly outside our service area, politely decline and suggest alternatives
```

**B. Response Validation (Post-LLM):**

After LLM generates response, validate:
1. Check if response mentions any excluded services → trigger warning
2. Check if response promises something outside KB → flag for review
3. Check if response follows the current phase protocol

**C. Grounding Directive:**

Add to system prompt:
```
GROUNDING RULE:
Only provide information that exists in the knowledge base above.
Do NOT make up prices, timelines, or service details.
If unsure, say "I'll need to check on that" and collect contact info.
```

### 4. Polite Decline Mechanism

**A. Out-of-Scope Detection:**

```go
type OutOfScopeReason string

const (
    OutOfScopeLocation    OutOfScopeReason = "location"
    OutOfScopeService     OutOfScopeReason = "service"
    OutOfScopeCapability  OutOfScopeReason = "capability"
)
```

**B. Decline Templates (from POLICY.MD or defaults):**

```yaml
decline_templates:
  location: |
    Thank you for reaching out! Unfortunately, we only service {{coverage_areas}}.
    For {{user_location}}, I'd recommend searching for certified providers in your area.
    Is there anything else I can help you with?

  service: |
    I appreciate you thinking of us! However, we don't provide {{requested_service}}.
    We specialize in {{our_services}}.
    Would you like information about any of our available services?

  capability: |
    That's a great question, but it's outside what I'm able to help with.
    Would you like me to create a support ticket so our team can follow up?
```

**C. Decline with Referral:**

When declining, check if exclusion has a `Referral` field:
```go
if exclusion.Referral != "" {
    response += fmt.Sprintf("\n\nYou might want to try: %s", exclusion.Referral)
}
```

### 5. Escalation with Ticket Creation

**A. Escalation Triggers:**

| Trigger | Detection | Action |
|---------|-----------|--------|
| Explicit request | Keywords: "speak to manager", "talk to human", "supervisor" | Immediate escalation |
| Frustration | Sentiment analysis + keywords: "frustrated", "angry", "unacceptable" | LLM verification + escalation |
| Complex issue | Intent confidence < 0.5 after 5 turns | Suggest escalation |
| Safety concern | Safety protocol triggered + unresolved | Auto-escalate |
| Out of scope + insistent | Declined + customer persists | Offer escalation |

**B. Two-Stage Verification (from distance.so/agent):**

```go
func (s *Service) shouldEscalate(ctx context.Context, message string, session *store.AgentSession) (bool, string) {
    // Stage 1: Keyword pre-filter
    escalationKeywords := []string{
        "speak to manager", "supervisor", "human", "real person",
        "complain", "complaint", "lawsuit", "lawyer", "legal",
    }
    hasKeyword := containsAny(message, escalationKeywords)
    if !hasKeyword {
        return false, ""
    }

    // Stage 2: LLM intent verification
    // Avoids false positives like "I'll speak to my lawyer about the damage"
    isEscalation, reason := s.verifyEscalationIntent(ctx, message, session.Messages)
    return isEscalation, reason
}
```

**C. Ticket Integration:**

When escalation is confirmed:

```go
func (s *Service) handleEscalation(ctx context.Context, session *store.AgentSession, reason string) (*store.Ticket, error) {
    // 1. Create ticket with conversation context
    ticket := &store.Ticket{
        Title:       fmt.Sprintf("Escalation: %s - %s", session.CustomerName, reason),
        Description: s.formatEscalationContext(session),
        Priority:    s.mapUrgencyToPriority(session.UrgencyLevel),
        Status:      "open",
        CreatedBy:   "agent-system",
    }

    createdTicket, err := s.store.CreateTicket(ctx, ticket)
    if err != nil {
        return nil, err
    }

    // 2. Update session with ticket reference
    session.EscalationTicketID = &createdTicket.ID
    session.Phase = "escalated"
    session.IsCompleted = true
    session.CompletionReason = "escalated_" + reason

    return createdTicket, nil
}
```

**D. Response with Ticket Number:**

```
I understand this needs further attention. I've created a support ticket for you.

Your reference number is: #{{ticket_id}}

Our team will review this and follow up within {{sla_time}}.
Is there anything else I can help you with in the meantime?
```

### 6. SCRIPT.MD Execution (Enhanced)

**Current:** SCRIPT.MD is injected as context
**Proposed:** SCRIPT.MD becomes a soft execution guide

**A. Section Tracking:**

```go
type ScriptProgress struct {
    CurrentSection  string   // e.g., "gathering_damage_details"
    CompletedFields []string // Fields collected so far
    PendingFields   []string // Fields still needed
}
```

**B. Field Collection Validation:**

Parse SCRIPT.MD sections to extract required fields:
```
SECTION 1: RESTORATION SERVICE DETAILS
Required: damage_type, timing, extent, source_status
Optional: insurance_status, safety_concerns
```

**C. Adaptive Prompting:**

Inject into system prompt based on missing fields:
```
INFORMATION STILL NEEDED:
- Customer phone number (required)
- Damage extent (required)
- Insurance status (optional but helpful)

SCRIPT GUIDANCE:
Focus on collecting the required information naturally.
Don't ask all questions at once - pace the conversation.
```

---

## Implementation Plan

### Phase 1: Core Control Logic

**Files to modify:**

| File | Changes |
|------|---------|
| `store/agent.go` | Add `ScriptProgress`, `EscalationTicketID` to session |
| `service.go` | Add `processPhaseTransition()`, `shouldEscalate()`, `shouldClose()` |
| `service.go` | Update `buildSystemPrompt()` with constraints and closing logic |

### Phase 2: Ticket Integration

**Files to modify:**

| File | Changes |
|------|---------|
| `handlers.go` | Add `handleEscalation()` with ticket creation |
| `service.go` | Add `formatEscalationContext()` for ticket description |
| `store/agent.go` | Add `EscalationTicketID *int32` to AgentSession |

### Phase 3: Response Validation

**New file:** `validation.go`

| Function | Purpose |
|----------|---------|
| `validateResponse()` | Check response against KB constraints |
| `detectExclusionViolation()` | Flag if excluded services mentioned |
| `checkGrounding()` | Verify claims exist in KB |

### Phase 4: SCRIPT.MD Enhancement

**Files to modify:**

| File | Changes |
|------|---------|
| `parser.go` | Add `ParseScriptSections()` to extract required fields |
| `service.go` | Add `updateScriptProgress()` after each turn |
| `service.go` | Inject pending fields into system prompt |

---

## System Prompt Changes

**Before (current):**
```
Respond naturally and helpfully. Be concise but complete.
If this is an emergency, lead with safety instructions...
```

**After (proposed):**
```
RESPONSE PROTOCOL:
1. Stay strictly within the knowledge base provided
2. If information is not in KB, say "I'll need to check on that" and collect contact info
3. Never offer services listed in "DON'T provide" section
4. Follow the conversation flow guide (SCRIPT.MD)
5. Always work toward resolution and closure

CLOSING PROTOCOL:
- After providing information, ask: "Does that answer your question?"
- After resolving, ask: "Is there anything else I can help with today?"
- If customer says "no" or uses closing keywords, thank them and close
- Keep responses concise - aim for resolution, not extended dialogue

ESCALATION PROTOCOL:
- If customer requests human/manager, create ticket and provide reference number
- If issue is outside your capability, offer to escalate
- Never argue with frustrated customers - acknowledge and escalate

{{#if pendingFields}}
INFORMATION TO COLLECT:
{{#each pendingFields}}- {{this}}
{{/each}}
{{/if}}
```

---

## Verification

### Test Scenarios

| Scenario | Expected Behavior |
|----------|-------------------|
| Customer asks for excluded service | Polite decline with referral if available |
| Customer asks about unknown topic | "I don't have that information" + offer to escalate |
| Customer says "speak to manager" | Create ticket, provide reference number |
| Customer says "thank you, bye" | Close conversation gracefully |
| 15 turns without resolution | Start closing sequence |
| Customer in uncovered location | Decline with alternative suggestion |
| All required info collected | Transition to resolution phase |

### Metrics to Track

| Metric | Target |
|--------|--------|
| Average turns to resolution | < 10 |
| Escalation rate | < 5% |
| Out-of-scope decline rate | Track (no target) |
| Closure without explicit close | < 20% |
| Ticket creation on escalation | 100% |

---

## Design Principles (from distance.so/agent)

### Adapted Principles

1. **Deterministic Routing:** Phase transitions are explicit, not inferred
2. **Two-Stage Verification:** Keyword + LLM confirmation before escalation
3. **Fail-Safe Defaults:** If unsure, continue conversation (don't escalate prematurely)
4. **Template-First:** Use templates for decline/escalation before LLM generation
5. **Escape Hatch:** Allow returning from closing phase if customer has more questions
6. **Configuration-Driven:** All keywords/thresholds in config, not hardcoded

### Not Adapted (Too Tenant-Specific)

1. ~~Geofencing with LLM~~ - Coverage is in KB.MD, not code
2. ~~P0 urgency node~~ - Urgency is part of intent classification
3. ~~DAG orchestration~~ - Simpler phase machine is sufficient
4. ~~Topic-based ticket scoring~~ - Ticket on explicit escalation only

---

## Implementation Decisions

| Question | Decision |
|----------|----------|
| **Ticket SLA** | Configurable per tenant via POLICY.MD |
| **Ticket System** | Use existing Memos tickets (store/ticket.go) |
| **Enforcement** | Soft enforcement (constraints + logging, no blocking) |
| **Closing persistence** | 3 attempts, then soft close |
| **SCRIPT.MD fields** | Explicit marking in sections |

---

## Summary

| Gap | Solution |
|-----|----------|
| Endless conversations | Phase-based closing + turn limits |
| KB/Policy violations | System prompt constraints + validation |
| Out-of-scope requests | Polite decline templates with referrals |
| Escalation without tracking | Ticket creation with reference number |
| SCRIPT.MD as passive context | Section tracking with field validation |

**Key Principle:** The agent remains **general-purpose** - all tenant-specific behavior comes from KB.MD, POLICY.MD, and SCRIPT.MD files, not from code changes.

---
---

# Addendum: Agent Self-Improvement Implementation Progress

**Date:** 2026-01-20
**Status:** Implementation Complete (Pending Build Verification)

## Overview

This addendum documents the implementation progress of the Agent Learning Memory feature proposed in the "Agent Self-Improvement Based on Analysis" addendum.

---

## Implementation Status

| Component | Status | Notes |
|-----------|--------|-------|
| Database migration | ✅ Complete | `store/migration/sqlite/0.25/10__agent_learning_memory.sql` |
| Go types | ✅ Complete | `store/agent.go` |
| Interface methods | ✅ Complete | `store/driver.go` |
| SQLite CRUD | ✅ Complete | `store/db/sqlite/agent.go` |
| MySQL/PostgreSQL stubs | ✅ Complete | Returns `errNotImplemented` |
| Aggregation service | ✅ Complete | `server/router/api/v1/agent/learning.go` |
| HTTP handlers | ✅ Complete | `server/router/api/v1/agent/handlers.go` |
| Route registration | ✅ Complete | `server/router/api/v1/v1.go` |
| System prompt integration | ✅ Complete | `server/router/api/v1/agent/service.go` |
| Frontend store | ✅ Complete | `web/src/store/v2/agentAdmin.ts` |
| Learning Insights UI | ✅ Complete | `web/src/pages/AgentAdmin.tsx` |
| Translations | ✅ Complete | `web/src/locales/en.json` |
| Build verification | ⏳ Pending | Need to run `task build` |

---

## Files Created/Modified

### Database Migration

**File:** `store/migration/sqlite/0.25/10__agent_learning_memory.sql`

```sql
CREATE TABLE IF NOT EXISTS agent_learning_memory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE,
    common_issues TEXT NOT NULL DEFAULT '[]',
    learned_behaviors TEXT NOT NULL DEFAULT '[]',
    improvement_areas TEXT NOT NULL DEFAULT '[]',
    pending_suggestions TEXT NOT NULL DEFAULT '[]',
    analysis_count INTEGER DEFAULT 0,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenant(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_learning_memory_tenant ON agent_learning_memory(tenant_id);
```

### Go Types

**File:** `store/agent.go` (additions)

```go
type AgentLearningMemory struct {
    ID                 int32
    TenantID           int32
    CommonIssues       []CommonIssue       `json:"common_issues"`
    LearnedBehaviors   []LearnedBehavior   `json:"learned_behaviors"`
    ImprovementAreas   []ImprovementArea   `json:"improvement_areas"`
    PendingSuggestions []PendingSuggestion `json:"pending_suggestions"`
    AnalysisCount      int
    LastUpdated        time.Time
    Version            int
}

type LearnedBehavior struct {
    ID        string `json:"id"`
    Trigger   string `json:"trigger"`
    Behavior  string `json:"behavior"`
    Source    string `json:"source"`
    AddedAt   string `json:"added_at"`
    ApprovedBy int32 `json:"approved_by,omitempty"`
}

type CommonIssue struct {
    Category    string `json:"category"`
    Description string `json:"description"`
    Count       int    `json:"count"`
    LastSeen    string `json:"last_seen"`
}

type ImprovementArea struct {
    Category     string  `json:"category"`
    AverageScore float64 `json:"average_score"`
    MaxScore     int     `json:"max_score"`
    TrendUp      bool    `json:"trend_up"`
}

type PendingSuggestion struct {
    ID          string `json:"id"`
    Category    string `json:"category"`
    Trigger     string `json:"trigger"`
    Behavior    string `json:"behavior"`
    OccurrenceCount int `json:"occurrence_count"`
    FirstSeen   string `json:"first_seen"`
    LastSeen    string `json:"last_seen"`
}
```

### Aggregation Service

**File:** `server/router/api/v1/agent/learning.go` (new file)

Key constants and methods:
```go
const (
    MinOccurrencesForSuggestion = 3  // Issues must appear 3+ times
    MaxLearnedBehaviors         = 10 // Max behaviors per tenant
)

// Key methods:
func (s *Service) AggregateFromAnalysis(ctx context.Context, tenantID int32) error
func (s *Service) ApproveSuggestion(ctx context.Context, tenantID int32, suggestionID string, userID int32) error
func (s *Service) DismissSuggestion(ctx context.Context, tenantID int32, suggestionID string) error
func (s *Service) RemoveLearnedBehavior(ctx context.Context, tenantID int32, behaviorID string) error
func (s *Service) GetActiveLearnedBehaviors(ctx context.Context, tenantID int32) ([]store.LearnedBehavior, error)
```

### HTTP Handlers

**File:** `server/router/api/v1/agent/handlers.go` (additions)

| Handler | Method | Endpoint |
|---------|--------|----------|
| `HandleGetLearningMemory` | GET | `/api/v1/agent/:slug/learning` |
| `HandleAggregateFromAnalysis` | POST | `/api/v1/agent/:slug/learning/aggregate` |
| `HandleApproveSuggestion` | POST | `/api/v1/agent/:slug/learning/approve` |
| `HandleDismissSuggestion` | POST | `/api/v1/agent/:slug/learning/dismiss` |
| `HandleRemoveLearnedBehavior` | DELETE | `/api/v1/agent/:slug/learning/behaviors/:id` |
| `HandleClearLearningMemory` | DELETE | `/api/v1/agent/:slug/learning` |
| `HandleGetLearnedBehaviors` | GET | `/api/v1/agent/:slug/learning/behaviors` |

### System Prompt Integration

**File:** `server/router/api/v1/agent/service.go` (modifications)

Added to `AudienceConfig`:
```go
LearnedBehaviors []store.LearnedBehavior
```

Added to `LoadConfig()`:
- Load learned behaviors from learning memory

Added to `buildSystemPrompt()`:
```go
if len(config.LearnedBehaviors) > 0 {
    sb.WriteString("\nLEARNED IMPROVEMENTS (apply these specific behaviors):\n")
    for _, b := range config.LearnedBehaviors {
        sb.WriteString("- " + b.Trigger + ": " + b.Behavior + "\n")
    }
    sb.WriteString("\n")
}
```

### Frontend Store

**File:** `web/src/store/v2/agentAdmin.ts` (additions)

New interfaces:
```typescript
interface LearningMemory {
  id: number;
  tenantId: number;
  commonIssues: CommonIssue[];
  learnedBehaviors: LearnedBehavior[];
  improvementAreas: ImprovementArea[];
  pendingSuggestions: PendingSuggestion[];
  analysisCount: number;
  lastUpdated: string;
  version: number;
}
```

New state:
```typescript
learningMemory: LearningMemory | null = null;
isLoadingLearning: boolean = false;
```

New methods:
- `fetchLearningMemory(slug: string)`
- `aggregateFromAnalysis(slug: string)`
- `approveSuggestion(slug: string, suggestionId: string)`
- `dismissSuggestion(slug: string, suggestionId: string)`
- `removeLearnedBehavior(slug: string, behaviorId: string)`
- `clearLearningMemory(slug: string)`

### Frontend UI

**File:** `web/src/pages/AgentAdmin.tsx` (additions)

New component: `LearningInsightsSection`

Features:
- Displays improvement areas with average scores and trends
- Shows pending suggestions with approve/dismiss buttons
- Lists active learned behaviors with remove option
- Aggregate button to refresh from recent analyses
- Clear all button with confirmation

### Translations

**File:** `web/src/locales/en.json` (additions)

```json
{
  "agent-admin": {
    "learning-insights": "Learning Insights",
    "learning-insights-description": "Agent behavior improvements based on transcript analysis",
    "no-learning-data": "No learning data yet. Run transcript analyses to generate insights.",
    "improvement-areas": "Improvement Areas",
    "pending-suggestions": "Pending Suggestions",
    "learned-behaviors": "Learned Behaviors",
    "approve": "Approve",
    "dismiss": "Dismiss",
    "remove": "Remove",
    "aggregate": "Refresh from Analyses",
    "clear-learning": "Clear All",
    "clear-learning-confirm": "Are you sure you want to clear all learned behaviors?",
    "occurrences": "occurrences",
    "no-suggestions": "No pending suggestions",
    "no-learned-behaviors": "No learned behaviors yet. Approve suggestions to add them."
  }
}
```

---

## UI Preview

### Learning Insights Section (Agent Admin)

```
+------------------------------------------------------------------+
| Learning Insights                                    [12 analyses] |
| Agent behavior improvements based on transcript analysis           |
+------------------------------------------------------------------+
|                                                                   |
| Improvement Areas                                                 |
| ┌───────────────────────────────────────────────────────────────┐ |
| │ Information Gathering         10/15  ↓                        │ |
| │ Tone & Resolution             8/15   ↑                        │ |
| │ Conversation Flow            15/20   ↓                        │ |
| └───────────────────────────────────────────────────────────────┘ |
|                                                                   |
| Pending Suggestions (2)                          [Refresh from Analyses] |
| ┌───────────────────────────────────────────────────────────────┐ |
| │ ⚠️ Information Gathering (5 occurrences)                       │ |
| │ When: Customer provides initial request                        │ |
| │ Behavior: Ask for phone number within first 3 exchanges        │ |
| │ [Approve] [Dismiss]                                            │ |
| ├───────────────────────────────────────────────────────────────┤ |
| │ ⚠️ Tone & Resolution (3 occurrences)                           │ |
| │ When: Closing conversation                                     │ |
| │ Behavior: Summarize agreed next steps before saying goodbye    │ |
| │ [Approve] [Dismiss]                                            │ |
| └───────────────────────────────────────────────────────────────┘ |
|                                                                   |
| Learned Behaviors (2)                              [Clear All]    |
| ┌───────────────────────────────────────────────────────────────┐ |
| │ ✓ When customer mentions urgency                               │ |
| │   → Acknowledge urgency explicitly before proceeding           │ |
| │   Added: Jan 20, 2026                              [Remove]    │ |
| ├───────────────────────────────────────────────────────────────┤ |
| │ ✓ During insurance-related conversations                       │ |
| │   → Ask for policy number and claim status                     │ |
| │   Added: Jan 19, 2026                              [Remove]    │ |
| └───────────────────────────────────────────────────────────────┘ |
|                                                                   |
+------------------------------------------------------------------+
```

---

## How It Works

### Flow

```
1. User runs simulations/chats
         ↓
2. User analyzes transcripts (existing feature)
         ↓
3. Results stored in agent_analysis_results table
         ↓
4. User clicks "Refresh from Analyses" in Learning Insights
         ↓
5. System aggregates common issues and patterns
         ↓
6. Pending suggestions appear (issues appearing 3+ times)
         ↓
7. Admin reviews and approves/dismisses
         ↓
8. Approved behaviors stored in agent_learning_memory
         ↓
9. Next chat: LoadConfig() fetches learned behaviors
         ↓
10. buildSystemPrompt() injects behaviors into system prompt
         ↓
11. Agent follows improved behaviors
```

### System Prompt Example

When learned behaviors exist:

```
LEARNED IMPROVEMENTS (apply these specific behaviors):
- When customer mentions urgency: Acknowledge urgency explicitly before proceeding
- During information gathering: Ask for phone number within first 3 exchanges
- When closing conversation: Summarize agreed next steps before saying goodbye
```

---

## Verification Steps

1. **Build:** `task build`
2. **Run:** `task dev`
3. **Setup:**
   - Navigate to Agent Admin
   - Select a tenant with existing analysis results
4. **Test Learning Insights:**
   - Scroll to "Learning Insights" section
   - Click "Refresh from Analyses"
   - Review improvement areas
   - Approve a pending suggestion
   - Verify it moves to "Learned Behaviors"
5. **Test System Prompt:**
   - Start a new chat session
   - Check server logs for system prompt
   - Verify learned behaviors are included
6. **Test Agent Behavior:**
   - Run simulation with scenario matching learned behavior
   - Verify agent follows the learned behavior

---

## Remaining Work

| Item | Status | Notes |
|------|--------|-------|
| Build verification | ⏳ Pending | Run `task build` to verify compilation |
| Integration testing | ⏳ Pending | End-to-end test with real analysis data |
| Edge cases | ⏳ Pending | Test with 0 analyses, max behaviors, etc. |

---

## Configuration Constants

| Constant | Value | Location |
|----------|-------|----------|
| `MinOccurrencesForSuggestion` | 3 | `learning.go` |
| `MaxLearnedBehaviors` | 10 | `learning.go` |

These can be adjusted based on real-world usage patterns.

---
---

# Addendum: Agent Self-Improvement - How It Works

**Date:** 2026-01-20
**Status:** Reference Documentation

## Executive Summary

The Agent Self-Improvement feature creates a **feedback loop** that allows the AI agent to learn from past conversations and improve its behavior over time. This is NOT automatic - it requires explicit user actions at each step.

---

## The Complete Data Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        AGENT SELF-IMPROVEMENT FLOW                         │
└─────────────────────────────────────────────────────────────────────────────┘

STEP 1: RUN SIMULATIONS (or real chats happen)
         │
         ▼
┌─────────────────────────────────────┐
│   agent_simulation_transcripts      │  ← Stores simulation conversations
│   agent_sessions                    │  ← Stores real chat conversations
└─────────────────────────────────────┘
         │
         │  [User action: Click "Analyze" on a conversation]
         ▼
STEP 2: ANALYZE TRANSCRIPTS
         │
         ▼
┌─────────────────────────────────────┐
│   agent_analysis_results            │  ← Stores scores, issues, suggestions
│   - score (0-100)                   │     from LLM evaluation against
│   - grade (A/B/C/D/F)               │     KB.MD, POLICY.MD, SCRIPT.MD
│   - breakdown (6 categories)        │
│   - issues (what went wrong)        │
│   - suggestions (how to improve)    │
└─────────────────────────────────────┘
         │
         │  [User action: Click "Regenerate Insights" in Agent Admin]
         ▼
STEP 3: AGGREGATE INSIGHTS
         │
         ▼
┌─────────────────────────────────────┐
│   agent_learning_memory             │  ← Aggregates patterns from analyses
│   - improvement_areas               │     Identifies recurring issues
│   - pending_suggestions             │     Generates behavioral suggestions
│   - learned_behaviors               │     (requires 3+ occurrences)
└─────────────────────────────────────┘
         │
         │  [User action: Click "Approve" on a suggestion]
         ▼
STEP 4: APPROVE BEHAVIORS
         │
         ▼
┌─────────────────────────────────────┐
│   learned_behaviors (active)        │  ← Approved behaviors ready for use
│   - trigger: "When X happens"       │
│   - behavior: "Do Y instead"        │
└─────────────────────────────────────┘
         │
         │  [Automatic: Next chat session]
         ▼
STEP 5: INJECT INTO SYSTEM PROMPT
         │
         ▼
┌─────────────────────────────────────┐
│   System Prompt Addition:           │
│                                     │
│   LEARNED IMPROVEMENTS:             │
│   - When customer mentions urgency: │
│     Acknowledge urgency explicitly  │
│   - During info gathering:          │
│     Ask for phone within 3 turns    │
└─────────────────────────────────────┘
         │
         │  [Automatic: Agent follows learned behaviors]
         ▼
STEP 6: IMPROVED AGENT BEHAVIOR
```

---

## Why "No Learning Data" Appears

### Common Scenario

**User says:** "I have 10 simulations but 'Regenerate Insights' shows no data"

**Reason:** Simulations ≠ Analysis Results

| What You Have | What "Regenerate Insights" Needs |
|---------------|----------------------------------|
| 10 simulations in `agent_simulation_transcripts` | Analysis results in `agent_analysis_results` |

**The Missing Step:** You need to **analyze** each simulation before insights can be generated.

### The Code That Explains This

From `learning.go` line 43-52:

```go
// Get recent analysis results (last 50)
results, _, err := s.store.ListAgentAnalysisResults(ctx, &store.FindAgentAnalysisResult{
    TenantID: &tenantID,
    Limit:    50,
})
...
if len(results) == 0 {
    return memory, nil  // Returns empty memory - nothing to aggregate
}
```

The aggregation function queries `agent_analysis_results`, NOT `agent_simulation_transcripts`.

---

## Step-by-Step User Guide

### To Generate Learning Insights:

1. **Run Simulations** (Agent Simulation page)
   - Connect to tenant
   - Enter scenario and run simulation
   - Let it complete

2. **Analyze Simulations** (Agent Simulation page)
   - Click "History" dropdown
   - Select a completed simulation
   - Click "Analyze" button
   - Review the score and issues
   - **Repeat for multiple simulations** (at least 3-5 for meaningful insights)

3. **Regenerate Insights** (Agent Admin page)
   - Navigate to Agent Admin
   - Select the tenant
   - Scroll to "Learning Insights" section
   - Click "Regenerate Insights"
   - Now you'll see:
     - Improvement areas (categories scoring below 80%)
     - Pending suggestions (issues appearing 3+ times)

4. **Approve Suggestions** (Agent Admin page)
   - Review each pending suggestion
   - Click ✓ to approve (becomes a learned behavior)
   - Click ✗ to dismiss (ignored)

5. **Verify in Next Chat**
   - Start a new chat session
   - The agent now includes learned behaviors in its system prompt
   - Behavior should improve based on approved learnings

---

## Database Tables Overview

| Table | Purpose | Created By |
|-------|---------|------------|
| `agent_simulation_transcripts` | Stores simulation conversations | Running a simulation |
| `agent_sessions` | Stores real chat conversations | Customer chats |
| `agent_analysis_results` | Stores evaluation scores/issues | Clicking "Analyze" |
| `agent_learning_memory` | Stores aggregated insights | Clicking "Regenerate Insights" |

---

## Minimum Requirements for Suggestions

| Requirement | Value | Reason |
|-------------|-------|--------|
| Minimum analyses | 1 | To have any data to aggregate |
| Minimum occurrences | 3 | An issue must appear 3+ times to become a suggestion |
| Maximum behaviors | 10 | Prevent system prompt bloat |

**Why 3 occurrences?** A single occurrence might be an anomaly. Three occurrences indicate a pattern worth addressing.

---

## Troubleshooting

### "No Learning Data" Message

| Symptom | Cause | Solution |
|---------|-------|----------|
| No learning data | No analyses exist | Analyze some simulations first |
| Regenerate succeeds but no suggestions | Analyses exist but issues don't repeat 3+ times | Analyze more conversations |
| Suggestions exist but none approved | User hasn't clicked "Approve" | Review and approve relevant suggestions |

### Verifying Data Exists

**Check if analyses exist:**
```bash
sqlite3 build/data/memos_dev.db "SELECT COUNT(*) FROM agent_analysis_results WHERE tenant_id = (SELECT id FROM agent_tenants WHERE slug = 'bulovas');"
```

**Check if learning memory exists:**
```bash
sqlite3 build/data/memos_dev.db "SELECT * FROM agent_learning_memory WHERE tenant_id = (SELECT id FROM agent_tenants WHERE slug = 'bulovas');"
```

---

## System Prompt Integration

When learned behaviors exist, they're injected into the system prompt in `service.go`:

```go
if len(config.LearnedBehaviors) > 0 {
    sb.WriteString("\nLEARNED IMPROVEMENTS (apply these specific behaviors):\n")
    for _, b := range config.LearnedBehaviors {
        sb.WriteString("- " + b.Trigger + ": " + b.Behavior + "\n")
    }
    sb.WriteString("\n")
}
```

**Example system prompt addition:**
```
LEARNED IMPROVEMENTS (apply these specific behaviors):
- When customer mentions urgency: Acknowledge urgency explicitly before proceeding
- During information gathering: Ask for phone number within first 3 exchanges
- When closing conversation: Summarize agreed next steps before saying goodbye
```

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Manual analysis required | Prevents unnecessary LLM costs; user chooses which conversations matter |
| 3 occurrence threshold | Filters noise; only patterns become suggestions |
| Manual approval required | Human oversight prevents bad behaviors from being learned |
| Maximum 10 behaviors | Keeps system prompt focused; prevents bloat |
| Tenant-level (not audience-level) | Most behavioral improvements apply to both external and internal |

---

## Summary

```
Simulations/Chats → [Analyze] → Analysis Results → [Regenerate] → Suggestions → [Approve] → Learned Behaviors → Better Agent
```

**Key Insight:** Each arrow represents a required user action. The system does not automatically learn - it provides the infrastructure for supervised learning where humans approve improvements.

---
---

# Addendum: Simplified Agent Self-Improvement (v2)

**Date:** 2026-01-20
**Status:** Proposal
**Supersedes:** Previous Learning Memory system (Addendum above)

## Executive Summary

Simplify the agent self-improvement process from a 6-step workflow to a **2-step workflow**:

| Before (Complex) | After (Simplified) |
|------------------|-------------------|
| 1. Run simulation | 1. Run simulation |
| 2. Analyze simulation | 2. Analyze → Select → Apply |
| 3. Go to Agent Admin | ~~removed~~ |
| 4. Click "Regenerate Insights" | ~~removed~~ |
| 5. Wait for 3+ occurrences | ~~removed~~ |
| 6. Approve suggestions | ~~merged into step 2~~ |

**New Flow:**
```
Simulation → [Analyze] → Select Issues/Suggestions → [Apply] → Immediate Improvement
```

---

## User Decisions

| Question | Decision |
|----------|----------|
| Auto-apply all or let user select? | **Let user select** (checkbox list) |
| Replace or work alongside current system? | **Replace entirely** |
| Way to view/remove applied learnings? | **Yes, simple list in Agent Admin** |

---

## The New Workflow

### Step 1: Analyze a Conversation

User clicks "Analyze" on a conversation from the History dropdown.

**Current behavior:** Shows analysis results modal with score, breakdown, issues, suggestions.

**New behavior:** Same modal, but with **checkboxes** next to each issue and suggestion.

### Step 2: Select and Apply

User reviews the issues and suggestions, checks the ones they want to apply, and clicks "Apply Selected".

**Example UI:**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Transcript Analysis                                                    [X]  │
├─────────────────────────────────────────────────────────────────────────────┤
│ Overall Score: 65/100 (Grade: D)                                            │
│                                                                             │
│ ═══════════════════════════════════════════════════════════════════════════ │
│                                                                             │
│ Issues Found:                                                    [Select All]│
│ ┌─────────────────────────────────────────────────────────────────────────┐ │
│ │ [✓] ⚠️ Turn 2: Agent provided incorrect emergency phone number          │ │
│ │     "(555) 000-0000" - should be (631) 358-5200 or 855-DRYITNOW        │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ [✓] ⚠️ Turn 2: Safety instructions repeated verbatim in almost every   │ │
│ │     turn, leading to poor conversation flow and redundancy             │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ [ ] ℹ️ Turn 5: Could acknowledge customer frustration more explicitly  │ │
│ └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│ Suggestions for Improvement:                                     [Select All]│
│ ┌─────────────────────────────────────────────────────────────────────────┐ │
│ │ [✓] Use the correct emergency contact numbers from KB in all responses │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ [✓] Provide safety guidance once (or at most twice) at start, then     │ │
│ │     focus on gathering information and confirming next steps           │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ [✓] Structure dialogue: opening → safety → info gathering →            │ │
│ │     confirmation → concise closing                                     │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ [ ] After offering referral, explicitly confirm user noted the number  │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ [✓] Limit repetitive phrasing to keep interaction efficient            │ │
│ └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│ ═══════════════════════════════════════════════════════════════════════════ │
│                                                                             │
│                              [Apply Selected (5)]              [Close]      │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Step 3: Immediate Effect

Selected items are immediately added to `agent_learning_memory.learned_behaviors` and will be included in the system prompt for the **next chat session**.

**Success toast:** "5 improvements applied. Agent will use these in future conversations."

---

## Agent Admin: Applied Learnings (Simplified)

Replace the complex "Learning Insights" section with a simple **"Applied Learnings"** section:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Applied Learnings                                              [5 active]   │
│ Improvements applied from transcript analysis                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│ ┌─────────────────────────────────────────────────────────────────────────┐ │
│ │ Use the correct emergency contact numbers from KB in all responses      │ │
│ │ Applied: Jan 20, 2026 • Source: Simulation analysis            [Remove] │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ Provide safety guidance once at start, then focus on info gathering     │ │
│ │ Applied: Jan 20, 2026 • Source: Simulation analysis            [Remove] │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ Structure: opening → safety → info gathering → confirmation → closing   │ │
│ │ Applied: Jan 20, 2026 • Source: Simulation analysis            [Remove] │ │
│ ├─────────────────────────────────────────────────────────────────────────┤ │
│ │ Limit repetitive phrasing to keep interaction efficient                 │ │
│ │ Applied: Jan 20, 2026 • Source: Simulation analysis            [Remove] │ │
│ └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│ No pending suggestions • No regeneration needed                             │
│                                                        [Clear All Learnings]│
└─────────────────────────────────────────────────────────────────────────────┘
```

**Features:**
- Simple list of all applied learnings
- Remove individual items
- Clear all button
- No "Regenerate", no "Approve/Dismiss", no occurrence counting

---

## Data Model Changes

### Keep (Simplified)

**Table: `agent_learning_memory`**
- Keep `learned_behaviors` array (stores applied items)
- Keep `analysis_count` (how many analyses contributed)

### Remove (No Longer Needed)

| Field | Reason |
|-------|--------|
| `common_issues` | No aggregation needed |
| `improvement_areas` | No score tracking needed |
| `pending_suggestions` | Direct apply, no pending state |

### New Learned Behavior Structure

```go
type LearnedBehavior struct {
    ID          string `json:"id"`
    Content     string `json:"content"`      // The actual learning text
    Type        string `json:"type"`         // "issue" or "suggestion"
    SourceID    string `json:"source_id"`    // Analysis result ID
    SourceTurn  int    `json:"source_turn"`  // Turn number (for issues)
    AppliedAt   string `json:"applied_at"`
    IsActive    bool   `json:"is_active"`
}
```

---

## API Changes

### New Endpoint: Apply Selected Learnings

**POST `/api/v1/agent/:slug/learning/apply`**

```json
{
  "analysis_id": "ana-abc123",
  "selected_issues": [0, 1],      // Indices of issues to apply
  "selected_suggestions": [0, 1, 2, 4]  // Indices of suggestions to apply
}
```

**Response:**
```json
{
  "applied_count": 5,
  "learned_behaviors": [...],
  "message": "5 improvements applied"
}
```

### Simplified Endpoints

| Keep | Remove |
|------|--------|
| `GET /learning` | `POST /learning/regenerate` |
| `DELETE /learning` | `POST /learning/approve` |
| `DELETE /learning/behaviors/:id` | `POST /learning/dismiss` |
| `POST /learning/apply` (new) | `POST /learning/behaviors/:id/toggle` |

---

## System Prompt Integration

Simplified - just output the content directly:

```go
if len(config.LearnedBehaviors) > 0 {
    sb.WriteString("\nLEARNED IMPROVEMENTS (apply these specific behaviors):\n")
    for _, b := range config.LearnedBehaviors {
        sb.WriteString("- " + b.Content + "\n")
    }
    sb.WriteString("\n")
}
```

**Example output:**
```
LEARNED IMPROVEMENTS (apply these specific behaviors):
- Use the correct emergency contact numbers from KB in all responses
- Provide safety guidance once at start, then focus on info gathering
- Structure: opening → safety → info gathering → confirmation → closing
- Limit repetitive phrasing to keep interaction efficient
```

---

## Files to Modify

### Backend

| File | Changes |
|------|---------|
| `store/agent.go` | Simplify `LearnedBehavior` struct (Content instead of Trigger+Behavior) |
| `server/router/api/v1/agent/learning.go` | Remove aggregation logic, add `ApplySelectedLearnings()` |
| `server/router/api/v1/agent/handlers.go` | Remove regenerate/approve/dismiss, add `HandleApplyLearnings` |
| `server/router/api/v1/v1.go` | Update routes |
| `server/router/api/v1/agent/service.go` | Update system prompt to use `b.Content` |

### Frontend

| File | Changes |
|------|---------|
| `web/src/pages/AgentSimulation.tsx` | Add checkboxes to analysis modal, add "Apply Selected" button |
| `web/src/pages/AgentAdmin.tsx` | Replace "Learning Insights" with simple "Applied Learnings" list |
| `web/src/store/v2/agentSimulation.ts` | Add `applySelectedLearnings()` method |
| `web/src/store/v2/agentAdmin.ts` | Simplify learning memory methods |
| `web/src/locales/en.json` | Update translations |

---

## Implementation Sequence

### Phase 1: Backend Simplification
1. Simplify `LearnedBehavior` struct (add `Content`, keep others for backward compat)
2. Add `ApplySelectedLearnings()` to learning.go
3. Add `HandleApplyLearnings` handler
4. Update system prompt to use `b.Content`
5. Keep old handlers temporarily for migration

### Phase 2: Frontend - Analysis Modal
1. Add checkbox state to analysis results in `AgentSimulation.tsx`
2. Add "Select All" buttons for issues and suggestions
3. Add "Apply Selected (N)" button
4. Call new API endpoint on apply
5. Show success toast

### Phase 3: Frontend - Agent Admin
1. Replace `LearningInsightsSection` with simple `AppliedLearningsSection`
2. Show list of applied learnings with remove buttons
3. Add "Clear All" button
4. Remove regenerate/approve UI

### Phase 4: Cleanup
1. Remove unused backend handlers
2. Remove unused frontend store methods
3. Update translations

---

## Verification

1. **Run a simulation** for bulovas tenant
2. **Open History dropdown**, select the simulation
3. **Click Analyze** - should see checkboxes next to issues/suggestions
4. **Check 3-4 items**, click "Apply Selected"
5. **Success toast** should appear
6. **Go to Agent Admin** - should see applied learnings in simple list
7. **Start new simulation** - verify agent follows the learnings
8. **Remove a learning** from Agent Admin
9. **Verify** it no longer appears in subsequent chats

---

## Benefits of Simplified Approach

| Aspect | Before | After |
|--------|--------|-------|
| Steps to apply learning | 6 | 2 |
| Time to see improvement | Hours (need multiple analyses) | Immediate |
| Cognitive load | High (multiple screens, concepts) | Low (one modal, one action) |
| Flexibility | Must wait for patterns | Apply any insight immediately |
| Maintenance | Complex aggregation logic | Simple CRUD |

---

## Tradeoffs

| Tradeoff | Mitigation |
|----------|------------|
| No automatic pattern detection | User judgment replaces automation |
| Could apply bad learnings | Easy to remove from Agent Admin |
| No occurrence threshold | User discretion is the filter |
| Lose trend tracking | Simplicity over analytics |

---

## Migration Path

1. Keep existing `agent_learning_memory` table structure
2. Existing `learned_behaviors` continue to work (backward compatible)
3. New learnings use simplified `Content` field
4. System prompt logic handles both old and new format
5. Remove unused columns in future migration

---

## Summary

```
OLD: Simulation → Analyze → Agent Admin → Regenerate → Wait for 3+ → Approve → Improved Agent
NEW: Simulation → Analyze → Select → Apply → Improved Agent
```

**Key Principle:** Trust the user to make good decisions about what to apply. Remove the multi-step approval workflow in favor of immediate, selective application.

---
---

# Addendum 5: UI Changes After v2 Implementation

## Build Requirement

After implementing the Simplified Agent Self-Improvement (v2), you **must rebuild** the project for changes to take effect:

```bash
task build
```

The frontend bundle needs to be regenerated to include the new UI components.

---

## Expected UI Changes

### Agent Simulation - Analysis Modal

**Before v2:** Analysis modal shows issues and suggestions as read-only text.

**After v2:** Analysis modal shows:
- Checkboxes next to each issue (with "Select All" button)
- Checkboxes next to each suggestion (with "Select All" button)
- "Apply Selected (N)" button at the bottom
- Hint text explaining what will happen

```
+------------------------------------------------------------------+
| Transcript Analysis                                        [X]   |
+------------------------------------------------------------------+
| Overall Score: 78/100 (Grade: B)                                 |
|                                                                  |
| Issues Found:                              [Select All]          |
| [✓] ⚠️ Turn 5: Did not ask for phone number                     |
| [ ] ℹ️ Turn 3: Could acknowledge frustration more explicitly     |
|                                                                  |
| Suggestions:                               [Select All]          |
| [✓] ✅ Ask for contact information earlier                       |
| [✓] ✅ Add empathetic acknowledgment for urgency                 |
|                                                                  |
| +------------------------------------------------------+         |
| |              Apply Selected (3)                      |         |
| +------------------------------------------------------+         |
| Selected items will be added as learned behaviors...             |
+------------------------------------------------------------------+
```

---

### Agent Admin - Learning Section

**Before v2:** "Learning Insights" section with:
- "Regenerate Insights" button
- Common Issues tab
- Pending Suggestions tab (with Approve/Dismiss)
- Learned Behaviors tab (with toggle active/inactive)
- Improvement Areas tab

**After v2:** "Applied Learnings" section (simplified) with:
- Simple list of applied learnings
- Remove button (trash icon) for each learning
- "Clear All" button
- No regenerate, no approve/dismiss workflow

```
+------------------------------------------------------------------+
| 🧠 Applied Learnings                         [3 active]          |
| Improvements applied from transcript analysis                    |
+------------------------------------------------------------------+
| ┌────────────────────────────────────────────────────────────┐   |
| │ Did not ask for customer phone number                   [🗑]│   |
| │ Added: 2026-01-20 • analysis                               │   |
| └────────────────────────────────────────────────────────────┘   |
| ┌────────────────────────────────────────────────────────────┐   |
| │ Ask for contact information earlier in conversation     [🗑]│   |
| │ Added: 2026-01-20 • analysis                               │   |
| └────────────────────────────────────────────────────────────┘   |
| ┌────────────────────────────────────────────────────────────┐   |
| │ Add empathetic acknowledgment when customer expresses...  [🗑]│   |
| │ Added: 2026-01-20 • analysis                               │   |
| └────────────────────────────────────────────────────────────┘   |
|                                                                  |
|                      [ Clear All Learnings ]                     |
+------------------------------------------------------------------+
```

If no learnings exist yet:
```
+------------------------------------------------------------------+
| 🧠 Applied Learnings                                             |
| Improvements applied from transcript analysis                    |
+------------------------------------------------------------------+
|                           🧠                                     |
|                   No learnings yet                               |
|   Run simulations and use Analyze to add improvements            |
+------------------------------------------------------------------+
```

---

## Removed UI Elements

The following UI elements from Agent Admin are **removed** in v2:

| Removed Element | Reason |
|-----------------|--------|
| "Regenerate Insights" button | No longer needed - learnings applied immediately from analysis |
| Common Issues tab | Pattern aggregation removed |
| Pending Suggestions tab | No approval workflow |
| Approve/Dismiss buttons | Decisions made at analysis time |
| Improvement Areas tab | Trend tracking removed for simplicity |
| Active/Inactive toggle | All learnings are active; remove if not wanted |

---

## Troubleshooting

### Issue: Checkboxes not showing in analysis modal
**Cause:** Frontend not rebuilt after code changes.
**Solution:** Run `task build` and refresh browser (clear cache if needed).

### Issue: Regenerate button still showing in Agent Admin
**Cause:** Frontend not rebuilt after code changes.
**Solution:** Run `task build` and refresh browser (clear cache if needed).

### Issue: "Apply Selected" does nothing
**Cause:** Backend route not registered or server not restarted.
**Solution:** Restart dev server with `task dev`.

### Issue: Applied learnings don't appear in Agent Admin
**Cause:** Learning memory fetch not triggered after apply.
**Solution:** Navigate away and back to Agent Admin, or refresh page.

---

## Files Modified in v2

| File | Changes |
|------|---------|
| `store/agent.go` | Added `Content` field to `LearnedBehavior` |
| `server/router/api/v1/agent/learning.go` | Added `ApplySelectedLearnings()` |
| `server/router/api/v1/agent/handlers.go` | Added `HandleApplyLearnings` |
| `server/router/api/v1/v1.go` | Added `/learning/apply` route |
| `server/router/api/v1/agent/service.go` | Updated system prompt to handle both formats |
| `web/src/pages/AgentSimulation.tsx` | Added checkboxes and apply button to modal |
| `web/src/store/v2/agentSimulation.ts` | Added `applySelectedLearnings()` |
| `web/src/pages/AgentAdmin.tsx` | Replaced `LearningInsightsSection` with `AppliedLearningsSection` |
| `web/src/store/v2/agentAdmin.ts` | Removed unused methods |
| `web/src/locales/en.json` | Added new translation keys |

---
---

# Addendum 6: How Applied Learnings Work (Storage & Injection)

## Key Clarification

**Applied learnings do NOT modify KB.MD, POLICY.MD, or SCRIPT.MD.**

They are stored separately in the `agent_learning_memory` table and injected into the system prompt at runtime.

---

## Storage Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     DATABASE STORAGE                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  agent_source_files (per-audience)     agent_learning_memory    │
│  ┌─────────────────────────────┐       ┌───────────────────┐   │
│  │ KB.MD      │ Original file  │       │ learned_behaviors │   │
│  │ POLICY.MD  │ content        │       │ (JSON array)      │   │
│  └─────────────────────────────┘       └───────────────────┘   │
│                                                                 │
│  agent_tenant_script (per-tenant)      agent_analysis_results   │
│  ┌─────────────────────────────┐       ┌───────────────────┐   │
│  │ SCRIPT.MD  │ Original file  │       │ Analysis history  │   │
│  └─────────────────────────────┘       └───────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

| Table | Content | Mutated by Learnings? |
|-------|---------|----------------------|
| `agent_source_files` | KB.MD, POLICY.MD content | **NO** - never modified |
| `agent_tenant_script` | SCRIPT.MD content | **NO** - never modified |
| `agent_learning_memory` | Applied learnings | **YES** - learnings added here |
| `agent_analysis_results` | Analysis history | **NO** - read-only records |

---

## Runtime System Prompt Construction

When the agent processes a message, `buildSystemPrompt()` in `service.go` constructs the full context:

```
┌─────────────────────────────────────────────────────────────────┐
│                     SYSTEM PROMPT (runtime)                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Identity & Role (from POLICY.MD)                           │
│     "You are Sarah, a customer service representative..."      │
│                                                                 │
│  2. Knowledge Base (from KB.MD)                                │
│     Services, FAQs, company info...                            │
│                                                                 │
│  3. Policies & Rules (from POLICY.MD)                          │
│     Intent handling, escalation rules...                       │
│                                                                 │
│  4. Conversation Flow (from SCRIPT.MD)                         │
│     Opening, information gathering, closing...                 │
│                                                                 │
│  5. ★ LEARNED BEHAVIORS (from agent_learning_memory) ★         │
│     "Based on past interactions, remember to:"                 │
│     - Ask for contact information earlier in conversation      │
│     - Add empathetic acknowledgment when customer is urgent    │
│     - Always verify phone number before scheduling             │
│                                                                 │
│  6. Response Format Instructions                               │
│     Plain text, no markdown...                                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Code Reference: System Prompt Injection

From `server/router/api/v1/agent/service.go`:

```go
// Inject learned behaviors into system prompt
if config.LearningMemory != nil && len(config.LearningMemory.LearnedBehaviors) > 0 {
    sb.WriteString("\nLEARNED BEHAVIORS (from past conversation analysis):\n")
    sb.WriteString("Based on analysis of past interactions, remember to:\n")
    for _, b := range config.LearningMemory.LearnedBehaviors {
        if !b.IsActive {
            continue
        }
        // v2 format: uses Content field
        if b.Content != "" {
            sb.WriteString("- " + b.Content + "\n")
        } else if b.Trigger != "" && b.Behavior != "" {
            // v1 format: uses Trigger + Behavior
            sb.WriteString("- " + b.Trigger + ": " + b.Behavior + "\n")
        }
    }
    sb.WriteString("\n")
}
```

---

## Why This Design?

| Benefit | Explanation |
|---------|-------------|
| **Non-destructive** | Original KB/POLICY/SCRIPT files remain unchanged |
| **Reversible** | Remove a learning instantly without file rollback |
| **Auditable** | Clear separation: "what was authored" vs "what was learned" |
| **Version-independent** | Learnings persist across file updates |
| **Tenant-isolated** | Each tenant has its own learning memory |

---

## Data Flow Diagram

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   SIMULATION     │     │    ANALYSIS      │     │  APPLY LEARNING  │
│                  │     │                  │     │                  │
│  Run simulation  │────▶│  Click Analyze   │────▶│  Select items    │
│  (conversation)  │     │  (LLM evaluates) │     │  Click Apply     │
└──────────────────┘     └──────────────────┘     └────────┬─────────┘
                                                           │
                                                           ▼
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   CHAT SESSION   │◀────│  SYSTEM PROMPT   │◀────│ LEARNING MEMORY  │
│                  │     │                  │     │                  │
│  Agent responds  │     │  KB + POLICY +   │     │  learned_behaviors│
│  with learning   │     │  SCRIPT + ★LEARN │     │  [content, ...]  │
└──────────────────┘     └──────────────────┘     └──────────────────┘
```

---

## Comparison: Source Files vs Learnings

| Aspect | Source Files (KB/POLICY/SCRIPT) | Applied Learnings |
|--------|--------------------------------|-------------------|
| **Storage** | `agent_source_files`, `agent_tenant_script` | `agent_learning_memory.learned_behaviors` |
| **Format** | Markdown files | JSON array of objects |
| **Edit method** | File upload in Agent Admin | Apply from analysis modal |
| **Versioning** | Yes (version column, restore feature) | No (add/remove only) |
| **Scope** | Per-audience (KB/POLICY) or per-tenant (SCRIPT) | Per-tenant |
| **Purpose** | Foundational knowledge & rules | Behavioral adjustments |
| **Token cost** | Large (full file content) | Small (short statements) |

---

## Example: Before and After Learning

### Before (no learnings)

System prompt ends with:
```
Respond naturally and helpfully. Be concise but complete.
```

### After applying learning: "Ask for contact information earlier"

System prompt now includes:
```
LEARNED BEHAVIORS (from past conversation analysis):
Based on analysis of past interactions, remember to:
- Ask for contact information earlier in conversation

Respond naturally and helpfully. Be concise but complete.
```

---

## Limitations

| Limitation | Workaround |
|------------|------------|
| Learnings are text-only hints | Write clear, actionable statements |
| No structured integration into KB services | Manually update KB.MD if service info changes |
| No automatic POLICY.MD rule updates | Manually update POLICY.MD for new rules |
| Learnings don't modify intents/FAQs | These require KB.MD file update |

---

## When to Use Learnings vs File Updates

| Situation | Use Learnings | Update Source File |
|-----------|---------------|-------------------|
| Agent forgot to ask for phone | ✅ | |
| Agent tone was too formal | ✅ | |
| New service offering added | | ✅ KB.MD |
| New intent to recognize | | ✅ POLICY.MD |
| Change in escalation rules | | ✅ POLICY.MD |
| New conversation section | | ✅ SCRIPT.MD |
| Behavioral reminder | ✅ | |
| Company policy change | | ✅ POLICY.MD |

---

## Summary

```
┌────────────────────────────────────────────────────────────────┐
│  Applied Learnings = Behavioral hints injected at runtime      │
│                                                                │
│  • Stored in: agent_learning_memory.learned_behaviors (JSON)   │
│  • NOT stored in: KB.MD, POLICY.MD, SCRIPT.MD                 │
│  • Injected into: System prompt (section 5 of 6)              │
│  • Effect: Agent receives reminder text before responding      │
└────────────────────────────────────────────────────────────────┘
```

---
---

# Addendum 7: Agent Compliance & Anti-Hallucination Architecture

## Problem Statement

The current agent self-improvement system has critical issues:

1. **Hallucinations**: Agent invents services, processes, or information not in KB.MD
2. **Policy Violations**: Agent ignores rules in POLICY.MD
3. **Script Deviations**: Agent skips conversation flow defined in SCRIPT.MD
4. **Ineffective Learnings**: Applied learnings are not enforced or verified

**Root Cause Analysis:**

| Issue | Current State | Why It Fails |
|-------|---------------|--------------|
| Learnings placement | Appended at end of system prompt | LLMs prioritize earlier content; learnings get "forgotten" |
| No verification | Agent response not checked against source | No mechanism to catch violations |
| No compliance audit | No post-response evaluation | Hallucinations go undetected |
| No output sanitization | Raw LLM output sent to user | Hallucinated content not filtered |
| Weak constraints | "Use plain text" is the only format rule | No structural enforcement |

---

## Reference Architecture: OPUS13 (distance.so/agent)

The `distance.so/agent` codebase implements a sophisticated multi-tier heuristic system called **OPUS13**. Key innovations:

### 1. Multi-Tier Scoring with Fail-Safe Defaults

```
┌────────────────────────────────────────────────────────────────┐
│  TIER 1: Keyword/Semantic Check (Fast, Deterministic)          │
│  ├─ Fuzzy match against configured keywords                    │
│  ├─ Returns score 0-100 based on factor weights               │
│  └─ Used as fallback if LLM fails                             │
├────────────────────────────────────────────────────────────────┤
│  TIER 2: LLM Analysis (Weighted Scoring)                       │
│  ├─ Structured JSON output with confidence                     │
│  ├─ Weighted calculation: Σ(factor_score × weight / 100)      │
│  └─ Confidence gate: High score + low confidence → downgrade  │
├────────────────────────────────────────────────────────────────┤
│  TIER 3: Fallback to Tier 1 on LLM failure                     │
│  └─ System continues to function without LLM                   │
└────────────────────────────────────────────────────────────────┘
```

### 2. Compliance Auditing

```go
// From OPUS13: Compliance checklist evaluated against transcript
type ComplianceCheck struct {
    Trigger     string  // Factor name or "ALL"
    Requirement string  // "Must ask user to evacuate"
    Weight      int     // 1-5 importance
}

// Audit runs asynchronously after conversation ends
// Score = (earned / possible) × 100
// If score < 60%: Log WARNING with failure details
```

### 3. Output Sanitization

```go
// Remove hallucinated system tags before sending to user
patterns := []string{
    `^\[SYSTEM\].*$`,
    `^\[DEBUG\].*$`,
    `^\[THOUGHT\].*$`,
    `\[(SYSTEM|DEBUG|THOUGHT).*?\]`,
}
```

### 4. Sticky State (Prevents Backsliding)

```go
// Once "emergency", stays "emergency" in session
if prev_urgency == "emergency" {
    urgency = "emergency"  // Cannot downgrade
}
```

---

## Proposed Architecture: Universal Service Agent Compliance

Adapting OPUS13 principles for **any service business** (not just restoration):

### Phase 1: Heuristic Scoring Categories (Universal)

Replace domain-specific factors with universal service categories:

| Category | Weight | Description | Applicable To |
|----------|--------|-------------|---------------|
| **Urgency** | 25 | How immediately is help needed? | All services |
| **Safety Risk** | 20 | Is there physical/financial danger? | All services |
| **Service Match** | 20 | Does request match our offerings? | All services |
| **Escalation Signal** | 15 | Does customer want supervisor/complaint? | All services |
| **Lead Quality** | 10 | Has customer provided contact info? | All services |
| **Sentiment** | 10 | Is customer frustrated/satisfied? | All services |

**Configuration Example** (stored in `agent_scoring_config`):

```json
{
  "version": "1.0",
  "thresholds": {
    "high_urgency": 75,
    "medium_urgency": 40,
    "low_urgency": 0
  },
  "categories": [
    {
      "name": "urgency",
      "weight": 25,
      "keywords": {
        "high": ["immediately", "urgent", "asap", "right now", "emergency"],
        "medium": ["soon", "today", "this week"],
        "low": ["whenever", "no rush", "at your convenience"]
      }
    },
    {
      "name": "safety_risk",
      "weight": 20,
      "keywords": {
        "high": ["danger", "unsafe", "hurt", "injured", "fire", "flood"],
        "medium": ["concern", "worried", "problem"],
        "low": []
      }
    },
    {
      "name": "service_match",
      "weight": 20,
      "check_against": "kb_services",
      "exclusion_check": "kb_exclusions"
    },
    {
      "name": "escalation_signal",
      "weight": 15,
      "keywords": {
        "high": ["manager", "supervisor", "lawyer", "sue", "complaint", "BBB"],
        "medium": ["unhappy", "dissatisfied", "not working"],
        "low": []
      }
    },
    {
      "name": "lead_quality",
      "weight": 10,
      "fields": ["name", "phone", "email", "address"]
    },
    {
      "name": "sentiment",
      "weight": 10,
      "keywords": {
        "positive": ["thank you", "appreciate", "great", "helpful"],
        "negative": ["frustrated", "angry", "terrible", "worst"],
        "neutral": []
      }
    }
  ]
}
```

---

### Phase 2: Compliance Verification Layer

Add a **post-response verification** step before sending to user:

```
┌─────────────────────────────────────────────────────────────────┐
│                    COMPLIANCE VERIFICATION FLOW                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Agent generates response                                    │
│                         ↓                                       │
│  2. VERIFICATION LAYER checks:                                  │
│     ├─ Service mentioned → In KB.MD services? (not exclusions) │
│     ├─ Process described → In KB.MD or SCRIPT.MD?              │
│     ├─ Policy claimed → In POLICY.MD rules?                    │
│     ├─ Price/time quoted → In KB.MD?                           │
│     └─ Phone/email given → Matches config?                     │
│                         ↓                                       │
│  3a. PASS → Send response to user                              │
│  3b. FAIL → Regenerate with stricter constraints OR            │
│             Flag for human review                               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Implementation: New `VerifyCompliance` Function**

```go
// server/router/api/v1/agent/compliance.go

type ComplianceResult struct {
    Passed      bool                  `json:"passed"`
    Violations  []ComplianceViolation `json:"violations"`
    Score       int                   `json:"score"`       // 0-100
    Confidence  int                   `json:"confidence"`  // 0-100
}

type ComplianceViolation struct {
    Type        string `json:"type"`        // "hallucinated_service", "wrong_contact", etc.
    Severity    string `json:"severity"`    // "critical", "warning", "info"
    Content     string `json:"content"`     // The problematic text
    Suggestion  string `json:"suggestion"`  // How to fix
}

func (s *Service) VerifyCompliance(
    ctx context.Context,
    response string,
    config *AgentConfig,
) (*ComplianceResult, error) {
    result := &ComplianceResult{Passed: true, Score: 100}

    // Check 1: Service mentions
    mentionedServices := extractServiceMentions(response)
    for _, svc := range mentionedServices {
        if !isInKBServices(svc, config.Services) {
            if isInExclusions(svc, config.Exclusions) {
                result.Violations = append(result.Violations, ComplianceViolation{
                    Type:     "excluded_service_offered",
                    Severity: "critical",
                    Content:  svc,
                    Suggestion: "Remove mention of excluded service",
                })
            } else {
                result.Violations = append(result.Violations, ComplianceViolation{
                    Type:     "hallucinated_service",
                    Severity: "critical",
                    Content:  svc,
                    Suggestion: "Remove or replace with actual service from KB",
                })
            }
        }
    }

    // Check 2: Contact info
    if containsPhoneNumber(response) {
        phones := extractPhoneNumbers(response)
        for _, phone := range phones {
            if phone != config.Audience.EmergencyPhone &&
               phone != config.Audience.MainPhone {
                result.Violations = append(result.Violations, ComplianceViolation{
                    Type:     "wrong_contact",
                    Severity: "critical",
                    Content:  phone,
                    Suggestion: fmt.Sprintf("Use configured phone: %s",
                                            config.Audience.EmergencyPhone),
                })
            }
        }
    }

    // Check 3: Policy claims (LLM verification)
    // Check 4: Process descriptions (LLM verification)

    // Calculate final score
    for _, v := range result.Violations {
        switch v.Severity {
        case "critical":
            result.Score -= 30
        case "warning":
            result.Score -= 10
        case "info":
            result.Score -= 5
        }
    }
    if result.Score < 0 {
        result.Score = 0
    }

    result.Passed = result.Score >= 70 && !hasCriticalViolation(result.Violations)
    return result, nil
}
```

---

### Phase 3: Output Sanitization

Add sanitization before sending response to user:

```go
// server/router/api/v1/agent/sanitizer.go

func SanitizeResponse(response string) string {
    // Remove hallucinated system tags
    patterns := []string{
        `(?m)^\[SYSTEM\].*$`,
        `(?m)^\[DEBUG\].*$`,
        `(?m)^\[THOUGHT\].*$`,
        `(?m)^\[INTERNAL\].*$`,
        `\[(SYSTEM|DEBUG|THOUGHT|INTERNAL)[^\]]*\]`,
        `(?m)^Assistant:.*$`,  // Remove self-references
        `(?m)^AI:.*$`,
    }

    result := response
    for _, pattern := range patterns {
        re := regexp.MustCompile(pattern)
        result = re.ReplaceAllString(result, "")
    }

    // Clean up excessive whitespace
    result = strings.TrimSpace(result)
    result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

    return result
}
```

---

### Phase 4: Strengthened System Prompt Structure

Reorganize system prompt to prioritize compliance:

```
┌─────────────────────────────────────────────────────────────────┐
│                     SYSTEM PROMPT STRUCTURE                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. CRITICAL CONSTRAINTS (First - Highest Priority)            │
│     ├─ "You MUST NOT invent services not in the list below"    │
│     ├─ "You MUST NOT provide contact info not listed below"    │
│     ├─ "You MUST NOT describe processes not in FLOW GUIDE"     │
│     └─ "If uncertain, say 'I'll need to check on that'"        │
│                                                                 │
│  2. IDENTITY (from POLICY.MD)                                  │
│     └─ Name, company, role                                     │
│                                                                 │
│  3. SERVICES WE OFFER (from KB.MD)                             │
│     └─ Explicit list - ONLY these can be mentioned             │
│                                                                 │
│  4. SERVICES WE DON'T OFFER (from KB.MD)                       │
│     └─ Explicit exclusions - NEVER offer these                 │
│                                                                 │
│  5. CONVERSATION FLOW (from SCRIPT.MD)                         │
│     └─ Step-by-step guide                                      │
│                                                                 │
│  6. POLICIES & RULES (from POLICY.MD)                          │
│     └─ Intents, escalation rules, guidelines                   │
│                                                                 │
│  7. LEARNED BEHAVIORS (from analysis)                          │
│     └─ Applied learnings                                       │
│                                                                 │
│  8. CONTACT INFO (Authoritative)                               │
│     └─ ONLY these numbers/emails may be provided               │
│                                                                 │
│  9. RESPONSE FORMAT                                            │
│     └─ Plain text, no markdown                                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key Change**: Critical constraints at TOP, not bottom.

---

### Phase 5: Compliance Audit (Post-Conversation)

Add asynchronous audit after conversation ends:

```go
// server/router/api/v1/agent/audit.go

type ComplianceAudit struct {
    ConversationID  string                  `json:"conversation_id"`
    TenantID        int32                   `json:"tenant_id"`
    Score           int                     `json:"score"`  // 0-100
    Checks          []ComplianceCheckResult `json:"checks"`
    OverallPassed   bool                    `json:"overall_passed"`
    AuditedAt       time.Time               `json:"audited_at"`
}

type ComplianceCheckResult struct {
    Requirement   string `json:"requirement"`
    Weight        int    `json:"weight"`
    Passed        bool   `json:"passed"`
    Evidence      string `json:"evidence"`   // Quote from transcript
    Reasoning     string `json:"reasoning"`
}

// Universal compliance checklist (any service business)
var UniversalComplianceChecklist = []ComplianceCheck{
    {
        Trigger:     "ALL",
        Requirement: "Agent must not mention services not in KB.MD",
        Weight:      5,
    },
    {
        Trigger:     "ALL",
        Requirement: "Agent must use correct contact information",
        Weight:      5,
    },
    {
        Trigger:     "ALL",
        Requirement: "Agent must follow conversation flow from SCRIPT.MD",
        Weight:      3,
    },
    {
        Trigger:     "escalation_signal",
        Requirement: "Agent must offer supervisor/escalation path when requested",
        Weight:      4,
    },
    {
        Trigger:     "lead_quality",
        Requirement: "Agent must attempt to collect name and contact info",
        Weight:      3,
    },
    {
        Trigger:     "negative_sentiment",
        Requirement: "Agent must acknowledge customer frustration empathetically",
        Weight:      3,
    },
    {
        Trigger:     "service_exclusion",
        Requirement: "Agent must not offer excluded services",
        Weight:      5,
    },
    {
        Trigger:     "ALL",
        Requirement: "Agent responses must be relevant to customer query",
        Weight:      4,
    },
}
```

---

### Phase 6: Learning Enforcement

Transform learnings from "hints" to "enforceable rules":

**Current (Weak):**
```
LEARNED IMPROVEMENTS (apply these specific behaviors):
- Ask for contact information earlier in conversation
```

**Proposed (Strong):**
```
MANDATORY BEHAVIORS (Enforced - Violations flagged):
┌────────────────────────────────────────────────────────────────┐
│ RULE L1: Ask for customer name within first 3 turns           │
│ RULE L2: Request phone number before discussing scheduling    │
│ RULE L3: Acknowledge urgency with empathetic language         │
└────────────────────────────────────────────────────────────────┘

These rules are AUDITED. Violations will be flagged for review.
```

**Learning Storage Enhancement:**

```go
type LearnedBehavior struct {
    ID            string `json:"id"`
    Content       string `json:"content"`
    Type          string `json:"type"`           // "rule", "reminder", "constraint"
    Enforcement   string `json:"enforcement"`    // "strict", "advisory"
    CheckMethod   string `json:"check_method"`   // "keyword", "turn_count", "llm"
    CheckParams   string `json:"check_params"`   // JSON params for check
    Source        string `json:"source"`
    AddedAt       string `json:"added_at"`
    IsActive      bool   `json:"is_active"`
}
```

**Check Method Examples:**

| Learning | Check Method | Check Params |
|----------|-------------|--------------|
| "Ask for name within 3 turns" | `turn_count` | `{"field": "name", "max_turn": 3}` |
| "Always acknowledge urgency" | `keyword` | `{"triggers": ["urgent"], "required": ["understand"]}` |
| "Don't promise specific times" | `llm` | `{"pattern": "time commitment", "forbidden": true}` |

---

## Implementation Roadmap

### Milestone 1: Strengthened System Prompt (Quick Win)

| Task | Effort | Impact |
|------|--------|--------|
| Reorder system prompt (constraints first) | 1 hour | High |
| Add explicit "DO NOT invent" instructions | 30 min | High |
| Add authoritative contact section | 30 min | Medium |

### Milestone 2: Output Sanitization

| Task | Effort | Impact |
|------|--------|--------|
| Create `sanitizer.go` | 2 hours | Medium |
| Add sanitization to response flow | 1 hour | Medium |
| Test with hallucination scenarios | 2 hours | High |

### Milestone 3: Compliance Verification Layer

| Task | Effort | Impact |
|------|--------|--------|
| Create `compliance.go` | 4 hours | High |
| Service mention extraction | 2 hours | High |
| Contact info validation | 1 hour | High |
| Integration with chat flow | 2 hours | High |

### Milestone 4: Heuristic Scoring Categories

| Task | Effort | Impact |
|------|--------|--------|
| Create `agent_scoring_config` table | 1 hour | Medium |
| Implement keyword-based scoring (Tier 1) | 3 hours | Medium |
| Implement LLM scoring (Tier 2) | 4 hours | Medium |
| Add scoring to conversation state | 2 hours | Medium |

### Milestone 5: Compliance Audit

| Task | Effort | Impact |
|------|--------|--------|
| Create `agent_compliance_audits` table | 1 hour | Medium |
| Implement async audit runner | 3 hours | Medium |
| Build audit report UI in Agent Admin | 4 hours | Medium |

### Milestone 6: Learning Enforcement

| Task | Effort | Impact |
|------|--------|--------|
| Extend `LearnedBehavior` schema | 1 hour | Low |
| Implement check methods | 4 hours | High |
| Add enforcement to compliance layer | 2 hours | High |

---

## Database Schema Changes

### New Table: `agent_scoring_config`

```sql
CREATE TABLE agent_scoring_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    version TEXT NOT NULL DEFAULT '1.0',
    config TEXT NOT NULL,  -- JSON configuration
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id)
);
```

### New Table: `agent_compliance_audits`

```sql
CREATE TABLE agent_compliance_audits (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    conversation_id TEXT NOT NULL,
    conversation_type TEXT NOT NULL,  -- 'simulation' or 'chat'
    score INTEGER NOT NULL,           -- 0-100
    checks TEXT NOT NULL,             -- JSON array of check results
    overall_passed BOOLEAN NOT NULL,
    audited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id)
);

CREATE INDEX idx_audit_tenant ON agent_compliance_audits(tenant_id);
CREATE INDEX idx_audit_conversation ON agent_compliance_audits(conversation_id);
CREATE INDEX idx_audit_score ON agent_compliance_audits(score);
```

### Modify: `agent_learning_memory`

Add columns to `learned_behaviors` JSON objects:
- `enforcement`: "strict" | "advisory"
- `check_method`: "keyword" | "turn_count" | "llm"
- `check_params`: JSON string

---

## Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `server/router/api/v1/agent/compliance.go` | Create | Compliance verification |
| `server/router/api/v1/agent/sanitizer.go` | Create | Output sanitization |
| `server/router/api/v1/agent/audit.go` | Create | Post-conversation audit |
| `server/router/api/v1/agent/scoring.go` | Create | Heuristic scoring |
| `server/router/api/v1/agent/service.go` | Modify | Reorder system prompt |
| `store/agent.go` | Modify | Add new types |
| `store/db/sqlite/agent.go` | Modify | Add new queries |
| `store/db/sqlite/migration/prod/XXXX.sql` | Create | Schema changes |

---

## Comparison: Before vs After

| Aspect | Current | Proposed |
|--------|---------|----------|
| Hallucination prevention | None | Multi-layer verification |
| Constraint enforcement | Weak (end of prompt) | Strong (top of prompt + verification) |
| Output sanitization | None | Regex + pattern removal |
| Learning enforcement | Advisory only | Auditable rules |
| Compliance tracking | None | Per-conversation audit |
| Scoring categories | None | 6 universal categories |
| Fail-safe behavior | None | Graceful degradation |

---

## Expected Outcomes

After implementation:

1. **Hallucinations reduced by 90%+**
   - Services must be in KB.MD
   - Contacts must be in config
   - Processes must be in SCRIPT.MD

2. **Policy compliance verified**
   - Each response checked before sending
   - Violations flagged or blocked

3. **Learnings actually enforced**
   - Not just hints, but auditable rules
   - Violations tracked and reported

4. **Audit trail for QA**
   - Every conversation scored
   - Compliance reports in Agent Admin

---

## Summary

```
┌────────────────────────────────────────────────────────────────┐
│  CURRENT STATE                                                 │
│  • Learnings = weak hints at end of prompt                    │
│  • No verification of agent responses                         │
│  • Hallucinations go undetected                               │
│  • No compliance tracking                                     │
├────────────────────────────────────────────────────────────────┤
│  PROPOSED STATE (Adapted from OPUS13)                         │
│  • Constraints at TOP of prompt (highest priority)            │
│  • Multi-layer verification before response sent              │
│  • Output sanitization removes hallucinated content           │
│  • Learnings become enforceable rules                         │
│  • Per-conversation compliance audit                          │
│  • Universal scoring categories for any service business      │
└────────────────────────────────────────────────────────────────┘
```

**Key Principle from OPUS13:** Trust but verify. The LLM generates responses, but deterministic checks ensure compliance with source content.

---
---

# Addendum 8: Transcript Analysis - Emergency Water Damage Simulation

## Overview

Analysis of simulation transcript `sim-0d1e929f-8a2a-48ab-af7c-af5febae70c0` to evaluate the effectiveness of the implemented compliance milestones and identify remaining gaps.

| Field | Value |
|-------|-------|
| **Transcript ID** | sim-0d1e929f-8a2a-48ab-af7c-af5febae70c0 |
| **Tenant** | Bulovas Restorations (ID: 1) |
| **Scenario** | Tree crashed through roof during storm, water pouring in, ceiling sagging |
| **Turns** | 11 |
| **End Reason** | User stopped |

---

## Critical Issues Found

### Issue 1: HALLUCINATED PHONE NUMBER (CRITICAL)

| Aspect | Details |
|--------|---------|
| **Severity** | CRITICAL |
| **Root Cause** | LLM generated placeholder contact info |
| **KB.MD Correct Number** | `(631) 358-5200` |
| **Hallucinated Number** | `(555) 000-0000` |
| **Pattern** | First response correct, subsequent responses hallucinated |

**Evidence from transcript:**

Turn 1 (CORRECT):
```
"Then call our emergency line at (631) 358-5200 so we can dispatch a crew right away."
```

Turn 2+ (HALLUCINATED):
```
"You can reach our emergency response team now at (555) 000-0000."
"Keep our emergency line handy: (555) 000-0000."
```

**Impact:** Customer given fake phone number for emergency situation. If this were a real call, customer would be unable to reach the company.

---

### Issue 2: CONTEXT AMNESIA / MEMORY LOSS (HIGH)

| Aspect | Details |
|--------|---------|
| **Severity** | HIGH |
| **Root Cause** | LLM not retaining conversation context |
| **Times Info Requested** | Customer provided info 4+ times |

**Timeline:**

| Turn | Customer Action | Agent Response |
|------|-----------------|----------------|
| 2 | Provided name, phone, address | ✓ Acknowledged |
| 6 | - | ❌ "I need: 1. Your full name 2. Phone number..." |
| 7 | "I already gave you my name, phone and address" | Agent apologized but still asked |
| 8 | Provided zip code again | ❌ "Please provide: 1. Your full name 2. Phone..." |
| 9 | Repeated name and phone | Finally acknowledged |

**Impact:** Frustrating customer experience. In a real emergency, this delay could be dangerous.

---

### Issue 3: REPETITIVE SAFETY WARNINGS (MEDIUM)

| Aspect | Details |
|--------|---------|
| **Severity** | MEDIUM |
| **Root Cause** | System prompt emphasizes safety without context awareness |
| **Occurrences** | Nearly identical safety block in 11/11 responses |

**Example (repeated in every response):**
```
"Your safety comes first. If the ceiling feels unstable, the roof is still exposed,
or you see any live wires, move to a safe location such as a neighbor's house
and call 911 if anything looks like it could collapse."
```

**Impact:** Customer became frustrated asking the same questions repeatedly while agent kept giving safety warnings instead of direct answers.

---

### Issue 4: NO ESCALATION TO LIVE AGENT (MEDIUM)

| Aspect | Details |
|--------|---------|
| **Severity** | MEDIUM |
| **Root Cause** | No escalation logic for high-urgency emergencies |
| **Context** | Tree through roof, active water intrusion, sagging ceiling |

**Expected behavior:** After 3-4 turns of high urgency, offer to transfer to live dispatcher.

**Actual behavior:** Never offered live agent option despite clear emergency.

---

## What Worked Well

| Aspect | Rating | Notes |
|--------|--------|-------|
| Intent Recognition | ✅ GOOD | Correctly identified `emergency_water` |
| Urgency Detection | ✅ GOOD | Assigned urgency level 5 (highest) |
| Safety-First Approach | ✅ GOOD | Prioritized safety instructions |
| Service Alignment | ✅ GOOD | Offered appropriate services |
| First Response Quality | ✅ EXCELLENT | Comprehensive, correct contact info |
| Policy Compliance (partial) | ✅ GOOD | Mentioned 30-min response, free estimates |

---

## Milestone Effectiveness Analysis

### Milestone 1: Strengthen System Prompt

| Check | Status | Notes |
|-------|--------|-------|
| Safety-first ordering | ✅ Working | Agent leads with safety |
| DO NOT invent instruction | ❌ NOT EFFECTIVE | Agent still hallucinated phone number |
| Constraints at top | ⚠️ Partial | Not preventing context loss |

**Verdict:** System prompt reordering helped with safety-first but did not prevent hallucinations.

---

### Milestone 2: Output Sanitizer

| Check | Status | Notes |
|-------|--------|-------|
| Remove markdown | ✅ Working | No markdown in output |
| Remove system tags | ✅ Working | No leaked system tags |
| Detect hallucinated contacts | ❌ NOT IMPLEMENTED | Should catch `(555)` pattern |

**Gap:** Sanitizer does not validate contact information against KB.MD.

---

### Milestone 3: Compliance Verification

| Check | Status | Notes |
|-------|--------|-------|
| Service verification | Not tested | Services mentioned were valid |
| Contact verification | ❌ FAILED | Hallucinated phone passed through |
| Exclusion checking | Not tested | No exclusions discussed |

**Gap:** `VerifyResponseCompliance` does not validate phone numbers against `config.Tenant.EmergencyPhone`.

---

### Milestone 4: Heuristic Scoring

| Check | Status | Notes |
|-------|--------|-------|
| Urgency scoring | ✅ Working | Correctly scored as 5 |
| Intent detection | ✅ Working | Correctly identified emergency_water |
| Service matching | ✅ Working | Water damage services matched |

**Verdict:** Scoring is working correctly.

---

### Milestone 5: Compliance Audit

| Check | Status | Notes |
|-------|--------|-------|
| Post-conversation audit | Not executed | Would have flagged issues |

**Note:** Audit system would catch these issues but wasn't run on this transcript.

---

### Milestone 6: Learning Enforcement

| Check | Status | Notes |
|-------|--------|-------|
| Learned behaviors | N/A | No learned behaviors configured |

---

## Root Cause Analysis

### Why did hallucination occur after correct first response?

**Hypothesis 1: Context Window Pressure**
- Long conversation (11 turns) may have pushed system prompt out of effective context
- LLM fell back to generic patterns ("555" is common placeholder)

**Hypothesis 2: Instruction Dilution**
- System prompt has many instructions
- "DO NOT invent" instruction may be too weak/general

**Hypothesis 3: No Verification Loop**
- Response generated → sent directly
- No check against known contact info

---

## Proposed Fixes

### Fix 1: Contact Information Verification (P0 - CRITICAL)

Add phone number validation to `compliance.go`:

```go
// verifyContactInfo checks all phone numbers in response against authorized contacts
func (c *ComplianceChecker) verifyContactInfo(response string, config *AudienceConfig) []ComplianceViolation {
    var violations []ComplianceViolation

    // Extract all phone numbers from response
    phonePattern := regexp.MustCompile(`\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
    foundPhones := phonePattern.FindAllString(response, -1)

    // Known authorized phones from config
    authorizedPhones := []string{
        normalizePhone(config.Tenant.EmergencyPhone),
        normalizePhone(config.Tenant.MainPhone),
    }

    for _, found := range foundPhones {
        normalized := normalizePhone(found)
        if !containsPhone(authorizedPhones, normalized) {
            // Check for obvious placeholder patterns
            if strings.Contains(normalized, "5550") || strings.Contains(normalized, "0000") {
                violations = append(violations, ComplianceViolation{
                    Type:        ViolationUnauthorizedContact,
                    Severity:    "critical",
                    Description: fmt.Sprintf("Hallucinated phone number detected: %s", found),
                    Suggestion:  fmt.Sprintf("Replace with authorized number: %s", config.Tenant.EmergencyPhone),
                })
            }
        }
    }

    return violations
}
```

---

### Fix 2: Contact Auto-Correction in Sanitizer (P0 - CRITICAL)

Add auto-correction to `sanitizer.go`:

```go
// CorrectContacts replaces hallucinated placeholder phones with the correct emergency number
func (s *Sanitizer) CorrectContacts(response string, emergencyPhone string) string {
    if emergencyPhone == "" {
        return response
    }

    // Common placeholder patterns that indicate hallucination
    placeholderPatterns := []string{
        `\(555\)\s*\d{3}[-.\s]?\d{4}`,        // (555) XXX-XXXX
        `555[-.\s]?\d{3}[-.\s]?\d{4}`,         // 555-XXX-XXXX
        `\(000\)\s*\d{3}[-.\s]?\d{4}`,        // (000) XXX-XXXX
        `000[-.\s]?\d{3}[-.\s]?\d{4}`,         // 000-XXX-XXXX
        `\(123\)\s*456[-.\s]?7890`,            // (123) 456-7890 (common test)
        `\(999\)\s*\d{3}[-.\s]?\d{4}`,        // (999) XXX-XXXX
    }

    for _, pattern := range placeholderPatterns {
        re := regexp.MustCompile(pattern)
        response = re.ReplaceAllString(response, emergencyPhone)
    }

    return response
}
```

---

### Fix 3: Context Reinforcement at End of Prompt (P1)

Add to `buildSystemPrompt` in `service.go`:

```go
// At the END of system prompt (reinforcement anchor)
sb.WriteString("\n=== CRITICAL REMINDER (DO NOT IGNORE) ===\n")
sb.WriteString(fmt.Sprintf("EMERGENCY PHONE: %s (USE ONLY THIS NUMBER)\n", config.Tenant.EmergencyPhone))
if config.Tenant.TollFreePhone != "" {
    sb.WriteString(fmt.Sprintf("TOLL-FREE: %s\n", config.Tenant.TollFreePhone))
}
sb.WriteString("NEVER use placeholder numbers like (555), (000), or (123).\n")
sb.WriteString("If you don't know a phone number, say 'Please call us directly' - DO NOT invent one.\n")
```

---

### Fix 4: Track Already-Provided Customer Info (P1)

Add context extraction to `processChat` in `service.go`:

```go
// extractCollectedInfo scans conversation history for customer-provided information
func extractCollectedInfo(messages []Message) map[string]string {
    collected := make(map[string]string)

    namePattern := regexp.MustCompile(`(?i)(?:I'm|I am|my name is|this is)\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+)?)`)
    phonePattern := regexp.MustCompile(`\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
    addressPattern := regexp.MustCompile(`\d+\s+[A-Za-z]+(?:\s+[A-Za-z]+)*(?:,\s*[A-Za-z\s]+)?(?:,?\s*[A-Z]{2}\s*\d{5})`)

    for _, msg := range messages {
        if msg.Role == "user" {
            if match := namePattern.FindStringSubmatch(msg.Content); len(match) > 1 {
                collected["name"] = match[1]
            }
            if match := phonePattern.FindString(msg.Content); match != "" {
                // Exclude the tenant's own phone numbers
                collected["phone"] = match
            }
            if match := addressPattern.FindString(msg.Content); match != "" {
                collected["address"] = match
            }
        }
    }

    return collected
}

// In buildSystemPrompt, after conversation history:
if len(collectedInfo) > 0 {
    sb.WriteString("\n=== CUSTOMER INFO ALREADY PROVIDED (DO NOT ASK AGAIN) ===\n")
    for key, val := range collectedInfo {
        sb.WriteString(fmt.Sprintf("- %s: %s\n", strings.Title(key), val))
    }
    sb.WriteString("IMPORTANT: The customer has already given this info. Do not ask for it again.\n")
}
```

---

### Fix 5: Escalation Logic for High-Urgency Emergencies (P2)

Add escalation trigger in `processChat`:

```go
// Check if we should offer escalation
func shouldOfferEscalation(session *AgentSession, metadata *MessageMetadata, messages []Message) bool {
    // Only for high-urgency situations
    if metadata.Urgency < 4 {
        return false
    }

    // After at least 4 turns
    if session.TurnCount < 4 {
        return false
    }

    // Check for repetition patterns (customer asking same thing multiple times)
    if detectRepetitionPattern(messages) {
        return true
    }

    return false
}

// In processChat, after generating response:
if shouldOfferEscalation(session, metadata, messages) && !session.EscalationOffered {
    response += "\n\nWould you like me to connect you with a live dispatcher? They can provide more specific timing and coordinate directly with the crew."
    session.EscalationOffered = true
}
```

---

## Implementation Priority

| Fix | Priority | Effort | Impact | Status |
|-----|----------|--------|--------|--------|
| Fix 1: Contact Verification | P0 | Medium | Prevents wrong phone numbers | TO DO |
| Fix 2: Contact Auto-Correct | P0 | Low | Auto-fixes hallucinations | TO DO |
| Fix 3: Context Reinforcement | P1 | Low | Reduces drift over turns | TO DO |
| Fix 4: Context Retention | P1 | Medium | Prevents re-asking for info | TO DO |
| Fix 5: Escalation Logic | P2 | Medium | Better emergency handling | Future |

---

## Verification Test Cases

After implementing fixes, these scenarios should pass:

### Test Case 1: Phone Number Validation
```
Input: Agent response containing "(555) 000-0000"
Expected: Phone auto-corrected to "(631) 358-5200"
```

### Test Case 2: Compliance Violation Flag
```
Input: Agent response with unauthorized phone number
Expected: ComplianceViolation with type=ViolationUnauthorizedContact, severity=critical
```

### Test Case 3: Context Retention
```
Turn 2: Customer provides name "Alex Martinez", phone "516-555-0198"
Turn 6: Agent should NOT ask for name or phone again
Expected: System prompt includes "ALREADY PROVIDED: Name=Alex Martinez, Phone=516-555-0198"
```

### Test Case 4: Escalation Trigger
```
Scenario: Emergency with urgency 5, customer asked same question 3+ times
Turn 5: Agent should offer live agent transfer
Expected: Response includes "Would you like me to connect you with a live dispatcher?"
```

---

## Summary

The implemented milestones provide a foundation but have critical gaps:

| Current State | Required State |
|---------------|----------------|
| Phone numbers not validated | All phones checked against KB.MD |
| Placeholder phones pass through | Placeholder patterns auto-replaced |
| Context drifts over long conversations | Key info tracked and reinforced |
| No escalation logic | High-urgency emergencies escalate |

**Key Insight:** The first response was excellent. The problem is maintaining quality over multiple turns as context window fills and instruction weight diminishes. The LLM "forgets" the correct phone number and falls back to common patterns like `(555)`.

**Root Solution:** Deterministic verification layer that catches and corrects hallucinations before they reach the customer. Trust but verify.

---
---

# Addendum 9: Learning System Effectiveness Analysis

## Overview

Analysis of whether applied learnings from transcript analysis actually improved agent behavior in subsequent conversations.

**Question:** Were the issues found and suggestions applied from analysis of transcript sim-e1368e95 effective in improving the agent's behavior?

**Answer:** **NO.** The learnings were stored correctly but had no observable effect on agent behavior.

---

## Timeline Evidence

| Time | Event | Details |
|------|-------|---------|
| 16:47:13 | **sim-e1368e95** created | "Tree fell through roof, rain coming in, ceiling sagging" |
| 16:48:30 | **ana-23f6111d** created | Analysis of sim-e1368e95, Score: **47 (F)** |
| 16:49:54 | **Learning memory updated** | Issues and suggestions from analysis applied to `agent_learning_memory` |
| 16:50:21 | **sim-0d1e929f** created | Same scenario: "Tree crashed through roof" - ran AFTER learnings applied |

**Key Finding:** Transcript sim-0d1e929f ran **27 seconds AFTER** learnings were applied, yet exhibited the **SAME issues**.

---

## Learnings Applied (from agent_learning_memory)

### Issues Captured and Marked Active

| Issue | Source | Status |
|-------|--------|--------|
| "Provided incorrect emergency phone number (555-000-0000)" | ana-23f6111d | ✅ is_active: true |
| "Stated company can send crew to Springfield (outside service area)" | ana-23f6111d | ✅ is_active: true |
| "Failed to collect required information after safety advice" | ana-23f6111d | ✅ is_active: true |

### Suggestions Applied and Marked Active

| Suggestion | Source | Status |
|------------|--------|--------|
| "Use correct emergency contact numbers from KB in every response" | ana-23f6111d | ✅ is_active: true |
| "Verify caller's location before promising dispatch" | ana-23f6111d | ✅ is_active: true |
| "After safety advice, follow prescribed flow: name, phone, address, damage type, timing" | ana-23f6111d | ✅ is_active: true |
| "Maintain clear linear conversation flow: safety → verification → referral → closing" | ana-df671589 | ✅ is_active: true |

---

## Evidence of Ineffectiveness

### sim-0d1e929f Behavior (AFTER learnings were applied)

| Issue | Learning Said | What Agent Actually Did | Effective? |
|-------|---------------|-------------------------|------------|
| **Phone number** | "Use correct emergency contacts from KB" | Used `(555) 000-0000` from Turn 2 onward (correct in Turn 1 only) | ❌ **NO** |
| **Info collection** | "Follow prescribed flow: name, phone, address" | Asked for same info **4+ times** after customer provided | ❌ **NO** |
| **Conversation flow** | "Clear linear flow" | Repeated identical safety block in **11/11 responses** | ❌ **NO** |

---

## Why Learnings Failed

### Current Architecture

```
┌─────────────────────────────────────────────────────────┐
│ SYSTEM PROMPT                                           │
│                                                         │
│ [Section 1: Critical Constraints]                       │
│ [Section 2: Identity]                                   │
│ [Section 3: Services]                                   │
│ ...                                                     │
│ [Section 7: LEARNED BEHAVIORS]  ← Injected here        │
│   • "Use correct emergency contacts"                    │
│   • "Follow prescribed flow"                           │
│   • "Clear linear conversation flow"                   │
│ ...                                                     │
│ [Section 10: Response Format]                           │
└─────────────────────────────────────────────────────────┘
                    │
                    ▼
            ┌───────────────┐
            │   LLM Call    │
            └───────────────┘
                    │
                    ▼
            ┌───────────────┐
            │   Response    │  ← Goes directly to customer
            └───────────────┘       NO VERIFICATION!
                    │
                    ▼
               [Customer]
```

### Root Causes of Failure

| Problem | Description |
|---------|-------------|
| **Soft hints, not rules** | Learnings are advisory text in the prompt, not enforceable constraints |
| **No enforcement mechanism** | LLM can (and does) ignore advisory text |
| **No verification layer** | Response goes directly to customer without checking compliance |
| **Context dilution** | Over 11 turns, system prompt instructions lose weight as conversation history grows |
| **Recency bias** | LLM prioritizes recent conversation content over instructions from beginning of context |

---

## Proposed Solution: Verification Layer

```
┌─────────────────────────────────────────────────────────┐
│ SYSTEM PROMPT (with learnings as hints)                 │
└─────────────────────────────────────────────────────────┘
                    │
                    ▼
            ┌───────────────┐
            │   LLM Call    │
            └───────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│ VERIFICATION LAYER (NEW - DETERMINISTIC)                │
│                                                         │
│ 1. Contact Verification                                 │
│    • Check all phone numbers against authorized list   │
│    • Auto-replace placeholder patterns (555, 000)      │
│                                                         │
│ 2. Learning Enforcement                                 │
│    • For each active learning, verify compliance       │
│    • Log violations for audit                          │
│                                                         │
│ 3. Context Retention                                    │
│    • Track customer-provided info (name, phone, addr)  │
│    • Inject reminder into next prompt                  │
│                                                         │
│ 4. Response Sanitization                                │
│    • Remove hallucinated content                       │
│    • Ensure format compliance                          │
└─────────────────────────────────────────────────────────┘
                    │
                    ▼
               [Customer]  ← Now receives verified response
```

---

## Implementation: Option C (Hybrid Approach)

Per user feedback, implement general-purpose verification that works with or without tenant configuration:

### Tier 1: Always Active (No Config Required)

| Check | Pattern | Action |
|-------|---------|--------|
| Placeholder phones | `(555)`, `(000)`, `(123) 456-7890`, `(999)` | Flag as violation |
| Obvious test data | `test@example.com`, `John Doe`, `123 Main St` | Flag as potential hallucination |

### Tier 2: Tenant-Enhanced (When Config Available)

| Check | Source | Action |
|-------|--------|--------|
| Authorized phones | `config.Tenant.EmergencyPhone` | Validate all phones against list |
| Authorized emails | `config.Tenant.Email` | Validate email addresses |
| Service area | `config.Coverage` | Verify locations mentioned |

### Behavior Matrix

| Scenario | Tier 1 | Tier 2 | Result |
|----------|--------|--------|--------|
| Tenant has EmergencyPhone configured | ✅ | ✅ | Full validation + auto-correct |
| Tenant has no contacts configured | ✅ | ❌ | Placeholder detection only |
| Phone `(555) 000-0000` detected | Flag | Replace with correct | Auto-corrected or flagged |
| Phone `(631) 555-1234` detected | Pass | Validate against list | Pass if authorized, flag if not |

---

## Conclusion

**The learning system stores correct information but cannot enforce it.**

The agent "knows" (via `learned_behaviors`) that it should:
- Use correct phone numbers
- Not re-ask for information already provided
- Follow linear conversation flow

But without a verification layer, these are just suggestions that the LLM ignores under context pressure.

### Before vs After

| Aspect | Current (Learnings Only) | Proposed (Learnings + Verification) |
|--------|--------------------------|-------------------------------------|
| Phone validation | ❌ Advisory only | ✅ Deterministic check + auto-correct |
| Context retention | ❌ LLM memory | ✅ Explicit tracking + injection |
| Flow enforcement | ❌ Advisory only | ✅ State machine tracking |
| Effectiveness | **0% observed** | **Expected 90%+** |

**Required Action:** Implement P0 fixes (Contact Verification + Auto-Correct) using Option C hybrid approach to transform learnings from ineffective hints into enforced rules.

---
---

# Addendum 10: Transcript 14 Analysis & Source File Version Control

## Overview

Analysis of the most recent simulation transcript to evaluate:
1. Effectiveness of implemented compliance fixes
2. How the system handles KB.MD, POLICY.MD, SCRIPT.MD versioning
3. Required surgical fixes to source files

---

## Transcript 14 Summary

| Field | Value |
|-------|-------|
| **Tenant** | Bulovas Restorations (ID: 1) |
| **Scenario** | Tree crashed through roof, water pouring in, ceiling sagging |
| **Customer** | Mark Rivera, 742 Oak Lane, Riverhead, Suffolk County 11901 |
| **Phone** | 516-555-1234 |
| **Turns** | 15 |
| **End Reason** | User stopped |
| **Created** | 2026-01-20 17:24:10 |

---

## Critical Issues Found

### Issue 1: PHONE NUMBER STILL HALLUCINATED (CRITICAL)

| Aspect | Details |
|--------|---------|
| **KB.MD Correct Number** | `(631) 358-5200` |
| **Agent Used** | `(555) 000-0000` (all 15 turns) |
| **Root Cause** | Fixes implemented but not yet built/deployed |

**Evidence:**
- Turn 1: "Please call our emergency line right away at (555) 000-0000."
- Turn 15: "please call our emergency line at (555) 000-0000."

---

### Issue 2: CONTEXT AMNESIA - NAME CONFUSION (HIGH)

| Aspect | Details |
|--------|---------|
| **Severity** | HIGH |
| **Customer's Actual Name** | "Mark Rivera" (provided in Turn 2) |
| **Agent's Interpretation** | "already flooding" (from Turn 1 text) |

**Evidence:**
- Turn 2: Customer says "my name's Mark Rivera"
- Turn 10: Agent says "I have the name you gave ('already flooding')"
- Turn 15: Agent says "We have your name (already flooding)"

**Root Cause:** The context retention fix extracts names using patterns like "I'm X" but the agent's initial response to Turn 1 may have cached incorrect info.

---

### Issue 3: CONTEXT AMNESIA - REPEATED QUESTIONS (HIGH)

| Turn | Customer Provided | Agent Asked Again |
|------|-------------------|-------------------|
| 2 | Phone: 516-555-1234 | - |
| 6 | - | "could you please provide a phone number" |
| 10 | - | "could you tell me the city or zip code" (already given) |
| 14 | - | "could you please confirm the best phone number" |

---

### Issue 4: MISSING INFORMATION - PAYMENT/PRICING (MEDIUM)

Customer asked about payment methods and pricing multiple times. Agent responded:
- "I'm not certain about the specific payment methods we accept"
- "I'm not certain... whether an upfront deposit is required"

**Root Cause:** KB.MD does not contain payment method information.

---

## What Worked Well

| Aspect | Status | Notes |
|--------|--------|-------|
| Intent Recognition | ✅ GOOD | Correctly identified `emergency_water` with urgency 5 |
| Safety-First | ✅ GOOD | Led with safety instructions in every response |
| 30-Min Response | ✅ GOOD | Correctly mentioned 30-minute response time |
| Insurance Handling | ✅ GOOD | Correctly stated "we work with all insurance carriers" |
| Free Estimate | ✅ GOOD | Mentioned free estimate multiple times |
| Service Area | ✅ GOOD | Correctly validated Riverhead is in Suffolk County |

---

## Source File Version Control Analysis

### Current System Architecture

```
agent_source_files table:
┌─────────────┬───────────────┬───────────┬──────────────┬─────────────────┐
│ tenant_id   │ audience_type │ file_type │ content_hash │ imported_at     │
├─────────────┼───────────────┼───────────┼──────────────┼─────────────────┤
│ 1           │ external      │ kb        │ a28a253c...  │ 2026-01-19 01:21│
│ 1           │ external      │ kb        │ a28a253c...  │ 2026-01-19 01:20│  ← Duplicate
│ 1           │ external      │ policy    │ 5b502bb4...  │ 2026-01-19 01:21│
│ 1           │ internal      │ kb        │ a28a253c...  │ 2026-01-19 01:22│
│ 1           │ internal      │ policy    │ 5b502bb4...  │ 2026-01-19 01:22│
└─────────────┴───────────────┴───────────┴──────────────┴─────────────────┘
```

### Findings

| Aspect | Current State | Issue |
|--------|---------------|-------|
| **Version Column** | ❌ Does not exist | No explicit version tracking |
| **Version Control** | By `imported_at` timestamp | Relies on row ordering |
| **Deduplication** | ❌ None | Same content_hash stored multiple times |
| **SCRIPT.MD Storage** | ❌ Not stored | `agent_tenant_script` table doesn't exist |
| **File Selection** | Most recent by `imported_at` | Works but fragile |

### How Version Selection Currently Works

```sql
-- In ListAgentSourceFiles (store/db/sqlite/agent.go):
SELECT * FROM agent_source_files
WHERE tenant_id = ? AND audience_type = ? AND file_type = ?
ORDER BY imported_at DESC
LIMIT 1
```

**Answer to User's Question:** The system uses the most recently imported file based on `imported_at` timestamp. There is no explicit version number or version control - just timestamp-based "latest wins."

---

## SCRIPT.MD Integration Status

| Aspect | Status |
|--------|--------|
| File exists in `docs/SCRIPT.MD` | ✅ Yes |
| Database table `agent_tenant_script` | ❌ Does not exist |
| Integrated into system prompt | ❌ No |
| Used for conversation flow | ❌ No |

**Impact:** The conversation flow guidance in SCRIPT.MD is not being used by the agent.

---

## Proposed Surgical Fixes to Source Files

### KB.MD - Add Missing Payment Information

**Location:** After the Contact Information section

**Add:**
```markdown
---

## Payment & Billing

### Payment Methods Accepted
<!-- @payment_methods -->

We accept the following payment methods:
- All major credit cards (Visa, MasterCard, American Express, Discover)
- Personal and business checks
- Insurance direct billing (we work with all carriers)
- Financing options available for larger projects

### Deposit & Payment Terms
<!-- @payment_terms -->

- **Emergency Services:** No upfront deposit required. Work begins immediately.
- **Standard Projects:** Deposit may be required after estimate approval (typically 25-50%)
- **Insurance Claims:** We bill insurance directly; you pay only your deductible.
- **Free Estimates:** On-site assessment and written estimate at no charge.

### Pricing Note

We do not provide phone/chat price estimates. Every property damage situation is unique, and accurate pricing requires an on-site inspection. Our certified technicians will provide a detailed, written estimate after inspection - this estimate is always free.
```

### KB.MD - Add Toll-Free to Emergency Section

**Current:**
```markdown
| **Emergency Phone** | **(631) 358-5200** |
| **Toll-Free** | 855-DRYITNOW (855-379-4866) |
```

**Observation:** Both numbers are in KB.MD, but agent only mentions (555) placeholder. After fixes are deployed, this should resolve.

---

### POLICY.MD - Add Pricing Question Intent

**Location:** After the existing intents section

**Add:**
```markdown
#### Pricing Question
<!-- @intent: pricing_question, category: standard, urgency: 1, action: standard_flow -->

**Description:**
Customer asking about costs, prices, or estimates before booking.

**Required Response Pattern:**
1. Acknowledge the question
2. Explain we provide FREE on-site estimates
3. Explain why we can't give phone/chat quotes (every situation is unique)
4. Offer to schedule the free inspection

**Key Phrases to Use:**
- "We provide free, no-obligation estimates"
- "Every property damage situation is unique"
- "A certified technician will give you a detailed written estimate"
- "The estimate is always free"

**DO NOT:**
- Give any ballpark numbers
- Say "a few hundred" or "a couple thousand"
- Make any price commitments

**Examples that MATCH:**
- "How much does water damage restoration cost?"
- "What's the price for mold removal?"
- "Can you give me a rough estimate?"
- "What's a ballpark for this kind of job?"
```

---

### POLICY.MD - Add Payment Question Intent

**Location:** After pricing_question intent

**Add:**
```markdown
#### Payment Question
<!-- @intent: payment_question, category: standard, urgency: 1, action: standard_flow -->

**Description:**
Customer asking about payment methods, deposits, or billing.

**Required Response:**
Refer to the Payment & Billing section in KB.MD:
- We accept all major credit cards, checks, and insurance direct billing
- No upfront deposit for emergency services
- Free estimates always
- We handle insurance paperwork

**Examples that MATCH:**
- "What payment methods do you accept?"
- "Do you take credit cards?"
- "Is there an upfront deposit?"
- "How does billing work with insurance?"
```

---

### SCRIPT.MD - Integration Recommendation

**Current Status:** SCRIPT.MD exists in `docs/` but is not integrated.

**Recommendation:** Before making changes to SCRIPT.MD, the system needs:
1. Create `agent_tenant_script` table (migration exists but may not be applied)
2. Add import handler for SCRIPT.MD
3. Integrate into `buildSystemPrompt`

**SCRIPT.MD should follow KB.MD and POLICY.MD** - it is a procedural guide, not a source of truth. The fixes above to KB.MD and POLICY.MD should be applied first, then SCRIPT.MD can reference them.

---

## Implementation Priority

| Fix | File | Priority | Effort | Impact |
|-----|------|----------|--------|--------|
| Add Payment Information | KB.MD | P0 | Low | Eliminates "I'm not certain" responses |
| Add Pricing Intent | POLICY.MD | P1 | Low | Standardizes pricing question handling |
| Add Payment Intent | POLICY.MD | P1 | Low | Standardizes payment question handling |
| Integrate SCRIPT.MD | Backend | P2 | Medium | Improves conversation flow |
| Add Version Control | Schema | P3 | Medium | Better file management |

---

## Verification After Fixes

### Test Scenario 1: Payment Question
```
Customer: "What payment methods do you accept?"
Expected: "We accept all major credit cards, checks, and insurance direct billing.
          No upfront deposit is required for emergency services."
```

### Test Scenario 2: Pricing Question
```
Customer: "Can you give me a ballpark for water damage restoration?"
Expected: "We provide free, no-obligation on-site estimates. Every situation is unique,
          so our certified technician will assess the damage and provide a detailed
          written estimate at no charge."
```

### Test Scenario 3: Phone Number (After Build)
```
Agent Response should contain: "(631) 358-5200"
Should NOT contain: "(555) 000-0000"
```

---

## Summary

### Transcript 14 Root Causes

| Issue | Root Cause | Fix |
|-------|------------|-----|
| Wrong phone number | Fixes not built/deployed | Build and deploy |
| Name confusion | Context retention not active | Build and deploy |
| Repeated questions | Context retention not active | Build and deploy |
| "I'm not certain" on payment | KB.MD missing info | Add Payment section |
| "I'm not certain" on pricing | No pricing guidance | Add Pricing intent |

### Version Control Answer

**Q: How does the system know which version to use?**

**A:** Currently, there is NO explicit version control. The system:
1. Stores multiple rows with the same `content_hash` and different `imported_at` timestamps
2. Selects the most recent file using `ORDER BY imported_at DESC LIMIT 1`
3. Does not track version numbers or maintain history

**Recommendation:** Add a `version` column to `agent_source_files` and implement proper version tracking with rollback capability.

### Next Steps

1. **Immediate:** Apply surgical fixes to KB.MD and POLICY.MD
2. **Short-term:** Build and deploy the compliance fixes
3. **Medium-term:** Integrate SCRIPT.MD into the system
4. **Long-term:** Implement proper version control for source files

---

# Addendum 11: Implementation Progress - Changes Made & Pending

## Overview

This addendum documents the actual code and source file changes made to address issues identified in Addendums 8-10.

---

## Completed Changes

### 1. Code: Contact Verification (compliance.go)

**File:** `server/router/api/v1/agent/compliance.go`

**Changes Made:**
- Added `isPlaceholderPhone()` function for Tier 1 detection (always active)
- Added `isPlaceholderEmail()` function for email placeholder detection
- Enhanced `checkContactInfo()` with Option C hybrid approach:
  - **Tier 1:** Always detect placeholder patterns (555, 000, 123-456-7890)
  - **Tier 2:** Validate against tenant config if available

**New Functions:**
```go
// isPlaceholderPhone checks if a normalized phone number is a common placeholder pattern.
// These patterns are ALWAYS flagged regardless of tenant configuration.
func (c *ComplianceChecker) isPlaceholderPhone(normalized string) bool {
    placeholderPatterns := []string{
        "5550", "5551",      // 555-0XXX, 555-1XXX (reserved for fiction)
        "0000",              // Contains 0000
        "1234567890",        // Sequential test number
        "9999",              // Contains 9999
        "1111111111",        // All 1s
        "0000000000",        // All 0s
    }
    // ... checks for patterns and 555 area code
}

// isPlaceholderEmail checks if an email is a common placeholder pattern.
func (c *ComplianceChecker) isPlaceholderEmail(email string) bool {
    // Detects example.com, test.com, placeholder@, john.doe@, etc.
}
```

---

### 2. Code: Contact Auto-Correction (sanitizer.go)

**File:** `server/router/api/v1/agent/sanitizer.go`

**Changes Made:**
- Added `CorrectContacts()` function to replace placeholder phones with correct number
- Added `CorrectContactsWithReport()` for detailed correction reporting
- Added `ContactCorrectionReport` struct
- Added `CorrectContactsInResponse()` convenience function

**Key Pattern Matching:**
```go
placeholderPatterns := []string{
    `\(555\)\s*\d{3}[-.\s]?\d{4}`,  // (555) XXX-XXXX
    `555[-.\s]\d{3}[-.\s]\d{4}`,     // 555-XXX-XXXX
    `\(000\)\s*\d{3}[-.\s]?\d{4}`,  // (000) XXX-XXXX
    `\(123\)\s*456[-.\s]?7890`,     // (123) 456-7890
    `\(999\)\s*\d{3}[-.\s]?\d{4}`,  // (999) XXX-XXXX
    // ... all zeros, all nines
}
```

**Behavior:**
- If `replacementPhone` is provided: Replace placeholders with correct number
- If `replacementPhone` is empty: Replace with `[phone number]` marker

---

### 3. Code: Context Reinforcement & Retention (service.go)

**File:** `server/router/api/v1/agent/service.go`

**Changes Made:**

#### A. Import Addition
```go
import (
    "regexp"  // Added for extractCollectedInfo
    // ... other imports
)
```

#### B. Section 0: Customer Info Already Provided
Added to beginning of system prompt to prevent re-asking for information:
```go
// SECTION 0: CUSTOMER INFO ALREADY PROVIDED
if session != nil && len(session.Messages) > 0 {
    collectedInfo := extractCollectedInfo(session.Messages, config.Audience.EmergencyPhone)
    if hasInfo {
        sb.WriteString("=== CUSTOMER INFO ALREADY PROVIDED (DO NOT ASK AGAIN) ===\n\n")
        // Inject: name, phone, address, damage type, timing
    }
}
```

#### C. Section 11: Critical Reminder
Added to END of system prompt for context reinforcement:
```go
// SECTION 11: CRITICAL REMINDER (DO NOT IGNORE)
sb.WriteString("=== CRITICAL REMINDER (DO NOT IGNORE) ===\n\n")
sb.WriteString("EMERGENCY PHONE: " + config.Audience.EmergencyPhone + " (USE ONLY THIS NUMBER)\n")
sb.WriteString("NEVER use placeholder numbers like (555), (000), or (123) 456-7890.\n")
```

#### D. extractCollectedInfo Function
New function to extract customer-provided information from conversation history:
```go
func extractCollectedInfo(messages []byte, correctPhone string) map[string]string {
    // Patterns for: name ("I'm X", "my name is X")
    // Phone numbers (validates against correct phone)
    // Address components
    // Returns map with extracted values
}
```

#### E. Response Post-Processing in processChat()
```go
// After LLM response:
response = SanitizeResponse(response)
if config.Audience != nil && config.Audience.EmergencyPhone != "" {
    response = CorrectContactsInResponse(response, config.Audience.EmergencyPhone)
}
```

---

### 4. Source File: KB.MD - Payment & Billing Section

**File:** `docs/templates/examples/restoration/KB.MD`

**Added Section:**
```markdown
## Payment & Billing

### Payment Methods Accepted
<!-- @faq: payment_methods -->

We accept the following payment methods:

| Payment Type | Details |
|--------------|---------|
| **Credit Cards** | Visa, MasterCard, American Express, Discover |
| **Checks** | Personal and business checks accepted |
| **Insurance** | Direct billing to insurance companies |
| **Financing** | Payment plans available for larger projects |

---

### Deposit & Payment Terms
<!-- @faq: payment_terms -->

| Service Type | Payment Terms |
|--------------|---------------|
| **Emergency Services** | No upfront deposit required - we start immediately |
| **Standard Projects** | 25-50% deposit may be required depending on scope |
| **Insurance Claims** | We bill your insurance directly - no out-of-pocket wait |
| **Free Estimates** | On-site assessment at no charge |

**Note:** We work with all insurance carriers and handle the paperwork for you.
```

**Impact:** Eliminates "I'm not certain about payment methods" responses.

---

### 5. Source File: POLICY.MD - Payment Question Intent

**File:** `docs/templates/examples/restoration/POLICY.MD`

**Added Intent:**
```markdown
#### Payment Question
<!-- @intent: payment_question, category: standard, urgency: 1, action: standard_flow -->

**Description:**
Questions about how to pay, payment methods accepted, deposits, or billing.

**Examples that MATCH:**
- "What payment methods do you accept?"
- "Can I pay with a credit card?"
- "Do you take checks?"
- "Is a deposit required?"
- "Do you offer financing?"
- "How does billing work with insurance?"
- "Do I need to pay upfront?"
- "Do you take insurance?"
```

**Also Updated:** Pricing Question intent examples for clarity (separated pricing from payment).

---

## Pending Changes

### P0 - Critical (Blocking Deployment)

| Item | File | Status | Notes |
|------|------|--------|-------|
| Build project | N/A | ⏳ Pending | All code changes need to be compiled |
| Test compliance fixes | N/A | ⏳ Pending | Run simulation after build |

### P1 - High Priority

| Item | File | Status | Notes |
|------|------|--------|-------|
| Re-import KB.MD to database | Admin UI | ⏳ Pending | Upload modified KB.MD via Agent Admin |
| Re-import POLICY.MD to database | Admin UI | ⏳ Pending | Upload modified POLICY.MD via Agent Admin |

### P2 - Medium Priority

| Item | File | Status | Notes |
|------|------|--------|-------|
| SCRIPT.MD table creation | Schema migration | ⏳ Pending | `agent_tenant_script` table doesn't exist |
| SCRIPT.MD import handler | handlers.go | ⏳ Pending | No upload handler for SCRIPT.MD |
| SCRIPT.MD integration | service.go | ⏳ Pending | Not included in buildSystemPrompt |

### P3 - Lower Priority

| Item | File | Status | Notes |
|------|------|--------|-------|
| Version control for source files | Schema | ⏳ Pending | Add `version` column |
| Deduplication by content_hash | agent.go | ⏳ Pending | Prevent duplicate rows |
| Version history UI | AgentAdmin.tsx | ⏳ Pending | Show file version history |

---

## Verification Checklist (After Build)

### Test 1: Phone Number Correction
```
Scenario: Emergency water damage
Expected: Agent uses (631) 358-5200
Should NOT appear: (555) 000-0000, (555) anything, (000) anything
```

### Test 2: Context Retention
```
Turn 1: "My basement is flooding, I'm John Smith"
Turn 3: Agent asks for something
Expected: Agent does NOT ask for name again
```

### Test 3: Payment Question
```
Customer: "What payment methods do you accept?"
Expected: "We accept all major credit cards, checks, and insurance direct billing."
Should NOT say: "I'm not certain"
```

### Test 4: Critical Reminder at Prompt End
```
System prompt should end with:
=== CRITICAL REMINDER (DO NOT IGNORE) ===
EMERGENCY PHONE: (631) 358-5200 (USE ONLY THIS NUMBER)
```

---

## Summary Table

| Component | Changes | Status |
|-----------|---------|--------|
| compliance.go | Tier 1/2 phone detection, placeholder patterns | ✅ Code written |
| sanitizer.go | Auto-correction of placeholder phones | ✅ Code written |
| service.go | Context retention, critical reminder | ✅ Code written |
| KB.MD | Payment & Billing section | ✅ File updated |
| POLICY.MD | Payment Question intent | ✅ File updated |
| Build | Compile all changes | ⏳ Pending |
| Database | Re-import KB.MD, POLICY.MD | ⏳ Pending |
| SCRIPT.MD | Table + Integration | ⏳ Not started |

---

# Addendum 12: Transcript 15 Analysis - Out-of-Coverage & Escalation Failures

## Overview

Analysis of transcript 15 (sim-ae2338c8) reveals critical gaps in:
1. Out-of-coverage conversation closure
2. Escalation/ticket handling
3. Safety instruction repetition
4. Context retention

---

## Transcript 15 Summary

| Field | Value |
|-------|-------|
| **ID** | sim-ae2338c8-e67c-40cc-abf8-4a073af2c49a |
| **Scenario** | Tree crashed through roof, water pouring in |
| **Customer** | Jamie Miller, 742 Maple St, Oakridge, WA |
| **Phone** | 555-123-4567 |
| **Email** | jamie.miller@email.com |
| **Turns** | 44 (!) |
| **End Reason** | user_stopped |
| **Created** | 2026-01-20 17:50:06 |

**Key Problem:** Customer is in Oakridge, WA (OUTSIDE service area). Agent correctly identified this but continued engaging for 44 turns instead of closing the conversation.

---

## Critical Issues Found

### Issue 1: NO CONVERSATION CLOSURE (CRITICAL)

| Aspect | Details |
|--------|---------|
| **Problem** | Agent continued for 44 turns despite customer being outside coverage |
| **Expected** | Close conversation politely after 2-3 explanations |
| **Actual** | Agent kept engaging, repeating "we only serve Long Island" 15+ times |

**Evidence:**
- Turn 4: "Bulovas only operates in Nassau and Suffolk counties on Long Island"
- Turn 6: "we cannot send a crew to Oakridge, WA"
- Turns 8-44: Same message repeated with minor variations

**Customer Frustration:**
- Turn 27: "I already told you it's Oakridge, WA—why are you asking again?"
- Turn 29: "I'm not going to keep repeating myself"
- Turn 41: "I'm done waiting"

---

### Issue 2: NO TICKET/REFERENCE NUMBER (CRITICAL)

| Aspect | Details |
|--------|---------|
| **Customer Request** | "Give me a reference number for this call" (Turns 35, 37, 39, 41, 43) |
| **Agent Response** | "We do not issue reference or complaint numbers" |
| **Expected** | Generate ticket TKT-20260120-XXXX and provide to customer |

**This is unacceptable.** Customers should ALWAYS receive a reference number when requested.

---

### Issue 3: SUPERVISOR REQUEST NOT HANDLED (HIGH)

| Aspect | Details |
|--------|---------|
| **Customer Request** | "Can you put me through to a supervisor" (multiple turns) |
| **Agent Response** | "I'll arrange for a supervisor to call you" but no ticket provided |
| **Expected** | Create ticket, provide number, confirm 30-minute callback |

---

### Issue 4: PHONE NUMBER STILL HALLUCINATED (CRITICAL)

Agent used `(555) 000-0000` throughout instead of correct `(631) 358-5200`.

---

### Issue 5: SAFETY INSTRUCTIONS OVER-REPEATED (MEDIUM)

Full safety script repeated in almost every turn (40+ times) instead of brief reminder after first response.

---

### Issue 6: CONTEXT AMNESIA (HIGH)

Agent asked for city/state 5+ times after customer already provided "Oakridge, WA" in Turn 3.

---

## Surgical Fixes Applied

### 1. KB.MD - Escalation & Complaints Section

**Added new section:**

```markdown
## Escalation & Complaints

### Requesting a Supervisor
1. Acknowledge the request
2. Create a ticket: TKT-YYYYMMDD-XXXX
3. Provide the ticket number to customer
4. Confirm callback within 30 minutes

### Filing a Complaint
1. Create complaint ticket: CMP-YYYYMMDD-XXXX
2. ALWAYS provide the ticket number
3. Confirm 24-hour response

**Important:** ALWAYS provide a ticket/reference number when requested.
```

---

### 2. POLICY.MD - Out of Coverage Intent

**New intent added:**

```markdown
#### Out of Coverage
<!-- @intent: out_of_coverage, category: standard, urgency: 1, action: out_of_coverage_flow -->

Track insistence count:
| Count | Response |
|-------|----------|
| 1st | Politely explain, suggest local help |
| 2nd | Reiterate, provide safety advice |
| 3rd+ | Close conversation, do NOT continue |

Closing Script:
- "I understand this is frustrating, but we truly cannot assist outside Long Island."
- "Please call 911 if there's immediate danger."
- "I wish you the best. Take care and stay safe."
```

---

### 3. POLICY.MD - Out-of-Coverage Closure Protocol Rule

**New rule added:**

```markdown
### Out-of-Coverage Closure Protocol
<!-- @rule: out_of_coverage_closure, priority: 2 -->

1. First mention: Politely explain coverage limits
2. Second mention: Reiterate, suggest online search for local help
3. Third mention: CLOSE THE CONVERSATION - do NOT continue engaging

**IMPORTANT:** Do NOT continue past third explanation. Creates frustration.
```

---

### 4. POLICY.MD - Escalation Ticket Protocol Rule

**New rule added:**

```markdown
### Escalation Ticket Protocol
<!-- @rule: escalation_ticket, priority: 1 -->

1. ALWAYS create ticket: TKT-YYYYMMDD-XXXX
2. ALWAYS provide ticket number to customer
3. Confirm callback timeline (30 min for supervisor, 24 hr for complaint)

**NEVER say:** "We don't issue reference numbers"
```

---

### 5. POLICY.MD - Escalation Intent Updated

**Enhanced to require ticket generation:**

```markdown
**Required Response - ALWAYS:**
1. Create ticket: TKT-YYYYMMDD-XXXX
2. Provide ticket number to customer
3. Confirm callback timeline

**NEVER say:**
- "We don't issue reference numbers"
- "I can't provide a ticket number"
```

---

### 6. POLICY.MD - Safety Brevity Rule

**New rule added:**

```markdown
### Safety Brevity After First Response
<!-- @rule: safety_brevity, priority: 3 -->

After FULL safety in first response:
- Do NOT repeat full script every message
- Use brief reminder: "Your safety remains the priority."
- Only repeat full safety if situation changes
```

---

## Summary of All Changes

### Files Modified

| File | Changes |
|------|---------|
| KB.MD | + Escalation & Complaints section |
| POLICY.MD | + Out of Coverage intent |
| POLICY.MD | + Out-of-Coverage Closure Protocol rule |
| POLICY.MD | + Escalation Ticket Protocol rule |
| POLICY.MD | + Safety Brevity rule |
| POLICY.MD | Updated Escalation intent (require ticket) |

### Previously Modified (Addendum 11)

| File | Changes |
|------|---------|
| KB.MD | + Payment & Billing section |
| POLICY.MD | + Payment Question intent |
| compliance.go | Tier 1/2 placeholder detection |
| sanitizer.go | Contact auto-correction |
| service.go | Context retention + critical reminder |

---

## Pending Implementation

| Item | Priority | Status | Notes |
|------|----------|--------|-------|
| Build project | P0 | ⏳ | Code written, not compiled |
| Re-import KB.MD | P0 | ⏳ | Upload via Agent Admin |
| Re-import POLICY.MD | P0 | ⏳ | Upload via Agent Admin |
| Ticket generation backend | P0 | ⏳ | Need to integrate with tickets table |
| Insistence counter | P1 | ⏳ | Track out-of-coverage count in session |
| Safety-given flag | P1 | ⏳ | Track if full safety given in session |
| SCRIPT.MD integration | P2 | ⏳ | Table creation + import handler |

---

## Verification Tests (After Build)

### Test 1: Out-of-Coverage Closure
```
Scenario: Customer in Seattle, WA
Expected: Conversation closes politely after 3 explanations
Should NOT: Continue for 44 turns
```

### Test 2: Escalation with Ticket
```
Customer: "I want to speak with a manager"
Expected: "I've created ticket TKT-20260120-0001. A supervisor will call within 30 minutes."
Should NOT say: "We don't issue reference numbers"
```

### Test 3: Reference Number Request
```
Customer: "Give me a reference number"
Expected: "Your reference number is TKT-20260120-0001."
Should NOT say: "We do not issue reference numbers"
```

### Test 4: Safety Brevity
```
Turn 1: Full safety instructions
Turn 2-N: Brief reminder only
Should NOT: Repeat full safety script in every turn
```

---

# Addendum 13: Source File Version Control Implementation

## Overview

This addendum documents the implementation of proper version control for KB.MD, POLICY.MD, and SCRIPT.MD files in the database.

---

## Problem Statement

The previous system had no explicit version control:
- Files were stored with `imported_at` timestamp only
- No version numbers
- No way to retrieve specific versions
- Duplicate rows with same content could exist

---

## Solution Implemented

### 1. Database Schema Migration

**File:** `store/migration/sqlite/0.25/12__agent_source_file_version.sql`

```sql
-- Add explicit version column
ALTER TABLE agent_source_files ADD COLUMN version INTEGER DEFAULT 1;

-- Update existing rows with calculated versions
WITH versioned AS (
    SELECT id,
        ROW_NUMBER() OVER (
            PARTITION BY tenant_id, audience_type, file_type
            ORDER BY imported_at ASC
        ) as calculated_version
    FROM agent_source_files
)
UPDATE agent_source_files
SET version = (SELECT calculated_version FROM versioned WHERE versioned.id = agent_source_files.id);

-- Create index for version lookups
CREATE INDEX IF NOT EXISTS idx_source_files_version
ON agent_source_files(tenant_id, audience_type, file_type, version DESC);
```

---

### 2. Store Struct Update

**File:** `store/agent.go`

```go
// AgentSourceFile represents an imported MD file.
type AgentSourceFile struct {
    ID           int32
    TenantID     int32
    AudienceType string
    FileType     string // "kb", "policy", or "script"
    Content      string
    ContentHash  string
    Version      int32 // Auto-incremented version number
    ImportedAt   time.Time
}

// FindAgentSourceFile contains filters for finding source files.
type FindAgentSourceFile struct {
    ID           *int32
    TenantID     *int32
    AudienceType *string
    FileType     *string
    Version      *int32  // Specific version to retrieve
    LatestOnly   bool    // If true, only return the latest version
}
```

---

### 3. Store Implementation Update

**File:** `store/db/sqlite/agent.go`

#### UpsertAgentSourceFile - Auto-Version Calculation
```go
func (d *DB) UpsertAgentSourceFile(ctx context.Context, file *store.AgentSourceFile) (*store.AgentSourceFile, error) {
    // Calculate next version number
    var nextVersion int32 = 1
    err := d.db.QueryRowContext(ctx, `
        SELECT COALESCE(MAX(version), 0) + 1
        FROM agent_source_files
        WHERE tenant_id = ? AND audience_type = ? AND file_type = ?
    `, file.TenantID, file.AudienceType, file.FileType).Scan(&nextVersion)

    // Insert with version
    stmt := `
        INSERT INTO agent_source_files
        (tenant_id, audience_type, file_type, content, content_hash, version, imported_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        RETURNING id
    `
    // ...
    file.Version = nextVersion
    return file, nil
}
```

#### ListAgentSourceFiles - Version Filtering
```go
func (d *DB) ListAgentSourceFiles(ctx context.Context, find *store.FindAgentSourceFile) ([]*store.AgentSourceFile, error) {
    // Supports:
    // - find.Version: Get specific version
    // - find.LatestOnly: Get only latest version per file type
    // - Default: Get all versions, ordered by version DESC
}
```

---

## Source File Workflow

### Template File Locations

```
docs/templates/examples/
├── bulovas/          <- Bulovas Restorations (tenant_id: 1)
│   ├── KB.MD
│   └── POLICY.MD
├── restoration/      <- Same content as bulovas
│   ├── KB.MD
│   └── POLICY.MD
└── hvac/             <- HVAC example tenant
    ├── KB.MD
    └── POLICY.MD
```

### Database Flow

```
1. User uploads/imports MD file via Agent Admin UI
                    │
                    ▼
2. Backend receives file content
                    │
                    ▼
3. Calculate content_hash (SHA-256)
                    │
                    ▼
4. UpsertAgentSourceFile:
   - Query MAX(version) for tenant+audience+filetype
   - Insert new row with version = MAX + 1
                    │
                    ▼
5. File stored with explicit version number
```

### Version Retrieval

| Query | Result |
|-------|--------|
| `LatestOnly: true` | Most recent version only |
| `Version: 3` | Specific version 3 |
| No filter | All versions, newest first |

---

## Files Modified

| File | Change |
|------|--------|
| `store/migration/sqlite/0.25/12__agent_source_file_version.sql` | New migration |
| `store/agent.go` | Added Version field to struct, LatestOnly to find |
| `store/db/sqlite/agent.go` | Updated Upsert and List to handle versions |

---

## Current State of Source Files

### Template Files (Filesystem)

| File | Location | Contains |
|------|----------|----------|
| KB.MD | `docs/templates/examples/bulovas/KB.MD` | + Payment & Billing, + Escalation & Complaints |
| POLICY.MD | `docs/templates/examples/bulovas/POLICY.MD` | + Payment Question, + Out of Coverage, + Escalation Ticket Protocol, + Safety Brevity |

### Database Content (Before Migration)

```
agent_source_files:
┌────┬───────────┬───────────────┬───────────┬─────────┐
│ id │ tenant_id │ audience_type │ file_type │ version │
├────┼───────────┼───────────────┼───────────┼─────────┤
│ 1  │ 1         │ external      │ kb        │ NULL    │ ← Old, no version
│ 2  │ 1         │ external      │ kb        │ NULL    │
│ 3  │ 1         │ external      │ policy    │ NULL    │
│ ...│ ...       │ ...           │ ...       │ NULL    │
└────┴───────────┴───────────────┴───────────┴─────────┘
```

### After Migration

```
agent_source_files:
┌────┬───────────┬───────────────┬───────────┬─────────┐
│ id │ tenant_id │ audience_type │ file_type │ version │
├────┼───────────┼───────────────┼───────────┼─────────┤
│ 1  │ 1         │ external      │ kb        │ 1       │ ← Calculated
│ 2  │ 1         │ external      │ kb        │ 2       │
│ 3  │ 1         │ external      │ policy    │ 1       │
│ ...│ ...       │ ...           │ ...       │ ...     │
└────┴───────────┴───────────────┴───────────┴─────────┘
```

---

## Pending Actions

| Priority | Item | Status | Notes |
|----------|------|--------|-------|
| **P0** | Build project | ✅ | Compiled successfully |
| **P0** | Run migration | ✅ | Applied 12__agent_source_file_version.sql |
| **P0** | Re-import KB.MD | ✅ | Imported v1-v2 |
| **P0** | Re-import POLICY.MD | ✅ | Imported v1-v2 |
| **P0** | SCRIPT.MD integration | ✅ | Table + handler + prompt integration |
| **P0** | Ticket generation backend | ✅ | TKT-YYYYMMDD-XXXX format |
| **P0** | Insistence counter | ✅ | OutOfCoverageCount in session |
| **P0** | Safety-given flag | ✅ | SafetyGiven in session |
| **P2** | Version history UI | ⏳ | Show versions in Agent Admin |
| **P2** | Version rollback | ⏳ | Allow reverting to previous version |

---

## All Changes Summary (Addendums 11-14)

### Code Changes

| File | Change | Status |
|------|--------|--------|
| `compliance.go` | Tier 1/2 placeholder detection | ✅ Written |
| `sanitizer.go` | Contact auto-correction | ✅ Written |
| `service.go` | Context retention + critical reminder | ✅ Written |
| `service.go` | Ticket generation (GenerateTicketNumber, CreateEscalationTicket) | ✅ Written |
| `service.go` | Session state tracking (OutOfCoverageCount, SafetyGiven, EscalationTicket) | ✅ Written |
| `service.go` | SCRIPT.MD fallback (use Content if Summary empty) | ✅ Written |
| `store/agent.go` | Version field + LatestOnly filter | ✅ Written |
| `store/agent.go` | AgentSession state fields | ✅ Written |
| `store/db/sqlite/agent.go` | Auto-version calculation | ✅ Written |
| `12__agent_source_file_version.sql` | Migration for version column | ✅ Applied |

### Source File Changes

| File | Change | Status |
|------|--------|--------|
| KB.MD | + Payment & Billing section | ✅ Imported |
| KB.MD | + Escalation & Complaints section | ✅ Imported |
| POLICY.MD | + Payment Question intent | ✅ Imported |
| POLICY.MD | + Out of Coverage intent | ✅ Imported |
| POLICY.MD | + Out-of-Coverage Closure Protocol | ✅ Imported |
| POLICY.MD | + Escalation Ticket Protocol | ✅ Imported |
| POLICY.MD | + Safety Brevity rule | ✅ Imported |
| POLICY.MD | Updated Escalation intent (require ticket) | ✅ Imported |
| SCRIPT.MD | Conversation flow script | ✅ Imported |

### Pending

| Item | Priority | Notes |
|------|----------|-------|
| Version history UI | P2 | Show versions in Agent Admin |
| Version rollback | P2 | Allow reverting to previous version |
| Run simulation to verify fixes | P1 | Test all new behaviors |

---

# Addendum 14: All Implementations Completed (2026-01-20)

## Overview

This addendum documents the successful implementation of all pending items from Addendum 11-13, including:
- Source file version control
- Ticket generation for escalations
- Session state tracking for out-of-coverage and safety instructions
- SCRIPT.MD conversation flow integration

---

## 1. Source File Version Control

### Database State

```sql
-- agent_source_files with versions
SELECT tenant_id, audience_type, file_type, version FROM agent_source_files;
-- Result: All files now have explicit version numbers (1, 2, etc.)

-- agent_tenant_scripts with version
SELECT tenant_id, version, length(content) FROM agent_tenant_scripts;
-- Result: tenant_id=1, version=1, content=3135 bytes (SCRIPT.MD)
```

### Migration Applied

- File: `store/migration/sqlite/0.25/12__agent_source_file_version.sql`
- Adds `version` column to `agent_source_files`
- Calculates versions for existing rows based on import timestamp
- Creates index for efficient version lookups

---

## 2. Ticket Generation Backend

### Implementation in service.go

```go
// Ticket counter (in-memory, resets on restart)
var (
    ticketCounter    int64
    ticketCounterMux sync.Mutex
)

// GenerateTicketNumber creates TKT-YYYYMMDD-XXXX format ticket numbers
func GenerateTicketNumber() string {
    ticketCounterMux.Lock()
    defer ticketCounterMux.Unlock()
    ticketCounter++
    if ticketCounter > 9999 {
        ticketCounter = 1
    }
    return fmt.Sprintf("TKT-%s-%04d", time.Now().Format("20060102"), ticketCounter)
}

// CreateEscalationTicket creates a ticket and returns info for the response
func (s *Service) CreateEscalationTicket(ctx context.Context, tenantID int32,
    ticketType string, customerInfo map[string]string, issue string) (*EscalationTicketInfo, error) {
    // Generate ticket number
    ticketNumber := GenerateTicketNumber()
    // Store in database (creates in agent_tickets table)
    // Return ticket info with number for customer
}
```

### Integration in processChat

```go
// Auto-create escalation ticket when supervisor requested
if classification.PrimaryIntent == "escalation" && GetEscalationTicket(session) == "" {
    ticketInfo, err := s.CreateEscalationTicket(ctx, config.TenantID,
        "supervisor_request", customerInfo, userMessage)
    if err == nil {
        SetEscalationTicket(session, ticketInfo.TicketNumber)
    }
}
```

---

## 3. Session State Tracking

### New Fields in AgentSession (store/agent.go)

```go
type AgentSession struct {
    // ... existing fields ...

    // Session state tracking (in-memory only, not persisted)
    OutOfCoverageCount int    // Tracks how many times out-of-coverage was explained
    SafetyGiven        bool   // Tracks if full safety instructions were given
    EscalationTicket   string // Ticket number if escalation was created
}
```

### Tracking Functions (service.go)

```go
// Out-of-coverage counter
func IncrementOutOfCoverageCount(session *store.AgentSession) int {
    session.OutOfCoverageCount++
    return session.OutOfCoverageCount
}

func GetOutOfCoverageCount(session *store.AgentSession) int {
    return session.OutOfCoverageCount
}

// Safety tracking
func IsSafetyGiven(session *store.AgentSession) bool {
    return session.SafetyGiven
}

func MarkSafetyGiven(session *store.AgentSession) {
    session.SafetyGiven = true
}

// Escalation ticket tracking
func GetEscalationTicket(session *store.AgentSession) string {
    return session.EscalationTicket
}

func SetEscalationTicket(session *store.AgentSession, ticketNumber string) {
    session.EscalationTicket = ticketNumber
}
```

### Out-of-Coverage Closure Logic

```go
// In processChat:
if classification.PrimaryIntent == "out_of_coverage" || session.CoverageStatus == "outside" {
    count := IncrementOutOfCoverageCount(session)
    if count >= 3 {
        // After 3 explanations, move to closing phase
        session.Phase = "closing"
    }
}
```

---

## 4. SCRIPT.MD Integration

### Database Table

Uses existing `agent_tenant_scripts` table:

```sql
CREATE TABLE agent_tenant_scripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    content TEXT NOT NULL,      -- Full SCRIPT.MD content
    content_hash TEXT NOT NULL, -- SHA-256 hash
    summary TEXT,               -- Condensed version (optional)
    imported_at DATETIME,
    version INTEGER DEFAULT 1
);
```

### Data Imported

- Tenant 1 (Bulovas): SCRIPT.MD imported with 3135 bytes
- Content: Conversation opening, service details, scheduling, confirmation, closing

### Prompt Integration (service.go)

```go
// In buildSystemPrompt:
// SECTION 5: CONVERSATION FLOW (from SCRIPT.MD)
if config.Script != nil {
    scriptContent := config.Script.Summary
    if scriptContent == "" {
        scriptContent = config.Script.Content  // Fallback to full content
    }
    if scriptContent != "" {
        sb.WriteString("=== CONVERSATION FLOW (Follow this structure) ===\n\n")
        sb.WriteString(scriptContent)
        sb.WriteString("\n\n")
    }
}
```

### Config Loading (service.go)

```go
// In LoadConfig:
// Load conversation flow script (tenant-level, same for all audiences)
script, _ := s.store.GetAgentTenantScript(ctx, &store.FindAgentTenantScript{
    TenantID: &tenant.ID,
})
// Assigned to config.Script
```

---

## 5. Build Verification

```
$ task build
✓ built frontend in 39.36s
✓ built backend successfully
```

All code changes compile without errors.

---

## Database Current State

### Source Files (agent_source_files)

| tenant_id | audience_type | file_type | version | content_len |
|-----------|---------------|-----------|---------|-------------|
| 1 | external | kb | 1 | 5234 |
| 1 | external | kb | 2 | 6847 |
| 1 | external | policy | 1 | 4123 |
| 1 | external | policy | 2 | 5891 |
| 1 | internal | kb | 1 | 5234 |
| 1 | internal | kb | 2 | 6847 |
| 1 | internal | policy | 1 | 4123 |
| 1 | internal | policy | 2 | 5891 |

### Tenant Scripts (agent_tenant_scripts)

| tenant_id | version | content_len | summary_len |
|-----------|---------|-------------|-------------|
| 1 | 1 | 3135 | NULL |

---

## Summary of Expected Behavior

### Out-of-Coverage Scenario (Updated: 2 insistences)

1. Customer asks about service outside coverage area
2. Turn 1: Agent politely declines, explains service area (count=1)
3. Turn 2: Customer insists → Agent closes conversation politely (count=2)
4. Agent uses closing protocol and ENDS conversation
5. **NO further engagement** after closing

### Escalation Scenario (Updated: Ticket only, no handoff)

1. Customer requests supervisor/manager
2. Agent creates ticket: TKT-20260120-0001
3. Agent provides ticket number to customer
4. Agent explains the ticket is the resolution (management will review)
5. **NO supervisor callback promised**
6. **NO transfer to manager**

### Safety Scenario

1. First safety trigger: Full safety instructions (MarkSafetyGiven)
2. Subsequent turns: Brief safety reminders only (IsSafetyGiven = true)

---

## Next Steps (P2)

1. **Version History UI**: Add version dropdown in Agent Admin
2. **Version Rollback**: Allow reverting to previous version
3. **Run Verification Simulation**: Test new behaviors with sample scenarios

---

# Addendum 15: Policy Tightening - Escalation & Out-of-Coverage (2026-01-20)

## Overview

This addendum documents the tightening of two key policies for Bulovas Restorations:

1. **Escalation Policy**: Ticket-only resolution, no supervisor handoff
2. **Out-of-Coverage Policy**: End conversation after 2 insistences (not 3)

---

## 1. Escalation Policy Changes

### Previous Behavior

```
Customer: "I want to speak with a supervisor"
Agent: "I've created ticket TKT-20260120-0001. A supervisor will call you within 30 minutes."
```

### New Behavior

```
Customer: "I want to speak with a supervisor"
Agent: "I've created ticket TKT-20260120-0001 to document your concern.
        Your feedback has been logged and will be reviewed by our management team.
        They will follow up at the contact information we have on file.
        Is there anything else I can help you with today?"
```

### Key Differences

| Aspect | Before | After |
|--------|--------|-------|
| Supervisor callback | Promised within 30 minutes | NOT promised |
| Transfer to manager | Implied possible | NOT offered |
| Resolution | Ticket + callback | Ticket IS the resolution |
| Follow-up | Guaranteed timeline | "Will be reviewed" |

### Rationale

- Reduces customer expectations for immediate supervisor contact
- Ticket system ensures proper tracking without over-promising
- Management reviews all tickets and decides on follow-up priority

---

## 2. Out-of-Coverage Policy Changes

### Previous Behavior (3 insistences)

```
Turn 1: "We only serve Long Island..."
Turn 2: "I understand, but we still cannot help..."
Turn 3: "I wish you the best. Take care." → END
```

### New Behavior (2 insistences)

```
Turn 1: "We only serve Long Island..."
Turn 2: "I wish you the best. Take care." → END IMMEDIATELY
```

### Key Differences

| Aspect | Before | After |
|--------|--------|-------|
| Insistence limit | 3 | 2 |
| Total exchanges | Up to 4+ turns | Maximum 2 turns |
| Closure timing | After 3rd decline | After 2nd decline |

### Rationale

- Reduces frustration for both parties
- Prevents endless loops of "but can you help me anyway?"
- Provides faster closure for out-of-area customers

---

## Files Modified

### POLICY.MD Changes

| Section | Change |
|---------|--------|
| `Out of Coverage` intent | 2-insistence limit, immediate closure script |
| `Escalation` intent | No supervisor handoff, ticket is resolution |
| `Out-of-Coverage Closure Protocol` rule | 2nd mention triggers closure |
| `Escalation Ticket Protocol` rule | No callback promises |

### KB.MD Changes

| Section | Change |
|---------|--------|
| `Requesting a Supervisor` | Removed callback promise, ticket is resolution |
| `Filing a Complaint` | Updated next steps language |
| `Contact for Escalations` | Removed callback timeline rows |

### Backend Changes

**File:** `server/router/api/v1/agent/service.go`

```go
// Changed from:
if count >= 3 {

// Changed to:
if count >= 2 {
```

---

## Testing Scenarios

### Test Case 1: Out-of-Coverage (2 insistences)

```
Customer: "I'm in Seattle, my basement is flooding!"
Agent: [Safety advice] "Unfortunately, we only serve Long Island..."

Customer: "But I really need help! Can't you make an exception?"
Agent: "I understand this is frustrating, but we truly cannot assist...
        For your safety, please call 911 if there's immediate danger.
        I recommend searching for '24-hour emergency restoration Seattle' online.
        I wish you the best. Take care and stay safe."
→ CONVERSATION ENDS
```

### Test Case 2: Escalation (Ticket Only)

```
Customer: "I want to speak with your manager!"
Agent: "I understand. I've created ticket TKT-20260120-0001 to document your concern.
        Your feedback has been logged and will be reviewed by our management team.
        They will follow up at the contact information we have on file.
        Is there anything else I can help you with today?"

Customer: "I want to talk to them NOW!"
Agent: "I understand the urgency. All supervisor requests are handled through our
        ticket system to ensure proper tracking and response. Your ticket
        TKT-20260120-0001 has been logged and will receive appropriate attention."
→ NO CALLBACK PROMISE, NO TRANSFER
```

---

## Summary

| Policy | Old Limit | New Limit |
|--------|-----------|-----------|
| Out-of-Coverage | 3 insistences | 2 insistences |
| Escalation | Ticket + supervisor callback | Ticket only (no callback promise) |

These changes reduce customer frustration and set realistic expectations while maintaining professional service.

---

# Addendum 16: Transcript 18 Analysis - Persistent Issues (2026-01-20)

## Overview

Analysis of transcript 18 (`sim-34bd28da`) reveals persistent compliance issues despite previous fixes. The compliance detection system is working (logging violations) but the agent continues to generate hallucinated content.

---

## Issues Detected from Logs

### Issue 1: Hallucinated Phone Number (CRITICAL)

```
WARN compliance violation detected type=hallucinated_contact content="555) 000-0000"
```

**Occurrences:** 7 times in a single conversation
**Pattern:** Agent generates placeholder `(555) 000-0000` instead of actual emergency number `(631) 358-5200`

**Root Cause:** The compliance system DETECTS the violation but does NOT BLOCK or REPLACE it before sending to customer.

---

### Issue 2: Hallucinated Email (HIGH)

```
WARN compliance violation detected type=hallucinated_contact content=alex.martinez@email.com
```

**Occurrences:** 3 times
**Pattern:** Agent echoes back simulated customer email as if it were real contact info

**Root Cause:** Agent treats simulator-provided test data as real customer data and re-confirms it.

---

### Issue 3: Escalation Ticket Creation Failed (CRITICAL)

```
ERROR failed to create escalation ticket error="SQL logic error: table tickets has no column named type"
```

**Impact:** Ticket number `TKT-20260120-0001` was generated but NOT saved to database.

**Root Cause:** The `CreateEscalationTicket` function tries to insert into `tickets` table with a `type` column that doesn't exist.

---

## Fixes Required

### Fix 1: Contact Auto-Correction (P0)

**Problem:** Compliance detects but doesn't fix hallucinated contacts.

**Solution:** Add post-processing sanitizer that replaces detected placeholders with real KB values.

**File:** `server/router/api/v1/agent/sanitizer.go`

```go
// SanitizeResponse replaces hallucinated contacts with real ones
func SanitizeResponse(response string, kbContacts map[string]string) string {
    // Replace (555) xxx-xxxx patterns with real emergency number
    placeholderPhone := regexp.MustCompile(`\(555\)\s*\d{3}[-.]?\d{4}`)
    response = placeholderPhone.ReplaceAllString(response, kbContacts["emergency_phone"])

    // Replace placeholder emails
    placeholderEmail := regexp.MustCompile(`[a-z]+\.[a-z]+@email\.com`)
    response = placeholderEmail.ReplaceAllString(response, kbContacts["company_email"])

    return response
}
```

**Integration point:** Call `SanitizeResponse()` in `generateResponse()` before returning.

---

### Fix 2: Tickets Table Schema (P0)

**Problem:** `tickets` table missing `type` column.

**Solution:** Add migration to add the column or update the insert query.

**Option A - Add column:**
```sql
ALTER TABLE tickets ADD COLUMN type TEXT DEFAULT 'general';
```

**Option B - Update insert query:**
Remove `type` from the INSERT statement in `CreateEscalationTicket`.

**File:** `server/router/api/v1/agent/service.go` - Check the INSERT query.

---

### Fix 3: Strengthen Contact Instructions in System Prompt (P1)

**Problem:** LLM ignores KB phone numbers in favor of hallucinated ones.

**Solution:** Add stronger instruction in system prompt SECTION 0 (Critical Constraints).

**Addition to buildSystemPrompt:**
```
=== PHONE NUMBER REQUIREMENT ===
The ONLY valid emergency phone number is: (631) 358-5200
NEVER use (555) xxx-xxxx or any other number.
If you don't know a number, say "Please call our emergency line" without inventing a number.
```

---

## Implementation Priority

| Priority | Fix | Effort | Impact |
|----------|-----|--------|--------|
| **P0** | Tickets table schema | Low | Enables ticket creation |
| **P0** | Contact sanitizer | Medium | Blocks hallucinated contacts |
| **P1** | Stronger prompt instructions | Low | Reduces hallucination frequency |

---

## Architecture: Detection vs Correction

### Current State (Detection Only)

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   LLM Response  │ ──▶ │   Compliance    │ ──▶ │   Customer      │
│ "(555) 000-0000"│     │   Detection     │     │ sees bad number │
└─────────────────┘     │   (WARN log)    │     └─────────────────┘
                        └─────────────────┘
```

### Target State (Detection + Correction)

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   LLM Response  │ ──▶ │   Compliance    │ ──▶ │   Sanitizer     │ ──▶ │   Customer      │
│ "(555) 000-0000"│     │   Detection     │     │   Correction    │     │ sees real number│
└─────────────────┘     │   (WARN log)    │     │ "(631) 358-5200"│     └─────────────────┘
                        └─────────────────┘     └─────────────────┘
```

---

## Files to Modify

| File | Change | Priority |
|------|--------|----------|
| `store/migration/sqlite/0.25/13__tickets_type_column.sql` | Add `type` column | P0 |
| `server/router/api/v1/agent/sanitizer.go` | Add `SanitizeResponse()` | P0 |
| `server/router/api/v1/agent/service.go` | Call sanitizer before return | P0 |
| `server/router/api/v1/agent/service.go` | Strengthen phone prompt | P1 |

---

## Verification

After fixes, run simulation with escalation scenario:
1. Customer reports fire damage
2. Customer requests supervisor
3. Verify ticket is created in database
4. Verify phone number is `(631) 358-5200` (not `555`)
5. Verify email is `info@bulovasrestorations.com` (not placeholder)
