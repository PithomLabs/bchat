# Observational Memory Analysis & Implementation Strategy for bchat

**Date:** 2026-02-13
**Status:** Draft
**Context:** Analysis of Mastra's "Observational Memory" (OM) system for potential adoption in bchat.

---

## 1. Executive Summary

Observational Memory (OM) is a memory architecture designed to solve the "context window" problem in long-running agent conversations without the complexity or information loss of traditional RAG (Retrieval Augmented Generation).

Instead of storing raw logs or retrieving snippets, OM uses periodic background agents ("Observer" and "Reflector") to **compress** conversation history into a concise, continuously updated "observation log". This log is small enough to stay in the context window *permanently*, giving the agent "infinite" effective memory of the conversation's key facts and events.

**Recommendation:** bchat should adopt this pattern to replace the current "last 10 messages" truncation logic. The specific architecture (Go backend + SQLite) is well-suited for a port of the OM logic using background goroutines.

---

## 2. Deep Dive: How Mastra's OM Works

Based on analysis of the Mastra source code (`packages/memory/src/processors/observational-memory/`), the system relies on three key components:

### 2.1 The Observer Agent (The "Scribe")
- **Trigger:** Runs when unobserved messages exceed a token threshold (default: 30,000 tokens).
- **Function:** Reads recent raw messages and extracts "observations" - distinct facts, events, or user preferences.
- **Key Logic:**
    - **Temporal Anchoring:** Every observation is timestamped (e.g., "User mentioned moving to London on Jan 15").
    - **Distinction:** detailed instructions (in `observer-agent.ts`) force the model to distinguish between *Assertions* (User: "I am hungry") and *Questions* (User: "Am I hungry?").
    - **Priority:** Assigns priority levels (High 🔴, Medium 🟡, Low 🟢) to observations.

### 2.2 The Reflector Agent (The "Editor")
- **Trigger:** Runs when the *observation log itself* gets too big (default: 40,000 tokens).
- **Function:** Reads the current observation log and "reflects" on it to consolidate redundancy, synthesize insights, and discard trivial details (Garbage Collection).
- **Logic:** It rewrites the log to be more compact while preserving the "narrative arc" and key facts.

### 2.3 The Context Runtime
- **Injection:** When the user sends a message, the system injects the *entire* current observation log into the System Prompt.
- **Truncation:** The system then appends only the *unobserved* recent messages (raw) to the context.
- **Result:** The LLM sees a summary of the past + verbatim recent context.

---

## 3. Strengths & Weaknesses

### Strengths
1.  **Stable Context Window:** Unlike RAG, which swaps context in/out (causing "gaslighting" where the agent forgets what it just knew), OM keeps key facts *always* in context.
2.  **Cost Efficiency:** Because the observation block changes infrequently (only after a long batch of messages), **Prompt Caching** (e.g., in Gemini/Anthropic) works extremely well, potentially reducing costs by 10x compared to sending full history.
3.  **Narrative Coherence:** The Reflector synthesizes "insights" over time, allowing the agent to "learn" user patterns rather than just retrieving disconnected snippets.
4.  **Performance:** No vector database lookups required for conversation history; it's just text processing.

### Weaknesses
1.  **Lossy Compression:** It is a compression algorithm. Specific phrasing of old messages is lost. The agent remembers *what* you said, not *how* you said it.
2.  **Latency/Complexity:**
    -   *Synchronous Mode:* The user has to wait for the Observer to run (bad UX).
    -   *Asynchronous Mode:* Requires background workers (complexity).
    -   Mastra handles this with an "async buffer" pattern which `bchat` would need to replicate.
3.  **Knowledge Ceiling:** Eventually, even the compressed log fills the context. At that point, the Reflector must aggressively "garbage collect," meaning oldest/least important memories are permanently deleted.

---

## 4. Strategic Fit for bchat

Current `bchat` implementation (in `server/router/api/v1/agent/service.go`):
```go
// Current Logic: Hard truncation
if len(session.Messages) > 10 {
    historyStart = len(session.Messages) - 10
}
```

**Problem:** The agent has a "goldfish memory." It forgets anything said 11 turns ago.
**Opportunity:** `bchat` is already architected with a database (`agent_sessions` table) and background processing capabilities. Porting OM allows `bchat` to offer "Pro" tier memory capabilities where agents remember specialized instructions or long context workflows.

---

## 5. Implementation Strategy: Porting to Go

Since `bchat` is Go-based, we cannot strictly "import" the Mastra TS library. We must implement the *pattern*.

### 5.1 Database Schema (`DB: SQLite`)

We need a table to store the state of the observational memory for each session.

```sql
CREATE TABLE agent_observations (
    session_id TEXT PRIMARY KEY REFERENCES agent_sessions(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL,
    
    -- The State
    observation_log TEXT,          -- The massive text block of observations
    last_observed_msg_index INTEGER DEFAULT 0, -- Pointer to last processed message
    
    -- Metrics
    tokens_in_log INTEGER DEFAULT 0,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 5.2 The "Observer" Service (Go)

We need a dedicated service or worker method, e.g., `s.AgentService.RunObserver(ctx, sessionID)`.

**Logic Flow:**
1.  **Check Trigger:** Is `(count(session.Messages) - last_observed_msg_index) > THRESHOLD` (e.g., 20 messages)?
2.  **Fetch Data:** Get `session.Messages[last_observed_msg_index:]`.
3.  **LLM Call:** Send these messages to the LLM with the **Observer System Prompt** (ported from `observer-agent.ts`).
    *   *Prompt adaptation:* converting the TS template variables to Go `text/template`.
4.  **Update DB:**
    *   Append new observations to `agent_observations.observation_log`.
    *   Update `last_observed_msg_index` to the end of the batch.
    *   Check if `tokens_in_log > REFLECTION_THRESHOLD`.
5.  **Trigger Reflector (Optional):** If log is too big, run the Reflector prompt to compress `observation_log`.

### 5.3 Modifying the Chat Loop (`service.go`)

We modify `CreateChatCompletion` to inject the memory:

```go
// Pseudo-code implementation structure

// 1. Fetch Observations
observations, _ := s.store.GetObservations(ctx, session.ID)

// 2. Build Context
messages := []openrouter.ChatCompletionMessage{}

// 2a. System Prompt + Memory Injection
sysPrompt := fmt.Sprintf("%s\n\n<memory_observations>\n%s\n</memory_observations>", 
    baseSystemPrompt, 
    observations.Log)
messages = append(messages, openrouter.SystemMessage(sysPrompt))

// 2b. Recent Messages (Unobserved Only)
// Only append messages that haven't been "observed" yet, OR 
// append a generous overlap (e.g. last 10) to ensure continuity.
recentMsgs := session.Messages[observations.LastObservedIndex:]
for _, msg := range recentMsgs {
    // ... append user/assistant messages ...
}
```

### 5.4 Async Strategy

To avoid latency, the Observer should run **asynchronously**.
-   **Trigger:** When `session.AddMessage()` is called, checking the count.
-   **Execution:** explicitly use `go s.RunObserver(...)` so the user gets their reply immediately.
-   **Concurrency Safety:** Use optimistic locking or a simple mutex on the session ID if running in a single instance, though database transactions are safer.

## 6. Next Steps

1.  **Create Prototypes:** Extract the System Prompts from `observer-agent.ts` and `reflector-agent.ts` and save them as `docs/prompts/observer.txt` and `docs/prompts/reflector.txt`.
2.  **Schema Migration:** Create a migration for `agent_observations`.
3.  **PoC Implementation:** Implement the `RunObserver` method in `service.go` and wire it up to a debug endpoint to test compression quality.
