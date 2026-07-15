# Implementation: memstate Integration — Revision 2

## Source documents
- `code.md` — original implementation
- `code_review_deepseek.md` — APPROVED WITH NITS
- `code_review_hy3.md` — REWORK (1 critical, 2 high)

## What changed from code.md → code2.md

### CRITICAL: Build non-reproducible (hy3 C1)

Local `replace` directive points to `/home/chaschel/Documents/go/mnem-main/go`. Remove it, run `go mod tidy` to fetch the published module and populate `go.sum`.

### HIGH: Section 0 vs Section 0.5a contradiction (hy3 H2 — corrected)

hy3 claimed Section 0 uses `session.CustomerName` (set only-if-empty). Actually, Section 0 calls `extractCollectedInfo` fresh from `session.Messages` — but with **first-match-wins** for name/address. Section 0.5a uses `extractLatestName` with **newest-first**. After "I'm John" → "call me Jonathan":

- Section 0: "Customer Name: John" (first-match)
- Section 0.5a: "Customer name is Jonathan" (newest-first)

The LLM sees both. **Fix:** In the memstate block in `processChat`, update `session.CustomerName`/`Phone`/`Location` from the latest extractors. Then modify `getContactState` to prefer session fields when set.

**processChat memstate block (revised):**
```go
if isMemstateEnabled() && session.Facts != nil {
    if name := extractLatestName(session.Messages); name != "" {
        session.CustomerName = name
        session.Facts.Add("Customer name is " + name)
    }
    if phone := extractLatestPhone(session.Messages, validatedCompanyPhone); phone != "" {
        session.CustomerPhone = phone
        session.Facts.Add("Customer phone is " + phone)
    }
    if addr := extractLatestAddress(session.Messages); addr != "" {
        session.CustomerLocation = addr
        session.Facts.Add("Customer location is " + addr)
    }
}
```

**getContactState (revised):**
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
    state.HasName = state.Name != ""
    state.HasEmailOrPhone = state.Phone != "" || info.Email != ""
    state.IsComplete = state.HasName && state.HasEmailOrPhone
    return state
}
```

### HIGH: Pre-existing session.Messages race (hy3 H3)

`extractLatest*` range over `session.Messages` with no lock; concurrent same-session turns can race on the append at line 2095. SafeMemory's mutex only protects `Facts`. This is pre-existing (identified in plan2-4, scoped out). Document it; do not fix in this PR.

### MEDIUM: Weak name markers (hy3 M1)

`latestNamePatterns[0]` includes `it's`/`this is`/`it is` which match phrases like "it's broken" → extracts "broken" as name. Drop them. Keep only: `I'm`, `I am`, `my name is`, `call me`.

**Revised pattern:**
```go
latestNamePatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)(?:I'm|I am|my name is|call me)\s+([A-Za-z][a-z]+(?:\s+[A-Za-z][a-z]+)?)`),
    regexp.MustCompile(`(?i)^([A-Za-z][a-z]+(?:\s+[A-Za-z][a-z]+)?)[,.]?\s+(?:here|speaking)`),
}
```

### MEDIUM: TestFactsNilByDefault tests compiler (DeepSeek #1)

Uses struct literal instead of `GetOrCreate`. Fix: use `NewMemorySessionStore` + `GetOrCreate`.

### MEDIUM: Missing phone correction test (DeepSeek #2)

Add `TestExtractLatestPhoneCorrection` that uses "correct my number to" keyword.

### LOW: gofmt (both L1)

Run `gofmt -w` on service.go.

### LOW: Tenant exclusion test (hy3 L2)

`TestExtractLatestPhoneExcludesTenant` uses 7-digit "555-9999" which never matches the 10-digit pattern. Use 10-digit phone.

### LOW: `// indirect` annotation (DeepSeek #4)

Run `go mod tidy` and verify the annotation is correct.

---

## Implementation steps

| Step | Files | Changes |
|------|-------|---------|
| 1 | `go.mod`, `go.sum` | Remove `replace` directive, run `go mod tidy` |
| 2 | `server/.../agent/service.go` | Drop weak markers from `latestNamePatterns[0]`; run `gofmt -w` |
| 3 | `server/.../agent/service.go` (processChat memstate block) | Update `session.CustomerName`/Phone/Location from latest extractors |
| 4 | `server/.../agent/service.go` (getContactState) | Prefer session fields over first-match extraction |
| 5 | `server/.../agent/memstate_test.go` | Fix `TestFactsNilByDefault` (use GetOrCreate), fix `TestExtractLatestPhoneExcludesTenant` (10-digit), add `TestExtractLatestPhoneCorrection` |
| 6 | Verify | `go vet`, `go test`, `gofmt -l` clean |
