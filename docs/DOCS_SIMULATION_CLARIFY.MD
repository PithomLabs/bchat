# MD File Workflow Clarification

**Date:** 2026-01-21
**Author:** Claude Code Analysis
**Purpose:** Clarify how KB.MD, POLICY.MD, and SCRIPT.MD files are used in the chat agent system

---

## Question

> "double check if the SCRIPT.MD is being followed at all, I am not seeing script file_type at agent_source_files table"

## Answer

**Your observation is correct** - there is no "script" file_type in `agent_source_files`.

**However, SCRIPT.MD IS fully implemented** - it uses a **separate dedicated table**: `agent_tenant_scripts`

### Why Two Tables?

| File | Database Table | Scope | Reason |
|------|----------------|-------|--------|
| KB.MD | `agent_source_files` | Per audience | Different audiences (external, internal) may have different knowledge bases |
| POLICY.MD | `agent_source_files` | Per audience | Different audiences may have different policies |
| SCRIPT.MD | `agent_tenant_scripts` | Per tenant | Conversation flow is the same for all audiences within a tenant |

This is an intentional design decision. SCRIPT.MD defines the conversation structure which typically doesn't vary by audience type.

---

## Complete Workflow: From Upload to Chat

### Phase 1: File Upload

```
┌─────────────────────────────────────────────────────────────────┐
│                     USER UPLOADS FILES                          │
├─────────────────────────────────────────────────────────────────┤
│ KB.MD + POLICY.MD               │  SCRIPT.MD                    │
│ POST /:slug/import              │  POST /:slug/script           │
│ handlers.go:476                 │  handlers.go:2438             │
│ (per audience)                  │  (per tenant)                 │
└─────────────────────────────────────────────────────────────────┘
```

**KB.MD and POLICY.MD:**
- Endpoint: `POST /api/v1/agent/:slug/import`
- Handler: `HandleImportSingleFile()` → calls `importFiles()`
- Parsed by: `ParseKB()` and `ParsePolicy()` in parser.go
- Storage: Multiple typed tables + raw content in `agent_source_files`

**SCRIPT.MD:**
- Endpoint: `POST /api/v1/agent/:slug/script`
- Handler: `HandleImportScript()`
- Parsed by: `ParseScript()` in parser.go
- Storage: `agent_tenant_scripts` table only

---

### Phase 2: Parsing

Each MD file has its own parser function that extracts structured data from annotation blocks.

| File | Parser Function | Location | Output |
|------|-----------------|----------|--------|
| KB.MD | `ParseKB()` | parser.go:165 | Services, Exclusions, FAQs, Safety protocols, Coverage, Sections |
| POLICY.MD | `ParsePolicy()` | parser.go:343 | Identity, Intents, Rules, Thresholds |
| SCRIPT.MD | `ParseScript()` | parser.go:893 | Sections with questions/actions + **Auto-generated summary** |

**SCRIPT.MD Special Feature:**
The parser automatically generates a condensed summary via `buildScriptSummary()` (parser.go:993). This summary is injected into the system prompt instead of the full script content, keeping the prompt efficient.

---

### Phase 3: Database Storage

```
KB.MD Parsed Data:
├── agent_services (per audience)
├── agent_exclusions (per audience)
├── agent_faqs (per audience)
├── agent_safety_protocols (per audience)
├── agent_kb_sections (per audience)
├── agent_coverage (shared across audiences)
└── agent_source_files (raw content, file_type='kb')

POLICY.MD Parsed Data:
├── agent_intents (per audience)
├── agent_rules (per audience)
├── agent_audiences (identity, tone, phone, email)
└── agent_source_files (raw content, file_type='policy')

SCRIPT.MD Parsed Data:
└── agent_tenant_scripts (content + summary, per tenant)
    ├── content: Full SCRIPT.MD markdown
    ├── content_hash: SHA256 for versioning
    ├── summary: Auto-generated condensed version
    └── version: Auto-incremented on updates
```

---

### Phase 4: Loading for Chat

When a chat message is received, `LoadConfig()` (service.go:382-491) loads all configuration:

```go
// 1. Load KB data from typed tables
services   := ListAgentServices(tenantID, audienceType)
exclusions := ListAgentExclusions(tenantID, audienceType)
faqs       := ListAgentFAQs(tenantID, audienceType)
safety     := ListAgentSafetyProtocols(tenantID, audienceType)
sections   := ListAgentKBSections(tenantID, audienceType)

// 2. Load Policy data from typed tables
intents    := ListAgentIntents(tenantID, audienceType)
rules      := ListAgentRules(tenantID, audienceType)

// 3. Load raw source files (for phone extraction fallback)
rawKB      := GetAgentSourceFile(tenantID, audienceType, "kb")
rawPolicy  := GetAgentSourceFile(tenantID, audienceType, "policy")

// 4. Load SCRIPT from SEPARATE TABLE (tenant-level, not audience-specific)
script     := GetAgentTenantScript(tenantID)  // ◄── Note: no audienceType
```

