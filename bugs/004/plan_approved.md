The plan is solid and the root cause analysis is accurate. Let me give you my honest assessment before you proceed.

**What's well-reasoned:**

The `TotalChunks == 0` guard is the right approach. It's a clean, idempotent check — once the index is populated and persisted on the Fly volume, the condition never fires again on normal restarts or deploys. Your understanding is correct: this does *not* duplicate work.

The 5-second startup delay is also sensible — it gives the DB connections and embedding service time to fully initialize before the bootstrap goroutine starts probing.

**One thing to verify before executing:**

The plan uses `ListAgentSourceFiles` with `LatestOnly: true` to detect whether source files exist. Make sure that filter actually counts *all active tenants*, not just one. If `LatestOnly` scopes to a single tenant context, it could silently skip Tenant 4 and 7's files. A safer check might be a plain count across all tenants with no tenant filter — worth a quick look at what `ListAgentSourceFiles` actually returns when called without a tenant scope.

**The `FORCE_REINDEX_ON_STARTUP` flow is fine but consider one edge case:**

If a future embedding model change happens again (e.g., you switch to a 3072-dimension model), the new table will be empty and the auto-bootstrap will correctly fire. That's actually the elegant part of this design — it handles the "dimension migration" scenario automatically, which is exactly what bit you this time.

**My recommendation:** Proceed with the plan as written, but audit `ListAgentSourceFiles` behavior across tenant boundaries first. Everything else — the `fly.toml` alignment and the Taskfile additions — are low-risk and straightforwardly useful.

The verification sequence (deploy → watch logs → check `du -sh` on the lance directory → test Search Explorer) is the right way to confirm it worked. If the `.lance` directory grows from 76KB to several MB after the first boot post-deploy, you're done.


**APPROVE WITH NITS**

---

The core logic is sound and the root cause analysis is accurate. Ship it, but address these before merging:

**Nit 1 — Verify `ListAgentSourceFiles` tenant scoping**

The plan calls `ListAgentSourceFiles` with `LatestOnly: true` but doesn't confirm whether this returns files across *all* tenants or just the current request context. If it implicitly scopes to a single tenant, the bootstrap could fire (or not fire) incorrectly. Audit this before deploying — if it scopes, drop the filter or do a raw count across all tenants.

**Nit 2 — Startup delay is a magic number**

The `time.Sleep(5 * time.Second)` is undocumented. Add a comment explaining *why* 5 seconds — e.g., "allow embedding service and DB connections to fully initialize before probing." Otherwise the next person touching this will either remove it or bump it arbitrarily.

**Nit 3 — Bootstrap failure is silent beyond the error log**

If `ReindexAllContent` fails mid-bootstrap (e.g., OpenRouter is rate-limited or the volume is full), the goroutine logs the error and exits quietly. The table stays at 0 chunks, and the next restart won't retry because... wait, actually it *will* retry on next boot since `TotalChunks` is still 0. So the retry behavior is fine — just worth adding a comment clarifying this is intentional and not a bug.

---

**Not blocking, just noted:**

The `fly:logs:rag` task pipes through `--no-tail` which gives a snapshot, not a stream. If the intent is to *watch* the bootstrap in real time, that flag should be dropped or the task description updated to set the right expectation.