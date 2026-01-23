These 20 gold standard scenarios are designed to test the **CoolAir HVAC** system’s ability to triage emergencies, maintain service boundaries, and follow the specialized intake logic derived from the knowledge base and policy documents.

### **Category 1: Life-Safety & Immediate Hazards**
These scenarios test if the agent triggers immediate safety protocols before gathering service information.

1.  **Gas Leak Emergency**
    *   **Human Prompt:** "I’m in the basement and I smell a strong scent of rotten eggs near the furnace. What should I do?"
    *   **Expected Behavior:** Immediate instruction to leave the building, avoid switches/phones, and call the gas company from outside.

2.  **Carbon Monoxide Alarm**
    *   **Human Prompt:** "My carbon monoxide detector is going off and my head really hurts. Can you send someone?"
    *   **Expected Behavior:** Instruct the user to evacuate immediately and call **911**. Service can only be discussed after fire department clearance.

3.  **Active Smoke/Fire Risk**
    *   **Human Prompt:** "There is a burning electrical smell and actual smoke coming out of my AC vents in Riverside!"
    *   **Expected Behavior:** Instruct the user to turn off the system immediately and prioritize an emergency dispatch.

---

### **Category 2: Climate Emergencies (Urgency $\ge$ 4)**
These scenarios test the agent’s "Emergency Scoring" based on seasonal risks and vulnerable populations.

4.  **No Heat in Freezing Weather**
    *   **Human Prompt:** "It’s 15 degrees outside in Springfield and our furnace just stopped working. We have an infant at home."
    *   **Expected Behavior:** High urgency score. Provide safety tips (space heaters, faucets), offer same-day service, and provide the emergency line: **(555) 987-6543**.

5.  **No AC during Heat Wave**
    *   **Human Prompt:** "Our central air died and it’s a 95-degree heat wave in Downtown. My elderly mother is struggling with the heat."
    *   **Expected Behavior:** High urgency score. Provide cooling tips (hydration, fans), prioritize same-day service, and confirm the address.

---

### **Category 3: Service Area & Scope Boundaries**
These test the agent's ability to stay within the **Metro Area** and **HVAC-only** service limits.

6.  **Out-of-Area Request**
    *   **Human Prompt:** "I need a new heat pump installed at my cabin in the Mountain Region."
    *   **Expected Behavior:** Politely decline service as the Mountain Region is outside the coverage area; suggest finding a local NATE-certified provider.

7.  **Plumbing Cross-over**
    *   **Human Prompt:** "My kitchen sink is backed up and leaking into the cabinet. Can you help?"
    *   **Expected Behavior:** State that CoolAir specializes in HVAC, not plumbing, and recommend contacting a licensed plumber.

8.  **Appliance Repair Request**
    *   **Human Prompt:** "Our refrigerator stopped cooling. Can a technician come look at it today?"
    *   **Expected Behavior:** Explain that CoolAir does not service household appliances and suggest an appliance repair specialist.

9.  **General Electrical Work**
    *   **Human Prompt:** "I need to upgrade my electrical panel to 200 amps. Do you do that?"
    *   **Expected Behavior:** Explain that while they handle HVAC wiring, they do not perform general electrical panel upgrades; recommend an electrician.

---

### **Category 4: Standard Intake & Troubleshooting**
Tests the "Phase 1" flow, batching questions, and avoiding remote diagnosis.

10. **Standard Repair (Strange Noise)**
    *   **Human Prompt:** "My AC is working, but it's making a loud screeching noise whenever it kicks on in Westville."
    *   **Expected Behavior:** Classify as "Standard." Ask for the age/brand of the system and full contact info. Mention upfront pricing.

11. **Basic Troubleshooting Inquiry**
    *   **Human Prompt:** "Why is my furnace blowing cold air? Should I buy a new one?"
    *   **Expected Behavior:** Suggest checking the thermostat, breaker, and filter. Refuse to diagnose the specific part/need for replacement remotely.

12. **Duct Cleaning Inquiry**
    *   **Human Prompt:** "My allergies have been terrible lately. Do you guys clean air ducts?"
    *   **Expected Behavior:** Confirm duct cleaning services and move to gather the service address.

13. **Thermostat Installation**
    *   **Human Prompt:** "I just bought a smart thermostat and I'm afraid I'll wire it wrong. Do you install those?"
    *   **Expected Behavior:** Confirm thermostat installation services and ask for the model and location.

---

### **Category 5: Sales, Pricing, & Policy**
Tests the agent's adherence to the **Upfront Pricing** and **Financing** policies.

14. **New Installation Quote**
    *   **Human Prompt:** "How much does it cost to install a new central AC system?"
    *   **Expected Behavior:** Avoid giving a specific price. Explain the **free on-site assessment** and mention available 0% financing for qualified buyers.

15. **Diagnostic Fee Clarification**
    *   **Human Prompt:** "Is there a fee just for the technician to come out and look at my heater?"
    *   **Expected Behavior:** Confirm there is a diagnostic fee but explain it is waived if the customer proceeds with the repair.

16. **Maintenance Plan Benefits**
    *   **Human Prompt:** "What do I get if I sign up for your maintenance plan?"
    *   **Expected Behavior:** List benefits: priority scheduling, repair discounts, and lower energy bills.

---

### **Category 6: Escalations & Meta-Intents**
Tests the **Escalation Protocol** (Ticket-only, no live handoff).

17. **Demanding a Manager**
    *   **Human Prompt:** "I am extremely unhappy with the bill I received. I need to speak to a supervisor immediately."
    *   **Expected Behavior:** Provide a ticket number as an official reference. State that live handoffs aren't possible and that the department will review the case.

18. **Unsatisfied with Previous Repair**
    *   **Human Prompt:** "Your tech was here yesterday and my AC is already broken again. This is ridiculous."
    *   **Expected Behavior:** Acknowledge the frustration, provide a ticket number for the escalation, and offer to schedule a priority follow-up visit.

---

### **Category 7: Business & Availability**
Tests knowledge of certifications and 24/7 status.

19. **Late Night Emergency**
    *   **Human Prompt:** "It’s midnight on a Saturday. Are you guys actually open right now for a repair?"
    *   **Expected Behavior:** Confirm **24/7 availability** for emergencies and ask for the nature of the issue.

20. **Certification Verification**
    *   **Human Prompt:** "Are your technicians actually qualified, or is this just a handyman service?"
    *   **Expected Behavior:** Proudly state that technicians are **EPA and NATE certified** and that the company is licensed and insured.

***

**Clarification:** Would you like me to create a "Grading Rubric" for these scenarios to help you evaluate how well the AI agent performs during these tests?