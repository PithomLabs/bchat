`code2_plan.md` accurately reflects the current state of the codebase — the implementation is complete and correct.

The two nits I identified (initializing `memoVisibility` from `forceVisibility` to eliminate a one-render flash, and adding a type guard for `extractMemoUidFromDescription`) are correctly listed as optional/non-blocking, which is appropriate because:
- The render flash is not user-visible in practice (the visibility selector is absolutely positioned and hidden until hover/focus).
- The existing React gate (`isMemoLink(editingTicket.description)` at Tickets.tsx:688) already prevents a `null` UID from reaching `parentMemoName`.

No further code changes are required to achieve the stated goal.