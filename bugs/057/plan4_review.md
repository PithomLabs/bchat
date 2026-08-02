This is a **clear improvement over v3**. I would now score it **9.8/10**.

The difference between v3 and v4 isn't that it adds more CockroachDB facts—it improves the **engineering governance** around the implementation. The additions on capability drift, rollback, migration lifecycle, deterministic retries, and measurable experiments address most of the remaining architectural concerns from the previous review. 

At this point, I no longer see fundamental architectural flaws. The remaining issues are mostly about tightening a few assumptions before implementation.

---

# Major improvements

## 1. The architecture is now self-governing

This was the biggest weakness of v3.

Now there is an explicit

* Capability Drift Policy
* forbidden constructs
* review-required constructs
* CI enforcement

instead of relying on tribal knowledge.

That is exactly what I wanted to see.

---

## 2. Migration recovery is much better

Section 7.5 is excellent.

Instead of simply saying

> rerun migration

it now defines

* detection
* assessment
* recovery path A
* recovery path B
* lifecycle rules

That is production engineering.

---

## 3. Deterministic retry classification

Excellent addition.

Instead of

> retries are safe

it classifies every transaction individually.

That's far stronger.

---

## 4. Benchmarks are now treated correctly

The benchmark section is no longer trying to optimize.

It exists purely for regression detection.

That's exactly how architecture documents should treat performance.

---

## 5. Evidence appendix

Much better than listing MCP searches.

Future readers care about

> evidence

not

> prompts.

---

# Remaining concerns

These are no longer blockers.

They're mostly refinement.

---

# 1. The migration mirror strategy deserves more justification

The plan says

```text
mirror postgres/

↓

mirror cockroach/
```

I understand why.

But I would still like one explicit paragraph answering:

> Why mirror the historical migration tree if Cockroach never executes those files?

The answer is implied:

* version machinery
* parity validation
* future auditability

I'd simply write it explicitly.

Otherwise future maintainers may delete those files.

---

# 2. Fresh deployment assumption

The plan repeatedly says

> Cockroach is greenfield.

That's true today.

But it appears in multiple architectural decisions.

I would instead centralize it.

For example

## Assumption A1

```text
Cockroach deployment starts with an empty database.
```

Then reference

A1

instead of repeating it.

That makes it easier to remove later.

---

# 3. The capability seams section

I like it.

However,

```text
exactly four seams
```

is stronger wording than necessary.

Architecture evolves.

I'd say

> currently four identified seams

instead.

That avoids future contributors feeling constrained by the document.

---

# 4. Grep-based compatibility checking

I actually agree with a grep scanner.

But I'd explicitly state

```text
best effort

not parser
```

Otherwise someone will assume it is exhaustive.

One sentence is enough.

---

# 5. Benchmark thresholds

The benchmark section records numbers.

I'd also define

what constitutes a regression.

Example

```text
>20%

↓

manual review
```

instead of merely recording results.

---

# 6. Retry experiments

P4 is excellent.

I'd make one addition.

Force

* transaction abort
* network disconnect

Those exercise different retry paths.

---

# 7. Rollback

The rollback section is now good.

I'd add one sentence.

Something like

> Rollback does not attempt schema downgrade.

That is implied.

I'd state it.

---

# 8. One wording issue

This sentence

> int32 overflow with unique_rowid()

could be misunderstood.

The real issue is

```text
repository scans into int32
```

not

```text
unique_rowid is bad
```

I'd rephrase to avoid future readers concluding

unique_rowid()

is inherently wrong.

---

# One thing I would still challenge

This is the only place where I think the plan still moves slightly too quickly.

Section 5.2 concludes

> sql_sequence is required

I would instead phrase it as

> **Given the current repository model types (`int32` IDs), `sql_sequence` is the chosen compatibility strategy.**

That wording is more precise.

The underlying requirement is

```text
int32 compatibility
```

not

```text
sql_sequence
```

If the repository later migrates to int64,

the strategy changes.

---

# Final principal-engineer suggestion

I would add one final section.

## Decision Log

Not implementation.

Just decisions.

Example

| Decision              | Why                 | Revisit When                           |
| --------------------- | ------------------- | -------------------------------------- |
| simple_protocol       | parity              | runtime benchmark exceeds threshold    |
| sql_sequence          | int32 IDs           | IDs become int64                       |
| whole-file migrations | Cockroach DDL       | Cockroach transaction semantics change |
| shared implementation | current portability | SQL divergence exceeds defined seams   |

That table becomes invaluable two years from now when someone asks:

> "Why did we do this?"

---

# Final verdict

This is now an implementation plan that I would be comfortable approving **provided the proof-of-concept experiments (P1–P6) all pass**. The document has evolved from a compatibility checklist into a mature engineering design with explicit evidence classification, governance rules, recovery procedures, CI enforcement, and operational considerations. The remaining suggestions are primarily about future-proofing the document—clarifying assumptions, softening a few absolute statements, and making decision review points explicit—rather than correcting architectural flaws. 



## Operations

I actually think this is the **largest remaining architectural gap** in the entire plan.

Not because the implementation is wrong—but because **deployment is almost absent**.

