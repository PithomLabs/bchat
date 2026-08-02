This plan is a good example of **changing strategy instead of changing code first**. That's the right instinct. Rather than immediately modifying `migrator.go`, it tries to prove the failure locally, isolate the bottleneck, and only then decide whether code changes are warranted. I think that's a stronger engineering approach than jumping straight into implementation. 

That said, I still found several issues I would address before redeploying.

# Critical findings

### C1. The root cause is still an inference, not a proven fact

The plan concludes:

> migration > grace period → Fly kills machine → restart → repeat

The evidence certainly supports this hypothesis:

* partial schema (42/57 tables),
* empty `migration_history`,
* health endpoint never became available,
* repeated restarts.

However, it does **not** conclusively prove that the migration was merely *slow* rather than *blocked*.

For example, the current evidence does not distinguish between:

* slow online schema changes,
* waiting on a background schema job,
* connection stalls,
* lock waits,
* a long-running statement,
* repeated retries inside Cockroach.

### Required before redeploy

During Phase 2, collect runtime evidence rather than only elapsed time.

For example:

* sample `SHOW JOBS` while migration runs,
* identify the currently executing statement,
* record which statement number is executing,
* observe whether progress is advancing.

That distinguishes

> progressing slowly

from

> effectively hung.

---

### C2. Increasing Fly timeouts may mask the wrong problem

The plan proposes

* `grace_period = 30m`
* `--wait-timeout 25m`

Those are reasonable mitigations.

But they are **not** proof that the deployment architecture is correct.

If the migration eventually takes

40 minutes,

the deployment still fails.

The document needs an upper-bound strategy.

For example:

> If migration exceeds X minutes, stop modifying Fly configuration and instead redesign migration execution.

I'd define that threshold explicitly.

---

### C3. Phase 2 should prove that one-shot execution is actually the bottleneck

The current comparison is

> one-shot ExecContext

versus

> per-statement execution.

Good.

I'd make the experiment stricter.

Measure:

* total wall clock,
* per-statement latency,
* server CPU,
* active schema jobs,
* retries,
* connection count.

Otherwise,

if chunking happens to be faster,

you still won't know *why*.

---

# High findings

### H1. Local three-node cluster is not equivalent to Cockroach Cloud

The document correctly notes this limitation, but I think it understates it. 

A three-node Docker cluster validates:

* distributed DDL correctness,
* lease movement,
* replication,
* version behavior.

It does **not** reproduce:

* Cockroach Cloud serverless scheduling,
* noisy neighbors,
* storage throttling,
* network latency,
* cloud resource limits.

Treat Phase 2 as

> functional validation,

not

> performance reproduction.

---

### H2. Reindex timing deserves its own measurement

The plan correctly notes that `/healthz` is unavailable until migration, workspace initialization, and reindex complete. 

However,

those three phases are currently treated as one.

I'd separately measure

```
migration

↓

workspace init

↓

vector reindex

↓

server listen
```

Otherwise you won't know which phase dominates startup.

---

### H3. Health endpoint placement

The document correctly states that `/healthz` is registered after migration.

I'd question that architectural decision.

For a hackathon,

I wouldn't necessarily change it,

but I'd at least ask:

Should readiness depend on

* successful startup

or

* complete database migration?

Those are different concepts.

I wouldn't change it during Bug 057,

but I'd document the trade-off.

---

### H4. Verify Fly behaviour experimentally

The plan correctly asks what happens when

`--wait-timeout`

expires.

Don't rely on assumptions.

Run a trivial deployment with an intentionally sleeping process and observe:

* whether Fly leaves the machine running,
* whether the deploy is marked failed,
* whether health checks continue,
* whether autostop still applies.

That experiment is cheap and removes uncertainty.

---

# Medium findings

### M1. Phase 1 should use the same Cockroach version

The plan acknowledges

* local v25.2.21
* cloud v26.2.1

I'd go further.

If Docker Hub already has v26.2.1,

Phase 1 should also use it.

That removes one variable immediately.

---

### M2. Timing instrumentation

I'd add timestamps around every startup phase.

For example

```
migration start

migration end

workspace init

reindex start

reindex end

server listen
```

Those timestamps are more valuable than aggregate boot time.

---

### M3. Deployment logs

I'd archive the first failed deployment.

Specifically

* Fly logs,
* Cockroach logs,
* migration output,

into

```
bugs/057/artifacts/
```

Future debugging becomes much easier.

---

### M4. `SHOW JOBS`

Since Cockroach performs schema changes as jobs,

I'd explicitly add

```
SHOW JOBS
```

to Phase 2 observations.

That gives visibility into background work rather than only SQL execution timing.

---

# Non-blocking

### N1. Great deployment philosophy

I particularly like this sentence:

> local-first diagnosis before Cloud redeploy

That dramatically reduces iteration cost.

Keep it.

---

### N2. Gates are well structured

The

Phase 1

↓

Phase 2

↓

Phase 3

progression is much cleaner than repeatedly deploying to Fly hoping for different results.

---

### N3. Guardrails

I like that the plan explicitly forbids touching the Neon deployment.

That preserves a working production path.

---

# One thing I would add

I'd introduce a **Phase 0** before everything else.

## Phase 0 — Startup timeline instrumentation

Before changing

* Fly,
* migrations,
* Docker,

instrument startup.

Log something like

```
T+0s   process start
T+1s   DB connected
T+2s   migration start
T+...
migration complete
workspace init
vector init
HTTP listen
```

That single addition would immediately answer whether the startup delay is dominated by migration, initialization, or vector indexing.

---

# Final verdict

I think this is the right recovery plan. Unlike earlier iterations, it does **not** immediately assume the migrator is broken or that CockroachDB requires architectural changes. Instead, it prioritizes reproducing the failure locally, collecting evidence, and only then deciding whether deployment configuration or migration execution needs to change. My remaining concerns are all about improving the quality of that evidence: prove whether the migration is genuinely progressing or blocked, separate migration time from later startup work, validate Fly's timeout behavior experimentally, and standardize on the same CockroachDB version locally and in the cloud. Once those observations are added, I would be comfortable using this as the execution plan for the next redeployment. 
