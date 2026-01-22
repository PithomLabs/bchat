# RBAC Implementation Details

## Document Info
- **Status:** Implementation Reference
- **Created:** 2026-01-19
- **Related:** DOCS_RBAC.MD (design spec)

---

## 1. Overview

The RBAC system provides granular access control for the Agent system. It layers tenant-specific permissions on top of the existing global role system (HOST/ADMIN/USER).

### Access Resolution Flow

```
User Request → Check Global Role → Check Tenant Permission → Allow/Deny

┌─────────────────────────────────────────────────────────────────┐
│  1. Is user HOST?                                               │
│     YES → Allow all operations                                  │
│     NO  → Continue                                              │
├─────────────────────────────────────────────────────────────────┤
│  2. Is user ADMIN?                                              │
│     YES → Allow tenant:read and api:config for all tenants      │
│     NO  → Continue                                              │
├─────────────────────────────────────────────────────────────────┤
│  3. Check user_tenant_permission table                          │
│     Has permission? → Allow                                     │
│     No permission?  → Deny                                      │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Permission Reference

### 2.1 All Permissions

| Permission | Description | Enforcement |
|------------|-------------|-------------|
| `tenant:admin` | Full tenant management | Grants all `tenant:*` permissions |
| `tenant:read` | View tenant configuration | Required for viewing config, stats |
| `tenant:write` | Modify tenant settings | Required for updating tenant fields |
| `chat:test` | Test chat functionality | Required for internal chat endpoint |
| `chat:logs` | View conversation history | Required for viewing session logs |
| `files:upload` | Upload KB/Policy files | Required for file import |
| `files:restore` | Restore file versions | Required for version restore |
| `api:config` | Configure LLM settings | Required for API key management |
| `*` | Wildcard (all permissions) | HOST role only |

### 2.2 Permission Presets

| Preset | Permissions | Use Case |
|--------|-------------|----------|
| **Viewer** | `tenant:read` | View-only access to configuration |
| **Tester** | `tenant:read`, `chat:test` | QA testing chat responses |
| **Analyst** | `tenant:read`, `chat:logs` | Review conversation history |
| **Editor** | `tenant:read`, `tenant:write`, `files:upload` | Content management |
| **Tenant Admin** | `tenant:admin` | Full tenant control |

---

## 3. Permission Enforcement Points

### 3.1 All Enforced Endpoints

| Endpoint | Method | Permission Required | Notes |
|----------|--------|---------------------|-------|
| `/api/v1/agent/:slug/validate` | GET | ADMIN OR `chat:test` | Validate tenant access for Internal Agent |
| `/api/v1/agent/:slug/config` | GET | ADMIN OR `tenant:read` | View tenant configuration |
| `/api/v1/agent/:slug` | PATCH | ADMIN OR `tenant:write` | Update tenant settings |
| `/api/v1/agent/:slug/chat/int` | POST | ADMIN OR `chat:test` | Test internal chat |
| `/api/v1/agent/:slug/import` | POST | ADMIN OR `files:upload` | Upload KB/Policy files |
| `/api/v1/agent/:slug/files/.../restore` | POST | ADMIN OR `files:restore` | Restore file versions |
| `/api/v1/agent/:slug/files/.../versions` | GET | ADMIN OR `files:restore` OR `tenant:read` | View version history |
| `/api/v1/agent/:slug/llm-config` | GET | `tenant:read` OR `api:config` | View LLM config |
| `/api/v1/agent/:slug/llm-config` | PUT | `api:config` OR ADMIN | Update LLM config |
| `/api/v1/agent/:slug/sessions` | GET | ADMIN OR `chat:logs` | List chat sessions |
| `/api/v1/agent/:slug/sessions/:id` | GET | ADMIN OR `chat:logs` | View session details |
| `/api/v1/agent/:slug/permissions` | GET | ADMIN OR `tenant:admin` | List user permissions |
| `/api/v1/agent/:slug/permissions` | POST | ADMIN OR `tenant:admin` | Grant permissions |
| `/api/v1/agent/:slug/permissions/:userId` | DELETE | ADMIN OR `tenant:admin` | Revoke permissions |
| `/api/v1/user/tenants` | GET | Authenticated | Get user's accessible tenants |

### 3.2 Admin-Only Endpoints

These endpoints are restricted to HOST/ADMIN roles only:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/agent/tenants` | GET | List all tenants |
| `/api/v1/agent/onboard` | POST | Create new tenant |
| `/api/v1/agent/:slug` | DELETE | Delete tenant |
| `/api/v1/agent/:slug/export` | GET | Export tenant data |

