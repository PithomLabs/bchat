# RBAC (Role-Based Access Control) Proposal

## Document Info
- **Status:** Proposal
- **Created:** 2026-01-19
- **Author:** Claude Code

---

## 1. Executive Summary

This document proposes enhancements to the existing role-based access control (RBAC) system to support:
1. **Secure API key storage** - Encrypted storage for OPENROUTER_API_KEY per tenant
2. **LLM model configuration** - Tenant-level LLM model selection
3. **User-tenant associations** - Allow users to access specific tenants for testing/monitoring
4. **Flexible permission model** - Extensible system for future access control needs

---

## 2. Current State Analysis

### 2.1 Existing Role System

The current implementation uses a **three-tier global role system**:

| Role | Description | Capabilities |
|------|-------------|--------------|
| HOST | System owner (first user) | Full system access, create users, manage all settings |
| ADMIN | Administrator | Access admin pages, manage tenants |
| USER | Regular user | Basic memo functionality only |

**Implementation locations:**
- Role enum: `store/user.go`
- Database constraint: `store/migration/sqlite/LATEST.sql`
- gRPC authorization: `server/router/api/v1/acl.go`
- REST authorization: `server/router/api/v1/agent/handlers.go` (`isAdmin()`)

### 2.2 Current Limitations

1. **No granular permissions** - Binary admin/non-admin check
2. **No user-tenant binding** - Admins can access ALL tenants
3. **No tenant-level configuration** - API keys stored in environment variables
4. **No audit trail** - No logging of who did what
5. **Frontend relies on API rejection** - No route-level guards

### 2.3 Database Schema (Current)

```sql
-- User table (existing)
CREATE TABLE user (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  role TEXT NOT NULL CHECK (role IN ('HOST', 'ADMIN', 'USER')) DEFAULT 'USER',
  -- ... other fields
);

-- Tenant table (existing)
CREATE TABLE agent_tenant (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL UNIQUE,
  company_name TEXT NOT NULL,
  -- ... other fields
);
```

---

## 3. Proposed Architecture

### 3.1 Design Principles

1. **Backward compatible** - Existing HOST/ADMIN/USER roles continue to work
2. **Additive permissions** - Users accumulate permissions from multiple sources
3. **Deny by default** - Access denied unless explicitly granted
4. **Tenant isolation** - Strong boundaries between tenant data
5. **Simple to use** - Minimal cognitive overhead for administrators

### 3.2 Permission Model Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                       GLOBAL ROLES                               │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐                        │
│  │  HOST   │ > │  ADMIN  │ > │  USER   │                        │
│  │ (super) │   │ (admin) │   │ (basic) │                        │
│  └─────────┘   └─────────┘   └─────────┘                        │
└─────────────────────────────────────────────────────────────────┘
                              +
┌─────────────────────────────────────────────────────────────────┐
│                    TENANT-LEVEL PERMISSIONS                      │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  user_tenant_permission                                   │   │
│  │  ┌────────────┬────────────┬──────────────────────────┐  │   │
│  │  │  user_id   │ tenant_id  │       permissions        │  │   │
│  │  ├────────────┼────────────┼──────────────────────────┤  │   │
│  │  │     1      │     1      │  tenant:read,chat:test   │  │   │
│  │  │     2      │     1      │  tenant:admin            │  │   │
│  │  │     2      │     2      │  tenant:read             │  │   │
│  │  └────────────┴────────────┴──────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 3.3 Permission Hierarchy

```
Permission String Format: <resource>:<action>

Defined Permissions:
├── tenant:admin      → Full tenant management (CRUD, config, files)
├── tenant:read       → View tenant config, stats, conversation logs
├── tenant:write      → Modify tenant settings (not delete)
├── chat:test         → Send test messages via internal chat
├── chat:logs         → View chat conversation history
├── files:upload      → Upload KB/Policy files
├── files:restore     → Restore file versions
└── api:config        → Configure API keys and LLM settings

Special Permissions:
├── *                 → All permissions (HOST only)
└── tenant:*          → All tenant permissions for specific tenant
```

### 3.4 Access Resolution Algorithm

