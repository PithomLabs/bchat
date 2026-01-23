# External Chat Widget

**Status:** Active
**Date:** 2026-01-23
**Related Documents:**
- [DOCS_README.MD](./DOCS_README.MD) - Project overview
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables

---

## Overview

The external chat widget allows you to embed an AI chat agent on any website using a single JavaScript include. The widget:

- Uses a **JavaScript embed approach** (not iframe)
- Is **self-contained** - no external dependencies required
- **Auto-configures** based on tenant settings
- Provides a **floating chat button** in the bottom-right corner

---

## Quick Start

Add this single line to your website's HTML, replacing `your-tenant` with your tenant slug:

```html
<script src="https://your-server.com/api/v1/agent/your-tenant/widget.js"></script>
```

That's it! A chat button will appear in the bottom-right corner of your page.

---

## Integration Guide

### Step 1: Get Your Tenant Slug

1. Log into the Agent Admin page
2. Select your tenant
3. Copy the slug (e.g., `acme-corp`)

### Step 2: Add the Widget Script

Add the script tag before the closing `</body>` tag:

```html
<!DOCTYPE html>
<html>
<head>
  <title>My Website</title>
</head>
<body>
  <!-- Your website content -->
  <h1>Welcome to our site</h1>

  <!-- Add the chat widget -->
  <script src="https://your-server.com/api/v1/agent/acme-corp/widget.js"></script>
</body>
</html>
```

### Step 3: Test the Widget

1. Open your page in a browser
2. Click the chat button (bottom-right)
3. Send a test message
4. Verify the agent responds

---

## API Reference

### GET /api/v1/agent/:slug/widget.js

Returns the embeddable widget JavaScript.

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `slug` | path | Tenant slug (e.g., `acme-corp`) |

**Response:**
- Content-Type: `application/javascript`
- Cache-Control: `public, max-age=3600` (1 hour)

**Example:**
```bash
curl https://your-server.com/api/v1/agent/acme-corp/widget.js
```

---

### POST /api/v1/agent/:slug/chat/ext

Sends a chat message and receives a response. **No authentication required.**

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `slug` | path | Tenant slug |

