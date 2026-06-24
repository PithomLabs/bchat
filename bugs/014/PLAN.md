# Plan: Chatbox Font Unification & Dark Mode Only

## Summary
Unify the font size and font family across all three chatbox surfaces (Agent Chat page, in-app AgentChatWidget, and the external embeddable widget), and enforce dark mode only across all three.

---

## Current State Analysis

### Font Sizes Currently Used

| Location | Input Font Size | Message Font Size | Role/Timestamp Font |
|----------|----------------|-------------------|---------------------|
| **Agent Chat page** (`web/src/pages/Chat.tsx`) | `text-sm` (14px) | `text-sm` (14px) | `text-[11px]` |
| **In-app AgentChatWidget** (`web/src/components/AgentChatWidget.tsx`) | `text-sm` (14px) | `text-sm` (14px) | `text-[11px]` |
| **External Widget** (`widget/src/styles/styles.ts`) | `14px` (explicit) | `14px` (explicit) | `12px` (role), `12px` (time) |

### Font Family Currently Used

| Location | Font |
|----------|------|
| **Agent Chat page** | `.chat-font` → Inter, system-ui, sans-serif (via `tailwind.css`) |
| **In-app AgentChatWidget** | `.chat-font` → same |
| **External Widget** | `'BChat Inter', Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif` (via `@font-face` in styles.ts) |

### Dark Mode Currently

| Location | Dark Mode Support |
|----------|-------------------|
| **Agent Chat page** | ✅ Full (uses `dark:` variants, root `<section>` has `dark` class) |
| **In-app AgentChatWidget** | ✅ Full (uses `dark:` variants) |
| **External Widget** | ❌ None (all colors are hardcoded light-mode values) |

---

## Plan

### Step 1: Enforce Dark Mode Only on Agent Chat Page

**File:** `web/src/pages/Chat.tsx`

- The `<section>` already has `dark` class — keep this.
- Ensure `tailwind.config.js` has `darkMode: ["class"]` (already set).
- The `index.html` `<body>` has `dark:bg-zinc-900` but no `dark` class by default. Since the Chat page self-applies `dark`, no global change needed.
- **No change needed** — Chat page is already dark-only via the `dark` class on its root.

### Step 2: Enforce Dark Mode Only on In-App AgentChatWidget

**File:** `web/src/components/AgentChatWidget.tsx`

- Add `dark` class to the outermost `<div>` (line 237-241).
- This ensures all `dark:` variant styles activate.
- Currently the widget relies on the parent's `dark` class from AgentAdmin, but when used standalone it should also be dark.

### Step 3: Convert External Widget to Dark Mode Only

**File:** `widget/src/styles/styles.ts`

Replace all light-mode-only colors with dark equivalents:

| Element | Current (Light) | New (Dark) |
|---------|----------------|------------|
| Panel background | `#ffffff` | `#18181b` (zinc-900) |
| Panel border | `#e7e3de` | `#27272a` (zinc-800) |
| Header background | `rgba(255,255,255,0.96)` | `rgba(24,24,27,0.96)` |
| Header text | `#292524` | `#e4e4e7` (zinc-200) |
| Header border | `#eeeae5` | `#27272a` |
| Content background | `#f7f5f2` | `#09090b` (zinc-950) |
| Input area background | `#ffffff` | `#18181b` |
| Input wrapper background | `#ffffff` | `#18181b` |
| Input wrapper border | `#ddd7d0` | `#3f3f46` (zinc-700) |
| Input text color | `#292524` | `#e4e4e7` |
| Input placeholder | `#a8a29e` | `#71717a` (zinc-500) |
| Send button (keep color) | `${color}` | `${color}` (unchanged) |
| User bubble bg | `rgba(color, 0.08)` | `rgba(color, 0.15)` |
| User bubble border | `rgba(color, 0.20)` | `rgba(color, 0.30)` |
| Assistant bubble bg | `#ffffff` | `#18181b` |
| Assistant bubble border | `#e7e3de` | `#27272a` |
| Message content text | `#292524` | `#e4e4e7` |
| Role label (assistant) | `#57534e` | `#a1a1aa` (zinc-400) |
| Timestamp | `#a8a29e` | `#71717a` (zinc-500) |
| Typing bubble bg | `#ffffff` | `#18181b` |
| Typing bubble border | `#e7e3de` | `#27272a` |
| Empty state text | `#65676b` | `#a1a1aa` |
| Header controls button bg | `#f7f5f2` | `#27272a` |
| Header controls button hover | `#efebe6` | `#3f3f46` |
| Header controls icon | `#78716c` | `#a1a1aa` |
| Scrollbar thumb | `rgba(0,0,0,0.15)` | `rgba(255,255,255,0.15)` |
| Error background | `#ffebe9` | `#450a0a` (red-950) |
| Error text | `#cf222e` | `#fca5a5` (red-300) |
| Error border | `#ffcecb` | `#7f1d1d` (red-900) |
| Box shadow (panel) | `rgba(41,37,36,0.18)` | `rgba(0,0,0,0.5)` |
| Focus ring | `rgba(color, 0.12)` | `rgba(color, 0.25)` |

