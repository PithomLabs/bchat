## claude

```
CONTEXT
bchat is migrating from its current DB to CockroachDB. Local dev runs
cockroachdb/cockroach:v26.2.1 via scripts/docker-compose.cockroach.yml.
Target production is CockroachDB Basic (serverless), confirmed also on v26.2.1.
Tracked under bugs/057. Relevant existing files: summary_pending_20260802_204056.md,
summary_testdeploy_20260802_193030.md, test_locally.md, scripts/docker-compose.cockroach.yml,
Taskfile.yml, scripts/verify-production.sh, vectordb_cockroach.go.

Do NOT write the full local-tuning tooling yet. Resolve the open unknowns below first —
several of them determine whether the tooling approach is even valid.

TASK 1 — Tenant cluster-setting support probe (BLOCKING, do first)
Connect to a real CockroachDB Basic (serverless) instance (free tier is fine) and run
each of the following individually, capturing success/failure/no-op for each:
  SET CLUSTER SETTING feature.vector_index.enabled = true;
  SET CLUSTER SETTING serial_normalization = 'sql_sequence';
  SET CLUSTER SETTING kv.range_merge.queue_interval = '50ms';
  SET CLUSTER SETTING jobs.registry.interval.gc = '30s';
  SET CLUSTER SETTING jobs.retention_time = '15s';
  SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false;
Deliverable: a markdown table (setting | accepted? | error text if rejected) added to
bugs/057. This determines which settings can go in the local init script AND survive
into prod vs. which are local-dev-only.

TASK 2 — Vector index backfill blocking window, measured not assumed
cockroachdb/cockroach#144443 is still open as of v26.2.1: creating a vector index on a
non-empty table requires disabling sql_safe_updates and blocks all
INSERT/UPSERT/UPDATE/DELETE on that table until backfill completes.
  a. Seed agent_vectors locally with a row count matching realistic production volume
     (not the current demo-sized seed) — pull the real target count from whoever owns
     the data, or use current prod row count if known.
  b. Time CREATE VECTOR INDEX against that seeded table. Record wall-clock duration.
  c. Decide and document in bugs/057: create the index BEFORE loading data (avoids the
     block) vs. AFTER (incurs it) — pick one and state why, based on (b).

TASK 3 — Replace serial_normalization reliance with explicit DDL
Do not depend on the serial_normalization setting applying at migration time (Task 1
may show it's rejected on tenants anyway). For every column currently relying on
implicit SERIAL/unique_rowid() behavior:
  a. Add explicit CREATE SEQUENCE + DEFAULT nextval('seq_name') to the migration DDL.
  b. Write a migration (not just a setting change) that ALTERs any already-created
     tables with unique_rowid() defaults to use the new sequence-backed default.
  c. Add a test that asserts column defaults via information_schema /
     pg_get_serial_sequence, not via re-checking the cluster setting value.

TASK 4 — Migration replay against simulated broken state
Current tests only validate a clean-slate migration (BCHAT_ALLOW_DB_RESET=1).
Add a test fixture that:
  a. Stands up a local cluster pre-seeded to look like current prod: tables present,
     unique_rowid() defaults, non-empty rows.
  b. Runs the Task 3 fix-forward migration against it (not a reset).
  c. Asserts final schema state matches a clean-slate migration's output exactly.

TASK 5 — Split docker-compose store modes
scripts/docker-compose.cockroach.yml currently implies one mode. Add two explicit
Task targets:
  a. crdb:up — disk-backed store (current default), used for the full E2E/vector/
     migration verification suite.
  b. crdb:up:fast — adds --store=type=mem,size=0.25, restricted to unit tests only.
     Fail CI if crdb:up:fast is used to run scripts/verify-production.sh.

TASK 6 — Connection/auth parity check against Basic tier
Using the same sandbox Basic cluster from Task 1:
  a. Confirm scripts/verify-production.sh's connection handling works against
     TLS-required + SCRAM auth (not just local --insecure root).
  b. Note Basic tier's connection/RU limits reached under a normal verify-production.sh
     run, if any throttling is observed.
  c. File any code path that assumes local's insecure/root connection semantics as a
     bug in bugs/057.

OUTPUT FORMAT
For each task, append findings to a single new file: bugs/057/summary_adversarial_followup_<date>.md
with sections named exactly Task 1 .. Task 6. Do not modify tooling/scripts until Tasks
1 and 2 are answered — they gate whether Phases 1–3 of the existing plan are valid as
written.
```



