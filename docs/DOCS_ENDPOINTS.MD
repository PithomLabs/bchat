# Chat Endpoint Analysis

**Date:** 2026-01-28
**Updated:** 2026-01-28 (implemented recommendations)
**Purpose:** Document chat-related endpoints and their design

## Overview

The codebase has **2 chat API endpoints** that provide chat functionality. Both use the **internal** audience configuration (cleaned/production-ready content).

---

## API Endpoints

### 1. `/api/v1/agent/:slug/chat/ext` (External Chat)

| Attribute | Value |
|-----------|-------|
| **File** | `server/router/api/v1/agent/handlers.go:92` |
| **Handler** | `HandleChatExternal` |
| **Route Registration** | `v1.go:189` (public group, no auth) |
| **Authentication** | None (public endpoint) |
| **Tenant Support** | Yes - loads `internal` audience config (cleaned content) |
| **Session Management** | Memory-based (in-memory session store) |
| **RAG Pipeline** | Yes |
| **Rate Limiting** | Yes (tracked as "external" for rate limiting) |
| **Transcript Recording** | Yes (if enabled) |

**Service Method:** `service.ChatExternal()` - loads `internal` audience config

**Frontend Usage:**
- `widget/src/core/api.ts` (external embeddable widget)
- `web/src/components/AgentChatWidget.tsx`

---

### 2. `/api/v1/agent/:slug/chat/int` (Internal Chat)

| Attribute | Value |
|-----------|-------|
| **File** | `server/router/api/v1/agent/handlers.go:138` |
| **Handler** | `HandleChatInternal` |
| **Route Registration** | `v1.go:201` (auth group) |
| **Authentication** | Required (Memos auth + `chat:test` permission) |
| **Tenant Support** | Yes - loads `internal` audience config |
| **Session Management** | Database-based (persistent sessions) |
| **RAG Pipeline** | Yes |
| **Rate Limiting** | No (internal users trusted) |
| **Transcript Recording** | Yes |

**Service Method:** `service.ChatInternal()`

**Frontend Usage:**
- `web/src/pages/Chat.tsx` via `chat.ts` store
- `web/src/pages/InternalAgent.tsx` via `agentChatStore.ts` store

---

## Frontend Pages

### `/internal-agent` Route (InternalAgent.tsx)

| Attribute | Value |
|-----------|-------|
| **Component** | `web/src/pages/InternalAgent.tsx` |
| **Store** | `web/src/store/v2/agentChat.ts` |
| **API Endpoint** | `/api/v1/agent/:slug/chat/int` |
| **Icon** | `BotIcon` |
| **Nav Label** | "Internal Agent" |
| **Sidebar** | Yes (for users with `chat:test` permission) |

### `/chat` Route (Chat.tsx) - Hidden

| Attribute | Value |
|-----------|-------|
| **Component** | `web/src/pages/Chat.tsx` |
| **Store** | `web/src/store/v2/chat.ts` |
| **API Endpoint** | `/api/v1/agent/:slug/chat/int` |
| **Sidebar** | No (removed from navigation) |

---

## Comparison Matrix

| Feature | `/chat/ext` | `/chat/int` |
|---------|-------------|-------------|
| Auth Required | No | Yes |
| Tenant-aware | Yes | Yes |
| KB/Policy/Script | Yes (internal) | Yes (internal) |
| RAG Pipeline | Yes | Yes |
| Rate Limiting | Yes | No |
| Session Storage | Memory | Database |
| Transcript Recording | Yes | Yes |
| Frontend Usage | Widget | InternalAgent.tsx |

---

## Design Rationale

### Why Both Endpoints Use Internal Audience

The "audience" concept reflects content lifecycle, not user type:

| Audience | Purpose |
|----------|---------|
| `external` | Draft/staging - raw uploads needing preprocessing |
| `internal` | Published - cleaned, production-ready content |

Both customer-facing widget and admin testing should use cleaned content. The `external` audience is a staging area for content preparation.

### Rate Limiting

Even though `/chat/ext` loads internal audience config, rate limiting still tracks requests as "external" to protect against public API abuse.

---

## Changes Made (2026-01-28)

### Deleted

1. **`server/router/api/v1/chat_service.go`** - Legacy unused endpoint (117 lines)
2. **`s.RegisterChatRoutes(ticketGroup)`** - Removed from `v1.go`
3. **Chat sidebar icon** - Removed from `Navigation.tsx`

### Modified

1. **`service.go:ChatExternal()`** - Now loads `internal` audience instead of `external`
   - Customers get cleaned/production-ready content
   - Rate limiting still tracked as "external"

---

## Remaining Frontend Pages

| Route | Page | API | Purpose |
|-------|------|-----|---------|
| `/chat` | Chat.tsx | `/chat/int` | Hidden (no sidebar link) |
| `/internal-agent` | InternalAgent.tsx | `/chat/int` | Admin testing |

The `/chat` route still exists but is no longer in the navigation. It can be removed in a future cleanup if desired.
