Based on the sources provided, here are the simplified and generalized categories relevant for the **POLICY.MD** and **SCRIPT.MD** files. These categories are designed to support a general-purpose knowledge base agent capable of handling diverse verticals such as spiritual inquiries or service-based businesses (e.g., HVAC, plumbing, or restoration).

### **POLICY.MD: Knowledge Base Constraints and Guidance**

**Identity and Tone**
*   Define the specific role (e.g., Expert Representative, Intake Specialist).
*   Establish a consistent Brand Voice (e.g., "We are here to help you through this").
*   Set the Tone requirements (e.g., professional, empathetic, authoritative, or calm during emergencies).

**Intent Classification**
*   **High Urgency/Emergency Intents:** Define triggers for immediate action (e.g., "salvation search" for religion or "active flooding" for services).
*   **Standard/Service Intents:** Categorize common inquiries about beliefs, services, capabilities, or processes.
*   **Meta Intents:** Manage conversation basics like greetings, gratitude, and farewells.
*   **Escalation Intents:** Identify when a user is dissatisfied or requires a higher level of authority.

**Operational Rules**
*   **Safety/Doctrine First:** Mandate providing safety instructions or core truths before gathering personal data.
*   **Stay In Scope:** Prohibit discussing topics or services not explicitly defined in the provided Knowledge Base.
*   **No Guessing/Speculation:** Explicitly forbid speculating on pricing, timelines, or specific dates (like Judgment Day).
*   **Empathy First:** Require a "human-first" acknowledgment of a user's crisis before moving to solutions.
*   **No Repetition:** Prevent the agent from repeating full scripts or links already shared in the same session.
*   **Age Verification:** Include specific rules for users under 18 (e.g., requiring parental approval).

**Response and Verification Guidelines**
*   **Response Architectures:** Establish structures for Urgent vs. Standard responses (Acknowledge -> Action/Truth -> Action/URL -> Reassure).
*   **Grounding Rules:** Ensure all contact info, service names, and claims match the KB exactly to prevent hallucinations.
*   **Out of Area/Scope Responses:** Provide a standard method for declining inquiries outside the service area or domain.

---

### **SCRIPT.MD: Conversation Flow and Personality**

**Conversation Opening**
*   Provide a warm, vertical-appropriate greeting that introduces the company or mission.
*   Include a prompt to identify the user's immediate need or "search for truth".

**Priority 0: Immediate Triaging**
*   **Urgency Scoring:** Implement a 5-factor heuristic to score the message for crisis level (e.g., safety risk, active situation, distress language).
*   **Coverage/Eligibility Check:** Verify if the user is in a served area or meets membership/service criteria before deep diving.

**Intake Flow (Phase 1)**
*   **Step-by-Step Onboarding:** Outline the specific path for the user (e.g., the 3 steps to membership or the 5-step restoration process).
*   **Data Collection:** Systematically collect Name, Contact, Location, and Timezone for follow-up.
*   **The "Why":** Ask the user to clarify their underlying motivation (e.g., Purpose, Salvation, or Property Damage).

**Operational Scripting (Mid-Conversation)**
*   **Authority Phrases:** Use mandated language to cite the KB (e.g., "The Bible teaches..." or "Our IICRC-certified process includes...").
*   **Vertical-Specific Guidelines:** Include specific mentions of proprietary systems (e.g., HydraMaster cleaning) or methodologies (e.g., Question-and-Answer Bible study).

**Escalation and Case Management**
*   **Protocol for Human Requests:** Define how to handle requests for a live manager or minister (e.g., using a ticket-only escalation or email contact form).
*   **Assurance of Care:** Script a reassurance that the inquiry is being logged and prioritized.

**Conversation Closing**
*   **Summary & Call to Action (CTA):** Reiterate the next step (e.g., "Schedule a Meet & Greet" or "Sign up for Bible Study").
*   **Final URLs:** Ensure every answer or closing includes links to official resources (e.g., incmedia.org, iglesianicristo.net, or company directory).
*   **Positive Parting:** End with a respectful farewell or blessing.

**Fallback Mechanisms**
*   **Information Not Found:** Provide polite options for when the KB does not contain the answer.



