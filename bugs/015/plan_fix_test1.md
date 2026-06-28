 # Fix Re-Asking For Already Provided Lead Info

  Summary

  - The bug is prompt-instruction conflict: the prompt detects customer name/email, but later generic fallback rules still tell the LLM to “offer to collect” contact info.
  - Fix by making follow-up collection conditional on missing contact fields and by injecting an explicit “lead complete” instruction into both long-context and RAG prompts.

  Implementation Changes

  - Add a small helper that returns customer contact state from the current session:
      - hasName
      - hasEmailOrPhone
      - hasCompleteFollowupContact
      - display strings for name/email/phone/address.

  - Use this helper in both prompt builders:
      - buildSystemPrompt
      - buildRAGSystemPrompt

  - Replace unconditional fallback instructions:
      - From: “offer to collect their name plus email or phone for follow-up”
      - To: “if name plus email/phone is missing, ask only for the missing fields; if already provided, acknowledge you have it and say the team can follow up.”

  - Add a stronger rule near the top of the prompt:
      - If customer has already provided name and email/phone, never ask whether they want to provide contact info.
      - Do not phrase follow-up as “I can collect your contact information.”
      - Instead say “I have your contact information and can pass this along for follow-up.”

  - Keep this generic and tenant-neutral; no Pithom-specific logic.

  Test Cases

  - Add prompt/unit tests for:
      - Name + email present, unknown topic: prompt must not contain unconditional “offer to collect their name plus email or phone.”
      - Name only present: prompt should allow asking for email or phone, but not ask for name again.
      - Email only present: prompt should allow asking for name, but not ask for email again.
      - No contact info: prompt may ask for name plus email or phone.

  - Add an external chat regression test using:
      - Hi, my name is Ada Lovelace and my email is ada@example.org. I need help with pricing.
      - Assert response does not contain collect your contact information, name and email, or similar re-ask language.
      - Assert lead is still captured.

  Acceptance Criteria

  - Customer-provided name/email/phone are recognized before first response.
  - The agent never asks for already provided fields in unknown-topic follow-up.
  - Lead capture remains unchanged: persist only when name plus email/phone exists.
  - No live handoff behavior changes.
    </proposed_plan>


## 2

 # Robust Contact-Aware Follow-Up Handling

  Summary

  - Fix the root problem by separating “answer/fallback decision” from “contact collection wording.”
  - Make contact collection state explicit, reusable, and deterministic before the LLM prompt is built.
  - The agent should never ask for fields already provided, regardless of topic, tenant, retrieval mode, or whether the request is pricing, unsupported, escalation, scheduling, or generic
    follow-up.

  Implementation Changes

  - Add a reusable contact-state helper used before response generation:
      - Extracts from the current session using the strongest available local extraction path.
      - Normalizes into name, email, phone, location.
      - Computes missing fields for follow-up: missing name, missing email_or_phone, missing optional location.
      - Distinguishes “no contact,” “partial contact,” and “follow-up ready.”

  - Use that contact state in both prompt builders:
      - buildSystemPrompt
      - buildRAGSystemPrompt

  - Replace all unconditional “offer to collect name/email/phone” instructions with state-aware language:
      - No contact: ask for name plus either email or phone only when follow-up is needed.
      - Name only: ask only for email or phone.
      - Email/phone only: ask only for name.
      - Complete contact: acknowledge it is already available; do not ask to collect it again.

  - Add a high-priority prompt rule:
      - Treat already-provided customer contact info as authoritative customer data.
      - Never ask for a field present in contact state.
      - Never say “I can collect your contact information” when contact state is follow-up ready.
      - If follow-up is appropriate and contact state is ready, say the team can follow up using the information already provided.

  - Make fallback behavior generic:
      - For unknown/out-of-scope/unsupported information, answer what is known, decline what is unknown, then use contact-state-aware follow-up wording.
      - Do not hardcode pricing, Pithom, or any tenant-specific phrasing.

  - Keep lead persistence rules unchanged:
      - Persist lead only when name plus email/phone exists.
      - Do not require persistence before prompt behavior works; prompt behavior should rely on in-session contact state.

  Functional Guardrails

  - The prompt must not contain contradictory instructions where one section says “do not ask again” and a later section says “offer to collect contact info” unconditionally.
  - The same contact-state wording rules must apply in long-context and RAG modes.
  - Contact correction should remain possible: if the customer says “actually…” or provides a corrected email/phone, update state and use the corrected value going forward.
  - Contact decline should be respected: if the customer declines to provide contact info, do not keep asking; offer non-contact alternatives when possible.
  - Escalation behavior remains separate:
      - External human requests still create/reuse escalation ticket.
      - Bridge handoff still starts only through the bridge takeover path.
      - Escalation responses use the same contact-state helper to avoid asking for already provided info.

  Test Plan

  - Add deterministic unit tests for contact-state rendering:
      - no contact
      - name only
      - email only
      - phone only
      - name + email
      - name + phone
      - corrected email/phone
      - declined contact

  - Add prompt tests for both long-context and RAG prompt builders:
      - no unconditional “collect name plus email or phone” when contact is complete.
      - partial contact prompts only for missing fields.
      - complete contact prompts tell the model to use already-provided info.

  - Add external chat regression tests:
      - Customer provides name + email and asks about unsupported info; response must not ask for name/email again.
      - Customer provides name only and asks for unsupported info; response may ask for email/phone, not name.
      - Customer declines contact; repeated turns should not ask again.

  - Keep existing lead, escalation, and bridge tests passing.

  Acceptance Criteria

  - Prompt construction is contact-state-aware and mode-independent.
  - The agent never re-asks for already provided contact fields.
  - The agent can still ask only for genuinely missing required follow-up fields.
  - Lead capture and escalation behavior remain generic, tenant-neutral, and configuration-driven.
    </proposed_plan>



