Implemented the consolidated external escalation flow.

  Changed server/router/api/v1/agent/service.go so external “human/supervisor/manager” escalation now:

  - Uses the existing lead extraction pipeline before ticket creation.
  - Creates/updates an agent_leads row when contact info is complete.
  - Creates or reuses one durable agent_escalation ticket per tenant/session.
  - Includes tenant ID, session ID, lead ID, detected intent, and contact info in the protected escalation memo.
  - Does not create a bridge handoff or suppress AI unless a bridge handoff already exists.
  - Responds differently when contact info is missing, asking for name plus phone/email.
  - Avoids FK failures by linking lead transcript_id only after a transcript exists and using a persisted creator for system escalation tickets.

  Added regression coverage in server/router/api/v1/agent/bridge_foundation_test.go for:

  - Lead + ticket creation without handoff.
  - Durable ticket dedupe across a new service instance.
  - Incomplete contact info ticket creation without lead persistence.

  Verification passed:

  - go test ./server/router/api/v1/agent -count=1
  - go test ./store/test -run 'AgentLead' -count=1

  Note: the worktree also has unrelated existing changes in build/memos and bugs/015/; I left them untouched.