**File:** `widget/src/iframe.html`

- Add `class="dark"` to `<html>` or `<body>` to ensure dark mode is active.
- Or add inline `color-scheme: dark` and dark background.

**File:** `widget/src/ui/Widget.ts`

- The widget injects styles via `<style>` tag into `<head>`. The styles are already dark-mode-only after Step 3 changes. No JS changes needed.

### Step 4: Unify Font Sizes

**Target font sizes (matching the current Inter-based 14px standard):**

| Element | Size | Tailwind Class (web) | CSS (widget) |
|---------|------|---------------------|--------------|
| Input text | 14px | `text-sm` | `font-size: 14px` |
| Message bubble text | 14px | `text-sm` | `font-size: 14px` |
| Role label | 12px | `text-xs` | `font-size: 12px` |
| Timestamp | 11px | `text-[11px]` | `font-size: 11px` |

**Changes needed:**

1. **Agent Chat page** (`Chat.tsx`):
   - Messages: already `text-sm` (line 186) ✅
   - Timestamp: already `text-[11px]` (line 197) ✅
   - Input: already `text-sm` (line 237) ✅
   - **No font size changes needed** — already matches target.

2. **In-app AgentChatWidget** (`AgentChatWidget.tsx`):
   - Messages: already `text-sm` (line 295) ✅
   - Timestamp: already `text-[11px]` (line 313) ✅
   - Input: already `text-sm` (line 356) ✅
   - **No font size changes needed** — already matches target.

3. **External Widget** (`widget/src/styles/styles.ts`):
   - Change `.acw-msg-role` from `font-size: 12px` → `font-size: 11px` (match other two)
   - Change `.acw-msg-time` from `font-size: 12px` → `font-size: 11px` (match other two)
   - Input: already `14px` ✅
   - Message content: already `14px` ✅

### Step 5: Unify Font Family

**Target:** All three surfaces use Inter with identical fallback stack.

**Current:**
- Web (both pages): `.chat-font` → `"Inter", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`
- Widget: `'BChat Inter', Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`

**Change in `widget/src/styles/styles.ts`:**
- Change the `@font-face` family from `'BChat Inter'` to `'Inter'` (or keep as `'BChat Inter'` — it's a local font-face name, doesn't matter as long as the family reference matches).
- Ensure the fallback stack matches the web version. Currently missing `ui-sans-serif` and `system-ui`. Add them for consistency.

**Final unified font stack:**
```
"Inter", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif
```

### Step 6: Update Widget Preview in AgentAdmin

**File:** `web/src/pages/AgentAdmin.tsx` (line ~1359)

- The preview already has `dark chat-font` classes — ✅ already correct.
- Font sizes in the preview are inline and match — verify they use the same sizes.

---

## Files to Modify

| File | Changes |
|------|---------|
| `web/src/components/AgentChatWidget.tsx` | Add `dark` class to root div |
| `widget/src/styles/styles.ts` | Dark mode colors + font size fix (12px→11px) + font stack alignment |
| `widget/src/iframe.html` | Add dark mode enforcement (e.g., `style="color-scheme: dark; background: #09090b"` on body) |

## Files Unchanged (already correct)

| File | Reason |
|------|--------|
| `web/src/pages/Chat.tsx` | Already dark-only with correct fonts |
| `web/src/css/tailwind.css` | Font-face and `.chat-font` already defined |
| `web/tailwind.config.js` | `darkMode: ["class"]` already set |
| `widget/src/ui/Widget.ts` | No changes — styles are injected from `styles.ts` |
| `widget/src/ui/Input.ts` | No changes — font sizes come from injected CSS |
| `widget/src/ui/Messages.ts` | No changes — font sizes come from injected CSS |
| `widget/src/ui/Panel.ts` | No changes — font sizes come from injected CSS |

---

## Implementation Order

1. **`widget/src/styles/styles.ts`** — Bulk of the work: dark colors + font size alignment
2. **`widget/src/iframe.html`** — Add dark background/color-scheme
3. **`web/src/components/AgentChatWidget.tsx`** — Add `dark` class
4. **Build widget** — `cd widget && npm run build` to verify
5. **Test all three surfaces** — Verify font consistency and dark mode

---

## Verification Checklist

- [ ] Agent Chat page: dark background, Inter font, 14px input/messages, 11px timestamps
- [ ] In-app AgentChatWidget: dark background, Inter font, 14px input/messages, 11px timestamps
- [ ] External Widget (standalone embed): dark background, Inter font, 14px input/messages, 11px timestamps
- [ ] All three surfaces visually consistent in font weight, sizing, and color scheme
- [ ] No light-mode fallback visible anywhere
- [ ] Widget preview in AgentAdmin reflects the changes
