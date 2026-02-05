# Create Tenant Modal File Upload Bug

**Date:** 2026-02-05
**Status:** Identified
**Severity:** High
**Affected Component:** `HandleOnboard` endpoint

---

## Summary

When creating a new tenant via the Create Tenant modal, uploaded KB.MD files are silently ignored if no POLICY.MD file is also uploaded. The tenant is created successfully, but the KB file is not stored in the database. No error or warning is shown to the user.

**Expected behavior:** KB.MD is required, POLICY.MD is optional.
**Actual behavior:** Both files are required together, or both are silently skipped.

---

## Root Cause

### Location
`server/router/api/v1/agent/handlers.go` line 854

### Buggy Code

```go
// HandleOnboard - lines 849-856
for _, audienceType := range []string{"external", "internal"} {
    kbFile, err := c.FormFile(audienceType + "_kb_file")
    policyFile, err2 := c.FormFile(audienceType + "_policy_file")

    if err != nil || err2 != nil || kbFile == nil || policyFile == nil {
        continue // <-- BUG: Skips entire audience if policy is missing
    }
    // File processing never happens when only KB is provided
}
```

### The Problem

The condition `policyFile == nil` causes the loop to skip the entire audience type when only KB.MD is provided without POLICY.MD. Since POLICY.MD should be optional, this is incorrect behavior.

### The Irony

The downstream `importFiles()` function (line 1003) already handles optional policy correctly:

```go
// importFiles() - line 1003
if policyContent != "" {
    // Save policy file - only runs if policy content provided
}
```

The import logic is correct - it's just never reached because the gate condition is wrong.

---

## Impact Analysis

### All 4 Upload Buttons Affected

| Button | Bug Affected? | Expected Behavior | Actual Behavior |
|--------|---------------|-------------------|-----------------|
| External KB.MD | **YES** | Upload without policy | Silently ignored |
| External POLICY.MD | No* | Optional | Works when KB present |
| Internal KB.MD | **YES** | Upload without policy | Silently ignored |
| Internal POLICY.MD | No* | Optional | Works when KB present |

*Policy buttons work correctly when KB is also provided, but the file is ignored when KB is missing (which is arguably correct since KB is required).

### User Impact

1. User fills in required fields (tenant slug, company name)
2. User clicks "Choose KB.MD" and selects a file
3. User does NOT select a POLICY.MD (it's optional)
4. User clicks "Create" button
5. Tenant created successfully (toast shows success)
6. **KB.MD file is NOT stored** (silently ignored)
7. User has no indication the upload failed
8. Agent will not function properly without KB content

---

## Fix Required

### Backend Change

**File:** `server/router/api/v1/agent/handlers.go`
**Lines:** 849-892

#### Current Code (Buggy)

```go
for _, audienceType := range []string{"external", "internal"} {
    kbFile, err := c.FormFile(audienceType + "_kb_file")
    policyFile, err2 := c.FormFile(audienceType + "_policy_file")

    if err != nil || err2 != nil || kbFile == nil || policyFile == nil {
        continue
    }

    // Read KB file
    kbSrc, err := kbFile.Open()
    if err != nil {
        continue
    }
    kbContent, _ := io.ReadAll(kbSrc)
    kbSrc.Close()

    // Read Policy file
    policySrc, err := policyFile.Open()
    if err != nil {
        continue
    }
    policyContent, _ := io.ReadAll(policySrc)
    policySrc.Close()

    // Import files
    audienceInfo, err := h.importFiles(ctx, tenant.ID, audienceType, string(kbContent), string(policyContent))
    // ...
}
```

#### Fixed Code

```go
for _, audienceType := range []string{"external", "internal"} {
    kbFile, _ := c.FormFile(audienceType + "_kb_file")
    policyFile, _ := c.FormFile(audienceType + "_policy_file")

    // Skip if no KB file provided (KB is required for audience)
    if kbFile == nil {
        continue
    }

    // Read KB file (required)
    kbSrc, err := kbFile.Open()
    if err != nil {
        slog.Warn("failed to open KB file", "audience", audienceType, "error", err)
        continue
    }
    kbContent, err := io.ReadAll(kbSrc)
    kbSrc.Close()
    if err != nil {
        slog.Warn("failed to read KB file", "audience", audienceType, "error", err)
        continue
    }

    // Read Policy file (optional)
    var policyContent []byte
    if policyFile != nil {
        if policySrc, err := policyFile.Open(); err == nil {
            policyContent, _ = io.ReadAll(policySrc)
            policySrc.Close()
        }
    }

    // Import files - importFiles() already handles empty policy correctly
    audienceInfo, err := h.importFiles(ctx, tenant.ID, audienceType, string(kbContent), string(policyContent))
    // ...
}
```

### Key Changes

1. **Remove policyFile nil check** from the skip condition
2. **Only require KB file** to proceed with audience import
3. **Make policy reading optional** with nil check
4. **Add logging** for file read errors (optional improvement)

---

## Secondary Issues Found

### 1. Silent Error Handling

```go
// Lines 863, 871 - errors silently ignored
kbContent, _ := io.ReadAll(kbSrc)
policyContent, _ := io.ReadAll(policySrc)
```

**Recommendation:** Log warnings when file reads fail.

### 2. No User Feedback

Backend returns HTTP 200 OK with an empty `Audiences` map when no files are processed. The user sees a success toast but doesn't know files were skipped.

**Recommendation:** Include a warning in the response when expected files are missing or return an error if KB files were provided but not processed.

---

## Testing Verification

### Test Case 1: KB Only (The Bug Scenario)
1. Create tenant with only External KB.MD selected
2. Click Create
3. Verify KB stored in database:
   ```sql
   SELECT * FROM agent_source_files
   WHERE tenant_id = <id> AND file_type = 'kb';
   ```
4. Expected: 1 row with KB content

### Test Case 2: KB + Policy (Normal Case)
1. Create tenant with both External KB.MD and POLICY.MD
2. Click Create
3. Verify both files stored:
   ```sql
   SELECT * FROM agent_source_files WHERE tenant_id = <id>;
   ```
4. Expected: 2 rows (kb and policy)

### Test Case 3: Policy Only (Edge Case)
1. Create tenant with only External POLICY.MD (no KB)
2. Click Create
3. Expected: Tenant created, no files stored (KB is required)

### Test Case 4: No Files (Valid Case)
1. Create tenant with no files selected
2. Click Create
3. Expected: Tenant created successfully, can add files later via Agent Admin

---

## Files to Modify

| File | Description |
|------|-------------|
| `server/router/api/v1/agent/handlers.go` | Fix `HandleOnboard` file processing logic (lines 849-892) |

---

## Related Documentation

- `docs/DOCS_CHAT_DESIGN_4_IMP_2.MD` - Main implementation specification
- `docs/DOCS_AGENT_ARCHITECTURE.MD` - Configuration-driven architecture
