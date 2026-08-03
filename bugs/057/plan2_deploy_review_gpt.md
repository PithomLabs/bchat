This is a **good deployment plan**, but unlike the previous Bug 057 documents, I don't think this one is quite ready to execute. I'd score it **9.4/10**. The reason is that it mixes **evidence-backed observations** with **new implementation work** that hasn't yet been justified by the evidence it already gathered. 

The biggest improvement is that the plan clearly narrows the remaining work to **deployment** rather than reopening the application code. That's exactly the right scope. However, I think there are still a few places where the plan is preparing to modify code before proving it's necessary.

## Biggest concern: Phase 0 changes production code before proving you need it

The plan proposes adding startup timeline logging to `server.go` as the very first step. 

I understand the motivation, but I don't think that's the highest-value first move.

You already have several external observability sources:

* Fly deployment logs
* Cockroach `SHOW JOBS`
* migration history
* health endpoint timing
* shell-side timing

Before modifying production code, I'd ask:

> Can those existing sources already answer the question?

If the answer is yes, don't touch `server.go`.

Only add instrumentation if those external observations prove insufficient.

I'd therefore reorder the phases:

1. External observations.
2. If ambiguity remains, add timeline instrumentation.
3. Re-run locally.

That keeps the codebase cleaner.

---

## C2. The 30-minute threshold is arbitrary

The plan now says:

> If migration exceeds 30 minutes, redesign migration execution.

I don't think the document justifies **30**.

Earlier revisions argued for adaptive decisions based on observed progress. This version hardcodes a wall-clock threshold. 

I'd instead define something measurable, for example:

* migration throughput stalls for N minutes,
* `SHOW JOBS` no longer advances,
* ETA exceeds deployment budget.

Those criteria are grounded in observed behavior rather than elapsed time alone.

---

## H1. Phase 2 is doing two experiments simultaneously

Phase 2 currently tries to answer:

* Is one-shot execution the bottleneck?
* How does a multi-region cluster behave?

Those are different experiments.

If one-shot execution is slower, you won't know whether that was caused by:

* multi-region latency,
* schema jobs,
* execution strategy.

I'd split them:

* **Experiment A:** single-node, compare one-shot vs per-statement.
* **Experiment B:** three-node, run only the chosen execution strategy.

That isolates variables much better.

---

## H2. Phase 3 jumps to migrator chunking too quickly

The document correctly says:

> only if Phase 2 proves a bottleneck.

However, "bottleneck" is not precisely defined.

For example:

* Is 32 minutes a bottleneck?
* Is 18 minutes acceptable?
* What if the migration progresses steadily but slowly?

I'd define explicit decision criteria before introducing migrator changes.

---

## H3. Startup timeline

If you keep Phase 0, I'd log one more event:

```text
process_start
DB connected
migration start
migration end
workspace init
HTTP listen
healthz ready
first successful request
```

The last event often reveals problems that startup logs alone don't.

---

## Medium findings

### M1. Version alignment

I fully agree with moving the local compose file to the same CockroachDB major version as Cloud. That's one of the strongest changes in the plan. 

---

### M2. Artifact collection

Excellent.

I'd slightly expand it to include:

* Fly deployment ID
* Cockroach version
* Git commit

Those three pieces make reproducing issues much easier.

---

### M3. Open questions

Question 3 currently asks whether to skip Phase 2.

Given the document's own reasoning, I'd actually remove that as an open question.

The plan consistently argues Phase 2 provides useful evidence, so I'd simply recommend doing it.

---

## One thing I'd add

I'd add a **deployment success checklist**.

Something like:

* Machine remains healthy for N minutes after startup.
* Migration history shows expected version.
* No active schema jobs remain.
* `verify:production` passes.
* Restart succeeds without rerunning migration.
* Second deployment completes without timeout.

That gives you objective completion criteria.

---

## One thing I'd remove

I'd remove:

> "No coding until you approve."

The document already contains code modifications (for example, the proposed logging in `server.go`), so that statement isn't really accurate. I'd instead say:

> "No functional changes until deployment evidence is collected."

That better reflects the intent.

## Final verdict

This is a well-focused deployment plan that correctly narrows the remaining work to operational deployment rather than reopening the application logic. The strongest parts are the verification of the current deployment configuration, the alignment of local and cloud CockroachDB versions, and the explicit separation between deployment concerns and the already-completed application fixes. 

The main refinements I'd make are methodological rather than architectural: avoid modifying `server.go` until existing observability proves insufficient, replace the fixed 30-minute cutoff with evidence-based decision criteria, separate the execution-strategy experiment from the multi-region experiment, and define explicit conditions for when migrator chunking becomes justified. With those adjustments, I would be comfortable approving the deployment plan.
