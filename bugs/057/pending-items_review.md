This is the **strongest implementation note** I've seen in the entire Bug 057 series because it does something the earlier plans didn't:

> It first verifies what has **already been fixed** before proposing new work.

That is excellent engineering practice. Too many implementation plans accumulate stale TODOs instead of reconciling them against the current branch. 

That said, I **do not agree that this plan is implementation-ready**. I'd score it **9.3/10**, which is actually *lower* than the previous revision—not because the document is worse, but because **its remaining claim (P1) is now much more consequential**, and I think it is presented with too much certainty.

---

# Critical findings

## C1. The new "root cause" is still an inference

This sentence is the biggest issue:

> CockroachDB's `<=>` operator requires the vector to appear as a literal in the query text.

The document does **not** demonstrate this.

It infers it from the observed error.

Those are not equivalent.

There are at least four plausible explanations:

1. TEXT → VECTOR cast isn't supported in this context.
2. pgx is binding the parameter differently than expected.
3. `simple_protocol` changes parameter typing.
4. Cockroach's VECTOR parser rejects this particular parameter path.

Your plan immediately concludes:

> therefore interpolate the literal.

That may turn out to be correct—but it has **not been proven**. 

---

## C2. The plan removed the spike

Ironically,

the previous revision improved because it introduced

> P0

to prove the binding behavior.

This revision deletes that completely.

Now we're back to:

```text
Observation

↓

Hypothesis

↓

Production code
```

instead of

```text
Observation

↓

Spike

↓

Evidence

↓

Production code
```

I think that's a regression.

I would **restore the spike**.

---

# High findings

## H1. SQL interpolation should be the fallback, not the primary solution

The plan proposes:

```go
fmt.Sprintf(... %s::VECTOR ...)
```

I agree it's likely safe because `formatVectorLiteral()` generates numeric content.

However,

from an architectural perspective,

this should be:

> last verified option

not

> preferred option.

If Cockroach can support native parameter binding,

that is almost always preferable.

I would only interpolate after proving that binding cannot work.

---

## H2. "No SQL injection risk"

The wording is too strong.

The real claim should be:

> The current implementation of `formatVectorLiteral()` emits only numeric tokens and structural characters, making SQL injection unlikely provided that function remains the sole source of the interpolated value.

That subtle difference matters.

Future maintainers may later change

```go
formatVectorLiteral()
```

without remembering this assumption.

---

## H3. Parameter renumbering

Good catch.

One thing I'd explicitly add:

Unit test

ensuring

```text
tenant_id

↓

top_k

↓

min_score
```

still bind correctly after removing the vector parameter.

This is exactly the sort of regression that's easy to miss.

---

# Medium findings

## M1. Risk table

The risk table is good.

I'd add one row.

| Risk                                               | Mitigation                   |
| -------------------------------------------------- | ---------------------------- |
| Query string becomes very large for 1536-d vectors | Benchmark parse time locally |

You're now constructing a large SQL statement for every search.

That may be perfectly acceptable,

but it's worth measuring.

---

## M2. Rollout

I'd insert

one local benchmark

before Fly deployment.

Not because performance is critical,

but because

you're changing

query construction.

---

## M3. Open decisions

I actually disagree with:

> None — implementation-ready.

I'd change it to:

> Decision pending outcome of vector-binding spike.

---

# One thing I think is missing

If you ultimately choose SQL interpolation,

I'd add

a permanent code comment.

Something like:

```go
// NOTE:
//
// CockroachDB VECTOR parameters could not be bound
// correctly through pgx (see Bug 057).
//
// formatVectorLiteral() intentionally emits only
// numeric vector literals for safe interpolation.
//
// If Cockroach or pgx gains native VECTOR parameter
// support, revisit this implementation.
```

Without that,

future contributors may "simplify" the code back to

```go
$1::VECTOR
```

and reintroduce the bug.

---

# One thing I'd remove

This sentence:

> None — plan is implementation-ready.

I don't think the document has earned that conclusion yet.

The only unresolved item is also

the most technically uncertain one.

---

# My biggest recommendation

I would merge

the previous plan

and

this plan.

Specifically:

* Keep the excellent **status audit** from this revision.
* Keep the excellent **file verification** from this revision.
* **Restore P0** from the previous revision.
* Only after P0 succeeds should Section 3 become the implementation.

That gives you:

```text
Verify repository state

↓

Spike

↓

Choose implementation

↓

Patch

↓

Validate
```

which is the strongest workflow of all the revisions.

# Final verdict

I agree with the document's assessment that **P1 is the only remaining functional blocker** and that the other pending items have either been implemented or intentionally deferred. The audit of the current HEAD is particularly valuable because it prevents redundant work and confirms that several earlier fixes are already present. 

Where I disagree is the final step: the plan jumps from a plausible diagnosis of the CockroachDB VECTOR binding problem directly to a production implementation based on SQL interpolation. That may indeed prove to be the correct solution, but the document no longer contains the small verification spike that earlier revisions wisely introduced. I would restore that spike, let it determine whether native binding is truly impossible, and only then commit to interpolation. That keeps the entire Bug 057 workflow consistent with the evidence-first engineering discipline that the previous revisions established.
