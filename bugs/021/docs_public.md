# Security Investigation: Memo Visibility & Tenant Isolation

**Date:** 2026-07-05
**Scope:** Multi-tenant security of memo visibility settings in bchat

---

## Executive Summary

**Setting memos to PUBLIC is a security risk in the current architecture.** Memos lack a `TenantID` field, meaning PUBLIC memos are globally accessible to anonymous users across all tenants. Even PROTECTED memos leak across tenant boundaries to any authenticated user. The bchat platform is not designed to publish memos to the internet, yet the current visibility model inherits a public-facing design from the original Memos application.

**Recommendation:** Disable PUBLIC visibility at the workspace level (`DisallowPublicVisibility=true`), and implement tenant-scoped visibility to retain an intra-tenant "public" concept without cross-tenant exposure.

---

## 1. Current Visibility Model

### 1.1 Visibility Levels

| Level | Anonymous Access | Authenticated Access | Cross-Tenant |
|-------|-----------------|---------------------|--------------|
| **PUBLIC** | Full read access | Full read access | **YES - Global** |
| **PROTECTED** | Denied | Read access | **YES - Global** |
| **PRIVATE** | Denied | Creator + Super only | No (creator-scoped) |

### 1.2 Key Code Paths

**Memo creation** (`server/router/api/v1/memo_service.go:40-61`):
- Non-super users are always forced to `Private` (line 53)
- `DisallowPublicVisibility` workspace setting blocks PUBLIC (line 59-61)
- Default fallback on conversion is `Private` (`memo_service_converter.go:147`)

**Memo listing** (`server/router/api/v1/memo_service.go:158-179`):
- Anonymous users: see only PUBLIC memos
- Authenticated users: see own memos + PUBLIC + PROTECTED from any user
- Super users: see everything

**Memo retrieval** (`server/router/api/v1/memo_service.go:250-261`):
- PUBLIC: no auth required
- PROTECTED: must be logged in
- PRIVATE: creator or super user only

---

## 2. Critical Security Issues

### 2.1 Memos Have No TenantID (CRITICAL)

**File:** `store/memo.go:36-56`, `store/migration/sqlite/LATEST.sql:41-52`

The `memo` table schema:
```sql
CREATE TABLE memo (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uid TEXT NOT NULL UNIQUE,
    creator_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'PRIVATE',
    -- NO tenant_id COLUMN
);
```

The `Memo` struct:
```go
type Memo struct {
    ID         int32
    UID        string
    CreatorID  int32
    Content    string
    Visibility Visibility
    Payload    *MemoPayload
    // NO TenantID field
}
```

**Impact:** All memos exist in a global shared namespace. There is zero tenant isolation at the data layer.

### 2.2 Escalation Memos Contain PII with Weak Visibility (CRITICAL)

**File:** `server/router/api/v1/agent/service.go:3695-3800`

When the agent creates escalation tickets, the associated memo:
- Contains customer PII (name, phone, email, location)
- Contains session details and tenant ID as plaintext in content
- Is created with `Visibility: Protected` (line 3754)
- Has no `TenantID` field on the memo record

**Any authenticated user can read escalation memos from ANY tenant** by listing memos with PROTECTED visibility.

### 2.3 Tickets Have No TenantID (CRITICAL)

**File:** `store/ticket.go:24-36`, `store/migration/sqlite/LATEST.sql:146-165`

The `tickets` table:
```sql
CREATE TABLE tickets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    -- NO tenant_id COLUMN
);
```

Ticket deduplication (`service.go:3592`) scans ALL tickets globally:
```go
tickets, err := s.store.ListTickets(ctx, &store.FindTicket{Type: &ticketType})
// Iterates over ALL tickets to find matching tenant
```

### 2.4 PUBLIC Memos Expose Data to Anonymous Users (HIGH)

If any user sets a memo to PUBLIC (or if the default changes), that memo becomes readable by:
- Unauthenticated HTTP requests
- RSS feed subscribers (`server/router/rss/rss.go:51,84`)
- Any crawler or bot

In a multi-tenant SaaS deployment, this means Tenant A's data is exposed to Tenant B's anonymous visitors.

---

## 3. Data Isolation Matrix

| Data Type | Has TenantID | Tenant-Scoped Queries | Isolation Status |
|-----------|-------------|----------------------|------------------|
| Agent Sessions | Yes | Yes | **OK** |
| Agent Messages | Yes | Yes (defense-in-depth gap) | **OK** |
| Source Files (KB/Policy/Script) | Yes | Yes | **OK** |
| RAG Vectors (LanceDB) | Yes | Yes | **OK** |
| Audiences/Services/FAQs | Yes | Yes | **OK** |
| Intents/Rules/Safety | Yes | Yes | **OK** |
| Learning Memory | Yes | Yes | **OK** |
| Observations | Yes | Yes | **OK** |
| **Memos** | **No** | **No** | **CRITICAL GAP** |
| **Tickets** | **No** | **No** | **CRITICAL GAP** |

---

## 4. Can We Retain "Public" for Tenant-Scoped Use?

### 4.1 Current Problem

The word "PUBLIC" in the current codebase means **globally public** — visible to anyone, including anonymous users and the internet. This is inherited from the original Memos application, which is a single-tenant note-taking app.

### 4.2 Proposed Solution: Tenant-Scoped Visibility

Redefine the visibility semantics to be tenant-aware:

