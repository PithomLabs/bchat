All changes implemented and verified:

**Backend:** `server/router/api/v1/memo_relation_service.go:ListMemoRelations` (lines 78-89) now has `var memoFilter *string` with admin bypass via `else if !isSuperUser(currentUser)`, matching the pattern in `ListMemoComments`.

**Frontend:** `MemoEditor/index.tsx` (lines 51-54) has `forceVisibility` and `hideVisibilitySelector` props, with visibility logic at lines 128-147 and selector hiding at lines 597-603.

**Tickets page:** Updated to use `forceVisibility={Visibility.PROTECTED}` and `hideVisibilitySelector={true}` for comment editor (lines 709-710), removing the redundant `defaultVisibility={Visibility.PUBLIC}`.