# Testing the Chat Widget Locally

**Status:** Active
**Date:** 2026-01-25
**Related Documents:**
- [DOCS_WIDGET.MD](./DOCS_WIDGET.MD) - Widget integration guide

---

## Overview

This document explains how to test the embeddable chat widget against a local development server.

---

## Prerequisites

1. **Build the widget** (if not already built):
   ```bash
   task build:widget
   ```

2. **Ensure a tenant exists** in your local database with the slug `test-tenant` (or modify the test page to use an existing tenant).

---

## Quick Start

### Step 1: Start the Server

```bash
# With RAG support (OpenRouter embeddings)
task run:rag

# Or with mock embeddings (no API key needed)
task run:rag:mock

# Or without RAG
task dev
```

The server runs on `http://localhost:8081` by default.

### Step 2: Open the Test Site

Open `widget/site/index.html` directly in your browser:

```bash
# Using your file manager or:
open widget/site/index.html        # macOS
xdg-open widget/site/index.html    # Linux
start widget/site/index.html       # Windows
```

Or serve it with a simple HTTP server:

```bash
# Using Python
cd widget/site && python -m http.server 8000

# Then open http://localhost:8000
```

### Step 3: Test the Widget

1. Click the chat button in the bottom-right corner
2. Send a test message (e.g., "Hello")
3. Verify the agent responds
4. Check browser console (F12) for any errors

---

## Test Pages

### JavaScript Embed Test (`index.html`)

Tests the primary JavaScript embed approach.

**URL:** `file:///path/to/widget/site/index.html`

**Features tested:**
- Widget loads and displays toggle button
- Opens/closes on button click
- Sends messages to `/api/v1/agent/:slug/chat/ext`
- Displays responses correctly
- Typing indicator appears during loading
- Rate limiting (send many messages quickly)

### iframe Embed Test (`iframe-test.html`)

Tests the iframe embedding approach.

**URL:** `file:///path/to/widget/site/iframe-test.html`

**Features tested:**
- Widget loads inside iframe
- Auto-opens in full-panel mode
- Cross-origin communication works
- Styling is isolated from parent page

---

## Configuration

The test pages are configured to connect to:

| Setting | Value |
|---------|-------|
| Server URL | `http://localhost:8081` |
| Tenant Slug | `test-tenant` |
| Position | `bottom-right` |
| Primary Color | `#0d9488` (teal) |

### Changing the Tenant

Edit `widget/site/index.html` and modify:

```javascript
window.AgentChatConfig = {
  baseUrl: 'http://localhost:8081',
  tenant: 'your-tenant-slug',  // Change this
  // ...
};
```

Also update the script src:

```html
<script src="http://localhost:8081/widget/your-tenant-slug/embed.js"></script>
```

### Changing the Server URL

If your server runs on a different port:

```javascript
window.AgentChatConfig = {
  baseUrl: 'http://localhost:YOUR_PORT',
  // ...
};
```

---

## Creating a Test Tenant

If you don't have a tenant, create one via the API or UI:

### Via API (curl)

```bash
# Login first to get a token (use your admin credentials)
TOKEN="your-access-token"

# Create tenant
curl -X POST http://localhost:8081/api/v1/agent/onboard \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "slug": "test-tenant",
    "company_name": "Test Company",
    "industry": "technology"
  }'
```

### Via Agent Admin UI

1. Login to the application
2. Navigate to Agent Admin
3. Click "Create Tenant"
4. Fill in the details with slug `test-tenant`

---

## Troubleshooting

### Widget Not Appearing

**Check server is running:**
```bash
curl http://localhost:8081/healthz
```

**Check tenant exists:**
```bash
curl http://localhost:8081/api/v1/agent/test-tenant/widget.js
# Should return JavaScript (not 404)
```

**Check browser console:**
- Open Developer Tools (F12)
- Look for errors in Console tab
- Look for failed requests in Network tab

### CORS Errors

If testing from `file://` protocol and seeing CORS errors:

1. Use a local HTTP server instead:
   ```bash
   cd widget/site && python -m http.server 8000
   ```

2. Or add CORS headers to your browser (dev only):
   - Chrome: Use "Allow CORS" extension
   - Firefox: Use "CORS Everywhere" extension

### "Tenant not found" Error

1. Verify the tenant slug matches exactly
2. Ensure the tenant is active (not deactivated)
3. Check the server logs for more details

### Widget Shows But Messages Fail

1. Check Network tab for failed API calls
2. Verify the chat endpoint is accessible:
   ```bash
   curl -X POST http://localhost:8081/api/v1/agent/test-tenant/chat/ext \
     -H "Content-Type: application/json" \
     -d '{"message": "Hello"}'
   ```
3. Check server logs for errors

### Build Not Found (Fallback to Legacy)

If the server shows "widget embed.min.js not found":

```bash
# Build the widget
task build:widget

# Verify the build exists
ls -la widget/dist/embed.min.js
```

---

## Development Workflow

### Making Widget Changes

1. Edit files in `widget/src/`
2. Rebuild:
   ```bash
   cd widget && npm run build
   ```
3. Refresh the test page (Ctrl+R / Cmd+R)

### Using Vite Dev Server

For hot-reload during widget development:

```bash
cd widget
npm run dev
```

This starts Vite on port 3002 with the `src/test.html` page. Note: The Vite dev server proxies API calls to `localhost:8081`.

---

## Test Checklist

| Test | Expected Result |
|------|-----------------|
| Page loads | Widget button visible in bottom-right |
| Click toggle button | Chat panel opens |
| Click close (X) | Panel closes, button reappears |
| Click minimize (-) | Panel collapses to header only |
| Send message | User message appears, typing indicator shows |
| Receive response | Assistant message appears |
| Send 10+ messages quickly | Rate limit error appears |
| Mobile viewport (380px) | Widget adapts to smaller size |
| Refresh page | Widget resets (new session) |

---

## Files

| File | Purpose |
|------|---------|
| `widget/site/index.html` | Main test page (JS embed) |
| `widget/site/iframe-test.html` | iframe embed test page |
| `widget/src/test.html` | Vite dev server test page |

---

*Last Updated: 2026-01-25*
