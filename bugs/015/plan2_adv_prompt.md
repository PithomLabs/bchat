Act as a Principal Software Engineer and Adversarial System Reviewer. Your task is to tear apart the proposed implementation plan for a Go-based AI agent backend. 

### Context:
The system is an AI agent platform that relies on extracting customer contact info (name, email, phone) from chat history to capture leads. The current bug is a "prompt-instruction conflict": the system extracts the data perfectly, but the hardcoded system prompt unconditionally tells the LLM to "offer to collect name and email/phone for follow-up" when it hits a fallback scenario (e.g., answering an out-of-scope question). This results in the LLM acknowledging the contact info, but annoyingly asking for it again.

### The Proposed Implementation Plan:
read /home/chaschel/Documents/go/bchat/bugs/015/plan2.md for the plan

### Your Mission:
Perform a ruthless, adversarial review of this plan. I want you to look for loopholes, edge cases, architectural smells, and logical failures. Specifically, evaluate the plan against the following vectors:

1. **State Extraction Vulnerabilities:** The plan relies on a `ContactState` helper that runs over the session history on every turn. What happens if a user provides a fake or malformed email/phone? What if the user says "Don't contact me at 555-1234 anymore"? Does the boolean logic (`HasEmailOrPhone`) account for explicit declines or redactions?
2. **Prompt Engineering Conflicts:** We are dynamically altering string constants inside a massive system prompt based on state. Will changing the fallback instruction from "ask for info" to "I have your info" inadvertently cause the LLM to hallucinate follow-up promises? Are there other sections of the prompt that might contradict this dynamic injection?
3. **Architecture & Maintainability:** Is duplicating the dynamic string injection logic across both `buildSystemPrompt` and `buildRAGSystemPrompt` a code smell? Should this be handled via a templating engine or a unified fallback instruction builder instead of raw string concatenation (`sb.WriteString`)?
4. **Edge Cases:** What happens if `IsComplete` is true, but the LLM decides the user's intent is NOT a fallback/out-of-scope question? Will the injected instruction "Instead say 'I have your contact information...'" force the LLM to say that phrase awkwardly in the middle of a normal conversation?
5. **Testing Blindspots:** Look at the Verification Plan. What critical test cases are missing? 

Do not hold back. Be highly critical and point out any area where this plan might fail in a production environment. Conclude your review with 2-3 specific, actionable recommendations to bulletproof the plan before coding begins.
