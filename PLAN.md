# Plan: Apply View Transcript Style to All Chat Widgets

## Source of Truth
**View Transcript modal** in `web/src/pages/AgentAdmin.tsx` (lines 2269-2372)

---

## Style Extraction from View Transcript

### Container
- `maxWidth: 700px, width: 90vw, maxHeight: 90vh`
- Rounded corners via `rounded-lg` (8px)
- Messages area: `space-y-3` (12px gap between messages)

### Message Bubbles
| | Customer (user) | Agent (assistant) |
|---|---|---|
| **Background** | `bg-blue-50 dark:bg-blue-900/20` | `bg-gray-100 dark:bg-gray-800` |
| **Text color** | `text-gray-800 dark:text-gray-200` | `text-gray-800 dark:text-gray-200` |
| **Alignment** | `ml-auto` (right-aligned) | `mr-auto` (left-aligned) |
| **Indent** | `ml-8` (indented from right) | `mr-8` (indented from left) |
| **Padding** | `p-3` (12px all around) | `p-3` |
| **Rounded** | `rounded-lg` (8px) | `rounded-lg` |
| **Max width** | implicit ~80% | implicit ~80% |

### Role Label
- `text-xs font-medium`
- Customer: `text-blue-600`
- Agent: `text-gray-600`

### Timestamp
- `text-xs text-gray-400`
- Positioned on its own line below content

### Messages Container
- `space-y-3` (12px vertical gap between messages)
- No dot pattern background
- Plain background

### Input Area
- Uses Joy UI `Input` / `Textarea` components
- No visible border/pill wrapper — clean flat input
- Send button as separate `Button` component

---

## Files to Modify

### 1. `widget/src/styles/styles.ts` — External embeddable widget

**Panel:**
- Width: `380px` → `min(700px, 90vw)`
- Height: `560px` → `min(90vh, 600px)`
- Background: keep dark `#18181b`
- Border radius: `18px` → `12px`

**Messages container:**
- Gap: `padding: 22px 18px` → `padding: 16px`
- Add `display: flex; flex-direction: column; gap: 12px` (space-y-3 equivalent)
- Remove dot pattern background

**Message rows:**
- Remove `margin-bottom: 14px` → use gap from parent
- User messages: `justify-content: flex-end` (keep), add `margin-left: 32px` (ml-8)
- Agent messages: `justify-content: flex-start` (keep), add `margin-right: 32px` (mr-8)

**Bubbles:**
- User bg: `${hexToRgba(color, 0.15)}` → `#dbeafe` (blue-100 equivalent for dark: `#1e3a5f` → use `#172554` for dark bg)
- User text: `#e4e4e7` → keep (already correct for dark)
- Agent bg: `#18181b` → `#27272a` (gray-800 / zinc-800)
- Agent border: `#27272a` → `#3f3f46` (zinc-700)
- Remove border-right accent on user bubble
- Padding: `12px 14px` → `12px` (p-3)
- Border radius: `12px` → `8px` (rounded-lg)
- Font size: keep `14px` (text-sm equivalent)
- Max width: `87%` → `85%`

**Role label:**
- Customer role: `${shadeColor(color, -15)}` → `#60a5fa` (blue-400)
- Agent role: `#a1a1aa` → `#71717a` (zinc-500)
- Size: keep `11px` → change to `12px` (text-xs)

**Timestamp:**
- Color: `#71717a` → `#71717a` (same ✅)
- Size: `11px` → `12px` (text-xs)

**Input area:**
- Remove pill-style wrapper
- Background: `#18181b` → keep
- Border: remove visible border, use subtle bottom border approach
- Input: `font-size: 14px` → keep
- Padding: adjust to match transcript style

### 2. `web/src/components/AgentChatWidget.tsx` — In-app widget

**Panel width:**
- `max-w-[380px]` → `max-w-[700px] w-[90vw]`
- Height: `min(560px, calc(100dvh - 7rem))` → `min(90vh, 600px)`

**Messages container:**
- Remove `space-y-4` → `space-y-3`
- Background: `bg-[#f7f5f2] dark:bg-zinc-950` → `bg-transparent` (let panel bg show through)
- Remove `chat-bg-pattern`

**Bubbles:**
- User bg: `style={{ backgroundColor: primaryColor }}` → `bg-blue-100 dark:bg-blue-950/40`
- User text: `text-white` → `text-gray-800 dark:text-gray-200`
- User border: `border-transparent` → `border-blue-200 dark:border-blue-900`
- Agent bg: `bg-white dark:bg-zinc-900` → `bg-zinc-100 dark:bg-zinc-800`
- Agent border: `border-stone-100 dark:border-zinc-800/80` → `border-zinc-200 dark:border-zinc-700`
- Padding: `px-4 py-2.5` → `p-3`
- Rounded: `rounded-xl` → `rounded-lg`
- Max width: `max-w-[85%]` → keep
- Add `ml-8` to user, `mr-8` to agent

**Role label:**
- Customer: remove color override → `text-blue-600 dark:text-blue-400`
- Agent: `text-stone-400 dark:text-zinc-500` → `text-zinc-600 dark:text-zinc-400`
- Size: `text-[11px]` → `text-xs` (12px)