---

## 4. How Each Permission Works

### 4.1 `tenant:read`

**Purpose:** View-only access to tenant configuration and statistics.

**What it allows:**
- View tenant details (name, slug, vertical, status)
- View endpoint URLs
- View service/intent/FAQ counts
- View LLM model configuration (not API key value)
- View file metadata and version history

**What it does NOT allow:**
- Modify any settings
- Upload files
- Test chat
- View conversation logs

**Enforcement:**
```go
// handlers.go - HandleGetLLMConfig
if !h.hasPermission(c, tenant.ID, PermTenantRead) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
}
```

### 4.2 `tenant:write`

**Purpose:** Modify tenant configuration settings.

**What it allows:**
- Update tenant name, vertical
- Toggle tenant active/inactive status

**What it does NOT allow:**
- Delete tenant
- Manage user permissions
- Upload/restore files
- Configure API keys

**Enforcement:**
```go
// handlers.go - HandleUpdateTenant
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantWrite) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or tenant:write permission")
}
```

### 4.3 `tenant:admin`

**Purpose:** Full administrative control over a specific tenant.

**What it allows:**
- All `tenant:*` permissions
- Manage user permissions for this tenant
- Delete tenant

**Special behavior:**
```go
// permissions.go
if p == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
    return true
}
```

### 4.4 `chat:test`

**Purpose:** Allow testing the internal chat endpoint.

**What it allows:**
- Send messages to `/api/v1/agent/:slug/chat/int`
- Receive AI responses
- Test conversation flow

**What it does NOT allow:**
- View other users' conversations
- Modify tenant configuration

**Enforcement:**
```go
// handlers.go - HandleChatInternal
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:test permission")
}
```

### 4.5 `chat:logs`

**Purpose:** View conversation history and session logs.

**What it allows:**
- View list of chat sessions
- View message history within sessions
- View session metadata (timestamps, completion status)

**What it does NOT allow:**
- Delete sessions
- Modify session data

**Enforcement:**
```go
// handlers.go - HandleListSessions, HandleGetSession
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatLogs) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:logs permission")
}
```

**Endpoints:**
```
GET /api/v1/agent/:slug/sessions        → List sessions
GET /api/v1/agent/:slug/sessions/:id    → Get session details
```

### 4.6 `files:upload`

**Purpose:** Upload KB and Policy markdown files.

**What it allows:**
- Upload new KB.md files
- Upload new POLICY.md files
- Replace existing files (creates new version)

**What it does NOT allow:**
- Delete files
- Restore previous versions

**Enforcement:**
```go
// handlers.go - HandleImportSingleFile
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermFilesUpload) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or files:upload permission")
}
```

### 4.7 `files:restore`

**Purpose:** Restore previous file versions.

**What it allows:**
- View version history
- Restore any previous version

**Enforcement:**
```go
// handlers.go - HandleRestoreFileVersion
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermFilesRestore) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or files:restore permission")
}
```

### 4.8 `api:config`

**Purpose:** Configure LLM settings and API keys.

**What it allows:**
- View current LLM model
- Change LLM model
- Set/update OpenRouter API key

**What it does NOT allow:**
- View the actual API key value (only shows if set)

**Enforcement:**
```go
// handlers.go - HandleSetLLMConfig
if !h.hasPermission(c, tenant.ID, PermAPIConfig) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
}
```

---

## 5. Database Schema

### 5.1 user_tenant_permission Table

```sql
CREATE TABLE user_tenant_permission (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES agent_tenants(id) ON DELETE CASCADE,
    permissions TEXT NOT NULL DEFAULT '',  -- Comma-separated
    granted_by INTEGER REFERENCES user(id) ON DELETE SET NULL,
    granted_at BIGINT NOT NULL,
    UNIQUE(user_id, tenant_id)
);
```

### 5.2 Permission Storage Format

Permissions are stored as comma-separated strings:
```
"tenant:read,chat:test"
"tenant:admin"
"tenant:read,tenant:write,files:upload"
```

---

## 6. API Reference

### 6.1 Get LLM Configuration

```
GET /api/v1/agent/:slug/llm-config

Requires: tenant:read OR api:config

Response:
{
  "tenant_slug": "acme-corp",
  "llm_model": "openai/gpt-oss-120b:free",
  "has_api_key": true,
  "updated_at": "2026-01-19T10:30:00Z"
}
```

### 6.2 Set LLM Configuration