All loaded data is assembled into an `AudienceConfig` struct and cached for 5 minutes.

---

### Phase 5: System Prompt Construction

`buildSystemPrompt()` (service.go:1066-1310) assembles the system prompt in a specific order designed to combat context dilution in LLMs:

| # | Section | Source | Purpose |
|---|---------|--------|---------|
| 0 | Context Retention | Session history | Preserve customer info already collected |
| 1 | Critical Constraints | Hardcoded | Anti-hallucination rules (MOST IMPORTANT) |
| 2 | Identity | POLICY.MD | Role, tone, brand voice |
| 3 | Services We Offer | KB.MD | List of available services |
| 4 | Services We Don't Provide | KB.MD | Excluded services |
| **5** | **Conversation Flow** | **SCRIPT.MD** | **Structure guide for conversation** |
| 6 | Policies & Rules | POLICY.MD | Rules filtered by current intent |
| 6b | Safety Protocol | KB.MD | Injected if safety trigger detected |
| 6c | Emergency Handling | POLICY.MD | Urgency thresholds |
| 7 | Learned Behaviors | Self-improvement | Patterns from analysis |
| 8 | FAQs | KB.MD | Reference material |
| 9 | Authorized Contact Info | KB.MD + POLICY.MD | Phone, email |
| 10 | Response Format | Hardcoded | Plain text, no markdown |
| 11 | Critical Reminder | Multiple | Reinforcement at END |

**SCRIPT.MD Injection Code** (service.go:1189-1201):
```go
if config.Script != nil {
    scriptContent := config.Script.Summary  // Prefer condensed summary
    if scriptContent == "" {
        scriptContent = config.Script.Content  // Fallback to full content
    }
    if scriptContent != "" {
        sb.WriteString("=== CONVERSATION FLOW (Follow this structure) ===\n\n")
        sb.WriteString(scriptContent)
        sb.WriteString("\n\n")
    }
}
```

---

## Visual Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                          FILE UPLOAD                                │
├─────────────────────────────────────────────────────────────────────┤
│ KB.MD ──────┐                                                       │
│             ├──→ importFiles() ──→ agent_source_files (file_type=kb)│
│ POLICY.MD ──┘                  └──→ agent_source_files (file_type=policy)
│                                                                     │
│ SCRIPT.MD ────→ HandleImportScript() ──→ agent_tenant_scripts       │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         CONFIG LOADING                              │
├─────────────────────────────────────────────────────────────────────┤
│ LoadConfig(tenantSlug, audienceType)                                │
│   ├── Load KB data from typed tables (services, faqs, etc.)         │
│   ├── Load Policy data from typed tables (intents, rules, etc.)     │
│   ├── Load raw KB/Policy from agent_source_files                    │
│   └── Load Script from agent_tenant_scripts ◄── DIFFERENT TABLE     │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         SYSTEM PROMPT                               │
├─────────────────────────────────────────────────────────────────────┤
│ buildSystemPrompt()                                                 │
│   Section 2: Identity ◄──────────────── POLICY.MD                   │
│   Section 3: Services ◄──────────────── KB.MD                       │
│   Section 4: Exclusions ◄────────────── KB.MD                       │
│   Section 5: Conversation Flow ◄─────── SCRIPT.MD                   │
│   Section 6: Rules ◄─────────────────── POLICY.MD                   │
│   Section 8: FAQs ◄──────────────────── KB.MD                       │
│   Section 9: Contact Info ◄──────────── KB.MD + POLICY.MD           │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                           LLM CALL                                  │
├─────────────────────────────────────────────────────────────────────┤
│ System Message: Complete prompt from buildSystemPrompt()            │
│ User Message: Current chat input                                    │
│ History: Last 10 conversation messages                              │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    POST-PROCESSING                                  │
├─────────────────────────────────────────────────────────────────────┤
│ 1. Verification: Check response against KB/Policy                   │
│ 2. Sanitization: Remove placeholder phones/emails                   │
│ 3. Replacement: Inject validated contact info from KB               │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Troubleshooting: Is SCRIPT.MD Working?

If SCRIPT.MD doesn't appear to be affecting conversations:

### Check 1: Is the script uploaded?
```sql
SELECT id, tenant_id, content_hash, summary, version, imported_at
FROM agent_tenant_scripts
WHERE tenant_id = 1;
```
**If empty:** SCRIPT.MD was never uploaded for this tenant.

### Check 2: Is the summary generated?
```sql
SELECT LENGTH(summary) as summary_length, summary
FROM agent_tenant_scripts
WHERE tenant_id = 1;
```
**If summary is NULL or empty:** Parsing issue in `buildScriptSummary()`.

### Check 3: API verification
```bash
curl -X GET http://localhost:8081/api/v1/agent/bulovas/script \
  -H "Authorization: Bearer $TOKEN"
```
**Expected response:**
```json
{
  "content": "... full SCRIPT.MD ...",
  "summary": "1. SECTION: question1, question2...",
  "version": 1
}
```

