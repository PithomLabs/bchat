# Auto-Generate Annotated KB.MD / POLICY.MD Feature

## Overview

This feature adds two buttons to Agent Admin that use an LLM to parse raw content from the database and generate properly annotated KB.MD or POLICY.MD files using the Annotation-Based Parsing System.

---

## Requirements

| Requirement | Value |
|-------------|-------|
| Location | Under "External (Customer-Facing)" section in Agent Admin |
| Buttons | "Generate KB.MD" and "Generate POLICY.MD" |
| LLM Model | `LLM_MODEL_REASONING` env var (default: `google/gemini-2.5-pro`) |
| Annotations | All types + custom types where relevant |
| Output | Modal displaying generated content with copy functionality |

---

## Architecture

### Flow

```
User clicks "Generate KB.MD"
         │
         ▼
Frontend calls POST /api/v1/agent/:slug/generate-kb
         │
         ▼
Backend loads raw content from agent_source_files (external)
         │
         ▼
Backend sends content to LLM with annotation instructions
         │
         ▼
LLM returns annotated KB.MD
         │
         ▼
Backend returns generated content
         │
         ▼
Frontend displays in modal with copy button
```

---

## API Endpoints

### Generate KB

**Endpoint:** `POST /api/v1/agent/:slug/generate-kb`

**Request:** None (uses stored content)

**Response:**
```json
{
  "content": "# Company Name Knowledge Base\n\n## About\n<!-- @section: about, type: general -->\n..."
}
```

**Errors:**
| Status | Message |
|--------|---------|
| 404 | Tenant not found |
| 404 | No KB content found |
| 403 | Admin required |
| 500 | LLM generation failed |

### Generate Policy

**Endpoint:** `POST /api/v1/agent/:slug/generate-policy`

**Request:** None (uses stored content)

**Response:**
```json
{
  "content": "# Company Name Policy\n\n## Identity\n<!-- @identity: agent -->\n..."
}
```

---

## Annotation Types Reference

The LLM generates content using these annotation types:

### KB.MD Annotations

| Annotation | Purpose | Example |
|------------|---------|---------|
| `@section` | General content sections | `<!-- @section: about, type: general -->` |
| `@service` | Services/products offered | `<!-- @service: water_damage, emergency: true -->` |
| `@faq` | Frequently asked questions | `<!-- @faq: response_time -->` |
| `@exclusion` | Things NOT offered | `<!-- @exclusion: plumbing, exception: "if caused damage" -->` |
| `@coverage` | Geographic/scope areas | `<!-- @coverage: include -->` |
| `@safety` | Safety protocols | `<!-- @safety: gas_leak, triggers: emergency_gas -->` |

### POLICY.MD Annotations

| Annotation | Purpose | Example |
|------------|---------|---------|
| `@identity` | Agent identity definition | `<!-- @identity: agent -->` |
| `@intent` | User intent classification | `<!-- @intent: schedule_service, priority: high -->` |
| `@rule` | Policy rules | `<!-- @rule: greeting, priority: 1 -->` |
| `@thresholds` | Escalation thresholds | `<!-- @thresholds: escalation -->` |

---

## LLM Prompts

### KB Generation Prompt

The system sends the following prompt structure to the LLM:

```
You are a technical writer creating a structured Knowledge Base document.

Analyze the following raw content and generate a properly formatted KB.MD file
using HTML comment annotations.

## Company Name
{companyName}

## Raw Content to Analyze
{rawContent}

## Required Output Format
[Annotation instructions for @section, @service, @faq, @exclusion, @coverage, @safety]

## Rules
1. Each annotation must have a unique code (lowercase_snake_case)
2. Group related content under appropriate headings
3. Identify and extract all FAQs from the content
4. Identify services/products if applicable
5. Create custom @section types for content that doesn't fit other categories
6. Use clear, descriptive titles
7. Maintain the original content's meaning
8. Add section separators (---) between entries

## Output
Return ONLY the formatted KB.MD content, starting with:
# {companyName} Knowledge Base
```

### Policy Generation Prompt

Similar structure but focused on:
- Agent identity (@identity)
- User intents (@intent)
- Policy rules (@rule)
- Escalation thresholds (@thresholds)

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_MODEL_REASONING` | `google/gemini-2.5-pro` | Model for content generation |
| `OPENROUTER_API_KEY` | (required) | API key for OpenRouter |

---

## Implementation Files

### Backend

| File | Changes |
|------|---------|
| `server/router/api/v1/agent/handlers.go` | Add `HandleGenerateKB`, `HandleGeneratePolicy` |
| `server/router/api/v1/agent/service.go` | Add `GenerateAnnotatedKB`, `GenerateAnnotatedPolicy` |
| `server/router/api/v1/v1.go` | Register new routes |

### Frontend

| File | Changes |
|------|---------|
| `web/src/store/v2/agentAdmin.ts` | Add `generateKB`, `generatePolicy` methods |
| `web/src/pages/AgentAdmin.tsx` | Add buttons, modal, handlers |
| `web/src/locales/en.json` | Add translations |

---

## UI Components

### Buttons

Located under "External (Customer-Facing)" section:

```
┌─────────────────────────────────────────────────────────┐
│ External (Customer-Facing)                              │
├─────────────────────────────────────────────────────────┤
│ KB.MD: [Upload] [View] [Download]                       │
│ POLICY.MD: [Upload] [View] [Download]                   │
│                                                         │
│ Auto-Generate:                                          │
│ [Generate KB.MD]  [Generate POLICY.MD]                  │
└─────────────────────────────────────────────────────────┘
```

### Modal

```
┌─────────────────────────────────────────────────────────┐
│ Generated KB.MD                                    [X]  │
├─────────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────────┐ │
│ │ # Company Knowledge Base                            │ │
│ │                                                     │ │
│ │ ## About                                            │ │
│ │ <!-- @section: about, type: general -->             │ │
│ │                                                     │ │
│ │ Company description here...                         │ │
│ │                                                     │ │
│ │ ---                                                 │ │
│ │ ...                                                 │ │
│ └─────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────┤
│                              [Close]  [Copy to Clipboard]│
└─────────────────────────────────────────────────────────┘
```

---

## Usage Guide

### Step 1: Ensure Raw Content Exists

The tenant must have KB.MD or POLICY.MD content uploaded (even if unformatted).

### Step 2: Click Generate Button

1. Go to Agent Admin
2. Select the tenant
3. Under "External (Customer-Facing)", click "Generate KB.MD" or "Generate POLICY.MD"
4. Wait for LLM processing (may take 10-30 seconds for large content)

### Step 3: Review Generated Content

The modal displays the LLM-generated annotated content.

### Step 4: Copy and Save

1. Click "Copy to Clipboard"
2. Save to a local `KB.MD` or `POLICY.MD` file
3. Review and make any manual adjustments
4. Upload via the standard "Upload" button

---

## Limitations

| Limitation | Description |
|------------|-------------|
| Large content | Very large files may exceed LLM context limits |
| LLM variability | Output may vary between generations |
| Manual review needed | Always review generated content before use |
| API dependency | Requires OpenRouter API access |

---

## Related Documentation

- [DOCS_AGENT_ARCHITECTURE.MD](./DOCS_AGENT_ARCHITECTURE.MD) - Annotation-based parsing system
- [DOCS_CHECK_KB.MD](./DOCS_CHECK_KB.MD) - KB.MD format requirements
- [DOCS_ENV_VAR.MD](./DOCS_ENV_VAR.MD) - Environment variables
- [DOCS_RAG_PIPELINE.MD](./DOCS_RAG_PIPELINE.MD) - RAG architecture