```
PUT /api/v1/agent/:slug/llm-config

Requires: api:config OR ADMIN role

Body:
{
  "llm_model": "openai/gpt-oss-120b:free",
  "openrouter_api_key": "sk-or-..." // Optional
}
```

### 6.3 List User Permissions

```
GET /api/v1/agent/:slug/permissions

Requires: ADMIN role OR tenant:admin

Response:
{
  "permissions": [
    {
      "user_id": 2,
      "username": "john.doe",
      "permissions": ["tenant:read", "chat:test"],
      "granted_by": "admin",
      "granted_at": "2026-01-19T10:30:00Z"
    }
  ]
}
```

### 6.4 Grant Permission

```
POST /api/v1/agent/:slug/permissions

Requires: ADMIN role OR tenant:admin

Body:
{
  "user_id": 2,
  "permissions": ["tenant:read", "chat:test"]
}
```

### 6.5 Revoke Permission

```
DELETE /api/v1/agent/:slug/permissions/:userId

Requires: ADMIN role OR tenant:admin
```

### 6.6 Get User's Accessible Tenants

```
GET /api/v1/user/tenants

Requires: Authenticated

Response:
{
  "tenants": [
    {
      "tenant": { "id": 1, "slug": "acme-corp", ... },
      "permissions": ["tenant:read", "chat:test"]
    }
  ]
}
```

---

## 7. Frontend Integration

### 7.1 Permission Presets in UI

```typescript
export const PERMISSION_PRESETS = {
  viewer: ["tenant:read"],
  tester: ["tenant:read", "chat:test"],
  analyst: ["tenant:read", "chat:logs"],
  editor: ["tenant:read", "tenant:write", "files:upload"],
  tenant_admin: ["tenant:admin"],
};
```

### 7.2 LLM Model Options

```typescript
export const LLM_MODEL_OPTIONS = [
  { value: "openai/gpt-oss-120b:free", label: "GPT-OSS 120B (Default)" },
  { value: "xiaomi/mimo-v2-flash:free", label: "Xiaomi MiMo v2 Flash" },
  { value: "nvidia/nemotron-3-nano-30b-a3b:free", label: "NVIDIA Nemotron-3 Nano 30B" },
  { value: "google/gemma-3-27b-it:free", label: "Google Gemma 3 27B" },
  { value: "qwen/qwen3-coder:free", label: "Qwen 3 Coder" },
  { value: "z-ai/glm-4.5-air:free", label: "Z-AI GLM 4.5 Air" },
  { value: "nousresearch/hermes-3-llama-3.1-405b:free", label: "Hermes 3 LLaMA 405B" },
];
```

---

## 8. Implementation Status

### 8.1 Completed

- [x] Database migration for RBAC tables
- [x] Store layer (CRUD operations)
- [x] Encryption service for API keys
- [x] Permission constants and helpers
- [x] LLM config API endpoints
- [x] Permission management API endpoints
- [x] User tenants API endpoint
- [x] Frontend LLM configuration UI
- [x] Frontend permission management UI
- [x] Enforce `tenant:read` on config endpoint
- [x] Enforce `tenant:write` on update endpoint
- [x] Enforce `chat:test` on internal chat endpoint
- [x] Implement and enforce `chat:logs` endpoint (sessions list and detail)
- [x] Enforce `files:upload` on import endpoint
- [x] Enforce `files:restore` on restore endpoint
- [x] Frontend: Show tenants based on user permissions (non-admin users)
- [x] Frontend: Conditional UI based on permissions

---

## 9. Security Notes

1. **API Key Encryption:** Tenant API keys are encrypted with AES-256-GCM using Argon2id key derivation.

2. **Cascade Deletes:** When a tenant or user is deleted, their permissions are automatically removed.

3. **Audit Trail:** `granted_by` and `granted_at` fields track who granted permissions and when.

4. **Deny by Default:** Users without explicit permissions cannot access tenant resources.

5. **Admin Override:** HOST/ADMIN roles bypass tenant-level permission checks for most operations.

---

## 10. Testing Guide

### 10.1 Prerequisites

1. **Build the project:**
   ```bash
   task build
   ```

2. **Set environment variables:**
   ```bash
   export ENCRYPTION_MASTER_KEY="your-secure-password-here"
   export OPENROUTER_API_KEY="sk-or-..."  # Optional, fallback key
   export LLM_MODEL="openai/gpt-oss-120b:free"  # Optional, fallback model
   ```

3. **Start the server:**
   ```bash
   ./build/memos
   ```

### 10.2 Test Scenarios

