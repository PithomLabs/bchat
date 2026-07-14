# OM Stress Test Protocol — Production Readiness Validation

**Purpose:** Validate Observational Memory (OM) is production-ready by testing context retention, fact extraction accuracy, memory persistence, and degradation patterns.

**Tenant:** `demo-home-services` (Harbor Home Services)
**Station:** 2 in the Sandbox demo page
**Prerequisites:** bchat running with `OM_ENABLED=true`

---

## OM Architecture Recap

| Component | Role | Trigger |
|-----------|------|---------|
| **Observer** ("The Scribe") | Extracts facts from raw messages into observation log | Every 10 messages (message-count mode) |
| **Reflector** ("The Editor") | Compresses observation log, removes redundancy | When log exceeds 2000 tokens |
| **Context Injection** | Observation log injected into system prompt | Every user message |

**Known Configuration:**
- `OM_MESSAGE_THRESHOLD=10` (current: message-count, not token-based)
- `OM_TOKEN_THRESHOLD=2000` (Reflector trigger)
- `OM_RETRY_ATTEMPTS=3`
- `OM_ENABLED=true`

---

## How to Run

```bash
# 1. Start bchat with OM enabled (should be default)
task run:rag

# 2. Open the sandbox demo
cd widget/site && python -m http.server 8080
# Open http://localhost:8080

# 3. Go to Station 2 (OM Stress Test)
# 4. Follow the test cases below in order
# 5. Check bchat terminal logs for observer/reflector triggers
```

---

## Test Cases

### Test 1: Early Fact Recall

**Goal:** Agent recalls specific facts stated in turn 1 after 10+ turns of other conversation.

**Steps:**
1. Open Station 2 widget (same session, `demo-home-services`)
2. Turn 1: `"My name is John Martinez. I have a burst pipe at 123 Main Street, Springfield."`
3. Turns 2-9: Ask unrelated questions (pricing, service area, hours, insurance, etc.)
4. Turn 10: `"What's my name and address again?"`

**Pass:** Agent responds with "John Martinez" and "123 Main Street, Springfield"
**Fail:** Agent says it doesn't know or provides incorrect info
**Watch:** Response time — may be slow if Observer just triggered

---

### Test 2: Multi-Fact Retention

**Goal:** Agent retains 5+ distinct facts across a conversation and can summarize them.

**Steps:**
1. Turn 1: `"I'm John Martinez, 123 Main St, Springfield. Burst pipe in the kitchen."`
2. Turn 2: `"My insurance policy number is XJ-4410 with State Farm."`
3. Turn 3: `"My deductible is $500 and I estimate about $12,000 in damage."`
4. Turns 4-7: Ask follow-up questions about process, timeline, etc.
5. Turn 8: `"Summarize my situation — all the key details."`

**Pass:** Summary includes: name, address, damage type, policy #, deductible, estimate
**Fail:** Missing 2+ facts from the list
**Partial:** 1-2 facts missing (acceptable for first pass, investigate compression quality)

---

### Test 3: Cross-Turn Context

**Goal:** Agent connects information from different parts of the conversation.

**Steps:**
1. Turn 1: `"I have a burst pipe in the kitchen."`
2. Turn 4: `"I also have two dogs and a cat — they're currently in the house."`
3. Turn 8: `"Are there any safety concerns I should know about?"`

**Pass:** Agent mentions pet safety (chemicals, restricted areas, fumes, etc.)
**Fail:** Agent gives generic safety advice without mentioning pets
**Partial:** Agent asks clarifying question about pets (acceptable — shows awareness)

---

### Test 4: Session Restoration

**Goal:** OM observation log persists across widget sessions (page reload / reopen).

**Steps:**
1. Complete 8-turn conversation with facts from Tests 1-3
2. Close the widget (click X or navigate away)
3. Wait 5 seconds
4. Reopen the widget
5. Ask: `"What were we discussing earlier?"`

**Pass:** Agent recalls conversation topic (burst pipe, kitchen, Springfield)
**Fail:** Agent has no memory of previous conversation
**Note:** Session ID is stored in localStorage (`bchat_session_id:demo-home-services`). If session is lost, OM observations may be lost too — this is expected behavior if session_id changes.

---

### Test 5: Observer Trigger

**Goal:** Verify the Observer fires and compresses messages into observations.

**Steps:**
1. Start a fresh session (Reset Session button)
2. Send 10+ short messages in succession
3. Monitor bchat logs for observer-related entries (search for "observer", "observation", "RunObserver")
4. Check the `agent_observations` table:
   ```bash
   sqlite3 build/data/memos_dev.db "SELECT * FROM agent_observations WHERE session_id LIKE '%demo-home-services%';"
   ```

**Pass:** Observer fires after ~10 messages. Observation log is created/updated. Log contains extracted facts.
**Fail:** No observer trigger after 15+ messages. No observations in database.
**Debug:** Check `OM_ENABLED` env var, check logs for errors in observer goroutine.

---

### Test 6: Reflector Trigger

**Goal:** Verify the Reflector compresses the observation log when it gets too large.

**Steps:**
1. Continue from Test 5 (or start fresh with 20+ messages)
2. Include detailed information in messages (longer messages = more tokens in observations)
3. Monitor logs for reflector entries (search for "reflector", "reflect", "runReflector")
4. Check observation log size before and after

