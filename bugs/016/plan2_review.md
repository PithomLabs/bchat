 # Decouple Contact Collection From Fallback Prompts

  ## Summary

  Add a tenant/audience setting that controls whether fallback responses collect customer contact details. Existing
  tenants keep current behavior by default; tenants can opt out via POLICY.MD.

  ## Key Changes

  - Add RequireContactOnFallback to store.AgentAudience, persisted with DB default enabled.
  - Update audience create/list/update persistence so the flag round-trips through SQLite and any active supported
    DB drivers.

  - Add a migration under the repo’s versioned migration layout and update LATEST.sql if required by project
    convention.

  - Parse <!-- @settings: require_contact_on_fallback: false --> in ParsePolicy.
  - Ensure ParsePolicy initializes result.Audience before writing settings or thresholds, so settings-only policies
    do not panic or get ignored.

  - Rework importFiles() to parse policy settings before creating the default audience, or immediately update the
    created audience from parsed policy values.

  - Add shouldCollectContact(config *AudienceConfig) bool, defaulting to true unless the audience explicitly
    disables fallback contact collection.

  - Gate fallback contact behavior in:
      - buildRule1
      - buildRule8
      - buildRAGFallback
      - buildRAGSection0
      - post-LLM CorrectContactsInResponse / CorrectEmailsInResponse

  - Keep company contact constraints intact; only disable customer lead/contact collection behavior.

  ## Test Plan

  - Add parser tests for settings-only, thresholds-only, and settings-plus-thresholds policies.
  - Add prompt tests for contact collection enabled and disabled across long-context and RAG fallback paths.
  - Add import/store coverage proving require_contact_on_fallback: false persists and reloads through LoadConfig.
  - Run:
      - GOCACHE=/tmp/go-build-cache go test ./server/router/api/v1/agent -count=1
      - GOCACHE=/tmp/go-build-cache go test ./store/test -count=1

  ## Assumptions

  - Default behavior remains contact collection enabled for existing tenants.
  - The policy annotation syntax is @settings: require_contact_on_fallback: false.
  - Admin UI exposure is optional follow-up work and does not block backend execution.