**Request Body:**
```json
{
  "session_id": "optional-existing-session-id",
  "message": "Hello, I need help"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | No | Existing session ID to continue conversation |
| `message` | string | Yes | User's message |

**Response (200 OK):**
```json
{
  "session_id": "abc123",
  "message": {
    "role": "assistant",
    "content": "Hello! How can I help you today?"
  },
  "metadata": {
    "intent": "greeting",
    "confidence": 0.95
  }
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 400 | Invalid request body or missing message |
| 404 | Tenant not found or inactive |
| 429 | Rate limit exceeded (see below) |
| 500 | Internal server error |

**Rate Limit Response (429):**
```json
{
  "error": "rate_limit_exceeded",
  "message": "Too many requests. Please try again in 60 seconds.",
  "retry_after": 60
}
```

**Example:**
```bash
curl -X POST https://your-server.com/api/v1/agent/acme-corp/chat/ext \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello"}'
```

---

## Widget Features

### User Interface

| Feature | Description |
|---------|-------------|
| Floating button | Circular chat icon, bottom-right corner |
| Chat panel | 350x500px expandable panel |
| Header controls | Minimize (−) and close (×) buttons |
| Message history | Auto-scrolling message list |
| Input area | Text input with send button |
| Welcome message | "How can we help you today?" |

### Behavior

| Feature | Description |
|---------|-------------|
| Toggle | Click button to open/close panel |
| Minimize | Click − to collapse panel (keeps button visible) |
| Enter to send | Press Enter to send message |
| Typing indicator | Shows "Typing..." while waiting for response |
| Auto-scroll | Scrolls to latest message automatically |
| Session persistence | Maintains conversation via `session_id` |

### Styling

| Property | Value |
|----------|-------|
| Primary color | Teal (#0d9488) |
| Font | System UI (system-ui, -apple-system, sans-serif) |
| Font size | 14px |
| Border radius | 12px (panel), 50% (button) |
| Shadow | Box shadow for depth |

### Mobile Responsive

On screens under 480px:
- Panel width adjusts to nearly full screen
- Bottom/right margins reduced
- Height set to 60vh

---

## Customization

### Currently Available

| Option | Source | Notes |
|--------|--------|-------|
| Company name | Tenant config | Displayed in header (if set) |
| Primary color | Hardcoded | #0d9488 (teal) |

### CSS Overrides

You can override widget styles after the script loads:

```html
<script src=".../widget.js"></script>
<style>
  /* Move to bottom-left */
  #acw-toggle, #acw-panel {
    right: auto !important;
    left: 20px !important;
  }

  /* Change colors */
  #acw-toggle, #acw-header, .acw-msg-user, #acw-send {
    background: #4f46e5 !important; /* Indigo */
  }

  /* Larger panel */
  #acw-panel {
    width: 400px !important;
    height: 600px !important;
  }
</style>
```

### Widget CSS Classes

| Selector | Element |
|----------|---------|
| `#agent-chat-widget` | Root container |
| `#acw-toggle` | Floating button |
| `#acw-panel` | Chat panel |
| `#acw-header` | Panel header |
| `#acw-content` | Content area |
| `#acw-messages` | Messages container |
| `.acw-msg` | Message bubble |
| `.acw-msg-user` | User message |
| `.acw-msg-assistant` | Agent message |
| `.acw-typing` | Typing indicator |
| `#acw-input-area` | Input container |
| `#acw-input` | Text input |
| `#acw-send` | Send button |

---

## Security Considerations

### Authentication
- The widget endpoint (`/chat/ext`) requires **no authentication**
- Designed for anonymous public users
- Internal chat (`/chat/int`) requires authentication

### Rate Limiting
- IP-based rate limiting prevents abuse
- Returns HTTP 429 when exceeded
- Default: 60-second cooldown

### Session Management
- Sessions stored in **memory only**
- Sessions are **lost on server restart**
- No persistent conversation history for external users

### XSS Protection
- All user input is HTML-escaped before display
- Uses `textContent` for safe string insertion

### CORS
- Server handles CORS headers automatically
- Widget script works cross-origin

---

## Limitations

| Limitation | Description |
|------------|-------------|
| No SSE streaming | Uses request/response, not real-time streaming |
| No file uploads | Text messages only |
| No history persistence | Sessions lost on server restart |
| Limited customization | Primary color is hardcoded |
| No typing detection | No "user is typing" indicator to agent |
| No offline support | Requires active server connection |
| No multi-language | Widget UI is English only |

---

## Troubleshooting

### Widget Not Appearing

**Symptoms:** No chat button visible after adding script.

**Solutions:**
1. Check browser console for JavaScript errors
2. Verify the tenant slug is correct
3. Confirm the tenant is active
4. Check if another element has `z-index: 9999+` blocking it

```javascript
// Debug: Check if widget loaded
console.log(document.getElementById('agent-chat-widget'));
```

### Rate Limit Errors

**Symptoms:** "Too many messages" error after several messages.

**Solutions:**
1. Wait 60 seconds before sending more messages
2. Contact admin to adjust rate limits if needed

### CORS Errors

**Symptoms:** Console shows "Access-Control-Allow-Origin" errors.

**Solutions:**
1. Ensure server is configured for CORS
2. Check if using correct server URL (http vs https)
3. Verify tenant exists and is active

### Messages Not Sending

**Symptoms:** Click send but nothing happens.

**Solutions:**
1. Check network tab for failed requests
2. Verify server is running
3. Check for JavaScript errors in console
4. Ensure message is not empty

### Styling Issues

**Symptoms:** Widget looks broken or unstyled.

**Solutions:**
1. Check for CSS conflicts with your site
2. Ensure no `* { }` reset styles affecting widget
3. Try loading widget on a blank page to isolate issue

---

## Examples

### Basic Integration

```html
<!DOCTYPE html>
<html>
<head>
  <title>Customer Support</title>
</head>
<body>
  <h1>Contact Us</h1>
  <p>Click the chat button for instant support.</p>

  <script src="https://chat.example.com/api/v1/agent/support/widget.js"></script>
</body>
</html>
```

### With Custom Styling

```html
<!DOCTYPE html>
<html>
<head>
  <title>My Store</title>
  <style>
    /* Purple theme */
    #acw-toggle, #acw-header, .acw-msg-user, #acw-send {
      background: #7c3aed !important;
    }
    #acw-input:focus {
      border-color: #7c3aed !important;
    }
  </style>
</head>
<body>
  <h1>Welcome to My Store</h1>

  <script src="https://chat.example.com/api/v1/agent/mystore/widget.js"></script>
</body>
</html>
```

### React Integration

```jsx
import { useEffect } from 'react';

function ChatWidget({ tenantSlug }) {
  useEffect(() => {
    const script = document.createElement('script');
    script.src = `https://chat.example.com/api/v1/agent/${tenantSlug}/widget.js`;
    script.async = true;
    document.body.appendChild(script);

    return () => {
      // Cleanup on unmount
      const widget = document.getElementById('agent-chat-widget');
      if (widget) widget.remove();
      script.remove();
    };
  }, [tenantSlug]);

  return null;
}

export default ChatWidget;
```

---

## Related Endpoints

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /:slug/widget.js` | None | Widget JavaScript |
| `POST /:slug/chat/ext` | None | External chat |
| `POST /:slug/chat/int` | Required | Internal chat |
| `GET /:slug/config` | Required | Tenant configuration |

---

*Document Version: 1.0*
*Last Updated: 2026-01-23*
