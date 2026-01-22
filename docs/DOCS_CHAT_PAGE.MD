# AI Chat Feature Implementation

## Reference Document

See [DOCS_INITIAL_CHAT_PAGE.MD](./DOCS_INITIAL_CHAT_PAGE.MD) for the original proposal and design overview.

---

## Summary

This document details the implementation of an AI chat feature for the Memos application. The feature adds a chat icon to the sidebar navigation that opens a dedicated chat page where users can converse with an AI assistant via the OpenRouter API.

---

## Files Modified

### Backend

| File | Change |
|------|--------|
| `internal/profile/profile.go` | Added `OpenRouterAPIKey` and `LLMModel` fields to Profile struct |
| `bin/memos/main.go` | Added CLI flags and environment variable bindings for `OPENROUTER_API_KEY` and `LLM_MODEL` |
| `server/router/api/v1/v1.go` | Registered chat routes with `s.RegisterChatRoutes(ticketGroup)` |
| `server/router/api/v1/chat_service.go` | **NEW** - REST endpoint for chat at POST `/api/v1/chat` |
| `go.mod` | Added `github.com/revrost/go-openrouter v1.1.5` dependency |

### Frontend

| File | Change |
|------|--------|
| `web/src/router/index.tsx` | Added `CHAT = "/chat"` to Routes enum, lazy import for Chat page, route configuration |
| `web/src/components/Navigation.tsx` | Added `MessageSquareIcon` import, `chatNavLink` definition, added to navLinks array |
| `web/src/store/v2/chat.ts` | **NEW** - MobX store for chat state management |
| `web/src/store/v2/index.ts` | Exported `chatStore` |
| `web/src/pages/Chat.tsx` | **NEW** - Chat page component with message UI |
| `web/src/locales/en.json` | Added `"chat"` key to common section and new `"chat"` section for UI strings |

---

## New Files Created

### `server/router/api/v1/chat_service.go`

REST endpoint implementation:
- `RegisterChatRoutes()` - Registers POST `/api/v1/chat` route
- `HandleChat()` - Handles incoming chat requests, validates auth, calls OpenRouter
- `callOpenRouter()` - Makes API call to OpenRouter with conversation history

### `web/src/store/v2/chat.ts`

MobX store with:
- `LocalState` class with `messages`, `isLoading`, `error` state
- `sendMessage(content)` - Sends message to API and updates state
- `clearChat()` - Resets conversation

### `web/src/pages/Chat.tsx`

React component with:
- Message display area with user/assistant message styling
- Input textarea with send button
- Clear chat functionality
- Auto-scroll to latest message
- Loading and error states

---

## Challenges Encountered and Fixes

### 1. Missing go.mod File

**Problem:** The project directory did not have a `go.mod` file initially.

**Fix:** User copied the `go.mod` file from the original memos project to `/home/chaschel/Documents/ibm/ai/chat/go.mod`.

---

### 2. Missing go.sum File

**Problem:** After copying `go.mod`, running `task build:backend` failed with "missing go.sum entry" errors for all dependencies.

**Fix:** Ran `go mod download` to download all dependencies and generate the `go.sum` file.

---

### 3. Missing Frontend dist Folder

**Problem:** Backend build failed with error:
```
server/router/frontend/frontend.go:19:12: pattern dist/*: no matching files found
```

The Go embed directive requires the `dist` folder to exist.

**Fix:** Created empty dist folder as workaround:
```bash
mkdir -p server/router/frontend/dist
touch server/router/frontend/dist/.gitkeep
```

---

### 4. OpenRouter Response Type Mismatch

**Problem:** Build failed with:
```
server/router/api/v1/chat_service.go:116:9: cannot use resp.Choices[0].Message.Content
(variable of struct type openrouter.Content) as string value in return statement
```

The `go-openrouter` library returns `Content` as a struct, not a string.

**Fix:** Changed the return statement in `chat_service.go`:
```go
// Before
return resp.Choices[0].Message.Content, nil

// After
return resp.Choices[0].Message.Content.Text, nil
```

---

### 5. Database Migration Conflict

**Problem:** Running the memos binary failed with:
```
SQL logic error: table tickets already exists (1)
```

The database was at version 0.24.2, but the `tickets` table already existed from a previous session. The migration script for version 0.25 was trying to create it again.

**Fix:** Updated the migration version in the database to skip already-applied migrations:
```bash
sqlite3 /home/chaschel/Documents/ibm/ai/chat/build/data/memos_dev.db \
  "UPDATE migration_history SET version = '0.25.5';"
```

---

### 6. Missing Frontend Dependencies

**Problem:** `task build:frontend` failed with:
```
Error: Cannot find module '@vitejs/plugin-legacy'
```

**Fix:** Install npm dependencies:
```bash
npm --prefix web install
```

---

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `OPENROUTER_API_KEY` | API key for OpenRouter service | Yes |
| `LLM_MODEL` | Model identifier (e.g., `openai/gpt-4o-mini`) | No (defaults to `openai/gpt-4o-mini`) |

---

## Build Commands

```bash
# Install dependencies
task setup

# Build frontend
task build:frontend

# Build backend
task build:backend

# Build everything
task build

# Run in development mode
./build/memos --mode dev --driver sqlite --data ./build/data
```

---

## API Endpoint

### POST `/api/v1/chat`

**Authentication:** Required (Bearer token or session cookie)

**Request Body:**
```json
{
  "messages": [
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi! How can I help?"},
    {"role": "user", "content": "What is this app?"}
  ]
}
```

**Response:**
```json
{
  "message": {
    "role": "assistant",
    "content": "This is the Memos application, a self-hosted note-taking service..."
  }
}
```

---

## Database Location

- Development database: `/home/chaschel/Documents/ibm/ai/chat/build/data/memos_dev.db`
- Binary location: `/home/chaschel/Documents/ibm/ai/chat/build/memos`

---

## Translation Keys Added

**`web/src/locales/en.json`:**

```json
{
  "common": {
    "chat": "Chat"
  },
  "chat": {
    "empty-conversation": "Start a conversation with the AI assistant",
    "loading": "Thinking...",
    "placeholder": "Type your message...",
    "title": "AI Chat"
  }
}
```
