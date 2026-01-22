# UI Fix: Translation Prefix Display Bug

> **Date:** 2026-01-21
> **Status:** Fixed

---

## Problem

Users reported seeing translation keys like `"agent-admin.title"` and `"agent-simulation.select-tenant"` displayed in the UI instead of the translated values `"Agent Admin"` and `"Select a Tenant"`.

**Affected Areas:**
- Agent Admin page
- Agent Simulation page
- Internal Agent page
- All chat agent related UI pages

---

## Root Cause

### Missing Translations in Non-English Locale Files

The `agent-admin`, `agent-simulation`, and `internal-agent` translation keys **only exist in `en.json`**. Other locale files (de.json, fr.json, etc.) don't have these keys.

When a user's browser is set to a non-English language:
1. i18next loads that language file (e.g., de.json)
2. Looks for key like `agent-simulation.title`
3. Key doesn't exist in de.json
4. Fallback to English wasn't working properly
5. i18next returns the key itself as fallback → user sees raw key

### Secondary Issue: Dynamic Import Wrapping

**File:** `web/src/i18n.ts`, line 53

When using dynamic `import()` for JSON files in Vite (and other modern bundlers), the imported content is wrapped in a module object with a `default` property:

```javascript
// What import() returns:
{
  default: {
    "common": {...},
    "agent-admin": {...},
    "agent-simulation": {...}
  }
}

// What i18next expects:
{
  "common": {...},
  "agent-admin": {...},
  "agent-simulation": {...}
}
```

### Why Keys Were Displayed

The `LazyImportPlugin` was passing the wrapped object directly to i18next:

```javascript
// Before (buggy)
callback(null, translation);  // translation = { default: {...} }
```

When i18next received `{ default: {...} }`, it couldn't find keys like `agent-admin.title` because the actual translations were nested one level deeper under `default`.

When i18next can't find a translation key, it returns the key itself as a fallback - hence users saw `"agent-admin.title"` instead of `"Agent Admin"`.

---

## The Fix

### 1. Preload English Translations as Fallback

Import English translations directly and bundle them so they're always available:

```javascript
// Import English translations directly
import enTranslations from "./locales/en.json";

// In i18n.init():
resources: {
  en: {
    translation: enTranslations,
  },
},
partialBundledLanguages: true,
```

### 2. Fix Dynamic Import Unwrapping

Handle the module wrapper from Vite's dynamic imports:

```javascript
// Before
callback(null, translation);

// After
callback(null, translation.default || translation);
```

### 3. Improve Fallback Configuration

Add proper fallback settings:

```javascript
returnEmptyString: false,
returnNull: false,
saveMissing: false,
```

---

## How i18n Works in This Codebase

### Translation File Structure

`web/src/locales/en.json`:
```json
{
  "common": {
    "agent-admin": "Agent Admin",
    "agent-simulation": "Agent Simulation"
  },
  "agent-admin": {
    "title": "Agent Admin",
    "select-tenant": "Select a Tenant",
    ...
  },
  "agent-simulation": {
    "title": "Agent Simulation",
    "select-tenant": "Select a Tenant",
    ...
  }
}
```

### How Keys Are Resolved

When you call `t("agent-admin.title")`:

1. i18next looks up the path in the JSON structure
2. `"agent-admin"` → finds the `agent-admin` object
3. `".title"` → finds the `title` key within that object
4. Returns the value: `"Agent Admin"`

The dot (`.`) is a **key separator**, not part of the displayed text.

### Namespace Organization

The translation file uses namespaces to:
- Prevent naming conflicts (both pages have a "title" key)
- Organize translations by feature
- Enable potential lazy-loading in the future

---

## Verification

After the fix:
1. Run `npm run dev` in the `web/` directory
2. Navigate to Agent Admin page
3. Verify "Agent Admin" displays (not "agent-admin.title")
4. Navigate to Agent Simulation page
5. Verify "Agent Simulation" and "Select a Tenant" display correctly

---

## Files Modified

| File | Change |
|------|--------|
| `web/src/i18n.ts` | Import English translations, fix dynamic import unwrapping, configure fallback |

---

## Technical Details

### Why This Happens with Vite

Vite (and webpack with certain configurations) treat JSON imports as ES modules. When you use `import()`, the JSON content is placed in the `default` export:

```javascript
// This is what actually happens internally
const module = {
  default: { /* JSON content */ }
};
```

This is consistent with ES module semantics where JSON modules expose their content as the default export.

### Static vs Dynamic Imports

Note that **static imports** work differently:

```javascript
// Static import - Vite handles this correctly
import translations from './locales/en.json';
// translations = { common: {...}, ... }

// Dynamic import - returns wrapped module
const module = await import('./locales/en.json');
// module = { default: { common: {...}, ... } }
// module.default = { common: {...}, ... }
```

The `LazyImportPlugin` uses dynamic imports for lazy-loading, which is why this fix was needed.

---

*End of Analysis*
