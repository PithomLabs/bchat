-- Agent Chat System Tables
-- Version: 0.25.5

-- ============================================================================
-- TENANT TABLES
-- ============================================================================

CREATE TABLE IF NOT EXISTS agent_tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    company_name TEXT NOT NULL,
    vertical TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audience-specific configuration
CREATE TABLE IF NOT EXISTS agent_audiences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,            -- 'external' or 'internal'

    -- Identity
    role TEXT NOT NULL,
    tone TEXT NOT NULL,
    brand_voice TEXT,
    guidelines TEXT,                        -- JSON array

    -- Contact
    emergency_phone TEXT NOT NULL,
    secondary_phones TEXT,                  -- JSON array
    email TEXT,
    address TEXT,

    -- Thresholds
    emergency_urgency_threshold INTEGER DEFAULT 4,
    escalation_confidence_threshold REAL DEFAULT 0.85,

    -- Rate limiting (per audience)
    rate_limit_rpm INTEGER DEFAULT 60,      -- Requests per minute

    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(tenant_id, audience_type)
);

-- ============================================================================
-- KNOWLEDGE BASE TABLES (per audience)
-- ============================================================================

CREATE TABLE IF NOT EXISTS agent_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,            -- 'external' or 'internal'
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    is_emergency INTEGER DEFAULT 0,
    response_time TEXT,
    is_active INTEGER DEFAULT 1,

    UNIQUE(tenant_id, audience_type, code)
);

CREATE TABLE IF NOT EXISTS agent_exclusions (
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

CREATE TABLE IF NOT EXISTS agent_coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    -- Coverage is shared across audiences (same service area)
    area_type TEXT NOT NULL,
    area_name TEXT NOT NULL,
    state_code TEXT,
    is_included INTEGER NOT NULL,

    UNIQUE(tenant_id, area_type, area_name)
);

CREATE TABLE IF NOT EXISTS agent_faqs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,

    UNIQUE(tenant_id, audience_type, code)
);

CREATE TABLE IF NOT EXISTS agent_safety_protocols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    trigger_intents TEXT NOT NULL,          -- JSON
    instructions TEXT NOT NULL,             -- JSON
    is_active INTEGER DEFAULT 1,

    UNIQUE(tenant_id, audience_type, code)
);

CREATE TABLE IF NOT EXISTS agent_kb_sections (
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

-- ============================================================================
-- POLICY TABLES (per audience)
-- ============================================================================

CREATE TABLE IF NOT EXISTS agent_intents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT,                     -- NULL = global, 'external'/'internal' = audience-specific
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    description TEXT NOT NULL,
    examples TEXT,                          -- JSON
    counter_examples TEXT,                  -- JSON
    urgency INTEGER,
    action TEXT NOT NULL,
    confidence_threshold REAL,
    is_active INTEGER DEFAULT 1
);

CREATE TABLE IF NOT EXISTS agent_rules (
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

-- ============================================================================
-- SESSION TABLES
-- ============================================================================

-- Internal sessions only (external sessions are in-memory)
CREATE TABLE IF NOT EXISTS agent_sessions (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES user(id),    -- Memos user for internal sessions
    audience_type TEXT NOT NULL DEFAULT 'internal',

    -- State
    phase TEXT DEFAULT 'triage',
    current_intent TEXT,
    urgency_level INTEGER DEFAULT 0,
    coverage_status TEXT DEFAULT 'unknown',

    -- Extracted data
    customer_name TEXT,
    customer_phone TEXT,
    customer_location TEXT,
    detected_service TEXT,

    -- History
    message_count INTEGER DEFAULT 0,
    messages TEXT DEFAULT '[]',             -- JSON

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    is_completed INTEGER DEFAULT 0,
    completion_reason TEXT
);

-- ============================================================================
-- SOURCE FILES
-- ============================================================================

CREATE TABLE IF NOT EXISTS agent_source_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,            -- 'external' or 'internal'
    file_type TEXT NOT NULL,                -- 'kb' or 'policy'
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(tenant_id, audience_type, file_type)
);

-- ============================================================================
-- RATE LIMITING
-- ============================================================================

CREATE TABLE IF NOT EXISTS agent_rate_limits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    audience_type TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    request_count INTEGER DEFAULT 0,
    window_start TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(tenant_id, audience_type, client_ip)
);

-- ============================================================================
-- INDEXES
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_agent_audiences_tenant ON agent_audiences(tenant_id, audience_type);
CREATE INDEX IF NOT EXISTS idx_agent_services_tenant_audience ON agent_services(tenant_id, audience_type);
CREATE INDEX IF NOT EXISTS idx_agent_intents_tenant_audience ON agent_intents(tenant_id, audience_type);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_tenant ON agent_sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_user ON agent_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_rate_limits_lookup ON agent_rate_limits(tenant_id, audience_type, client_ip);
