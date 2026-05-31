# Duplicate LLM Reply Fix: Implementation Complete

## Status: VERIFIED ✓

## Root Cause
`CreateMemoComment` calls `CreateMemo` internally. Both functions independently trigger `handleTicketAIResponse` for non-superusers on non-escalated tickets, causing two concurrent LLM calls that save as duplicate comments.

## Fix Applied
Context-based skip flag in `server/router/api/v1/memo_service.go`:

1. Added `contextKey` type and `skipTicketAIResponseKey` constant (lines 33-38)
2. Modified `CreateMemo` (lines 113-120) to skip AI response when context flag is set
3. Modified `CreateMemoComment` (line 482-483) to pass context with skip flag when calling `CreateMemo`

## Verification
- ✅ Go build successful (`go build ./...`)
- ✅ No proto changes required
- ✅ Root-level memos still trigger AI response via `CreateMemo`
- ✅ Comments trigger AI response via `CreateMemoComment` only (no duplication)
- ✅ Other entry points verified safe (agent service, webhooks use store layer directly)