## chatgpt

I'd give the coding agent a **very constrained investigation prompt**. The goal is **eliminate assumptions**, not produce another architectural document.

---

## Prompt

You are the lead Go + CockroachDB engineer for Bug 057.

**Do NOT propose code changes yet.**
**Do NOT rewrite the implementation plan.**
Your job is to eliminate the remaining unknowns with evidence.

We have already established:

* CockroachDB Basic (Serverless) is **v26.2.1**.
* Local Docker will also use **v26.2.1**.
* Application-level Cockroach fixes (VECTOR, NULL scans, etc.) are already complete.
* The remaining work is deployment readiness.

Investigate the following questions **one by one**. Every answer must include:

1. **Repository evidence** (where applicable)
2. **Official CockroachDB documentation** (if applicable)
3. **Conclusion**
4. **Action required (if any)**

If no action is required, explicitly state:

> **No change required.**

---

# Q1. Cluster initialization ownership

Determine which of these belong in:

* Docker Compose
* `task crdb:init`
* Application startup
* Never

Evaluate each individually:

* `serial_normalization`
* `feature.vector_index.enabled`
* test-only GC tuning
* SQL stats tuning
* range merge tuning

For each item explain:

* persistent?
* cluster-wide?
* idempotent?
* supported on Basic?
* required for bchat?

Do **not** assume.

Verify.

---

# Q2. Single-node as source of truth

Can a local **single-node v26.2.1** cluster validate all of these?

* migrations
* sequences
* VECTOR
* RAG
* verify-production
* retry wrapper
* pgx compatibility

For every item answer

YES

or

NO

and explain exactly what **cannot** be validated on single-node.

---

# Q3. Three-node justification

Assume single-node passes 100%.

What additional failures could **only** be discovered using a 3-node cluster?

Do not answer generally.

List only issues relevant to bchat.

If there are only 2 or 3, say so.

---

# Q4. Basic Serverless differences

Ignore version.

Investigate operational differences only.

Specifically:

* unsupported cluster settings
* permission restrictions
* VECTOR limitations
* background job behavior
* migration behavior
* deployment behavior

For every difference answer

Does this affect bchat?

YES / NO

with evidence.

---

# Q5. Golden state

Define the exact pass/fail checklist before deploying to Fly.

Example format:

```
57 tables
PASS

migration_history
PASS

VECTOR index
PASS

verify-production
PASS

restart
PASS

...
```

No prose.

Checklist only.

---

# Q6. Task responsibilities

Audit every Cockroach Taskfile target.

Recommend whether each should exist:

* `crdb:up`
* `crdb:init`
* `crdb:reset`
* `crdb:seed`
* `crdb:verify`
* `crdb:smoke`

For each task explain:

* responsibility
* dependencies
* idempotent?
* should it exist?

If a task should not exist,

say so.

---

# Q7. Deployment blockers

After answering Q1-Q6,

produce exactly three sections.

## Blockers

Things that must be solved before deploying.

## Risks

Things acceptable for the hackathon.

## Deferred

Things intentionally postponed until after the hackathon.

---

# Output constraints

* No implementation plan.
* No code.
* No speculative architecture.
* No "future enhancements".
* No enterprise recommendations.
* Every statement must be backed by either repository evidence or official CockroachDB documentation.
* If evidence is missing, say **"Not verified"** instead of guessing.

The deliverable should be a **deployment readiness assessment**, not a redesign.


