# Agent Chat Widget

Embeddable chat widget for the bchat AI Agent system.

## Development

```bash
# Install dependencies
npm install

# Start dev server (opens test page)
npm run dev

# Build for production
npm run build

# Type check
npm run typecheck
```

## Usage

### JavaScript Embed

Add to any website:

```html
<script
  src="https://your-server.com/widget/tenant-slug/embed.js"
  data-position="bottom-right"
  data-color="#0d9488"
  data-welcome="How can we help?"
></script>
```

Or configure via global object:

```html
<script>
  window.AgentChatConfig = {
    baseUrl: 'https://your-server.com',
    tenant: 'tenant-slug',
    color: '#0d9488',
    position: 'bottom-right',
    welcomeMessage: 'How can we help you today?'
  };
</script>
<script src="https://your-server.com/widget/embed.min.js"></script>
```

### iframe Embed

```html
<iframe
  src="https://your-server.com/widget/tenant-slug/iframe?color=%230d9488"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat Widget"
></iframe>
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `tenant` | string | (required) | Tenant slug |
| `baseUrl` | string | (from script URL) | Server base URL |
| `companyName` | string | - | Company name in header |
| `color` | string | `#0d9488` | Primary color (hex) |
| `position` | string | `bottom-right` | `bottom-right` or `bottom-left` |
| `welcome` | string | `How can we help you today?` | Welcome message |
| `buttonSize` | number | `56` | Toggle button size (px) |
| `panelWidth` | number | `350` | Panel width (px) |
| `panelHeight` | number | `500` | Panel height (px) |

## Architecture

```
src/
├── core/           # Core logic
│   ├── types.ts    # TypeScript interfaces
│   ├── api.ts      # API client
│   └── state.ts    # State management
├── ui/             # UI components
│   ├── Widget.ts   # Main orchestrator
│   ├── Button.ts   # Toggle button
│   ├── Panel.ts    # Chat panel
│   ├── Messages.ts # Message list
│   └── Input.ts    # Input area
├── styles/         # Styling
│   └── styles.ts   # Inline CSS
├── embed.ts        # JS embed entry point
├── iframe.html     # iframe HTML template
└── iframe.ts       # iframe entry point
```

## Build Output

After running `npm run build`:

```
dist/
└── embed.min.js    # Self-contained IIFE bundle (~15-20KB)
```

The iframe HTML is served dynamically by the backend with injected configuration.