| Current | Proposed | Meaning |
|---------|----------|---------|
| PUBLIC | **TENANT_PUBLIC** | Visible to any authenticated user **within the same tenant** |
| PROTECTED | **WORKSPACE_PROTECTED** | Visible to any authenticated user in the workspace |
| PRIVATE | PRIVATE | Creator + super user only |

### 4.3 Implementation Options

**Option A: New Visibility Level (Recommended)**

Add a new `TENANT_PUBLIC` visibility level that is filtered by `tenant_id`:

1. Add `tenant_id` column to `memo` table
2. Add `TENANT_PUBLIC` to the visibility enum
3. In listing/retrieval queries, filter `TENANT_PUBLIC` memos by the requesting user's tenant
4. Keep `DisallowPublicVisibility=true` to block true PUBLIC visibility
5. Allow tenants to use `TENANT_PUBLIC` for intra-tenant sharing

**Pros:** Clean separation, backward compatible, explicit semantics
**Cons:** Requires migration, enum changes

**Option B: Keep Existing Levels, Add Tenant Filtering**

1. Add `tenant_id` column to `memo` table
2. Keep PUBLIC/PROTECTED/PRIVATE as-is
3. Add tenant filtering to all memo queries
4. PUBLIC means "visible within the tenant to anyone" (anonymous or authenticated)
5. PROTECTED means "visible within the tenant to authenticated users"
6. PRIVATE means "creator + super user only"

**Pros:** No enum changes, simpler migration
**Cons:** "PUBLIC" name is misleading for tenant-scoped use

**Option C: Disable PUBLIC, Use PRIVATE + Tenant Scoping**

1. Set `DisallowPublicVisibility=true` globally
2. Add `tenant_id` column to `memo` table
3. Add tenant filtering to all memo queries
4. Use PRIVATE for creator-only, add a new "TENANT_VISIBLE" for intra-tenant sharing
5. Never expose memos to anonymous users

**Pros:** Most secure, no anonymous access
**Cons:** No intra-tenant sharing without new visibility level

### 4.4 Recommendation

**Option A** provides the cleanest security model:

```
PRIVATE          → Creator + super user only
TENANT_PUBLIC    → Any authenticated user within the same tenant
(TENANT_PROTECTED → Optional: any authenticated user in the workspace)
```

With `DisallowPublicVisibility=true` enforced globally, no memo can ever be exposed to anonymous users or the internet. The "public" concept is retained but scoped to the tenant boundary.

---

## 5. Immediate Mitigations (Before Full Fix)

### 5.1 Enable DisallowPublicVisibility

Set in workspace settings or environment:
```
DISALLOW_PUBLIC_VISIBILITY=true
```

This blocks PUBLIC memos at the API level but does NOT fix the PROTECTED cross-tenant leak.

### 5.2 Restrict Escalation Memo Visibility

Change `service.go:3754` from `store.Protected` to `store.Private`:
```go
memo := &store.Memo{
    // ...
    Visibility: store.Private,  // Was: store.Protected
}
```

This limits escalation memos to creator (system user) + super users only.

### 5.3 Add TenantID to Memo Table

Create a migration:
```sql
ALTER TABLE memo ADD COLUMN tenant_id INTEGER DEFAULT NULL;
CREATE INDEX idx_memo_tenant ON memo(tenant_id);
```

Backfill from escalation memo content (parse tenant ID from markdown).

Add tenant filtering to `ListMemos` and `GetMemo` queries.

---

## 6. Affected Files

| File | Lines | Change Required |
|------|-------|----------------|
| `store/memo.go` | 36-56, 58-87 | Add TenantID field to Memo and FindMemo |
| `store/migration/sqlite/` | New migration | Add tenant_id column + index |
| `server/router/api/v1/memo_service.go` | 40-61, 158-179, 250-261 | Tenant filtering in create/list/get |
| `server/router/api/v1/memo_service_converter.go` | 125-148 | Add TENANT_PUBLIC conversion |
| `store/db/sqlite/memo.go` | 42-210 | Add tenant_id to SQL queries |
| `store/db/sqlite/memo_filter.go` | 100-198 | Add tenant_id to CEL filter |
| `store/db/postgres/memo.go` | 42-210 | Add tenant_id to SQL queries |
| `store/db/postgres/memo_filter.go` | 100-189 | Add tenant_id to CEL filter |
| `server/router/api/v1/agent/service.go` | 3592, 3754 | Fix ticket dedupe, change escalation visibility |
| `server/router/rss/rss.go` | 51, 84 | Scope RSS to tenant |
| `proto/api/v1/memo_service.proto` | 121-126 | Add TENANT_PUBLIC enum |
| `store/ticket.go` | 24-36 | Add TenantID field |
| `store/migration/sqlite/` | New migration | Add tenant_id to tickets table |

---

## 7. Conclusion

1. **PUBLIC memos are a security risk** — they expose tenant data to anonymous users globally
2. **PROTECTED memos also leak** — they expose tenant data to any authenticated user from any tenant
3. **Memos and tickets lack TenantID** — the fundamental architectural gap
4. **RAG, sessions, and agent data are properly isolated** — the gap is specific to inherited Memos platform tables
5. **We can retain a "public" concept** — by scoping it to the tenant boundary with a new `TENANT_PUBLIC` visibility level

The bchat platform is not meant to publish memos to the internet. The visibility model should reflect this by making tenant isolation the default and eliminating true public exposure.
