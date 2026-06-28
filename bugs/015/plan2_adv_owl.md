  🔴 Adversarial Review: Plan 2 — Contact-Aware Follow-Up Handling                                                                    
                                                                                                                                        
    Executive Summary                                                                                                                   
                                                                                                                                        
    This plan attempts to fix a real and annoying bug (re-asking for already-provided contact info) by introducing state-awareness      
    into prompt construction. The intent is sound, but the implementation approach contains critical logic gaps, architectural          
    smells, and testing blindspots that will cause erratic LLM behavior in production. I strongly recommend addressing the issues       
    below before writing a single line of code.                                                                                         
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    1. State Extraction Vulnerabilities                                                                                                 
                                                                                                                                        
    🔴 CRITICAL: No Handling of Explicit Declarations/Redactions                                                                        
                                                                                                                                        
    The plan's getContactState will call extractCollectedInfo, which is a one-way accumulator — it only ever adds or overwrites (via    
    correction patterns). It never removes. Consider:                                                                                   
                                                                                                                                        
    │ User (turn 3): "My phone is 555-123-4567"                                                                                         
    │ User (turn 7): "Don't contact me at 555-123-4567 anymore"                                                                         
    │ User (turn 8): "What are your pricing plans?"                                                                                     
                                                                                                                                        
    The current extractCollectedInfo will return Phone: "555-123-4567" because it found a correction pattern or direct mention. The     
    plan's HasEmailOrPhone will be true, and the prompt will tell the LLM "the customer has provided contact info" — but the customer   
    has explicitly retracted it. The LLM will then say something like "I have your information and can pass this along" — directly      
    contradicting the user's wishes.                                                                                                    
                                                                                                                                        
    There is no negation detection anywhere in the extraction pipeline. Phrases like:                                                   
                                                                                                                                        
    • "Don't use that number"                                                                                                           
    • "I'd rather not give my email"                                                                                                    
    • "Forget I said that"                                                                                                              
    • "That's the wrong number"                                                                                                         
                                                                                                                                        
    ...all get silently ignored, and the stale data persists.                                                                           
                                                                                                                                        
    🔴 CRITICAL: IsComplete is Undefined in the Plan                                                                                    
                                                                                                                                        
    The plan references IsComplete as a boolean in ContactState, but nowhere does it define what makes state "complete." Looking at     
    the existing CollectedCustomerInfo struct, there is no such field. The plan needs to explicitly define:                             
                                                                                                                                        
    • Is "complete" = Name + (Email OR Phone)?                                                                                          
    • Is "complete" = Name + Email + Phone?                                                                                             
    • What about Address — does it count?                                                                                               
                                                                                                                                        
    Without a clear definition, the implementation will be arbitrary and inconsistent.                                                  
                                                                                                                                        
    🟡 MEDIUM: Fake/Malformed Data Propagation                                                                                          
                                                                                                                                        
    The existing extractCollectedInfo has minimal validation:                                                                           
                                                                                                                                        
    • Email: Only checks against a tiny placeholder domain list (example.com, test.com, etc.). An email like asdf@asdf.com passes       
    through.                                                                                                                            
    • Phone: Only rejects 555-01XX and obvious placeholders. A real-but-wrong number like 555-987-6543 (a real number belonging to      
    someone else) gets captured and injected into the prompt.                                                                           
    • Name: The isCommonWord filter is a hardcoded allowlist of ~30 words. It misses tons of false positives ("Chris", "Pat",           
    "Jordan" — wait, those are names, but "May" is both a name and a month).                                                            
                                                                                                                                        
    The plan does zero additional validation on the extracted data before injecting it into the prompt. If a user jokingly says "My     
    name is Mickey Mouse", that becomes ContactState.Name = "Mickey Mouse" and gets injected as fact.                                   
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    2. Prompt Engineering Conflicts                                                                                                     
                                                                                                                                        
    🔴 CRITICAL: Contradictory Instructions in buildSystemPrompt                                                                        
                                                                                                                                        
    The plan modifies Section 1, Rule 1 to dynamically change wording based on state. But it fails to address the existing              
    contradiction in Section 0:                                                                                                         
                                                                                                                                        
    Current Section 0 (line 2240):                                                                                                      
                                                                                                                                        
    │ "IMPORTANT: Do NOT ask for this information again. Acknowledge that you have it.\n"                                               
                                                                                                                                        
    And Section 1, Rule 1 (line 2283-2284):                                                                                             
                                                                                                                                        
    │ "If a customer asks about a service not listed, say 'I don't have information about that service' and offer to collect their      
    name plus email or phone for follow-up."                                                                                            
                                                                                                                                        
    The plan modifies Rule 1 to be dynamic, but Section 0 remains unchanged. This creates a scenario where:                             
                                                                                                                                        
    1. Section 0 says: "DO NOT ask for this info again. Acknowledge that you have it."                                                  
    2. Section 1 (modified) says: "acknowledge you have their contact information and say the team can follow up."                      
                                                                                                                                        
    These are compatible but redundant. The LLM receives the same instruction twice with different phrasing. In my experience with      
    LLM prompt compliance, redundant instructions with slightly different wording can cause the model to fixate on the more specific    
    one (Section 0) and ignore the dynamic one in Section 1 — or vice versa, depending on position bias.                                
                                                                                                                                        
    🔴 CRITICAL: The "Instead say..." Instruction is Too Prescriptive                                                                   
                                                                                                                                        
    The plan proposes injecting:                                                                                                        
                                                                                                                                        
    │ "Instead say 'I have your contact information and can pass this along for follow-up.'"                                            
                                                                                                                                        
    This is a literal string injection telling the LLM exactly what to say. This is dangerous because:                                  
                                                                                                                                        
    1. It breaks conversational flow — the LLM will insert this phrase verbatim in contexts where it doesn't fit.                       
    2. It creates a foot-in-the-door pattern where the LLM may use this phrase even when the user's question is not a fallback          
    scenario (see Edge Cases below).                                                                                                    
    3. The existing Section 0 already says "Acknowledge that you have it" — so now we have three places telling the LLM about contact   
    info.                                                                                                                               
                                                                                                                                        
    🟡 MEDIUM: The RAG Prompt Has a Different Structure Altogether                                                                      
                                                                                                                                        
    Looking at buildRAGSystemPrompt (line 2716), the fallback instruction is:                                                           
                                                                                                                                        
    │ "- If topic not in retrieved context, politely decline and offer to collect the customer's name plus email or phone for           
    follow-up"                                                                                                                          
                                                                                                                                        
    This is a bullet point in a constraints section, not a numbered rule. The plan says to apply "identical" logic here, but the        
    surrounding context is completely different. In the RAG prompt, there's no Section 0 equivalent that says "DO NOT ask again" — so   
    the dynamic injection here might actually be more prone to hallucination because the LLM has no pre-context about already having    
    the info.                                                                                                                           
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    3. Architecture & Maintainability                                                                                                   
                                                                                                                                        
    🔴 CRITICAL: Code Duplication is Inevitable with This Approach                                                                      
                                                                                                                                        
    The plan calls for:                                                                                                                 
                                                                                                                                        
    1. Computing getContactState at the top of buildSystemPrompt                                                                        
    2. Computing getContactState again at the top of buildRAGSystemPrompt                                                               
    3. Writing dynamic string injection logic in both functions                                                                         
                                                                                                                                        
    These two functions are already enormous (buildSystemPrompt is ~400 lines, buildRAGSystemPrompt is ~200 lines). Adding 30+ lines    
    of contact-state logic to each is a maintenance nightmare. When (not if) the business rules change, someone must remember to        
    update both functions identically.                                                                                                  
                                                                                                                                        
    🟡 MEDIUM: No Abstraction Layer                                                                                                     
                                                                                                                                        
    The plan uses raw sb.WriteString concatenation for dynamic prompt assembly. This is the same anti-pattern that created the          
    original bug. A better approach would be:                                                                                           
                                                                                                                                        
    • A ContactInstructionBuilder that takes ContactState and returns the appropriate instruction strings                               
    • Or even better, Go's text/template for the prompt sections                                                                        
                                                                                                                                        
    The current approach means that if someone adds a new state (e.g., HasAddress but no email/phone), they must modify string          
    concatenation in 4+ places (Section 0, Rule 1, Rule 8, RAG fallback).                                                               
                                                                                                                                        
    🟡 MEDIUM: No Separation of Concerns                                                                                                
                                                                                                                                        
    buildSystemPrompt already does: phone validation, observation log injection, customer info extraction, service listing, exclusion   
    listing, script injection, FAQ injection, etc. Adding contact-state computation violates the single-responsibility principle.       
    The prompt builder should receive a pre-computed ContactState, not compute it internally.                                           
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    4. Edge Cases                                                                                                                       
                                                                                                                                        
    🔴 CRITICAL: The Dynamic Injection Applies to ALL Intents, Not Just Fallbacks                                                       
                                                                                                                                        
    The plan modifies Section 1, Rule 1 which applies to every out-of-scope service inquiry. But the ContactState is computed from      
    the entire session history. Consider:                                                                                               
                                                                                                                                        
    │ User (turn 1): "Hi, I'm John, my email is john@example.com"                                                                       
    │ User (turn 2): "What are your business hours?"                                                                                    
    │ LLM: "We're open 9-5 Monday through Friday."                                                                                      
    │ User (turn 3): "What about pricing?"                                                                                              
                                                                                                                                        
    In turn 3, the ContactState.IsComplete is true. The plan's modified Rule 1 now says:                                                
                                                                                                                                        
    │ "...acknowledge you have their contact information and say the team can follow up."                                               
                                                                                                                                        
    But the user is asking about pricing — potentially an in-scope question! The LLM might respond with: "I have your contact           
    information and can pass this along for follow-up" instead of answering the pricing question, because the dynamic instruction is    
    not conditional on the current intent being a fallback.                                                                             
                                                                                                                                        
    The plan conflates "has contact info" with "should mention contact info." These are orthogonal concerns.                            
                                                                                                                                        
    🟡 MEDIUM: Partial Info Scenarios Create Awkward Prompts                                                                            
                                                                                                                                        
    The plan defines three partial states:                                                                                              
                                                                                                                                        
    • HasName only → "ask only for email or phone"                                                                                      
    • HasEmailOrPhone only → "ask only for name"                                                                                        
                                                                                                                                        
    But the existing Section 0 already lists what info was provided:                                                                    
                                                                                                                                        
      - Customer Name: John Smith                                                                                                       
                                                                                                                                        
    So the LLM sees: "You have their name. Do NOT ask for it again. Also, ask only for email or phone." This is three separate          
    instructions about the same topic. The cognitive load on the LLM increases, and compliance typically drops.                         
                                                                                                                                        
    🟡 MEDIUM: Race Condition with Session State                                                                                        
                                                                                                                                        
    extractCollectedInfo runs over session.Messages. But looking at line 1690-1699, the user's current message is appended to           
    session.Messages before extractCollectedInfo is called. However, buildSystemPrompt is called later in the flow. If there are        
    concurrent messages (e.g., user sends two messages rapidly), the state could be computed on a different message set than the one    
    used for the actual prompt construction.                                                                                            
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    5. Testing Blindspots                                                                                                               
                                                                                                                                        
    🔴 CRITICAL: Missing Test Cases                                                                                                     
                                                                                                                                        
    The verification plan only tests 3 scenarios. Here are the critical scenarios it completely misses:                                 
                                                                                                                                        
      #    Scenario                                                                    What It Tests                                    
      ───  ──────────────────────────────────────────────────────────────────────────  ────────────────────────────────────             
      4    User provides info, then retracts it                                        Negation handling                                
      5    User provides fake data (Mickey Mouse, 555-0100)                            Validation                                       
      6    User provides info across multiple turns (name in turn 1, email in turn 5)  Accumulation                                     
      7    User asks in-scope question AFTER providing info                            Intent/state decoupling                          
      8    User provides email but no name/phone                                       Partial state (HasEmailOrPhone only)             
      9    RAG path with complete info                                                 RAG-specific behavior                            
      10   Concurrent messages / rapid fire                                            Race conditions                                  
      11   User provides info, then asks a question the KB CAN answer                  No spurious "I have your info"                   
      12   Empty session (no messages)                                                 Nil pointer safety                               
                                                                                                                                        
    🟡 MEDIUM: No Unit Test for getContactState Logic                                                                                   
                                                                                                                                        
    The plan says to "run go test ./..." but doesn't specify writing new unit tests for the getContactState function. This is the       
    core logic — it deserves isolated tests covering all 4 states (complete, partial-name, partial-email/phone, none) plus edge cases   
    (negation, malformed data).                                                                                                         
                                                                                                                                        
    🟡 MEDIUM: No Integration Test for Prompt Assembly                                                                                  
                                                                                                                                        
    The manual verification relies on booting the full app and eyeballing LLM output. There's no automated test that:                   
                                                                                                                                        
    1. Constructs a ContactState                                                                                                        
    2. Calls the prompt builder                                                                                                         
    3. Asserts the output contains/doesn't contain specific strings                                                                     
                                                                                                                                        
    This is testable with a simple unit test that checks the string output, without needing an LLM.                                     
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    6. Additional Issues Not in the Original Vectors                                                                                    
                                                                                                                                        
    🟡 MEDIUM: buildRAGSystemPrompt Lacks Section 0's Explicit "DO NOT ASK" Instruction                                                 
                                                                                                                                        
    The RAG prompt (line 2678-2698) has a "CUSTOMER INFO (DO NOT ASK AGAIN)" section, but it's less emphatic than the full prompt's     
    Section 0. It doesn't have the === header style or the "IMPORTANT: Do NOT ask for this information again" line. This means the      
    RAG path will be less compliant with the contact-aware instructions.                                                                
                                                                                                                                        
    🟡 MEDIUM: The Plan Doesn't Address session.CustomerName/session.CustomerPhone Persistence                                          
                                                                                                                                        
    Lines 1700-1708 show that extracted info is also persisted to session.CustomerName, session.CustomerPhone, etc. These are used      
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    Recommendations                                                                                                                     
                                                                                                                                        
    Recommendation 1: Add Negation Detection to extractCollectedInfo Before Anything Else                                               
                                                                                                                                        
    Before implementing the prompt changes, add a negation pass to the extraction function. If a message contains patterns like         
    "don't contact me," "forget that number," "I take back my email," etc., the corresponding field in CollectedCustomerInfo should     
    be cleared. This is the single highest-impact fix.                                                                                  
                                                                                                                                        
    Recommendation 2: Decouple State from Prompt — Build a ContactInstruction Abstraction                                               
                                                                                                                                        
    Instead of duplicating logic across buildSystemPrompt and buildRAGSystemPrompt, create:                                             
                                                                                                                                        
      ─ go                                                                                                                              
      type ContactInstruction struct {                                                                                                  
          Section0Addition    string  // "DO NOT ASK AGAIN" block                                                                       
          Rule1Text           string  // Dynamic fallback instruction                                                                   
          Rule8Text           string  // FOLLOW-UP CAPTURE line                                                                         
          RAGFallbackText     string  // RAG-specific fallback                                                                          
      }                                                                                                                                 
      func buildContactInstruction(state ContactState, classification *Classification) ContactInstruction {                             
          // All logic lives here, testable in isolation                                                                                
      }                                                                                                                                 
                                                                                                                                        
    This eliminates the code duplication, makes the logic unit-testable without booting the LLM, and ensures consistency across both    
    prompt builders.                                                                                                                    
                                                                                                                                        
    Recommendation 3: Scope the Dynamic Injection to Out-of-Scope Intents Only                                                          
                                                                                                                                        
    The dynamic "I have your info" instruction should only be injected when classification.PrimaryIntent == "out_of_coverage" or        
    equivalent. For in-scope questions, the existing Section 0 instruction ("Do NOT ask for this info again") is sufficient. This       
    prevents the LLM from awkwardly inserting "I have your contact information" in the middle of answering a legitimate                 
     uestion.                                                                                                                           
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    Verdict                                                                                                                             
                                                                                                                                        
    DO NOT implement as-is. The plan has a solid conceptual foundation but will introduce new bugs (negation ignorance,                 
    intent-agnostic injection, redundant instructions) while only partially solving the original problem. Address the three             
    recommendations above, then the plan will be production-ready.       