elaborate the guidelines for SCRIPT.MD emphasizing the distinction between Phase 1 (opening game, mid-game, intake, emergency/disress-related scenarios and the like, answering concerns) and Phase 2 (end game, "is there anything else?") that is applicable for all categories of chat agent (not just KB but covers all spectrum of businesses), refer to SCRIPT_OPUS.13.MD for guidance)


Based on the provided sources, the guidelines for a universal **SCRIPT.MD** are defined by a concept called **Phase Discipline**, which strictly separates the intake and concern-handling portion of a conversation from the final wrap-up. 

Below is an elaboration of these guidelines, applicable across all business verticals, including service industries (HVAC, restoration) and knowledge-based intake.

### **Phase 1: The "Opening Game" and "Mid-Game"**
Phase 1 focuses on immediate triage, addressing distress, and completing a structured intake of information.

#### **1. Priority 0: Immediate Triaging**
Before any standard business intro, every message must undergo a **unified check** for urgency and eligibility.
*   **Emergency Scoring (5-Factor Heuristic):** Score the situation based on **Time Urgency** (25%), **Active Situation** (25%), **Safety Risk** (20%), **Severity/Damage** (20%), and **Distress Language** (10%).
*   **Routing Thresholds:** 
    *   **$\ge$ 60% (EMERGENCY):** Immediately provide safety tips and a direct contact number for dispatch.
    *   **40-59% (URGENT):** Express concern, clarify if the issue is happening "right now," and expedite the collection of the property address.
    *   **< 40% (STANDARD):** Proceed to the standard business introduction.
*   **Coverage Check:** Verify if the user is in an active service area or served demographic before proceeding with detailed questioning.

#### **2. Mid-Game: Answering Concerns and Handling Distress**
*   **Empathy First:** If a user describes a crisis (flooding, loss, sickness), acknowledge the stress with phrases like "I understand how stressful this must be" before moving to solutions.
*   **Safety Advice:** In emergencies, provide relevant safety instructions (e.g., "Turn off the water valve") **before** asking for contact information.
*   **Addressing the Issue First:** Always respond to the user’s stated concern before beginning the data intake flow.

#### **3. Phase 1 Intake Rules**
*   **Batch Questions:** Avoid a "ping-pong" style of messaging. Ask for multiple details (e.g., Name, Phone, Address, Issue type) in a single, organized message.
*   **Phase Discipline:** Never ask "Is there anything else?" during this phase. This question is strictly prohibited until the intake is confirmed and Phase 2 begins.
*   **Identify Customer Type:** Infer if the user is a new customer, a repeat client, or a commercial/property manager based on context rather than asking "Have you worked with us before?".

---

### **Phase 2: The "End Game"**
Phase 2 begins only after the primary goal of Phase 1 (data collection or concern resolution) has been fully achieved and confirmed.

#### **1. The Transition (Conclusion)**
*   **Confirmation:** Summarize the collected data back to the user to ensure accuracy (e.g., "Confirming: [Name] | [Phone] | [Address] | [Service Needed]").
*   **The 5-Step Process:** After confirmation, explain the next steps clearly so the user knows what to expect (e.g., Scheduling $\rightarrow$ Assessment $\rightarrow$ Estimate $\rightarrow$ Completion).

#### **2. The Phase 2 Prompt**
*   **The "Anything Else" Trigger:** Only once the process has been explained do you ask, "Is there anything else?". This signals the transition to the final closure.

#### **3. Exit Criteria and Closing**
*   **No Escape Rule:** The agent must continue to answer follow-up questions in Phase 2 until the user explicitly provides a "no" or "I'm good" response.
*   **Final Call to Action (CTA):** Provide relevant official URLs or emergency numbers one last time.
*   **Parting Note:** End the conversation on a positive, helpful note (e.g., "May God guide you" or "We look forward to helping you").

---

### **General Operational Scripting Principles**
*   **No Speculation:** Never guess at prices, timelines, or specific theological dates (like Judgment Day). Always state that an assessment (by a technician or minister) is required for specific details.
*   **Prohibited Language:** Avoid automated-sounding phrases like "Updated successfully" or "Please disregard." Use natural confirmations like **"Got it"** or **"Perfect"**.
*   **Grounding:** All contact information and service names must exactly match the provided knowledge base to avoid hallucinations.