**Timestamp:**
- Customer: `text-white/70` → `text-blue-500/70` → use `text-blue-600/60 dark:text-blue-400/60`
- Agent: `text-stone-400 dark:text-zinc-500` → `text-zinc-500 dark:text-zinc-400`
- Size: `text-[11px]` → `text-xs` (12px)

**Input area:**
- Remove outer pill border wrapper styling
- Input: `text-stone-800 dark:text-stone-100` → `text-gray-800 dark:text-gray-200`
- Placeholder: `text-stone-400 dark:text-zinc-500` → `text-gray-400 dark:text-gray-500`
- Send button: keep primary color

### 3. `web/src/pages/Chat.tsx` — Agent Chat page

**Panel/section:**
- Already dark-only, keep `dark` class
- Width: `max-w-5xl` → `max-w-[700px]`
- Background: `bg-zinc-950` → keep

**Messages container:**
- `bg-[#f7f5f2] dark:bg-zinc-950 chat-bg-pattern` → `bg-transparent` (remove pattern)
- Padding: `p-4 sm:p-6` → `p-4`
- Gap: `gap-4` → `gap-3`

**Bubbles:**
- User bg: `bg-teal-600` → `bg-blue-100 dark:bg-blue-950/40`
- User text: `text-white` → `text-gray-800 dark:text-gray-200`
- User border: `border-transparent` → `border-blue-200 dark:border-blue-900`
- User shadow: `shadow-[0_2px_8px_rgba(13,148,136,0.12)]` → remove
- Agent bg: `bg-white dark:bg-zinc-900` → `bg-zinc-100 dark:bg-zinc-800`
- Agent border: `border-stone-200/80 dark:border-zinc-800` → `border-zinc-200 dark:border-zinc-700`
- Padding: `px-4 py-2.5` → `p-3`
- Rounded: `rounded-xl` → `rounded-lg`
- Max width: `max-w-[85%]` → keep
- Add `ml-8` to user messages, `mr-8` to agent messages

**Role label (inside bubble header):**
- Currently no explicit role label in Chat.tsx — need to add one
- Customer: `text-blue-600 dark:text-blue-400`, `text-xs font-medium`
- Agent: `text-zinc-600 dark:text-zinc-400`, `text-xs font-medium`

**Timestamp:**
- Customer: `text-teal-100/80` → `text-blue-600/60 dark:text-blue-400/60`
- Agent: `text-stone-400 dark:text-zinc-500` → `text-zinc-500 dark:text-zinc-400`
- Size: `text-[11px]` → `text-xs` (12px)

**Input area:**
- Remove outer pill border
- Input: `text-stone-900 dark:text-stone-100` → `text-gray-800 dark:text-gray-200`
- Placeholder: `text-stone-400 dark:text-zinc-500` → `text-gray-400 dark:text-gray-500`
- Send button: keep primary color

### 4. `web/src/pages/InternalAgent.tsx` — Internal Agent page

**Messages container:**
- `bg-white dark:bg-zinc-800` → `bg-transparent`
- Remove border/shadow from container
- Gap: `gap-4` → `gap-3`

**Bubbles:**
- User bg: `bg-teal-500 text-white` → `bg-blue-100 dark:bg-blue-950/40 text-gray-800 dark:text-gray-200`
- Agent bg: `bg-gray-100 dark:bg-zinc-700` → `bg-zinc-100 dark:bg-zinc-800`
- Padding: `p-3` → keep ✅
- Rounded: `rounded-lg` → keep ✅
- Max width: `max-w-[80%]` → `max-w-[85%]`
- Add `ml-8` to user, `mr-8` to agent

**Timestamp:**
- `text-xs opacity-60` → `text-xs text-zinc-500 dark:text-zinc-400`

**Input area:**
- Remove Joy UI Input/Textarea — use plain styled input matching other pages
- Or keep Joy UI but override styles to match

---

## Implementation Order

1. **`widget/src/styles/styles.ts`** — Most changes, pure CSS
2. **`web/src/components/AgentChatWidget.tsx`** — JSX className changes
3. **`web/src/pages/Chat.tsx`** — JSX className changes + add role labels
4. **`web/src/pages/InternalAgent.tsx`** — JSX className changes
5. **Build widget** — Verify compilation
6. **Test all 4 surfaces** — Visual comparison

---

## Verification Checklist

- [ ] All message bubbles: blue-tinted for Customer, gray for Agent
- [ ] All bubbles: `rounded-lg`, `p-3`, no colored borders
- [ ] All role labels: `text-xs font-medium`, blue for Customer, gray for Agent
- [ ] All timestamps: `text-xs`, muted gray
- [ ] User messages indented from right (`ml-8`), Agent from left (`mr-8`)
- [ ] Messages container: `space-y-3`, no dot pattern
- [ ] Panel width: `min(700px, 90vw)` on all surfaces
- [ ] Input area: clean, no pill border wrapper
- [ ] Font: Inter 14px text, 12px labels/timestamps across all surfaces
- [ ] Dark mode only everywhere
