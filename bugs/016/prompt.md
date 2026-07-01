## prompt

read codebase at /home/chaschel/Documents/go/bchat when I run "hugo server" from the terminal at                                  
   /home/chaschel/Documents/go/extract.github.io and from the chat widget I ask "what did maria clara give to the leper" the chat    
   reply was "I don't have information about this specific detail in the retrieved context. The documents I can access don't         
   contain details about a character named Maria Clara giving something to a leper." but when I run "task run:rag" at                
   /home/chaschel/Documents/go/bchat and access http://localhost:8081/rag-stats under RAG Stats > Search Testing I put the same      
   "what did maria clara give to the leper" and clicked Search (note the tenant is rizal and the audience is set to external), it    
   gave 5 search results but none when audience is set to internal. why? also when I test the chat widget locally via                
   http://localhost:1313/ is the audience set to external or internal? give comprehensive report about RAG search with respect to    
   the chat widget 




## prompt

I already set to rizal at head-end-prod.html when I ask "look deeper into the story of the leper"  it said: "I've searched through all the retrieved context provided, and there is no mention of a leper or any story involving a leper in the material available to me. The context contains information about various characters and events, but this specific detail is not present.

Since this topic is not covered in the retrieved business context, I'm unable to provide information about it. Would you like me to collect your name and contact information (email or phone) so I can follow up with you when this information becomes available?"

I want to decouple the logic asking for name and contact informaiton in the system prompt (so that the logic will be generic for knowledge bases), write plan on how to do that