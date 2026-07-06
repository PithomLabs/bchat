# Bchat/022 bug fix plan discussion

**Session ID:** ses_0c9d6c890ffeYrO3jmOdfKf2sN
**Created:** 7/6/2026, 2:41:38 PM
**Updated:** 7/6/2026, 3:25:57 PM

---

## User

---

## Assistant

## Goal
- Implement all 14 security fixes (13 from DeepWiki analysis + memo comment isolation) in bugs/022, incorporating 6 nits from plan_imp_review.md

## Constraints & Preferences
- CORS admin default: empty (deny cross-origin)
- Deployment always single-container → accept gRPC loopback risk, document only
- Playground stays public for demo/marketing
- Interactive Q&A until plan approved; no implementation until approval
- Nit #1: Use sentinel error `ErrMessageTooLong` instead of string matching
- Nit #2: Login rate limiting must avoid FK violation (tenant_id=0 on agent_rate_limits) → use in-memory rate limiter
- Nit #3: Admin tenant binding middleware must use closure with store reference
- Nit #4: HMAC token expiry returned as separate query param
- Nit #5: SQLite migration idempotency (ALTER TABLE can't use IF NOT EXISTS)
- Nit #6: Add `getEnvSlice` helper for CORS origins env var

## Progress
### Done
- Read `bugs/022/deepwiki.md` and `bugs/022/plan.md`
- Wrote `bugs/022/plan_imp.md` with full implementation plan (all 14 issues)
- Read `bugs/022/plan_imp_review.md` for 6 nits
- Phase 1: Memo comment isolation (#14) — migration, store layer, handlers
- Phase 1: Cross-tenant audit (#11) — verified existing patterns, defense-in-depth added
- Phase 2: Remove never-expire tokens (#4) — always use 7-day expiry
- Phase 2: HMAC session tokens for transcript (#6) — with expiry param (nit #4)
- Phase 2: Rate limiting on login (#9) — in-memory limiter (nit #2), internal chat 30 RPM
- Phase 3: Message length limit (#2) — sentinel error `ErrMessageTooLong` (nit #1), `MaxMessageLength` field
- Phase 3: Input sanitization (#3) — `SanitizeUserInput` + `detectPromptInjection`
- Phase 3: Slug enumeration fix (#8) — uniform "Access denied" for all tenant lookups
- Phase 4: Route-level CORS (#1) — per-group CORS with `getEnvSlice` helper (nit #6)
- Phase 4: Domain allowlist fix (#13) — empty array now denies all
- Phase 5: gRPC insecure docs (#5) — comment documenting accepted risk

### In Progress
- Phase 5: ENCRYPTION_MASTER_KEY safeguards (#10)
- Phase 5: Admin tenant binding middleware (#12) — with nit #3 (closure)
- Phase 5: Playground demo provisioning (#7)

### Blocked
- (none)

## Key Decisions
- Login rate limiting uses new `loginRateLimiter` in-memory struct (avoids FK constraint on `agent_rate_limits` requiring `tenant_id` to reference `agent_tenants`)
- HMAC session tokens: `generateSessionToken`/`verifySessionToken` in `service.go`, 30-min expiry, tenant GUID as key
- `MaxMessageLength` added to `AgentAudience` store struct with DB column (default 4000)
- `isDomainAllowed`: `AllowedDomains == "[]"` now denies all (was allow-all)
- Per-group CORS: `publicCORS` (AllowOrigins: `["*"]`) for widget/chat, `adminCORS` (from `ADMIN_CORS_ORIGINS` env) for admin/auth routes
- gRPC-web `WithOriginFunc` now checks against `adminOrigins` instead of always returning true
- Never-expire token field kept in proto for backward compat, but always uses `AccessTokenDuration` (7 days)

## Next Steps
- Implement ENCRYPTION_MASTER_KEY safeguards (#10): startup validation in `main.go`, backup key support in `encryption.go`
- Implement admin tenant binding middleware (#12) with closure pattern (nit #3): `AllowedTenantIDs` on User, migration, middleware
- Implement playground demo provisioning (#7): move `ensurePlaygroundDemo` to startup task
- Verify all changes compile (`go build`)
- Run tests

## Critical Context
- `agent_rate_limits` table has FK `REFERENCES agent_tenants(id)` — cannot use `tenant_id=0`
- `MemoRelation` and `FindMemoRelation` now have `TenantID *int32` field
- `ListMemoRelations` SQL updated with `mr.` table prefix for all columns and new `tenant_id` WHERE clause
- `ChatResponse` struct now includes `SessionToken` and `SessionTokenExpiry` fields
- `ErrMessageTooLong` sentinel error defined in `agent/service.go`
- Migration files created in `0.28/00__add_tenant_to_memo_relation.sql` and `0.28/01__add_max_message_length.sql`
- `ListAgentAudiences` SQL query and INSERT/UPDATE all updated for `max_message_length` column
- `login_ratelimit.go` is a new file with in-memory sliding window rate limiter

## Relevant Files
- `bugs/022/deepwiki.md`: Security analysis with 13 issues
- `bugs/022/plan.md`: Original fix plan from DeepWiki
- `bugs/022/plan_imp.md`: Full implementation plan written by assistant
- `bugs/022/plan_imp_review.md`: Review with 6 nits incorporated
- `store/migration/sqlite/0.28/00__add_tenant_to_memo_relation.sql`: New migration for memo tenant isolation
- `store/migration/sqlite/0.28/01__add_max_message_length.sql`: New migration for message length column
- `store/memo_relation.go`: Added `TenantID *int32` to `MemoRelation` and `FindMemoRelation`
- `store/db/sqlite/memo_relation.go`: Updated `UpsertMemoRelation` and `ListMemoRelations` for tenant_id
- `store/agent.go`: Added `MaxMessageLength int` to `AgentAudience`
- `store/db/sqlite/agent.go`: Updated INSERT/UPDATE/SELECT for `max_message_length` column
- `server/router/api/v1/memo_service.go`: Tenant filtering in CreateMemoComment, ListMemoComments, DeleteMemo cascade, handleTicketAIResponse
- `server/router/api/v1/memo_relation_service.go`: Tenant filtering in SetMemoRelations, ListMemoRelations, convertMemoRelationFromStore
- `server/router/api/v1/agent/service.go`: `ErrMessageTooLong`, `SanitizeUserInput`, `detectPromptInjection`, `generateSessionToken`, `verifySessionToken`, `SessionToken`/`SessionTokenExpiry` in `ChatResponse`, message length validation in `ChatExternal`
- `server/router/api/v1/agent/handlers.go`: HMAC token verification in transcript endpoint, message length error handling, slug enumeration uniform errors, domain allowlist fix, internal chat rate limiting
- `server/router/api/v1/v1.go`: Route-level CORS (public/admin), `getEnvSlice` helper, `loginRateLimiter` field, gRPC-web origin check, gRPC insecure credential documentation
- `server/router/api/v1/auth_service.go`: Never-expire deprecation, login rate limiting
- `server/router/api/v1/auth.go`: Contains `AccessTokenDuration`, `MaxNeverExpireDuration` constants
- `server/router/api/v1/login_ratelimit.go`: New file — in-memory rate limiter for login endpoints
- `internal/crypto/encryption.go`: Pending — needs backup key support
- `bin/memos/main.go`: Pending — needs startup validation for ENCRYPTION_MASTER_KEY

---

## User

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: read**

---

## Assistant

Now let me verify the changes compile:

**Tool: bash**

---

## Assistant

**Tool: grep**

**Tool: grep**

---

## Assistant

**Tool: read**

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

The test failures are because the test DB doesn't have the new migration `0.28/00__add_tenant_to_memo_relation.sql` applied. Let me check how migrations run:

**Tool: grep**

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: grep**

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: bash**

---

## Assistant

The memo_relation and max_message_length failures are fixed. Now there's a test failure in `bridge_delivery_test.go` because the transcript endpoint now requires HMAC tokens. Let me check the test:

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

I need to fix the tests that call `HandleGetExternalTranscript` to include token/expiry. Let me find all test occurrences:

**Tool: bash**

---

## Assistant

**Tool: grep**

---

## Assistant

I need to add a test helper that generates a valid HMAC token, and update all test callsites. Let me look at `generateSessionToken`:

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

I need to update the tests to include HMAC tokens. Since they're in the same package, they can call `generateSessionToken` directly. Let me read each test callsite and fix them:

**Tool: read**

---

## Assistant

I'll add a helper to generate test tokens and fix all test callsites:

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

Now fix each test callsite. First one at line 78:

**Tool: edit**

---

## Assistant

Now let me find and fix the other occurrences:

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: edit**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

This test intentionally triggers an error (no token) to verify session ID isn't logged — should stay as-is. Let me verify build and tests:

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

The HMAC tokens are failing because the test tenants may not have a GUID set. Let me check how test tenants are created:

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: grep**

---

## Assistant

The test tenants have empty GUID. Let me check if the `GUID` is auto-populated:

**Tool: read**

---

## Assistant

**Tool: read**

---

## Assistant

The issue is the session ID normalization. The test passes `"session-visitor-1"` directly but the handler calls `NormalizeExternalSessionID` which may transform it:

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

The token verification is failing. Let me check if `GUID` is being populated on the test tenants by checking the ListAgentTenants query:

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

Let me check if the GUID might be empty for test tenants:

**Tool: read**

---

## Assistant

The test tenants have GUIDs auto-generated by sqlite. The issue is likely that the handler uses `h.store` to look up the tenant and gets the same GUID. Let me check if `HandleGetExternalTranscript` uses `h.store` or `h.service` for the tenant lookup:

**Tool: read**

---

## Assistant

**Tool: grep**

---

## Assistant

The issue is that RFC3339 timestamps with `+` in timezone offsets get mangled by URL parsing. Let me fix the helper:

**Tool: read**

---

## Assistant

The problem is that RFC3339 timestamps with `+` in timezone offsets get converted to spaces during URL parsing. I need to URL-encode the query params properly:

**Tool: edit**

---

## Assistant

Now rebuild and test:

**Tool: bash**

---

## Assistant

**Tool: bash**

---

## Assistant

All four pass. Now run the full test suite:

**Tool: bash**

---

## Assistant

Two still failing. Let me check the first one - the log test:

**Tool: read**

---

## Assistant

The `TestBChatLiveTranscriptEndpointDoesNotLogRawSessionID` test expects the handler to reach the tenant lookup before failing, but now it returns early because of missing token/expiry. I need to update this test to provide a valid token, then the failure will come from the dropped `agent_transcripts` table.

**Tool: edit**

---

## Assistant

Now check the other failing test:

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

The test checks `ChatResponse` has exactly 5 fields but we added `SessionToken` and `SessionTokenExpiry` (now 7). Let me fix it:

**Tool: edit**

---

## Assistant

**Tool: grep**

---

## Assistant

**Tool: read**

---

## Assistant

**Tool: bash**

---

## Assistant

All tests pass. Let me update the progress:

**Tool: todowrite**

---

## Assistant

All 14 security fixes are now implemented and all tests pass. Here's what was completed in this session:

## Completed
1. **ENCRYPTION_MASTER_KEY safeguards (#10)**: Added startup validation for key length + backup key support in `internal/crypto/encryption.go` with `ENCRYPTION_MASTER_KEY_BACKUP` env var and `decryptWithKey` helper
2. **Admin tenant binding middleware (#12)**: Created `server/router/api/v1/tenant_binding.go` with closure pattern (nit #3), added `AllowedTenantIDs` to User struct, migration `02__add_allowed_tenant_ids_to_user.sql`, wired into admin group
3. **Playground demo provisioning (#7)**: Added `StartupSeedPlaygroundDemos()` method, wired into `server.go` at startup, removed per-request auto-provisioning from catalog/run endpoints

## Test fixes
- Updated `ChatResponse` field count test (5→7)
- Fixed transcript test to use HMAC tokens via `testTranscriptURL` helper
- Updated LATEST.sql with `tenant_id`, `max_message_length`, `allowed_tenant_ids` columns
- Removed duplicate `publicCORS` declaration

All 14 issues from the DeepWiki security analysis are now implemented with the 6 nits incorporated.

---

