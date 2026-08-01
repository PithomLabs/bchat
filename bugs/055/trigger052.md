# Trigger 052 — Test Memo for Bug 052 Inference

**Date:** 2026-08-01
**Purpose:** Happy-path test — create a ticket about bug 052 and verify RAG inference fires

---

## Ticket Title

```
InferResolutionForNewTicket returns empty — no similar tickets found
```

## Memo Content (paste into MemoEditor)

```
## Bug Description

InferResolutionForNewTicket is called when a new ticket is created, but it
always returns empty results even when similar tickets exist in the vector DB.

## Expected Behavior

When a new ticket is created about a topic that matches existing indexed tickets,
InferResolutionForNewTicket should find those similar tickets and populate
InternalNotes with a "## Suggested Resolution (Auto-generated)" section.
A system-authored "AI Suggestion" comment should also appear on the ticket.

## Actual Behavior

InferResolutionForNewTicket returns empty string. InternalNotes is not populated.
No AI Suggestion comment is created. The search queries the vector DB but finds
nothing because ticket content was never indexed into the vector DB in the first place.

## Root Cause

The IndexTicketContent function exists but is never called during ticket creation.
Without indexing, the vector DB has no ticket content to search against.
```

---

## Test Steps

1. Open Tickets page → click "New Ticket"
2. Select tenant
3. Paste the ticket title above
4. Click "Add description (Create Memo)"
5. Paste the memo content above → confirm
6. Click "Create Ticket"
7. Wait ~2 seconds for async inference
8. Open the ticket detail modal
9. Check "Comments" section for "AI Suggestion" comment
10. Check ticket detail page for "Internal Notes" yellow box

## Expected Results

| Check | Result |
|-------|--------|
| Ticket created | YES |
| `IndexTicketContent` called | YES (goroutine, `triggerInference=true`) |
| Vector DB has indexed content | YES (title + description blob) |
| `InferResolutionForNewTicket` runs | YES |
| AI Suggestion comment appears | Only if similar tickets exist in vector DB |
| InternalNotes populated | Only if similar tickets exist in vector DB |

## Note

If this is the **first** ticket about bug 052, inference will return empty —
no similar tickets to find. Create a second ticket with similar content after
the first one is indexed, and it should find the first.
