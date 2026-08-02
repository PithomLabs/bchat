This is **substantially better** than the previous plans. It also shows exactly where DeepSeek V4 is stronger: it stopped making broad architectural assertions and started **proving them against the repository**. 

If I were acting as principal engineer, I'd score the progression like this:

* v1: **6.5/10**
* v2 review: **8.5/10**
* this revised plan: **9.3/10**

That said, I still would **not approve it yet**. I see several remaining issues—some are architectural, some are factual, and one is potentially a major implementation mistake.

---

# What DeepSeek improved dramatically

## 1. It stopped assuming Cockroach incompatibility

This is the biggest improvement.

Instead of saying

> Cockroach doesn't support X

it now says

> audited repository usage
> verified against docs
> verify remaining items experimentally

That is exactly how an engineering plan should be written.

---

## 2. SQL portability audit

Excellent addition.

Instead of vague prose it now has

```
construct

↓

usage

↓

support

↓

action
```

That should stay.

---

## 3. Single implementation

I strongly agree.

This

```
postgres.NewDB()

postgres.NewCockroachDB()
```

is far cleaner than

```
postgres/

cockroach/
```

This removes what I considered the biggest blocker.

---

## 4. Retry inventory

Excellent.

Instead of

> wrap transactions

it inventories every transaction.

That's principal-level planning.

---

## 5. CI

Much improved.

Cross-database parity belongs in CI rather than tribal knowledge.

---

# Remaining issues

These are the things I would still send back.

---

# Issue 1 — `serial_normalization`

This is the one I would investigate before implementation.

The plan proposes

```sql
SET serial_normalization = sql_sequence;
```

at the beginning of

```
LATEST.sql
```

The question is:

**Does this session variable affect migrations executed through the existing migration framework exactly as expected?**

I don't know.

The plan assumes

```
SET

↓

CREATE TABLE

↓

persistent behavior
```

without proving it.

I would require:

```
SHOW CREATE TABLE
```

after migration

to prove

```
nextval(...)
```

was actually generated.

If not,

the whole strategy fails.

So I'd keep this as

> VERIFY FIRST

rather than architectural truth.

---

# Issue 2 — statement splitter

This is actually my biggest concern.

The plan proposes

> split SQL statements

instead of

> execute whole file

Writing a SQL splitter is notoriously difficult.

Consider

```sql
INSERT ...

VALUES (
'hello;
world'
);
```

or

```sql
$$
BEGIN
...
END;
$$
```

A naive semicolon splitter breaks.

If Cockroach already has an accepted migration pattern,

I'd much rather reuse that than maintain a custom parser.

I'd challenge this entire section.

---

# Issue 3 — DDL transactions

Related.

The plan changes

```
transaction

↓

statement-by-statement
```

for Cockroach.

That changes semantics.

If statement 18 fails,

statements 1–17 remain committed.

Previously everything rolled back.

That's a very significant behavioral change.

I would require justification.

Maybe that's correct.

Maybe Cockroach documentation recommends it.

But the plan should explicitly discuss the loss of atomicity.

---

# Issue 4 — `default_query_exec_mode`

The plan now recommends always keeping

```
simple_protocol
```

because of migrations.

I think that's a reasonable hypothesis.

But I'd separate

migration execution

from

runtime query execution.

For example

```
migrator

↓

simple protocol

driver

↓

default protocol
```

That gives the runtime the benefit of pgx's extended protocol while keeping migrations safe.

Right now the plan ties the entire driver to migration behavior.

---

# Issue 5 — Runtime-created vector table

I actually agree with keeping it runtime-created.

But I'd like to see one more justification.

Something like

> this schema only exists when VECTOR support is compiled

That makes the architectural intent obvious.

---

# Issue 6 — Transaction retry helper

This section is good,

but I'd like one additional audit.

Specifically:

```
inside retry closure

↓

external side effects?
```

Examples:

* logging
* metrics
* websocket events
* HTTP calls
* queue publish

Those become duplicated if a transaction retries.

The plan assumes all closures are pure.

It should prove that.

---

# Issue 7 — Build strategy

One thing still bothers me.

The plan now says

```
postgres.NewCockroachDB()
```

inside the PostgreSQL package.

That works.

But I wonder if

```
NewSQLDriver(profile)
```

would be cleaner.

Then

```
profile.Driver

↓

capabilities
```

instead of

```
postgres.NewCockroachDB()
```

It's mostly naming,

but it better reflects reality.

---

# Biggest thing still missing

This is the largest omission.

## Capability matrix

The plan now has a SQL portability table.

I'd go one step further.

Build a capability matrix.

Example

| Capability             | PostgreSQL | Cockroach     | Used by bchat | Abstraction        |
| ---------------------- | ---------- | ------------- | ------------- | ------------------ |
| Retry                  | optional   | mandatory     | yes           | `withTx()`         |
| Vector                 | pgvector   | native VECTOR | yes           | runtime capability |
| SERIAL                 | sequence   | configurable  | yes           | migration layer    |
| JSONB                  | yes        | yes           | yes           | shared             |
| FOR UPDATE SKIP LOCKED | yes        | yes           | yes           | shared             |

That gives future maintainers a design reference, not just an implementation checklist.

---

# One prompt I'd give DeepSeek for the next revision

I would tell it:

> **Do one final adversarial pass focused exclusively on semantic changes introduced by the migration. Every time you modify behavior relative to the PostgreSQL implementation (for example, statement-by-statement migrations, retry wrappers, connection protocol, sequence generation, runtime schema creation), explicitly answer three questions: (1) What exact PostgreSQL behavior is being changed? (2) Why is the change required by CockroachDB rather than merely convenient? (3) What new failure modes or regressions does this introduce? Do not add new features—only justify or challenge every behavioral change already proposed.**

That prompt forces the plan to defend every deviation from the existing implementation, which is the final step before I'd consider it implementation-ready.

## Final verdict

This is now a **high-quality engineering plan**. The remaining issues are no longer about misunderstanding CockroachDB—they're about validating a handful of implementation choices (notably migration execution strategy, `serial_normalization`, and protocol selection) before coding begins. Those are the kinds of questions I'd expect to resolve with small proof-of-concept experiments rather than another major redesign. 



Absolutely. If the goal is **zero hallucination**, I would stop giving the coding agent random documentation and instead give it a **curated reference pack** that it is *required* to cite for every architectural claim.

I would organize it like this.

---

# Bug 057 Reference Pack

```text
bugs/
└──057/
    README.md
    01-architecture.md
    02-transactions.md
    03-migrations.md
    04-sql-compatibility.md
    05-pgx.md
    06-connection-pooling.md
    07-serial.md
    08-schema-changes.md
    09-vectors.md
    10-production.md
    11-local-development.md
```

---

# 01-architecture.md

Official references:

* [Developer Basics](https://www.cockroachlabs.com/docs/stable/developer-basics.html?utm_source=chatgpt.com)
* [Transaction Layer Architecture](https://www.cockroachlabs.com/docs/stable/architecture/transaction-layer/?utm_source=chatgpt.com)

Purpose

Ground all architectural decisions.

Required evidence:

* PostgreSQL wire compatibility
* ACID semantics
* SERIALIZABLE default
* client responsibilities

---

# 02-transactions.md

Official references

* [Transaction Retry Error Reference](https://www.cockroachlabs.com/docs/stable/transaction-retry-error-reference?utm_source=chatgpt.com)
* [Transaction Layer](https://www.cockroachlabs.com/docs/stable/architecture/transaction-layer/?utm_source=chatgpt.com)

Purpose

Everything involving

* retry
* SAVEPOINT
* SERIALIZABLE
* READ COMMITTED
* 40001
* crdb.ExecuteTx

must cite this file.

Never rely on memory.

---

# 03-migrations.md

Official references

* CockroachDB Schema Changes documentation
* Known Limitations
* Online Schema Changes

This should answer

* DDL
* migration execution
* explicit transactions
* prepared statements
* ALTER TABLE
* migration ordering

The current plan still has some uncertainty here, so this deserves its own reference document rather than scattered notes.

---

# 04-sql-compatibility.md

Official references

CockroachDB PostgreSQL Compatibility documentation.

Organize it as

```text
Supported

Partially supported

Unsupported

Behavior differences
```

Not prose.

Example

| SQL Feature | Status | Official URL |
| ----------- | ------ | ------------ |

Every SQL claim in the plan must cite this table.

---

# 05-pgx.md

Official references

Cockroach pgx integration docs.

Include

* crdb.ExecuteTx
* pgx
* database/sql
* retry wrapper
* QueryExecMode

This prevents invented recommendations.

---

# 06-connection-pooling.md

Official references

Cockroach production checklist.

Summarize

* max connections
* cluster vCPU rule
* idle connections
* connection lifetime

This avoids future hallucinations like

> runtime.NumCPU()

instead of

> cluster CPUs.

---

# 07-serial.md

Official references

Cockroach SERIAL documentation.

Include

* serial_normalization
* rowid
* sql_sequence
* virtual_sequence
* unique_rowid
* identity columns

This document alone would have prevented the first plan's largest mistake.

---

# 08-schema-changes.md

Official references

Known Limitations

Online Schema Changes

Topics

* prepared statements
* schema changes
* DDL
* transaction restrictions
* ALTER COLUMN TYPE

This becomes the single authority.

---

# 09-vectors.md

Official references

Cockroach VECTOR documentation.

Include

* VECTOR type
* vector indexes
* supported operators
* ANN
* limitations

Then nobody invents pgvector assumptions.

---

# 10-production.md

Official references

Production checklist.

Topics

* TLS
* sslmode
* verify-full
* certificates
* locality
* backups
* retries
* monitoring

---

# 11-local-development.md

Official references

Docker

Single node

Testing

Compose

Environment variables

Bootstrap

---

# Add a Rules.md

This is actually the most important file.

I'd give DeepSeek something like this:

```markdown
# Rules

Every technical claim MUST be classified.

## VERIFIED

Supported by:

- repository code

or

- official Cockroach documentation

Citation required.

---

## REPOSITORY FACT

Derived from source code.

Must include

- filename
- line numbers

---

## DOCUMENTATION FACT

Derived from official Cockroach docs.

Must include

- document name
- section

---

## INFERENCE

Logical conclusion from verified facts.

Must explicitly say

"This is an inference."

---

## SPECULATION

Allowed only in recommendations.

Must begin with

"I recommend..."

Never present speculation as fact.

---

## PROHIBITED

Never write

- "Cockroach doesn't support X"

unless official documentation says so.

Never write

- "should"

without explaining why.

Never recommend changing architecture without first explaining the problem.
```

---

# Add one more document

This would dramatically improve future plans.

```
12-proof-checklist.md
```

For every recommendation,

require proof.

Example

```markdown
# Recommendation

Keep simple protocol.

Proof required

□ Official documentation

□ Repository evidence

□ Experimental verification

□ Benchmark

□ Recommendation only
```

The coding agent cannot move forward until it checks one.

---

## Finally, I'd change the prompt

Instead of

> Produce implementation plan

I'd say

> **Produce an implementation plan whose every non-trivial engineering claim is backed by either (1) repository evidence, (2) official CockroachDB documentation, or (3) an explicitly labeled experiment that must be run before implementation. If a claim cannot be supported by one of those three categories, remove it from the plan.**

That single instruction is probably the highest-leverage change you can make. It transforms the plan from "expert opinion" into an evidence-backed engineering design, and it naturally suppresses hallucinated compatibility claims.
