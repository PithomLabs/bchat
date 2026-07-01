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

- [ ] Create KB.MD using template or from scratch
- [ ] Include ALL services offered (with `@service` annotations)
- [ ] Include ALL services NOT offered (with `@exclusion` annotations)
- [ ] List complete service areas with `@coverage: include/exclude`
- [ ] Add emergency protocol with after-hours contact
- [ ] Include pricing guidance (structure, not fixed prices)
- [ ] Add 6+ FAQs: hours, estimates, payment, insurance, scheduling, cancellation
- [ ] Add safety procedures for emergency services
- [ ] Remove ALL placeholder text (`[Your Company Name]`, etc.)
- [ ] Review KB.MD with business owner for accuracy

---

## Phase 4: Policy Setup

- [ ] Create POLICY.MD using template
- [ ] Define agent identity and tone
- [ ] Set escalation trigger keywords
- [ ] Configure urgency thresholds
- [ ] Define lead capture rules
- [ ] Set "Say Unknown + Capture" behavior
- [ ] Define after-hours handling

---

## Phase 5: Widget Deployment

- [ ] Get embed URL: `https://your-server.com/widget/[slug]/embed.js`
- [ ] Add JavaScript embed to website
- [ ] Test widget loads on page
- [ ] Verify widget styling matches brand
- [ ] Test on mobile device

---

## Phase 6: Testing (Critical!)

- [ ] Click "Rebuild Index" after uploading files
- [ ] Run 3+ simulations with different personas:
  - Emergency scenario
  - Quote request scenario
  - General inquiry scenario
- [ ] Test RAG debug with 10+ queries
- [ ] Verify lead capture extracts name/email/phone
- [ ] Test escalation flow ("I want to speak to a manager")
- [ ] Check "Say Unknown" behavior works

---

## Phase 7: Go Live

- [ ] Confirm business owner has admin access
- [ ] Show business owner how to view leads
- [ ] Set up lead notification (email/SMS bridge)
- [ ] Train on daily lead review process
- [ ] Schedule follow-up for agent optimization

---

## Quick Reference

**Admin URL:** `https://your-server.com/agent-admin`

**Embed Script:**
```html
<script
  src="https://your-server.com/widget/[slug]/embed.js"
  data-position="bottom-right"
  data-color="#0d9488"
></script>
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