#### Scenario 1: Admin Creates Tenant and Grants Permissions

1. **Login as HOST or ADMIN user**
2. **Navigate to Agent Admin** (`/agent-admin`)
3. **Create a new tenant:**
   - Click "Add Tenant"
   - Fill in tenant slug, company name
   - Upload KB and Policy files
   - Click "Create"
4. **Configure LLM:**
   - Select tenant from list
   - Choose LLM model from dropdown
   - Optionally enter OpenRouter API key
   - Click "Save Configuration"
5. **Grant user permissions:**
   - Click "Add User" in User Access section
   - Select a user from dropdown
   - Choose permission preset (e.g., "Tester")
   - Click "Grant Access"

#### Scenario 2: Non-Admin User with Permissions

1. **Login as regular USER with granted permissions**
2. **Navigate to Agent Admin** (`/agent-admin`)
3. **Verify tenant visibility:**
   - Should only see tenants they have access to
   - Should NOT see "Add Tenant" button
4. **Test permission restrictions based on granted preset:**

   | Preset | What They Can Do | What They Cannot Do |
   |--------|------------------|---------------------|
   | Viewer | View config, stats | Edit, upload, delete |
   | Tester | View config, test chat | Upload files, manage permissions |
   | Analyst | View config, view chat logs | Edit, upload, manage permissions |
   | Editor | View, edit, upload files | Manage permissions, delete tenant |
   | Tenant Admin | All except delete tenant | Delete tenant (admin only) |

#### Scenario 3: Test chat:test Permission

1. **Grant `chat:test` permission to a user**
2. **Login as that user**
3. **Navigate to Internal Agent** (`/internal-agent`)
4. **Select the tenant and send a message**
5. **Verify:**
   - User CAN chat with the agent
   - User CANNOT access admin features

#### Scenario 4: Test chat:logs Permission

1. **Grant `chat:logs` permission to a user**
2. **As ADMIN, have some chat sessions with the tenant**
3. **Login as the user with chat:logs**
4. **Call API directly:**
   ```bash
   curl -H "Authorization: Bearer <token>" \
     "http://localhost:8081/api/v1/agent/<slug>/sessions"
   ```
5. **Verify:**
   - User can see list of sessions
   - User can view session details including messages

#### Scenario 5: Test files:upload Permission

1. **Grant `files:upload` permission to a user (via Editor preset)**
2. **Login as that user**
3. **Navigate to Agent Admin and select the tenant**
4. **Verify:**
   - Upload buttons are visible and functional
   - User CAN upload KB and Policy files
   - Version history button is NOT visible (requires files:restore)

#### Scenario 6: Test files:restore Permission

1. **Grant `files:restore` permission to a user**
2. **Login as that user**
3. **Navigate to Agent Admin and select the tenant**
4. **Verify:**
   - "Versions" button is visible
   - User CAN view version history
   - User CAN restore previous versions

#### Scenario 7: Test Permission Denial

1. **Create a user with NO permissions**
2. **Login as that user**
3. **Try to access Agent Admin:**
   - Should see empty tenant list
4. **Try to call API directly:**
   ```bash
   curl -H "Authorization: Bearer <token>" \
     "http://localhost:8081/api/v1/agent/<slug>/config"
   ```
5. **Verify:**
   - Returns 403 Forbidden: "Permission denied"

### 10.3 API Testing with curl

#### Get LLM Config (requires tenant:read or api:config)
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8081/api/v1/agent/<slug>/llm-config"
```

#### Set LLM Config (requires api:config)
```bash
curl -X PUT -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"llm_model": "openai/gpt-oss-120b:free"}' \
  "http://localhost:8081/api/v1/agent/<slug>/llm-config"
```

#### List Permissions (requires tenant:admin)
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8081/api/v1/agent/<slug>/permissions"
```

#### Grant Permission (requires tenant:admin)
```bash
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"user_id": 2, "permissions": ["tenant:read", "chat:test"]}' \
  "http://localhost:8081/api/v1/agent/<slug>/permissions"
```

#### Revoke Permission (requires tenant:admin)
```bash
curl -X DELETE -H "Authorization: Bearer <token>" \
  "http://localhost:8081/api/v1/agent/<slug>/permissions/2"
```

#### Get User's Accessible Tenants
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8081/api/v1/user/tenants"
```

#### List Chat Sessions (requires chat:logs)
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8081/api/v1/agent/<slug>/sessions"
```

