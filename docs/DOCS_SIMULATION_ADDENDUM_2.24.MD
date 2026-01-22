# Simulation Analysis Addendum: Issues 24 & Ticket 5

> **Date:** 2026-01-21
> **Related:** Simulation ID 24, Ticket ID 5
> **Status:** Fixed

---

## Executive Summary

This document analyzes and proposes fixes for three interconnected issues discovered through simulation testing:

1. **Ticket/Memo Integration** - Ticket descriptions contain embedded text instead of memo references
2. **Session Persistence** - Customer phone corrections not persisted across messages
3. **Phone Correction Detection** - System fails to recognize "phone is actually X" patterns

---

## Issue 1: Ticket/Memo Integration (Ticket ID 5)

### Problem

Ticket ID 5 has this description:
```
/m/agent-escalation

Ticket: TKT-20260120-0001
Type: supervisor_request
Customer: John Smith
Phone: 555-987-6543
Issue: Customer requested supervisor...
```

**Expected:** `/m/esc-abc123xyz` (only a memo UID reference)

### Root Cause

`CreateEscalationTicket()` in `service.go:1507-1566`:
- Did NOT create a memo
- Used hardcoded `/m/agent-escalation` prefix
- Embedded all metadata as plain text in description field

### Fix

Rewrote `CreateEscalationTicket()` to:

1. Generate unique memo UID: `esc-<uuid12>`
2. Build structured memo content with all escalation details
3. Create memo with `Protected` visibility (visible to logged-in users)
4. Store ONLY `/m/<memo_uid>` in ticket description
5. Added fallback to old behavior if memo creation fails

**File:** `server/router/api/v1/agent/service.go` (lines 1510-1647)

---

## Issue 2: Customer Phone Not Persisted (Simulation ID 24)

### Evidence

In simulation transcript:
```
Human: "I think you got my phone wrong – it's still 555-123-4567"
```

The agent acknowledged but the corrected phone was lost on subsequent messages.

### Root Cause

In `service.go:596-611`, `UpdateAgentSession` was missing customer fields:

```go
// BEFORE (broken):
_, err = s.store.UpdateAgentSession(ctx, &store.UpdateAgentSession{
    ID:            session.ID,
    Phase:         &session.Phase,
    CurrentIntent: &session.CurrentIntent,
    UrgencyLevel:  &session.UrgencyLevel,
    MessageCount:  &msgCount,
    Messages:      session.Messages,
    // MISSING: CustomerPhone, CustomerName, CustomerLocation
})
```

### Fix

Added missing customer fields to session persistence:

```go
// AFTER (fixed):
_, err = s.store.UpdateAgentSession(ctx, &store.UpdateAgentSession{
    ID:               session.ID,
    Phase:            &session.Phase,
    CurrentIntent:    &session.CurrentIntent,
    UrgencyLevel:     &session.UrgencyLevel,
    MessageCount:     &msgCount,
    Messages:         session.Messages,
    CustomerName:     &session.CustomerName,
    CustomerPhone:    &session.CustomerPhone,
    CustomerLocation: &session.CustomerLocation,
})
```

**File:** `server/router/api/v1/agent/service.go` (lines 596-611)

---

## Issue 3: Phone Correction Detection

### Problem

The system didn't recognize correction patterns like:
- "I think you got my phone wrong – it's still 555-123-4567"
- "My number is actually 555-123-4567"
- "Correct my phone to 555-123-4567"

### Fix

Added phone correction detection patterns to `extractCollectedInfo()`:

```go
phoneCorrectionPatterns := []*regexp.Regexp{
    regexp.MustCompile(`(?i)(?:phone|number)\s+(?:is\s+)?(?:actually|should\s+be|still)\s+(\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`),
    regexp.MustCompile(`(?i)(?:correct|change|update)\s+(?:my\s+)?(?:phone|number)\s+to\s+(\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`),
    regexp.MustCompile(`(?i)(?:got|have)\s+(?:my\s+)?(?:phone|number)\s+wrong[^0-9]*(\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`),
    regexp.MustCompile(`(?i)(?:it's|its|it\s+is)\s+(?:still\s+)?(\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`),
}
```

These patterns are checked FIRST and override any previously captured phone.

**File:** `server/router/api/v1/agent/service.go` (lines 1353-1403)

---

## MD File Versioning

Created versioned snapshots from database with UNIX epoch suffix:

| File | Purpose |
|------|---------|
| `KB_1768964222.MD` | Knowledge base snapshot |
| `POLICY_1768964222.MD` | Policy rules snapshot |
| `SCRIPT_1768964222.MD` | Conversation flow snapshot |

**Location:** `docs/templates/examples/bulovas/`

---

## Policy Update

Added new rule to `POLICY_DRAFT.MD`:

```markdown
### Handle Customer Corrections
<!-- @rule: handle_corrections, priority: 1 -->

When customer corrects previously provided information (name, phone, address):

1. **Acknowledge explicitly:** "Got it, I've updated your phone number."
2. **Update the info:** Replace the old value with the corrected one
3. **Echo back:** Confirm the corrected info to show it was captured
```

---

## Files Modified

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/service.go` | Session persistence fix, memo/ticket integration, phone correction patterns |
| `docs/templates/examples/bulovas/POLICY_DRAFT.MD` | Added Handle Customer Corrections rule |

## Files Created

| File | Purpose |
|------|---------|
| `docs/templates/examples/bulovas/KB_1768964222.MD` | Version snapshot |
| `docs/templates/examples/bulovas/POLICY_1768964222.MD` | Version snapshot |
| `docs/templates/examples/bulovas/SCRIPT_1768964222.MD` | Version snapshot |
| `docs/DOCS_SIMULATION_ADDENDUM_2.24.MD` | This document |

---

## Verification Checklist

- [ ] Run simulation with phone correction scenario
- [ ] Verify customer phone persisted in `agent_sessions` table
- [ ] Verify escalation creates memo in `memo` table
- [ ] Verify ticket description is only `/m/<uid>` format
- [ ] Verify memo contains full escalation details
- [ ] Build completes without errors

---

*End of Addendum*