```go
func HasPermission(user User, tenantID int32, permission string) bool {
    // 1. HOST has all permissions
    if user.Role == RoleHost {
        return true
    }

    // 2. Check global role grants
    if user.Role == RoleAdmin {
        // ADMIN has implicit tenant:read for all tenants
        if permission == "tenant:read" {
            return true
        }
    }

    // 3. Check explicit user-tenant permissions
    perms := GetUserTenantPermissions(user.ID, tenantID)
    return perms.Contains(permission) || perms.Contains("*")
}
```

---

## 4. Database Schema Changes

### 4.1 New Tables

```sql
-- User-Tenant Permission Association
CREATE TABLE user_tenant_permission (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenant(id) ON DELETE CASCADE,
    permissions TEXT NOT NULL DEFAULT '',  -- Comma-separated: "tenant:read,chat:test"
    granted_by INTEGER REFERENCES user(id),
    granted_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    UNIQUE(user_id, tenant_id)
);

CREATE INDEX idx_user_tenant_permission_user ON user_tenant_permission(user_id);
CREATE INDEX idx_user_tenant_permission_tenant ON user_tenant_permission(tenant_id);
```

### 4.2 Tenant Configuration Table (New)

```sql
-- Tenant-level configuration (API keys, LLM settings)
CREATE TABLE tenant_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE REFERENCES agent_tenant(id) ON DELETE CASCADE,

    -- LLM Configuration
    llm_model TEXT NOT NULL DEFAULT '',           -- e.g., "openai/gpt-4o-mini"
    openrouter_api_key_encrypted BLOB,            -- AES-256-GCM encrypted
    openrouter_api_key_nonce BLOB,                -- Nonce for decryption

    -- Feature flags (extensible)
    features TEXT NOT NULL DEFAULT '{}',          -- JSON object

    -- Audit
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_by INTEGER REFERENCES user(id)
);

CREATE INDEX idx_tenant_config_tenant ON tenant_config(tenant_id);
```

### 4.3 System Settings Table (New)

```sql
-- System-wide encryption key (stored separately, HOST-only access)
CREATE TABLE system_secret (
    id INTEGER PRIMARY KEY CHECK (id = 1),  -- Singleton
    encryption_key_encrypted BLOB NOT NULL, -- Encrypted with master password
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT
);
```

### 4.4 Migration Script

```sql
-- Migration: Add RBAC tables
-- Version: 20260119_add_rbac

-- 1. User-Tenant Permissions
CREATE TABLE IF NOT EXISTS user_tenant_permission (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenant(id) ON DELETE CASCADE,
    permissions TEXT NOT NULL DEFAULT '',
    granted_by INTEGER REFERENCES user(id),
    granted_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    UNIQUE(user_id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_user
    ON user_tenant_permission(user_id);
CREATE INDEX IF NOT EXISTS idx_user_tenant_permission_tenant
    ON user_tenant_permission(tenant_id);

-- 2. Tenant Configuration
CREATE TABLE IF NOT EXISTS tenant_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL UNIQUE REFERENCES agent_tenant(id) ON DELETE CASCADE,
    llm_model TEXT NOT NULL DEFAULT '',
    openrouter_api_key_encrypted BLOB,
    openrouter_api_key_nonce BLOB,
    features TEXT NOT NULL DEFAULT '{}',
    updated_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_by INTEGER REFERENCES user(id)
);

CREATE INDEX IF NOT EXISTS idx_tenant_config_tenant
    ON tenant_config(tenant_id);

-- 3. System Secrets
CREATE TABLE IF NOT EXISTS system_secret (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    encryption_key_encrypted BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    rotated_at BIGINT
);
```

---

## 5. API Key Encryption

### 5.1 Encryption Strategy

**Algorithm:** AES-256-GCM (Authenticated Encryption)

**Key Derivation:**
1. Master password provided via environment variable: `ENCRYPTION_MASTER_KEY`
2. Derive encryption key using Argon2id (memory-hard KDF)
3. Store derived key encrypted in `system_secret` table
4. Key rotation supported via `key_version`

### 5.2 Go Implementation

