# Fix for Scraper Detail-URL Resolution Bug

## Problem Summary

The `scraper scrape` command fails to properly resolve detail URLs. The extraction engine follows `intent.json` instructions that specify `attribute: "innerText"` for fields marked as `IsDetailLink=true`, resulting in text content extraction instead of href URLs. This causes invalid URLs like `https://en.wikipedia.org/The Stranger` instead of valid paths like `/wiki/The_Stranger_(novel)`.

## Root Cause Analysis

The architectural flaw is a **semantic mismatch**: the extraction engine prioritizes literal intent attributes over the functional requirements of a detail-link field. When a field is marked as a detail link, its primary purpose is navigation, not data extraction. The current implementation violates this principle by allowing `innerText` to override URL extraction.

## Expert Architect Solution

### 1. Extraction Engine Hardening (schema.go)

**Refactor `extractStructural`**:
- Add semantic override: If `spec.IsDetailLink` is true and the matched element is an `<a>` tag, extract `href` directly regardless of `spec.Attribute`
- Implementation: `val, ok := sel.Attr("href")` when `IsDetailLink && sel.Is("a")`

**Refactor `extractSemantic`**:
- After `findValByLabel` returns a result for a detail-link, validate if the value "looks like" a URL
- If it contains spaces (signature of `innerText` extraction), re-extract the `href` from the same matched node
- This creates a "link-first invariant" that ensures functional navigation

### 2. URL Resolution Resilience (types.go)

**Harden `ResolveDetailURL`**:
- Add sanity check: if `rawURL` contains spaces or doesn't start with valid path indicator (`/`, `./`, `../`, `http`), treat as suspicious
- Return the value as-is or empty string instead of prepending base domain
- This prevents creating invalid URLs from text content

## Implementation Plan

### Phase 1: Schema.go Modifications

```go
// In extractStructural
if spec.IsDetailLink && strings.ToLower(spec.Attribute) != "href" {
    // Check if we can get href from anchor tag
    if anchor := sel.Find("a"); anchor.Length() > 0 {
        if href, ok := anchor.Attr("href"); ok && href != "" {
            return href, nil
        }
    }
}

// In extractSemantic - add fallback logic
if spec.IsDetailLink {
    // After getting result from findValByLabel
    if !isURL(result) {
        // Try to find anchor within the same scope
        if anchor := scope.Find("a"); anchor.Length() > 0 {
            if href, ok := anchor.Attr("href"); ok {
                return href, nil
            }
        }
    }
}
```

### Phase 2: Types.go Modifications

```go
// In ResolveDetailURL
if strings.Contains(rawURL, " ") || !isValidPath(rawURL) {
    // Don't prepend base domain to malformed paths
    return rawURL, nil
}
```

## Testing Strategy (4-Stage Protocol)

### Stage 1: Discover
- Run `scraper discover` on the Wikipedia page
- Verify that link detection correctly identifies detail links
- Expected: Links to book pages are marked with appropriate intent

### Stage 2: Intent Validation
- Run `scraper intent --file intent.json --harden`
- Verify that the generated model maintains link semantics
- Expected: No regression in intent generation

### Stage 3: Replay
- Run `scraper replay` on the LeMonde2 dataset
- Verify that existing bindings still work correctly
- Expected: All rows process without errors, existing data remains consistent

### Stage 4: Scrape (Live Test)
- Run `scraper scrape` on LeMonde2
- **Verification Points**:
  1. Log check: `[DEBUG] [MERGE] Resolved detail URL for row 1:` should show a valid `/wiki/` path
  2. Output CSV: Row 1 must have all detail fields populated (author, translator, etc.)
  3. URL validation: No spaces in resolved URLs

## Validation Criteria

**Success Metrics**:
- ✅ Valid Wikipedia paths in logs (e.g., `/wiki/The_Stranger_(novel)`)
- ✅ All detail fields populated in output CSV
- ✅ No regressions in replay functionality
- ✅ Code compiles without errors
- ✅ Existing tests pass

**Edge Cases to Consider**:
- Multiple `<a>` tags within the matched scope
- Relative vs absolute URLs
- Malformed HTML structures
- Internationalized domain names

## Architectural Benefits

This fix achieves the desired balance:
- **General Resilience**: The "link-first invariant" protects against misconfigured intents across all tenant configurations
- **Pragmatic Output**: Maintains exact functional requirements for deep scraping while allowing flexibility in data extraction
- **Non-invasive**: Changes are localized to extraction and resolution logic without affecting other components

## Open Questions

None - the path resolution error is unambiguous from user logs.

## Implementation Priority

High - this bug prevents the core `scrape` functionality from working correctly. The fix is localized and follows existing patterns in the codebase.