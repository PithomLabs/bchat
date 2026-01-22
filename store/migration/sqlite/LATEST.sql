-- migration_history
CREATE TABLE migration_history (
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
  creator_id INTEGER NOT NULL,
  assignee_id INTEGER,
  created_ts BIGINT NOT NULL,
  updated_ts BIGINT NOT NULL,
  type TEXT NOT NULL DEFAULT 'TASK',
  tags TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX idx_tickets_creator_id ON tickets (creator_id);
CREATE INDEX idx_tickets_status ON tickets (status);

-- notifications
CREATE TABLE notification (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  type TEXT NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}',
  is_read INTEGER NOT NULL DEFAULT 0,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX idx_notification_user_id ON notification (user_id);
CREATE INDEX idx_notification_is_read ON notification (is_read);

-- ============================================================================
-- AGENT CHAT SYSTEM TABLES
-- ============================================================================

-- agent_tenants
CREATE TABLE agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    company_name TEXT NOT NULL,
    vertical TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

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
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_source_files_lookup ON agent_source_files(tenant_id, audience_type, file_type, imported_at DESC);

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
