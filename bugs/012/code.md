Implementation complete. The duplicate LLM reply bug was caused by `CreateMemoComment` calling `CreateMemo` internally, with both functions triggering `handleTicketAIResponse` for non-superusers on non-escalated tickets.

**Fix applied:**
- Added context key `skipTicketAIResponseKey` at lines 33-38
- Modified `CreateMemo` (lines 113-120) to check context and skip AI response when flag is set
- Modified `CreateMemoComment` (lines 480-483) to pass context with skip flag when calling `CreateMemo`

This ensures only one AI response is generated per comment, while preserving the AI response behavior for root-level memos.