# Trigger 052 — Second Ticket (Should Find First via Inference)

**Date:** 2026-08-01
**Purpose:** Create this AFTER trigger052.md ticket is indexed. Inference should find the first ticket and generate an AI Suggestion.

---

## Ticket Title

```
Ticket inference returns empty results despite existing similar tickets
```

## Memo Content (paste into MemoEditor)

```
## Problem

When creating a new ticket, the AI Suggestion system does not surface similar
past tickets even though matching tickets exist in the vector database.

The InferResolutionForNewTicket function is called during ticket creation but
returns an empty string. No "## Suggested Resolution (Auto-generated)" section
appears in InternalNotes, and no system-authored AI Suggestion comment is created.

## Steps to Reproduce

1. Create a first ticket about a topic (e.g., RAG indexing issues)
2. Wait for the ticket to be indexed into the vector DB
3. Create a second ticket with a similar title/description
4. Open the second ticket detail modal

## Expected

The second ticket should show an AI Suggestion comment referencing the first
ticket, with a "100% match" score and a content preview.

## Actual

No AI Suggestion comment appears. The inference search returns empty results
even though the first ticket's content is now in the vector DB.
```

---

## Test Steps

1. First, create and index the trigger052.md ticket (wait ~2s for goroutine)
2. Open Tickets → click "New Ticket"
3. Select **same tenant** as first ticket
4. Paste the ticket title above
5. Click "Add description (Create Memo)"
6. Paste the memo content above → confirm
7. Click "Create Ticket"
8. Wait ~2 seconds for async inference
9. Open the ticket detail modal
10. Check "Comments" section — should now show "AI Suggestion" comment

## Expected Results

| Check | Result |
|-------|--------|
| Ticket created | YES |
| `IndexTicketContent` called | YES |
| `InferResolutionForNewTicket` runs | YES |
| Search 1 (tickets, MinScore=0.7) | Should find first ticket (keywords match) |
| AI Suggestion comment appears | **YES** — references first ticket |
| InternalNotes contains "Suggested Resolution" | **YES** |
| Match score shown | ~100% (same keywords in both titles) |

## Why This Works

Both tickets share keywords: `ticket`, `inference`, `returns`, `empty`, `results`,
`similar`. The `controlledEmbeddingService` in tests and the real embedding service
in production both produce high-similarity vectors for texts with shared vocabulary.

First ticket indexed content: `# InferResolutionForNewTicket returns empty...\n\n## Bug Description...`
Second ticket indexed content: `# Ticket inference returns empty results...\n\n## Problem...`

Both contain overlapping terms that produce high cosine similarity.