### Check 4: Add debug logging
In service.go after `LoadConfig()`:
```go
slog.Debug("Config loaded",
    "tenant", config.TenantSlug,
    "hasScript", config.Script != nil,
    "scriptSummaryLen", len(config.Script.Summary) if config.Script != nil else 0)
```

---

## Summary Table

| Aspect | KB.MD | POLICY.MD | SCRIPT.MD |
|--------|-------|-----------|-----------|
| **Scope** | Per audience | Per audience | Per tenant |
| **Storage Table** | `agent_source_files` | `agent_source_files` | `agent_tenant_scripts` |
| **file_type value** | `"kb"` | `"policy"` | N/A (separate table) |
| **Upload Endpoint** | `POST /:slug/import` | `POST /:slug/import` | `POST /:slug/script` |
| **Parser Function** | `ParseKB()` | `ParsePolicy()` | `ParseScript()` |
| **Prompt Section** | 3, 4, 6b, 8, 9 | 2, 6, 6c | 5 |

---

## Files Reference

| File | Lines | Purpose |
|------|-------|---------|
| handlers.go | 476-548 | `HandleImportSingleFile()` - KB/Policy upload |
| handlers.go | 740-876 | `importFiles()` - Core import logic |
| handlers.go | 2438-2515 | `HandleImportScript()` - Script upload |
| handlers.go | 2517-2556 | `HandleGetScript()` - Script retrieval |
| parser.go | 165-341 | `ParseKB()` - KB parsing |
| parser.go | 343-529 | `ParsePolicy()` - Policy parsing |
| parser.go | 893-972 | `ParseScript()` - Script parsing |
| parser.go | 993-1017 | `buildScriptSummary()` - Summary generation |
| service.go | 382-491 | `LoadConfig()` - Configuration loading |
| service.go | 1066-1310 | `buildSystemPrompt()` - Prompt construction |
| service.go | 1189-1201 | Script injection into prompt |
| store/agent.go | 366-375 | `AgentTenantScript` type definition |
| store/db/sqlite/agent.go | 1306-1382 | SQLite CRUD for scripts |
| migration/0.25/05__agent_tables.sql | - | `agent_source_files` schema |
| migration/0.25/09__agent_script_analysis.sql | 1-15 | `agent_tenant_scripts` schema |

---

## Conclusion

The system is correctly designed with intentional separation:
- **`agent_source_files`** holds KB.MD and POLICY.MD (audience-scoped)
- **`agent_tenant_scripts`** holds SCRIPT.MD (tenant-scoped)

SCRIPT.MD is fully integrated and injected as Section 5 of the system prompt. If it's not working, verify the script was uploaded and the summary was generated correctly.

---

## Addendum: Simulations Use Uploaded Files Immediately

**Date Added:** 2026-01-21

### Question

> "When I upload MD files in Agent Admin and then start a simulation, will the simulation pick up the just uploaded files?"

### Answer: YES

Simulations **immediately use newly uploaded MD files** because:

1. **Files are stored in the database** - When you upload via Agent Admin:
   - KB.MD and POLICY.MD → `agent_source_files` table
   - SCRIPT.MD → `agent_tenant_scripts` table

2. **Simulations read from the database** - When you start a simulation:
   ```go
   // handlers.go line 1885
   config, err := h.service.LoadConfig(ctx, slug, "internal")
   ```
   `LoadConfig()` fetches all configuration from the database, not from local filesystem files.

3. **Local `*_DRAFT.MD` files are for review only**
   - Files in `docs/templates/examples/bulovas/` are working copies
   - They have **no effect** on simulations until uploaded via Agent Admin

### Config Cache (5-minute TTL)

There is a config cache with a 5-minute TTL (service.go line 41):
```go
configCache: NewConfigCache(5 * time.Minute)
```

**In practice:**
- If you upload new files and start a simulation **immediately**, the cache will miss and load fresh content
- If you ran a simulation recently (within 5 min), cached config may be used
- Cache invalidation: `InvalidateConfigCache(tenantSlug)` is called automatically on file upload

### Visual Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    YOUR WORKFLOW                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Edit local DRAFT files (KB_DRAFT.MD, POLICY_DRAFT.MD, etc.) │
│                          │                                       │
│                          ▼                                       │
│  2. Upload via Agent Admin (POST /api/v1/agent/:slug/import)    │
│                          │                                       │
│                          ▼                                       │
│  3. Files parsed & stored in DATABASE                           │
│     ├── agent_source_files (KB, POLICY)                         │
│     └── agent_tenant_scripts (SCRIPT)                           │
│                          │                                       │
│                          ▼                                       │
│  4. Start Simulation (LoadConfig fetches from DB)               │
│                          │                                       │
│                          ▼                                       │
│  5. Simulation uses NEW uploaded content ✓                      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### TL;DR

| Step | What Happens |
|------|--------------|
| Edit local file | No effect on simulations |
| Upload via Agent Admin | Stored in database |
| Start simulation | Reads from database → uses new content |

**The local `*_DRAFT.MD` files are just for your review. Upload them via Agent Admin to test in simulations.**

---

*End of Analysis*
