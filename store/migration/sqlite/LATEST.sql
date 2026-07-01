-- migration_history
CREATE TABLE IF NOT EXISTS migration_history (
  version TEXT NOT NULL PRIMARY KEY,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);

-- system_setting
CREATE TABLE system_setting (
  name TEXT NOT NULL,
  value TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  UNIQUE(name)
);

-- user
CREATE TABLE user (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  username TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL CHECK (role IN ('HOST', 'ADMIN', 'USER')) DEFAULT 'USER',
  email TEXT NOT NULL DEFAULT '',
  nickname TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_user_username ON user (username);

-- user_setting
CREATE TABLE user_setting (
  user_id INTEGER NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  UNIQUE(user_id, key)
);

-- memo
CREATE TABLE memo (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  content TEXT NOT NULL DEFAULT '',
  visibility TEXT NOT NULL CHECK (visibility IN ('PUBLIC', 'PROTECTED', 'PRIVATE')) DEFAULT 'PRIVATE',
  pinned INTEGER NOT NULL CHECK (pinned IN (0, 1)) DEFAULT 0,
  payload TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_memo_creator_id ON memo (creator_id);

-- memo_organizer
CREATE TABLE memo_organizer (
  memo_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  pinned INTEGER NOT NULL CHECK (pinned IN (0, 1)) DEFAULT 0,
  UNIQUE(memo_id, user_id)
);

-- memo_relation
CREATE TABLE memo_relation (
  memo_id INTEGER NOT NULL,
  related_memo_id INTEGER NOT NULL,
  type TEXT NOT NULL,
  UNIQUE(memo_id, related_memo_id, type)
);

-- resource
CREATE TABLE resource (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  filename TEXT NOT NULL DEFAULT '',
  blob BLOB DEFAULT NULL,
  type TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL DEFAULT 0,
  memo_id INTEGER,
  storage_type TEXT NOT NULL DEFAULT '',
  reference TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_resource_creator_id ON resource (creator_id);

CREATE INDEX idx_resource_memo_id ON resource (memo_id);

-- activity
CREATE TABLE activity (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  type TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL CHECK (level IN ('INFO', 'WARN', 'ERROR')) DEFAULT 'INFO',
  payload TEXT NOT NULL DEFAULT '{}'
);

-- idp
CREATE TABLE idp (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  identifier_filter TEXT NOT NULL DEFAULT '',
  config TEXT NOT NULL DEFAULT '{}'
);

-- inbox
CREATE TABLE inbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  sender_id INTEGER NOT NULL,
  receiver_id INTEGER NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '{}'
);

-- webhook
CREATE TABLE webhook (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  creator_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  url TEXT NOT NULL
);

CREATE INDEX idx_webhook_creator_id ON webhook (creator_id);

-- reaction
CREATE TABLE reaction (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  creator_id INTEGER NOT NULL,
  content_id TEXT NOT NULL,
  reaction_type TEXT NOT NULL,
  UNIQUE(creator_id, content_id, reaction_type)
);

-- tickets
CREATE TABLE tickets (
   id INTEGER PRIMARY KEY AUTOINCREMENT,
   title TEXT NOT NULL,
   description TEXT NOT NULL DEFAULT '',
   status TEXT NOT NULL DEFAULT 'OPEN',
   priority TEXT NOT NULL DEFAULT 'MEDIUM',
   creator_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
   assignee_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
   created_ts BIGINT NOT NULL,
   updated_ts BIGINT NOT NULL,
   type TEXT NOT NULL DEFAULT 'TASK',
   tags TEXT NOT NULL DEFAULT '[]',
   beads_id TEXT UNIQUE,
   parent_id INTEGER REFERENCES tickets(id) ON DELETE CASCADE,
   labels TEXT DEFAULT '[]',
   dependencies TEXT DEFAULT '[]',
   discovery_context TEXT,
   closed_reason TEXT,
   issue_type TEXT
 );

CREATE INDEX idx_tickets_creator_id ON tickets (creator_id);
CREATE INDEX idx_tickets_status ON tickets (status);
CREATE INDEX idx_tickets_assignee_id ON tickets (assignee_id);
CREATE UNIQUE INDEX idx_tickets_beads_id ON tickets(beads_id) WHERE beads_id IS NOT NULL;
CREATE INDEX idx_tickets_parent_id ON tickets(parent_id);
CREATE INDEX idx_tickets_issue_type ON tickets(issue_type);
CREATE UNIQUE INDEX idx_tickets_creator_description_memo ON tickets(creator_id, description) WHERE description LIKE '/m/%';

-- notifications
CREATE TABLE notifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  initiator_id INTEGER NOT NULL,
  receiver_id INTEGER NOT NULL,
  ticket_url TEXT NOT NULL,
  created_ts BIGINT NOT NULL,
  is_read BOOLEAN NOT NULL DEFAULT 0
);

CREATE INDEX idx_notifications_receiver ON notifications(receiver_id);
CREATE INDEX idx_notifications_is_read ON notifications(is_read);

-- ============================================================================
-- AGENT CHAT SYSTEM TABLES
-- ============================================================================

-- agent_tenants
CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    company_name TEXT NOT NULL,
    guid TEXT,
    vertical TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    processing_options TEXT,
    allowed_domains TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_tenants_guid ON agent_tenants(guid);

-- agent_audiences
CREATE TABLE agent_audiences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    role TEXT NOT NULL,
    tone TEXT NOT NULL,
    brand_voice TEXT,
    guidelines TEXT,
    emergency_phone TEXT NOT NULL,
    secondary_phones TEXT,
    email TEXT,
    address TEXT,
    emergency_urgency_threshold INTEGER DEFAULT 4,
    escalation_confidence_threshold REAL DEFAULT 0.85,
    rate_limit_rpm INTEGER DEFAULT 60,
    require_contact_on_fallback INTEGER DEFAULT 1,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, audience_type)
);

