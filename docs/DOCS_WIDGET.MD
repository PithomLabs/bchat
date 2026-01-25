# External Chat Widget

**Status:** Active
**Date:** 2026-01-25
**Related Documents:**
- [DOCS_README.MD](./DOCS_README.MD) - Project overview
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables

---

## Overview

The external chat widget allows you to embed an AI chat agent on any website. The widget supports multiple embedding approaches:

| Approach | Best For | Isolation |
|----------|----------|-----------|
| **JavaScript Embed** | General websites, full control | Low (shares DOM) |
| **iframe Embed** | CMS, WordPress, Shopify | High (isolated) |
| **Legacy Script** | Backward compatibility | Low (shares DOM) |

All approaches:
- Are **self-contained** - no external dependencies required
- **Auto-configure** based on tenant settings
- Provide a **floating chat button** (JS) or **full panel** (iframe)

---

## Quick Start

### Option 1: JavaScript Embed (Recommended)

```html
<script
  src="https://your-server.com/widget/your-tenant/embed.js"
  data-position="bottom-right"
  data-color="#0d9488"
></script>
```

### Option 2: iframe Embed

```html
<iframe
  src="https://your-server.com/widget/your-tenant/iframe?color=%230d9488"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat Widget"
></iframe>
```

### Option 3: Legacy Script (Backward Compatible)

```html
<script src="https://your-server.com/api/v1/agent/your-tenant/widget.js"></script>
```

---

## Integration Guide

### Step 1: Get Your Tenant Slug

1. Log into the Agent Admin page
2. Select your tenant
3. Copy the slug (e.g., `acme-corp`)

### Step 2: Choose Your Embedding Method

#### JavaScript Embed (Recommended)

Best for most websites. Provides a floating button that expands into a chat panel.

```html
<!DOCTYPE html>
<html>
<head>
  <title>My Website</title>
</head>
<body>
  <h1>Welcome to our site</h1>

  <!-- Chat widget with configuration -->
  <script
    src="https://your-server.com/widget/acme-corp/embed.js"
    data-position="bottom-right"
    data-color="#0d9488"
    data-welcome="How can we help you today?"
  ></script>
</body>
</html>
```

#### iframe Embed

Best for CMS platforms (WordPress, Shopify, Wix) where script injection is limited.

```html
<iframe
  src="https://your-server.com/widget/acme-corp/iframe?color=%230d9488&welcome=How%20can%20we%20help%3F"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat Widget"
></iframe>
```

#### Advanced: Global Configuration

For more control, configure via JavaScript before loading the widget:

```html
<script>
  window.AgentChatConfig = {
    baseUrl: 'https://your-server.com',
    tenant: 'acme-corp',
    color: '#0d9488',
    position: 'bottom-right',
    welcomeMessage: 'How can we help you today?',
    buttonSize: 56,
    panelWidth: 350,
    panelHeight: 500
  };
</script>
<script src="https://your-server.com/widget/acme-corp/embed.js"></script>
```

### Step 3: Test the Widget

1. Open your page in a browser
2. Click the chat button (bottom-right)
3. Send a test message
4. Verify the agent responds

---

## Configuration Options

### JavaScript Embed (data-* attributes)

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `data-position` | string | `bottom-right` | `bottom-right` or `bottom-left` |
| `data-color` | string | `#0d9488` | Primary color (hex) |
| `data-welcome` | string | `How can we help you today?` | Welcome message |
| `data-company-name` | string | (from tenant) | Company name in header |
| `data-button-size` | number | `56` | Toggle button size (px) |
| `data-panel-width` | number | `350` | Panel width (px) |
| `data-panel-height` | number | `500` | Panel height (px) |

### iframe Embed (query parameters)

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `color` | string | `#0d9488` | Primary color (URL-encoded hex) |
| `welcome` | string | `How can we help you today?` | Welcome message |
| `companyName` | string | (from tenant) | Company name in header |

**Note:** URL-encode special characters. `#0d9488` becomes `%230d9488`.

---

## API Reference

### GET /widget/:slug/embed.js

Returns the built widget JavaScript bundle with tenant configuration injected.

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `slug` | path | Tenant slug (e.g., `acme-corp`) |

**Response:**
- Content-Type: `application/javascript`
- Cache-Control: `public, max-age=3600` (1 hour)

**Example:**
```bash
curl https://your-server.com/widget/acme-corp/embed.js
```

---

### GET /widget/:slug/iframe

Returns a standalone HTML page for iframe embedding.

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `slug` | path | Tenant slug |
| `color` | query | Primary color (URL-encoded hex) |
| `welcome` | query | Welcome message |
| `companyName` | query | Company name override |

**Response:**
- Content-Type: `text/html; charset=utf-8`
- Cache-Control: `public, max-age=3600` (1 hour)

**Example:**
```bash
curl "https://your-server.com/widget/acme-corp/iframe?color=%230d9488"
```

---

### GET /api/v1/agent/:slug/widget.js (Legacy)

Returns the legacy inline-generated widget JavaScript.

**Note:** This endpoint is maintained for backward compatibility. New integrations should use `/widget/:slug/embed.js`.

---

### POST /api/v1/agent/:slug/chat/ext

