# Adversarial Review: Plan v2 — bchat E2E Critical Path Testing

**Reviewer:** AI Agent (Senior Go Architect)
**Date:** 2026-07-17
**Status:** Rework Required

---

## Summary

V2 addresses all v1 issues correctly, but introduces **one critical blocker** and **four new gaps** that must be resolved before implementation. Clean sequencing and coverage are otherwise solid.

---

## Must Rework

### 1. CRITICAL: No way to obtain widget key for external chat tests (Group 6)

External chat auth requires passing the tenant's `WidgetKey` in the `X-Widget-Key` header. However:

- `OnboardResponse` (handlers.go:1310) returns `ID`, `Slug`, `CompanyName` only — **no `widget_key`**
- `HandleGetTenantFullConfig` (handlers.go:753) returns tenant info — **no `widget_key`**
- `HandleUpdateTenant` (handlers.go:801) does **not** accept `widget_key` in the request body
- There is **no API endpoint** to create, read, or set a bridge auth key

The test scripts have no way to obtain or set the widget key via HTTP. Tests 6.1-6.3 will all fail at auth.

**Options:**
1. **(Recommended) Minor code change**: Add `widget_key` to `HandleGetTenantFullConfig` response and/or `HandleUpdateTenant` request body. This also unblocks actual operational needs (admins need to know their widget key).
2. **Extract from widget.js**: Parse the generated `widget.js` output which contains the key — fragile but requires no code change.
3. **Skip Group 6**: Document the gap and defer external chat coverage.

The plan should specify which option is chosen and add the necessary code change to the scope.

---

### 2. Reindex is async — RAG search will hit before indexing completes (Group 5)

`HandleReindexTenant` returns `202 Accepted` immediately and runs reindex in a background goroutine (handlers.go:1228). Test 5.2 (`POST /:slug/rag/search`) will likely return empty results if it runs before reindex finishes.

**Fix:** Add a polling step between 5.1 and 5.2 using `GET /api/v1/agent/:slug/reindex/status` until status indicates completion, or add a fixed sleep with a comment explaining the async nature.

---

### 3. Grant permission test needs a second user (Group 9)

`HandleGrantPermission` (handlers.go:2639) requires a `user_id` in the request body — it looks up the user and confirms they exist. The admin/HOST user from Group 1 will already have all permissions, so granting to them is uninteresting and may not test the path correctly.

**Fix:** Add a step in Group 1 to create a second user (e.g., `viewer-user` with limited role), capture their user ID from the signup response, and use that ID for the grant/revoke tests.

---

### 4. Onboard request body undocumented (Test 2.1)

The plan references `POST /api/v1/agent/onboard` but doesn't specify the required form fields. The handler expects `multipart/form-data` with at minimum:

| Field | Required | Value |
|-------|----------|-------|
| `tenant_slug` | Yes | e.g. `e2e-test-tenant` |
| `company_name` | No | e.g. `E2E Test Corp` |
| `vertical` | No | e.g. `restoration` |
| `external_kb_file` | No | `@lib/KB.MD` (file upload) |
| `external_policy_file` | No | `@lib/POLICY.MD` (file upload) |

Include a sample curl invocation like the import endpoint has.

---

### 5. LLM config PUT request body undocumented (Test 4.2)

`HandleSetLLMConfig` (handlers.go:2422) expects a JSON body with these required fields:

```json
{
  "llm_model": "openai/gpt-4o-mini",
  "simulation_human_model": "openai/gpt-4o-mini",
  "reasoning_model": "google/gemini-2.5-pro"
}
```

The plan should specify the expected payload. Without it, implementers will guess wrong.

---

## V1 Rework Items — All Correctly Fixed

| V1 Issue | V2 Status | Evidence |
|----------|-----------|----------|
| 2.4 destroys tenant chain | ✅ Removed; cleanup is Group 11 | Line 39 |
| Import endpoint params undocumented | ✅ Curl invocation + form fields documented | Lines 49-67 |
| Simulation stream route param mismatch | ✅ Uses `:sessionId` with note | Line 108 |
| Missing health check | ✅ `wait_for_server()` polls `GET /healthz` | Lines 176-193 |
| Missing LLM config test | ✅ New Group 4 (2 tests) | Lines 69-77 |
| First-run auth assumption | ✅ Fallback to sign-in on 409 | Line 29 |
| JSON assertions missing | ✅ `JSON Assertions` column added to every table | Passim |
| Taskfile_pg.yml existence | ✅ Confirmed exists at project root | Verified |

---

## Coverage Verifications

- **27 tests across 11 groups** — correct count
- **15 files to create** — matches script structure
- **All 22 endpoints verified** — match actual registered routes (with the widget key auth gap above)
- **`:sessionId` route param** — correctly matches registered route name

---

## Nits

### Naming collision: test script 04 vs file upload group

The v1 plan used `04_rag.sh`. V2 adds `04_llm_config.sh`, shifting all subsequent scripts up by one. This is correct but the plan should double-check that `run_all.sh` references are updated to match the new numbering.

### State leak between test runs

The plan handles auth re-runs (fallback to sign-in on 409) but doesn't handle an orphaned tenant from a previous failed run. If `11_cleanup.sh` didn't run (e.g., SIGKILL), the next run's onboard will get 409. The `run_all.sh` orchestrator should attempt to delete the tenant before creating it.

### Widget key should be in the PATCH endpoint

This relates to the critical blocker above, but worth noting independently: `HandleUpdateTenant` accepts `is_active`, `company_name`, `vertical`, `allowed_domains` but **not** `widget_key`. This is an operational gap even outside testing — if an admin loses the widget key, they can't retrieve or rotate it via API. Consider adding `widget_key` to the PATCH schema as a separate improvement.

---

## Final Verdict

**Rework Required.** The widget key blocker (item 1) is a showstopper for Group 6. Items 2-5 are high-severity gaps that will cause test failures or implementer confusion. V1 fixes are solid and don't need revisiting.