-- agent_services
CREATE TABLE agent_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    is_emergency INTEGER DEFAULT 0,
    response_time TEXT,
    is_active INTEGER DEFAULT 1,
    UNIQUE(tenant_id, audience_type, code)
);

-- agent_exclusions
CREATE TABLE agent_exclusions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    exception_rule TEXT,
    referral TEXT,
    is_active INTEGER DEFAULT 1,
    UNIQUE(tenant_id, audience_type, code)
);

-- agent_coverage
CREATE TABLE agent_coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    area_type TEXT NOT NULL,
    area_name TEXT NOT NULL,
    state_code TEXT,
    is_included INTEGER NOT NULL,
    UNIQUE(tenant_id, area_type, area_name)
);

-- agent_faqs
CREATE TABLE agent_faqs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
    UNIQUE(tenant_id, audience_type, code)
);

-- agent_safety_protocols
CREATE TABLE agent_safety_protocols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    trigger_intents TEXT NOT NULL,
    instructions TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
    UNIQUE(tenant_id, audience_type, code)
);

-- agent_kb_sections
CREATE TABLE agent_kb_sections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    section_type TEXT DEFAULT 'general',
    is_active INTEGER DEFAULT 1,
    UNIQUE(tenant_id, audience_type, code)
);

-- agent_intents
CREATE TABLE agent_intents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    description TEXT NOT NULL,
    examples TEXT,
    counter_examples TEXT,
    urgency INTEGER,
    action TEXT NOT NULL,
    confidence_threshold REAL,
    is_active INTEGER DEFAULT 1
);

-- agent_rules
CREATE TABLE agent_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    priority INTEGER DEFAULT 5,
    applies_to TEXT,
    is_active INTEGER DEFAULT 1,
    UNIQUE(tenant_id, audience_type, code)
);

-- agent_sessions
CREATE TABLE agent_sessions (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES user(id),
    audience_type TEXT NOT NULL DEFAULT 'internal',
    phase TEXT DEFAULT 'triage',
    current_intent TEXT,
    urgency_level INTEGER DEFAULT 0,
    coverage_status TEXT DEFAULT 'unknown',
    customer_name TEXT,
    customer_phone TEXT,
    customer_location TEXT,
    detected_service TEXT,
    message_count INTEGER DEFAULT 0,
    messages TEXT DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    is_completed INTEGER DEFAULT 0,
    completion_reason TEXT
);

