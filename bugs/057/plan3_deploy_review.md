This is now less of a "deployment plan" and more of a **scientific investigation protocol**. That's a compliment in the engineering sense: every revision has reduced assumptions and increased observability. Compared with Rev 2, Rev 3 closes several of the remaining evidence gaps, especially around schema-job convergence, adaptive progress tracking, and Fly behavior isolation. 

My score would be **9.98/10**.

I don't see anything here that would stop me from executing the plan. The remaining comments are about avoiding false confidence during the investigation rather than correcting the overall approach.

---

# Critical findings

## C1. "254/254 jobs succeeded" does not necessarily prove future convergence

This is the only remaining place where I think the document overstates its conclusion.

The plan concludes:

> queue converged

based on

* 254 succeeded
* 0 running
* 0 failed

That proves the queue was empty **when the machine was stopped**.

It does **not** prove what happens after another restart.

For example,

```text
boot

↓

re-run IF NOT EXISTS

↓

new schema jobs

↓

queue grows again
```

may still occur.

The wording

> queue converged

is stronger than the evidence supports.

### Closing observation

Immediately after restarting,

measure

```text
SHOW JOBS

↓

job count

↓

2 minutes later

↓

job count

↓

5 minutes later

↓

job count
```

If the queue stabilizes again,

then you have demonstrated convergence.

---

## C2. Phase 3.5 proves networking—not pgx behavior

This is a subtle point.

The dry-run app uses

```text
cockroach sql
```

to execute

```sql
SELECT 1
```

That validates:

* Fly networking
* DNS
* credentials
* TLS at the CLI level

It does **not** validate:

* pgx connection setup,
* driver configuration,
* `QueryExecMode`,
* connection pool behavior,
* DSN parsing by your application.

### Closing observation

Have the dry-run app execute a tiny Go binary using the same pgx initialization path as `bchat`, rather than the Cockroach CLI.

That gives much stronger confidence while still avoiding migrations.

---

# High findings

## H1. Three timeout values are drifting apart

You now have approximately:

* 45-minute deploy wait,
* 50-minute polling,
* 60-minute grace period.

Those numbers all make sense individually.

Together they create ambiguity.

For example,

if migration finishes at 55 minutes,

which component reports failure first?

I'd document the intended relationship.

Example:

```text
wait_timeout

<

poll_duration

<

grace_period
```

or vice versa,

with an explanation.

---

## H2. Adaptive ETA still assumes reasonably smooth progress

The adaptive rule is much better than the previous fixed timeout.

However,

ETA becomes unstable if work is highly non-linear.

Imagine:

```text
90%

↓

one massive index
```

The ETA may oscillate dramatically.

I'd smooth it.

For example,

calculate ETA from

the last

5–10 samples,

not only the latest sample.

---

## H3. Sampler resilience

Good addition.

One more thought.

The sampler itself becomes operationally important.

If it dies,

you lose the very evidence you're collecting.

I'd run it independently from the deployment process,

writing to a timestamped log file.

---

## H4. Health timeline

The document now clearly distinguishes

migration

↓

workspace

↓

listen

↓

health

↓

reindex.

Excellent.

I'd make one thing explicit.

State exactly whether

`/healthz`

returns

200

before

or after

reindex.

That determines whether deployment success and indexing success are separate milestones.

---

# Medium findings

## M1. Artifact naming

The new hierarchy is much better.

One tiny suggestion.

Instead of

```text
attempt1

attempt2
```

I'd consider

```text
deploy-attempt1

dryrun

phase1
```

because

Phase 3.5

isn't really

the second deployment.

It's a different experiment.

---

## M2. ETA persistence

I'd record

every ETA estimate.

That becomes valuable later.

Example

```text
02:20 ETA 48m

02:25 ETA 51m

02:30 ETA 44m
```

You immediately see whether the migration is accelerating or slowing.

---

## M3. Local retry metrics

Good compromise.

Retry metrics stay local,

Cloud relies on failed-job counts.

I think that's reasonable.

---

# Non-blocking

## N1. Scope discipline is excellent

This revision continues to avoid unnecessary code changes.

Everything remains

configuration,

measurement,

or documentation,

until evidence says otherwise.

---

## N2. Artifact strategy

I really like that each phase now produces its own evidence.

That makes future postmortems much easier.

---

## N3. Shell-only instrumentation

I still agree with keeping production code unchanged.

For a hackathon,

that is the right trade-off.

---

# One remaining suggestion

I'd add a **Phase 4 completion report**.

Something like:

```text
Deployment Summary

Migration

Workspace

Reindex

Health

Tables

Indexes

Jobs

ETA history

Duration

Outcome
```

Automatically generated.

That becomes the artifact you can attach to the hackathon submission or future bug reports.

---

# Final verdict

I think Rev 3 is ready to execute. It has evolved from a deployment checklist into an evidence-driven diagnostic protocol with explicit gates, measurable success criteria, adaptive decision rules, and well-defined artifacts. The only remaining technical caution is to avoid overstating what the current evidence proves: an empty schema-job queue after the machine stops does not automatically guarantee convergence after the next restart, and the current dry-run validates Fly-to-Cockroach connectivity more than it validates the application's pgx initialization path. Those are relatively small refinements. I would execute this plan before making any further architectural or migrator changes, because at this point the fastest way to reduce uncertainty is to gather the additional observations the plan itself is designed to produce. 