#### Get Session Details (requires chat:logs)
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8081/api/v1/agent/<slug>/sessions/<session-id>"
```

### 10.4 Expected Error Responses

| Scenario | HTTP Status | Message |
|----------|-------------|---------|
| Not authenticated | 401 | "Missing access token" |
| No permission | 403 | "Permission denied: requires admin role or {permission}" |
| Tenant not found | 404 | "Tenant not found" |
| Session not found | 404 | "Session not found" |

### 10.5 Verifying Encryption

1. **Set a custom API key for a tenant**
2. **Check the database directly:**
   ```sql
   SELECT openrouter_api_key_encrypted, openrouter_api_key_nonce
   FROM tenant_config WHERE tenant_id = 1;
   ```
3. **Verify:**
   - The encrypted data should be binary (not readable plaintext)
   - Different nonces for each encryption operation

### 10.6 Lost Encryption Key Recovery

If you lose the `ENCRYPTION_MASTER_KEY`:

1. **Encrypted API keys cannot be recovered**
2. **To recover:**
   - Clear the encrypted key data:
     ```sql
     UPDATE tenant_config
     SET openrouter_api_key_encrypted = NULL,
         openrouter_api_key_nonce = NULL;
     ```
   - Clear the salt (forces new key derivation):
     ```sql
     DELETE FROM system_secret;
     ```
   - Set a new `ENCRYPTION_MASTER_KEY`
   - Re-enter API keys via the UI

---

## Addendum: Changes Made (2026-01-19)

### A.1 Permission Enforcement Updates

The following changes were made to enforce permissions across all endpoints:

#### Backend Changes (`server/router/api/v1/agent/handlers.go`)

| Handler | Change | Permission Required |
|---------|--------|---------------------|
| `HandleGetTenantFullConfig` | Added permission check | ADMIN OR `tenant:read` |
| `HandleUpdateTenant` | Added permission check | ADMIN OR `tenant:write` |
| `HandleChatInternal` | Added permission check | ADMIN OR `chat:test` |
| `HandleImportSingleFile` | Added permission check | ADMIN OR `files:upload` |
| `HandleRestoreFileVersion` | Added permission check | ADMIN OR `files:restore` |
| `HandleValidateTenant` | Added permission check | ADMIN OR `chat:test` |

#### New Endpoints Added

| Endpoint | Method | Handler | Permission |
|----------|--------|---------|------------|
| `/api/v1/agent/:slug/sessions` | GET | `HandleListSessions` | ADMIN OR `chat:logs` |
| `/api/v1/agent/:slug/sessions/:sessionId` | GET | `HandleGetSession` | ADMIN OR `chat:logs` |

### A.2 Frontend Permission Integration

#### Agent Admin Page (`web/src/pages/AgentAdmin.tsx`)

1. **User Role Detection**: Added check for current user's role (HOST/ADMIN/USER)
2. **Conditional Tenant Fetching**:
   - ADMIN users: Fetch all tenants via `/api/v1/agent/tenants`
   - Regular users: Fetch accessible tenants via `/api/v1/user/tenants`
3. **Conditional UI Rendering**:
   - "Add Tenant" button: Only visible to ADMIN
   - "Delete Tenant" button/section: Only visible to ADMIN
   - LLM Configuration: Visible if `canRead` OR `canConfigApi`, editable only if `canConfigApi`
   - User Permissions section: Only visible if `canManagePermissions`
   - File Upload buttons: Only visible if `canUpload`
   - Version History buttons: Only visible if `canRestore`

#### Agent Admin Store (`web/src/store/v2/agentAdmin.ts`)

1. **New State Fields**:
   - `myTenantAccess`: Stores user's accessible tenants with permissions
   - `myPermissions`: Current user's permissions for selected tenant

2. **New Methods**:
   - `fetchUserTenants()`: Fetch tenants accessible to current user
   - `setMyPermissions(tenantId)`: Set permissions for selected tenant
   - `hasPermission(permission)`: Check if user has specific permission
   - `clearMyPermissions()`: Clear permissions on tenant deselect

#### Internal Agent Page - Tenant Validation

**Problem**: Users could enter any tenant slug and connect to Internal Agent chat without permission checks.

**Solution**: Updated `HandleValidateTenant` endpoint to check `chat:test` permission before allowing connection.

**Files Changed**:
- `server/router/api/v1/agent/handlers.go`: Added permission check to `HandleValidateTenant`
- `web/src/store/v2/agentChat.ts`: Updated error handling to show permission denied messages

**Before**:
```
User enters slug → Validate tenant exists → Allow connection → 403 on send (maybe)
```

**After**:
```
User enters slug → Validate tenant exists → Check chat:test permission → 403 if denied → Allow connection
```

### A.3 Grant User Access Modal Fix

**Problem**: User dropdown in Grant User Access modal was unclickable due to z-index issue.

**Solution**: Added `slotProps` to MUI Joy Select component to ensure dropdown appears above modal.

```tsx
<Select
  slotProps={{
    listbox: {
      sx: { zIndex: 99999 },
    },
  }}