```go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"

    "golang.org/x/crypto/argon2"
)

const (
    KeySize   = 32 // AES-256
    NonceSize = 12 // GCM standard
    SaltSize  = 16
)

type EncryptionService struct {
    key []byte
}

func NewEncryptionService(masterPassword string, salt []byte) *EncryptionService {
    // Argon2id key derivation
    key := argon2.IDKey(
        []byte(masterPassword),
        salt,
        1,      // time
        64*1024, // memory (64 MB)
        4,       // parallelism
        KeySize,
    )
    return &EncryptionService{key: key}
}

func (s *EncryptionService) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, nil, err
    }

    nonce = make([]byte, NonceSize)
    if _, err := rand.Read(nonce); err != nil {
        return nil, nil, err
    }

    ciphertext = gcm.Seal(nil, nonce, []byte(plaintext), nil)
    return ciphertext, nonce, nil
}

func (s *EncryptionService) Decrypt(ciphertext, nonce []byte) (string, error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", errors.New("decryption failed: invalid key or corrupted data")
    }

    return string(plaintext), nil
}
```

### 5.3 Security Considerations

1. **Master key in environment** - Never stored in database
2. **Unique nonce per encryption** - Prevents replay attacks
3. **Authenticated encryption** - GCM mode detects tampering
4. **Key rotation** - Supported via version field
5. **Memory-hard KDF** - Argon2id resists GPU attacks
6. **Clear from memory** - Sensitive data zeroed after use

---

## 6. API Changes

### 6.1 New Endpoints

```
# Tenant Configuration
GET    /api/v1/agent/:slug/llm-config     → Get LLM config (model only, not key)
PUT    /api/v1/agent/:slug/llm-config     → Set LLM config (admin only)

# User-Tenant Permissions
GET    /api/v1/agent/:slug/permissions    → List users with access
POST   /api/v1/agent/:slug/permissions    → Grant user access
DELETE /api/v1/agent/:slug/permissions/:userId → Revoke user access

# User's Accessible Tenants
GET    /api/v1/user/tenants               → List tenants user can access
```

### 6.2 Request/Response Schemas

```typescript
// PUT /api/v1/agent/:slug/llm-config
interface SetLLMConfigRequest {
    llmModel: string;           // e.g., "openai/gpt-4o-mini"
    openrouterApiKey?: string;  // Only sent when updating, not returned
}

interface LLMConfigResponse {
    tenantSlug: string;
    llmModel: string;
    hasApiKey: boolean;         // true if key is set, never returns actual key
    updatedAt: string;
}

// POST /api/v1/agent/:slug/permissions
interface GrantPermissionRequest {
    userId: number;
    permissions: string[];      // ["tenant:read", "chat:test"]
}

interface UserTenantPermission {
    userId: number;
    username: string;
    permissions: string[];
    grantedBy: string;
    grantedAt: string;
}

// GET /api/v1/user/tenants
interface UserAccessibleTenant {
    tenant: AgentTenant;
    permissions: string[];
}
```

### 6.3 Modified Endpoints

```go
// handlers.go - Updated authorization check

func (h *Handler) hasPermission(c echo.Context, tenantID int32, permission string) bool {
    userID, ok := c.Get("user-id").(int32)
    if !ok {
        return false
    }

    user, err := h.store.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
    if err != nil || user == nil {
        return false
    }

    // HOST has all permissions
    if user.Role == store.RoleHost {
        return true
    }

    // ADMIN has implicit read access
    if user.Role == store.RoleAdmin && permission == "tenant:read" {
        return true
    }

    // Check explicit permissions
    perms, err := h.store.GetUserTenantPermissions(c.Request().Context(), userID, tenantID)
    if err != nil {
        return false
    }

    return containsPermission(perms, permission)
}
```

---

## 7. Frontend Changes

### 7.1 Admin Page Updates

**AgentAdmin.tsx additions:**

1. **LLM Configuration Section**
   ```tsx
   <LLMConfigSection>
     <TextField label="LLM Model" value={config.llmModel} />
     <TextField label="OpenRouter API Key" type="password"
                placeholder={config.hasApiKey ? "••••••••" : "Not set"} />
     <Button onClick={saveLLMConfig}>Save</Button>
   </LLMConfigSection>
   ```

