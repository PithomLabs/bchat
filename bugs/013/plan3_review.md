plan3.md meets the minimum viable bar and is approved for implementation.

  Two implementation clarifications—no further plan revision required:

  - “Atomic” means all authorization checks occur before mutation. Full database transaction handling is not
    required for this fix.

  - resolveRootMemo must use the existing derived Memo.ParentID, which represents MemoRelationComment, and return
    an error rather than authorize against an unresolved intermediate memo.

  The plan now adequately covers the reported workflow, preserves normal user ownership, gives Host/Admin cross-
  user read access, closes existing resource endpoint authorization gaps, and includes proportionate tests.
  Further review would have diminishing value.

