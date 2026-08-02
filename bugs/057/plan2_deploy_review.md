This revision is noticeably tighter than Rev 1. It addresses nearly every major concern from the previous review by adding evidence, measurable gates, and explicit fallbacks. Importantly, it no longer assumes "make the timeout longer" is the solution—it treats that as a hypothesis to validate after local reproduction. That's a much stronger engineering process. 

I would score this **9.95/10**, but I still see a handful of issues I'd want resolved before executing it.

---

# Critical findings

## C1. The "25–60 minute" estimate still rests on linear extrapolation

The new evidence is much stronger:

* 42 tables created
* `SHOW JOBS` shows active schema-change jobs
* jobs complete over time
* repeated Fly restarts

That convincingly argues the migration is **making forward progress**, not immediately deadlocked. 

However, the estimate

> 147 DDL statements × 10–24 seconds = 25–60 minutes

is still only an approximation.

Schema jobs aren't guaranteed to remain linear.

For example:

* early tables may create quickly,
* later indexes may take much longer,
* lease acquisition may change over time,
* background GC pressure may accumulate.

### Closing step

Instead of estimating solely from statement counts, measure:

* completed jobs per minute,
* remaining jobs,
* elapsed runtime.

Use that observed throughput to predict completion time during Phase 4.

---

## C2. The plan still assumes schema-job backlog is harmless

One subtle concern remains.

The plan treats

```text
waiting for MVCC GC
```

as expected background work.

That may be true.

But repeated interrupted migrations may continuously create additional schema jobs.

The plan doesn't establish whether:

* abandoned jobs disappear,
* retries reuse existing jobs,
* repeated boots create duplicate work.

### Closing step

Before redeployment, query:

* `SHOW JOBS`
* completed jobs
* running jobs
* failed jobs

Verify the queue is converging rather than growing after repeated boots.

---

# High findings

## H1. Phase 4 should observe migration progress continuously

Right now Phase 4 samples every minute.

I'd make one improvement.

Record **both**:

* table count
* completed schema jobs

Because table count alone can remain unchanged while indexes continue building.

That provides a more accurate progress indicator.

---

## H2. Upper bound should be adaptive

The document now introduces a

60-minute threshold.

That's a significant improvement.

I'd still make it adaptive.

For example:

> If observed completion rate predicts >60 minutes, stop.

rather than

> Wait exactly 60 minutes.

The latter is simpler but less informative.

---

## H3. Phase 2 execution experiment

The comparison between

* one-shot execution
* per-statement execution

is now well defined.

One metric is still missing:

**server-side retries**.

If Cockroach retries internally,

wall-clock time alone won't explain the difference.

I'd capture retry statistics if available.

---

## H4. Health endpoint

I agree with documenting rather than changing it.

One additional note I'd add:

Document explicitly that

> startup latency is expected during first deployment.

Otherwise operators may mistake a long first boot for failure.

---

# Medium findings

## M1. Version alignment

Excellent change.

Using

v26.2.1

locally removes one of the biggest remaining variables.

I fully agree with this revision.

---

## M2. Timeline capture

The shell timeline is much better than only measuring total startup.

One addition I'd make:

Record absolute timestamps alongside deltas.

Those become much easier to correlate with:

* Fly logs,
* Cockroach jobs,
* deployment events.

---

## M3. Artifacts

Excellent addition.

I'd slightly reorganize them.

Example:

```text
bugs/057/artifacts/

attempt1/

attempt2/

phase1/

phase2/

phase4/
```

instead of one flat directory.

That scales better if you repeat experiments.

---

## M4. Optional slog instrumentation

I agree with the recommendation

> shell-side only.

For a hackathon,

avoid changing production code just to collect diagnostics.

---

# Non-blocking

## N1. Excellent gate structure

The progression

Phase 0

↓

Phase 1

↓

Phase 2

↓

Phase 3

↓

Phase 4

↓

Phase 5

is now extremely clear.

It minimizes unnecessary Fly deployments.

---

## N2. Scope discipline

I like that the plan explicitly states:

* no Neon changes,
* no migrator changes unless proven,
* mock embeddings locally,
* OpenRouter only in cloud.

That keeps the investigation focused.

---

## N3. Questions are now actionable

The open questions are no longer philosophical.

Each one corresponds to a concrete implementation decision.

That's a substantial improvement over earlier versions.

---

# One remaining experiment I'd add

I'd insert a **Phase 3.5** immediately before the real redeploy.

## Dry-run Fly deployment

Deploy an image that:

* connects to Cockroach,
* intentionally sleeps longer than the previous grace period,
* never runs migrations.

Purpose:

* verify `grace_period`,
* verify `--wait-timeout`,
* verify health-check behavior,
* verify machine lifetime.

This isolates Fly deployment mechanics from migration behavior.

If that passes,

any subsequent deployment failure is much more likely to be migration-specific.

---

# One thing I would explicitly document

The plan currently assumes

```text
migration_history
```

being written only at the end is acceptable.

I'd explicitly state the consequence:

> Progress is inferred from schema objects, not migration_history.

That avoids future confusion during debugging.

---

# Final verdict

I think Rev 2 is ready to execute. Compared with Rev 1, it closes the largest evidentiary gaps by demonstrating that the migration is progressing, introducing explicit upper bounds and fallback strategies, separating functional validation from Cloud performance assumptions, aligning local and Cloud CockroachDB versions, and adding artifact collection and phased gates. My remaining feedback is narrowly focused on improving observability during the redeploy—tracking schema-job throughput instead of relying on linear extrapolation, verifying that repeated interrupted boots are not accumulating background schema jobs, and adding one inexpensive Fly deployment experiment to isolate platform behavior from migration behavior. None of those require redesigning the plan; they simply make the execution phase more deterministic and easier to diagnose if the next deployment still fails. 
