# DeepWiki Q&A with Code Context for Repository: PithomLabs/bchat
## Q1
what are the security risks of this codebase
### Answer
The user wants to know about security risks in the PithomLabs/bchat codebase. Let me start by getting an overview of the codebase and then search for security-related concerns.

Let me dig deeper into specific security concerns - the external chat endpoint (unauthenticated), rate limiting, prompt injection, CORS, and any other potential issues.

Let me look at a few more specific security concerns - the message length limits, the playground endpoint (public), the transcript endpoint (public), and the RBAC limitations mentioned in the docs.

I have enough information to provide a comprehensive security risk analysis. Let me compile the findings from my research.

Key security risks I've identified:

1. **Wildcard CORS** - `AllowOrigins: []string{"*"}` in v1.go
2. **No message length limit on external chat** - The external chat endpoint doesn't validate message length
3. **Domain allowlist is optional** - By default, the widget can be embedded anywhere
4. **Public transcript endpoint** - `HandleGetExternalTranscript` is public
5. **Public playground endpoint** - `HandlePlaygroundRun` and `HandlePlaygroundCatalog` are public
6. **Never-expire tokens** - The auth service supports `never_expire` tokens
7. **Cross-tenant data access vulnerability** (documented as fixed in DOCS_BUGS_QA.MD)
8. **No granular permissions** - Binary admin/non-admin check (documented in DOCS_RBAC.MD as a limitation)
9. **Prompt injection** - No real input sanitization for user messages before sending to LLM
10. **LLM verification layer disabled by default** - The semantic verification layer is disabled by default
11. **Rate limiting only on external chat** - Internal chat has no rate limiting
12. **ENCRYPTION_MASTER_KEY** - If lost, all encrypted API keys are unrecoverable; if weak, all tenant API keys are compromised
13. **Tenant slug enumeration** - The external chat endpoint reveals whether a tenant exists or not
14. **gRPC internal connection uses insecure credentials** - `insecure.NewCredentials()` in v1.go

Let me look at a few more specific things to confirm.

Now I have enough information to provide a comprehensive security risk analysis. Let me synthesize all findings.


Here is a structured breakdown of the security risks found in the codebase, organized by severity.

---

## Critical / High

### 1. Wildcard CORS (`AllowOrigins: ["*"]`)
The global CORS middleware allows requests from any origin. bchat:140-144 

This means any website can make credentialed cross-origin requests to the API. The per-tenant domain allowlist (on the chat/widget endpoints) is a partial mitigation, but it does not cover the admin API endpoints.

---

### 2. No Message Length Limit on External Chat
The public `POST /api/v1/agent/:slug/chat/ext` endpoint only checks that `req.Message != ""` — there is no upper bound on message size. bchat:434-436 

An attacker can send arbitrarily large messages, inflating LLM token costs and potentially causing memory pressure. The `client_message_id` field has a 128-char cap, but the message body itself does not. bchat:1520-1522 

---

### 3. Prompt Injection (No Input Sanitization)
User messages are passed directly into the LLM system prompt context without sanitization. The only defenses are:
- Output sanitization (`sanitizer.go`) — strips hallucinated tags from the *response*, not the input
- An LLM-based verifier (`verifier.go`) — **disabled by default** bchat:73-77 

A malicious user can craft messages like `"Ignore all previous instructions and..."` to attempt to override the system prompt. There is no server-side input sanitization layer before the LLM call.

---

### 4. Never-Expiring Access Tokens
The `SignIn` proto supports a `never_expire` flag, and the auth service honors it. bchat:44-47 

Tokens that never expire cannot be invalidated except by explicit revocation. If a token is leaked (e.g., in logs, browser history, or a compromised client), it remains valid indefinitely.

---

### 5. gRPC Internal Connection Uses Insecure Credentials
The internal gRPC-Gateway connection (from the HTTP gateway to the gRPC server) uses `insecure.NewCredentials()`. bchat:90-93 

This is a loopback connection, so the risk is lower in practice, but it means traffic between the gateway and gRPC server is unencrypted and unauthenticated at the transport layer.

---

## Medium

### 6. Public Transcript Endpoint Leaks Conversation History
`GET /api/v1/agent/:slug/chat/ext/transcript` is in the public (no-auth) route group. bchat:187-192 

Anyone who knows a valid `session_id` can retrieve the full conversation transcript of an anonymous user. Session IDs are UUIDs, so guessing is hard, but the endpoint is unauthenticated by design.

---

### 7. Public Playground Endpoints Expose Tenant Data
`GET /api/v1/agent/playground/catalog` and `POST /api/v1/agent/:slug/playground/run` are public with no authentication. bchat:187-192 