2. **User Access Management**
   ```tsx
   <UserAccessSection>
     <Typography variant="h6">User Access</Typography>
     <UserList users={tenantUsers} onRevoke={revokeAccess} />
     <AddUserButton onClick={openGrantDialog} />
   </UserAccessSection>
   ```

### 7.2 New Components

```tsx
// LLMConfigPanel.tsx
interface LLMConfigPanelProps {
    tenantSlug: string;
    config: LLMConfig;
    onSave: (config: LLMConfigUpdate) => Promise<void>;
}

// UserPermissionManager.tsx
interface UserPermissionManagerProps {
    tenantSlug: string;
    permissions: UserTenantPermission[];
    onGrant: (userId: number, permissions: string[]) => Promise<void>;
    onRevoke: (userId: number) => Promise<void>;
}
```

### 7.3 MobX Store Updates

```typescript
// agentAdmin.ts additions

interface LLMConfig {
    llmModel: string;
    hasApiKey: boolean;
    updatedAt: string;
}

interface UserTenantPermission {
    userId: number;
    username: string;
    permissions: string[];
    grantedAt: string;
}

class AgentAdminStore {
    // Existing...

    // New
    llmConfig: LLMConfig | null = null;
    tenantPermissions: UserTenantPermission[] = [];

    async fetchLLMConfig(slug: string): Promise<LLMConfig>;
    async updateLLMConfig(slug: string, config: LLMConfigUpdate): Promise<void>;
    async fetchTenantPermissions(slug: string): Promise<UserTenantPermission[]>;
    async grantPermission(slug: string, userId: number, perms: string[]): Promise<void>;
    async revokePermission(slug: string, userId: number): Promise<void>;
}
```

---

## 8. User-Tenant Association UI

### 8.1 Admin View: Manage Tenant Access

```
┌─────────────────────────────────────────────────────────────┐
│  Tenant: acme-corp                                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  LLM Configuration                                          │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ Model:    [openai/gpt-4o-mini    ▼]                 │   │
│  │ API Key:  [••••••••••••••••     ] [Update]          │   │
│  │           ✓ Key is set                              │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  User Access                                   [+ Add User] │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ User          Permissions              Actions      │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │ john.doe      tenant:read, chat:test   [Revoke]     │   │
│  │ jane.smith    tenant:admin             [Revoke]     │   │
│  │ test.user     chat:logs                [Revoke]     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 8.2 User View: My Tenants

Regular users with tenant access see a simplified view:

```
┌─────────────────────────────────────────────────────────────┐
│  My Tenants                                                 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌───────────────────────────────────────────────────┐     │
│  │ ACME Corp                                         │     │
│  │ Permissions: View config, Test chat               │     │
│  │ [View Details] [Test Chat]                        │     │
│  └───────────────────────────────────────────────────┘     │
│                                                             │
│  ┌───────────────────────────────────────────────────┐     │
│  │ Beta Inc                                          │     │
│  │ Permissions: View logs                            │     │
│  │ [View Logs]                                       │     │
│  └───────────────────────────────────────────────────┘     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 9. Permission Presets

For ease of use, provide common permission combinations:

| Preset | Permissions | Use Case |
|--------|-------------|----------|
| **Viewer** | `tenant:read` | Read-only access to config and stats |
| **Tester** | `tenant:read`, `chat:test` | QA team testing chat responses |
| **Analyst** | `tenant:read`, `chat:logs` | Review conversation history |
| **Editor** | `tenant:read`, `tenant:write`, `files:upload` | Content managers |
| **Tenant Admin** | `tenant:*` | Full tenant management |

---

## 10. Implementation Plan

### Phase 1: Database & Backend Foundation (Required First)
- [ ] Create migration for new tables
- [ ] Implement encryption service
- [ ] Add store methods for user-tenant permissions
- [ ] Add store methods for tenant config

### Phase 2: API Endpoints
- [ ] LLM config endpoints (GET/PUT)
- [ ] Permission management endpoints (GET/POST/DELETE)
- [ ] User tenants endpoint (GET)
- [ ] Update existing handlers to use new permission checks

