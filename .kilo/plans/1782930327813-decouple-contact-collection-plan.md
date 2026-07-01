# Decouple Contact Collection from System Prompt — Adversarial Review & Revised Plan

## Adversarial Review of Original Proposal

### Critical Gaps Found

| Gap | Severity | Explanation |
|-----|----------|-------------|
| `importFiles()` doesn't call parser for policy | **HIGH** | The file upload path (`importFiles`) directly creates `AgentAudience` with hardcoded defaults and never reads `@settings` from POLICY.MD. The requested `@settings` annotation would be silently ignored on every upload. |
| Post-LLM correctors bypass the flag | **HIGH** | `CorrectContactsInResponse()` and `CorrectEmailsInResponse()` are called unconditionally in `processChat()` (lines 1872–1900). Even with the prompt stripped, the backend still rewrites phone numbers and emails in the LLM output. |
| `buildRAGSection0` ignores the flag | **MEDIUM** | In RAG mode, `buildRAGSection0` injects "CUSTOMER INFO ALREADY PROVIDED" into the prompt regardless of the setting. A generic KB doesn't need this state tracking injected into the prompt. |
| `extractCollectedInfo` runs unconditionally | **LOW** | Progressive-profiling regex extraction runs at line 1737 and line 3945 even when contact collection is off. Not harmful, but unnecessary work and could still surface contact data in session state. |
| `buildRule1` vs `buildRAGFallback` mismatch | **MEDIUM** | The user's symptom ("no mention of a leper") is a RAG fallback case, handled by `buildRAGFallback`, not `buildRule1`. The plan targets the wrong function for the actual reported symptom. |
| `ParsePolicy` only initializes `result.Audience` in `thresholds` case | **HIGH** | If `@settings` appears without a `@thresholds` block, `result.Audience` is nil and setting fields on it panics. |
| No admin UI exposure | **MEDIUM** | Requiring a POLICY.MD annotation is workable but not discoverable. The admin UI needs at least a read/write toggle so operators can see and change the value without editing markdown. |
| `DEFAULT 1` preserves forced collection for all existing tenants | **LOW/MEDIUM** | Correct for backward compatibility, but should be documented as a deliberate choice. |

---

## Revised Plan: Option 2 with Gap Fixes

### Overview

Introduce a `require_contact_on_fallback` flag on `AgentAudience` (default `true` for backward compatibility). When `false`, the system prompt stops injecting contact-collection instructions and post-LLM contact correction is skipped. The flag can be set via `@settings` in POLICY.MD **and** toggled in the Agent Admin UI.

---

### File Changes

#### 1. `store/agent.go` — Add field to `AgentAudience`

```go
type AgentAudience struct {
    // ... existing fields ...

    // Contact fallback behavior
    RequireContactOnFallback bool // default true; when false, no contact collection prompts
}
```

#### 2. `store/migration/sqlite/016__audience_contact_fallback.sql`

```sql
ALTER TABLE agent_audience ADD COLUMN require_contact_on_fallback BOOLEAN DEFAULT 1;
```

Backward-compatible: existing rows default to `1` (true), preserving current behavior.

---

#### 3. `server/router/api/v1/agent/parser.go` — Parse `@settings` annotation

Add a new `"settings"` case in the `ParsePolicy` switch. Ensure `result.Audience` is initialized before setting fields:

```go
case "settings":
    if result.Audience == nil {
        result.Audience = &store.AgentAudience{
            TenantID:     tenantID,
            AudienceType: audienceType,
        }
    }
    if v, ok := block.params["require_contact_on_fallback"]; ok {
        result.Audience.RequireContactOnFallback = (v != "false")
    }
```

The annotation syntax in POLICY.MD:

```markdown
<!-- @settings: require_contact_on_fallback: false -->
```

---

#### 4. `server/router/api/v1/agent/handlers.go` — Two changes

**4a. `importFiles()` — Read `@settings` from policy content after saving**

After saving the policy source file and before/after `indexContentForRAG()`, scan the raw `policyContent` for the setting and update the just-created `AgentAudience`:

```go
// After CreateAgentAudience in importFiles()
if policyContent != "" {
    // Parse @settings from raw policy for import-time defaults
    if strings.Contains(policyContent, "@settings") {
        if parsed, err := h.service.parser.ParsePolicy(policyContent, tenantID, audienceType); err == nil && parsed != nil && parsed.Audience != nil {
            // Merge settings into existing audience record
            audienceRecord, _ := h.store.GetAgentAudience(ctx, &store.FindAgentAudience{TenantID: &tenantID, AudienceType: &audienceType})
            if audienceRecord != nil {
                audienceRecord.RequireContactOnFallback = parsed.Audience.RequireContactOnFallback
                h.store.UpdateAgentAudience(ctx, audienceRecord)
            }
        }
    }
}
```

