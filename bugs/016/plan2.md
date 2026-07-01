# Decouple Contact Collection Logic from System Prompt

Currently, the `bchat` backend hardcodes aggressive contact information collection (progressive profiling) into the system prompt for all fallback scenarios (e.g., when the RAG pipeline doesn't have an answer). This is injected via `buildRule1`, `buildRule8`, and `buildRAGFallback` in `service.go`. 

To make this generic for knowledge bases, we need to decouple this logic so it's not forced on every tenant.

## Open Questions

> [!IMPORTANT]
> **Design Decision Required**
> There are two main ways to approach this decoupling. Please review the options below and let me know which direction you prefer:
> 
> **Option 1: Complete Removal (Configuration via `POLICY.MD` Rules)**
> - We completely strip out the hardcoded contact collection logic from `service.go`.
> - The prompt will simply say: "If topic not in retrieved context, politely decline."
> - If a tenant *wants* to collect leads on fallbacks, they must explicitly write it into their `POLICY.MD` file as a custom `<!-- @rule -->`.
> - *Pros*: Truly generic, 100% policy-driven.
> - *Cons*: We lose the built-in "progressive profiling" (the code currently checks `ContactState` and smartly asks *only* for the missing piece of info, like asking for email if name is already provided).
> 
> ## Proposed Changes

We will implement the **Configuration Flag** approach.

### `store/` (Database & Models)

#### [MODIFY] [store/agent.go](file:///home/chaschel/Documents/go/bchat/store/agent.go)
- Add `RequireContactOnFallback bool` to the `AgentAudience` struct.

#### [NEW] [store/migration/sqlite/016__audience_contact_fallback.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/016__audience_contact_fallback.sql)
- Create a new migration file to add the column:
  `ALTER TABLE agent_audience ADD COLUMN require_contact_on_fallback BOOLEAN DEFAULT 1;`

---

### `server/router/api/v1/agent/` (Logic & Parsing)

#### [MODIFY] [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go)
- Add support for a new `settings` annotation type in `ParsePolicy` (e.g., `<!-- @settings: require_contact_on_fallback: false -->`).
- **Fix**: Inside `ParsePolicy`, when handling `@settings` (or `@thresholds`), ensure `result.Audience` is safely initialized (nil-check) before assigning fields to prevent panics when `@thresholds` is missing.

#### [MODIFY] [handlers.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/handlers.go)
- **Fix**: In `importFiles()`, the `@settings` annotation would currently be ignored because the audience is created with hardcoded defaults. Update `importFiles()` to either pre-parse the policy content to extract settings *before* creating the `AgentAudience` record, or update the record immediately after parsing.

#### [MODIFY] [service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go)
- **Helper**: Create a `shouldCollectContact(config *AudienceConfig) bool` helper function that defaults to true but checks `config.Audience.RequireContactOnFallback`.
- **RAG Fallback Prompt**: Update `buildRAGFallback` to accept the flag. If disabled, return only "- If topic not in retrieved context, politely decline." without asking for follow-up contact info.
- **Out-of-Coverage Prompt**: Update `buildRule1` and `buildRule8` to also respect the flag, omitting the "FOLLOW-UP CAPTURE" instructions if disabled.
- **RAG Section 0**: In `buildRAGSystemPrompt` (around line 2691), wrap the call to `buildRAGSection0(contactState)` with `if shouldCollectContact(config)` so previously collected info isn't injected into the prompt when disabled.
- **Post-LLM Correctors**: In the `generateResponse` / `generateRAGResponse` (or `processChat`) logic around line 1872, wrap `CorrectContactsInResponse` and `CorrectEmailsInResponse` with `if shouldCollectContact(config)`. This prevents the sanitizers from hallucinating or inserting placeholder emails when contact collection is intentionally bypassed.

## Verification Plan

### Automated Tests
- Run backend tests: `task test:backend`
- Update `server/router/api/v1/agent/contact_state_test.go` to test prompt generation with `RequireContactOnFallback` set to both `true` and `false`.

### Manual Verification
- Re-import the `POLICY.MD` for the `rizal` tenant with `<!-- @settings: require_contact_on_fallback: false -->`.
- Test the chat widget with the query "look deeper into the story of the leper".
- Verify that the agent replies stating it doesn't have the information *without* asking for a name or email.
- Verify that post-LLM correctors didn't kick in.
