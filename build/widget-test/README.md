Based on my analysis, here's how the widget embed code works and how to apply the Agent Admin embed code to your test site.

## How Agent Admin Generates the Embed Code

In [`web/src/pages/AgentAdmin.tsx`](web/src/pages/AgentAdmin.tsx:1052) (lines 1052-1063), the generated code format is:

```html
<script src="{baseUrl}/widget/{slug}/embed.js"></script>
<script>
  AgentChatWidget.init({
    tenant: "{slug}",
    baseUrl: "{baseUrl}",
    color: "{widgetColor}",
    position: "{widgetPosition}",
    welcomeMessage: "{widgetWelcome}",
    companyName: "{companyName}"
  });
</script>
```

The backend serves this at two endpoints registered in [`server/router/api/v1/v1.go`](server/router/api/v1/v1.go:201):
- `GET /widget/:slug/embed.js` — serves the JS bundle with injected config
- `GET /widget/:slug/iframe` — serves a standalone iframe HTML page

## Your Current Tenant

Your database (`memos_dev.db`) has one active tenant:

| id | slug | company_name | is_active |
|----|------|--------------|-----------|
| 7 | `scraper` | Pithom Labs | 1 |

## Mismatch in Your Test Files

Your test site files currently reference non-existent tenants:

- `build/widget-test/static/simple.html` — uses tenant `rizal` / `bchat`
- `build/widget-test/static/simplelive.html` — uses tenant `bchat`
- `build/widget-test/static/iframe.html` — uses tenant `rizal`
- `build/widget-test/static/index.html` — shows tenant `inc`

## How to Embed the Correct Agent Admin Code

### Option A: Update `build/widget-test/static/simple.html` (JS injection)

Replace the embed block at the bottom with your actual tenant:

```html
<!-- Widget embed code -->
<script src="http://localhost:8081/widget/scraper/embed.js"></script>
<script>
  AgentChatWidget.init({
    tenant: "scraper",
    baseUrl: "http://localhost:8081",
    color: "#0d9488",
    position: "bottom-right",
    welcomeMessage: "Hi! How can I help you today?",
    companyName: "Pithom Labs"
  });
</script>
```

Also update the explanatory text (lines 95-99) to reference `/api/v1/agent/scraper/chat/ext` instead of `/api/v1/agent/rizal/chat/ext`.

### Option B: Update `build/widget-test/static/iframe.html` (iframe method)

Replace the iframe `src` on line 178:

```html
<iframe
  src="http://localhost:8081/widget/scraper/iframe?color=%230d9488&welcome=Hi!%20How%20can%20I%20help%20you%20today%3F&companyName=Pithom%20Labs"
  title="Chat Widget"
></iframe>
```

### Option C: Use `widget/site/index.html` (the canonical test page)

The file [`widget/site/index.html`](widget/site/index.html:317) already uses the Agent Admin format with tenant `scraper` pointing to `http://localhost:8081`. This is the reference implementation. To use it:

```bash
# Terminal 1: Backend
task run:rag

# Terminal 2: Serve widget site
cd widget/site && python3 -m http.server 8080
```

Then open `http://localhost:8080`.

### Option D: Use `build/widget-test` Go server

The Go server in [`build/widget-test/main.go`](build/widget-test/main.go:9) serves the `static/` folder on port 8888. Update the HTML files in `static/` with the correct tenant slug (`scraper`), then:

```bash
./build/widget-test/test  # or go run main.go
# Then open http://localhost:8888/simple.html
```

## Summary of What Needs to Change

| File | Current tenant | Correct tenant |
|------|---------------|----------------|
| `widget-test/static/simple.html` | `rizal` / `bchat` | `scraper` |
| `widget-test/static/simplelive.html` | `bchat` | `scraper` |
| `widget-test/static/iframe.html` | `rizal` | `scraper` |
| `widget-test/static/index.html` | `inc` | `scraper` |

The embed code pattern is identical to what Agent Admin generates — just substitute the actual tenant slug (`scraper`), base URL (`http://localhost:8081` for local testing), and company name (`Pithom Labs`).