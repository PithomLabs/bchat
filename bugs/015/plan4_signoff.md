 Final Adversarial Review: Plan 4                                                                                                 
                                                                                                                                        
    Verdict: ✅ Ship it.                                                                                                                
                                                                                                                                        
    All critical issues from prior rounds are resolved. The two must-fix items from Plan 3 are explicitly addressed. Remaining          
    concerns are edge cases that won't manifest at scale or are acceptable trade-offs for a minimum viable plan.                        
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    What was fixed from prior reviews                                                                                                   
                                                                                                                                        
      Previous Issue                             Status                                                                                 
      ─────────────────────────────────────────  ──────────────────────────────────────────                                             
      No negation detection                      ✅ User-scoped negation pass added                                                     
      PrimaryIntent string fragility             ✅ isFallbackIntent() helper with synonyms                                             
      Assistant message false-positive negation  ✅ Explicitly scoped to msg.Role == "user"                                             
      Redundant prompt instructions              ✅ Scoped to fallback intents only                                                     
      Code duplication                           ✅ ContactInstruction abstraction                                                      
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    Residual risks (acceptable for MVP)                                                                                                 
                                                                                                                                        
    🟡 LOW: isFallbackIntent still won't catch tenant-specific intent codes                                                             
                                                                                                                                        
    If a tenant defines their own intent like "pricing_not_listed", it won't match the hardcoded switch. However:                       
                                                                                                                                        
    • The common defaults (out_of_coverage, unknown) are covered.                                                                       
    • Tenants with custom intents are already editing config.Intents — they can add to this list.                                       
    • This is a future enhancement, not a bug.                                                                                          
                                                                                                                                        
    logic change.                                                                                                                       
                                                                                                                                        
    🟡 LOW: Negation patterns are still regex-based and will miss creative phrasing                                                     
                                                                                                                                        
    Users can say "scratch that number" or "nah forget my email" and the regex won't catch it. This is true of the entire existing      
    extraction pipeline. Not new to this plan.                                                                                          
                                                                                                                                        
    Mitigation for MVP: None needed. LLM-based intent extraction means if the user is clearly retracting, the classifier will likely    
    handle it at the response level even if the state isn't cleaned.                                                                    
                                                                                                                                        
    🟢 NEGLIGIBLE: IsComplete = false when email+phone provided but no name                                                             
                                                                                                                                        
    Already discussed — the prompt handles this gracefully by showing Section 0 ("you have email and phone, don't ask again") while     
    Rule 1 says "ask only for name." LLM will synthesize a coherent response.                                                           
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    One-line implementation reminder                                                                                                    
                                                                                                                                        
    When writing buildContactInstruction, prefer a table-driven approach:                                                               
                                                                                                                                        
      ─ go                                                                                                                              
      func buildContactInstruction(state ContactState, classification *Classification) ContactInstruction {                             
          fallback := isFallbackIntent(classification.PrimaryIntent)                                                                    
          return ContactInstruction{                                                                                                    
              Section0Addition: buildSection0(state),                                                                                   
              Rule1Text:        buildRule1(state, fallback),                                                                            
              Rule8Text:        buildRule8(state),                                                                                      
              RAGFallbackText:  buildRAGFallback(state, fallback),                                                                      
          }                                                                                                                             
      }                                                                                                                                 
                                                                                                                                        
    This makes each builder independently testable and keeps the function small. With 4 states × 2 intent types = 8 test cases, all     
    covered by table-driven unit tests.                                                                                                 
                                                                                                                                        
    ────────────────────────────────────                                                                                                
                                                                                                                                        
    Final call                                                                                                                          
                                                                                                                                        
    Implement this plan as-is. No remaining blockers. The two "low" items can be follow-up tickets if they ever become relevant. This   
    is as minimal and complete as a production-ready fix gets.   
