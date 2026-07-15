# memstate Integration — Implementation Sign-Off

**Date:** 2026-07-15
**Status:** COMPLETE — all tests pass, no regressions
**Default:** ENABLED (`MEMSTATE_ENABLED` defaults to `true`)

## Summary

Added deterministic belief revision to bchat's external chat agent using the `memstate` library. Customer facts (name, phone, address) are tracked in-memory per session. When a customer changes their mind, the old fact is automatically superseded and only the current truth is injected into the LLM prompt.

**Memstate is enabled by default** in all environments (local, fly.io production). Set `MEMSTATE_ENABLED=false` to disable.

## Review history

| Round | Verdict | Key findings |
|-------|---------|--------------|
| plan.md → plan_review | APPROVED WITH NITS / REWORK | Import cycle, belief revision broken, raw messages as facts |
| plan2.md → plan2_review | APPROVED WITH NITS / REWORK | extractLatestField naive, import cycle, panic coverage |
| plan3.md → plan3_review | READY / APPROVE W/ FIXES | Wrong submatch index, wrong file reference, supersession scoring |
| plan4.md → plan4_review | APPROVED | Re-add Facts() for tests, use validatedCompanyPhone, drop standalone pattern #3 |
| code.md → code_review | APPROVED WITH NITS / REWORK | Test quality, Section 0 vs 0.5a contradiction, gofmt |
| code2.md → code2_review | APPROVED WITH NITS / APPROVE | Email regression in getContactState |
| code3.md → code3_review | APPROVED — no findings | Final |

## Files changed

### New files
| File | Lines | Purpose |
|------|-------|---------|
| `store/safe_memory.go` | 97 | Thread-safe wrapper: mutex + recover in Add/Prompt/Facts; deep-copy Facts() |
| `server/.../agent/memstate_test.go` | 198 | 13 tests: 4 supersession, 2 nil/init, 7 extraction |

### Modified files
| File | Change |
|------|--------|
| `go.mod` | Added `github.com/PithomLabs/memstate v0.0.0-20260714224641-ff73beb8902f` |
| `go.sum` | Populated with memstate checksums |
| `store/agent.go:268` | Added `Facts *SafeMemory` field to `AgentSession` |
| `fly.toml:27` | Added `MEMSTATE_ENABLED = 'true'` |
| `.env:82` | Added `MEMSTATE_ENABLED=true` |
| `.env.example:69` | Added `MEMSTATE_ENABLED=true` |
| `server/.../agent/service.go` | `isMemstateEnabled` (default true via `getEnvBool`), `extractLatest*` functions, memstate block in `processChat`, Section 0.5a in `buildSystemPrompt`, revised `getContactState` |

## Architecture

```
processChat (per turn)
  │
  ├─ extractCollectedInfo()        ← existing, first-match (sets session.CustomerName etc.)
  │
  ├─ extractLatest*()              ← NEW: newest-first, user-role only, with safeguards
  │   ├─ extractLatestName()       ← I'm/I am/my name is/call me + here/speaking
  │   ├─ extractLatestPhone()      ← correction patterns first, then main, tenant excluded
  │   └─ extractLatestAddress()    ← match[0], len>10 guard
  │
  ├─ session.CustomerName = name   ← NEW: update session field for Section 0 consistency
  └─ session.Facts.Add(...)        ← NEW: feeds memstate (supersession automatic)

buildSystemPrompt (per turn)
  │
  ├─ Section 0:    Contact info    ← existing; now uses session fields (latest) via getContactState
  ├─ Section 0.5a: Facts prompt    ← NEW: "FACTS EXTRACTED FROM CUSTOMER"
  └─ Section 0.5:  OM injection    ← existing
```

## Key design decisions

1. **`SafeMemory` in `store` package** — avoids import cycle
2. **`recover()` in each method** — Add, Prompt, and Facts all protected
3. **`Facts()` returns deep copy** — pointer safety after mutex release
4. **Standalone name pattern #3 excluded** — prevents false positives under newest-first
5. **Weak markers (`it's`/`this is`) excluded** — prevents "it's broken" → "broken" as name
6. **No duplicate-fact guard** — memstate deduplicates via supersession
7. **Default-enabled** — `MEMSTATE_ENABLED` defaults to `true` via `getEnvBool`; set to `false` to disable
8. **Default threshold 0.55 works** — no tuning needed
9. **Session fields updated from latest extractors** — Section 0 and 0.5a agree

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `MEMSTATE_ENABLED` | `true` | Enable/disable belief revision. Set to `false` to disable. |

Uses the existing `getEnvBool` helper (from `om_config.go`) which accepts `true/1/yes/on` and `false/0/no/off`.

**To disable in any environment:**
```bash
export MEMSTATE_ENABLED=false
```

Or in fly.toml:
```toml
[env]
  MEMSTATE_ENABLED = 'false'
```

## Verification

| Check | Result |
|-------|--------|
| `go vet ./...` | Clean |
| `gofmt -l` | Clean |
| `go test ./...` | All pass (0 failures) |
| Supersession: John → Jonathan | Supersedes (1 current fact) |
| Supersession: phone change | Supersedes (1 current fact) |
| Supersession: name + location | Both current (no cross-topic supersession) |
| Supersession: name + billing | Both current (different topics) |
| `TestFactsNilByDefault` | Facts nil when disabled (explicit override) |
| `TestFactsInitializedWhenEnabled` | Facts initialized when enabled |
| `TestExtractLatestName` | Returns "Jonathan" (newest-first) |
| `TestExtractLatestNameSkipsAssistant` | Skips assistant messages |
| `TestExtractLatestPhone` | Returns latest number |
| `TestExtractLatestPhoneCorrection` | Returns corrected number |
| `TestExtractLatestPhoneExcludesTenant` | Excludes tenant phone |
| `TestExtractLatestAddress` | Returns latest address |

## Known limitations

1. **Session.Messages race** — pre-existing; `extractLatest*` range over unlocked slice; concurrent same-session turns can race. SafeMemory mutex covers only Facts. Deferred to separate fix.
2. **Email not tracked by memstate** — always first-match from `extractCollectedInfo`. No session field or memstate tracking.
3. **`latestNamePatterns` narrower than `extractCollectedInfo`** — "This is John" captured by Section 0 but not by 0.5a. Documented tradeoff.
4. **Per-turn re-add** — unchanged facts re-added each turn (idempotent, bounded by 50-turn cap).