**4b. `HandleReindexTenant` / reindex path** — Ensure reindex reads the current `AgentAudience` setting so that prompt behavior is updated without re-upload. (The reindex itself doesn't rebuild prompts, but the next chat request will pick up the new flag via `LoadConfig` → `GetAgentAudience`.)

**4c. `HandleGetTenantFullConfig`** — Include `requireContactOnFallback` in the response so the admin UI can display it.

---

#### 5. `server/router/api/v1/agent/service.go` — Core logic changes

**5a. Add a helper to check the effective flag**

```go
func (s *Service) shouldCollectContact(config *AudienceConfig) bool {
    if config == nil || config.Audience == nil {
        return true // conservative default
    }
    return config.Audience.RequireContactOnFallback
}
```

**5b. Modify `buildContactInstruction` to conditionally include contact logic**

`buildContactInstruction` currently returns `Section0Addition`, `Rule1Text`, `Rule8Text`, and `RAGFallbackText` unconditionally. Change it to accept the `requireContact` boolean and skip contact instructions when false:

```go
func buildContactInstruction(state ContactState, classification *Classification, validatedPhone string, requireContact bool) ContactInstruction {
    if !requireContact {
        return ContactInstruction{
            Section0Addition: "", // no "customer info already provided" injection
            Rule1Text:        buildRule1(state, false), // fallback=false removes contact asks
            Rule8Text:        "", // no FOLLOW-UP CAPTURE
            RAGFallbackText:  "- If topic not in retrieved context, politely decline.\n",
        }
    }
    // existing logic unchanged
    ...
}
```

**5c. Modify `buildRAGSection0` for RAG path**

`buildRAGSection0` is called directly in `buildRAGSystemPrompt` (line 2691). Guard it:

```go
if requireContact && section0 := buildRAGSection0(state); section0 != "" {
    sb.WriteString(section0)
}
```

Pass `requireContact` into `buildRAGSystemPrompt` and derive it from config.

**5d. Modify `processChat()` post-LLM correctors**

Wrap `CorrectContactsInResponse` and `CorrectEmailsInResponse` in a conditional:

```go
if s.shouldCollectContact(config) {
    validatedPhone := GetValidatedReplacementPhone(...)
    response = CorrectContactsInResponse(response, validatedPhone)
    response = CorrectEmailsInResponse(response, config.Audience.Email)
}
```

Also wrap the post-verification resanitization block (lines 1896–1901) with the same guard.

**5e. Update call sites**

- `generateResponse()` line ~2257: pass `s.shouldCollectContact(config)` into `buildContactInstruction`
- `generateRAGResponse()` / `buildRAGSystemPrompt()`: accept and use the flag
- `processChat()` line 1737: `extractCollectedInfo` can remain (doesn't inject into prompt), but `CustomerName`/`CustomerPhone` storage at line 1738–1746 can be guarded if desired. Safer to leave extraction active so that if a user *does* volunteer contact info, the system doesn't re-ask — but the prompt won't ask for it in the first place.

---

#### 6. Frontend — `web/src/pages/RagStats.tsx` / new admin panel section

**6a. `AgentAdmin.tsx` or tenant config page** — Add a toggle row for "Collect contact info on fallback" under the audience settings section. No separate plan file needed; this is a small UI addition.

**6b. `web/src/store/v2/agentAdmin.ts`** — Add `requireContactOnFallback` to the tenant config fetch and update mutation.

---

#### 7. Tests

- `server/router/api/v1/agent/contact_state_test.go` (if it exists; create if not):
  - Test `buildContactInstruction` with `requireContact=true` preserves existing output
  - Test `buildContactInstruction` with `requireContact=false` strips all contact collection text
- `server/router/api/v1/agent/parser_test.go`:
  - Test `ParsePolicy` with `@settings: require_contact_on_fallback: false` sets the flag on `result.Audience`
- `server/router/api/v1/agent/handlers_test.go`:
  - Test `importFiles` reads `@settings` from policy content

---

### Rollout / Migration Path

1. Deploy migration `016__audience_contact_fallback.sql` — existing tenants default to `true`
2. Deploy backend code changes
3. For the `rizal` tenant specifically:
   - Admin UI: toggle "Collect contact info on fallback" to `false`, OR
   - Update POLICY.MD to include `<!-- @settings: require_contact_on_fallback: false -->` and re-import
4. Verify: ask "look deeper into the story of the leper" → expect "I don't have information about that" with **no** follow-up contact request

### Out-of-Scope (explicitly deferred)

- Removing `extractCollectedInfo` entirely — harmless if left in
- Removing phone/email correctors for tenants that disable contact collection — they still produce correct output, just run unnecessarily; optimize in a follow-up if needed
- Frontend changes beyond the admin toggle — widget itself needs no changes since it just calls `/chat/ext`
