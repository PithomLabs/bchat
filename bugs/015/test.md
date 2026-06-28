## prompt

how to test the lead capture, give scenarios from external customer POV

answer:

• Test through the external widget/API as an anonymous customer:

  POST /api/v1/agent/:slug/chat/ext

  Use the same session_id across turns when testing multi-message capture.

  Scenario 1: Name + Email Lead

  {
    "session_id": "lead-test-001",
    "message": "Hi, my name is Ada Lovelace and my email is ada@example.org. I need help with pricing."
  }

  Expected:

  - Lead is created.
  - Name: Ada Lovelace
  - Email: ada@example.org
  - No escalation ticket unless the message includes an escalation phrase.

  Scenario 2: Name + Phone Lead

  {
    "session_id": "lead-test-002",
    "message": "This is Grace Hopper. Please call me at 415-555-1212 about service availability."
  }

  Expected:

  - Lead is created.
  - Name: Grace Hopper
  - Phone: 415-555-1212

  Scenario 3: Multi-Turn Lead Capture
  Turn 1:

  {
    "session_id": "lead-test-003",
    "message": "I need help with an appointment."
  }

  Turn 2:

  {
    "session_id": "lead-test-003",
    "message": "My name is Alan Turing."
  }

  Turn 3:

  {
    "session_id": "lead-test-003",
    "message": "My email is alan@example.org."
  }

  Expected:

  - No lead after turn 1.
  - No lead after name-only turn 2.
  - Lead created after turn 3.

  Scenario 4: Incomplete Contact

  {
    "session_id": "lead-test-004",
    "message": "My name is Katherine Johnson. I need help."
  }

  Expected:

  - No lead persisted yet.
  - Lead requires name plus phone or email.

  Scenario 5: Escalation With Complete Lead

  {
    "session_id": "lead-test-005",
    "message": "I need to speak to a manager. My name is Ada Lovelace and my email is ada@example.org."
  }

  Expected:

  - Lead is created.
  - Escalation ticket is created.
  - Response includes I've created ticket TKT-...
  - No live handoff starts automatically.

  Scenario 6: Escalation Without Contact Info

  {
    "session_id": "lead-test-006",
    "message": "I want to speak to your supervisor."
  }

  Expected:

  - Escalation ticket is created.
  - No lead is created yet.
  - Response asks for name and phone/email.

  Scenario 7: Correction
  Turn 1:

  {
    "session_id": "lead-test-007",
    "message": "My name is John Smith and my email is john@example.org."
  }

  Turn 2:

  {
    "session_id": "lead-test-007",
    "message": "Actually my email is john.smith@example.org."
  }

  Expected:

  - Lead remains one row for the session.
  - Email updates to corrected value if extraction confidence accepts it.

  Verify In Admin/API
  List leads:

  curl -s http://localhost:8081/api/v1/agent/:slug/leads \
    -H "Authorization: Bearer <token>"

  Check tickets for escalation scenarios in the Tickets UI, or inspect agent_escalation tickets.