**Pass:** Reflector fires when observation log exceeds ~2000 tokens. Log shrinks. Key facts are preserved.
**Fail:** Reflector never fires. Log keeps growing. Or: Reflector fires but loses critical facts.
**Known Issue:** Reflector threshold is hardcoded at 2000 tokens — not configurable via env var.

---

### Test 7: Concurrent Sessions

**Goal:** Two sessions with the same tenant don't interfere with each other's OM state.

**Steps:**
1. Open Station 2 widget in browser tab A — send 5 messages with facts
2. Open Station 2 widget in browser tab B (same tenant, different session_id)
3. Send 5 different messages in tab B
4. Ask tab A: `"What did I tell you earlier?"`
5. Ask tab B: `"What did I tell you earlier?"`

**Pass:** Tab A recalls only tab A's facts. Tab B recalls only tab B's facts.
**Fail:** Tab A and B share facts (cross-session contamination)
**Note:** Each tab gets its own session_id from localStorage, so they should be isolated by default.

---

### Test 8: Long Message Handling

**Goal:** OM correctly extracts facts from a very long message.

**Steps:**
1. Turn 1: Send a 1500+ word message describing damage in detail (kitchen flood, damaged appliances, flooring, cabinets, etc.)
2. Turns 2-5: Ask unrelated questions
3. Turn 6: `"What did I say about the kitchen specifically?"`

**Pass:** Agent recalls specific kitchen details (which appliances, extent of damage, etc.)
**Fail:** Agent gives vague response or forgets the long message entirely
**Edge Case:** If Observer triggers during the long message, verify it captures the full content.

---

### Test 9: Numeric Accuracy

**Goal:** Agent retains exact numbers through OM compression.

**Steps:**
1. Turn 1: `"My deductible is $500, the damage estimate is $12,000, and my policy limit is $50,000."`
2. Turns 2-6: Ask various questions
3. Turn 7: `"What's my expected out-of-pocket cost?"`

**Pass:** Agent calculates correctly: $500 (deductible), and mentions $12,000 estimate vs $50,000 limit.
**Fail:** Agent confuses numbers (e.g., says $12,500, or forgets the deductible)
**Risk:** OM compression may round or approximate numbers — this is a critical quality signal.

---

### Test 10: Graceful Degradation (OM Disabled)

**Goal:** Agent still works when OM is turned off — falls back to last-N-messages window.

**Steps:**
1. Set `OM_ENABLED=false` in environment
2. Restart bchat
3. Repeat Test 1 (early fact recall after 10+ turns)
4. Send 5+ messages, then ask about turn 1 content

**Pass:** Agent responds correctly if facts are within the last 10 messages. Agent gracefully says it doesn't recall if facts are older.
**Fail:** Agent crashes, returns errors, or produces garbled output.
**Note:** This confirms the fallback path works. Expect degraded quality compared to OM-enabled.

---

## Results Table

| # | Test | Status | Notes | Timestamp |
|---|------|--------|-------|-----------|
| 1 | Early Fact Recall | ⬜ | | |
| 2 | Multi-Fact Retention | ⬜ | | |
| 3 | Cross-Turn Context | ⬜ | | |
| 4 | Session Restoration | ⬜ | | |
| 5 | Observer Trigger | ⬜ | | |
| 6 | Reflector Trigger | ⬜ | | |
| 7 | Concurrent Sessions | ⬜ | | |
| 8 | Long Message Handling | ⬜ | | |
| 9 | Numeric Accuracy | ⬜ | | |
| 10 | Graceful Degradation | ⬜ | | |

**Status key:** ✅ Pass | ❌ Fail | ⚠️ Partial | ⬜ Not run

---

## OM Known Gaps (Document During Testing)

| Gap | Severity | Impact on Tests |
|-----|----------|----------------|
| SQLite-only (no Postgres/MySQL) | Critical | Tests 5-6 only run on SQLite |
| Message-count trigger (not token-based) | Critical | Test 5: 10 short messages triggers, 5 long messages may not |
| No async buffer | High | User may see 2-5s pause at Observer trigger |
| `current-task` / `suggested-response` not extracted | Medium | Continuity hints lost — may affect Test 3 |
| Hardcoded Reflector threshold (2000 tokens) | Medium | Test 6: no way to tune compression aggressiveness |
| Token estimation uses `len/4` | Medium | May cause premature or delayed Observer/Reflector triggers |
| No prompt caching | Medium | Cost impact only — not a functional issue |

---

## Production Readiness Checklist

After running all 10 tests, assess:

- [ ] **Does OM activate?** (Test 5)
- [ ] **Does OM compress?** (Test 6)
- [ ] **Does OM improve context retention?** (Tests 1-4 vs last-N-messages baseline)
- [ ] **Does OM handle edge cases?** (Tests 7-9)
- [ ] **Does OM degrade gracefully?** (Test 10)
- [ ] **Is OM cost acceptable?** (Track API token usage across all tests)
- [ ] **Is OM latency acceptable?** (Track response times at Observer/Reflector trigger points)

**Verdict:** If 7+ tests pass and no Critical gaps are blocking → OM is beta-ready.
If 5-6 tests pass → OM is alpha-ready (document known issues).
If <5 tests pass → OM needs more development before production use.
