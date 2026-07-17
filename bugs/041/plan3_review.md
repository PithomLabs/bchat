# Adversarial Review: Plan v3 — bchat E2E Critical Path Testing

**Reviewer:** AI Agent (Senior Go Architect)
**Date:** 2026-07-17
**Status:** Approved with Nits

---

## Summary

V3 resolves all v2 rework items. Two minor bugs remain but neither is a blocker. Ready for implementation.

---

## Bugs

### 1. Grant permission curl sends string instead of array of strings

Line 202:
```bash
-d "{\"user_id\":$VIEWER_USER_ID,\"permissions\":\"tenant:read\"}"
```

`GrantPermissionRequest.Permissions` is `[]string` (handlers.go:2576). This will fail JSON decode — Go returns "cannot unmarshal string into Go struct field GrantPermissionRequest.permissions of type []string".

**Fix:** Send as array:
```bash
-d "{\"user_id\":$VIEWER_USER_ID,\"permissions\":[\"tenant:read\"]}"
```

### 2. Onboard response assertion expects `widget_key` but code change only adds to config

Line 77 asserts `tenant.widget_key` present in the onboard response. The code changes (Scope Addition) only add `widgetKey` to the config endpoint (`HandleGetTenantFullConfig`), not to `OnboardResponse`/`HandleOnboard`.

**Fix:** Either:
- Remove `widget_key` from the onboard assertion (not needed — config test 2.2 covers it)
- Or add `WidgetKey` to `TenantInfo` struct + populate it in `HandleOnboard`

---

## V2 Rework Items — All Fixed

| V2 Issue | V3 Status |
|----------|-----------|
| Widget key not obtainable | ✅ Added to config response + PATCH request |
| Reindex async — RAG search race | ✅ Poll reindex status before search |
| Grant permission needs second user | ✅ Sign up viewer in Group 1 |
| Onboard request body undocumented | ✅ Curl invocation with form fields |
| LLM config PUT body undocumented | ✅ Curl invocation with JSON body |

## Minimum Viable — OK to Ship

- All 29 test scenarios mapped
- All endpoints verified against actual routes
- All documented request bodies match handler schemas (except bug #1 above)
- Orphaned tenant cleanup, re-run safety, health check — all covered
- Two code changes scoped and small

---

## Final Verdict

**Approved with Nits.** Fix the permissions JSON format (string→array) and drop the `widget_key` assertion from the onboard response. No structural rework needed.
