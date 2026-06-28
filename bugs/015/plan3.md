# Robust Contact-Aware Follow-Up Handling (Revised)

This implementation plan incorporates the recommendations from the adversarial review (`plan2_adv_owl.md`) to create a robust, bulletproof solution for the prompt-instruction conflict. It addresses state extraction vulnerabilities, architectural code duplication, prompt engineering conflicts, and testing blindspots.

## Proposed Changes

### 1. State Extraction & Negation Detection (`server/router/api/v1/agent/service.go`)

- **Add Negation Pass to `extractCollectedInfo`**:
  Before returning the accumulated `CollectedCustomerInfo`, apply regular expressions to detect redaction or negation phrases (e.g., "don't contact me", "wrong number", "forget that number", "don't use my email").
  If a negation pattern matches, clear the corresponding field (`Phone`, `Email`, or `Name`) from the extracted state.
- **Define Explicit Contact State**:
  Create the `ContactState` struct with explicit definitions. `IsComplete` is strictly defined as having a Name AND (Email OR Phone).
  ```go
  type ContactState struct {
      Name            string
      Phone           string
      Email           string
      Address         string
      HasName         bool
      HasEmailOrPhone bool
      IsComplete      bool // Explicitly: HasName && HasEmailOrPhone
  }
  ```

### 2. ContactInstruction Abstraction (`server/router/api/v1/agent/service.go`)

- **Create Abstraction Layer**:
  Instead of scattering string concatenations across multiple massive prompt builder functions, centralize the logic into a highly testable builder function:
  ```go
  type ContactInstruction struct {
      Section0Addition string  // "DO NOT ASK AGAIN" block (if info exists)
      Rule1Text        string  // Dynamic fallback instruction for out-of-scope services
      Rule8Text        string  // FOLLOW-UP CAPTURE line
      RAGFallbackText  string  // RAG-specific fallback
  }

  func buildContactInstruction(state ContactState, classification *Classification) ContactInstruction {
      // Computes the exact strings needed based on state and intent
  }
  ```

### 3. Scope Dynamic Injection to Intent and State

- **Resolve Prompt Conflicts inside `buildContactInstruction`**:
  - **Section 0**: If any info exists, generate the standard "DO NOT ASK AGAIN" block. (Apply this to both long-context and RAG prompts for consistency).
  - **Dynamic Fallbacks (Rule 1 & RAG Fallback)**:
    - *Only* inject the explicit "I have your contact info and will pass this along" instruction if `classification.PrimaryIntent == "out_of_coverage"` (or equivalent fallback scenarios).
    - If it is an in-scope question, keep the fallback rule mild (e.g., "If asked about unlisted services, decline politely. Since you have their contact info, do not ask for it again.") to prevent the LLM from forcefully injecting follow-up language into normal answers.
    - Handle partial states by asking *only* for the missing fields (e.g., if `HasName`, ask only for email or phone).

### 4. Refactor Prompt Builders

- **Update `buildSystemPrompt` & `buildRAGSystemPrompt`**:
  - Call `getContactState` and `buildContactInstruction` at the top of the functions.
  - Replace the hardcoded string injections for Section 0, Rule 1, Rule 8, and the RAG fallback with the fields from the returned `ContactInstruction` struct.

## Verification Plan

### Automated Tests
- **Unit Test `getContactState` / `extractCollectedInfo`**: Add tests for negation handling ("don't contact me"), malformed data, and accumulation across multiple turns.
- **Unit Test `buildContactInstruction`**: Add isolated tests to verify the exact string outputs for all permutations of `ContactState` (Complete, Name-Only, Email-Only, None) and `Classification` (In-Scope, Out-of-Scope).

### Manual Verification
- **Scenario 1 (Negation)**: User provides info, then retracts it. Verify the agent asks for info again if an out-of-scope question is asked.
- **Scenario 2 (In-Scope with Info)**: User provides info, then asks an in-scope question. Verify the agent just answers the question without spuriously saying "I have your info".
- **Scenario 3 (Out-of-Scope with Info)**: User provides complete info, then asks an out-of-scope question. Verify the agent declines the question and acknowledges it has the contact info.
- **Scenario 4 (Partial Info)**: User provides name only, asks out-of-scope question. Verify the agent asks only for email/phone.
- **Scenario 5 (RAG Path)**: Force RAG mode and verify compliance across the scenarios above.
