Following the review format requested at the end of the doc:

## Findings

**F1 — `MEMOS_DRIVER` env var is unverified and could invalidate the whole test silently. Severity: High. Blocker: yes, before running Phase 3.**
Phase 2's `crdb:migrate` boots the app via a CLI flag (`--driver=cockroach`). Phase 3 switches to setting `export MEMOS_DRIVER=cockroach` as an environment variable before `task run:cockroach` — a name that hasn't appeared anywhere in this whole thread's env var lists (`COCKROACH_DSN`, `RAG_PIPELINE_ENABLED`, `LANCEDB_STORAGE_PROVIDER`, etc., yes; `MEMOS_DRIVER`, no). If `task run:cockroach` doesn't actually bind to that exact variable name, the app could silently fall back to its default driver (likely SQLite) instead of erroring — and Phase 3's `/healthz` returning 200 would look like success while testing nothing about CockroachDB at all. This is the one finding that can produce a false-positive "everything passed" result, which is worse than an honest failure. **Fix:** `grep -rn "MEMOS_DRIVER" .` (or check the `run:cockroach` Taskfile target directly) before running Phase 3, and confirm it's actually read, not just assumed.

**F2 — No explicit handoff between Phase 2 and Phase 3; risk of a port conflict or a hung script. Severity: High. Blocker: yes, as written it may not be runnable end-to-end unattended.**
Phase 2 step 4 boots the app (`./build/memos --driver=cockroach ...`) to apply migrations. Phase 3 step 9 boots it again via `task run:cockroach`, described only as "in background or separate terminal." Two open questions: (a) does step 4's process exit on its own once migration completes, or does it keep running an HTTP server on 5230 that would collide with step 9's attempt to bind the same port? (b) if step 9 is meant to background itself, the plan doesn't show how (`&` + captured PID, `nohup`, a separate pane) — "in background or separate terminal" is an instruction for a human to improvise, not a copy-pasteable command, which matters if anyone tries to script this phase-by-phase. **Fix:** state explicitly whether Phase 2's process needs to be killed before Phase 3, and give the actual backgrounding command (with a PID capture for the teardown step in Phase 6, which currently has nothing to kill by PID either).

**F3 — Phase 5's rationale for skipping `crdb:init` conflates two different mechanisms. Severity: Medium. Not blocking.**
"Cluster settings persist in volume" is true for `feature.vector_index.enabled` / `jobs.*` / `sql.stats.*` (real cluster settings, durably stored). It is not true for `serial_normalization` — that's a session variable and never persists across connections, restart or not. The reason it's safe to skip here isn't persistence; it's that `serial_normalization` only matters at `CREATE TABLE` time, and Phase 5 isn't creating new tables. Correct outcome, imprecise reasoning — worth fixing the wording so a future maintainer adding a new migration after a restart doesn't assume the setting is durably "already handled" and skip re-prepending it where it's actually needed (which `migrator.go` already does correctly, but the doc's phrasing could mislead someone reasoning from this paragraph alone).

**F4 — Phase 4 has two very different failure modes bundled under one gate. Severity: Medium. Not blocking, but affects triage speed.**
The RAG round-trip depends on a live, paid, external OpenRouter call. A Phase 4 failure could mean a CockroachDB/vector-search bug, or it could mean an expired API key / rate limit / network blip — nothing to do with the database this whole thread has been about. The troubleshooting table already has an OpenRouter row, so this isn't uncovered, but the Gate Criteria table doesn't separate "DB path broken" from "embedding provider unavailable," which matters for whoever's triaging a failed run at 2am. Small fix: add a line to Phase 4's gate criteria distinguishing "reindex/search errors reference CockroachDB" vs. "errors reference OpenRouter/HTTP 429/401."

**F5 — Cosmetic connection-string inconsistency. Severity: Low.**
Phase 1 uses `bchat_user@localhost` (no password); the Infrastructure table and Phase 3 use `bchat_user:bchat_pass@localhost`. Harmless under `--insecure` mode (password isn't checked), but worth tidying so nobody spends time debugging a "wrong password" that was never actually validated.

## What's already resolved from prior rounds — good to confirm explicitly

- The `set -e` fix and the SQL-readiness retry loop (30 attempts, 2s apart) are both now in place — this closes out both blocking items from the last review.
- "Reindex triggered by admin API, singleton per-tenant" directly answers the open question about how likely the concurrent-`CREATE VECTOR INDEX` race actually is in practice — much narrower than "every replica on every boot," as suspected. Still worth the concurrent-test as cheap insurance (an admin double-clicking "reindex" is a realistic enough scenario), but it's no longer a priority blocker.
- `SHOW JOBS` for failed schema changes, and the A1–A4 migration-replay scenarios (including corrupted-history recovery), directly satisfy the fix-forward-migration-test-fixture ask from several rounds back.

## Overall verdict

**APPROVE WITH NITS** — F3, F4, F5 are wording/triage improvements, not redesigns. **F1 and F2 should be confirmed/fixed before actually executing Phase 3**, since F1 specifically can produce a passing result that proves nothing, which is a worse outcome than a clean failure.

## Additional tests before cloud deployment (carried forward, still open)

- Vector index backfill timing on a realistically-sized populated table (still deferred every round so far — fine for local E2E, but needed before Basic tier goes live).
- TLS/SCRAM auth parity against the actual Basic tier connection (cloud-only, not exercisable locally).
- The written safety-gate confirmation on the live Basic cluster's data (from several rounds ago) — unrelated to this plan's scope but still outstanding before `plan_cloud.md` executes anything against production.