I think this is the strongest version so far. The evolution from v1 → v3 is noticeable: instead of proposing solutions first, the plan now establishes evidence, records design decisions, and explicitly distinguishes repository facts, documentation facts, inferences, and recommendations. That's a significant improvement. 

That said, if I were the final principal engineer signing off before implementation, I would still send it back for one last revision. My overall score would be **9.6/10**. The remaining issues are no longer about CockroachDB knowledge—they're about engineering rigor and future maintainability.

## What is excellent

### 1. Evidence classification

This is probably the biggest improvement in the entire document.

Having explicit categories such as:

* Repository Fact
* Documentation Fact
* Inference
* Speculation

dramatically reduces hallucination risk.

I would keep this permanently for future architecture plans.

---

### 2. Every architectural change is defended

Section 13 is exactly what I asked for previously.

Instead of saying

> "Do X"

it answers

* what changed
* why Cockroach requires it
* new failure modes

That is principal-level documentation.

---

### 3. Capability matrix

Excellent addition.

This is much better than a SQL compatibility list because future contributors immediately understand

> why the abstraction exists.

---

### 4. Transaction inventory

Excellent.

Instead of

> wrap transactions

the plan inventories all transaction sites and audits side effects.

That's the correct engineering approach.

---

### 5. VERIFY-FIRST experiments

Very good.

This moves uncertain assumptions into

> prove before coding

rather than pretending they are facts.

---

# Remaining issues

These are the things I'd still challenge.

---

# 1. The plan still assumes one implementation is sufficient

Ironically, after arguing against code duplication, the plan now goes to the opposite extreme.

It effectively concludes:

> One PostgreSQL implementation is sufficient forever.

I don't think that has actually been demonstrated.

It has demonstrated

> today's repository can share implementation.

It has not demonstrated

> future repository evolution won't require capability separation.

I would like one paragraph discussing

> "Why shared implementation remains maintainable as Cockroach-specific features evolve."

---

# 2. The migration atomicity discussion is incomplete

Section 7 correctly explains the loss of atomicity.

However, it never discusses recovery for this scenario:

```
Statement 1
Statement 2
Statement 3

↓

Statement 4 fails

↓

Developer edits migration

↓

Redeploy
```

The plan assumes idempotency solves everything.

Not necessarily.

What if

Statement 3

partially transformed data?

I'd like a section titled

> Recovery after partially applied migration.

---

# 3. The retry audit is excellent—but incomplete

The plan proves

> no external side effects.

Good.

It does **not** prove

> deterministic SQL.

For example

```
SELECT MAX(...)

INSERT MAX+1
```

inside retries.

Could another retry observe different data?

Maybe that's okay.

Maybe it's intended.

But retries change observable behavior.

I would explicitly classify every transaction as

* deterministic under retry
* optimistic by design
* may observe newer state

---

# 4. Capability drift

This is probably the biggest missing architectural discussion.

The capability matrix exists.

But there is no governance.

For example

Future PR:

```
Postgres adds

CREATE EXTENSION xyz
```

Who prevents Cockroach divergence?

I'd add

## Capability Drift Policy

Example

* every new SQL feature must update the capability matrix
* every migration must pass portability audit
* CI rejects undocumented divergence

---

# 5. SQL audit scope

The SQL audit is good.

But it only covers current usage.

I would require one additional rule.

Whenever a new migration is added,

CI automatically checks for

* CREATE EXTENSION
* advisory locks
* LISTEN/NOTIFY
* COPY
* ALTER TYPE
* unsupported extensions

before merge.

That makes compatibility continuous rather than a one-time effort.

---

# 6. Missing rollback strategy

The plan explains migration behavior.

It never explains

> How do we roll back Bug 057?

For example

```
Deploy Cockroach support

↓

Major bug

↓

Rollback
```

Can PostgreSQL continue using the same migrations?

Can Cockroach data simply be discarded because it's a first deployment?

Document it.

---

# 7. Experiments need exit criteria

The experiments are good.

I'd tighten them.

Instead of

```
P4

Does retry work?
```

I'd require

```
1000 concurrent retries

0 duplicate rows

0 lost rows

40001 observed

retry succeeds
```

Measurable.

---

# 8. Performance assumptions

The plan intentionally avoids optimization.

That's good.

But I'd like one benchmark section.

Specifically

```
Migration runtime

Postgres

↓

Cockroach

↓

difference
```

and

```
CreateAgentMessages TPS

Bridge throughput

Vector search latency
```

Not optimization.

Just regression detection.

---

# 9. Connection pool reasoning

The plan says

> 10 is okay.

I think that's still a weak argument.

Instead I'd phrase it as

> Preserve existing defaults initially.
>
> Validate against production cluster sizing after deployment.

Configuration should be treated as operational tuning rather than architecture.

---

# One thing I would remove

I would actually remove most of Section 2.

The list of MCP queries is interesting historically.

It is not implementation guidance.

Instead I'd replace it with

```
Evidence Appendix

Topic

↓

Official documentation

↓

Repository evidence

↓

Decision
```

Much shorter.

Future maintainers care about

> where the rule came from,

not

> what search query found it.

---

# Final request before approval

This is the prompt I would send back to DeepSeek:

> **Perform one final principal-engineer review focused on operational resilience rather than compatibility. Assume all CockroachDB compatibility questions are resolved. Review only the long-term maintainability of the design: migration recovery, rollback strategy, capability drift prevention, deterministic retry behavior, CI enforcement, and operational ownership. Remove historical discussion (such as MCP search logs) that does not directly justify implementation decisions, and replace it with concise evidence tables. Every remaining recommendation must describe not only why it exists today, but also how future contributors will know when it must change.**

## Final verdict

Compared to the previous versions, this plan is a substantial improvement. It has shifted from a "migration proposal" to an evidence-backed engineering design, and most of the earlier speculative compatibility claims have either been eliminated or converted into explicit verification gates. The remaining work is largely about **operational maturity**—ensuring that future contributors can evolve the PostgreSQL and CockroachDB implementations without accidental divergence, and that deployment, rollback, and long-term governance are documented as rigorously as the implementation itself. At that point, I would consider the plan ready for implementation. 
