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
> **Option 2: Configuration Flag (Recommended)**
> - We introduce a new setting in `POLICY.MD` (e.g. `<!-- @setting: require_contact_on_fallback, value: false -->`).
> - We add a `RequireContactOnFallback` boolean to the `AgentAudience` database schema.
> - If the flag is false, the system prompt just says "politely decline". If true (or default), it uses the smart progressive profiling logic.
> - *Pros*: Preserves the advanced progressive profiling capabilities for sales/support agents while allowing generic knowledge bases (like Rizals) to easily turn it off.

---

## Proposed Changes (Assuming Option 2 - Configuration Flag)

If we go with **Option 2**, here is the technical plan:

### `store/` (Database & Models)

#### [MODIFY] [store/agent.go](file:///home/chaschel/Documents/go/bchat/store/agent.go)
- Add `RequireContactOnFallback bool` to the `AgentAudience` struct.

#### [NEW] [store/migration/sqlite/016__audience_contact_fallback.sql](file:///home/chaschel/Documents/go/bchat/store/migration/sqlite/016__audience_contact_fallback.sql)
- Create a new migration file to add the column:
  `ALTER TABLE agent_audience ADD COLUMN require_contact_on_fallback BOOLEAN DEFAULT 1;`

---

### `server/router/api/v1/agent/` (Logic & Parsing)

#### [MODIFY] [parser.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/parser.go)
- Add support for a new `settings` annotation type in `ParsePolicy`:
  `<!-- @settings: require_contact_on_fallback: false -->`
- Update the `AgentAudience` initialization in `ParsePolicy` to map this setting.

#### [MODIFY] [service.go](file:///home/chaschel/Documents/go/bchat/server/router/api/v1/agent/service.go)
- Update `buildContactInstruction`, `buildRule1`, `buildRule8`, and `buildRAGFallback` to accept the `RequireContactOnFallback` boolean.
- If `RequireContactOnFallback` is `false`:
  - `buildRule1`: Append only "say 'I don't have information about that service'."
  - `buildRule8`: Omit the entire "FOLLOW-UP CAPTURE" section.
  - `buildRAGFallback`: Return only "- If topic not in retrieved context, politely decline."
- Pass `config.Audience.RequireContactOnFallback` (or similar derived value) into the `buildContactInstruction` call inside `generateResponse` and `generateRAGResponse`.

## Verification Plan

### Automated Tests
- Run backend tests: `task test:backend`
- Update `server/router/api/v1/agent/contact_state_test.go` to test prompt generation with `RequireContactOnFallback` set to both `true` and `false`.

### Manual Verification
- Re-import the `POLICY.MD` for the `rizal` tenant with the new setting set to `false`.
- Test the chat widget with the query "look deeper into the story of the leper".
- Verify that the agent replies stating it doesn't have the information *without* asking for a name or email.