The catalog endpoint enumerates demo tenant slugs, company names, and verticals. The `playground/run` endpoint can invoke the full chat pipeline against any tenant without authentication or rate limiting (it's unclear if rate limiting applies here).

---

### 8. Tenant Slug Enumeration
The external chat endpoint returns `404 "Agent not found"` for inactive/nonexistent tenants and `200` for active ones. bchat:406-409 

An attacker can enumerate all active tenant slugs by brute-forcing common names. Slugs are human-readable (e.g., `acme-restoration`), making this practical.

---

### 9. No Rate Limiting on Internal Chat
The internal chat endpoint (`/chat/int`) has no rate limiting — it is explicitly excluded. bchat:47-49 

A compromised or malicious authenticated user can flood the LLM API at full speed, incurring unbounded costs.

---

### 10. ENCRYPTION_MASTER_KEY Is a Single Point of Failure
All tenant API keys are encrypted with a single master key derived via Argon2id. bchat:25-34 

If `ENCRYPTION_MASTER_KEY` is weak, rotated incorrectly, or leaked, all tenant API keys are compromised simultaneously. The docs acknowledge that losing the key makes all encrypted keys unrecoverable. bchat:655-660 

---

## Low / Design Limitations

### 11. Previously Fixed: Cross-Tenant QA Pair Deletion
A documented and fixed vulnerability allowed an admin of Tenant B to delete QA pairs belonging to Tenant A by crafting a DELETE request with a foreign ID. bchat:150-162 

The fix adds `WHERE id = ? AND tenant_id = ?` compound checks. However, this pattern should be audited across all other resource types (sessions, files, etc.) to ensure the same class of bug doesn't exist elsewhere.

---

### 12. ADMIN Role Has No Tenant Isolation
Any `ADMIN`-role user can access **all** tenants. There is no binding between an admin user and specific tenants unless the granular RBAC permissions are explicitly configured. bchat:39-44 

This is documented as a known limitation, not a bug, but it means a compromised admin account exposes all tenant data.

---

### 13. Domain Allowlist Is Opt-In and Bypassable
The domain allowlist check relies on the `Origin` header, which is browser-enforced but trivially spoofed by non-browser clients (e.g., `curl`). bchat:411-417 

The allowlist provides protection against unauthorized *embedding* on third-party sites, but not against direct API abuse from scripts or bots.
### Citations
**File:** server/router/api/v1/v1.go (L90-93)
```go
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
```
**File:** server/router/api/v1/v1.go (L140-144)
```go
	echoServer.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
```
**File:** server/router/api/v1/v1.go (L187-192)
```go
	publicGroup := echoServer.Group("/api/v1/agent")
	publicGroup.GET("/playground/catalog", s.agentHandler.HandlePlaygroundCatalog)
	publicGroup.POST("/:slug/chat/ext", s.agentHandler.HandleChatExternal)
	publicGroup.GET("/:slug/chat/ext/transcript", s.agentHandler.HandleGetExternalTranscript)
	publicGroup.POST("/:slug/playground/run", s.agentHandler.HandlePlaygroundRun)
	publicGroup.GET("/:slug/widget.js", s.agentHandler.HandleWidget) // Legacy - inline JS
```
**File:** server/router/api/v1/agent/handlers.go (L406-409)
```go
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil || !tenant.IsActive {
		return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
	}
```
**File:** server/router/api/v1/agent/handlers.go (L411-417)
```go
	// Check domain allowlist if enabled
	if tenant.AllowedDomains != "" {
		origin := c.Request().Header.Get("Origin")
		if !h.isDomainAllowed(tenant.AllowedDomains, origin, "") {
			return echo.NewHTTPError(http.StatusForbidden, "Domain not allowed")
		}
	}
```
**File:** server/router/api/v1/agent/handlers.go (L434-436)
```go
	if req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Message is required")
	}
```
**File:** server/router/api/v1/agent/service.go (L1520-1522)
```go
	if len(req.ClientMessageID) > 128 {
		return nil, fmt.Errorf("client_message_id must be at most 128 characters")
	}
```
**File:** docs/DOCS_HALLUCINATION_GROUNDING.MD (L73-77)
```markdown
│ • Semantic verification against KB                               │
│ • Violation detection with severity levels                       │
│ • Auto-correction capability                                     │
│ • Status: DISABLED BY DEFAULT (enable with env var)             │
└─────────────────────────┬───────────────────────────────────────┘
```
**File:** proto/api/v1/auth_service.proto (L44-47)
```text
  }
  // Whether the session should never expire.
  bool never_expire = 3;
}
```
**File:** docs/DOCS_ENDPOINTS.MD (L47-49)
```markdown
| **RAG Pipeline** | Yes |
| **Rate Limiting** | No (internal users trusted) |
| **Transcript Recording** | Yes |
```
**File:** internal/crypto/encryption.go (L25-34)
```go
func NewEncryptionService(masterPassword string, salt []byte) *EncryptionService {
	key := argon2.IDKey(
		[]byte(masterPassword),
		salt,
		1,       // time
		64*1024, // memory (64 MB)
		4,       // parallelism
		KeySize,
	)
	return &EncryptionService{key: key}
```
**File:** docs/DOCS_RBAC_2.MD (L655-660)
```markdown
### 10.6 Lost Encryption Key Recovery

If you lose the `ENCRYPTION_MASTER_KEY`:

1. **Encrypted API keys cannot be recovered**
2. **To recover:**
```
**File:** docs/DOCS_BUGS_QA.MD (L150-162)
```markdown
## Security Implications

This was a **cross-tenant data access vulnerability**. While the attack required admin privileges, it still violated the principle of tenant isolation in a multi-tenant system.

### Attack Vector (Before Fix)
1. Admin authenticates to Agent Admin
2. Admin selects Tenant B
3. Admin crafts DELETE request: `DELETE /api/v1/agent/tenant-b/qa-pairs/123`
4. Where ID 123 belongs to Tenant A
5. **Result:** Tenant A's QA pair deleted (VULNERABILITY)

### After Fix
Same attack vector now returns error: "QA pair not found or not owned by tenant"
```
**File:** docs/DOCS_RBAC.MD (L39-44)
```markdown

1. **No granular permissions** - Binary admin/non-admin check
2. **No user-tenant binding** - Admins can access ALL tenants
3. **No tenant-level configuration** - API keys stored in environment variables
4. **No audit trail** - No logging of who did what
5. **Frontend relies on API rejection** - No route-level guards
```