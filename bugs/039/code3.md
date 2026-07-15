# Implementation: memstate Integration — Revision 3 (FINAL)

## Source documents
- `code2.md` — base revision
- `code2_review_deepseek.md` — APPROVED WITH NITS
- `code2_review_hy3.md` — APPROVE WITH REQUIRED FIXES (1 regression)

## What changed from code2.md → code3.md

### HIGH: Email regression in getContactState (hy3 F1)

The revised `getContactState` in code2.md omits `state.Email = info.Email`. The current code sets it at line 3490, and `buildSection0` renders it. Without it, emails vanish from the banner.

**Fix:** Add `state.Email = info.Email` back in the revised `getContactState`.

**Final getContactState:**
```go
func getContactState(session *store.AgentSession, validatedPhone string) ContactState {
    state := ContactState{}
    if session == nil || len(session.Messages) == 0 {
        return state
    }
    info := extractCollectedInfo(session.Messages, validatedPhone)
    if info == nil {
        return state
    }
    // Prefer session fields (updated by memstate) over first-match extraction
    if session.CustomerName != "" {
        state.Name = session.CustomerName
    } else {
        state.Name = info.Name
    }
    if session.CustomerPhone != "" {
        state.Phone = session.CustomerPhone
    } else {
        state.Phone = info.Phone
    }
    if session.CustomerLocation != "" {
        state.Address = session.CustomerLocation
    } else {
        state.Address = info.Address
    }
    state.Email = info.Email // always from extractCollectedInfo (no memstate tracking for email)
    state.HasName = state.Name != ""
    state.HasEmailOrPhone = state.Phone != "" || state.Email != ""
    state.IsComplete = state.HasName && state.HasEmailOrPhone
    return state
}
```

### No other changes

All other code2.md steps remain as-is. Both reviewers confirmed correctness.

---

## Implementation steps (unchanged from code2.md, plus email fix)

| Step | Files | Changes |
|------|-------|---------|
| 1 | `go.mod`, `go.sum` | Remove `replace` directive, run `go mod tidy` |
| 2 | `server/.../agent/service.go` | Drop weak markers from `latestNamePatterns[0]`; run `gofmt -w` |
| 3 | `server/.../agent/service.go` (processChat memstate block) | Update `session.CustomerName`/Phone/Location from latest extractors |
| 4 | `server/.../agent/service.go` (getContactState) | Prefer session fields over first-match extraction; **keep `state.Email = info.Email`** |
| 5 | `server/.../agent/memstate_test.go` | Fix `TestFactsNilByDefault` (use GetOrCreate), fix `TestExtractLatestPhoneExcludesTenant` (10-digit), add `TestExtractLatestPhoneCorrection` |
| 6 | Verify | `go vet`, `go test`, `gofmt -l` clean |
