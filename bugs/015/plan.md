## prompt 

I want to consolidate the process when an external customer intends to escalate to human and ability of bchat to capture leads. analyze the codebase about lead capture and how do you plan to implement that requirement 


answer:

 # Consolidate External Human Escalation With Lead Capture

  Summary

  - Keep the selected default: when an external customer asks for a human, bchat creates/updates a lead when enough contact info exists, creates one escalation ticket, acknowledges the
    request, and does not auto-start live bridge handoff.

  - Use the existing lead extraction pipeline and agent_leads table as the lead source of truth.
  - Keep live handoff separate: staff or an HMAC bridge client can still call /bridge/takeover, but customer escalation intent alone will not suppress AI via bridge.

  Current State

  - External chat already calls captureLeadFromSession after normal AI responses and during existing bridge handoff messages.
  - Lead persistence requires name + (email or phone) because agent_leads.name is required and the DB has CHECK (email IS NOT NULL OR phone IS NOT NULL).
  - Escalation ticket creation currently uses only session.CustomerName and session.CustomerPhone; it ignores the richer LeadDraft data, including email and better extraction confidence.
  - Escalation ticket dedupe is in-memory only through session.EscalationTicket, so a process restart can allow another escalation ticket for the same external session.
  - ScoreUserMessage(...).ShouldEscalate is computed but not used; ticket creation depends only on classification.PrimaryIntent == "escalation".

  Implementation Changes

  - Add a single external escalation helper in Service, used from processChat, that does all escalation work in this order:
      1. Run/refresh lead extraction with ExtractContactInfoFull.
      2. Update session customer fields from the lead draft: name, phone, location, and add email handling where transcript/lead paths already support it.
      3. Call captureLeadFromSession; it should persist a lead only when the current DB rule is satisfied.
      4. Create or reuse a durable escalation ticket for the session.
      5. Store the resulting ticket number on the session for response injection.

  - Make escalation detection more robust but still tenant-configurable:
      - Primary trigger remains classification.PrimaryIntent == "escalation".
      - Also allow ScoreUserMessage(userMessage, config).ShouldEscalate as a fallback only when confidence/category indicates an explicit supervisor/human/complaint request.
      - Do not add tenant-specific keywords or tenant-specific behavior.

  - Make ticket creation durable and lead-aware:
      - Include session_id, customer name, phone, email, location, detected intent, and current lead ID if available in the protected escalation memo.
      - Keep ticket Type: "agent_escalation" and default priority MEDIUM.
      - Use HIGH only when the selected ticket type is explicitly "complaint" or the scoring/classification clearly identifies complaint/legal/supervisor severity.
      - Add a durable lookup before creating a new escalation ticket by searching existing agent_escalation tickets/memos for the same tenant + session ID, so restart/retry does not
        create duplicates.

      - Preserve TKT-YYYYMMDD-XXXX; do not use CMP-... unless the flow explicitly classifies ticket type as "complaint".

  - Adjust the customer response behavior:
      - If ticket was created and contact info is complete, prepend the existing ticket acknowledgement with the ticket number.
      - If contact info is incomplete, still create the ticket, but make the response ask for the missing name and either phone or email so the lead can be completed.
      - Do not tell the customer a human is actively handling the chat unless a bridge handoff actually exists.

  - Keep memo/ticket #staff, #human, #escalated behavior separate:
      - Existing memo-created tickets tagged escalated should continue to skip ticket AI auto-replies.
      - Do not merge that path with external widget escalation except by reusing shared lead/ticket helper logic where practical.

  Test Plan

  - Add service tests for external escalation:
      - Creates exactly one escalation ticket for a session.
      - Repeated escalation messages reuse the same ticket.
      - Lead is created when customer provided name plus phone/email.
      - No lead is persisted when contact info is incomplete, but ticket is still created and response asks for missing contact.
      - Email and location extracted by the lead pipeline appear in the escalation memo.
      - Existing bridge handoff still suppresses AI and does not create a new escalation ticket.

  - Add regression tests for restart/durable dedupe if feasible by rebuilding session from transcript and re-sending escalation.
  - Run focused Go tests for server/router/api/v1/agent and store lead tests.

  Assumptions

  - Lead persistence keeps the current minimum rule: name + phone/email; no migration to partial leads.
  - External human escalation does not automatically queue bridge handoff.
  - No tenant-specific escalation logic will be added; tenant files and generic classifiers drive behavior.



