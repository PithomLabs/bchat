 Adversarial Review: Plan 3 — "Ship It" Edition                                                                                   
                                                                                                                                        
    Constraint acknowledged: we don't have time for endless iterations. This review focuses only on things that will actually break     
    in production vs. things that are "not ideal but survivable."                                                                       
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    Verdict: ✅ Good enough to implement, with 2 must-fix items                                                                         
                                                                                                                                        
    The plan correctly addresses all 3 recommendations from the previous review. The architecture is sound, the intent-scoping is       
    correct, and the test plan covers the critical paths. However, there are 2 issues that will manifest in production and need to be   
    addressed before or during implementation, plus 1 "fix if you have 5 minutes" item.                                                 
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    🔴 MUST-FIX #1: PrimaryIntent is a free-form LLM output — not a reliable enum                                                       
                                                                                                                                        
    The plan's entire intent-scoping logic hinges on:                                                                                   
                                                                                                                                        
      ─ go                                                                                                                              
      classification.PrimaryIntent == "out_of_coverage"                                                                                 
                                                                                                                                        
    But looking at classifyIntent (line 1926-2006), PrimaryIntent is parsed from arbitrary LLM-generated JSON. The LLM classifier is    
    given a list of intent codes from config.Intents (tenant-defined), but it can return any string. In practice, I've seen LLM         
    classifiers return:                                                                                                                 
                                                                                                                                        
    • "out_of_coverage" (exact match ✅)                                                                                                
    • "out_of_scope" (synonym — miss ❌)                                                                                                
    • "not_in_coverage" (creative synonym — miss ❌)                                                                                    
    • "" (empty on parse failure)                                                                                                       
    • "unknown" (the fallback default)                                                                                                  
                                                                                                                                        
    Impact: If a tenant's intent codes don't use the exact string "out_of_coverage", the dynamic injection silently falls through to    
    the "no contact info" path, and the bug reappears. This is not a theoretical concern — it's a config-dependent behavior that will   
    bite some tenants and not others.                                                                                                   
                                                                                                                                        
    Fix (5 minutes): Define a set of fallback-matching constants and use strings.Contains or a helper:                                  
                                                                                                                                        
      ─ go                                                                                                                              
      func isFallbackIntent(intent string) bool {                                                                                       
          switch intent {                                                                                                               
          case "out_of_coverage", "out_of_scope", "not_found", "unsupported":                                                           
              return true                                                                                                               
          }                                                                                                                             
          return false                                                                                                                  
      }                                                                                                                                 
                                                                                                                                        
    Or better: check if the intent is NOT in the set of known in-scope intents (derived from config.Intents).                           
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    🔴 MUST-FIX #2: Negation detection will false-positive on assistant messages                                                        
                                                                                                                                        
    The plan says:                                                                                                                      
                                                                                                                                        
    │ "Before returning the accumulated CollectedCustomerInfo, apply regular expressions to detect negation phrases"                    
                                                                                                                                        
    But extractCollectedInfo iterates over all messages including assistant messages (line 2927-2930 shows it filters msg.Role ==       
    "user", which is good). However, the negation patterns mentioned ("don't contact me", "wrong number") could appear in assistant     
    messages if the LLM says something like:                                                                                            
                                                                                                                                        
    │ "Got it, I won't contact you at 555-1234 anymore."                                                                                
                                                                                                                                        
    If the negation pass runs over the full message history (including assistant turns), it could clear fields that the user never      
    asked to clear. The current code only scans role == "user" messages for extraction, but the plan doesn't explicitly state whether   
    the negation pass should also be user-only.                                                                                         
                                                                                                                                        
    Fix (3 minutes): Ensure negation detection only scans user messages. If the existing loop already filters msg.Role == "user", add   
    a comment in the plan making this explicit. If not, add the filter.                                                                 
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    🟡 NICE-TO-HAVE: IsComplete definition is still slightly ambiguous                                                                  
                                                                                                                                        
    The plan defines it as HasName && HasEmailOrPhone. But what about:                                                                  
                                                                                                                                        
    • User provides email + phone but no name → IsComplete = false, HasEmailOrPhone = true                                              
    • The prompt will say "ask only for name" — which is correct ✅                                                                     
    • But Section 0 will still show "CUSTOMER INFO ALREADY PROVIDED" with email and phone listed, and say "Do NOT ask for this          
    information again" — which is also correct ✅                                                                                       
                                                                                                                                        
    This actually works fine. The only edge case is: if a user provides only email (no name, no phone), the prompt will say "ask only   
    for name" but also "Do NOT ask for email again." That's slightly confusing but not broken — the LLM will typically infer "ask       
    for name, you already have email."                                                                                                  
                                                                                                                                        
    Verdict: Ship it. Revisit if users report awkwardness.                                                                              
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    What the plan gets right (no changes needed)                                                                                        
                                                                                                                                        
      Item                                        Status                                                                                
      ──────────────────────────────────────────  ─────────────────────────────────────────────────────────                             
      ContactInstruction abstraction              ✅ Clean, testable, DRY                                                               
      Intent-scoped dynamic injection             ✅ Prevents spurious "I have your info" in normal answers                             
      Negation detection (conceptually)           ✅ Closes the retraction loophole                                                     
      Unit test plan for buildContactInstruction  ✅ Covers all state × intent permutations                                             
      Manual scenarios (5 scenarios)              ✅ Covers the critical paths                                                          
      RAG parity                                  ✅ Explicitly tested                                                                  
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    Final Recommendation                                                                                                                
                                                                                                                                        
    Ship it. Fix the two must-fix items during implementation (they're both ~5-line changes). The plan is minimal, addresses the root   
    cause, and won't introduce new failure modes beyond the two identified above. The "nice-to-have" can be a follow-up ticket.   