-- agent_source_files (supports versioning - no unique constraint)
CREATE TABLE agent_source_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    file_type TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_source_files_lookup ON agent_source_files(tenant_id, audience_type, file_type, imported_at DESC);
CREATE INDEX idx_source_files_version ON agent_source_files(tenant_id, audience_type, file_type, version DESC);

-- agent_rate_limits
CREATE TABLE agent_rate_limits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    request_count INTEGER DEFAULT 0,
    window_start TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, audience_type, client_ip)
);

CREATE INDEX idx_agent_audiences_tenant ON agent_audiences(tenant_id, audience_type);
CREATE INDEX idx_agent_services_tenant_audience ON agent_services(tenant_id, audience_type);
CREATE INDEX idx_agent_intents_tenant_audience ON agent_intents(tenant_id, audience_type);
CREATE INDEX idx_agent_sessions_tenant ON agent_sessions(tenant_id);
CREATE INDEX idx_agent_sessions_user ON agent_sessions(user_id);
CREATE INDEX idx_agent_rate_limits_lookup ON agent_rate_limits(tenant_id, audience_type, client_ip);

-- ============================================================================
-- RBAC TABLES (migration 07)
-- ============================================================================

-- user_tenant_permission
CREATE TABLE user_tenant_permission (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    permissions TEXT NOT NULL DEFAULT '',
    granted_by INTEGER REFERENCES user(id) ON DELETE SET NULL,
    granted_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    UNIQUE(user_id, tenant_id)
);

CREATE INDEX idx_user_tenant_permission_user ON user_tenant_permission(user_id);
CREATE INDEX idx_user_tenant_permission_tenant ON user_tenant_permission(tenant_id);

-- tenant_config
CREATE TABLE tenant_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE REFERENCES agent_tenants(id) ON DELETE CASCADE,
    llm_model TEXT NOT NULL DEFAULT '',
    openrouter_api_key_encrypted BLOB,
    openrouter_api_key_nonce BLOB,
    features TEXT NOT NULL DEFAULT '{}',
    simulation_human_model TEXT DEFAULT '',
    retrieval_mode TEXT DEFAULT 'long_context',
    content_tokens INTEGER DEFAULT 0,
    record_transcripts INTEGER DEFAULT 1,
    reasoning_model TEXT DEFAULT '',
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_by INTEGER REFERENCES user(id) ON DELETE SET NULL
);

CREATE INDEX idx_tenant_config_tenant ON tenant_config(tenant_id);

-- system_secret
CREATE TABLE system_secret (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    encryption_salt BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT
);

-- ============================================================================
-- AGENT SIMULATION TABLES (migration 08)
-- ============================================================================

-- agent_simulations
CREATE TABLE agent_simulations (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES user(id) ON DELETE SET NULL,
    audience_type TEXT NOT NULL DEFAULT 'external',
    status TEXT NOT NULL DEFAULT 'pending',
    scenario TEXT,
    messages TEXT DEFAULT '[]',
    message_count INTEGER DEFAULT 0,
    max_turns INTEGER DEFAULT 20,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT
);

CREATE INDEX idx_agent_simulations_tenant ON agent_simulations(tenant_id);
CREATE INDEX idx_agent_simulations_user ON agent_simulations(user_id);
CREATE INDEX idx_agent_simulations_status ON agent_simulations(status);

-- ============================================================================
-- AGENT SCRIPT TABLES (migration 09)
-- ============================================================================

-- agent_tenant_scripts
CREATE TABLE agent_tenant_scripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    audience_type TEXT NOT NULL DEFAULT 'external',
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    summary TEXT,
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_tenant_scripts_lookup ON agent_tenant_scripts(tenant_id, audience_type, imported_at DESC);
CREATE INDEX idx_agent_tenant_scripts_tenant ON agent_tenant_scripts(tenant_id);

