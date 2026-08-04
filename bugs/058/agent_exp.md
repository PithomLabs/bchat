# Agent Experience: Fascinating Insights from Bug 058

**Date:** August 4, 2026  
**Agent:** opencode (mimo-v2-pro)  
**Context:** CockroachDB Cloud Basic deployment, RAG pipeline debugging, build optimization

---

## 1. Bugs Are Layers, Not Points

The most fascinating insight: this wasn't a bug. It was a stack of five independent failures, each masking the one beneath it.

```
Layer 5: RAG search returns 0 results (user-visible symptom)
    ↓
Layer 4: resolveQueryVersion() returns nil (no active version)
    ↓
Layer 3: Reindex checkpoint can't save (ON CONFLICT, no unique constraint)
    ↓
Layer 2: 6 unique indexes missing (DDL backfill timed out)
    ↓
Layer 1: Stale vector DB state from previous session (nil initialization)
```

A junior developer would have fixed Layer 5 and declared victory. The real fix required understanding all five layers. As an agent, I had to resist the urge to apply the first fix that worked and instead trace the entire causal chain.

**Lesson:** When a symptom appears, ask "what caused this?" at least five times before acting.

---

## 2. Cloud Serverless Is a Different Database

The same `CREATE UNIQUE INDEX` statement that completes in 50ms locally takes 25-60 minutes on CockroachDB Cloud Basic. This isn't a performance difference — it's a category difference.

On a local instance, you can run DDL in a transaction and commit. On Cloud Basic, DDL backfills run asynchronously. The `CREATE TABLE` succeeds immediately. The `CREATE INDEX` starts a background job that may take an hour. If the migration window closes before the job completes, you have a table without constraints.

The application never noticed. `IF NOT EXISTS` is idempotent — it returns "already exists" whether the index is there or not. The app operated in a degraded state for hours, silently skipping operations that required the missing constraints.

**Lesson:** Cloud Serverless DDL is not like local DDL. Your migration strategy must account for asynchronous backfills.

---

## 3. Idempotency Hides Failures

`IF NOT EXISTS` is a safety feature. It prevents errors when you re-run a migration. But it also prevents signals when something went wrong.

The migration ran. The tables were created. The indexes were supposed to be created. Some of them timed out. Because `IF NOT EXISTS` returned "already exists" (whether true or not), the application never complained. It just operated without the constraints.

This is a design trade-off: resilience at the cost of observability. The application is robust against re-runs but blind to partial failures.

**Lesson:** Idempotent DDL must be paired with startup verification that checks not just "does this table exist?" but "does this table have the constraints we expect?"

---

## 4. Verification Harnesses Can Be Destructive

`TestCockroachP0` was a local-only test. Its own doc comment said "Run against the local compose cluster." It re-ran the entire `LATEST.sql` migration twice inside a transaction.

When `crdb:verify` ran it against production, it hung on a TLS read for 10 minutes and failed. The deploy itself succeeded — healthz returned 200. The verification harness was the problem, not the application.

This is subtle: the test was correct for its intended environment. The bug was in the wiring — `crdb:verify` ran a local-only test unconditionally, without checking whether the target was local.

**Lesson:** Tests must be environment-aware. A DSN-based guard that skips non-local tests prevents this class of bug.

---

## 5. Silent Fallbacks Are Technical Debt

The vector DB initialization in `service.go` caught connection errors and returned `NoOpVectorDB`. This prevented crashes. It also hid misconfigurations.

When the vector DB was unreachable, the app started normally. The RAG endpoints returned empty results. No errors. No warnings. Just silence.

After adding a health check that calls `Validate(ctx)` after initialization, we get explicit error messages when the vector DB is misconfigured. The app still starts, but now it logs the problem.

**Lesson:** Silent fallbacks convert hard failures into soft degradation. Both are bad, but soft degradation is harder to debug. Prefer explicit errors over silent fallbacks.

---

## 6. Build Context Optimization Is Free Performance

