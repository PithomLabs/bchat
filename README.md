# DOCS_HOWTO_BIZ.md — Business Owner Guide to bchat

> **Audience:** Business owners, operators, and Pithom Labs deployers who want to set up a working AI chat agent on a website — from zero to receiving qualified leads.
>
> **Scope:** This document covers understanding the platform, writing knowledge-base and policy content, deploying the widget, capturing leads, and operating the admin surface. It does **not** replace developer documentation; see `docs/DOCS_README.MD` and `AGENTS.md` for technical details.

---

## Table of Contents

1. [What Is bchat?](#1-what-is-bchat)
2. [What It Does For Your Business](#2-what-it-does-for-your-business)
3. [The Business Model](#3-the-business-model)
4. [Architecture Overview (Business View)](#4-architecture-overview-business-view)
5. [Getting Started as a Deployer](#5-getting-started-as-a-deployer)
6. [Setting Up Your First Tenant](#6-setting-up-your-first-tenant)
7. [How to Write a High-Impact KB.md](#7-how-to-write-a-high-impact-kbmd)
8. [POLICY.md — Business Rules, Do's and Don'ts](#8-policymd--business-rules-dos-and-donts)
9. [SCRIPT.md — Conversation Flow](#9-scriptmd--conversation-flow)
10. [Deploying the Chat Widget](#10-deploying-the-chat-widget)
11. [Lead Capture Workflow](#11-lead-capture-workflow)
12. [The Admin Panel](#12-the-admin-panel)
13. [RAG Knowledge Base Management](#13-rag-knowledge-base-management)
14. [Testing Before Going Live](#14-testing-before-going-live)
15. [Common Business Workflows](#15-common-business-workflows)
16. [Policy & Legal Guardrails](#16-policy--legal-guardrails)
17. [What NOT To Do (Anti-Patterns)](#17-what-not-to-do-anti-patterns)
18. [Pricing Model](#18-pricing-model)
19. [Troubleshooting](#19-troubleshooting)

---

## 1. What Is bchat?

**bchat** is a multi-tenant AI chat agent platform. In plain language: it is a single software system that can power an unlimited number of independent AI chat agents — one per business — without any code changes between businesses.

Each business (called a **tenant**) gets:

- Its own **AI chat agent** trained on that business's knowledge
- Its own **widget** (chat bubble + panel) that can be embedded on any website
- Its own **lead capture** pipeline that extracts contact info from conversations
- Its own **admin dashboard** to review leads, test the agent, and manage knowledge content
- Full **data isolation** — no business can see another business's data

The system is built on:

| Layer | Technology |
|-------|-----------|
| Backend | Go + Echo (fast, reliable API server) |
| Database | SQLite (dev/local) / PostgreSQL (production SaaS) |
| Frontend | React + TypeScript |
| AI / LLM | OpenRouter (access to GPT-4o, Claude, Gemini, etc.) |
| Vector Search | LanceDB (RAG — retrieval-augmented generation) |
| Embeddings | OpenAI `text-embedding-3-small` (or local / mock) |

---

## 2. What It Does For Your Business

As a business owner or agency, bchat solves these problems:

### 24/7 Lead Capture
A visitor who lands on your client's website at 10 PM gets an immediate, intelligent response — not a voicemail. The agent answers questions, qualifies the prospect, and captures their contact details.

### No More "I'll Get Back To You"
The agent handles first-contact qualification so the owner doesn't have to. Leads arrive pre-qualified with name, email/phone, service needed, and urgency level.

### Knowledge-at-Scale
The agent knows everything about the business — services, pricing, coverage areas, exclusions, FAQs, emergency protocols — because it reads from the KB.MD/POLICY.MD files. Update the files, update the agent instantly. No retraining, no redeployment.

### Multi-Channel Follow-Up (Roadmap)
Email digests, SMS notifications, magic links for offline follow-up — all routed through a durable outbox system so no lead slips through.

### Professional Credibility
A branded, always-on chat widget signals that the business is modern and responsive. The agent's tone, voice, and rules are set by the business owner.

---

## 3. The Business Model

Pithom Labs' recommended model: **"AI Lead Desk for Local Service Businesses."**

### Three Tiers

| Package | Target | Setup | Monthly | Includes |
|---------|--------|-------|---------|----------|
| **Chat Widget Only** | Business already has a website | $99–$299 | $149–$299 | Widget install, FAQ training, lead capture form, email/SMS alerts, human takeover, weekly report |
| **Hugo Site + AI Lead Desk** | No website / bad website / Facebook-only | $399–$999 | $199–$499 | Fast landing site, domain/DNS help, service pages, quote form, widget, notification workflow |
| **Managed Lead Desk** | Growing SMBs that want white-glove | $499–$1,500 setup-equivalent | $499–$1,500 | Everything above + CRM or Google Sheets pipeline, appointment routing, missed-lead follow-up, conversation review, monthly optimization, owner dashboard |

### Positioning Statement
> "You keep doing the work. We handle the digital front desk."

Don't say "we sell AI chatbot software." Say "we make sure your website never ignores a prospect again."

### First-Market Vertical
Start with one: pressure washing, junk removal, cleaning, landscaping, or pest control. These businesses:
- Have high-intent, urgent leads
- Are often in the field (can't answer phones)
- Have simple knowledge bases
-don't have heavy compliance overhead

---

## 4. Architecture Overview (Business View)

```
┌─────────────────────────────────────────────────────────────┐
│                    YOUR CLIENT'S WEBSITE                     │
│  ┌──────────────────────────────────────────────────────┐   │
│  │           bchat Embed Widget (chat bubble)             │   │
│  │  • Floating button (bottom-right or inline)            │   │
│  │  • Expands into full chat panel                        │   │
│  │  • Visitor sends a message → agent responds            │   │
│  │  • Agent collects name, email, phone, service needed   │   │
│  └──────────────────────────────────────────────────────┘   │
└───────────────────────┬─────────────────────────────────────┘
                        │ HTTPS (SSE streaming)
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                   BCHAT SERVER (Yours / Pithom's)            │
│                                                              │
│  ┌────────────┐  ┌──────────────┐  ┌────────────────────┐   │
│  │  Tenant    │  │  RAG Engine  │  │   Lead Capture     │   │
│  │  Registry  │  │  (LanceDB)   │  │   Pipeline         │   │
│  └────────────┘  └──────────────┘  └────────────────────┘   │
│                                                              │
│  Each tenant has:                                           │
│  • KB.MD     — knowledge base (facts, services, FAQs)       │
│  • POLICY.MD — agent rules, identity, tone, thresholds      │
│  • SCRIPT.MD — conversation stages & flow                   │
│                                                              │
│  OpenRouter (GPT-4o, Claude, Gemini, etc.)                  │
└─────────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                   YOUR ADMIN PANEL                           │
│  • Upload KB/Policy/Script files per tenant                  │
│  • Review leads (name, email, phone, topic, status)          │
│  • Export leads as CSV                                      │
│  • Test agent via simulation sandbox                        │
│  • View RAG search debug (what chunks were retrieved)        │
│  • Manage users & permissions                               │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. Getting Started as a Deployer

### Prerequisites

- A server (Fly.io, VPS, or local machine)
- Go 1.21+ (for building backend)
- Node.js 18+ (for building frontend/widget)
- An OpenRouter account + API key (`sk-or-v1-...`)
- A domain name (for the admin panel and widget delivery)

### Quick Start (Local Development)

```bash
# Clone repo
git clone <your-bchat-repo>
cd bchat

# Install dependencies
task setup

# Set your OpenRouter API key
export OPENROUTER_API_KEY=sk-or-v1-your-key-here

# Build (without RAG, for quick start)
task build

# Run the server
./build/memos --mode dev --data build/data
```

The app starts at `http://localhost:8081`. Admin UI is at `/agent-admin`.

### Quick Start (With RAG)

RAG = the agent searches your KB.MD for relevant info before answering. Strongly recommended for production.

```bash
# Build with LanceDB native support
task build:rag

# Run with RAG enabled
task run:rag

# Or use mock embeddings (no API key needed for testing)
task run:rag:mock
```

### Quick Start (Production on Fly.io)

```bash
# Set Fly.io tokens
export FLY_API_TOKEN=...
export FLY_ORG=...

# Deploy
fly launch
fly deploy
```

See `docs/DOCS_DEPLOY_FLY.MD` for full production deployment steps.

---

## 6. Setting Up Your First Tenant

A **tenant** = one business. Each tenant is completely isolated.

### Step 1: Create a Tenant

1. Log in to `http://your-server:8081/agent-admin`
2. Go to **Tenants** page
3. Click **Create Tenant**
4. Fill in:

| Field | Example | Notes |
|-------|---------|-------|
| **Slug** | `acme-pressure-washing` | URL-safe; used in widget embed URL |
| **Company Name** | `Acme Pressure Washing` | Displayed in chat header |
| **Vertical** | `pressure_washing` | For your reference (not used in code) |
| **LLM Model** | `openai/gpt-4o-mini` | Override per tenant; falls back to `LLM_MODEL` env var |
| **Temperature** | `0.3–0.7` | 0 = very factual; 1 = very creative |
| **Allowed Domains** | `acmepw.com, acmepw.net` | Restrict widget embedding; blank = all domains allowed |
| **Is Active** | Yes/No | Toggle to pause/activate the tenant |

5. Click **Create** — you now have a tenant.

### Step 2: Get Your Embed URL

The tenant slug is your tenant's unique identifier. Your embed URL is:

```
https://your-server.com/widget/<slug>/embed.js
# or
https://your-server.com/api/v1/agent/<slug>/widget.js
```

### Step 3: Upload KB.MD, POLICY.MD, SCRIPT.MD

Go to the tenant's detail page in Agent Admin and upload the three markdown files (described in detail below).

**Critical:** Uploading files stores them but does NOT auto-rebuild the RAG index. After uploading, click **Rebuild Index** to make the content searchable.

---

## 7. How to Write a High-Impact KB.md

**KB.MD = the agent's brain.** It contains every fact about the business that the agent can reference when answering visitor questions. It is the single most important file you will write.

### Philosophy

Write KB.MD for a **smart, friendly new hire** — someone who needs context to represent the business accurately. The agent reads the whole file (or chunks from it via RAG) before answering. Be comprehensive but organized.

### Structure Template

```markdown
<!-- @section: company_overview -->
## About [Business Name]

- Founded: [Year]
- Owner: [Name]
- Headquarters: [City, State]
- Service Area: [Primary cities, counties, radius]
- Years in Business: [N]
- License/Insurance: [Details if relevant]

---

<!-- @section: services -->
## Services We Offer

### <!-- @service: service_code_1, emergency: true/false, response_time: "60 minutes" -->
#### [Service Name]
[2–4 sentences describing what this service covers, when it's needed,
and what the customer can expect.]

**Typical causes:** [Bullet list of common triggers]
**What's included:** [Bullet list of deliverables]
**Response time:** [e.g., "60 minutes or less for emergencies"]

---

### <!-- @service: service_code_2, emergency: false -->
#### [Second Service]
...

---

<!-- @section: exclusions -->
## Services We Don't Provide

<!-- @exclusion: code, exception: "secondary to X" -->
### [Service Not Offered]
We don't provide standalone [service]. [One sentence explanation + referral if available].

---

<!-- @section: coverage -->
## Service Areas

<!-- @coverage: include -->
### Areas We Serve
**Primary:** [City1], [City2], [City3]
**Secondary:** [City4], [City5] (additional travel fee may apply)

<!-- @coverage: exclude -->
### Areas We Don't Serve
[City/Region] — [Reason, if relevant]

---

<!-- @section: emergency_protocol -->
## Emergency Protocol

<!-- @intent: emergency, urgency: immediate -->
### How Emergencies Are Handled
1. Caller/Victim reports the issue
2. Agent collects: name, phone, address, nature of emergency
3. Dispatch team is notified within [N] minutes
4. Crew arrives within [time window]
5. Assessment is free; work begins after approval

**After-hours emergency line:** [Phone]

---

<!-- @section: pricing -->
## Pricing Information

<!-- @faq: pricing -->
### How much does [service] cost?
[Honest, helpful answer that sets expectations without quoting fixed prices
if they vary. Include factors that affect cost.]

Example:
> Costs vary based on [factors]. For a pressure washing job, we consider
> square footage, surface type, level of staining, and accessibility.
> We offer free estimates so you know the exact cost before we begin.

**Do not quote exact prices unless they are fixed and current.**

---

<!-- @section: faqs -->
## Frequently Asked Questions

<!-- @faq: hours -->
### What are your hours?
[Clear answer about business hours + emergency availability]

<!-- @faq: estimate -->
### Do you offer free estimates?
[Yes/No + details]

<!-- @faq: payment -->
### What payment methods do you accept?
[List: credit cards, checks, cash, financing options]

<!-- @faq: insurance -->
### Are you licensed and insured?
[Details about licensing, insurance coverage, bonding]

---

<!-- @section: process -->
## What To Expect

### Step 1: Contact
Call, text, or use this chat. We respond within [timeframe].

### Step 2: Assessment
We assess the situation (free for most services).

### Step 3: Quote
You get a clear, written quote before work begins.

### Step 4: Service
We complete the work on the agreed schedule.

### Step 5: Follow-Up
We confirm satisfaction and document any warranty info.
```

### Annotation System

The `<!-- @type: value -->` annotations make content machine-readable for the RAG system. Here are the ones you will use most:

#### KB.MD Annotations

| Annotation | Purpose | Example |
|------------|---------|---------|
| `@service: code, emergency: true, response_time: "60min"` | Mark a service entry | `@service: water_extraction, emergency: true` |
| `@exclusion: code, exception: "secondary to X"` | Mark a service NOT offered | `@exclusion: mold_remediation` |
| `@coverage: include/exclude` | Mark service area sections | `@coverage: include` |
| `@faq: category` | Mark FAQ entries | `@faq: pricing` |
| `@section: name` | Mark logical document sections | `@section: emergency_protocol` |
| `@safety: hazard_type` | Mark safety-related content | `@safety: electrical_hazard` |

### Writing Rules

**DO:**
- Write every service, exclusion, coverage area, and FAQ precisely
- Use bullet points — the agent handles structured content better than long paragraphs
- Include every phone number, email, address, and emergency contact
- Name specific cities, ZIP codes, neighborhoods
- Specify exact time windows ("within 60 minutes" not "fast response")
- Include pricing structure and factors that affect cost
- List accepted payment methods
- State licensing, insurance, bonding status
- Note warranty/guarantee terms
- Include any permitting requirements

**DON'T:**
- Write vague content ("we do great work at fair prices")
- Use competitor names or reference competitor pricing
- Include content from other businesses (even template "fill-in-the-blanks" that's not been customized)
- Leave placeholder text like "[Your Company Name]"
- Include personal/private information about the owner
- Promise things the business doesn't actually deliver
- Quote exact prices that change frequently without noting "starting at" or "varies"

### Content Quality Checklist

Before publishing a KB.MD, verify:
- [ ] Every service the business offers has a `<!-- @service -->` entry
- [ ] Every major service NOT offered has an `<!-- @exclusion -->` entry
- [ ] All service areas (cities, ZIPs, counties) are listed with **include** or **exclude**
- [ ] All phone numbers, emails, and physical addresses are current
- [ ] Emergency protocol includes after-hours contact
- [ ] Pricing section sets expectations without false specific quotes
- [ ] FAQs cover: hours, estimates, payment, insurance, scheduling, cancellation
- [ ] No placeholder text like `[fill in]` or `[company name]`
- [ ] File is named exactly `KB.MD` (case-insensitive) when uploaded

---

## 8. POLICY.md — Business Rules, Do's and Don'ts

**POLICY.MD = the agent's operating system.** It defines who the agent is, how it behaves, what it can and cannot do, and how it handles edge cases.

### Structure Template

```markdown
<!-- @identity, tone: professional_empathetic, voice: first_person_plural -->
## Identity

- **Name:** [Agent Name — or use business name]
- **Role:** Customer Service Representative for [Business Name]
- **Tone:** [Professional and warm / Urgent and direct / Casual and friendly]
- **Voice:** First-person plural ("we") or third-person ("the team")
- **Escalation contact:** [Name/role, phone, email]
- **Business hours:** [e.g., "Monday–Saturday, 7 AM – 7 PM"]
- **After-hours handling:** [e.g., "Collect lead, promise morning callback"]

---

## Behavioral Rules

<!-- @rule: id, priority: high, condition: always -->
1. **Never** make up pricing. If not in KB.MD, say so and offer an estimate.
2. **Never** promise availability or a specific crew without checking with dispatch.
3. **Always** collect contact info before transferring to human (if applicable).
4. **Always** treat the customer with respect, even if they're frustrated.
5. **Never** share internal business information (costs, margins, employee issues).
6. **If** the visitor asks about a competitor, redirect gracefully to [Business Name]'s strengths.
7. **If** the visitor asks about something outside our services, say so and offer alternative suggestions from KB.MD exclusions.
8. **Never** pretend to be human if the user directly asks "are you a bot/robot/AI?"

---

## Intents (Trigger Conditions)

<!-- @intent: emergency, urgency_threshold: 75, priority: P0 -->
### Emergency
A customer reports urgent, time-sensitive damage or safety issue.

**Agent behavior:**
- Acknowledge urgency immediately
- Collect: name, phone, location, nature of emergency
- Confirm dispatch within stated response window
- Transfer to human immediately

---

<!-- @intent: quote_request, priority: P1 -->
### Quote Request
A customer wants pricing or an estimate.

**Agent behavior:**
- Acknowledge request
- Collect: name, contact, service needed, location, timeframe
- Explain what determines pricing (per KB.MD)
- NEVER give exact figure unless listed in KB.MD as fixed-price
- Set expectation: "We'll follow up with a detailed quote"

---

<!-- @intent: schedule_service, priority: P1 -->
### Schedule Service
A customer wants to book an appointment.

**Agent behavior:**
- Check availability framework (if configured)
- Collect preferred date/time
- Confirm or offer alternatives
- Send confirmation details

---

<!-- @intent: escalation, priority: P0 -->
### Escalation
A customer explicitly asks for a human, complains, or uses escalation keywords.

**Trigger keywords:** "manager", "supervisor", "lawyer", "complain", "unsatisfied", "human", "real person"

**Agent behavior:**
- Acknowledge frustration
- Collect: name, contact, brief summary of issue
- Confirm: "I'm connecting you with [Name]. They'll reach you within [timeframe]."
- Pause chat; do not continue resolving on behalf of human without instruction

---

## Thresholds

<!-- @threshold: urgency_emergency, value: 75 -->
### Urgency Scoring

| Score Range | Classification | Agent Response |
|-------------|---------------|----------------|
| 75–100 | **EMERGENCY** | Immediate acknowledgment, dispatch within response window |
| 40–74 | **URGENT** | Priority response within 1 hour during business hours |
| 1–39 | **STANDARD** | Response within normal service window |
| 0 | **INFORMATIONAL** | Standard informational response, no urgency flags |

---

## Fallback Rules

<!-- @rule: fallback, priority: medium -->
### When the Agent Doesn't Know

If the agent cannot answer a question using KB.MD or POLICY.MD:

1. **Say so honestly. Not** "I think so" or hedging guesses.
2. **Offer an alternative:** "I don't have that information in my knowledge base, but I can [collect your contact info and have someone follow up / connect you with our team]."
3. **Transition to lead capture** — ask for name, contact, and question so a human can respond.

This is called the **"Say Unknown + Capture"** behavior. It protects trust and turns unknowns into leads.

---

## Lead Capture Policy

<!-- @rule: lead_capture -->
### When to Ask for Contact Details

The agent should collect contact info when:
- The visitor asks for a quote or estimate
- The visitor asks to schedule or book
- The visitor reports an emergency
- The visitor explicitly provides info (agent confirms and stores)
- The conversation reaches a natural close after a substantive inquiry

The agent should NOT aggressively collect contact info:
- On the very first message before understanding the need
- If the visitor only says "hi" or "hello"
- If the visitor asks a simple FAQ that is resolved in KB.MD

### Minimum Required Lead Fields
- **Name** OR **Phone** (at least one)
- **Email** OR **Phone** (at least one)
- **Service/need** (extracted from conversation)
- **Location** (if service is location-dependent)
- **Urgency level** (from urgency scoring above)

### Decline Handling
If the visitor says "I'd rather not share" or similar:
- Accept gracefully: "No problem — feel free to reach out anytime."
- Mark the lead as `declined: true` so the system doesn't re-prompt
- Do NOT block the conversation; continue helping

---

## Human Takeover / Escalation

<!-- @rule: human_takeover -->
### When to Hand Off to a Human

The agent should trigger human takeover when:
- The visitor explicitly requests a human
- Urgency score = 75+ (emergency)
- The conversation is clearly frustrated/escalating
- A question requires legal/contractual/liability decisions
- The visitor says "I need to speak to someone"

### After Human Takeover
- Show a transition message: "I'm connecting you with [Name] from our team. They'll respond within [timeframe]."
- Create a system notification/email for the human agent
- Do NOT continue resolving the issue autonomously
- Optionally create a support ticket
```

### POLICY.md Rules of Thumb

**DO:**
- Define the agent's identity as clearly as you would a new employee's job description
- Specify exact trigger words for escalation
- Set urgency score thresholds that match the business's actual emergency response times
- Include after-hours behavior explicitly
- Define what happens when the agent doesn't know the answer
- Include anti-hallucination rules: "Never guess pricing, availability, or policy"

**DON'T:**
- Write POLICY.md as if the agent is a human — it is an AI assistant representing the business
- Use vague rules ("be polite") without defining what that means operationally
- Forget to define escalation behavior — this is the #1 cause of bad customer experiences
- Allow the agent to negotiate pricing, promises, or guarantees

---

## 9. SCRIPT.md — Conversation Flow

**SCRIPT.MD = the agent's playbook.** It defines the stages of a conversation and what happens at each stage.

### Template

```markdown
## Stage: Opening
- Greet the visitor warmly
- State who you represent: "I'm the [Business Name] assistant"
- Ask how you can help today
- DO NOT collect contact info yet — understand the need first

## Stage: Discovery
- Ask clarifying questions to understand the service needed
- Confirm location/service area if relevant
- Determine urgency (emergency vs. standard)

## Stage: Information
- Answer questions using KB.MD
- If answer not in KB.MD → use fallback rule
- Be specific, not vague

## Stage: Qualification
- Once service type is understood and visitor shows intent:
  - Collect: name, email/phone, service needed, location, timeframe
- Confirm: "I have your details — [Name] will follow up within [timeframe]"

## Stage: Closing
- Confirm next step
- Provide any reference numbers or expectation-setting
- Thank the visitor
- Invite them to reach out again
```

### Key Principle

The agent should never ask for contact details before it understands what the visitor needs. The conversation should feel natural: the agent is helping first, collecting second.

---

## 10. Deploying the Chat Widget

The widget is the visible chat interface your website visitors interact with. There are three embedding approaches:

### Option 1: JavaScript Embed (Recommended)

Best for custom websites, full control, floating bubble + expandable panel.

```html
<!DOCTYPE html>
<html>
<head>
  <title>Acme Pressure Washing</title>
</head>
<body>
  <!-- Your existing website content -->
  <h1>Welcome to Acme Pressure Washing</h1>
  <p>We serve the greater Phoenix area...</p>

  <!-- bchat widget -->
  <script
    src="https://your-server.com/widget/acme-pressure-washing/embed.js"
    data-position="bottom-right"
    data-color="#0d9488"
    data-welcome="Hi! How can we help with your pressure washing needs today?"
  ></script>
</body>
</html>
```

**Configuration options:**

| Attribute | Description | Example |
|-----------|-------------|---------|
| `data-position` | Button position | `bottom-right`, `bottom-left` |
| `data-color` | Brand color (hex) | `#0d9488` |
| `data-welcome` | Initial greeting message | `"Hi! How can we help?"` |

### Option 2: iframe Embed

Best for WordPress, Wix, Shopify, or any CMS where you can paste an iframe code.

```html
<iframe
  src="https://your-server.com/widget/acme-pressure-washing/iframe?color=%230d9488"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat with Acme Pressure Washing"
></iframe>
```

### Option 3: Legacy Script (Backward Compatible)

```html
<script src="https://your-server.com/api/v1/agent/acme-pressure-washing/widget.js"></script>
```

### Domain Restrictions

For security, you can restrict which domains can embed your widget:

In the tenant settings, set `Allowed Domains` to:
```
acmepw.com, www.acmepw.com, acmepw.net
```

Leave blank to allow embedding on any domain.

### Widget Behavior at Runtime

When a visitor opens the widget:
1. Widget loads tenant-specific config (slug, company name, color)
2. Widget opens a new conversation session
3. Visitor sends a message → goes to `POST /api/v1/agent/:slug/chat`
4. Backend:
   - Looks up tenant by slug
   - Parses KB.MD / POLICY.MD / SCRIPT.MD
   - Optionally runs RAG search to find relevant chunks
   - Builds system prompt
   - Calls OpenRouter LLM
   - Returns streaming SSE response
5. Agent response renders in widget
6. If agent detects lead info → stores in `agent_leads` table
7. Business owner sees lead in admin panel

---

## 11. Lead Capture Workflow

Lead capture is automatic and conversational — no forms required.

### How It Works

1. **Conversational extraction:** The agent asks for contact details naturally during the conversation (not on first message).
2. **Multi-message accumulation:** The agent collects name, email, phone, and other fields across multiple turns.
3. **Intelligent merging:** A three-layer extraction pipeline combines regex, structural analysis, and AI extraction.
4. **Confidence scoring:** Each field has a confidence score; high-confidence leads are committed immediately.
5. **Correction handling:** "No, I meant john2000@email.com" replaces the previous email.
6. **Decline handling:** "I'd rather not share" sets `declined: true` and stops re-prompting.

### Lead Fields

| Field | Required | Source |
|-------|----------|--------|
| `tenant_id` | Always | From session context |
| `session_id` | Always | From chat session |
| `customer_name` | At least one of name/email/phone | Extracted from conversation |
| `customer_email` | At least one of name/email/phone | Extracted from conversation |
| `customer_phone` | At least one of name/email/phone | Extracted from conversation |
| `customer_location` | Optional | Extracted from context |
| `service_interest` | Optional | From RAG context or conversation |
| `detected_intent` | Optional | Intent classification (emergency/quote/schedule/etc.) |
| `urgency_level` | Optional | Urgency score from POLICY.MD thresholds |
| `transcript_summary` | Auto | Brief summary of conversation |
| `status` | Default: `new` | new → contacted → converted → lost |
| `extraction_metadata` | Auto | JSON with confidence scores, extraction method |

### Viewing Leads

1. Go to **Agent Admin → [Tenant] → Leads**
2. See list of all leads with: Name, Email, Phone, Service, Status, Created At
3. Filter by status: new, contacted, converted, lost
4. Click a lead to see full transcript
5. Export as CSV for CRM import

### Lead Notification (Admin)

Leads are visible in the admin panel. For email/SMS notification:
- The platform captures leads locally
- You (or Pithom Labs) implement a notification bridge using the admin API or outbox system
- Email digests can be batched and sent on a schedule

---

## 12. The Admin Panel

Access at: `https://your-server.com/agent-admin`

### Sections

| Section | What It Does |
|---------|-------------|
| **Tenants** | List, create, edit, activate/deactivate businesses |
| **Tenant Detail** | Upload KB.MD, POLICY.MD, SCRIPT.MD; configure LLM model; manage users |
| **Leads** | View, filter, export leads for a tenant |
| **RAG Debug** | Test what the vector search retrieves for any query (admin only) |
| **Simulations** | Run virtual customer conversations to test the agent before going live |
| **Transcripts** | View and export full conversation logs |

### Admin Permissions (RBAC)

| Permission | Access Level |
|------------|-------------|
| `tenant:admin` | Full tenant management — create, edit, delete tenants |
| `tenant:read` | View tenant configuration |
| `api:config` | Configure LLM settings, rebuild index |
| `chat:test` | Run simulations, view test chat history |
| `chat:logs` | View real chat session logs |
| `files:upload` | Upload KB.MD, POLICY.MD, SCRIPT.MD |

### Permissions Are Assigned Per User Per Tenant
When you add a team member, assign them a preset role:
- **Viewer** — read-only access
- **Tester** — can run simulations and view logs
- **Editor** — can upload files and edit content
- **Tenant Admin** — full control over that tenant

---

## 13. RAG Knowledge Base Management

**RAG (Retrieval-Augmented Generation)** is how the agent finds relevant knowledge before answering. Without RAG, the entire KB.MD is stuffed into the LLM prompt (works for small files, degrades for large ones). With RAG, only the most relevant chunks are retrieved and shown to the LLM.

### RAG Pipeline (Business View)

```
Visitor asks: "Do you handle mold remediation?"
         │
         ▼
[1] Agent sends query to embedding service → converts to vector
         │
         ▼
[2] Vector search in LanceDB → finds top 5 most relevant KB chunks
         │
         ▼
[3] Agent builds LLM prompt: "Here are the relevant facts: [chunks]. Now answer the visitor's question."
         │
         ▼
[4] LLM returns grounded, accurate answer
```

### Managing RAG

After uploading or updating KB.MD:

1. Click **Rebuild Index** in the tenant admin panel
2. System chunks the file, generates embeddings, stores in LanceDB
3. This takes 30–120 seconds depending on file size
4. Admin can see: total chunks indexed, last indexed timestamp, any errors

### Hybrid Search

RAG uses hybrid search by default:
- **70% vector similarity** (semantic meaning — finds related concepts)
- **30% BM25 keyword matching** (exact terms — finds exact service names, city names)

This means both "water removal" and "standing water extraction" return the same relevant chunk.

### What to Expect

| File Size | Chunks Created | Index Time |
|-----------|---------------|------------|
| 5 KB | ~10 | 10s |
| 20 KB | ~40 | 30s |
| 100 KB | ~200 | 90s |
| 500 KB | ~1000 | 4 min |

### RAG vs. Long Context

The system automatically chooses:
- **RAG mode** when KB content exceeds ~6,000 tokens (about 24 KB)
- **Long context mode** when content is smaller (everything goes in the prompt)

This is handled automatically. You don't need to configure anything.

---

## 14. Testing Before Going Live

### Simulation Testing

Before exposing the widget to real visitors:

1. Go to **Agent Admin → [Tenant] → Simulations**
2. Configure a virtual customer persona:
   - **Name:** John Doe
   - **Scenario:** Needs emergency water extraction in Phoenix
   - **Goal:** Get a quote and schedule service ASAP
   - **Tone:** Urgent, slightly anxious
3. Click **Run Simulation**
4. Watch the conversation unfold in real-time (SSE streaming)
5. Review transcript:
   - Did the agent ask the right qualifying questions?
   - Did it capture name, email, phone?
   - Did it handle the urgency correctly?
   - Did it hallucinate any information?
   - Did it follow POLICY.MD rules?

### RAG Debug Testing

1. Go to **Agent Admin → [Tenant] → RAG Debug**
2. Type a test query: "emergency water extraction in Phoenix"
3. See what chunks were retrieved, with relevance scores
4. Verify: Are the right services showing up? Are exclusions being respected?

### Recommended Pre-Launch Checklist

- [ ] KB.MD has been reviewed by business owner (not just copy-pasted)
- [ ] POLICY.MD covers: identity, escalation, urgency thresholds, fallback behavior
- [ ] SCRIPT.MD defines clear conversation stages
- [ ] All phone numbers, emails, and service areas are verified
- [ ] RAG index rebuilt after any file upload
- [ ] At least 3 simulations run and passed
- [ ] RAG debug returns correct chunks for 10+ test queries
- [ ] Lead capture tested: name + email/phone extracted correctly
- [ ] Escalation workflow tested: "I want to speak to a manager" → proper handoff
- [ ] Widget embedded on staging/test website
- [ ] Widget looks correct on mobile
- [ ] Domain restrictions set (if applicable)
- [ ] Business owner has admin access and knows how to view leads

---

## 15. Common Business Workflows

### Workflow 1: After-Hours Lead
```
22:00 — Visitor lands on website, asks about emergency water extraction
22:00 — Agent responds immediately (no human needed)
22:01 — Agent collects: name (Sarah), phone (555-0142), address (Phoenix), nature of emergency (bathroom flooding)
22:02 — Agent says: "I've sent this to our on-call team. Someone will call you within 30 minutes."
22:03 — Lead created with urgency_score = emergency, status = new
22:03 — (Optional) SMS/email alert sent to on-call technician
```

### Workflow 2: Informational Inquiry Becomes Lead
```
14:00 — Visitor: "Do you handle mold remediation?"
14:00 — Agent: "We don't provide standalone mold remediation, but if the mold is
           secondary to water damage, we can help with the water extraction first.
           Would you like to schedule an assessment?"
14:02 — Visitor: "Yes, that sounds right"
14:02 — Agent collects contact details
14:03 — Lead captured with service = water_extraction, notes = "mold secondary"
```

### Workflow 3: Self-Service FAQ (No Lead)
```
10:00 — Visitor: "What are your business hours?"
10:00 — Agent: "We're open Monday–Saturday, 7 AM – 7 PM. For emergencies,
           our 24/7 line is 555-0199."
10:00 — Lead NOT created (no contact info, no service interest)
```

### Workflow 4: Escalation
```
16:00 — Visitor: "This is the third time I've called and nobody shows up"
16:00 — Agent detects escalation keywords + high urgency
16:00 — Agent: "I understand your frustration. I'm connecting you with [Manager Name]
           right now. They'll call you within 15 minutes."
16:00 — Agent pauses chat, creates internal alert, marks session for human follow-up
```

---

## 16. Policy & Legal Guardrails

### Compliance Notes

**CAN-SPAM (Email):**
- Commercial emails must have truthful headers, non-deceptive subject lines
- Must identify sender and include physical address
- Must provide opt-out mechanism
- Applies to lead follow-up emails sent by your business

**TCPA (SMS/Text):**
- Consent is required for automated marketing text messages
- Must provide opt-out mechanism
- Consent revocation must be honored immediately
- Lead capture chat ≠ consent for SMS unless explicitly agreed

**State Privacy Laws (CCPA/CPRA/VCDPA):**
- California, Colorado, Virginia, and other states have consumer privacy laws
- You may need to disclose data collection in your privacy policy
- Visitors should understand their contact info will be stored for follow-up

### Recommended Legal Language

Add to your client's website near the chat widget:

> By using this chat, you agree that [Business Name] may collect your name, email, and phone number for the purpose of following up on your inquiry. We do not share your information with third parties. See our Privacy Policy for full details.

Add to your privacy policy:

> Contact Information: When you contact us through our website chat, we collect your name, email address, phone number, and the details of your inquiry. This information is used solely to respond to your request and is not shared with third parties without your consent.

---

## 17. What NOT To Do (Anti-Patterns)

These are the most common mistakes that destroy trust and waste money:

| Anti-Pattern | Why It's Bad | What To Do Instead |
|-------------|-------------|-------------------|
| **Copy-paste KB.MD from another business** | Agent gives wrong services, pricing, coverage areas | Write KB.MD from scratch for each business |
| **Vague service descriptions** | Agent can't answer "Do you do X?" accurately | Be specific: what's included, what's not, what triggers the service |
| **No exclusions section** | Agent promises services the business doesn't offer | Always list what's NOT included |
| **No after-hours policy** | Agent behaves as if business is 24/7 | Define after-hours behavior in POLICY.MD |
| **Hardcoded pricing in KB.MD** | Agent quotes stale or wrong prices | Describe pricing structure, not fixed numbers (unless truly fixed) |
| **Skipping simulation** | Agent has untested blind spots | Run at least 3 simulations per tenant before launch |
| **Ignoring RAG rebuilds** | Agent answers from stale or missing knowledge | Always rebuild index after KB.MD changes |
| **Over-engineering SCRIPT.MD** | Long scripts that LLM ignores | Keep SCRIPT.MD to clear stage names and bullet points |
| **Setting temperature > 0.8** | Agent becomes creative, hallucinates quotes/promises | 0.3–0.5 for local service businesses; 0.7 max for creative verticals |
| **Linking widget to wrong tenant slug** | Agent uses wrong knowledge base | Double-check the slug in the embed URL matches the tenant |
| **Leaving widget on broken tenant** | Visitor sees errors, trust lost | Always verify tenant is `Is Active = true` before deploying |
| **Not reviewing leads daily** | Leads go cold, expectation mismatch between agent promises and human follow-up | Set a daily reminder to review new leads |

---

## 18. Pricing Model

### Recommended Package Structure (Pithom Labs)

```text
Package 1 — Chat Widget Only
  For: Businesses with an existing website
  Setup: $149–$299
  Monthly: $149–$299
  Includes:
    • bchat widget installation
    • KB/Policy training for that business
    • Lead capture + email notification
    • Basic conversation transcript
    • Human takeover/escalation
    • Weekly lead report

Package 2 — Hugo Site + AI Lead Desk
  For: Businesses with no website or obsolete web presence
  Setup: $399–$999
  Monthly: $199–$499
  Includes:
    • Fast Hugo landing site
    • Domain/DNS configuration help
    • Service pages + quote/request form
    • bchat widget (included)
    • Lead notification workflow
    • Google Business Profile CTA optimization
    • Monthly updates

Package 3 — Managed Lead Desk
  For: Growing businesses that want full outsourcing
  Monthly: $499–$1,500
  Includes:
    • Everything from Package 1 or 2
    • CRM or Google Sheets lead pipeline
    • Appointment routing
    • Missed-lead follow-up sequences
    • Conversation quality review
    • Monthly lead optimization report
    • Owner-facing dashboard
```

### Internal Cost Structure (What It Costs You)

| Cost Component | Estimate | Notes |
|---------------|----------|-------|
| OpenRouter API | $5–$50/tenant/month | Depends on chat volume + model choice. GPT-4o-mini is cheap and fast. |
| Server hosting (Fly.io) | $15–$50/month | Small VM sufficient for 25–50 tenants |
| LanceDB storage | negligible | Stored on same disk |
| Domain + SSL | $1–$2/month | Per custom tenant domain if used |
| **Your total cost per tenant** | **$5–$30/month** | Margins of 80–90% on the $199–$499/month retail price |

---

## 19. Troubleshooting

### Agent doesn't know about a service

**Cause:** KB.MD doesn't contain that service, or RAG index hasn't been rebuilt.

**Fix:**
1. Check KB.MD has `<!-- @service: code, emergency: true/false -->` entry
2. In Admin panel → Rebuild Index
3. Test via RAG Debug panel

### Agent gives wrong answer (hallucination)

**Cause:** RAG not enabled, content not in KB, or temperature too high.

**Fix:**
1. Verify `RAG_PIPELINE_ENABLED=true` in environment
2. Add missing content to KB.MD
3. Rebuild index
4. Lower `temperature` to 0.3–0.5
5. Ensure POLICY.MD has the `@rule: fallback` section — agent should say "I don't have that information" rather than guessing

### Agent doesn't capture leads

**Cause:** Lead extraction regex doesn't match the input format.

**Fix:**
1. Check that the conversation actually contained name + email/phone
2. Verify the lead fields are being extracted from session messages
3. Check that `customer_name` and `customer_email` or `customer_phone` are populated
4. Test with known inputs: "My name is John Smith, my email is john@example.com"
5. Check `agent_leads` table: `SELECT * FROM agent_leads ORDER BY id DESC LIMIT 5;`

### Widget won't load on website

**Cause:** Wrong slug in embed URL, domain restriction blocking, or server not reachable.

**Fix:**
1. Verify embed URL: `https://your-server.com/widget/EXACT-SLUG/embed.js`
2. Check domain is in `Allowed Domains` in tenant settings
3. Check CORS if using script embed
4. Open browser DevTools → Console → check for errors
5. Verify `Is Active = true` for the tenant

### RAG index keeps failing to build

**Cause:** Embedding API issue, chunking error, or network timeout.

**Fix:**
1. Check `OPENROUTER_API_KEY` is set and valid
2. Check server logs: `grep "RAG" build/memos.log`
3. Increase `EMBEDDING_TIMEOUT` to 300s for large files
4. Verify LanceDB storage path exists and is writable
5. Try mock embeddings: `task run:rag:mock` (no API key needed)

### Leads not showing in admin panel

**Cause:** Tenant filter, status filter, or lead creation failure.

**Fix:**
1. Verify you are viewing leads for the correct tenant
2. Check `status` filter — default shows `new` only
3. Check `agent_leads` table directly in SQLite/PostgreSQL
4. Verify `captureLeadFromSession` is being called (check server logs)
5. Ensure lead extraction pipeline completed (check `extraction_metadata` JSON in lead record)

---

## Appendix A: Complete Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `OPENROUTER_API_KEY` | Yes | (none) | OpenRouter LLM access |
| `LLM_MODEL` | No | `openai/gpt-4o-mini` | Default chat model |
| `LLM_MODEL_REASONING` | No | `google/gemini-2.5-pro` | Content generation model |
| `RAG_PIPELINE_ENABLED` | No | `false` | Enable/disable RAG |
| `EMBEDDING_PROVIDER` | No | `openrouter` | `openrouter`, `local`, `mock` |
| `EMBEDDING_MODEL` | No | `text-embedding-3-small` | Embedding model name |
| `EMBEDDING_TIMEOUT` | No | `180s` | Embedding API timeout |
| `EMBEDDING_BATCH_SIZE` | No | `10` | Chunks per embedding batch |
| `LANCEDB_STORAGE_PROVIDER` | No | `local` | `local`, `s3` |
| `LANCEDB_LOCAL_PATH` | No | `build/data/lancedb` | Disk path for LanceDB |
| `LANCEDB_S3_BUCKET` | No | (none) | S3 bucket for LanceDB |
| `HYBRID_SEARCH_ENABLED` | No | `true` | Enable hybrid vector+BM25 search |
| `HYBRID_VECTOR_WEIGHT` | No | `0.7` | Vector similarity weight |
| `HYBRID_TEXT_WEIGHT` | No | `0.3` | BM25 keyword weight |
| `OM_ENABLED` | No | `false` | Observational Memory |
| `OM_OBSERVER_TOKEN_THRESHOLD` | No | `30000` | Token threshold for observer |
| `OM_TOKEN_THRESHOLD` | No | `2000` | Token threshold for reflector |
| `FORCE_REINDEX_ON_STARTUP` | No | `false` | Reindex all content at boot |
| `LLM_VERIFIER_ENABLED` | No | `false` | LLM response verification layer |
| `ENCRYPTION_MASTER_KEY` | No | (none) | Encrypt tenant API keys at rest |

**Configuration priority (highest → lowest):**
1. Tenant-specific setting (set in Admin UI)
2. Environment variable
3. Hardcoded default

---

## Appendix B: File Naming Conventions

When uploading files to a tenant:

| File Name | Purpose | Required |
|-----------|---------|----------|
| `KB.MD` | Knowledge base | Yes |
| `POLICY.MD` | Agent policy & rules | Yes |
| `SCRIPT.MD` | Conversation flow | No (recommended) |
| `KB_<epoch>.MD` | Versioned backup | Auto |
| `POLICY_<epoch>.MD` | Versioned backup | Auto |

**Important:** File names are case-insensitive in the upload system, but `KB.MD`, `POLICY.MD`, and `SCRIPT.MD` are the canonical names used in documentation.

---

## Appendix C: API Endpoints Reference

### Public (No Auth)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/v1/agent/:slug/chat` | Send a chat message |
| `GET` | `/api/v1/agent/:slug/chat/stream` | SSE stream of responses |
| `GET` | `/api/v1/agent/:slug/widget.js` | Legacy widget script |
| `GET` | `/widget/:slug/embed.js` | JavaScript embed |
| `GET` | `/widget/:slug/iframe` | iframe embed |

### Admin (Authenticated)

| Method | Path | Permission | Purpose |
|--------|------|-----------|---------|
| `GET` | `/api/v1/agent/tenants` | `tenant:admin` | List tenants |
| `POST` | `/api/v1/agent/tenants` | `tenant:admin` | Create tenant |
| `GET` | `/api/v1/agent/:slug` | `tenant:read` | Get tenant config |
| `PUT` | `/api/v1/agent/:slug` | `tenant:admin` | Update tenant |
| `POST` | `/api/v1/agent/:slug/files` | `files:upload` | Upload KB/Policy/Script |
| `POST` | `/api/v1/agent/:slug/reindex` | `api:config` | Rebuild RAG index |
| `POST` | `/api/v1/agent/:slug/simulate` | `chat:test` | Run simulation |
| `GET` | `/api/v1/agent/:slug/simulations` | `chat:test` | List simulations |
| `POST` | `/api/v1/agent/:slug/generate-kb` | `tenant:admin` | Auto-generate KB.MD |
| `POST` | `/api/v1/agent/:slug/generate-policy` | `tenant:admin` | Auto-generate POLICY.MD |
| `POST` | `/api/v1/agent/:slug/rag/search` | `api:config` | Test RAG search |
| `GET` | `/api/v1/agent/:slug/leads` | `tenant:admin` | List leads |
| `GET` | `/api/v1/agent/:slug/leads/:id` | `tenant:admin` | Lead detail |
| `GET` | `/api/v1/agent/:slug/leads/export` | `tenant:admin` | Export CSV |

---

## Appendix D: Key Concepts Glossary

| Term | Definition |
|------|-----------|
| **Tenant** | One business/client. All data (KB, leads, transcripts) is scoped to a tenant. |
| **Slug** | URL-safe tenant identifier (e.g., `acme-pressure-washing`). Used in embed URLs and API routes. |
| **KB.MD** | Markdown knowledge base. The agent's factual memory. |
| **POLICY.MD** | Markdown file defining agent identity, rules, intents, thresholds, and escalation behavior. |
| **SCRIPT.MD** | Markdown file defining conversation stages and flow. |
| **RAG** | Retrieval-Augmented Generation. The agent searches relevant KB chunks before LLM asks. |
| **LanceDB** | Embedded vector database used for RAG storage. |
| **Embedding** | Numerical vector representation of text, used for similarity search. |
| **SSE** | Server-Sent Events. How the agent streams responses in real-time. |
| **Lead** | A captured prospect (name, email, phone, service, urgency, transcript link). |
| **Audience** | `external` (visitors/chat widget) or `internal` (staff/employees). Configs are per audience. |
| **Idempotency** | Sending the same message twice produces the same stored result (no duplicates). |
| **Urgency Score** | 0–100 score classifying a conversation as informational, standard, urgent, or emergency. |
| **Simulation** | Virtual conversation test using an LLM-powered simulated customer. |
| **Hybrid Search** | 70% vector similarity + 30% keyword matching (BM25). |

---

*End of DOCS_HOWTO_BIZ.md*