>
```

### A.4 User Fetching Fix

**Problem**: `fetchUsers` was calling wrong endpoint format.

**Solution**: Changed to use gRPC client `userServiceClient.listUsers({})` with proper response mapping.

```typescript
const { users } = await userServiceClient.listUsers({});
state.allUsers = users.map((u) => ({
  id: parseInt(u.name.replace("users/", ""), 10),
  name: u.nickname || u.username,
  username: u.username,
  role: u.role.toString(),
}));
```

### A.5 Summary of All Files Changed

| File | Type | Changes |
|------|------|---------|
| `handlers.go` | Backend | Permission checks on 6 handlers, 2 new session endpoints |
| `v1.go` | Backend | Route registration for session endpoints |
| `AgentAdmin.tsx` | Frontend | Conditional UI, permission flags, user role detection |
| `agentAdmin.ts` | Frontend | Permission state, user tenant access, gRPC user fetching |
| `agentChat.ts` | Frontend | Better 403 error handling |

### A.6 Permission Separation Clarification

**Important:** Permissions are intentionally granular and do not imply each other.

| Permission | Grants Access To | Does NOT Grant |
|------------|------------------|----------------|
| `tenant:read` | Agent Admin: view config, stats, LLM model | Internal Agent chat |
| `chat:test` | Internal Agent: send/receive messages | Agent Admin config viewing |
| `tenant:write` | Agent Admin: edit tenant settings | File uploads, chat |
| `files:upload` | Agent Admin: upload KB/Policy files | File restore, chat |
| `tenant:admin` | All `tenant:*` permissions (read, write) | `chat:*`, `files:*`, `api:*` |

**Permission Inheritance:**
- `tenant:admin` → implies `tenant:read`, `tenant:write` (prefix match `tenant:*`)
- `tenant:admin` → does **NOT** imply `chat:test`, `chat:logs`, `files:upload`, etc.
- `*` (wildcard) → implies ALL permissions (HOST role only)

**Example Scenario:**
- User has `tenant:read` for tenant "bulovas"
- User tries to use Internal Agent with "bulovas" → **403 Permission Denied**
- This is **expected behavior** - they need `chat:test` permission

**Preset Mapping:**
| Preset | Permissions | Internal Agent Access | Agent Admin Access |
|--------|-------------|----------------------|--------------------|
| Viewer | `tenant:read` | No | View only |
| Tester | `tenant:read`, `chat:test` | **Yes** | View only |
| Analyst | `tenant:read`, `chat:logs` | No | View + session logs |
| Editor | `tenant:read`, `tenant:write`, `files:upload` | No | View + edit + upload |
| Tenant Admin | `tenant:admin` | No | Full tenant config |

**To grant Internal Agent access**, use the **Tester** preset or manually add `chat:test` permission.

**To grant full access** (Admin + Chat + Files), grant multiple permissions:
```
["tenant:admin", "chat:test", "chat:logs", "files:upload", "files:restore", "api:config"]
```

### A.7 A.7 Completed Implementation (2026-01-19)

The following items have been implemented:

#### 1. Session Logs UI

**Files Changed:**
- `web/src/store/v2/agentAdmin.ts`: Added session-related methods to return object
- `web/src/pages/AgentAdmin.tsx`: Added `SessionLogsSection` component

**Features:**
- List of chat sessions with metadata (customer name, phase, intent, message count)
- Session detail modal with full conversation history
- Refresh button to reload sessions
- Color-coded session phases (completed=green, in_progress=yellow, etc.)
- Requires `chat:logs` permission or ADMIN role

**UI Location:** Agent Admin → Select Tenant → "Chat Session Logs" section

#### 2. Permission-based Navigation

**Files Changed:**
- `web/src/store/v2/agentAdmin.ts`: Added `hasAnyChatTestPermission()` method
- `web/src/components/Navigation.tsx`: Conditional rendering of Internal Agent menu

**Behavior:**
- Internal Agent menu item is only visible if:
  - User is HOST or ADMIN role, OR
  - User has `chat:test` permission on at least one tenant
- Non-admin users have their tenant permissions fetched on navigation mount
- Menu visibility updates reactively when permissions change

**Note:** `tenant:admin` does NOT grant `chat:test` (permissions are intentionally separate)

#### 3. Audit Logging for Permission Checks

**Files Changed:**
- `server/router/api/v1/agent/handlers.go`: Updated `hasPermission()` function

**Log Levels:**
- `DEBUG`: Successful permission grants (to avoid log noise)
- `INFO`: Permission denials (for security monitoring)
- `WARN`: Failed permission checks due to missing user ID or user not found

**Log Fields:**
| Field | Description |
|-------|-------------|
| `user_id` | The user's ID |
| `username` | The user's username |
| `tenant_id` | The tenant being accessed |
| `permission` | The permission being checked |
| `result` | "granted" or "denied" |
| `reason` | Why the result occurred |
| `user_permissions` | User's explicit permissions (if applicable) |

**Example Log Entries:**
```
# Granted via HOST role
level=DEBUG msg="permission check" user_id=1 username=admin tenant_id=5 permission=chat:test result=granted reason=host_role

