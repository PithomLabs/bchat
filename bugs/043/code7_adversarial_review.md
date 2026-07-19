# Adversarial Code Review — Code 7 Security Hardening (F1–F6)

**Date:** 2026-07-20
**Reviewer:** Senior Go Architect (bchat)
**Files reviewed (live source):**
- [main.go](file:///home/chaschel/Documents/go/bchat/bin/memos/main.go) — `getOrCreateEncryptionKey`, `rotateKeysCmd`, `agentServiceForRotation`
- [service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go) — `NewService`, `decryptForRotation`, `ReEncryptOnStartup`, `getTranscriptSigningSeed`, `detectPromptInjection`, `appendInjectionGuardrail`, `buildSystemPrompt`, `buildRAGSystemPrompt`, production callers of `ExtractContactInfoFull`
- [lead_llm.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/lead_llm.go) — `ExtractContactInfoLLM`, `ExtractContactInfoLLMCached`, `ExtractContactInfoFull`
- [lead_extraction_test.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/lead_extraction_test.go) — 3 test call sites
- [bridge_auth.go](file:///home/chaschel/Documents/go/bchat/store/db/sqlite/bridge_auth.go) — `ListBridgeAuthKeys`, `RevokeBridgeAuthKey`, `CreateBridgeAuthKey`

**Plan reviewed:** [code7_plan.md](file:///home/chaschel/Documents/go/bchat/bugs/043/code7_plan.md)
**Prior review:** [code7_plan_review.md](file:///home/chaschel/Documents/go/bchat/bugs/043/code7_plan_review.md)
**Impl summary:** [code7_summary.md](file:///home/chaschel/Documents/go/bchat/bugs/043/code7_summary.md)

---

## Verdict: APPROVED WITH NITS

All six findings (F1–F6) are implemented as planned. The code is build-clean, vet-clean, and tests pass. The core security invariants (no key divergence, no silent partial rotation, no auto-salt on error, backup-key gating, prompt-injection guardrail on lead extraction) are correctly enforced. Two MEDIUM items below deserve follow-up but do not block this merge.

---

## Findings

### R1 — MEDIUM: Bridge key rotation is NOT idempotent on resume (F2)

**Location:** [service.go:337-384](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L337-L384)

**Description:** The bridge key rotation loop does `RevokeBridgeAuthKey` → `CreateBridgeAuthKey` with the same `key.KeyID`. On a **resume** after cancellation:

1. First run: key is revoked (status='revoked'), new row inserted with status='active'. ✅
2. Second run (resume): `ListBridgeAuthKeys` returns **all** keys including already-revoked ones (the SQLite query at [bridge_auth.go:68-74](file:///home/chaschel/Documents/go/bchat/store/db/sqlite/bridge_auth.go#L68-L74) has no `WHERE status = 'active'` filter). The code will try to `decryptForRotation` on the **revoked** old row (which still has the backup-key ciphertext). `decryptForRotation` succeeds via `backupSvc`. Then `RevokeBridgeAuthKey` is called again — but the `WHERE status = 'active'` guard makes this a no-op (returns nil, not error). Then `CreateBridgeAuthKey` inserts a **second** active row with the same `key_id`.

**Impact:** Duplicate rows per key_id accumulate on each re-run. The `GetBridgeAuthKey` query returns the first match without `LIMIT 1` ordering guarantees, so which row is "live" becomes non-deterministic. Not a security vulnerability per se (the plaintext is the same), but it's data corruption and will confuse the HMAC validation path if it picks up a stale ciphertext.

**Fix:** Either:
- Filter `ListBridgeAuthKeys` to `status = 'active'` keys only, OR
- Before the revoke+create pair, check if the key already decrypts under the primary (the `decryptForRotation` primary fallback succeeds) and `continue` — exactly as the API-key and transcript loops do implicitly (re-encrypt is a no-op overwrite on the same row). The bridge loop is unique because it does revoke+insert instead of update-in-place.

> [!IMPORTANT]
> This is the most actionable finding. On the happy path (single clean run) it's harmless, but `failed>0` → re-run is now the *expected* operator flow, and it will create duplicates.

---

### R2 — MEDIUM: `llmExtractionCache` is a bare `map` with no mutex

**Location:** [lead_llm.go:207](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/lead_llm.go#L207)

**Description:** `var llmExtractionCache = make(map[string]*ExtractionResult)` is a package-level `map` read/written from `ExtractContactInfoLLMCached` which is called from concurrent HTTP request goroutines. Go maps are not safe for concurrent use; this is a data race that can panic the server.

**Impact:** Not introduced by Code 7 (the `flagged` key suffix was added, but the underlying race pre-exists). Mentioning it because Code 7 touches this code path and the cache key format, which means the review should note it. A `sync.Map` or `sync.RWMutex`-guarded map would fix it.

> [!NOTE]
> Pre-existing issue, not a Code 7 regression. But since Code 7 modified the cache key format, it's worth flagging for follow-up.

---

### R3 — LOW: `getOrCreateEncryptionKey` returns `key` on `MkdirAll` failure (pre-panic escape hatch)

**Location:** [main.go:342-345](file:///home/chaschel/Documents/go/bchat/bin/memos/main.go#L342-L345)

**Description:** If `os.MkdirAll(dataDir, 0700)` fails, the function logs a warning and `return key` — a locally generated key that is never persisted. This is exactly the kind of divergent-key escape the F1 fix was designed to eliminate. However:

- In practice, if `MkdirAll` fails, the `O_EXCL` that follows would also fail, leading to the final `panic`. But the code short-circuits *before* the loop.
- The `return key` at line 344 means the server starts successfully with an in-memory-only key that no subsequent process can ever recover.

**Fix:** Replace `return key` with `panic(fmt.Sprintf("cannot create data directory for encryption key: %v", err))`. If the data dir doesn't exist and can't be created, starting the server is unsafe regardless.

> [!WARNING]
> This is a residual divergence path that the F1 plan's "last-resort is fatal" logic does not cover because it occurs *before* the retry loop.

---

### R4 — LOW: `agentServiceForRotation` double-checks `failed > 0` redundantly

**Location:** [main.go:191-205](file:///home/chaschel/Documents/go/bchat/bin/memos/main.go#L191-L205)

**Description:** The wrapper method calls `svc.ReEncryptOnStartup(ctx)` which already returns a non-nil error when `failed > 0` (F2 fix at [service.go:421-427](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L421-L427)). Then the wrapper *also* checks `if failed > 0` at line 199 and returns its own error. This means a partial failure generates **two** error messages with different wording:
- From `service.go`: `"key rotation partially failed: N of M secrets not re-encrypted; ..."`
- From `main.go`: `"key rotation partially failed: N of M secrets not re-encrypted"`

The wrapper's `reErr` check at line 195 catches the service error first (since `failed > 0` returns non-nil `err`), so the `failed > 0` branch at line 199 is **dead code** — it is never reached. It's harmless but confusing.

**Fix:** Remove the dead `failed > 0` check at lines 199-201 or, if kept for defense-in-depth, add a comment explaining it's a belt-and-suspenders guard.

---

### R5 — LOW: `detectPromptInjection` substring `"system:"` false-positive surface

**Location:** [service.go:2416](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L2416)

**Description:** The pattern `"system:"` will match any customer message containing common phrases like:
- `"My operating system: Windows 10"`
- `"The sound system: Bose"`
- `"filing system: alphabetical"`

The plan acknowledged this (`"also matches 'system prompt:' substring, acceptable"`) but the broader false-positive surface of bare `"system:"` is wider than anticipated. Since detection is heuristic-only and the guardrail (not blocking) is the primary defense, this is acceptable — but the false-positive rate may generate noisy `WARN` logs.

**Mitigation:** Consider tightening to `"system: ignore"` or `"\nsystem:"` (line-start) in a future pass to reduce noise without losing the core F5 bypass detection.

---

### R6 — INFO: `getTranscriptSigningSeed` re-reads `os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP")` despite `backupKeyActive` gate

**Location:** [service.go:1982](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L1982)

**Description:** The function correctly gates entry into the backup path on `s.backupKeyActive` (F3). But inside the block, it still reads `os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP")` to get the actual key value. This is correct behavior (the flag gates *whether* to try; the env holds *what* to try), but it means the function's effective security depends on the env var not being mutated at runtime. Since env vars are process-global and immutable in normal operation, this is fine — but worth documenting.

---

### R7 — INFO: No sleep/backoff between O_EXCL retry attempts

**Location:** [main.go:348-362](file:///home/chaschel/Documents/go/bchat/bin/memos/main.go#L348-L362)

**Description:** The `for attempt := 0; attempt < 2; attempt++` loop has no `time.Sleep` between iterations. If a peer's key file is empty/short (still being written), the retry will fire immediately and almost certainly see the same empty file, wasting the retry. A 50-100ms `time.Sleep` between attempts would significantly improve the chance of adopting the peer's valid key.

**Impact:** Minimal in practice (single-process-per-datadir is the norm), but the plan explicitly designed this retry for multi-process safety.

---

## Summary Table

| ID | Severity | Location | Description | Action |
|----|----------|----------|-------------|--------|
| R1 | MEDIUM | service.go:337-384 | Bridge key rotation creates duplicates on resume | Follow-up: filter active-only or skip already-rotated |
| R2 | MEDIUM | lead_llm.go:207 | `llmExtractionCache` bare map is racy | Follow-up: use `sync.Map` or guarded map |
| R3 | LOW | main.go:342-345 | `MkdirAll` failure returns divergent key | Nit: change to panic |
| R4 | LOW | main.go:199-201 | Dead code: `failed > 0` check never reached | Nit: remove or add comment |
| R5 | LOW | service.go:2416 | `"system:"` pattern broader FP than expected | Accept for now; tighten later |
| R6 | INFO | service.go:1982 | Env re-read inside `backupKeyActive` gate | Document; no change needed |
| R7 | INFO | main.go:348-362 | No backoff between O_EXCL retries | Consider 50ms sleep |

---

## Correctness Verification (Positive)

The following were verified correct against live source:

| Claim | Verified |
|-------|----------|
| F1: O_EXCL-failure branch no longer removes peer's file | ✅ [main.go:350-362](file:///home/chaschel/Documents/go/bchat/bin/memos/main.go#L350-L362) — `continue` on empty/short, no `os.Remove` |
| F1: Last-resort panics instead of `return key` | ✅ [main.go:387-388](file:///home/chaschel/Documents/go/bchat/bin/memos/main.go#L387-L388) |
| F2: `decryptForRotation` tries backup then primary | ✅ [service.go:253-264](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L253-L264) |
| F2: `failed > 0` returns non-nil error | ✅ [service.go:421-427](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L421-L427) |
| F3: `backupKeyActive` set inside `EncryptionMasterKey != ""` block | ✅ [service.go:124-127](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L124-L127) |
| F3: `getTranscriptSigningSeed` gates on `s.backupKeyActive` | ✅ [service.go:1979](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L1979) |
| F4: `flagged bool` threaded through all 3 functions | ✅ lead_llm.go:51, 210, 239 |
| F4: Cache key includes `flagged` | ✅ [lead_llm.go:213](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/lead_llm.go#L213) |
| F4: All 3 test sites updated to `false` | ✅ lead_extraction_test.go:246, 261, 273 |
| F4: Both production callers pass `session.FlaggedInput` | ✅ service.go:4483, 5164 |
| F5: 4 high-precision patterns re-added | ✅ [service.go:2416-2419](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L2416-L2419) |
| F6: Error path sets `encryptionService = nil`, returns early | ✅ [service.go:101-106](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L101-L106) |
| F6: First-run (`secret == nil && err == nil`) still generates salt | ✅ [service.go:108-116](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go#L108-L116) |
| `appendInjectionGuardrail` shared across `buildSystemPrompt`, `buildRAGSystemPrompt`, and `ExtractContactInfoLLM` | ✅ service.go:3332, 3827; lead_llm.go:86 |

---

**Bottom line:** The implementation is solid and addresses all six original findings faithfully. R1 (bridge key duplication on resume) is the most actionable follow-up; R3 (MkdirAll escape hatch) is a small residual divergence path. Neither blocks the merge.
