# Bug Report: QA Pairs Tenant Isolation Vulnerabilities

**Date:** 2026-01-30
**Status:** Fixed
**Severity:** Critical (Security vulnerability)

---

## Summary

The Generate Pairs and Test Pairs functionality in Agent Admin had **critical tenant isolation bugs** where delete and update operations did not verify that the QA pair belonged to the requesting tenant.

---

## Issues Found

### Issue #1: CRITICAL - DeleteAgentQAPair Missing Tenant Verification

**Location:**
- `server/router/api/v1/agent/handlers.go:4398`
- `store/db/sqlite/agent.go:1902`

**Problem:** Delete operation did not verify the QA pair belongs to the resolved tenant.

```go
// BEFORE (vulnerable)
// Handler resolves tenant but doesn't use it for verification
tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
// ...
// Delete ignores tenant - could delete another tenant's pair!
if err := h.store.DeleteAgentQAPair(ctx, int32(id)); err != nil {
```

**Database layer had no tenant check:**
```sql
-- BEFORE (vulnerable)
DELETE FROM agent_qa_pairs WHERE id = ?  -- NO tenant_id verification!
```

**Impact:** An admin could delete another tenant's QA pair by knowing/guessing its ID.

---

### Issue #2: HIGH - UpdateAgentQAPair Missing Tenant Verification

**Location:**
- `server/router/api/v1/agent/handlers.go:4369`
- `store/db/sqlite/agent.go:1876-1898`

**Problem:** Same pattern - update did not verify tenant ownership.

```sql
-- BEFORE (vulnerable)
UPDATE agent_qa_pairs SET ... WHERE id = ?  -- NO tenant_id check!
```

**Impact:** An admin could modify another tenant's QA pair data.

---

## Fixes Applied

### 1. Updated Interface Signatures

**File:** `store/driver.go` and `store/agent.go`

```go
// BEFORE
DeleteAgentQAPair(ctx context.Context, id int32) error
UpdateAgentQAPair(ctx context.Context, pair *AgentQAPair) (*AgentQAPair, error)

// AFTER - Added tenant verification
DeleteAgentQAPair(ctx context.Context, id int32, tenantID int32) error
UpdateAgentQAPair(ctx context.Context, pair *AgentQAPair, tenantID int32) (*AgentQAPair, error)
```

### 2. Updated SQLite Implementation

**File:** `store/db/sqlite/agent.go`

```go
// DELETE - Now verifies tenant ownership
func (d *DB) DeleteAgentQAPair(ctx context.Context, id int32, tenantID int32) error {
    result, err := d.db.ExecContext(ctx,
        "DELETE FROM agent_qa_pairs WHERE id = ? AND tenant_id = ?", id, tenantID)
    if err != nil {
        return err
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("QA pair not found or not owned by tenant")
    }
    return nil
}

// UPDATE - Now verifies tenant ownership
func (d *DB) UpdateAgentQAPair(ctx context.Context, pair *store.AgentQAPair, tenantID int32) (*store.AgentQAPair, error) {
    // ... UPDATE query now includes: WHERE id = ? AND tenant_id = ?
    // Returns error if rows affected = 0
}
```

### 3. Updated Handlers

**File:** `server/router/api/v1/agent/handlers.go`

```go
// DELETE - Now passes tenant.ID
if err := h.store.DeleteAgentQAPair(ctx, int32(id), tenant.ID); err != nil {
    slog.Error("failed to delete Q&A pair", "error", err, "pairID", id, "tenantID", tenant.ID)
    return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete Q&A pair")
}

// UPDATE - Now passes tenant.ID
updated, err := h.store.UpdateAgentQAPair(ctx, &pair, tenant.ID)
```

### 4. Updated Database Stubs

**Files:** `store/db/mysql/agent.go`, `store/db/postgres/agent.go`

Updated function signatures to match the new interface.

---

## Files Modified

| File | Changes |
|------|---------|
| `store/driver.go` | Updated `DeleteAgentQAPair` and `UpdateAgentQAPair` signatures |
| `store/agent.go` | Updated interface and delegate method signatures |
| `store/db/sqlite/agent.go` | Added tenant verification to Delete and Update with RowsAffected check |
| `store/db/mysql/agent.go` | Updated stub signatures |
| `store/db/postgres/agent.go` | Updated stub signatures |
| `server/router/api/v1/agent/handlers.go` | Pass `tenant.ID` to delete/update calls, added error logging |

---

## Verification

1. **Build verification:** `go build ./bin/memos/main.go` - Passes
2. **Manual test:**
   - Create QA pairs for tenant A
   - Switch to tenant B in Agent Admin
   - Attempt to delete tenant A's pair ID via API
   - Should return error: "QA pair not found or not owned by tenant"

---

## Security Implications

This was a **cross-tenant data access vulnerability**. While the attack required admin privileges, it still violated the principle of tenant isolation in a multi-tenant system.

### Attack Vector (Before Fix)
1. Admin authenticates to Agent Admin
2. Admin selects Tenant B
3. Admin crafts DELETE request: `DELETE /api/v1/agent/tenant-b/qa-pairs/123`
4. Where ID 123 belongs to Tenant A
5. **Result:** Tenant A's QA pair deleted (VULNERABILITY)

### After Fix
Same attack vector now returns error: "QA pair not found or not owned by tenant"

---

## Related Issues

### Issue #3 (Not Fixed - Future Enhancement): Global Admin Only

All QA pair operations currently require global admin role (`RoleHost` or `RoleAdmin`), ignoring tenant-level permissions like `chat:test` or `tenant:admin`.

This is a design consideration, not a security bug. Future enhancement could allow users with `chat:test` permission to manage QA pairs for their assigned tenants.

---

## Lessons Learned

1. **Always verify resource ownership** - Don't rely solely on URL-based tenant context
2. **Use compound WHERE clauses** - `WHERE id = ? AND tenant_id = ?` instead of just `WHERE id = ?`
3. **Check RowsAffected** - Return meaningful errors when no rows match
4. **Add error logging** - Helps with debugging and security auditing