# Granted via explicit permission
level=DEBUG msg="permission check" user_id=2 username=john tenant_id=5 permission=chat:test result=granted reason=explicit_permission user_permissions="tenant:read,chat:test"

# Denied - no explicit permission
level=INFO msg="permission check" user_id=3 username=jane tenant_id=5 permission=chat:test result=denied reason=no_explicit_permission

# Denied - permission not in list
level=INFO msg="permission check" user_id=2 username=john tenant_id=5 permission=files:upload result=denied reason=permission_not_in_list user_permissions="tenant:read,chat:test"
```

**Security Monitoring:**
To monitor for permission denials, filter logs:
```bash
# View all permission denials
grep 'result=denied' /var/log/memos.log

# View denials for a specific user
grep 'result=denied' /var/log/memos.log | grep 'username=jane'

# View denials for a specific tenant
grep 'result=denied' /var/log/memos.log | grep 'tenant_id=5'
```

### A.8 Summary of All Files Changed (A.7 Implementation)

| File | Type | Changes |
|------|------|---------|
| `agentAdmin.ts` | Frontend Store | Added session methods export, `hasAnyChatTestPermission()` |
| `AgentAdmin.tsx` | Frontend Page | Added `SessionLogsSection` component with modal |
| `Navigation.tsx` | Frontend Component | Conditional Internal Agent menu visibility |
| `handlers.go` | Backend | Audit logging in `hasPermission()` |

### A.10 Permission-based Navigation Fix (Implemented)

#### Issue: Permission-based Navigation - Create User Record Failed

**Problem:** When testing permission-based navigation with a newly created user, the system failed to properly evaluate permissions because:

1. The `fetchUserTenants()` call did not complete before the navigation rendered
2. Backend returned `{"tenants": null}` instead of `{"tenants": []}` for users with no permissions
3. The `hasAnyChatTestPermission()` method relies on `myTenantAccess` array being populated

**Root Cause:** Race condition between navigation mount and async permission fetch, plus null vs empty array handling.

**Fixes Applied:**

1. **Backend (`handlers.go:1634`):** Changed `var result []map[string]interface{}` to `result := make([]map[string]interface{}, 0)` to return empty array instead of null

2. **Navigation.tsx:** Added `permissionsLoaded` state to wait for permissions before checking access:
   ```typescript
   const [permissionsLoaded, setPermissionsLoaded] = useState(false);

   useEffect(() => {
     // ...
     if (!isAdmin) {
       agentAdminStore.fetchUserTenants().then(() => {
         setPermissionsLoaded(true);
       });
     } else {
       setPermissionsLoaded(true);
     }
   }, [currentUser, isAdmin]);

   const canAccessInternalAgent = isAdmin || (permissionsLoaded && agentAdminStore.hasAnyChatTestPermission());
   ```

3. **agentAdmin.ts:** Updated `fetchUserTenants()` to handle null responses and 404 errors gracefully:
   ```typescript
   const tenants = response.data.tenants ?? [];  // Handle null
   // ...
   if (error.response?.status === 404) {
     state.myTenantAccess = [];  // Treat 404 as "no permissions"
     state.tenants = [];
   }
   ```

**Files Changed:**
| File | Change |
|------|--------|
| `server/router/api/v1/agent/handlers.go` | Line 1634: Initialize result as empty slice |
| `web/src/components/Navigation.tsx` | Added `permissionsLoaded` state |
| `web/src/store/v2/agentAdmin.ts` | Handle null/404 in `fetchUserTenants()` |

**Status:** Fixed

---

### A.11 Consistency Audit - Completed Fixes (2026-01-19)

This section documents the fixes applied during a comprehensive audit of the RBAC implementation.

#### 1. Permission Check Added to HandleGetFileVersions ✓

**Problem:** Users with `files:restore` permission could restore versions but couldn't view version history.

**Fix Applied (`handlers.go`):**
```go
// HandleGetFileVersions now checks:
if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermFilesRestore) && !h.hasPermission(c, tenant.ID, PermTenantRead) {
    return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role, files:restore, or tenant:read permission")
}
```

#### 2. Legacy/Duplicate Endpoints

| Current Endpoint | Legacy Endpoint | Notes |
|------------------|-----------------|-------|
| `HandleImportSingleFile` | `HandleImport` | Both handle `/import`, Single has permission check |
| `HandleGetTenantFullConfig` | `HandleGetConfig` | Both handle `/config`, FullConfig has permission check |

**Status:** To be reviewed in future cleanup - legacy endpoints may be used by different routes.

#### 3. Error Messages Standardized ✓

All error messages now use consistent format:

| Type | Format |
|------|--------|
| Admin-only | `"Permission denied: requires admin role"` |
| Permission-based | `"Permission denied: requires admin role or {permission} permission"` |
| Multiple permissions | `"Permission denied: requires {perm1} or {perm2} permission"` |

**Locations Updated:** 19 error messages across `handlers.go`

#### 4. Documentation Updated ✓

Removed "(TODO)" markers from sections 4.2, 4.4, 4.5, 4.6, 4.7 - all enforcement is now implemented.

#### 5. Frontend Consistency ✓

All frontend checks verified as working:
- Agent Admin tenant list: Shows only accessible tenants
- Agent Admin conditional UI: Hides/shows based on permissions
- Navigation Internal Agent: Hidden for users without chat:test
- Session Logs section: Hidden for users without chat:logs
- Grant permissions modal: z-index fixed
- Empty tenant handling: Returns `[]` not `null`

#### 6. Audit Logging Added to isAdmin() ✓

**Fix Applied (`handlers.go`):**
```go
func (h *Handler) isAdmin(c echo.Context) bool {
    // ... user lookup ...
    isAdmin := user.Role == store.RoleHost || user.Role == store.RoleAdmin
    if isAdmin {
        slog.Debug("admin check", "user_id", userID, "username", user.Username, "role", user.Role, "result", "granted")
    } else {
        slog.Info("admin check", "user_id", userID, "username", user.Username, "role", user.Role, "result", "denied", "reason", "not_admin_role")
    }
    return isAdmin
}
```

#### 7. Summary of Completed Changes

| Priority | File | Change | Status |
|----------|------|--------|--------|
| **High** | `handlers.go` | Add permission check to `HandleGetFileVersions` | ✓ Done |
| **Medium** | `handlers.go` | Standardize error message format (19 locations) | ✓ Done |
| **Medium** | `DOCS_RBAC_2.MD` | Remove "(TODO)" from implemented sections | ✓ Done |
| **Low** | `handlers.go` | Verify and clean up legacy endpoints | Deferred |
| **Low** | `handlers.go` | Add audit logging to `isAdmin()` | ✓ Done |

---

### A.9 Testing the A.7 Changes

#### Test Session Logs UI

1. **Login as ADMIN or user with `chat:logs` permission**
2. **Navigate to Agent Admin → Select a tenant**
3. **Verify "Chat Session Logs" section is visible**
4. **If sessions exist:**
   - Should see list of sessions with customer name, phase, intent
   - Click on a session to view details
   - Verify messages are displayed chronologically
5. **Click "Refresh" to reload sessions**

#### Test Permission-based Navigation

1. **Create a regular user with NO permissions**
2. **Login as that user**
3. **Verify Internal Agent is NOT visible in left sidebar**
4. **Grant `chat:test` permission to user for any tenant**
5. **Refresh the page**
6. **Verify Internal Agent IS now visible in left sidebar**

#### Test Audit Logging

1. **Set log level to DEBUG:**
   ```bash
   export LOG_LEVEL=debug
   ```
2. **Start the server and attempt various actions**
3. **Check logs for permission check entries:**
   ```bash
   tail -f /var/log/memos.log | grep "permission check"
   ```
4. **Verify:**
   - Grants are logged at DEBUG level
   - Denials are logged at INFO level
   - All required fields are present