-- agent_script_analysis
CREATE TABLE agent_script_analysis (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    simulation_id TEXT REFERENCES agent_simulations(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL DEFAULT 'external',
    analysis_type TEXT NOT NULL DEFAULT 'compliance',
    input_messages TEXT NOT NULL,
    result TEXT NOT NULL,
    score REAL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_script_analysis_tenant ON agent_script_analysis(tenant_id);
CREATE INDEX idx_script_analysis_simulation ON agent_script_analysis(simulation_id);

-- ============================================================================
-- AGENT QA PAIRS TABLE (migration 17)
-- ============================================================================

CREATE TABLE agent_qa_pairs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    expected_answer TEXT NOT NULL,
    source_section TEXT,
    source_chunk_id TEXT,
    difficulty TEXT DEFAULT 'medium',
    category TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_qa_pairs_tenant ON agent_qa_pairs(tenant_id);
CREATE INDEX idx_qa_pairs_category ON agent_qa_pairs(category);
CREATE INDEX idx_qa_pairs_active ON agent_qa_pairs(is_active);

-- ============================================================================
-- AGENT TRANSCRIPTS TABLE (migration 18)
-- ============================================================================

CREATE TABLE agent_transcripts (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    audience_type TEXT NOT NULL,
    messages TEXT NOT NULL DEFAULT '[]',
    message_count INTEGER DEFAULT 0,
    client_ip TEXT,
    user_agent TEXT,
    customer_name TEXT,
    customer_phone TEXT,
    customer_email TEXT,
    customer_location TEXT,
    detected_intent TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    last_message_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_completed INTEGER DEFAULT 0,
    completion_reason TEXT,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_transcripts_tenant ON agent_transcripts(tenant_id);
CREATE INDEX idx_transcripts_started ON agent_transcripts(started_at DESC);
CREATE INDEX idx_transcripts_audience ON agent_transcripts(tenant_id, audience_type);
CREATE INDEX idx_transcripts_session ON agent_transcripts(session_id);

-- ============================================================================
-- AGENT WORKFLOWS TABLE (migration 03)
-- ============================================================================

CREATE TABLE agent_workflows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    agent_name TEXT NOT NULL DEFAULT 'antigravity',
    task_name TEXT,
    task_mode TEXT CHECK(task_mode IN ('PLANNING', 'EXECUTION', 'VERIFICATION')),
    task_status TEXT,
    task_summary TEXT,
    predicted_size INTEGER,
    created_ts INTEGER NOT NULL,
    metadata TEXT DEFAULT '{}'
);

CREATE INDEX idx_workflows_ticket ON agent_workflows(ticket_id);
CREATE INDEX idx_workflows_session ON agent_workflows(session_id);
CREATE INDEX idx_workflows_created ON agent_workflows(created_ts);

-- ============================================================================
-- AGENT SIMULATION TRANSCRIPTS TABLE (migration 08)
-- ============================================================================

CREATE TABLE agent_simulation_transcripts (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES user(id),
    initial_prompt TEXT NOT NULL,
    persona_hint TEXT,
    total_turns INTEGER NOT NULL DEFAULT 0,
    end_reason TEXT NOT NULL DEFAULT 'unknown',
    messages TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_simulation_transcript_tenant ON agent_simulation_transcripts(tenant_id);
CREATE INDEX idx_simulation_transcript_user ON agent_simulation_transcripts(user_id);
CREATE INDEX idx_simulation_transcript_created ON agent_simulation_transcripts(created_at);

-- ============================================================================
-- AGENT ANALYSIS RESULTS TABLE (migration 09)
-- ============================================================================

CREATE TABLE agent_analysis_results (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    conversation_id TEXT NOT NULL,
    conversation_type TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    score INTEGER NOT NULL,
    grade TEXT NOT NULL,
    breakdown TEXT NOT NULL,
    issues TEXT NOT NULL,
    suggestions TEXT,
    benchmark_version TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_analysis_tenant ON agent_analysis_results(tenant_id);
CREATE INDEX idx_agent_analysis_conversation ON agent_analysis_results(conversation_id);
CREATE INDEX idx_agent_analysis_created ON agent_analysis_results(created_at);

-- ============================================================================
-- AGENT LEARNING MEMORY TABLE (migration 10)
-- ============================================================================

CREATE TABLE agent_learning_memory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE,
    common_issues TEXT NOT NULL DEFAULT '[]',
    learned_behaviors TEXT NOT NULL DEFAULT '[]',
    improvement_areas TEXT NOT NULL DEFAULT '[]',
    pending_suggestions TEXT NOT NULL DEFAULT '[]',
    analysis_count INTEGER DEFAULT 0,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_learning_memory_tenant ON agent_learning_memory(tenant_id);

-- ============================================================================
-- AGENT COMPLIANCE TABLES (migration 11)
-- ============================================================================

CREATE TABLE agent_compliance_audits (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    conversation_id TEXT NOT NULL,
    conversation_type TEXT NOT NULL,
    score INTEGER NOT NULL,
    checks TEXT NOT NULL,
    overall_passed BOOLEAN NOT NULL DEFAULT 0,
    audited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_compliance_audit_tenant ON agent_compliance_audits(tenant_id);
CREATE INDEX idx_compliance_audit_conversation ON agent_compliance_audits(conversation_id);
CREATE INDEX idx_compliance_audit_score ON agent_compliance_audits(score);
CREATE INDEX idx_compliance_audit_date ON agent_compliance_audits(audited_at);

CREATE TABLE agent_scoring_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE,
    version TEXT NOT NULL DEFAULT '1.0',
    config TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_scoring_config_tenant ON agent_scoring_config(tenant_id);

-- ============================================================================
-- AGENT REINDEX CHECKPOINTS TABLE (migration 21)
-- ============================================================================

CREATE TABLE agent_reindex_checkpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    audience TEXT NOT NULL,
    total_chunks INTEGER NOT NULL,
    processed_chunks INTEGER NOT NULL DEFAULT 0,
    current_batch INTEGER NOT NULL DEFAULT 0,
    total_batches INTEGER NOT NULL,
    batch_size INTEGER NOT NULL DEFAULT 25,
    status TEXT NOT NULL DEFAULT 'in_progress',
    error_message TEXT,
    last_message TEXT NOT NULL DEFAULT '',
    error_batch INTEGER,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_reindex_checkpoint_tenant_audience
ON agent_reindex_checkpoints(tenant_id, audience);

-- ============================================================================
-- AGENT OBSERVATIONS TABLE (migration 24, enhanced in migration 26, 27)
-- ============================================================================

CREATE TABLE agent_observations (
    session_id TEXT PRIMARY KEY REFERENCES agent_sessions(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    
    -- The Observation State
    observation_log TEXT DEFAULT '',
    last_observed_msg_index INTEGER DEFAULT 0,
    
    -- Metrics
    tokens_in_log INTEGER DEFAULT 0,
    
    -- Task Tracking (migration 26)
    current_task TEXT,
    suggested_response TEXT,
    
    -- Resource Scope (migration 27)
    resource_id TEXT DEFAULT '',
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_observations_tenant ON agent_observations(tenant_id);
CREATE INDEX idx_agent_observations_resource ON agent_observations(resource_id);

-- ============================================================================
-- BRIDGE SESSION FOUNDATION (migration 29)
-- ============================================================================

CREATE TABLE bridge_external_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'closed', 'expired')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    expires_at INTEGER,
    last_seen_at INTEGER,
    UNIQUE(tenant_id, session_id)
);

CREATE INDEX idx_bridge_external_sessions_tenant_status ON bridge_external_sessions(tenant_id, status);
CREATE INDEX idx_bridge_external_sessions_expiry ON bridge_external_sessions(expires_at);
CREATE INDEX idx_bridge_external_sessions_tenant_session ON bridge_external_sessions(tenant_id, session_id);

CREATE TABLE bridge_handoffs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_session_id INTEGER NOT NULL REFERENCES bridge_external_sessions(id) ON DELETE CASCADE,
    handoff_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    generation INTEGER NOT NULL CHECK(generation > 0),
    routing_mode TEXT NOT NULL DEFAULT 'handoff_queued' CHECK(routing_mode IN ('handoff_queued', 'human_active', 'closed')),
    outcome TEXT CHECK(outcome IS NULL OR outcome IN ('released', 'timeout_released', 'resolved', 'rejected', 'failed', 'closed')),
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version > 0),
    harness_id TEXT,
    operator_id TEXT,
    ticket_id INTEGER,
    memo_uid TEXT,
    transition_reason TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    closed_at INTEGER,
    UNIQUE(external_session_id, generation),
    UNIQUE(tenant_id, session_id, generation),
    UNIQUE(tenant_id, handoff_id)
);

CREATE INDEX idx_bridge_handoffs_external_active ON bridge_handoffs(external_session_id, active);
CREATE INDEX idx_bridge_handoffs_tenant_session_active ON bridge_handoffs(tenant_id, session_id, active);
CREATE INDEX idx_bridge_handoffs_tenant_mode ON bridge_handoffs(tenant_id, routing_mode);
CREATE INDEX idx_bridge_handoffs_tenant_handoff ON bridge_handoffs(tenant_id, handoff_id);
CREATE UNIQUE INDEX idx_bridge_handoffs_one_active ON bridge_handoffs(external_session_id) WHERE active = 1;

CREATE TABLE bridge_auth_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    key_id TEXT NOT NULL,
    label TEXT,
    secret_key_encrypted BLOB NOT NULL,
    secret_key_nonce BLOB NOT NULL CHECK(length(secret_key_nonce) = 12),
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'revoked')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_used_at INTEGER,
    revoked_at INTEGER,
    UNIQUE(tenant_id, key_id),
    CHECK(length(key_id) BETWEEN 16 AND 128),
    CHECK(
        (status = 'active' AND revoked_at IS NULL)
        OR
        (status = 'revoked' AND revoked_at IS NOT NULL)
    )
);

CREATE INDEX idx_bridge_auth_keys_tenant_status ON bridge_auth_keys(tenant_id, status);

CREATE TABLE bridge_auth_nonces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    key_id TEXT NOT NULL,
    nonce TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    UNIQUE(tenant_id, key_id, nonce),
    FOREIGN KEY (tenant_id, key_id) REFERENCES bridge_auth_keys(tenant_id, key_id) ON DELETE CASCADE,
    CHECK(length(key_id) BETWEEN 16 AND 128),
    CHECK(length(nonce) BETWEEN 16 AND 128),
    CHECK(expires_at > created_at),
    CHECK(expires_at > timestamp)
);

