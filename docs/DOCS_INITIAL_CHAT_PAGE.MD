# Proposal: AI Chat Agent Integration

## Overview

This document proposes adding an AI chat agent feature to the Memos application. The feature will add a chat icon to the existing sidebar navigation that, when clicked, opens a dedicated chat page where users can converse with an AI assistant.

---

## Architecture Summary

### Frontend Integration Point

The chat icon will be added to the existing **Navigation component** (`web/src/components/Navigation.tsx`). This vertical sidebar currently contains icons for Memos, Explore, Resources, Tickets, and Notifications. The new chat icon will follow the same pattern:

- Icon-only display on small screens with tooltip
- Icon + label display on medium and larger screens
- Consistent styling with existing navigation items (MUI Joy + Tailwind CSS)

### New Route

A new route `/chat` will be added to the React Router configuration (`web/src/router/index.tsx`) following the existing routing patterns. This route will render a dedicated Chat page component.

---

## Chat Page Design

### User Interface

The chat page will implement a standard AI chat interface typical of modern chat agents:

1. **Message Area** - Scrollable container displaying the conversation history
   - User messages aligned right
   - Assistant messages aligned left
   - Timestamps on messages
   - Support for markdown rendering (utilizing existing markdown capabilities in the codebase)

2. **Input Area** - Fixed at bottom of the page
   - Text input field for user messages
   - Send button
   - Support for multi-line input

3. **Header** - Optional header showing chat context or model information

### State Management

Conversation state will be managed using MobX, consistent with the existing application architecture:

- Message history array (role + content format)
- Loading/streaming state
- Error state

---

## Backend Integration

### Environment Variables

The chat feature will use two environment variables, consistent with the existing agent implementation:

| Variable | Purpose | Requirement |
|----------|---------|-------------|
| `OPENROUTER_API_KEY` | API key for OpenRouter service | Required |
| `LLM_MODEL` | Model identifier (e.g., `openai/gpt-4o-mini`) | Required |

### OpenRouter Library

The backend will use the **`github.com/revrost/go-openrouter`** library (same as the existing agent codebase). This library provides:

- Client creation with API key
- Chat completion requests
- Message formatting (system, user, assistant roles)
- Response parsing

### API Endpoint

A new gRPC service or HTTP endpoint will be added to handle chat requests:

- **Input**: User message + conversation history
- **Output**: Assistant response
- **Authentication**: Requires authenticated user session (existing auth system)

---

## Initial Conversation Flow

The implementation will focus on providing a clean initial conversation experience:

### Opening State

When a user first opens the chat page:

1. Empty message area with welcome prompt or placeholder
2. Input field ready for user's first message
3. No pre-loaded system messages visible to user

### First Message Exchange

1. User types and submits their message
2. Message appears in the chat area immediately
3. Loading indicator shows while waiting for response
4. Backend receives message and constructs API call with:
   - System prompt (defining assistant behavior)
   - User message
5. Response returned and displayed in chat area
6. Conversation history maintained for context in subsequent messages

### Message Format (Internal)

Messages will follow the OpenRouter/OpenAI message format:

- Role: `system` | `user` | `assistant`
- Content: Message text

The system prompt will be configured on the backend and not exposed to the user interface.

---

## Integration Points with Existing Codebase

| Component | Integration |
|-----------|-------------|
| Navigation | Add chat icon following existing nav item pattern |
| Router | Add `/chat` route to existing route configuration |
| gRPC Services | Add ChatService following existing service patterns |
| Theme | Use existing MUI Joy theme and Tailwind classes |
| State Management | Create chat store following MobX patterns in `store/v2/` |
| Authentication | Require authenticated user (existing auth flow) |
| i18n | Add chat-related translation keys to existing locale files |

---

## Scope Boundaries

### In Scope

- Chat icon in navigation sidebar
- Basic chat page with message display and input
- Single conversation session per page load
- Integration with OpenRouter via environment variables
- Use of existing UI component library

### Out of Scope (Future Enhancements)

- Conversation persistence/history across sessions
- Multiple conversation threads
- Streaming responses (token-by-token display)
- File/image attachments in chat
- Typing indicators
- Chat export functionality
- Advanced prompt engineering or tool use

---

## Dependencies

### Frontend

- Existing: React, MUI Joy, Tailwind CSS, MobX, React Router
- No new dependencies required

### Backend

- Existing: Go, gRPC, Echo
- New: `github.com/revrost/go-openrouter` v1.1.5

---

## Summary

This proposal outlines adding a chat icon to the sidebar navigation that opens a dedicated AI chat page. The implementation leverages the existing `github.com/revrost/go-openrouter` library pattern from the agent codebase, using `OPENROUTER_API_KEY` and `LLM_MODEL` environment variables for configuration. The focus is on providing a clean initial conversation experience with a standard chat interface, integrating seamlessly with the existing Memos application architecture.