### Phase 3: Frontend - Admin Page
- [ ] Add LLM configuration section to tenant detail view
- [ ] Add user access management section
- [ ] Implement grant/revoke permission dialogs

### Phase 4: Frontend - User View
- [ ] Create "My Tenants" page for non-admin users
- [ ] Add navigation link for users with tenant access
- [ ] Implement test chat interface

### Phase 5: Testing & Documentation
- [ ] Unit tests for encryption
- [ ] Integration tests for permission checks
- [ ] API documentation updates

---

## 11. Environment Variables

### New Variables

```bash
# Required for API key encryption (32+ chars recommended)
ENCRYPTION_MASTER_KEY=your-secure-master-key-here

# Optional: Override default LLM model for tenants without config
LLM_MODEL=openai/gpt-oss-120b:free

# Optional: Fallback API key if tenant has none configured
OPENROUTER_API_KEY_FALLBACK=sk-or-xxxx
```

### LLM_MODEL Options

The `LLM_MODEL` environment variable can be set to any OpenRouter model. Users can enter free text for custom models, but the following free-tier models are recommended:

| Model ID | Description |
|----------|-------------|
| `openai/gpt-oss-120b:free` | **Default** - Open-source GPT-style 120B model |
| `xiaomi/mimo-v2-flash:free` | Xiaomi's MiMo v2 Flash model |
| `nvidia/nemotron-3-nano-30b-a3b:free` | NVIDIA Nemotron-3 Nano 30B |
| `google/gemma-3-27b-it:free` | Google Gemma 3 27B Instruct |
| `qwen/qwen3-coder:free` | Qwen 3 Coder for code generation |
| `z-ai/glm-4.5-air:free` | Z-AI GLM 4.5 Air |
| `nousresearch/hermes-3-llama-3.1-405b:free` | Hermes 3 LLaMA 3.1 405B |

**Note:** Free text entry is supported for any valid OpenRouter model ID.

### Fallback Behavior

```
1. Check tenant_config.openrouter_api_key_encrypted
   ↓ (if not set)
2. Check OPENROUTER_API_KEY_FALLBACK environment variable
   ↓ (if not set)
3. Check OPENROUTER_API_KEY environment variable (existing)
   ↓ (if not set)
4. Return error: "No API key configured"
```

---

## 12. Security Considerations

### 12.1 API Key Handling

1. **Never log API keys** - Mask in all log output
2. **Never return keys in API responses** - Only return `hasApiKey: true/false`
3. **Validate key format** - OpenRouter keys start with `sk-or-`
4. **Rate limit key updates** - Prevent brute-force attempts
5. **Audit key changes** - Log who updated when (without the key value)

### 12.2 Permission Enforcement

1. **Always check server-side** - Never trust frontend
2. **Deny by default** - Explicit grants required
3. **Validate tenant access** - User must have permission for specific tenant
4. **Cascade deletes** - Remove permissions when user/tenant deleted

### 12.3 Encryption Key Management

1. **Never commit master key** - Use secrets management
2. **Rotate periodically** - Support key version field
3. **Secure key derivation** - Use Argon2id, not PBKDF2
4. **Clear memory** - Zero sensitive data after use

---

## 13. Future Extensions

This design supports future enhancements:

1. **Time-limited access** - Add `expires_at` to permission table
2. **Invitation system** - Generate invite links with preset permissions
3. **Audit logging** - Add `permission_audit_log` table
4. **Permission groups** - Create reusable permission sets
5. **API token scoping** - Scope API tokens to specific tenants
6. **SSO integration** - Map SAML/OIDC groups to permissions

---

## 14. Summary

This proposal introduces:

| Feature | Implementation |
|---------|----------------|
| Secure API key storage | AES-256-GCM encryption with Argon2id KDF |
| LLM model per tenant | `tenant_config` table with model field |
| User-tenant association | `user_tenant_permission` table with granular permissions |
| Backward compatibility | Existing roles (HOST/ADMIN/USER) unchanged |
| Extensibility | Permission string format supports new permissions |
| Ease of use | Permission presets for common scenarios |

The design maintains the simplicity of the current role system while adding the flexibility needed for multi-tenant management.
