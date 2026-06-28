# Fix Re-Asking For Already Provided Lead Info

This plan implements the robust contact-aware follow-up handling strategy to prevent the agent from re-asking for contact information that the customer has already provided. This resolves conflicting LLM prompt instructions that were telling the agent to unconditionally ask for contact info during fallbacks.

## Proposed Changes

### Backend Component

#### [MODIFY] `server/router/api/v1/agent/service.go`

- **Define Contact State Helper**:
  Add a new struct and helper function to compute contact state based on existing extraction logic.
  ```go
  type ContactState struct {
      Name            string
      Phone           string
      Email           string
      Address         string
      HasName         bool
      HasEmailOrPhone bool
      IsComplete      bool
  }
  
  func getContactState(session *store.AgentSession, validatedPhone string) ContactState {
      // Calls extractCollectedInfo and computes boolean flags
  }
  ```

- **Update `buildSystemPrompt`**:
  - Call `getContactState` at the beginning of the prompt generation.
  - Refactor **Section 0 (CUSTOMER INFO ALREADY PROVIDED)**: If `IsComplete` is true, inject a strong, high-priority rule telling the agent: *"Never ask whether they want to provide contact info. Never say 'I can collect your contact information'. Instead say 'I have your contact information and can pass this along for follow-up.'"*
  - Refactor **Section 1 (Critical Constraints - Rule 1)**: Replace the hardcoded string `"offer to collect their name plus email or phone for follow-up"` with dynamic wording based on `ContactState`:
    - **IsComplete:** `"...acknowledge you have their contact information and say the team can follow up."`
    - **HasName only:** `"...and ask only for their email or phone for follow-up."`
    - **HasEmailOrPhone only:** `"...and ask only for their name for follow-up."`
    - **No contact info:** `"...and ask for their name and either email or phone for follow-up."`
  - Refactor **Section 1 (Critical Constraints - Rule 8 FOLLOW-UP CAPTURE)**: Update the explanation to match the dynamic state rules instead of unconditionally asking.

- **Update `buildRAGSystemPrompt`**:
  - Apply the exact same `getContactState` logic at the top of the function.
  - Refactor the fallback instruction (around line 2716: `"- If topic not in retrieved context, politely decline and offer to collect the customer's name plus email or phone for follow-up"`) to use the identical dynamic wording based on the computed contact state.

## Verification Plan

### Automated Tests
- Run `go test ./...` in the backend to ensure no build failures and that any existing unit tests still pass.

### Manual Verification
- Boot the application using `task run:rag:mock` or `task dev:backend`.
- Initiate an external widget chat session.
- **Scenario 1 (Complete Info)**: Send `"Hi, my name is Ada Lovelace and my email is ada@example.org. I need help with pricing."` (Assuming pricing is out of scope).
  - *Verify*: The agent declines the pricing request and states it already has the contact info to pass along, without asking for name/email again.
- **Scenario 2 (Partial Info)**: Send `"My name is John Smith. What is your pricing?"`.
  - *Verify*: The agent asks *only* for email or phone.
- **Scenario 3 (No Info)**: Send `"What is your pricing?"`.
  - *Verify*: The agent asks for both name and email/phone.
- Confirm lead persistence continues to capture leads correctly in the database.
