# `go test ./...` Failure Output — 2026-07-23

## Summary

- **17 failing tests** across 2 packages
- **Root cause A (16 tests):** Bridge middleware does not set tenant context
- **Root cause B (1 test):** Stale migration test assertion

---

## Package 1: `server/router/api/v1/agent` — 16 FAIL

All 16 failures share the same error: `"tenant context not set - middleware may not be configured correctly"`

### bridge_delivery_test.go (5 failures)

```
--- FAIL: TestBChatLiveHumanReplyAppearsInVisitorTranscript (0.11s)
    bridge_delivery_test.go:98:
        Error: code=400, message=tenant context not set

--- FAIL: TestBChatLiveDoesNotExposeClaimTokenToVisitor (0.11s)
    bridge_delivery_test.go:670:
        Error: code=400, message=tenant context not set

--- FAIL: TestBChatLiveBridgeReplyResponseAddsOnlyWebChatDeliveryTelemetry (0.09s)
    bridge_delivery_test.go:728:
        Error: expected 200, actual 400

--- FAIL: TestBChatLiveEndToEndVisitorHumanReplyFlow (0.09s)
    bridge_delivery_test.go:857:
        Error: code=400, message=tenant context not set

--- FAIL: TestBChatLiveTranscriptEndpointDoesNotReturnSessionIDOrInternalIDs (0.07s)
    bridge_delivery_test.go:1009:
        Error: code=400, message=tenant context not set
```

### bridge_endpoints_test.go (10 failures)

```
--- FAIL: TestBridgeTakeoverConcurrentSameSessionSingleActiveHandoff (0.15s)
    bridge_endpoints_test.go:340: Unexpected status code 400 (x10)
    bridge_endpoints_test.go:345: "0" is not greater than or equal to "1"

--- FAIL: TestBridgeReplyRejectsStaleHandoffID (0.09s)
    bridge_endpoints_test.go:386: expected 200, actual 400

--- FAIL: TestBridgeReleaseRejectsStaleHandoffID (0.09s)
    bridge_endpoints_test.go:472: expected 200, actual 400

--- FAIL: TestBridgeReleaseNoActiveHandoffSemantics (0.09s)
    bridge_endpoints_test.go:558: expected 404, actual 400

--- FAIL: TestBridgeReplySuccessPersisted (0.09s)
    bridge_endpoints_test.go:651: expected 200, actual 400

--- FAIL: TestBridgeReplyDuplicateMessageIDSameTextIdempotent (0.08s)
    bridge_endpoints_test.go:785: expected 200, actual 400

--- FAIL: TestBridgeReplyDuplicateMessageIDDifferentTextConflict (0.08s)
    bridge_endpoints_test.go:870: expected 200, actual 400

--- FAIL: TestBridgeReplyRejectsQueuedHandoff (0.10s)
    bridge_endpoints_test.go:945: expected 409, actual 400

--- FAIL: TestBridgeReplyRejectsClosedHandoff (0.09s)
    bridge_endpoints_test.go:978: expected 200, actual 400

--- FAIL: TestBridgeReplyRejectsNoActiveHandoffUnknownID (0.09s)
    bridge_endpoints_test.go:1043: expected 404, actual 400

--- FAIL: TestBridgeDeliveryDoesNotChangeReplyResponseShape (0.08s)
    bridge_endpoints_test.go:1249: expected 200, actual 400
```

### role_template_handler_test.go (1 failure)

```
--- FAIL: TestRoleTemplateEndpoints (0.04s)
    role_template_handler_test.go:83:
        Error: code=400, message=tenant context not set
    --- FAIL: TestRoleTemplateEndpoints/list_templates_includes_seeded_templates (0.00s)
        testing.go:1913: test executed panic(nil) or runtime.Goexit
```

---

## Package 2: `store/test` — 1 FAIL

```
--- FAIL: TestMigrationLoopSkipsAlreadyApplied (0.03s)
    migrator_test.go:123:
        Error: []string{"0.31.3", "0.34.1"} does not contain "0.33.2"
        Message: 0.33.x migrations should have been applied
```

---

## Passing Packages (for reference)

```
ok   internal/base              0.005s
ok   internal/bridgeworker      0.283s
ok   internal/util              0.005s
ok   internal/version           0.005s
ok   plugin/cron               41.472s
ok   plugin/httpgetter          0.006s
ok   plugin/idp/oauth2          0.009s
ok   plugin/webhook             0.008s
ok   server/router/api/v1       1.052s
ok   store/cache                0.476s
ok   store/db/mysql             0.014s
ok   store/db/postgres          0.016s
ok   store/db/sqlite            0.017s
```
