# Plan 13 — Adversarial Review

**Reviewed by:** Senior Go Architect
**Date:** 2026-07-28
**Verdict:** APPROVED WITH NITS — 0 critical, 0 significant, 2 minor

---

## Prior Finding Status

| Source | Finding | Status in plan13 |
|--------|---------|-----------------|
| plan12_review C1 | `readJSONL` uses `dec.More()` | ✅ Fixed (`io.EOF` loop) |
| plan12_review S1 | Aggregate dedup wrong subtraction | ✅ Fixed (removed) |
| plan12_review N1 | `readJSONL` silent on missing file | ✅ Fixed (logging added) |
| plan11_code_review B1 | `loadCompletedFromJSONL` same bug | ✅ Fix included (same fix) |

All findings properly addressed. The plan also fixes the pre-existing B1.

---

## Minor Findings

### Nit 1: `else if` pattern adds unnecessary nesting

**Location:** Lines 54-60, 104-110

```go
if err := dec.Decode(&r); err == io.EOF {
    break
} else if err != nil {
    continue
}
```

The `else if` is dead code in the `err == io.EOF` branch:

```go
err := dec.Decode(&r)
if err == io.EOF {
    break
}
if err != nil {
    continue
}
```

### Nit 2: Non-`NotExist` errors still silent in `readJSONL`

**Location:** Lines 95-99

```go
if err != nil {
    if os.IsNotExist(err) {
        t.Logf("no results file: %s (run per-type test first)", filepath.Base(path))
    }
    return results
}
```

Permission errors or removed-directory errors return empty results without logging:

```go
if err != nil {
    t.Logf("error reading %s: %v", filepath.Base(path), err)
    return results
}
```

---

## Everything Else: Correct

| Component | Verdict |
|-----------|---------|
| C1: `readJSONL` `io.EOF` loop | ✅ |
| B1: `loadCompletedFromJSONL` same fix | ✅ |
| S1: `uniqueTotal = len(allResults)` | ✅ |
| N1: missing file logging | ✅ |
| `"io"` import added | ✅ |
| `t *testing.T` on `readJSONL` | ✅ |
| All callsites updated | ✅ |
| Implementation order correct | ✅ |

---

**Approved — no rework needed.** Both nits are style and edge-case hardening. The plan is ~15 correct lines across 4 locations.
