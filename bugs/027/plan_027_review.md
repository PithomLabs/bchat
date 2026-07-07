# Adversarial review of plan_027.md under bugs/027

**Session ID:** ses_0c28110b0ffeCHJbBa88vY18NT
**Created:** 7/8/2026, 12:52:36 AM
**Updated:** 7/8/2026, 12:54:54 AM

---

## User

under bugs/027 do adversarial plan review of plan_027.md and say if approved with nits or rework

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

Let me verify the claims in the plan against the actual codebase.

**Tool: task**

---

## Assistant

Now let me check one more thing - the `00__relax_agent_rate_limits_fk.sql` migration to verify idempotency:

**Tool: read**

---

## Assistant

---

## Adversarial Plan Review: plan_027.md

**Verdict: APPROVED WITH NITS**

The plan is well-researched, root cause is correctly identified, and the migration approach is sound. However, I found 2 issues that should be addressed before implementation.

---

### NIT 1 — SECURITY: Don't leak error details to HTTP response

**Plan says** (`handlers.go` fix):
```go
return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save config: "+err.Error())
```

**Problem**: Appending `err.Error()` to the HTTP response exposes internal database details (table names, column names, SQL syntax) to clients. This is a security anti-pattern.

**Fix**: Keep the user-facing message generic. The `slog.Error` line is the correct place for diagnostics:
```go
slog.Error("failed to save tenant config", "tenant", slug, "error", err)
return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save config")
```

---

### NIT 2 — FACTUAL ERROR: Postgres/MySQL risk section is wrong

**Plan claims** (line 79):
> Verify Postgres/MySQL: their `rbac.go` upserts also reference `vector_db_s3_override`

**Investigation shows this is false:**
| Driver | `UpsertTenantConfig` | References `vector_db_s3_override`? | `LATEST.sql` has column? |
|--------|----------------------|-------------------------------------|--------------------------|
| SQLite | Full implementation | Yes (lines 265, 278) | Yes (line 461) |
| Postgres | Full implementation | **No** (lines 172-184) | **No** |
| MySQL | Stub (`return nil, nil`) | N/A | N/A |

The Postgres driver silently ignores `VectorDBS3Override` — no crash, but data is lost. The risk is different from what the plan describes. The plan should note this correctly or remove the Postgres/MySQL section entirely since the user is on SQLite.

---

### Everything else checks out

| Claim | Verified? |
|-------|-----------|
| `UpsertTenantConfig` references missing column in SQLite | ✅ Confirmed (`store/db/sqlite/rbac.go:265,278`) |
| `execute()` tolerates "duplicate column" errors | ✅ Confirmed (`store/migrator.go:259-273`) |
| Migration file naming/versioning is correct | ✅ `0.30/01__` → `0.30.2` > current `0.30.1` |
| Handler doesn't log the error | ✅ Confirmed (`handlers.go:2468-2471`) |
| `LATEST.sql` has the column for SQLite | ✅ Confirmed (line 461) |
| Validation steps are practical | ✅ Good coverage |

---

**Summary**: Core fix is correct. Address the two nits (security leak in HTTP response + factual Postgres/MySQL claim) and it's good to go.

---

