# Business Owner Quick Start Checklist

> **Printing this? Keep it handy during setup.**

---

## Phase 1: Technical Setup

- [ ] Clone bchat repository
- [ ] Install prerequisites (Go 1.21+, Node.js 18+, Task)
- [ ] Get OpenRouter API key
- [ ] Set up server (Fly.io, VPS, or local machine)
- [ ] Configure environment variables
- [ ] Build and run the application
- [ ] Verify admin panel loads at `/agent-admin`

---

## Phase 2: Tenant Creation

- [ ] Create tenant in admin panel
- [ ] Note the **slug** (URL-safe identifier)
- [ ] Set LLM model (recommended: `gpt-4o-mini`)
- [ ] Set temperature (recommended: `0.3–0.5` for service businesses)
- [ ] Configure allowed domains (if needed)
- [ ] Toggle "Is Active" to Yes

---

## Phase 3: Knowledge Base Setup

- [ ] Create KB.MD using template (`docs/templates/KB_MINIMAL.MD` for simple businesses, `docs/templates/KB.MD` for emergency services)
- [ ] Include ALL services offered (with `@service` annotations)
- [ ] Include ALL services NOT offered (with `@exclusion` annotations)
- [ ] List complete service areas with `@coverage: include/exclude`
- [ ] Add emergency protocol with after-hours contact (if applicable)
- [ ] Include pricing guidance (structure, not fixed prices)
- [ ] Add 6+ FAQs: hours, estimates, payment, insurance, scheduling, cancellation
- [ ] Add safety procedures for emergency services (if applicable)
- [ ] Remove ALL placeholder text (`[Your Company Name]`, etc.)
- [ ] Review KB.MD with business owner for accuracy

---

## Phase 4: Policy Setup

- [ ] Create POLICY.MD using template (`docs/templates/POLICY_MINIMAL.MD` for FAQ-only, `docs/templates/POLICY_TRANSACTIONAL.MD` for full booking flow)
- [ ] Define agent identity and tone
- [ ] Set escalation trigger keywords
- [ ] Configure urgency thresholds
- [ ] Define lead capture rules
- [ ] Set "Say Unknown + Capture" behavior
- [ ] Define after-hours handling

---

## Phase 5: RAG Setup

- [ ] Enable RAG: set `RAG_PIPELINE_ENABLED=true` in environment
- [ ] Configure embedding provider: `EMBEDDING_PROVIDER=openrouter`
- [ ] Build with RAG: `task build:rag`
- [ ] Upload KB.MD, POLICY.MD, SCRIPT.MD to tenant
- [ ] Click **Rebuild Index** after uploading files
- [ ] Verify RAG debug returns correct chunks for test queries

---

## Phase 6: Widget Deployment

- [ ] Choose embedding method:
  - **IIFE (recommended)** — `<script src="https://your-server.com/widget/[slug]/embed.js">`
  - **iframe** — for WordPress, Shopify, Wix (low-trust CMS)
  - **npm** — for React apps
- [ ] Add embed code to website (before `</body>`)
- [ ] Test widget loads on page
- [ ] Verify widget styling matches brand (color, position, welcome message)
- [ ] Test on mobile device
- [ ] Verify allowed domains configured in tenant settings

---

## Phase 7: Testing (Critical!)

- [ ] Run 3+ simulations with different personas:
  - Emergency scenario (if applicable)
  - Quote request scenario
  - General inquiry scenario
- [ ] Test RAG debug with 10+ queries
- [ ] Verify lead capture extracts name/email/phone
- [ ] Test escalation flow ("I want to speak to a manager")
- [ ] Check "Say Unknown" behavior works
- [ ] Verify conversation replay works in admin

---

## Phase 8: Go Live

- [ ] Confirm business owner has admin access
- [ ] Show business owner how to view leads
- [ ] Set up lead notification (email/SMS bridge)
- [ ] Train on daily lead review process
- [ ] Schedule follow-up for agent optimization

---

## Quick Reference

**Admin URL:** `https://your-server.com/agent-admin`

**Embed Script (IIFE):**
```html
<script
  src="https://your-server.com/widget/[slug]/embed.js"
  data-position="bottom-right"
  data-color="#0d9488"
></script>
```

**Embed Script (iframe — for CMS):**
```html
<iframe
  src="https://your-server.com/widget/[slug]/iframe?color=%230d9488"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat Widget"
></iframe>
```

**Critical Buttons:**
- **Create Tenant** → Sets up new business
- **Upload Files** → KB.MD, POLICY.MD, SCRIPT.MD
- **Rebuild Index** → Must click after every file upload!
- **Simulations** → Test before going live

**Lead Statuses:**
- `new` → Fresh lead, not yet contacted
- `contacted` → Someone reached out
- `converted` → Became a customer
- `lost` → Not interested or went elsewhere

**Configuration Priority:**
1. Tenant Config (Agent Admin UI) → Highest
2. Environment Variable → Fallback
3. Hardcoded Default → Lowest
