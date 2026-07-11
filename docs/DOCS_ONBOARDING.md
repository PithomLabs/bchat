# Onboarding Playbook

> **Audience:** Implementation specialists — product people with technical skills who onboard new customers onto bchat.
>
> **Purpose:** End-to-end guide for taking a customer from first call to live AI agent. Covers discovery, content creation with LLM assistance, technical setup, widget deployment, validation, and ongoing maintenance.
>
> **Estimated time per customer:** 4–8 hours (first time), 2–3 hours (experienced)

---

## Table of Contents

1. [Welcome & Scope](#1-welcome--scope)
2. [Customer Intake](#2-customer-intake)
3. [Feature Selection](#3-feature-selection)
4. [KB.MD Design](#4-kbmd-design)
5. [POLICY.MD Design](#5-policymd-design)
6. [SCRIPT.MD Design](#6-scriptmd-design)
7. [LLM Prompting Recipes](#7-llm-prompting-recipes)
8. [Tenant Provisioning](#8-tenant-provisioning)
9. [RAG Configuration](#9-rag-configuration)
10. [Observational Memory](#10-observational-memory)
11. [Webhook Integrations](#11-webhook-integrations)
12. [Widget Embedding](#12-widget-embedding)
13. [CMS-Specific Checklists](#13-cms-specific-checklists)
14. [Widget Customization](#14-widget-customization)
15. [Widget Testing](#15-widget-testing)
16. [Simulation Testing](#16-simulation-testing)
17. [Pre-Launch Checklist](#17-pre-launch-checklist)
18. [Go-Live & Maintenance](#18-go-live--maintenance)
19. [Appendix A: Annotation Quick Reference](#appendix-a-annotation-quick-reference)
20. [Appendix B: Common Customer Patterns](#appendix-b-common-customer-patterns)
21. [Appendix C: Troubleshooting Matrix](#appendix-c-troubleshooting-matrix)
22. [Appendix D: Glossary](#appendix-d-glossary)

---

# Phase 1: Discovery

---

## 1. Welcome & Scope

This playbook guides you through onboarding a new customer onto bchat. You'll gather their requirements, help create their AI agent's knowledge base and policy files using LLM assistance, deploy the chat widget on their website, and validate everything before go-live.

### What you'll deliver

- A configured tenant with KB.MD, POLICY.MD, and SCRIPT.MD
- RAG pipeline indexed and searchable
- Chat widget embedded and tested on the customer's website
- Simulation tests passed
- Customer trained on admin panel and lead management

### Prerequisites

- Access to the bchat admin panel
- OpenRouter API key configured on the server
- Basic understanding of markdown editing
- Customer's website CMS access (or their developer's contact)

---

## 2. Customer Intake

Complete this questionnaire during your first call or email exchange with the customer. Every field informs which features to enable and how to write their configuration files.

### Business Profile

| Question | Why it matters |
|----------|---------------|
| What is your business name? | Displayed in chat header, used throughout agent responses |
| What industry/vertical are you in? | Determines template selection (minimal vs transactional) |
| What services or products do you offer? | Maps to KB.MD `@service` entries |
| What services do you NOT offer? | Maps to KB.MD `@exclusion` entries |
| What is your service area? | Maps to KB.MD `@coverage` entries |
| Do you have 24/7 emergency service? | Determines if emergency flow is needed |
| What are your business hours? | KB.MD FAQ + POLICY.MD identity |
| Do you offer free estimates? | KB.MD FAQ |
| What payment methods do you accept? | KB.MD FAQ |
| Are you licensed/insured? | KB.MD FAQ + credibility signal |

### Customer Experience

| Question | Why it matters |
|----------|---------------|
| What is your preferred tone? (professional, friendly, casual, urgent) | POLICY.MD identity |
| How should the agent handle emergencies? | POLICY.MD intent routing |
| Who should the agent escalate to? | POLICY.MD escalation rules |
| Do you want the agent to collect leads? | POLICY.MD lead capture policy |
| How do you want to be notified of new leads? | Webhook/notification setup |

### Technical Profile

| Question | Why it matters |
|----------|---------------|
| Do you have an existing website? | Widget deployment method |
| What CMS/platform is your website on? | Widget embedding approach (IIFE vs iframe) |
| What is your website URL? | Allowed domains configuration |
| Do you need multi-language support? | Future feature; note for now |
| Do you have CRM or ticketing software? | Webhook integrations |

### Timeline

| Question | Why it matters |
|----------|---------------|
| When do you want to go live? | Scopes work effort |
| Who will provide the KB content? | You draft with LLM, they review |
| Who has final approval authority? | Avoids rework loops |

---

## 3. Feature Selection

Based on the intake, select which features to enable. Use this decision matrix:

### Feature Matrix

| Feature | When to enable | Configuration |
|---------|---------------|---------------|
| **RAG Pipeline** | KB > 20KB or complex knowledge base | `RAG_PIPELINE_ENABLED=true` |
| **Observational Memory** | Long-running conversations, repeat visitors | `OM_ENABLED=true` |
| **Lead Capture** | Service businesses, B2C | POLICY.MD lead capture rules |
| **Emergency Flow** | 24/7 service businesses (restoration, HVAC, plumbing) | POLICY.MD emergency intents + SCRIPT.MD priority routing |
| **Booking/Scheduling** | Appointment-based businesses | POLICY.MD booking intents + SCRIPT.MD intake flow |
| **Outbound Webhooks** | CRM integration, ticketing, notifications | Webhook configuration in tenant settings |

### Template Selection

| Customer type | KB template | POLICY template | SCRIPT template |
|--------------|-------------|-----------------|-----------------|
| Simple service business (no emergencies) | `KB_MINIMAL.MD` | `POLICY_MINIMAL.MD` | `SCRIPT_MINIMAL.MD` |
| Emergency service (restoration, HVAC, plumbing) | `KB.MD` (full) | `POLICY_TRANSACTIONAL.MD` | `SCRIPT_TRANSACTIONAL.MD` |
| E-commerce / product catalog | `KB_ECOMMERCE.MD` | `POLICY_MINIMAL.MD` | `SCRIPT_MINIMAL.MD` |
| Complex transactional (insurance, legal) | `KB.MD` (full) | `POLICY_TRANSACTIONAL.MD` | `SCRIPT_TRANSACTIONAL.MD` |

### Quick Decision Tree

```
Does the business have emergency/urgent services?
├── YES → Use full templates (KB.MD, POLICY_TRANSACTIONAL.MD, SCRIPT_TRANSACTIONAL.MD)
└── NO
    ├── Does the business take bookings/appointments?
    │   ├── YES → Use MINIMAL KB + TRANSACTIONAL POLICY/SCRIPT
    │   └── NO → Use all MINIMAL templates
    └── Is it e-commerce?
        ├── YES → Use KB_ECOMMERCE.MD + MINIMAL POLICY/SCRIPT
        └── NO → Use MINIMAL templates
```

---

# Phase 2: Content Creation with LLMs

---

## 4. KB.MD Design

KB.MD is the agent's factual memory. Every fact the agent references comes from this file. Write it for a **smart, friendly new hire** who needs context to represent the business accurately.

### Writing Principles

1. **Be specific.** "We serve Nassau and Suffolk Counties" beats "We serve the tri-state area."
2. **Use bullet points.** The agent handles structured content better than long paragraphs.
3. **Include every phone number, email, and address.** The agent must never guess contact info.
4. **Name specific cities and ZIP codes.** "We serve 110XX, 117XX, 118XX" beats "We serve Long Island."
5. **Specify time windows.** "Within 60 minutes" beats "fast response."
6. **List what you DON'T do.** Exclusions prevent the agent from making false promises.

### Structure

Every KB.MD should contain these sections in order:

1. **Company Overview** — name, type, certifications, availability
2. **Contact Information** — phone, email, address
3. **Services** — each with `@service` annotation
4. **Services We Don't Provide** — each with `@exclusion` annotation
5. **Service Areas** — `@coverage: include` and `@coverage: exclude`
6. **Frequently Asked Questions** — each with `@faq` annotation
7. **Safety Procedures** (if emergency services) — each with `@safety` annotation
8. **What To Expect** — process overview

### Annotation Reference

| Annotation | Purpose | Example |
|------------|---------|---------|
| `@service: code, emergency: true/false` | Service entries | `@service: water_damage, emergency: true, response_time: "30 minutes"` |
| `@exclusion: code` | Services NOT offered | `@exclusion: general_plumbing` |
| `@coverage: include` | Service area sections | `@coverage: include` |
| `@coverage: exclude` | Excluded areas | `@coverage: exclude` |
| `@faq: category` | FAQ entries | `@faq: pricing` |
| `@safety: hazard_type` | Safety procedures | `@safety: water_emergency` |
| `@section: name` | Logical sections | `@section: company_overview` |

### Anti-Hallucination Rules

- **Never** leave placeholder text (`[Your Company Name]`, `[Phone Number]`). The agent will fill these with guesses.
- **Never** include content from other businesses or templates that hasn't been customized.
- **Never** quote exact prices unless they are truly fixed. Use "Costs vary based on [factors]. We offer free estimates."
- **Always** include a fallback contact ("Call us at...") for information the agent might not have.

### Content Quality Checklist

Before finalizing KB.MD:

- [ ] Every service the business offers has a `@service` entry
- [ ] Every major service NOT offered has an `@exclusion` entry
- [ ] All service areas listed with `@coverage: include` or `@coverage: exclude`
- [ ] All phone numbers, emails, and addresses are current
- [ ] Emergency protocol includes after-hours contact (if applicable)
- [ ] Pricing section sets expectations without false specifics
- [ ] FAQs cover: hours, estimates, payment, insurance, scheduling
- [ ] No placeholder text remains
- [ ] Reviewed by business owner for accuracy

---

## 5. POLICY.MD Design

POLICY.MD is the agent's operating system. It defines who the agent is, how it behaves, and how it handles edge cases.

### Key Sections

#### Identity

Define the agent's persona:

```markdown
## Identity
<!-- @identity -->

- **Role:** Customer Service Representative for [Business Name]
- **Tone:** Professional, empathetic, and helpful
- **Brand Voice:** "We're here to help."
- **Business hours:** Monday–Friday, 8 AM – 6 PM
- **After-hours handling:** Collect lead, promise next-business-day callback
```

**Tone guidelines by vertical:**
| Vertical | Recommended tone |
|----------|-----------------|
| Restoration / Emergency | Calm, reassuring, action-oriented |
| Professional services | Professional, knowledgeable, measured |
| Retail / E-commerce | Friendly, helpful, enthusiastic |
| Healthcare | Empathetic, careful, compliant |

#### Intents

Each intent defines a category of customer message and how the agent should respond:

```markdown
### Emergency
<!-- @intent: emergency, category: emergency, urgency: 5, action: emergency_flow -->

**Description:** Urgent, time-sensitive damage or safety issue.

**Agent behavior:**
- Acknowledge urgency immediately
- Provide safety instructions
- Collect: name, phone, location, nature of emergency
```

**Required intents for every tenant:**
- `emergency` (if applicable)
- `service_inquiry`
- `quote_request` or `booking_request` (or both)
- `escalation`
- `greeting`
- `farewell`

#### Rules

Behavioral rules the agent must follow:

```markdown
### Safety First
<!-- @rule: safety_first, priority: 1 -->

In any emergency, provide safety instructions BEFORE collecting contact info.
```

**Minimum required rules:**
- Safety first (if emergency services)
- No price guessing
- Stay in scope
- Empathy first
- Fallback behavior ("Say Unknown + Capture")

#### Thresholds

Numeric thresholds for intent classification:

```markdown
## Thresholds
<!-- @thresholds -->

| Metric | Value | Description |
|--------|-------|-------------|
| Emergency Urgency | >= 4 | Route to emergency flow |
| Escalation Confidence | >= 0.85 | Confirm escalation intent |
```

#### Lead Capture

When and how the agent collects contact information:

```markdown
## Lead Capture Policy
<!-- @rule: lead_capture -->

Collect contact info when:
- Visitor asks for a quote or estimate
- Visitor asks to schedule or book
- Visitor reports an emergency

Do NOT aggressively collect on:
- First message before understanding the need
- Simple FAQ resolved in KB.MD
```

### POLICY.MD Quality Checklist

- [ ] Identity defines role, tone, brand voice, and after-hours behavior
- [ ] All required intents are defined with clear agent behavior
- [ ] Rules cover: safety, pricing, scope, empathy, fallback
- [ ] Thresholds are set appropriately for the vertical
- [ ] Lead capture rules are defined (when to ask, when NOT to ask)
- [ ] Escalation behavior is defined with trigger keywords
- [ ] Human takeover rules are defined

---

## 6. SCRIPT.MD Design

SCRIPT.MD defines the conversation flow — what stages a conversation goes through and what happens at each stage. It's optional but recommended for transactional businesses.

### When to use SCRIPT.MD

| Use SCRIPT.MD | Skip SCRIPT.MD |
|---------------|----------------|
| Appointment booking flows | Simple FAQ-only agents |
| Lead qualification | Informational chat bots |
| Emergency intake | Product catalog browsing |
| Multi-step intake processes | Single-question Q&A |

### Minimal Script (3 stages)

For simple service businesses:

```markdown
## Stage: Opening
- Greet the visitor warmly
- State who you represent
- Ask how you can help today

## Stage: Discovery
- Ask clarifying questions
- Determine urgency and service needed

## Stage: Closing
- If showing intent: collect contact info, confirm next steps
- Thank the visitor
```

### Transactional Script (full intake)

For emergency services or appointment booking. See `docs/templates/SCRIPT_TRANSACTIONAL.MD` for the complete template. Key elements:

1. **Priority 0: Emergency + Coverage Check** — runs on every message before other processing
2. **Emergency responses** — pre-built templates for emergency + in-area, out-of-area, unknown location
3. **Business introduction** — greeting after priority check passes
4. **Service intake** — structured questions for collecting service details
5. **Contact information** — collecting name, phone, address
6. **Confirmation** — summarizing and confirming details
7. **Conclusion** — next steps and closing

### Writing Tips

- Keep stages concise — bullet points, not paragraphs
- Define clear transitions between stages
- Include "skip if already provided" rules to avoid repetitive questions
- Add special scenario handling (upset customers, repeat visitors)

---

## 7. LLM Prompting Recipes

Use these generic prompts with any LLM (GPT-4o, Claude, Gemini) to draft and refine KB.MD, POLICY.MD, and SCRIPT.MD.

### Drafting KB.MD

**Prompt:**
```
I'm onboarding a new customer onto an AI chat agent platform. Help me draft their knowledge base file (KB.MD).

Business details:
- Name: [Business Name]
- Industry: [Industry]
- Services: [List of services]
- Service areas: [Cities, counties, ZIP codes]
- Business hours: [Hours]
- Emergency service: [Yes/No, response time]
- Phone: [Number]
- Email: [Address]

Write a KB.MD file in markdown format using these annotations:
- <!-- @service: code, emergency: true/false --> for each service
- <!-- @exclusion: code --> for services NOT offered
- <!-- @coverage: include --> and <!-- @coverage: exclude --> for service areas
- <!-- @faq: category --> for each FAQ entry
- <!-- @safety: hazard_type --> for emergency safety procedures (if applicable)

Include sections for: Company Overview, Contact Info, Services, Exclusions, Service Areas, FAQs (minimum 6), Safety Procedures (if emergency), and What To Expect.
```

### Drafting POLICY.MD

**Prompt:**
```
I'm onboarding a new customer onto an AI chat agent platform. Help me draft their agent policy file (POLICY.MD).

Business details:
- Name: [Business Name]
- Tone: [Professional/Friendly/Casual/Urgent]
- Emergency service: [Yes/No]
- Escalation contact: [Name, phone]
- Business hours: [Hours]
- After-hours behavior: [Collect lead/Show message/etc.]

Write a POLICY.MD file using these annotations:
- <!-- @identity --> for agent persona
- <!-- @intent: name, category: standard/emergency/meta, urgency: 1-5, action: flow_name --> for each intent
- <!-- @rule: name, priority: N --> for behavioral rules
- <!-- @thresholds --> for urgency scoring thresholds

Include: Identity, Intents (emergency if applicable, service_inquiry, quote_request, escalation, greeting, farewell), Rules (safety, pricing, scope, empathy, fallback), Thresholds, Lead Capture Policy, Human Takeover rules.
```

### Drafting SCRIPT.MD

**Prompt:**
```
I'm onboarding a new customer onto an AI chat agent platform. Help me draft their conversation flow file (SCRIPT.MD).

Business details:
- Name: [Business Name]
- Type: [Service business / Emergency service / E-commerce]
- Services: [List]
- Service areas: [Areas]
- Emergency: [Yes/No]

Write a SCRIPT.MD file that defines conversation stages. Use a structure like:
- Priority 0: Emergency + Coverage Check (if emergency service)
- Business Introduction
- Service Intake (questions to ask)
- Contact Information
- Confirmation
- Conclusion

Include response templates for different scenarios (emergency in-area, emergency out-of-area, standard inquiry, etc.).
```

### Refining Content

**Prompt:**
```
Review this KB.MD file for a [business type] business. Check for:
1. Missing services or exclusions
2. Placeholder text that wasn't replaced
3. Missing contact information
4. Vague or unhelpful FAQ answers
5. Missing service areas

Suggest specific improvements. Here's the current file:
[paste file content]
```

### Iterative Workflow

1. **Draft** — Use the appropriate prompt to generate an initial draft
2. **Review** — Read through with the customer (or alone) to identify gaps
3. **Refine** — Use the refinement prompt to improve specific sections
4. **Validate** — Check against the content quality checklist (sections 4-6)
5. **Test** — Upload to tenant, run simulations, identify issues
6. **Repeat** — Iterate based on simulation results

---

# Phase 3: Technical Setup

---

## 8. Tenant Provisioning

### Create the Tenant

1. Log in to the admin panel at `https://your-server.com/agent-admin`
2. Navigate to **Tenants** → **Create Tenant**
3. Fill in:

| Field | Value | Notes |
|-------|-------|-------|
| Slug | `[business-name]` | URL-safe, lowercase, hyphens only |
| Company Name | `[Business Name]` | Display name in chat header |
| Vertical | `[industry]` | For your reference |
| LLM Model | `openai/gpt-4o-mini` | Default; can override per tenant |
| Temperature | `0.3–0.5` | Lower = more factual; higher = more creative |
| Allowed Domains | `[customer-domain.com]` | Comma-separated; blank = all domains |
| Is Active | `Yes` | Must be active for widget to work |

### Slug Guidelines

- Use the business name: `acme-pressure-washing`
- Lowercase only, hyphens for spaces
- No special characters, underscores, or uppercase
- Keep it short but descriptive

### Post-Creation

- Note the tenant slug — it's used in the widget embed URL
- Verify the GUID was generated (used for widget auth)
- Confirm the tenant appears in the tenant list

---

## 9. RAG Configuration

### When to Enable RAG

| KB Size | Recommendation |
|---------|---------------|
| < 20KB | RAG optional (everything fits in prompt) |
| 20KB–100KB | RAG recommended |
| > 100KB | RAG required |

When in doubt, enable RAG. It costs nothing extra and improves accuracy for larger knowledge bases.

### Setup Steps

1. **Set environment variables:**
   ```bash
   RAG_PIPELINE_ENABLED=true
   EMBEDDING_PROVIDER=openrouter
   EMBEDDING_MODEL=text-embedding-3-small
   ```

2. **Build with RAG support:**
   ```bash
   task build:rag
   ```

3. **Upload files to tenant** (via admin panel)

4. **Click "Rebuild Index"** in the tenant detail page

5. **Verify indexing:**
   - Check the RAG debug panel for the tenant
   - Run test queries to confirm relevant chunks are retrieved
   - Verify total chunks indexed and last indexed timestamp

### Embedding Providers

| Provider | Use case | API key needed |
|----------|----------|---------------|
| `openrouter` | Production | Yes |
| `mock` | Testing pipeline only | No |
| `local` | Local embedding server | No |

### Hybrid Search

By default, RAG uses hybrid search:
- **70% vector similarity** — semantic matching (finds related concepts)
- **30% BM25 keyword matching** — exact term matching (finds service names, city names)

This means both "water removal" and "standing water extraction" return the same relevant chunk.

### When to Rebuild Index

- After every KB.MD upload or edit
- After changing processing options
- When the agent gives wrong answers about existing content
- Force reindex on startup: `FORCE_REINDEX_ON_STARTUP=true`

---

## 10. Observational Memory

Observational Memory (OM) gives agents long-term memory by compressing conversation history into an observation log. This is useful for businesses with repeat customers or long-running conversations.

### When to Enable

| Scenario | Enable OM? |
|----------|-----------|
| Single-session FAQ bot | No |
| Repeat customer relationships | Yes |
| Long intake processes | Yes |
| Enterprise/CRM-integrated | Yes |

### Configuration

```bash
OM_ENABLED=true
OM_OBSERVER_TOKEN_THRESHOLD=30000  # Trigger observer after N tokens
OM_TOKEN_THRESHOLD=2000             # Trigger reflector to compress observations
```

### How It Works

1. Messages accumulate in a buffer
2. When token threshold is reached, the observer compresses them into observations
3. Observations are stored as system context for future messages
4. The agent can reference earlier parts of the conversation without the full history

---

## 11. Webhook Integrations

### Outbound Webhooks

Send conversation events to external systems (CRM, ticketing, calendar):

| Event | Payload | Use case |
|-------|---------|----------|
| `lead.created` | Lead details + transcript | CRM integration |
| `lead.updated` | Status change | Pipeline sync |
| `message.sent` | Full message | Audit log |
| `simulation.completed` | Transcript + analysis | QA tracking |

### Configuration

In the tenant admin panel, under **Integrations**:

1. Click **Add Webhook**
2. Enter the target URL
3. Select events to subscribe to
4. Configure authentication (if required)
5. Test with the webhook test button

### Inbound Webhooks

Receive data from external systems:

| Use case | Example |
|----------|---------|
| CRM lead lookup | Enrich conversation with customer history |
| Calendar availability | Check real-time availability during booking |
| Ticketing system | Create tickets from escalation events |

---

# Phase 4: Widget Deployment

---

## 12. Widget Embedding

Three methods to embed the chat widget. Choose based on the customer's website platform.

### Option 1: IIFE Script (Recommended)

Best for: Custom websites, HTML sites, Hugo, Jekyll, static sites.

```html
<script
  src="https://your-server.com/widget/[slug]/embed.js"
  data-position="bottom-right"
  data-color="#0d9488"
  data-welcome="Hi! How can we help you today?"
></script>
```

**Placement:** Before `</body>` tag.

**Configuration attributes:**

| Attribute | Default | Options |
|-----------|---------|---------|
| `data-position` | `bottom-right` | `bottom-right`, `bottom-left` |
| `data-color` | `#0d9488` | Any hex color |
| `data-welcome` | `How can we help you today?` | Any string |
| `data-button-size` | `56` | Pixel size |
| `data-panel-width` | `350` | Pixel width |
| `data-panel-height` | `500` | Pixel height |

### Option 2: iframe Embed

Best for: WordPress, Shopify, Wix, Squarespace — any CMS with limited script injection.

```html
<iframe
  src="https://your-server.com/widget/[slug]/iframe?color=%230d9488"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat Widget"
></iframe>
```

**Note:** URL-encode `#` as `%23` in query parameters.

### Option 3: npm Package

Best for: React/Vue/Angular apps.

```jsx
import { useEffect } from 'react';

function ChatWidget({ tenantSlug, color = '#0d9488' }) {
  useEffect(() => {
    window.AgentChatConfig = { color };
    const script = document.createElement('script');
    script.src = `https://your-server.com/widget/${tenantSlug}/embed.js`;
    script.async = true;
    document.body.appendChild(script);

    return () => {
      const widget = document.getElementById('agent-chat-widget');
      if (widget) widget.remove();
      script.remove();
    };
  }, [tenantSlug, color]);

  return null;
}
```

### Domain Restrictions

For security, set allowed domains in tenant settings:

```
customer-domain.com, www.customer-domain.com
```

Leave blank to allow embedding on any domain. The widget checks the `Referer` header against this list.

---

## 13. CMS-Specific Checklists

### Hugo (Self-Hosted)

1. Create a partial: `layouts/partials/bchat-widget.html`
2. Paste the IIFE script tag
3. Include the partial in your base template: `{{ partial "bchat-widget.html" . }}`
4. Build and deploy: `hugo && rsync public/ user@server:/var/www/`

**Checklist:**
- [ ] Widget partial created
- [ ] Partial included in base template
- [ ] Build succeeds without errors
- [ ] Widget loads on deployed site

### WordPress

**Option A: Plugin (if available)**
1. Install a "Custom HTML" or "Insert Headers and Footers" plugin
2. Paste the iframe embed code
3. Save and verify

**Option B: Manual (footer.php)**
1. Log in to WordPress admin
2. Go to Appearance → Theme Editor
3. Edit `footer.php`
4. Paste the iframe code before `</body>`
5. Save and verify

**Checklist:**
- [ ] Embed code added to site
- [ ] Widget appears on frontend
- [ ] No conflicts with existing plugins
- [ ] Mobile responsive

### Shopify

1. Go to Online Store → Themes → Edit Code
2. Open `theme.liquid`
3. Paste the iframe embed before `</body>`
4. Save and preview

**Checklist:**
- [ ] Code added to `theme.liquid`
- [ ] Widget appears on storefront
- [ ] No conflicts with Shopify scripts
- [ ] Works on mobile

### Wix / Squarespace

These platforms don't allow direct HTML editing on all pages. Use the iframe method:

1. Add an "Embed HTML" element to the page
2. Paste the iframe code
3. Resize the element to cover the desired widget area
4. Publish and verify

**Checklist:**
- [ ] Embed element added
- [ ] iframe code pasted
- [ ] Widget visible after publish
- [ ] No layout conflicts

### Generic HTML

1. Open the site's HTML files
2. Paste the IIFE script before `</body>`
3. Deploy to hosting

**Checklist:**
- [ ] Script added to all relevant pages
- [ ] Widget loads on every page
- [ ] No JavaScript errors in console

---

## 14. Widget Customization

### Brand Matching

| Element | How to customize |
|---------|-----------------|
| Primary color | `data-color` attribute or `color` query param |
| Button position | `data-position` attribute (bottom-right or bottom-left) |
| Welcome message | `data-welcome` attribute |
| Company name | Set in tenant settings (auto-populated) |
| Panel size | `data-panel-width` and `data-panel-height` |

### CSS Overrides

For advanced customization, override widget CSS after the script loads:

```html
<style>
  #acw-toggle, #acw-header, .acw-msg-user, #acw-send {
    background: #4f46e5 !important; /* Indigo */
  }
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
| `#acw-messages` | Messages container |
| `#acw-input` | Text input |
| `#acw-send` | Send button |

---

## 15. Widget Testing

### Local Testing

1. Open the page with the widget embed in a browser
2. Click the chat button — panel should open
3. Send a test message — verify agent responds
4. Check browser console for JavaScript errors
5. Verify the welcome message appears

### Cross-Browser Checklist

- [ ] Chrome (latest)
- [ ] Firefox (latest)
- [ ] Safari (latest)
- [ ] Edge (latest)
- [ ] Mobile Safari (iOS)
- [ ] Chrome for Android

### Mobile Testing

- Widget should be responsive on screens < 480px
- Panel should expand to near-full-width on mobile
- Input should be usable with virtual keyboard
- Button should not overlap critical page content

### Security Verification

- [ ] Widget loads over HTTPS
- [ ] No mixed content warnings
- [ ] Domain restrictions are working (if configured)
- [ ] iframe is isolated from parent page

---

# Phase 5: Validation & Launch

---

## 16. Simulation Testing

### Running Simulations

1. Go to **Agent Admin** → **[Tenant]** → **Simulations**
2. Configure a virtual customer persona:
   - **Name:** [Test name]
   - **Scenario:** [Description of the test scenario]
   - **Goal:** [What the virtual customer wants to achieve]
   - **Tone:** [Anxious, neutral, friendly, frustrated]
3. Click **Run Simulation**
4. Watch the conversation unfold in real-time

### Test Scenarios (minimum 3 per tenant)

| # | Scenario | Tests |
|---|----------|-------|
| 1 | Emergency inquiry | Emergency flow, safety instructions, contact collection |
| 2 | Quote/service request | Service matching, lead capture, follow-up promise |
| 3 | General FAQ | Information retrieval, no unnecessary lead capture |
| 4 | Out-of-area | Coverage handling, polite decline |
| 5 | Escalation | Trigger detection, handoff behavior |

### Interpreting Results

| Issue | Likely cause | Fix |
|-------|-------------|-----|
| Agent gives wrong info | KB.MD missing or incomplete | Add content to KB.MD, rebuild index |
| Agent guesses pricing | KB.MD has pricing or POLICY.MD missing rule | Add "no price guessing" rule |
| Agent doesn't capture lead | POLICY.MD missing lead capture rules | Add lead capture policy |
| Agent can't answer basic question | RAG not enabled or index stale | Enable RAG, rebuild index |
| Agent escalates too easily | Thresholds too low | Adjust urgency/confidence thresholds |
| Agent is too robotic | Temperature too low | Increase to 0.4–0.6 |

### RAG Debug Testing

1. Go to **Agent Admin** → **[Tenant]** → **RAG Debug**
2. Enter test queries and verify:
   - Correct chunks are retrieved
   - Relevance scores are reasonable (> 0.5)
   - Exclusions are respected
   - Service areas are matched

---

## 17. Pre-Launch Checklist

### Content Verification

- [ ] KB.MD reviewed by business owner (not just you)
- [ ] All phone numbers, emails, addresses verified current
- [ ] All services and exclusions accurate
- [ ] Service areas complete and correct
- [ ] No placeholder text remaining

### Technical Verification

- [ ] Tenant is active (`Is Active = Yes`)
- [ ] RAG index rebuilt after final content upload
- [ ] Widget loads without errors on customer's website
- [ ] Widget works on mobile
- [ ] Domain restrictions configured (if needed)
- [ ] HTTPS working for widget delivery

### Agent Behavior

- [ ] At least 3 simulations passed
- [ ] RAG debug returns correct chunks for 10+ queries
- [ ] Lead capture tested (name + email/phone extracted)
- [ ] Escalation flow tested ("I want a manager")
- [ ] Fallback behavior works ("I don't have that info")
- [ ] Emergency flow works (if applicable)

### Operational Readiness

- [ ] Business owner has admin access
- [ ] Business owner trained on viewing leads
- [ ] Lead notification method configured
- [ ] Rollback plan documented (deactivate tenant if needed)

---

## 18. Go-Live & Maintenance

### Go-Live Steps

1. **Final review:** Walk through pre-launch checklist with customer
2. **Activate widget:** Ensure `Is Active = Yes` and widget is live
3. **First lead:** Monitor for the first real conversation
4. **Follow up:** Check that lead notification works
5. **Confirm:** Tell the customer they're live

### Ongoing Maintenance

| Task | Frequency | How |
|------|-----------|-----|
| Review lead quality | Daily | Admin panel → Leads |
| Check for wrong answers | Weekly | Review conversation transcripts |
| Update KB.MD | As needed | Upload new version, rebuild index |
| Review RAG quality | Monthly | RAG debug panel, test queries |
| Update pricing/info | When changes occur | Edit KB.MD, rebuild index |

### Content Refresh Triggers

Rebuild the RAG index when:
- Services change (new or removed)
- Pricing structure changes
- Business hours change
- Contact information changes
- Service areas change
- New FAQs emerge from real conversations

### Monitoring

| What to watch | Where | Action |
|---------------|-------|--------|
| Wrong answers | Conversation transcripts | Update KB.MD |
| No leads captured | Lead list | Check POLICY.MD lead rules |
| Widget not loading | Customer website | Check domain config, server status |
| Slow responses | Server logs | Check LLM API, increase timeout |

### Scaling Considerations

| Metric | Threshold | Action |
|--------|-----------|--------|
| Leads per day > 50 | High volume | Consider webhook automation |
| KB.MD > 200KB | Large knowledge base | Split into focused sections |
| Response time > 5s | Slow | Check LLM model, reduce context |
| Multiple locations | Multi-site | Consider separate tenants |

---

# Appendix A: Annotation Quick Reference

### KB.MD Annotations

| Annotation | Purpose | Example |
|------------|---------|---------|
| `@service: code, emergency: bool` | Service entry | `@service: water_damage, emergency: true` |
| `@exclusion: code` | Service not offered | `@exclusion: general_plumbing` |
| `@coverage: include` | Service area section | `@coverage: include` |
| `@coverage: exclude` | Excluded area section | `@coverage: exclude` |
| `@faq: category` | FAQ entry | `@faq: pricing` |
| `@safety: type, triggers: list` | Safety procedure | `@safety: water_emergency, triggers: flood` |
| `@section: name` | Logical section | `@section: company_overview` |

### POLICY.MD Annotations

| Annotation | Purpose | Example |
|------------|---------|---------|
| `@identity` | Agent persona | `@identity` |
| `@intent: name, category, urgency, action` | Intent definition | `@intent: emergency, category: emergency, urgency: 5, action: emergency_flow` |
| `@rule: name, priority: N` | Behavioral rule | `@rule: safety_first, priority: 1` |
| `@thresholds` | Urgency scoring | `@thresholds` |
| `@verification_rule: name, type, severity` | Response verification | `@verification_rule: contact_grounding, type: exact_match, severity: critical` |

###SCRIPT.MD

SCRIPT.MD uses plain markdown section headers (no annotations). Define stages as `## Stage: Name` headers with bullet-point instructions.

---

# Appendix B: Common Customer Patterns

### Pattern 1: Local Service Business

**Profile:** Plumber, electrician, landscaper, cleaning service
- Simple services, defined service area
- No emergency flow needed
- Lead capture is primary goal

**Templates:** KB_MINIMAL.MD + POLICY_MINIMAL.MD + SCRIPT_MINIMAL.MD
**Features:** RAG (if KB > 20KB), Lead Capture
**Widget:** IIFE script on existing website

---

### Pattern 2: Emergency Service Business

**Profile:** Restoration, HVAC, water extraction, fire damage
- 24/7 emergency response
- Complex intake process
- Urgency classification critical

**Templates:** KB.MD (full) + POLICY_TRANSACTIONAL.MD + SCRIPT_TRANSACTIONAL.MD
**Features:** RAG, Emergency Flow, Lead Capture, Observational Memory
**Widget:** IIFE script or iframe (depending on CMS)

---

### Pattern 3: E-Commerce / Product Catalog

**Profile:** Online retail, product-based business
- Product catalog instead of services
- Shipping, returns, order tracking FAQs
- No emergency flow

**Templates:** KB_ECOMMERCE.MD + POLICY_MINIMAL.MD + SCRIPT_MINIMAL.MD
**Features:** RAG (for large catalogs), Lead Capture (optional)
**Widget:** IIFE or iframe

---

### Pattern 4: Professional Services

**Profile:** Law firm, accounting, consulting, agency
- Appointment-based
- Complex qualification process
- High-value leads

**Templates:** KB_MINIMAL.MD (customized) + POLICY_TRANSACTIONAL.MD + SCRIPT_TRANSACTIONAL.MD
**Features:** RAG, Booking/Scheduling, Lead Capture
**Widget:** IIFE script

---

### Pattern 5: Multi-Location Business

**Profile:** Franchise, chain, multi-site operation
- Multiple service areas
- May need separate tenants per location
- Shared knowledge base possible

**Templates:** Per-tenant configuration
**Features:** RAG, Lead Capture, Webhook Integrations
**Widget:** Same embed code, different slug per location

---

# Appendix C: Troubleshooting Matrix

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| Widget not appearing | Wrong slug, tenant inactive, domain restriction | Check slug, verify `Is Active`, check allowed domains |
| Agent gives wrong answer | KB.MD incomplete or RAG not indexed | Update KB.MD, rebuild index |
| Agent hallucinates pricing | No "no price guessing" rule in POLICY.MD | Add rule, lower temperature |
| No leads captured | Lead capture rules missing in POLICY.MD | Add lead capture policy |
| Widget loads but no response | LLM API key invalid or quota exceeded | Check OpenRouter dashboard |
| Slow responses | Large context, slow model | Reduce KB size, switch to faster model |
| Agent can't find service | RAG index stale or service not in KB.MD | Rebuild index, verify KB.MD content |
| Agent says wrong phone number | Phone number placeholder not replaced | Search KB.MD for placeholders, replace |
| Widget broken on mobile | CSS conflict, panel too small | Check responsive styles, adjust panel size |
| CORS errors | Server not configured for widget domain | Check server CORS settings |
| Simulation fails | API key issue, tenant misconfigured | Check API key, verify tenant settings |

---

# Appendix D: Glossary

| Term | Definition |
|------|-----------|
| **Tenant** | One business/client. All data is scoped to a tenant. |
| **Slug** | URL-safe tenant identifier (e.g., `acme-corp`). Used in embed URLs. |
| **KB.MD** | Markdown knowledge base. The agent's factual memory. |
| **POLICY.MD** | Agent identity, rules, intents, thresholds, and escalation behavior. |
| **SCRIPT.MD** | Conversation flow stages. Optional but recommended for transactional agents. |
| **RAG** | Retrieval-Augmented Generation. Agent searches relevant KB chunks before answering. |
| **LanceDB** | Embedded vector database for RAG storage. |
| **Embedding** | Numerical vector representation of text for similarity search. |
| **Hybrid Search** | 70% vector similarity + 30% BM25 keyword matching. |
| **Observational Memory (OM)** | Long-term memory via conversation compression. |
| **Lead** | Captured prospect with contact info and conversation context. |
| **Simulation** | Virtual conversation test using LLM-powered simulated customer. |
| **SSE** | Server-Sent Events. How the agent streams responses in real-time. |
| **IIFE** | Immediately Invoked Function Expression. The `<script>` embed method. |
| **Guid** | Globally unique identifier for tenant widget authentication. |

---

*End of DOCS_ONBOARDING.md*
