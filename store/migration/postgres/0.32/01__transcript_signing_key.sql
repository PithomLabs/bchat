ALTER TABLE agent_tenants ADD COLUMN transcript_signing_key BYTEA;
ALTER TABLE agent_tenants ADD COLUMN transcript_signing_key_nonce BYTEA;