Sends a chat message and receives a response. **No authentication required.**

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
| 429 | Rate limit exceeded |
| 500 | Internal server error |

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
| Welcome message | Customizable via config |

### Behavior

| Feature | Description |
|---------|-------------|
| Toggle | Click button to open/close panel |
| Minimize | Click − to collapse panel (keeps button visible) |
| Enter to send | Press Enter to send message |
| Typing indicator | Shows animated dots while waiting for response |
| Auto-scroll | Scrolls to latest message automatically |
| Session persistence | Maintains conversation via `session_id` |

### Styling

| Property | Value |
|----------|-------|
| Primary color | Configurable (default: #0d9488) |
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

### Using Data Attributes (Recommended)

```html
<script
  src="https://your-server.com/widget/acme-corp/embed.js"
  data-position="bottom-left"
  data-color="#7c3aed"
  data-welcome="Hi! What can I help you with?"
></script>
```

### CSS Overrides

You can override widget styles after the script loads:

```html
<script src=".../embed.js"></script>
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
| `.acw-msg` | Message wrapper |
| `.acw-msg-bubble` | Message bubble |
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

### iframe Security
- iframe embed is isolated from parent page
- No access to parent page DOM or cookies

---

## Building the Widget

The widget source code is in the `widget/` folder. To build:

```bash
# Build widget only
task build:widget

# Build everything including widget
task build:all

# Build with RAG support including widget
task build:rag:all
```

### Development

```bash
cd widget
npm install
npm run dev     # Start dev server on port 3002
npm run build   # Build for production
```

### Widget Architecture

```
widget/
├── src/
│   ├── core/           # Core logic
│   │   ├── types.ts    # TypeScript interfaces
│   │   ├── api.ts      # API client
│   │   └── state.ts    # State management
│   ├── ui/             # UI components
│   │   ├── Widget.ts   # Main orchestrator
│   │   ├── Button.ts   # Toggle button
│   │   ├── Panel.ts    # Chat panel
│   │   ├── Messages.ts # Message list
│   │   └── Input.ts    # Input area
│   ├── styles/
│   │   └── styles.ts   # Inline CSS
│   └── embed.ts        # Entry point
├── dist/
│   └── embed.min.js    # Built bundle (~15-20KB)
└── package.json
```

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
console.log(window.AgentChatWidget);
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

### iframe Not Loading

**Symptoms:** iframe shows blank or error.

**Solutions:**
1. Check browser console for errors
2. Ensure URL is correct and tenant is active
3. Check for Content Security Policy (CSP) restrictions on parent page

### Build Not Found

**Symptoms:** Fallback to legacy widget, console shows "widget embed.min.js not found".

**Solutions:**
1. Run `task build:widget` to build the widget
2. Ensure the server is running from the project root directory

---

## Examples

### Basic JavaScript Embed

```html
<!DOCTYPE html>
<html>
<head>
  <title>Customer Support</title>
</head>
<body>
  <h1>Contact Us</h1>
  <p>Click the chat button for instant support.</p>

  <script src="https://chat.example.com/widget/support/embed.js"></script>
</body>
</html>
```

### Customized JavaScript Embed

```html
<!DOCTYPE html>
<html>
<head>
  <title>My Store</title>
</head>
<body>
  <h1>Welcome to My Store</h1>

  <script
    src="https://chat.example.com/widget/mystore/embed.js"
    data-position="bottom-left"
    data-color="#7c3aed"
    data-welcome="Hi! Need help finding something?"
  ></script>
</body>
</html>
```

### iframe in WordPress

Add this to a Custom HTML block:

```html
<iframe
  src="https://chat.example.com/widget/mystore/iframe?color=%237c3aed"
  style="position:fixed;bottom:20px;right:20px;width:380px;height:550px;border:none;z-index:9999;border-radius:12px;box-shadow:0 8px 32px rgba(0,0,0,0.15);"
  title="Chat Support"
></iframe>
```

### React Integration

```jsx
import { useEffect } from 'react';

function ChatWidget({ tenantSlug, color = '#0d9488' }) {
  useEffect(() => {
    // Configure before loading
    window.AgentChatConfig = {
      color: color
    };

    const script = document.createElement('script');
    script.src = `https://chat.example.com/widget/${tenantSlug}/embed.js`;
    script.async = true;
    document.body.appendChild(script);

    return () => {
      // Cleanup on unmount
      const widget = document.getElementById('agent-chat-widget');
      if (widget) widget.remove();
      const styles = document.getElementById('acw-styles');
      if (styles) styles.remove();
      script.remove();
      delete window.AgentChatWidget;
    };
  }, [tenantSlug, color]);

  return null;
}

export default ChatWidget;
```

---

## Related Endpoints

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /widget/:slug/embed.js` | None | Built widget bundle |
| `GET /widget/:slug/iframe` | None | iframe HTML page |
| `GET /api/v1/agent/:slug/widget.js` | None | Legacy widget (inline JS) |
| `POST /api/v1/agent/:slug/chat/ext` | None | External chat API |
| `POST /api/v1/agent/:slug/chat/int` | Required | Internal chat API |

---

*Document Version: 2.0*
*Last Updated: 2026-01-25*