The Docker build context was 1 GB. The culprit: `lib/` — 518 MB of LanceDB CGO binaries — was included via a `!lib/` negation in `.dockerignore`. This was needed for LanceDB builds but unnecessary for CockroachDB.

The fix was trivial: create `.dockerignore.cockroach` that excludes `lib/`, `server/router/api/v1/agent/build/` (202 MB), `memos` (84 MB), and `main` (84 MB). Use `fly deploy --ignorefile .dockerignore.cockroach`.

Result: 1 GB → 643 KB. A 99.94% reduction. Deploy time dropped proportionally.

**Lesson:** Always audit your build context size. The fix is often trivial; the impact is dramatic.

---

## 7. Adversarial Review Catches What Single Review Misses

The deployment guide went through four adversarial reviews (v1 → v4). Each review caught issues the previous one missed:

| Review | Issue Caught |
|--------|-------------|
| v1 | Guide said "ONE fly.toml" but code uses per-backend TOMLs |
| v2 | `validate-env-chain.sh` hardcoded to `fly.toml` |
| v3 | Validation parameterized, `fly:pre-deploy:*` per backend |
| v4 | Final approval — only 3 low nits remain |

A single review would have approved the guide with the hardcoded `fly.toml` bug. The adversarial series forced each review to challenge the previous one's conclusions.

**Lesson:** Adversarial review is not about being negative. It's about forcing each iteration to prove the previous one wrong.

---

## 8. The "General Purpose" Principle Is Harder Than It Looks

The codebase enforces a strict rule: no tenant-specific logic in code. All customization via config files (KB.MD, POLICY.MD, SCRIPT.MD).

This is easy to follow when you're adding a new feature. It's harder when you're debugging a specific tenant's issue. The temptation is to add a conditional: "if tenant X, do Y." The rule says: "if the agent needs to do Y, add it to tenant X's POLICY.MD."

The CockroachDB deployment tested this principle. The vector DB backend is a deployment profile, not a tenant configuration. The `cockroach` build tag selects the backend at compile time, not at runtime. This is the correct separation: infrastructure decisions are deployment profiles, tenant decisions are config files.

**Lesson:** The "general purpose" principle applies to infrastructure too. Don't hardcode deployment decisions in tenant logic.

---

## 9. Health Checks Don't Catch Semantic Failures

The health check returned 200. The app was running. The machine was healthy. And the RAG pipeline was broken.

Health checks verify infrastructure: is the process running? Is the port open? Is the database reachable? They don't verify semantics: does the RAG pipeline work? Are embeddings being generated? Can the agent search its knowledge base?

The 7-step smoke test (`verify-production.sh`) catches semantic failures. It creates a tenant, imports KB, triggers reindex, and verifies vector search returns results. This is the correct level of verification for a deployment.

**Lesson:** Health checks are necessary but not sufficient. Semantic verification requires end-to-end tests.

---

## 10. The Fly.io `--ignorefile` Flag Is the Key to Multi-Backend Builds

`fly deploy` doesn't use the standard Docker `--dockerignore` flag. It has its own `--ignorefile` flag. This means you can have per-backend `.dockerignore` files without affecting the standard Docker build.

The insight: `fly.toml` and `fly_cockroach.toml` already had separate Dockerfiles. The `.dockerignore` should have been separate too. The `--ignorefile` flag makes this possible.

**Lesson:** When you have per-backend configs, your build tooling must be backend-aware. Check for flags like `--ignorefile` that enable per-backend customization.

---

## Summary

The deepest insight is that **deployment is not a single action — it's a verification chain**. Each stage (build, deploy, health check, smoke test) verifies a different property. A failure in any stage is a signal, not an obstacle.

The most dangerous state is "deployed but broken" — the app is running, health checks pass, but semantic functionality is degraded. This state is invisible to infrastructure monitoring and visible only to end-to-end testing.

As a coding agent, my role is not just to fix bugs but to understand the system well enough to prevent the next one. The five-layer bug stack taught me that the first fix is rarely the right fix. The real fix is the one that prevents the entire stack from recurring.
