Findings:

**Root Cause: Confirmed ✓**
Your analysis is correct. `CreateMemoComment` at line 471 calls `s.CreateMemo(ctx, ...)`. Inside `CreateMemo` (lines 106-110), for non-superusers `handleAutoTicketCreation` returns `false` for comments, so the first `handleTicketAIResponse` fires. Then `CreateMemoComment` itself fires a second one at line 527. Two concurrent LLM calls with slightly different context (first has no parent relation yet, second does) → different answers, duplicate comments.

**Fix Assessment: Sound with minor additions needed**
The context-based skip flag is the right approach. It is surgical and avoids touching proto or store layers.

**Nits / additions to consider before implementation:**

1. The plan should note that both existing call sites already pass `context.Background()` to the goroutine (lines 109 and 527), so the fix only needs to change the `CreateMemo` call at line 471 and the check at line 107 — no change to the goroutine launch sites.

2. One additional read-only check worth adding: confirm `memo_service.go` has no other entry points that create comments (webhook handler, admin imports, etc.) to ensure no other duplicate-fire path exists. See `DispatchMemoCreatedWebhook` at line 97 in `CreateMemo`; if a webhook fired after `handleTicketAIResponse` saved a reply, it would not trigger AI because the reply is created via `s.Store.CreateMemo` (store layer), not `s.CreateMemo` (service layer).

3. The plan is correct that `CreateMemo` at line 1123 calls `s.Store.CreateMemo`, not `s.CreateMemo`, so the AI-generated reply memo itself won't re-enter the service and re-trigger. Good.

**Recommendation: Approve with nits.** The root cause and fix are correct. Add the two bullets above as documented considerations in the plan, then implement.