CREATE INDEX idx_bridge_auth_nonces_tenant_key ON bridge_auth_nonces(tenant_id, key_id);
CREATE INDEX idx_bridge_auth_nonces_expiry ON bridge_auth_nonces(expires_at);

CREATE TABLE IF NOT EXISTS bridge_handoff_replies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reply_id TEXT NOT NULL UNIQUE CHECK(length(reply_id) > 0 AND length(reply_id) <= 36),
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL CHECK(length(session_id) > 0),
    handoff_id TEXT NOT NULL CHECK(length(handoff_id) > 0),
    generation INTEGER NOT NULL,
    client_message_id TEXT NOT NULL CHECK(length(client_message_id) > 0 AND length(client_message_id) <= 128),
    text TEXT NOT NULL CHECK(length(text) > 0 AND length(text) <= 2000),
    delivery_status TEXT NOT NULL DEFAULT 'not_delivered' CHECK(delivery_status = 'not_delivered'),
    created_at INTEGER NOT NULL,

    UNIQUE(tenant_id, session_id, handoff_id, client_message_id),
    FOREIGN KEY (tenant_id, handoff_id)
        REFERENCES bridge_handoffs(tenant_id, handoff_id)
        ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, session_id)
        REFERENCES bridge_external_sessions(tenant_id, session_id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bridge_handoff_replies_lookup
ON bridge_handoff_replies(tenant_id, session_id, handoff_id, client_message_id);

-- bridge_reply_outbox is a durable preparation table only.
-- The claim layer has no delivery worker, no adapter, no endpoint, and no ChatExternal consumer.

CREATE UNIQUE INDEX IF NOT EXISTS idx_bridge_handoff_replies_tenant_reply
ON bridge_handoff_replies(tenant_id, reply_id);

CREATE TABLE IF NOT EXISTS bridge_reply_outbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  outbox_id TEXT NOT NULL UNIQUE CHECK(length(outbox_id) = 36),

  tenant_id INTEGER NOT NULL,
  session_id TEXT NOT NULL CHECK(length(session_id) > 0),
  handoff_id TEXT NOT NULL CHECK(length(handoff_id) > 0),
  reply_id TEXT NOT NULL CHECK(length(reply_id) = 36),

  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),

  claim_token TEXT UNIQUE CHECK(claim_token IS NULL OR length(claim_token) = 36),
  claimed_by TEXT CHECK(claimed_by IS NULL OR length(claimed_by) BETWEEN 1 AND 128),
  claimed_at INTEGER CHECK(claimed_at IS NULL OR claimed_at > 0),
  claim_expires_at INTEGER CHECK(claim_expires_at IS NULL OR claim_expires_at > 0),

  completed_at INTEGER CHECK(completed_at IS NULL OR completed_at > 0),

  failed_at INTEGER CHECK(failed_at IS NULL OR failed_at > 0),
  failure_code TEXT CHECK(failure_code IS NULL OR length(failure_code) BETWEEN 1 AND 64),
  failure_message TEXT CHECK(failure_message IS NULL OR length(failure_message) BETWEEN 1 AND 1000),

  created_at INTEGER NOT NULL,

  UNIQUE(tenant_id, reply_id),

  FOREIGN KEY (tenant_id, session_id)
    REFERENCES bridge_external_sessions(tenant_id, session_id)
    ON DELETE CASCADE,

  FOREIGN KEY (tenant_id, handoff_id)
    REFERENCES bridge_handoffs(tenant_id, handoff_id)
    ON DELETE CASCADE,

  FOREIGN KEY (tenant_id, reply_id)
    REFERENCES bridge_handoff_replies(tenant_id, reply_id)
    ON DELETE CASCADE,

  -- Strict State Machine Constraints
  CHECK(
    (status = 'pending'
      AND claim_token IS NULL
      AND claimed_by IS NULL
      AND claimed_at IS NULL
      AND claim_expires_at IS NULL
      AND completed_at IS NULL
      AND failed_at IS NULL
      AND failure_code IS NULL
      AND failure_message IS NULL)
    OR
    (status = 'claimed'
      AND claim_token IS NOT NULL
      AND claimed_by IS NOT NULL
      AND claimed_at IS NOT NULL
      AND claim_expires_at IS NOT NULL
      AND claim_expires_at > claimed_at
      AND completed_at IS NULL
      AND failed_at IS NULL
      AND failure_code IS NULL
      AND failure_message IS NULL)
    OR
    (status = 'completed'
      AND claim_token IS NOT NULL
      AND claimed_by IS NOT NULL
      AND claimed_at IS NOT NULL
      AND claim_expires_at IS NOT NULL
      AND claim_expires_at > claimed_at
      AND completed_at IS NOT NULL
      AND completed_at >= claimed_at
      AND failed_at IS NULL
      AND failure_code IS NULL
      AND failure_message IS NULL)
    OR
    (status = 'failed'
      AND claim_token IS NOT NULL
      AND claimed_by IS NOT NULL
      AND claimed_at IS NOT NULL
      AND claim_expires_at IS NOT NULL
      AND claim_expires_at > claimed_at
      AND completed_at IS NULL
      AND failed_at IS NOT NULL
      AND failed_at >= claimed_at
      AND failure_code IS NOT NULL
      AND failure_message IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_bridge_reply_outbox_claimable
ON bridge_reply_outbox(tenant_id, status, claim_expires_at, created_at);

CREATE TABLE IF NOT EXISTS agent_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL,
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_source_lookup
    ON agent_messages(session_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_agent_messages_tenant ON agent_messages(tenant_id);

CREATE TABLE IF NOT EXISTS agent_leads (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    transcript_id TEXT,
    name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    topic TEXT,
    location TEXT,
    detected_intent TEXT,
    status TEXT NOT NULL DEFAULT 'new',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_message_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    converted_at TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES agent_tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (transcript_id) REFERENCES agent_transcripts(id) ON DELETE SET NULL,
    CHECK (email IS NOT NULL OR phone IS NOT NULL),
    UNIQUE(tenant_id, session_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_leads_tenant_status
    ON agent_leads(tenant_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_leads_session
    ON agent_leads(tenant_id, session_id);
