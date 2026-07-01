# DOCS_HOWTO_BIZ.md — Complete Business Owner's Guide to bchat

> **Audience:** Business owners, agency operators, and Pithom Labs deployers who want to set up a working AI chat agent on a website — from zero to receiving qualified leads.  
> **Goal:** Maximize ROI from bchat's AI-powered RAG-based knowledge base, lead capture, and chat widget functionality.

---

## Table of Contents

1. [What Is bchat?](#1-what-is-bchat)
2. [What It Does For Your Business](#2-what-it-does-for-your-business)
3. [The Business Model](#3-the-business-model)
4. [Architecture Overview (Business View)](#4-architecture-overview-business-view)
5. [Getting Started as a Deployer](#5-getting-started-as-a-deployer)
6. [Setting Up Your First Tenant](#6-setting-up-your-first-tenant)
7. [How to Write a High-Impact KB.MD](#7-how-to-write-a-high-impact-kbmd)
8. [POLICY.MD — Business Rules, Do's and Don'ts](#8-policymd--business-rules-dos-and-donts)
9. [SCRIPT.MD — Conversation Flow](#9-scriptmd--conversation-flow)
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
20. [Quick Reference Cheatsheets](#20-quick-reference-cheatsheets)

---

## 1. What Is bchat?

**bchat** is a multi-tenant AI chat agent platform. Think of it as a "digital front desk" that can be cloned and customized for each of your clients or your own business.

Each business (called a **tenant**) gets its own:
- **AI chat agent** trained on that business's knowledge
- **Widget** (chat bubble + panel) embeddable on any website
- **Lead capture** pipeline that extracts contact info from conversations
- **Admin dashboard** to review leads, test the agent, and manage knowledge content
- **Full data isolation** — no business can see another business's data

**Technology Stack:**
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

### 24/7 Lead Capture
A visitor who lands on your website at 10 PM gets an immediate, intelligent response — not a voicemail. The agent answers questions, qualifies the prospect, and captures their contact details.

### No More "I'll Get Back To You"
The agent handles first-contact qualification so you don't have to. Leads arrive pre-qualified with name, email/phone, service needed, and urgency level.

### Knowledge-at-Scale
The agent knows everything about your business — services, pricing, coverage areas, exclusions, FAQs, emergency protocols — because it reads from the KB.MD/POLICY.MD files. Update the files, update the agent instantly. No retraining, no redeployment.

### Professional Credibility
A branded, always-on chat widget signals that your business is modern and responsive. The agent's tone, voice, and rules are set by you.

---

## 3. The Business Model

**Recommended Model:** "AI Lead Desk for Local Service Businesses"

### Three Tiers

| Package | Target | Setup | Monthly | Includes |
|---------|--------|-------|---------|----------|
| **Chat Widget Only** | Business already has a website | $99–$299 | $149–$299 | Widget install, FAQ training, lead capture form, email/SMS alerts, human takeover, weekly report |
| **Hugo Site + AI Lead Desk** | No website / bad website / Facebook-only | $399–$999 | $199–$499 | Fast landing site, domain/DNS help, service pages, quote form, widget, notification workflow |
| **Managed Lead Desk** | Growing SMBs that want white-glove | $499–$1,500 setup-equivalent | $499–$1,500 | Everything above + CRM/Sheets pipeline, appointment routing, missed-lead follow-up, conversation review, monthly optimization, owner dashboard |

### Positioning Statement
> "You keep doing the work. We handle the digital front desk."

Don't say "we sell AI chatbot software." Say "we make sure your website never ignores a prospect again."

### First-Market Vertical
Start with one: pressure washing, junk removal, cleaning, landscaping, or pest control. These businesses:
- Have high-intent, urgent leads
- Are often in the field (can't answer phones)
- Have simple knowledge bases
- Don't have heavy compliance overhead

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
│  │  • Agent collects: name, email, phone, service needed   │   │
│  └──────────────────────────────────────────────────────┘   │
└───────────────────────┬─────────────────────────────────────┘
                        │ HTTPS (SSE streaming)
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                   BCHAT SERVER (Yours or Pithom's)         │
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
export OPENROUTER_API_KEY="sk-or-v1-..."

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
| **Vertical** | `pressure_washing` | For your reference |
| **LLM Model** | `openai/gpt-4o-mini` | Override per tenant; falls back to `LLM_MODEL` env var |
| **Temperature** | `0.3–0.7` | 0 = factual; 1 = creative |
| **Allowed Domains** | `acmepw.com, acmepw.net` | Restrict widget embedding; blank = all domains |
| **Is Active** | Yes/No | Toggle to pause/activate the tenant |

### Step 2: Get Your Embed URL

Your embed URL is:
```
https://your-server.com/widget/<slug>/embed.js
# or
https://your-server.com/api/v1/agent/<slug>/widget.js
```

### Step 3: Upload KB.MD, POLICY.MD, SCRIPT.MD

Go to the tenant's detail page in Agent Admin and upload the three markdown files.

**Critical:** Uploading files stores them but does NOT auto-rebuild the RAG index. After uploading, click **Rebuild Index** to make the content searchable.

---

## 7. How to Write a High-Impact KB.MD

**KB.MD = the agent's brain.** It contains every fact about the business that the agent can reference when answering visitor questions. This is the single most important file you will write.

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

### <!-- @service: service_code_1, emergency: true, response_time: "60 minutes" -->
#### [Service Name]
[2–4 sentences describing what this service covers, when it's needed, and what the customer can expect.]

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
[Honest, helpful answer that sets expectations without quoting fixed prices if they vary. Include factors that affect cost.]

Example:
> Costs vary based on [factors]. For a pressure washing job, we consider
> square footage, surface type, level of staining, and accessibility.
> We offer free estimates so you know the exact cost before we begin.

**Do NOT quote exact prices unless they are fixed and current.**

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

The `<!-- @type: value -->` annotations make content machine-readable for the RAG system:

| Annotation | Purpose | Example |
|------------|---------|---------|
| `@service: code, emergency: true, response_time: "60min"` | Mark a service entry | `@service: water_extraction, emergency: true` |
| `@exclusion: code, exception: "secondary to X"` | Mark a service NOT offered | `@exclusion: mold_remediation` |
| `@coverage: include/exclude` | Mark service area sections | `@coverage: include` |
| `@faq: category` | Mark FAQ entries | `@faq: pricing` |
| `@section: name` | Mark document sections | `@section: emergency_protocol` |
| `@safety: hazard_type` | Mark safety content | `@safety: electrical_hazard` |

### Writing Rules

**DO:**
- Write every service, exclusion, coverage area, and FAQ precisely
- Use bullet points — the agent handles structured content better
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
- Include content from other businesses (even templates)
- Leave placeholder text like "[Your Company Name]"
- Include personal/private information about the owner
- Promise things the business doesn't actually deliver
- Quote exact prices that change frequently without noting "starting at" or "varies"

### Content Quality Checklist

Before publishing a KB.MD, verify:
- [ ] Every service has a `<!-- @service -->` entry
- [ ] Every major service NOT offered has an `<!-- @exclusion -->` entry
- [ ] All service areas (cities, ZIPs, counties) are listed
- [ ] All phone numbers, emails, and addresses are current
- [ ] Emergency protocol includes after-hours contact
- [ ] Pricing section sets expectations without false quotes
- [ ] FAQs cover: hours, estimates, payment, insurance, scheduling, cancellation
- [ ] No placeholder text like `[fill in]` or `[company name]`
- [ ] File is named exactly `KB.MD` when uploaded

---

## 8. POLICY.MD — Business Rules, Do's and Don'ts

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
6. **If** the visitor asks about a competitor, redirect gracefully to [Business Name] strengths.
7. **If** the visitor asks about something outside services, say so and offer alternatives.
8. **Never** pretend to be human if asked "are you a bot/robot/AI?"

---

## Intents (Trigger Conditions)

<!-- @intent: emergency, urgency_threshold: 75, priority: P0 -->
### Emergency
A customer reports urgent, time-sensitive damage or safety issue.

**Agent behavior:**
- Acknowledge urgency immediately
- Collect: name, phone, location, nature of emergency
- Confirm dispatch within response window
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

## Thresholds

<!-- @threshold: urgency_emergency, value: 75 -->
### Urgency Scoring

| Score Range | Classification | Agent Response |
|-------------|----------------|----------------|
| 75–100 | **EMERGENCY** | Immediate acknowledgment, dispatch within response window |
| 40–74 | **URGENT** | Priority response within 1 hour during business hours |
| 1–39 | **STANDARD** | Response within normal service window |
| 0 | **INFORMATIONAL** | Standard response, no urgency flags |

---

## Fallback Rules

<!-- @rule: fallback, priority: medium -->
### When the Agent Doesn't Know

If the agent cannot answer using KB.MD or POLICY.MD:

1. **Say so honestly.** Not "I think so" or hedging guesses.
2. **Offer an alternative:** "I don't have that information, but I can [collect your contact info and have someone follow up / connect you with our team]."
3. **Transition to lead capture** — ask for name, contact, and question.

This is the **"Say Unknown + Capture"** behavior. It protects trust and turns unknowns into leads.

---

## Lead Capture Policy

<!-- @rule: lead_capture -->
### When to Ask for Contact Details

The agent should collect contact info when:
- The visitor asks for a quote or estimate
- The visitor asks to schedule or book
- The visitor reports an emergency
- The visitor explicitly provides info (agent confirms and stores)
- The conversation reaches a natural close after substantive inquiry

The agent should NOT aggressively collect contact info:
- On the very first message before understanding need
- If the visitor only says "hi" or "hello"
- If the visitor asks a simple FAQ that is resolved in KB.MD

---

### Minimum Required Lead Fields

- **Name** OR **Phone** (at least one)
- **Email** OR **Phone** (at least one)
- **Service/need** (extracted from conversation)
- **Location** (if service is location-dependent)
- **Urgency level** (from urgency scoring above)

---

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

---

### After Human Takeover

- Show transition message: "I'm connecting you with [Name] from our team. They'll respond within [timeframe]."
- Create a system notification/email for the human agent
- Do NOT continue resolving autonomously
- Optionally create a support ticket
```

### POLICY.md Rules of Thumb

**DO:**
- Define the agent's identity like a new employee job description
- Specify exact trigger words for escalation
- Set urgency thresholds that match actual emergency response times
- Include after-hours behavior explicitly
- Define what happens when the agent doesn't know
- Include anti-hallucination rules: "Never guess pricing, availability, or policy"

**DON'T:**
- Write POLICY.md as if the agent is a human
- Use vague rules ("be polite") without operational definitions
- Forget to define escalation behavior — this destroys trust
- Allow the agent to negotiate pricing, promises, or guarantees

---

## 9. SCRIPT.MD — Conversation Flow

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

The agent should never ask for contact details before understanding what the visitor needs. The conversation should feel natural: helping first, collecting second.

---

## 10. Deploying the Chat Widget

The widget is the visible chat interface your website visitors interact with.

### Option 1: JavaScript Embed (Recommended)

```html
<!DOCTYPE html>
<html>
<head>
  <title>Acme Pressure Washing</title>
</head>
<body>
  <!-- Your existing website content -->
  <h1>Welcome to Acme Pressure Washing</h1>
  
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
| `data-welcome` | Initial greeting | `"Hi! How can we help?"` |

### Option 2: iframe Embed

Best for WordPress, Wix, Shopify, or any CMS:

```html
<iframe
  src="https://your-server.com/widget/acme-pressure-washing/iframe?color=%230d9488"
  style="position:fixed;bottom:0;right:0;width:400px;height:600px;border:none;z-index:9999;"
  title="Chat with Acme Pressure Washing"
></iframe>
```

---

## 11. Lead Capture Workflow

Lead capture is automatic and conversational — no forms required.

### How It Works

1. **Conversational extraction:** Agent asks for contact details naturally during conversation
2. **Multi-message accumulation:** Agent collects name, email, phone across multiple turns
3. **Intelligent merging:** Three-layer extraction pipeline (regex + structural analysis + AI)
4. **Confidence scoring:** Each field has a confidence score; high-confidence leads commit immediately
5. **Correction handling:** "No, I meant john2000@email.com" replaces the previous email
6. **Decline handling:** "I'd rather not share" sets `declined: true` and stops re-prompting

### Lead Fields

| Field | Required | Source |
|-------|----------|--------|
| `tenant_id` | Always | Session context |
| `session_id` | Always | Chat session |
| `customer_name` | One of name/email/phone | Extracted |
| `customer_email` | One of name/email/phone | Extracted |
| `customer_phone` | One of name/email/phone | Extracted |
| `customer_location` | Optional | From conversation |
| `service_interest` | Optional | From RAG or conversation |
| `detected_intent` | Optional | Intent classification |
| `urgency_level` | Optional | Urgency score |
| `status` | Default: `new` | new → contacted → converted → lost |

---

## 12. The Admin Panel

Access at: `https://your-server.com/agent-admin`

### Sections

| Section | What It Does |
|---------|-------------|
| **Tenants** | List, create, edit, activate/deactivate businesses |
| **Tenant Detail** | Upload KB.MD, POLICY.MD, SCRIPT.MD; configure LLM model; manage users |
| **Leads** | View, filter, export leads for a tenant |
| **RAG Debug** | Test what vector search retrieves for any query |
| **Simulations** | Run virtual customer conversations to test agent |
| **Transcripts** | View and export full conversation logs |

---

## 13. RAG Knowledge Base Management

**RAG (Retrieval-Augmented Generation)** is how the agent finds relevant knowledge before answering. Without RAG, the entire KB.MD is stuffed into the LLM prompt (works for small files, degrades for large ones).

### RAG Pipeline

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
[3] Agent builds LLM prompt: "Here are relevant facts: [chunks]. Now answer."
         │
         ▼
[4] LLM returns grounded, accurate answer
```

### Managing RAG

After uploading or updating KB.MD:

1. Click **Rebuild Index** in tenant admin panel
2. System chunks the file, generates embeddings, stores in LanceDB
3. Takes 30–120 seconds depending on file size
4. Admin sees: total chunks, last indexed timestamp, any errors

### Hybrid Search

RAG uses hybrid search by default:
- **70% vector similarity** (semantic meaning)
- **30% BM25 keyword matching** (exact terms)

This means both "water removal" and "standing water extraction" return the same relevant chunk.

---

## 14. Testing Before Going Live

### Simulation Testing

Before exposing the widget to real visitors:

1. Go to **Agent Admin → [Tenant] → Simulations**
2. Configure a virtual customer persona:
   - Name, scenario, goal, tone
3. Click **Run Simulation**
4. Watch conversation unfold in real-time
5. Review transcript:
   - Did agent ask qualifying questions?
   - Did it capture name/email/phone?
   - Did it handle urgency correctly?
   - Did it hallucinate info?
   - Did it follow POLICY.MD rules?

### RAG Debug Testing

1. Go to **Agent Admin → [Tenant] → RAG Debug**
2. Type test query: "emergency water extraction in Phoenix"
3. See what chunks were retrieved with relevance scores
4. Verify: Are the right services showing up?

### Pre-Launch Checklist

- [ ] KB.MD reviewed by business owner
- [ ] POLICY.MD covers: identity, escalation, urgency thresholds, fallback
- [ ] SCRIPT.MD defines clear conversation stages
- [ ] All phone numbers, emails, areas verified
- [ ] RAG index rebuilt after file upload
- [ ] At least 3 simulations run and passed
- [ ] RAG debug returns correct chunks for 10+ test queries
- [ ] Lead capture tested
- [ ] Escalation workflow tested
- [ ] Widget embedded on staging site
- [ ] Widget works on mobile
- [ ] Domain restrictions set
- [ ] Business owner has admin access

---

## 15. Common Business Workflows

### Workflow 1: After-Hours Lead

```
22:00 — Visitor asks about emergency water extraction
22:00 — Agent responds immediately (no human needed)
22:01 — Agent collects: name (Sarah), phone (555-0142), address (Phoenix), nature of emergency
22:02 — Agent: "I've sent this to our on-call team. Someone will call within 30 minutes."
22:03 — Lead created with urgency_score = emergency, status = new
22:03 — (Optional) SMS/email alert sent to on-call technician
```

### Workflow 2: FAQ Becomes Lead

```
14:00 — Visitor: "Do you handle mold remediation?"
14:00 — Agent: "We don't provide standalone mold remediation, but if mold is secondary to water damage, we can help with water extraction first. Would you like to schedule an assessment?"
14:02 — Visitor: "Yes, that sounds right"
14:02 — Agent collects contact details
14:03 — Lead captured with service = water_extraction, notes = "mold secondary"
```

### Workflow 3: Self-Service FAQ (No Lead)

```
10:00 — Visitor: "What are your business hours?"
10:00 — Agent: "We're open Monday–Saturday, 7 AM – 7 PM. For emergencies, our 24/7 line is 555-0199."
10:00 — Lead NOT created (no contact info, no service interest)
```

### Workflow 4: Escalation

```
16:00 — Visitor: "This is the third time I've called and nobody shows up"
16:00 — Agent detects escalation keywords + high urgency
16:00 — Agent: "I understand your frustration. I'm connecting you with [Manager Name] right now. They'll call within 15 minutes."
16:00 — Agent pauses chat, creates internal alert, marks session for human follow-up
```

---

## 16. Policy & Legal Guardrails

### Compliance Notes

**CAN-SPAM (Email):**
- Commercial emails must have truthful headers, non-deceptive subject lines
- Must identify sender and include physical address
- Must provide opt-out mechanism
- Applies to lead follow-up emails

**TCPA (SMS/Text):**
- Consent required for automated marketing text messages
- Must provide opt-out mechanism
- Consent revocation must be honored immediately
- Lead capture ≠ consent for SMS (must be explicit)

**State Privacy Laws (CCPA/CPRA/VCDPA):**
- California, Colorado, Virginia have consumer privacy laws
- Must disclose data collection in privacy policy
- Visitors should understand contact info will be stored

### Recommended Legal Language

Add to client's website near the chat widget:

> By using this chat, you agree that [Business Name] may collect your name, email, and phone number for following up on your inquiry. We do not share your information with third parties. See our Privacy Policy for full details.

Add to privacy policy:

> Contact Information: When you contact us through our website chat, we collect your name, email address, phone number, and the details of your inquiry. This information is used solely to respond to your request and is not shared without your consent.

---

## 17. What NOT To Do (Anti-Patterns)

| Anti-Pattern | Why It's Bad | What To Do Instead |
|--------------|--------------|-------------------|
| **Copy-paste KB.MD from another business** | Agent gives wrong services, pricing, coverage | Write KB.MD from scratch for each business |
| **Vague service descriptions** | Agent can't answer "Do you do X?" accurately | Be specific: what's included, what's not, triggers |
| **No exclusions section** | Agent promises services business doesn't offer | Always list what's NOT included |
| **No after-hours policy** | Agent behaves as if business is 24/7 | Define after-hours behavior in POLICY.MD |
| **Hardcoded pricing in KB.MD** | Agent quotes stale or wrong prices | Describe pricing structure, not fixed numbers |
| **Skipping simulation** | Agent has untested blind spots | Run at least 3 simulations per tenant before launch |
| **Ignoring RAG rebuilds** | Agent answers from stale knowledge | Always rebuild index after KB.MD changes |
| **Over-engineering SCRIPT.MD** | Long scripts that LLM ignores | Keep to clear stage names and bullet points |
| **Setting temperature > 0.8** | Agent becomes creative, hallucinates quotes | Use 0.3–0.5 for local services; 0.7 max |
| **Linking widget to wrong tenant slug** | Agent uses wrong knowledge base | Double-check slug in embed URL |
| **Leaving widget on broken tenant** | Visitor sees errors, trust lost | Verify tenant is `Is Active = true` |
| **Not reviewing leads daily** | Leads go cold, expectation mismatch | Set daily reminder to review new leads |

---

## 18. Pricing Model

### Recommended Package Structure (Pithom Labs)

**Package 1 — Chat Widget Only** ($149–$299 setup / $149–$299/month)
- For businesses with existing websites
- Widget install, KB/Policy training, lead capture, email/SMS alerts, weekly report

**Package 2 — Hugo Site + AI Lead Desk** ($399–$999 setup / $199–$499/month)
- For businesses with no/bad websites
- Fast landing site, domain/DNS, service pages, widget, notification workflow

**Package 3 — Managed Lead Desk** ($499–$1,500 setup-equivalent / $499–$1,500/month)
- For growing SMBs wanting white-glove service
- Everything above + CRM/Sheets pipeline, appointment routing, follow-up, monthly optimization

### Internal Cost Structure (Your Costs)

| Component | Estimate | Notes |
|-----------|----------|-------|
| OpenRouter API | $5–$50/tenant/month | Depends on chat volume + model |
| Server hosting (Fly.io) | $15–$50/month | Small VM handles 25–50 tenants |
| LanceDB storage | Negligible | On same disk |
| Domain + SSL | $1–$2/month | Per custom domain |
| **Total per tenant** | **$5–$30/month** | 80–90% margin on $199–$499 retail |

---

## 19. Troubleshooting

### Agent Doesn't Know About a Service

**Cause:** KB.MD missing the service, or RAG index not rebuilt.

**Fix:**
1. Check KB.MD has `<!-- @service: code -->` entry
2. In Admin → Rebuild Index
3. Test via RAG Debug panel

### Agent Gives Wrong Answer (Hallucination)

**Cause:** RAG not enabled, content missing, or temperature too high.

**Fix:**
1. Verify `RAG_PIPELINE_ENABLED=true` in environment
2. Add missing content to KB.MD
3. Rebuild index
4. Lower temperature to 0.3–0.5
5. Ensure POLICY.MD has fallback rule — agent should say "I don't have that information"

### Agent Doesn't Capture Leads

**Cause:** Lead extraction regex doesn't match input format.

**Fix:**
1. Check conversation contained name + email/phone
2. Verify fields in `agent_leads` table
3. Test with: "My name is John Smith, email is john@example.com"
4. Check: `SELECT * FROM agent_leads ORDER BY id DESC LIMIT 5;`

### Widget Won't Load

**Cause:** Wrong slug, domain restriction blocking, or server unreachable.

**Fix:**
1. Verify embed URL: `https://your-server.com/widget/EXACT-SLUG/embed.js`
2. Check domain is in `Allowed Domains`
3. Check DevTools → Console for errors
4. Verify `Is Active = true`

### RAG Index Fails to Build

**Cause:** Embedding API issue, chunking error, or timeout.

**Fix:**
1. Check `OPENROUTER_API_KEY` is set
2. Check logs: `grep "RAG" build/memos.log`
3. Increase `EMBEDDING_TIMEOUT` to 300s
4. Verify LanceDB path exists and is writable
5. Try mock embeddings: `task run:rag:mock`

---

## 20. Quick Reference Cheatsheets

### File Naming Conventions

| File Name | Purpose | Required |
|-----------|---------|----------|
| `KB.MD` | Knowledge base | Yes |
| `POLICY.MD` | Agent policy & rules | Yes |
| `SCRIPT.MD` | Conversation flow | Recommended |
| `KB_<epoch>.MD` | Versioned backup | Auto-generated |
| `POLICY_<epoch>.MD` | Versioned backup | Auto-generated |

### API Endpoints

**Public (No Auth):**
| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/agent/:slug/chat` | Send a chat message |
| GET | `/api/v1/agent/:slug/chat/stream` | SSE stream of responses |
| GET | `/api/v1/agent/:slug/widget.js` | Legacy widget script |
| GET | `/widget/:slug/embed.js` | JavaScript embed |
| GET | `/widget/:slug/iframe` | iframe embed |

**Admin (Authenticated):**
| Method | Path | Permission | Purpose |
|--------|------|------------|---------|
| GET | `/api/v1/agent/tenants` | `tenant:admin` | List tenants |
| POST | `/api/v1/agent/tenants` | `tenant:admin` | Create tenant |
| GET | `/api/v1/agent/:slug` | `tenant:read` | Get tenant config |
| POST | `/api/v1/agent/:slug/files` | `files:upload` | Upload KB/Policy/Script |
| POST | `/api/v1/agent/:slug/reindex` | `api:config` | Rebuild RAG index |
| POST | `/api/v1/agent/:slug/simulate` | `chat:test` | Run simulation |

### Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `OPENROUTER_API_KEY` | Yes | (none) | OpenRouter LLM access |
| `LLM_MODEL` | No | `openai/gpt-4o-mini` | Default chat model |
| `RAG_PIPELINE_ENABLED` | No | `false` | Enable/disable RAG |
| `EMBEDDING_PROVIDER` | No | `openrouter` | `openrouter`, `local`, `mock` |
| `EMBEDDING_MODEL` | No | `text-embedding-3-small` | Embedding model |
| `LANCEDB_STORAGE_PROVIDER` | No | `local` | `local`, `s3` |
| `LANCEDB_LOCAL_PATH` | No | `build/data/lancedb` | Disk path |

### Key Terms Glossary

| Term | Definition |
|------|-----------|
| **Tenant** | One business/client; all data scoped to tenant |
| **Slug** | URL-safe tenant ID (e.g., `acme-pressure-washing`) |
| **KB.MD** | Markdown knowledge base; the agent's factual memory |
| **POLICY.MD** | Agent policy; identity, rules, intents, thresholds |
| **SCRIPT.MD** | Conversation stages and flow |
| **RAG** | Retrieval-Augmented Generation; searches KB before LLM responds |
| **LanceDB** | Embedded vector database for RAG |
| **Embedding** | Numerical vector representation of text |
| **SSE** | Server-Sent Events; real-time response streaming |
| **Lead** | Captured prospect with name, email, phone, service, urgency |
| **Urgency Score** | 0–100 score classifying conversation |

---

*End of DOCS_HOWTO_BIZ.md*