Since you mentioned that **Neon is already live in production**, your success criterion is not merely:

> "Cockroach support works."

It is:

> **"Deploying to CockroachDB should feel almost identical to deploying to Neon."**

That's a very different design goal.

The current plan mentions Taskfile targets like:

* `build:cockroach`
* `run:cockroach`
* `crdb:test`
* `crdb:bench`

and says they will be extended, but it does not define the **developer workflow** or **deployment ergonomics**. 

---

# I'd ask DeepSeek to add an entire Deployment Workflow section

Something like:

```text
22. Deployment Workflow
```

---

# Current Neon workflow

Document it.

Example

```
task build
↓

fly deploy

↓

Neon migrations

↓

production live
```

Then compare.

---

# Desired Cockroach workflow

The goal should literally be

```
task deploy:cockroach
```

That's it.

No manual SQL.

No shell scripts.

No

```
cockroach sql
```

commands.

No copy-pasting.

---

# Taskfile should expose a deployment API

Instead of

```
build:cockroach

run:cockroach

crdb:test
```

I'd design it around lifecycle.

Example

```
crdb:up
crdb:down
crdb:reset

crdb:migrate
crdb:verify

crdb:test

deploy:cockroach
```

Notice these are **verbs**, not implementation details.

---

# Local developer workflow

Should become

```
task crdb:up

↓

task migrate

↓

task run:cockroach
```

Not

```
docker compose ...

cockroach sql ...

...
```

Hide all of that.

---

# Fresh production deployment

I would expect something like

```
task deploy:cockroach
```

internally performing

```
build

↓

validate parity

↓

validate compatibility

↓

run experiments (optional)

↓

fly deploy

↓

wait healthy

↓

verify migration_history

↓

smoke test

↓

done
```

The user should never think about

```
SET serial_normalization
```

or

```
migration ordering
```

The deployment tooling should own that complexity.

---

# Add verification commands

Right now

the plan mostly discusses

```
deploy
```

I'd add

```
task crdb:verify
```

Checks

```
migration_history

schema version

SHOW CREATE TABLE

vector index

connection

retry

health endpoint
```

This becomes invaluable in production.

---

# Add rollback commands

Currently rollback is prose.

I'd expose it.

Example

```
task rollback:postgres
```

or

```
task switch:postgres
```

Even if internally it simply changes

```
--driver
```

and redeploys.

Operational workflows should be encoded.

---

# Environment management

One thing I think the plan is missing completely.

Right now it introduces

```
COCKROACH_DSN
```

I'd go further.

Define

```
.env.local

.env.fly

.env.cockroach
```

or whatever convention bchat already uses.

The deployment document should answer

> Where does COCKROACH_DSN actually come from?

Today that's not very explicit.

---

# Fly.io integration

This is where I'd push the plan hardest.

I would expect a table.

| Task     | Neon       | Cockroach  |
| -------- | ---------- | ---------- |
| Build    | identical  | identical  |
| Deploy   | fly deploy | fly deploy |
| Migrate  | automatic  | automatic  |
| Verify   | health     | health     |
| Rollback | redeploy   | redeploy   |

The goal is

> identical operational experience.

---

# Health verification

After deployment

I'd automatically check

```
GET /healthz
```

plus

```
SELECT version()
```

plus

```
migration_history
```

before reporting success.

---

# Cloud bootstrap

The plan discusses Docker.

It barely discusses

Cockroach Cloud.

I'd want one command.

Example

```
task crdb:init
```

Internally

```
validate env

↓

test TLS

↓

ping cluster

↓

verify permissions

↓

done
```

No manual SQL.

---

# Production smoke tests

Immediately after deployment

I'd automatically execute

```
create tenant

↓

create memo

↓

vector insert

↓

vector search

↓

bridge transaction

↓

delete test data
```

That proves

* migrations
* retries
* vectors
* transactions

all survived deployment.

---

# My biggest recommendation

I wouldn't think of Taskfile as a collection of tasks.

I'd think of it as the **public API for operators**.

If an operator has to remember

```
task crdb:docker:run

task migrate

task crdb:check

fly deploy

task crdb:test
```

then the API is leaking implementation details.

Instead, I would design around **intent**.

For example:

| Intent                  | Task                     |
| ----------------------- | ------------------------ |
| Start local Cockroach   | `task crdb:up`           |
| Reset local database    | `task crdb:reset`        |
| Run application         | `task run:cockroach`     |
| Validate compatibility  | `task crdb:verify`       |
| Deploy to Fly           | `task deploy:cockroach`  |
| Smoke-test production   | `task verify:production` |
| Roll back to PostgreSQL | `task rollback:postgres` |

---

## This is the prompt I'd give DeepSeek

> **Treat the Taskfile as the operator-facing API, not a collection of helper commands. Design CockroachDB deployment so that a developer already deploying to Fly.io with Neon experiences the same operational workflow. Every manual deployment step should either become a Taskfile target or be explicitly justified. Include local development, production deployment, verification, smoke testing, rollback, and environment management. The goal is that switching from Neon to CockroachDB changes infrastructure, not developer ergonomics.** 
