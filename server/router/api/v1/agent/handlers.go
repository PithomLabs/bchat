package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/usememos/memos/store"
)

// Handler handles HTTP requests for the agent API.
type Handler struct {
	service *Service
	store   *store.Store
}

// NewHandler creates a new agent handler.
func NewHandler(service *Service, store *store.Store) *Handler {
	return &Handler{
		service: service,
		store:   store,
	}
}

// ============================================================================
// CHAT ENDPOINTS
// ============================================================================

// HandleValidateTenant validates that a tenant exists, is active, and user has access.
// GET /api/v1/agent/:slug/validate
// Requires: ADMIN role OR chat:test permission
func (h *Handler) HandleValidateTenant(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]interface{}{
			"valid":   false,
			"message": "Tenant not found",
		})
	}

	if !tenant.IsActive {
		return echo.NewHTTPError(http.StatusNotFound, map[string]interface{}{
			"valid":   false,
			"message": "Tenant is not active",
		})
	}

	// Check if internal audience is configured (for internal agent chat)
	internalType := "internal"
	audience, err := h.store.GetAgentAudience(ctx, &store.FindAgentAudience{
		TenantID:     &tenant.ID,
		AudienceType: &internalType,
	})
	if err != nil || audience == nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]interface{}{
			"valid":   false,
			"message": "Internal agent not configured for this tenant. Please upload internal KB and Policy files in Agent Admin.",
		})
	}

	// Check if user has permission to use internal chat (ADMIN or chat:test)
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
		return echo.NewHTTPError(http.StatusForbidden, map[string]interface{}{
			"valid":   false,
			"message": "Permission denied: you do not have access to this tenant's internal chat. Contact an admin to grant you chat:test permission.",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid":       true,
		"companyName": tenant.CompanyName,
	})
}

// HandleChatExternal handles external (anonymous) chat requests.
// POST /api/v1/agent/:slug/chat/ext
func (h *Handler) HandleChatExternal(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get client IP for rate limiting
	clientIP := c.RealIP()
	if clientIP == "" {
		clientIP = c.Request().RemoteAddr
	}

	// Bind request
	var req ChatRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	if req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Message is required")
	}

	// Process chat
	response, err := h.service.ChatExternal(ctx, slug, clientIP, req)
	if err != nil {
		if strings.Contains(err.Error(), "rate limit") {
			return echo.NewHTTPError(http.StatusTooManyRequests, map[string]interface{}{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests. Please try again in 60 seconds.",
				"retry_after": 60,
			})
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not active") {
			return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
		}
		slog.Error("chat external failed", "slug", slug, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Chat service unavailable")
	}

	return c.JSON(http.StatusOK, response)
}

// HandleChatInternal handles internal (authenticated) chat requests.
// POST /api/v1/agent/:slug/chat/int
// Requires: ADMIN role OR chat:test permission
func (h *Handler) HandleChatInternal(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get user ID from context (set by auth middleware)
	userID, ok := c.Get("user-id").(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
	}

	// Get tenant for permission check
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
	}

	// Check admin role OR chat:test permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:test permission")
	}

	// Bind request
	var req ChatRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	if req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Message is required")
	}

	// Process chat
	response, err := h.service.ChatInternal(ctx, slug, userID, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not active") {
			return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
		}
		slog.Error("chat internal failed", "slug", slug, "userID", userID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Chat service unavailable")
	}

	return c.JSON(http.StatusOK, response)
}

// ============================================================================
// ADMIN ENDPOINTS
// ============================================================================

// HandleListTenants returns all tenants.
// GET /api/v1/agent/tenants
func (h *Handler) HandleListTenants(c echo.Context) error {
	ctx := c.Request().Context()

	// Check admin role
	if !h.isAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role")
	}

	tenants, err := h.store.ListAgentTenants(ctx, &store.FindAgentTenant{})
	if err != nil {
		slog.Error("list tenants failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list tenants")
	}

	// Convert to response format
	result := make([]map[string]interface{}, len(tenants))
	for i, t := range tenants {
		result[i] = map[string]interface{}{
			"id":          t.ID,
			"slug":        t.Slug,
			"companyName": t.CompanyName,
			"vertical":    t.Vertical,
			"isActive":    t.IsActive,
			"createdAt":   t.CreatedAt,
			"updatedAt":   t.UpdatedAt,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"tenants": result,
	})
}

// HandleGetTenantFullConfig returns full tenant config including both audiences.
// GET /api/v1/agent/:slug/config (updated to return full config)
// Requires: ADMIN role OR tenant:read permission
func (h *Handler) HandleGetTenantFullConfig(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant first (needed for permission check)
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check admin role OR tenant:read permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantRead) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or tenant:read permission")
	}

	// Get stats for both audiences
	getAudienceStats := func(audienceType string) map[string]interface{} {
		services, _ := h.store.ListAgentServices(ctx, &store.FindAgentService{TenantID: &tenant.ID, AudienceType: &audienceType})
		intents, _ := h.store.ListAgentIntents(ctx, &store.FindAgentIntent{TenantID: &tenant.ID, AudienceType: &audienceType})
		faqs, _ := h.store.ListAgentFAQs(ctx, &store.FindAgentFAQ{TenantID: &tenant.ID, AudienceType: &audienceType})
		files, _ := h.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{TenantID: &tenant.ID, AudienceType: &audienceType})

		fileList := make([]map[string]interface{}, len(files))
		for i, f := range files {
			fileList[i] = map[string]interface{}{
				"id":          f.ID,
				"fileType":    f.FileType,
				"contentHash": f.ContentHash,
				"importedAt":  f.ImportedAt,
			}
		}

		return map[string]interface{}{
			"stats": map[string]int{
				"servicesCount": len(services),
				"intentsCount":  len(intents),
				"faqsCount":     len(faqs),
			},
			"sourceFiles": fileList,
		}
	}

	response := map[string]interface{}{
		"tenant": map[string]interface{}{
			"id":          tenant.ID,
			"slug":        tenant.Slug,
			"companyName": tenant.CompanyName,
			"vertical":    tenant.Vertical,
			"isActive":    tenant.IsActive,
			"createdAt":   tenant.CreatedAt,
			"updatedAt":   tenant.UpdatedAt,
		},
		"external": getAudienceStats("external"),
		"internal": getAudienceStats("internal"),
		"endpoints": map[string]string{
			"externalChat": "/api/v1/agent/" + tenant.Slug + "/chat/ext",
			"internalChat": "/api/v1/agent/" + tenant.Slug + "/chat/int",
			"widget":       "/api/v1/agent/" + tenant.Slug + "/widget.js",
		},
	}

	return c.JSON(http.StatusOK, response)
}

// HandleUpdateTenant updates tenant properties.
// PATCH /api/v1/agent/:slug
// Requires: ADMIN role OR tenant:write permission
func (h *Handler) HandleUpdateTenant(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant first (needed for permission check)
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check admin role OR tenant:write permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantWrite) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or tenant:write permission")
	}

	// Bind update request
	var req struct {
		IsActive    *bool   `json:"is_active"`
		CompanyName *string `json:"company_name"`
		Vertical    *string `json:"vertical"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Apply updates
	if req.IsActive != nil {
		tenant.IsActive = *req.IsActive
	}
	if req.CompanyName != nil {
		tenant.CompanyName = *req.CompanyName
	}
	if req.Vertical != nil {
		tenant.Vertical = *req.Vertical
	}

	// Save
	tenant, err = h.store.UpdateAgentTenant(ctx, tenant)
	if err != nil {
		slog.Error("update tenant failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update tenant")
	}

	// Invalidate cache
	h.service.configCache.Invalidate(tenant.Slug)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"tenant": map[string]interface{}{
			"id":          tenant.ID,
			"slug":        tenant.Slug,
			"companyName": tenant.CompanyName,
			"vertical":    tenant.Vertical,
			"isActive":    tenant.IsActive,
		},
	})
}

// HandleGetFileVersions returns version history for a file.
// GET /api/v1/agent/:slug/files/:audienceType/:fileType/versions
// Requires: ADMIN role OR files:restore OR tenant:read permission
func (h *Handler) HandleGetFileVersions(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	audienceType := c.Param("audienceType")
	fileType := c.Param("fileType")

	// Validate params
	if audienceType != "external" && audienceType != "internal" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid audience type")
	}
	if fileType != "kb" && fileType != "policy" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid file type")
	}

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check admin role OR files:restore OR tenant:read permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermFilesRestore) && !h.hasPermission(c, tenant.ID, PermTenantRead) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role, files:restore, or tenant:read permission")
	}

	// Get file versions
	files, err := h.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
		TenantID:     &tenant.ID,
		AudienceType: &audienceType,
		FileType:     &fileType,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get file versions")
	}

	versions := make([]map[string]interface{}, len(files))
	for i, f := range files {
		versions[i] = map[string]interface{}{
			"id":          f.ID,
			"contentHash": f.ContentHash,
			"importedAt":  f.ImportedAt,
			"version":     len(files) - i, // Latest first
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"versions": versions,
	})
}

// HandleRestoreFileVersion restores a previous file version.
// POST /api/v1/agent/:slug/files/:audienceType/:fileType/restore
// Requires: ADMIN role OR files:restore permission
func (h *Handler) HandleRestoreFileVersion(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	audienceType := c.Param("audienceType")
	fileType := c.Param("fileType")

	// Get tenant first (needed for permission check)
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check admin role OR files:restore permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermFilesRestore) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or files:restore permission")
	}

	// Bind request
	var req struct {
		VersionID int32 `json:"version_id"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Get the version to restore
	file, err := h.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{ID: &req.VersionID})
	if err != nil || file == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Version not found")
	}

	// Verify it belongs to this tenant/audience/fileType
	if file.TenantID != tenant.ID || file.AudienceType != audienceType || file.FileType != fileType {
		return echo.NewHTTPError(http.StatusBadRequest, "Version does not match tenant/audience/file type")
	}

	// Re-import the content (this creates a new version with the old content)
	if fileType == "kb" {
		// Need to also get the policy file to re-import
		policyFile, _ := h.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
			TenantID:     &tenant.ID,
			AudienceType: &audienceType,
			FileType:     stringPtr("policy"),
		})
		policyContent := ""
		if policyFile != nil {
			policyContent = policyFile.Content
		}
		_, err = h.importFiles(ctx, tenant.ID, audienceType, file.Content, policyContent)
	} else {
		// Need to also get the KB file to re-import
		kbFile, _ := h.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
			TenantID:     &tenant.ID,
			AudienceType: &audienceType,
			FileType:     stringPtr("kb"),
		})
		kbContent := ""
		if kbFile != nil {
			kbContent = kbFile.Content
		}
		_, err = h.importFiles(ctx, tenant.ID, audienceType, kbContent, file.Content)
	}

	if err != nil {
		slog.Error("restore version failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to restore version")
	}

	// Invalidate cache
	h.service.configCache.Invalidate(tenant.Slug)

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

// HandleImportSingleFile handles importing a single file for a tenant.
// POST /api/v1/agent/:slug/import (updated to support single file)
// Requires: ADMIN role OR files:upload permission
func (h *Handler) HandleImportSingleFile(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant first (needed for permission check)
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check admin role OR files:upload permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermFilesUpload) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or files:upload permission")
	}

	audienceType := c.FormValue("audience_type")
	fileType := c.FormValue("file_type")

	if audienceType != "external" && audienceType != "internal" {
		return echo.NewHTTPError(http.StatusBadRequest, "audience_type must be 'external' or 'internal'")
	}
	if fileType != "kb" && fileType != "policy" {
		return echo.NewHTTPError(http.StatusBadRequest, "file_type must be 'kb' or 'policy'")
	}

	// Read uploaded file
	file, err := c.FormFile("file")
	if err != nil || file == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file is required")
	}

	src, _ := file.Open()
	content, _ := io.ReadAll(src)
	src.Close()

	// Get the other file (we need both to re-import)
	otherFileType := "policy"
	if fileType == "policy" {
		otherFileType = "kb"
	}

	otherFile, _ := h.store.GetAgentSourceFile(ctx, &store.FindAgentSourceFile{
		TenantID:     &tenant.ID,
		AudienceType: &audienceType,
		FileType:     &otherFileType,
	})

	otherContent := ""
	if otherFile != nil {
		otherContent = otherFile.Content
	}

	// Import
	var audienceInfo *AudienceInfo
	if fileType == "kb" {
		audienceInfo, err = h.importFiles(ctx, tenant.ID, audienceType, string(content), otherContent)
	} else {
		audienceInfo, err = h.importFiles(ctx, tenant.ID, audienceType, otherContent, string(content))
	}

	if err != nil {
		slog.Error("import failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Import failed: "+err.Error())
	}

	// Invalidate cache
	h.service.configCache.Invalidate(tenant.Slug)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"audience": audienceInfo,
	})
}

func stringPtr(s string) *string {
	return &s
}

// OnboardRequest represents the onboarding request.
type OnboardRequest struct {
	TenantSlug  string `form:"tenant_slug" json:"tenant_slug"`
	CompanyName string `form:"company_name" json:"company_name"`
	Vertical    string `form:"vertical" json:"vertical"`
}

// OnboardResponse represents the onboarding response.
type OnboardResponse struct {
	Success   bool                     `json:"success"`
	Tenant    *TenantInfo              `json:"tenant"`
	Audiences map[string]*AudienceInfo `json:"audiences"`
	Endpoints map[string]string        `json:"endpoints"`
}

// TenantInfo contains basic tenant information.
type TenantInfo struct {
	ID          int32  `json:"id"`
	Slug        string `json:"slug"`
	CompanyName string `json:"company_name"`
}

// AudienceInfo contains audience import statistics.
type AudienceInfo struct {
	ServicesCount int `json:"services_count"`
	IntentsCount  int `json:"intents_count"`
	FAQsCount     int `json:"faqs_count"`
	RulesCount    int `json:"rules_count"`
}

// HandleOnboard handles tenant onboarding.
// POST /api/v1/agent/onboard
func (h *Handler) HandleOnboard(c echo.Context) error {
	ctx := c.Request().Context()

	// Check admin role
	if !h.isAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role")
	}

	// Get form values
	tenantSlug := c.FormValue("tenant_slug")
	companyName := c.FormValue("company_name")
	vertical := c.FormValue("vertical")

	if tenantSlug == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant_slug is required")
	}

	// Create tenant
	tenant := &store.AgentTenant{
		Slug:        tenantSlug,
		CompanyName: companyName,
		Vertical:    vertical,
		IsActive:    true,
	}

	tenant, err := h.store.CreateAgentTenant(ctx, tenant)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return echo.NewHTTPError(http.StatusConflict, "Tenant already exists")
		}
		slog.Error("create tenant failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create tenant")
	}

	response := &OnboardResponse{
		Success: true,
		Tenant: &TenantInfo{
			ID:          tenant.ID,
			Slug:        tenant.Slug,
			CompanyName: tenant.CompanyName,
		},
		Audiences: make(map[string]*AudienceInfo),
		Endpoints: map[string]string{
			"external_chat": "/api/v1/agent/" + tenant.Slug + "/chat/ext",
			"internal_chat": "/api/v1/agent/" + tenant.Slug + "/chat/int",
			"widget_script": "/api/v1/agent/" + tenant.Slug + "/widget.js",
		},
	}

	// Process files for each audience
	for _, audienceType := range []string{"external", "internal"} {
		kbFile, err := c.FormFile(audienceType + "_kb_file")
		policyFile, err2 := c.FormFile(audienceType + "_policy_file")

		if err != nil || err2 != nil || kbFile == nil || policyFile == nil {
			continue // Skip if files not provided
		}

		// Read KB file
		kbSrc, err := kbFile.Open()
		if err != nil {
			continue
		}
		kbContent, _ := io.ReadAll(kbSrc)
		kbSrc.Close()

		// Read Policy file
		policySrc, err := policyFile.Open()
		if err != nil {
			continue
		}
		policyContent, _ := io.ReadAll(policySrc)
		policySrc.Close()

		// Import files
		audienceInfo, err := h.importFiles(ctx, tenant.ID, audienceType, string(kbContent), string(policyContent))
		if err != nil {
			slog.Error("import files failed", "tenant", tenant.Slug, "audience", audienceType, "error", err)
			continue
		}

		response.Audiences[audienceType] = audienceInfo

		// Update company name from KB if not provided
		if tenant.CompanyName == "" {
			parsedKB, _ := h.service.parser.ParseKB(string(kbContent), tenant.ID, audienceType)
			if parsedKB != nil && parsedKB.CompanyName != "" {
				tenant.CompanyName = parsedKB.CompanyName
				h.store.UpdateAgentTenant(ctx, tenant)
				response.Tenant.CompanyName = tenant.CompanyName
			}
		}
	}

	return c.JSON(http.StatusOK, response)
}

// HandleImport handles re-importing files for a tenant.
// POST /api/v1/agent/:slug/import
func (h *Handler) HandleImport(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Check admin role
	if !h.isAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role")
	}

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	audienceType := c.FormValue("audience_type")
	if audienceType != "external" && audienceType != "internal" {
		return echo.NewHTTPError(http.StatusBadRequest, "audience_type must be 'external' or 'internal'")
	}

	// Read files
	kbFile, err := c.FormFile("kb_file")
	if err != nil || kbFile == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "kb_file is required")
	}
	policyFile, err := c.FormFile("policy_file")
	if err != nil || policyFile == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "policy_file is required")
	}

	kbSrc, _ := kbFile.Open()
	kbContent, _ := io.ReadAll(kbSrc)
	kbSrc.Close()

	policySrc, _ := policyFile.Open()
	policyContent, _ := io.ReadAll(policySrc)
	policySrc.Close()

	// Import
	audienceInfo, err := h.importFiles(ctx, tenant.ID, audienceType, string(kbContent), string(policyContent))
	if err != nil {
		slog.Error("import failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Import failed: "+err.Error())
	}

	// Invalidate cache
	h.service.configCache.Invalidate(tenant.Slug)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"audience": audienceInfo,
	})
}

// importFiles imports KB and Policy files for a tenant/audience.
func (h *Handler) importFiles(ctx context.Context, tenantID int32, audienceType, kbContent, policyContent string) (*AudienceInfo, error) {
	parser := h.service.parser

	// Parse KB
	parsedKB, err := parser.ParseKB(kbContent, tenantID, audienceType)
	if err != nil {
		return nil, err
	}

	// Parse Policy
	parsedPolicy, err := parser.ParsePolicy(policyContent, tenantID, audienceType)
	if err != nil {
		return nil, err
	}

	// Clear existing data for this audience
	h.store.DeleteAgentServices(ctx, tenantID, audienceType)
	h.store.DeleteAgentExclusions(ctx, tenantID, audienceType)
	h.store.DeleteAgentFAQs(ctx, tenantID, audienceType)
	h.store.DeleteAgentSafetyProtocols(ctx, tenantID, audienceType)
	h.store.DeleteAgentKBSections(ctx, tenantID, audienceType)
	h.store.DeleteAgentIntents(ctx, tenantID, &audienceType)
	h.store.DeleteAgentRules(ctx, tenantID, audienceType)
	h.store.DeleteAgentAudience(ctx, tenantID, audienceType)

	// Insert services
	for _, s := range parsedKB.Services {
		h.store.CreateAgentService(ctx, s)
	}

	// Insert exclusions
	for _, e := range parsedKB.Exclusions {
		h.store.CreateAgentExclusion(ctx, e)
	}

	// Insert coverage (shared, so only insert if not exists)
	if audienceType == "external" {
		h.store.DeleteAgentCoverage(ctx, tenantID)
		for _, c := range parsedKB.Coverage {
			h.store.CreateAgentCoverage(ctx, c)
		}
	}

	// Insert FAQs
	for _, f := range parsedKB.FAQs {
		h.store.CreateAgentFAQ(ctx, f)
	}

	// Insert safety protocols
	for _, s := range parsedKB.Safety {
		h.store.CreateAgentSafetyProtocol(ctx, s)
	}

	// Insert KB sections
	for _, s := range parsedKB.Sections {
		h.store.CreateAgentKBSection(ctx, s)
	}

	// Insert intents
	for _, i := range parsedPolicy.Intents {
		h.store.CreateAgentIntent(ctx, i)
	}

	// Insert rules
	for _, r := range parsedPolicy.Rules {
		h.store.CreateAgentRule(ctx, r)
	}

	// Create audience config
	// FIX: Extract emergency phone from KB content instead of using placeholder
	// This prevents the situation where DB has (555) 000-0000 but KB.MD has real phone
	emergencyPhoneFromKB := ExtractPhoneFromKB(kbContent)
	if emergencyPhoneFromKB != "" {
		slog.Info("extracted emergency phone from KB", "phone", emergencyPhoneFromKB)
	}

	if parsedPolicy.Audience != nil {
		audience := parsedPolicy.Audience
		// Prioritize phone from KB, then policy, then leave empty (no placeholder!)
		if audience.EmergencyPhone == "" || !IsValidReplacementPhone(audience.EmergencyPhone) {
			if emergencyPhoneFromKB != "" {
				audience.EmergencyPhone = emergencyPhoneFromKB
			}
			// If still no valid phone, leave empty - do NOT set placeholder
			// The sanitizer will handle missing phones by using [phone number] placeholder
		}
		h.store.CreateAgentAudience(ctx, audience)
	} else {
		// Create default audience config
		// Use extracted phone from KB, or leave empty (no placeholder!)
		h.store.CreateAgentAudience(ctx, &store.AgentAudience{
			TenantID:                      tenantID,
			AudienceType:                  audienceType,
			Role:                          parsedPolicy.Identity.Role,
			Tone:                          parsedPolicy.Identity.Tone,
			BrandVoice:                    parsedPolicy.Identity.BrandVoice,
			Guidelines:                    parsedPolicy.Identity.Guidelines,
			EmergencyPhone:                emergencyPhoneFromKB, // Use KB phone or empty, never placeholder
			EmergencyUrgencyThreshold:     4,
			EscalationConfidenceThreshold: 0.85,
			RateLimitRPM:                  60,
		})
	}

	// Store source files
	if kbContent != "" {
		if _, err := h.store.UpsertAgentSourceFile(ctx, &store.AgentSourceFile{
			TenantID:     tenantID,
			AudienceType: audienceType,
			FileType:     "kb",
			Content:      kbContent,
			ContentHash:  ContentHash(kbContent),
		}); err != nil {
			slog.Error("failed to save KB source file", "error", err)
			return nil, fmt.Errorf("failed to save KB file: %w", err)
		}
	}
	if policyContent != "" {
		if _, err := h.store.UpsertAgentSourceFile(ctx, &store.AgentSourceFile{
			TenantID:     tenantID,
			AudienceType: audienceType,
			FileType:     "policy",
			Content:      policyContent,
			ContentHash:  ContentHash(policyContent),
		}); err != nil {
			slog.Error("failed to save Policy source file", "error", err)
			return nil, fmt.Errorf("failed to save Policy file: %w", err)
		}
	}

	// Index content for RAG pipeline
	if err := h.indexContentForRAG(ctx, tenantID, audienceType, parsedKB, parsedPolicy); err != nil {
		slog.Warn("Failed to index content for RAG", "error", err, "tenantID", tenantID, "audience", audienceType)
		// Don't fail the import if indexing fails - RAG is an enhancement
	}

	return &AudienceInfo{
		ServicesCount: len(parsedKB.Services),
		IntentsCount:  len(parsedPolicy.Intents),
		FAQsCount:     len(parsedKB.FAQs),
		RulesCount:    len(parsedPolicy.Rules),
	}, nil
}

// indexContentForRAG indexes parsed KB and Policy content into the vector database.
func (h *Handler) indexContentForRAG(ctx context.Context, tenantID int32, audienceType string, kb *ParsedKB, policy *ParsedPolicy) error {
	chunker := h.service.chunker
	vectorDB := h.service.vectorDB

	if chunker == nil || vectorDB == nil {
		return nil // RAG not enabled
	}

	// Delete existing chunks for this tenant/audience
	if err := vectorDB.Delete(ctx, tenantID, audienceType); err != nil {
		return fmt.Errorf("failed to delete existing chunks: %w", err)
	}

	// Chunk KB content
	var allChunks []DocumentChunk
	if kb != nil {
		kbChunks := chunker.ChunkKBContent(kb, tenantID, audienceType, 1)
		allChunks = append(allChunks, kbChunks...)
	}

	// Chunk Policy content
	if policy != nil {
		policyChunks := chunker.ChunkPolicyContent(policy, tenantID, audienceType, 1)
		allChunks = append(allChunks, policyChunks...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	// Insert chunks (embeddings will be generated automatically)
	if err := vectorDB.Insert(ctx, allChunks); err != nil {
		return fmt.Errorf("failed to insert chunks: %w", err)
	}

	slog.Info("Indexed content for RAG",
		"tenantID", tenantID,
		"audience", audienceType,
		"chunks", len(allChunks))

	return nil
}

// HandleExport exports the configuration to files.
// GET /api/v1/agent/:slug/export
func (h *Handler) HandleExport(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Check admin role
	if !h.isAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role")
	}

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	format := c.QueryParam("format")
	if format == "" {
		format = "json"
	}

	audienceType := c.QueryParam("audience")
	if audienceType == "" {
		audienceType = "external"
	}

	// Load source files
	files, _ := h.store.ListAgentSourceFiles(ctx, &store.FindAgentSourceFile{
		TenantID:     &tenant.ID,
		AudienceType: &audienceType,
	})

	result := make(map[string]interface{})
	for _, f := range files {
		result[f.FileType] = map[string]string{
			"content":     f.Content,
			"content_hash": f.ContentHash,
		}
	}

	result["tenant"] = map[string]interface{}{
		"slug":         tenant.Slug,
		"company_name": tenant.CompanyName,
		"vertical":     tenant.Vertical,
	}

	return c.JSON(http.StatusOK, result)
}

// HandleGetConfig gets the current configuration.
// GET /api/v1/agent/:slug/config
func (h *Handler) HandleGetConfig(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Check admin role
	if !h.isAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role")
	}

	audienceType := c.QueryParam("audience")
	if audienceType == "" {
		audienceType = "external"
	}

	config, err := h.service.LoadConfig(ctx, slug, audienceType)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return c.JSON(http.StatusOK, config)
}

// HandleDeleteTenant deletes a tenant.
// DELETE /api/v1/agent/:slug
func (h *Handler) HandleDeleteTenant(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Check admin role
	if !h.isAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role")
	}

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Delete tenant (cascades to all related data)
	if err := h.store.DeleteAgentTenant(ctx, tenant.ID); err != nil {
		slog.Error("delete tenant failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete tenant")
	}

	// Invalidate cache
	h.service.configCache.Invalidate(tenant.Slug)

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

// ============================================================================
// WIDGET ENDPOINT
// ============================================================================

// HandleWidget serves the embeddable widget script.
// GET /api/v1/agent/:slug/widget.js
func (h *Handler) HandleWidget(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil || !tenant.IsActive {
		return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
	}

	// Get base URL from request
	scheme := "https"
	if c.Request().TLS == nil {
		scheme = "http"
	}
	baseURL := scheme + "://" + c.Request().Host

	// Generate widget script
	script := generateWidgetScript(baseURL, slug, tenant.CompanyName)

	c.Response().Header().Set("Content-Type", "application/javascript")
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.String(http.StatusOK, script)
}

// generateWidgetScript generates the embeddable widget JavaScript.
func generateWidgetScript(baseURL, tenantSlug, companyName string) string {
	return `(function() {
  'use strict';

  // Configuration
  var config = {
    baseURL: '` + baseURL + `',
    tenantSlug: '` + tenantSlug + `',
    companyName: '` + companyName + `',
    primaryColor: '#0d9488'
  };

  // Create widget container
  var container = document.createElement('div');
  container.id = 'agent-chat-widget';
  container.innerHTML = createWidgetHTML();
  document.body.appendChild(container);

  // Add styles
  var styles = document.createElement('style');
  styles.textContent = getWidgetStyles();
  document.head.appendChild(styles);

  // State
  var state = {
    isOpen: false,
    isMinimized: false,
    messages: [],
    sessionId: null,
    isLoading: false
  };

  // Elements
  var btn = document.getElementById('acw-toggle');
  var panel = document.getElementById('acw-panel');
  var messagesEl = document.getElementById('acw-messages');
  var inputEl = document.getElementById('acw-input');
  var sendBtn = document.getElementById('acw-send');
  var closeBtn = document.getElementById('acw-close');
  var minBtn = document.getElementById('acw-minimize');

  // Event listeners
  btn.addEventListener('click', function() {
    state.isOpen = !state.isOpen;
    updateUI();
  });

  closeBtn.addEventListener('click', function() {
    state.isOpen = false;
    updateUI();
  });

  minBtn.addEventListener('click', function() {
    state.isMinimized = !state.isMinimized;
    updateUI();
  });

  sendBtn.addEventListener('click', sendMessage);
  inputEl.addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });

  function sendMessage() {
    var message = inputEl.value.trim();
    if (!message || state.isLoading) return;

    inputEl.value = '';
    state.isLoading = true;

    // Add user message
    addMessage('user', message);

    // Send to API
    fetch(config.baseURL + '/api/v1/agent/' + config.tenantSlug + '/chat/ext', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: state.sessionId,
        message: message
      })
    })
    .then(function(res) {
      if (res.status === 429) {
        addMessage('assistant', 'Too many messages. Please wait a moment.');
        return null;
      }
      return res.json();
    })
    .then(function(data) {
      if (data) {
        state.sessionId = data.session_id;
        addMessage('assistant', data.message.content);
      }
    })
    .catch(function(err) {
      addMessage('assistant', 'Sorry, something went wrong. Please try again.');
    })
    .finally(function() {
      state.isLoading = false;
      updateUI();
    });
  }

  function addMessage(role, content) {
    state.messages.push({ role: role, content: content, timestamp: new Date() });
    renderMessages();
  }

  function renderMessages() {
    messagesEl.innerHTML = state.messages.map(function(msg) {
      var cls = msg.role === 'user' ? 'acw-msg-user' : 'acw-msg-assistant';
      return '<div class="acw-msg ' + cls + '">' + escapeHtml(msg.content) + '</div>';
    }).join('');

    if (state.isLoading) {
      messagesEl.innerHTML += '<div class="acw-msg acw-msg-assistant acw-typing">Typing...</div>';
    }

    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  function updateUI() {
    btn.style.display = state.isOpen ? 'none' : 'flex';
    panel.style.display = state.isOpen ? 'flex' : 'none';

    var content = document.getElementById('acw-content');
    if (content) {
      content.style.display = state.isMinimized ? 'none' : 'flex';
    }
  }

  function escapeHtml(text) {
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  function createWidgetHTML() {
    return '\
      <button id="acw-toggle" aria-label="Open chat">\
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">\
          <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path>\
        </svg>\
      </button>\
      <div id="acw-panel">\
        <div id="acw-header">\
          <span>Chat with us</span>\
          <div>\
            <button id="acw-minimize">−</button>\
            <button id="acw-close">×</button>\
          </div>\
        </div>\
        <div id="acw-content">\
          <div id="acw-messages">\
            <div class="acw-welcome">How can we help you today?</div>\
          </div>\
          <div id="acw-input-area">\
            <input type="text" id="acw-input" placeholder="Type your message...">\
            <button id="acw-send">\
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">\
                <line x1="22" y1="2" x2="11" y2="13"></line>\
                <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>\
              </svg>\
            </button>\
          </div>\
        </div>\
      </div>\
    ';
  }

  function getWidgetStyles() {
    return '\
      #agent-chat-widget { font-family: system-ui, -apple-system, sans-serif; font-size: 14px; }\
      #acw-toggle { position: fixed; bottom: 20px; right: 20px; width: 56px; height: 56px; border-radius: 50%; background: ' + config.primaryColor + '; color: white; border: none; cursor: pointer; display: flex; align-items: center; justify-content: center; box-shadow: 0 4px 12px rgba(0,0,0,0.15); z-index: 9999; transition: transform 0.2s; }\
      #acw-toggle:hover { transform: scale(1.1); }\
      #acw-panel { position: fixed; bottom: 20px; right: 20px; width: 350px; height: 500px; background: white; border-radius: 12px; box-shadow: 0 8px 32px rgba(0,0,0,0.2); display: none; flex-direction: column; z-index: 9999; overflow: hidden; }\
      #acw-header { background: ' + config.primaryColor + '; color: white; padding: 12px 16px; display: flex; justify-content: space-between; align-items: center; }\
      #acw-header button { background: none; border: none; color: white; font-size: 18px; cursor: pointer; padding: 4px 8px; opacity: 0.8; }\
      #acw-header button:hover { opacity: 1; }\
      #acw-content { flex: 1; display: flex; flex-direction: column; }\
      #acw-messages { flex: 1; overflow-y: auto; padding: 16px; background: #f9fafb; }\
      .acw-welcome { text-align: center; color: #6b7280; padding: 32px 16px; }\
      .acw-msg { max-width: 80%; padding: 8px 12px; margin: 8px 0; border-radius: 12px; word-wrap: break-word; white-space: pre-wrap; }\
      .acw-msg-user { background: ' + config.primaryColor + '; color: white; margin-left: auto; }\
      .acw-msg-assistant { background: white; border: 1px solid #e5e7eb; }\
      .acw-typing { color: #9ca3af; font-style: italic; }\
      #acw-input-area { display: flex; padding: 12px; border-top: 1px solid #e5e7eb; gap: 8px; }\
      #acw-input { flex: 1; border: 1px solid #d1d5db; border-radius: 8px; padding: 8px 12px; outline: none; }\
      #acw-input:focus { border-color: ' + config.primaryColor + '; }\
      #acw-send { background: ' + config.primaryColor + '; color: white; border: none; border-radius: 8px; width: 36px; height: 36px; cursor: pointer; display: flex; align-items: center; justify-content: center; }\
      #acw-send:hover { opacity: 0.9; }\
      @media (max-width: 480px) {\
        #acw-panel { width: calc(100% - 20px); right: 10px; bottom: 10px; height: 60vh; }\
      }\
    ';
  }

  // Initialize
  updateUI();
})();`
}

// ============================================================================
// HELPERS
// ============================================================================

// isAdmin checks if the current user has admin role.
// Includes audit logging for security monitoring.
func (h *Handler) isAdmin(c echo.Context) bool {
	userID, ok := c.Get("user-id").(int32)
	if !ok {
		slog.Warn("admin check failed: no user ID in context",
			"result", "denied",
			"reason", "no_user_id",
		)
		return false
	}

	user, err := h.store.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		slog.Warn("admin check failed: user not found",
			"user_id", userID,
			"result", "denied",
			"reason", "user_not_found",
		)
		return false
	}

	isAdmin := user.Role == store.RoleHost || user.Role == store.RoleAdmin
	if isAdmin {
		slog.Debug("admin check",
			"user_id", userID,
			"username", user.Username,
			"role", user.Role,
			"result", "granted",
		)
	} else {
		slog.Info("admin check",
			"user_id", userID,
			"username", user.Username,
			"role", user.Role,
			"result", "denied",
			"reason", "not_admin_role",
		)
	}

	return isAdmin
}

// getUserID returns the current user's ID from the context, or 0 if not available.
func (h *Handler) getUserID(c echo.Context) int32 {
	userID, ok := c.Get("user-id").(int32)
	if !ok {
		return 0
	}
	return userID
}

// hasPermission checks if the current user has a specific permission for a tenant.
// Includes audit logging for security monitoring.
func (h *Handler) hasPermission(c echo.Context, tenantID int32, permission string) bool {
	userID, ok := c.Get("user-id").(int32)
	if !ok {
		slog.Warn("permission check failed: no user ID in context",
			"tenant_id", tenantID,
			"permission", permission,
			"result", "denied",
			"reason", "no_user_id",
		)
		return false
	}

	user, err := h.store.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		slog.Warn("permission check failed: user not found",
			"user_id", userID,
			"tenant_id", tenantID,
			"permission", permission,
			"result", "denied",
			"reason", "user_not_found",
		)
		return false
	}

	// HOST has all permissions
	if user.Role == store.RoleHost {
		slog.Debug("permission check",
			"user_id", userID,
			"username", user.Username,
			"tenant_id", tenantID,
			"permission", permission,
			"result", "granted",
			"reason", "host_role",
		)
		return true
	}

	// ADMIN has implicit tenant:read for all tenants
	if user.Role == store.RoleAdmin {
		if permission == PermTenantRead {
			slog.Debug("permission check",
				"user_id", userID,
				"username", user.Username,
				"tenant_id", tenantID,
				"permission", permission,
				"result", "granted",
				"reason", "admin_implicit_read",
			)
			return true
		}
		// ADMIN also has implicit api:config permission
		if permission == PermAPIConfig {
			slog.Debug("permission check",
				"user_id", userID,
				"username", user.Username,
				"tenant_id", tenantID,
				"permission", permission,
				"result", "granted",
				"reason", "admin_implicit_api_config",
			)
			return true
		}
	}

	// Check explicit user-tenant permissions
	perms, err := h.store.GetUserTenantPermission(c.Request().Context(), &store.FindUserTenantPermission{
		UserID:   &userID,
		TenantID: &tenantID,
	})
	if err != nil || perms == nil {
		slog.Info("permission check",
			"user_id", userID,
			"username", user.Username,
			"tenant_id", tenantID,
			"permission", permission,
			"result", "denied",
			"reason", "no_explicit_permission",
		)
		return false
	}

	granted := ContainsPermission(perms.Permissions, permission)
	if granted {
		slog.Debug("permission check",
			"user_id", userID,
			"username", user.Username,
			"tenant_id", tenantID,
			"permission", permission,
			"result", "granted",
			"reason", "explicit_permission",
			"user_permissions", perms.Permissions,
		)
	} else {
		slog.Info("permission check",
			"user_id", userID,
			"username", user.Username,
			"tenant_id", tenantID,
			"permission", permission,
			"result", "denied",
			"reason", "permission_not_in_list",
			"user_permissions", perms.Permissions,
		)
	}

	return granted
}

// ============================================================================
// LLM CONFIGURATION ENDPOINTS
// ============================================================================

// LLMConfigResponse represents the LLM configuration response.
type LLMConfigResponse struct {
	TenantSlug string `json:"tenant_slug"`
	LLMModel   string `json:"llm_model"`
	HasAPIKey  bool   `json:"has_api_key"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// SetLLMConfigRequest represents the request to set LLM config.
type SetLLMConfigRequest struct {
	LLMModel         string `json:"llm_model"`
	OpenRouterAPIKey string `json:"openrouter_api_key,omitempty"`
}

// HandleGetLLMConfig returns the LLM configuration for a tenant.
// GET /api/v1/agent/:slug/llm-config
func (h *Handler) HandleGetLLMConfig(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission (tenant:read or api:config)
	if !h.hasPermission(c, tenant.ID, PermTenantRead) && !h.hasPermission(c, tenant.ID, PermAPIConfig) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:read or api:config permission")
	}

	// Get config
	config, err := h.store.GetTenantConfig(ctx, &store.FindTenantConfig{TenantID: &tenant.ID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get config")
	}

	response := LLMConfigResponse{
		TenantSlug: slug,
		LLMModel:   "",
		HasAPIKey:  false,
	}

	if config != nil {
		response.LLMModel = config.LLMModel
		response.HasAPIKey = len(config.OpenRouterAPIKeyEncrypted) > 0
		response.UpdatedAt = config.UpdatedAt.Format(time.RFC3339)
	}

	return c.JSON(http.StatusOK, response)
}

// HandleSetLLMConfig sets the LLM configuration for a tenant.
// PUT /api/v1/agent/:slug/llm-config
func (h *Handler) HandleSetLLMConfig(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission (api:config or admin)
	if !h.hasPermission(c, tenant.ID, PermAPIConfig) && !h.isAdmin(c) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or api:config permission")
	}

	// Bind request
	var req SetLLMConfigRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Get current user ID for audit
	userID, _ := c.Get("user-id").(int32)

	// Build config
	config := &store.TenantConfig{
		TenantID:  tenant.ID,
		LLMModel:  req.LLMModel,
		UpdatedBy: &userID,
	}

	// Encrypt API key if provided
	if req.OpenRouterAPIKey != "" {
		// Validate key format (OpenRouter keys start with sk-or-)
		if !strings.HasPrefix(req.OpenRouterAPIKey, "sk-or-") {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid API key format (must start with sk-or-)")
		}

		if h.service.encryptionService == nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Encryption not configured. Set ENCRYPTION_MASTER_KEY environment variable.")
		}

		ciphertext, nonce, err := h.service.encryptionService.Encrypt(req.OpenRouterAPIKey)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to encrypt API key")
		}
		config.OpenRouterAPIKeyEncrypted = ciphertext
		config.OpenRouterAPIKeyNonce = nonce
	}

	// Upsert config
	config, err = h.store.UpsertTenantConfig(ctx, config)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save config")
	}

	// Invalidate cache
	h.service.configCache.Invalidate(tenant.Slug)

	return c.JSON(http.StatusOK, LLMConfigResponse{
		TenantSlug: slug,
		LLMModel:   config.LLMModel,
		HasAPIKey:  len(config.OpenRouterAPIKeyEncrypted) > 0,
		UpdatedAt:  config.UpdatedAt.Format(time.RFC3339),
	})
}

// ============================================================================
// PERMISSION MANAGEMENT ENDPOINTS
// ============================================================================

// UserPermissionResponse represents a user's permissions for a tenant.
type UserPermissionResponse struct {
	UserID      int32    `json:"user_id"`
	Username    string   `json:"username"`
	Permissions []string `json:"permissions"`
	GrantedBy   string   `json:"granted_by,omitempty"`
	GrantedAt   string   `json:"granted_at"`
}

// GrantPermissionRequest represents the request to grant permissions.
type GrantPermissionRequest struct {
	UserID      int32    `json:"user_id"`
	Permissions []string `json:"permissions"`
}

// HandleListPermissions lists all users with access to a tenant.
// GET /api/v1/agent/:slug/permissions
func (h *Handler) HandleListPermissions(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Must be admin or have tenant:admin
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or tenant:admin permission")
	}

	perms, err := h.store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{TenantID: &tenant.ID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list permissions")
	}

	// Build response with usernames
	result := make([]UserPermissionResponse, 0, len(perms))
	for _, p := range perms {
		user, _ := h.store.GetUser(ctx, &store.FindUser{ID: &p.UserID})
		username := ""
		if user != nil {
			username = user.Username
		}

		grantedBy := ""
		if p.GrantedBy != nil {
			grantor, _ := h.store.GetUser(ctx, &store.FindUser{ID: p.GrantedBy})
			if grantor != nil {
				grantedBy = grantor.Username
			}
		}

		result = append(result, UserPermissionResponse{
			UserID:      p.UserID,
			Username:    username,
			Permissions: p.Permissions,
			GrantedBy:   grantedBy,
			GrantedAt:   p.GrantedAt.Format(time.RFC3339),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"permissions": result})
}

// HandleGrantPermission grants a user access to a tenant.
// POST /api/v1/agent/:slug/permissions
func (h *Handler) HandleGrantPermission(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Must be admin or have tenant:admin
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or tenant:admin permission")
	}

	var req GrantPermissionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Validate permissions
	if !ValidatePermissions(req.Permissions) {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid permissions")
	}

	// Verify user exists
	user, err := h.store.GetUser(ctx, &store.FindUser{ID: &req.UserID})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "User not found")
	}

	grantorID, _ := c.Get("user-id").(int32)

	perm := &store.UserTenantPermission{
		UserID:      req.UserID,
		TenantID:    tenant.ID,
		Permissions: req.Permissions,
		GrantedBy:   &grantorID,
	}

	// Check if exists - update, else create
	existing, _ := h.store.GetUserTenantPermission(ctx, &store.FindUserTenantPermission{
		UserID:   &req.UserID,
		TenantID: &tenant.ID,
	})

	if existing != nil {
		perm.ID = existing.ID
		_, err = h.store.UpdateUserTenantPermission(ctx, perm)
	} else {
		_, err = h.store.CreateUserTenantPermission(ctx, perm)
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to grant permission")
	}

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

// HandleRevokePermission revokes a user's access to a tenant.
// DELETE /api/v1/agent/:slug/permissions/:userId
func (h *Handler) HandleRevokePermission(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	userIDStr := c.Param("userId")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Must be admin or have tenant:admin
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or tenant:admin permission")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user ID")
	}

	if err := h.store.DeleteUserTenantPermission(ctx, int32(userID), tenant.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to revoke permission")
	}

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

// HandleGetUserTenants returns tenants the current user can access.
// GET /api/v1/user/tenants
func (h *Handler) HandleGetUserTenants(c echo.Context) error {
	ctx := c.Request().Context()

	userID, ok := c.Get("user-id").(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
	}

	user, err := h.store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	result := make([]map[string]interface{}, 0)

	// HOST/ADMIN can access all tenants
	if user.Role == store.RoleHost || user.Role == store.RoleAdmin {
		tenants, _ := h.store.ListAgentTenants(ctx, &store.FindAgentTenant{})
		for _, t := range tenants {
			perms := []string{"*"}
			if user.Role == store.RoleAdmin {
				perms = []string{PermTenantRead, PermAPIConfig}
			}
			result = append(result, map[string]interface{}{
				"tenant":      t,
				"permissions": perms,
			})
		}
	} else {
		// Get explicit permissions
		perms, _ := h.store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &userID})
		for _, p := range perms {
			tenant, _ := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: &p.TenantID})
			if tenant != nil {
				result = append(result, map[string]interface{}{
					"tenant":      tenant,
					"permissions": p.Permissions,
				})
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"tenants": result})
}

// ============================================================================
// CHAT LOGS ENDPOINTS (requires chat:logs permission)
// ============================================================================

// HandleListSessions lists all chat sessions for a tenant.
// GET /api/v1/agent/:slug/sessions
// Query params: limit (default 50, max 100)
// Requires: ADMIN role OR chat:logs permission
func (h *Handler) HandleListSessions(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check admin role OR chat:logs permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatLogs) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:logs permission")
	}

	// Parse limit (default 50, max 100)
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Get sessions for this tenant
	sessions, err := h.store.ListAgentSessions(ctx, &store.FindAgentSession{TenantID: &tenant.ID, Limit: &limit})
	if err != nil {
		slog.Error("list sessions failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list sessions")
	}

	// Convert to response format (without full message content for list view)
	result := make([]map[string]interface{}, len(sessions))
	for i, s := range sessions {
		result[i] = map[string]interface{}{
			"id":               s.ID,
			"audienceType":     s.AudienceType,
			"userId":           s.UserID,
			"phase":            s.Phase,
			"currentIntent":    s.CurrentIntent,
			"urgencyLevel":     s.UrgencyLevel,
			"coverageStatus":   s.CoverageStatus,
			"customerName":     s.CustomerName,
			"customerPhone":    s.CustomerPhone,
			"customerLocation": s.CustomerLocation,
			"detectedService":  s.DetectedService,
			"messageCount":     s.MessageCount,
			"createdAt":        s.CreatedAt,
			"updatedAt":        s.UpdatedAt,
			"completedAt":      s.CompletedAt,
			"isCompleted":      s.IsCompleted,
			"completionReason": s.CompletionReason,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"sessions": result})
}

// HandleGetSession returns details of a specific chat session including messages.
// GET /api/v1/agent/:slug/sessions/:sessionId
// Requires: ADMIN role OR chat:logs permission
func (h *Handler) HandleGetSession(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	sessionID := c.Param("sessionId")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check admin role OR chat:logs permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatLogs) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:logs permission")
	}

	// Get the session
	sessions, err := h.store.ListAgentSessions(ctx, &store.FindAgentSession{ID: &sessionID, TenantID: &tenant.ID})
	if err != nil {
		slog.Error("get session failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get session")
	}

	if len(sessions) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Session not found")
	}

	session := sessions[0]

	// Convert messages to response format
	messages := make([]map[string]interface{}, len(session.Messages))
	for i, m := range session.Messages {
		messages[i] = map[string]interface{}{
			"role":      m.Role,
			"content":   m.Content,
			"timestamp": m.Timestamp,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"session": map[string]interface{}{
			"id":               session.ID,
			"audienceType":     session.AudienceType,
			"userId":           session.UserID,
			"phase":            session.Phase,
			"currentIntent":    session.CurrentIntent,
			"urgencyLevel":     session.UrgencyLevel,
			"coverageStatus":   session.CoverageStatus,
			"customerName":     session.CustomerName,
			"customerPhone":    session.CustomerPhone,
			"customerLocation": session.CustomerLocation,
			"detectedService":  session.DetectedService,
			"messageCount":     session.MessageCount,
			"messages":         messages,
			"createdAt":        session.CreatedAt,
			"updatedAt":        session.UpdatedAt,
			"completedAt":      session.CompletedAt,
			"isCompleted":      session.IsCompleted,
			"completionReason": session.CompletionReason,
		},
	})
}

// ============================================================================
// SIMULATION ENDPOINTS
// ============================================================================

// HandleStartSimulation starts a new simulation session.
// POST /api/v1/agent/:slug/simulate
// Requires: ADMIN role OR chat:test permission
func (h *Handler) HandleStartSimulation(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:test permission")
	}

	// Bind request
	var req SimulationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if strings.TrimSpace(req.InitialPrompt) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Initial prompt is required")
	}

	// Get user ID
	userID, _ := c.Get("user-id").(int32)
	if userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
	}

	// Load config
	config, err := h.service.LoadConfig(ctx, slug, "internal")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load config: "+err.Error())
	}

	// Create simulation session
	state := h.service.GetSimulationSessions().Create(tenant.ID, userID, slug, req.InitialPrompt, req.PersonaHint)

	// Start simulation in background goroutine
	go func() {
		msgChan := make(chan SimulationMessage, 100)
		statusChan := make(chan SimulationStatus, 100)
		completeChan := make(chan SimulationComplete, 1)

		// Run simulation
		bgCtx := context.Background()
		h.service.RunSimulation(bgCtx, config, state, msgChan, statusChan, completeChan)

		// Save transcript when complete
		state.mu.RLock()
		if state.Status == "completed" || state.Status == "stopped" {
			state.mu.RUnlock()
			_, err := h.service.SaveSimulationTranscript(bgCtx, state)
			if err != nil {
				slog.Error("failed to save simulation transcript", "error", err)
			}
		} else {
			state.mu.RUnlock()
		}
	}()

	return c.JSON(http.StatusOK, SimulationStartResponse{
		SessionID: state.ID,
		Status:    "running",
		StreamURL: fmt.Sprintf("/api/v1/agent/%s/simulate/%s/stream", slug, state.ID),
	})
}

// HandleSimulationStream provides SSE stream for simulation messages.
// GET /api/v1/agent/:slug/simulate/:sessionId/stream
// Requires: ADMIN role OR chat:test permission
func (h *Handler) HandleSimulationStream(c echo.Context) error {
	slug := c.Param("slug")
	sessionID := c.Param("sessionId")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
	}

	// Get simulation state
	state := h.service.GetSimulationSessions().Get(sessionID)
	if state == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Simulation session not found")
	}

	// Set up SSE response
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Accel-Buffering", "no")
	c.Response().WriteHeader(http.StatusOK)

	// Track which messages we've sent
	lastMsgIndex := 0

	// Poll for updates
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request().Context().Done():
			return nil
		case <-ticker.C:
			state.mu.RLock()
			status := state.Status
			messages := state.Messages
			currentTurn := state.CurrentTurn
			endReason := state.EndReason
			state.mu.RUnlock()

			// Send new messages
			for i := lastMsgIndex; i < len(messages); i++ {
				msg := messages[i]
				data := fmt.Sprintf(`{"role":"%s","content":%q,"turn_num":%d,"timestamp":"%s"`,
					msg.Role, msg.Content, msg.TurnNum, msg.Timestamp.Format(time.RFC3339))
				if msg.Metadata != nil {
					data += fmt.Sprintf(`,"metadata":{"intent":"%s","phase":"%s","urgency":%d}`,
						msg.Metadata.Intent, msg.Metadata.Phase, msg.Metadata.Urgency)
				}
				data += "}"

				_, err := fmt.Fprintf(c.Response(), "event: message\ndata: %s\n\n", data)
				if err != nil {
					return nil
				}
				c.Response().Flush()
				lastMsgIndex = i + 1
			}

			// Send status update
			statusData := fmt.Sprintf(`{"status":"%s","current_turn":%d}`, status, currentTurn)
			_, err := fmt.Fprintf(c.Response(), "event: status\ndata: %s\n\n", statusData)
			if err != nil {
				return nil
			}
			c.Response().Flush()

			// Check for completion
			if status == "completed" || status == "stopped" {
				completeData := fmt.Sprintf(`{"status":"%s","total_turns":%d,"end_reason":"%s"}`,
					status, currentTurn, endReason)
				fmt.Fprintf(c.Response(), "event: complete\ndata: %s\n\n", completeData)
				c.Response().Flush()
				return nil
			}
		}
	}
}

// HandleSimulationControl handles pause/resume/stop for a simulation.
// POST /api/v1/agent/:slug/simulate/:sessionId/control
// Requires: ADMIN role OR chat:test permission
func (h *Handler) HandleSimulationControl(c echo.Context) error {
	slug := c.Param("slug")
	sessionID := c.Param("sessionId")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
	}

	// Bind request
	var req SimulationControlRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	// Get simulation state
	state := h.service.GetSimulationSessions().Get(sessionID)
	if state == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Simulation session not found")
	}

	// Apply control action
	switch req.Action {
	case "pause":
		select {
		case state.pauseCh <- struct{}{}:
		default:
		}
		return c.JSON(http.StatusOK, SimulationControlResponse{Success: true, Status: "paused"})
	case "resume":
		select {
		case state.resumeCh <- struct{}{}:
		default:
		}
		return c.JSON(http.StatusOK, SimulationControlResponse{Success: true, Status: "running"})
	case "stop":
		select {
		case state.stopCh <- struct{}{}:
		default:
		}
		return c.JSON(http.StatusOK, SimulationControlResponse{Success: true, Status: "stopped"})
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid action. Use: pause, resume, stop")
	}
}

// HandleListSimulations lists saved simulation transcripts.
// GET /api/v1/agent/:slug/simulations
// Requires: ADMIN role OR chat:logs permission
func (h *Handler) HandleListSimulations(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatLogs) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:logs permission")
	}

	// Parse pagination params
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if limit <= 0 {
		limit = 20
	}

	// List transcripts
	transcripts, total, err := h.service.ListSimulationTranscripts(ctx, tenant.ID, limit, offset)
	if err != nil {
		slog.Error("list simulations failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list simulations")
	}

	// Convert to response format
	result := make([]map[string]interface{}, len(transcripts))
	for i, t := range transcripts {
		result[i] = map[string]interface{}{
			"id":            t.ID,
			"initialPrompt": t.InitialPrompt,
			"personaHint":   t.PersonaHint,
			"totalTurns":    t.TotalTurns,
			"endReason":     t.EndReason,
			"createdAt":     t.CreatedAt,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"simulations": result,
		"total":       total,
	})
}

// HandleGetSimulation returns a specific simulation transcript.
// GET /api/v1/agent/:slug/simulations/:simulationId
// Requires: ADMIN role OR chat:logs permission
func (h *Handler) HandleGetSimulation(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	simulationID := c.Param("simulationId")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatLogs) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires admin role or chat:logs permission")
	}

	// Get transcript
	transcript, err := h.service.GetSimulationTranscript(ctx, simulationID)
	if err != nil {
		slog.Error("get simulation failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get simulation")
	}
	if transcript == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Simulation not found")
	}

	// Verify tenant ownership
	if transcript.TenantID != tenant.ID {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
	}

	// Convert messages to response format
	messages := make([]map[string]interface{}, len(transcript.Messages))
	for i, m := range transcript.Messages {
		msg := map[string]interface{}{
			"role":      m.Role,
			"content":   m.Content,
			"turnNum":   m.TurnNum,
			"timestamp": m.Timestamp,
		}
		if m.Metadata != nil {
			msg["metadata"] = map[string]interface{}{
				"intent":  m.Metadata.Intent,
				"phase":   m.Metadata.Phase,
				"urgency": m.Metadata.Urgency,
			}
		}
		messages[i] = msg
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":            transcript.ID,
		"tenantId":      transcript.TenantID,
		"userId":        transcript.UserID,
		"initialPrompt": transcript.InitialPrompt,
		"personaHint":   transcript.PersonaHint,
		"totalTurns":    transcript.TotalTurns,
		"endReason":     transcript.EndReason,
		"messages":      messages,
		"createdAt":     transcript.CreatedAt,
	})
}

// ============================================================================
// UNIFIED CONVERSATION HISTORY (combines simulations and chat sessions)
// ============================================================================

// ConversationSummary represents a unified view of either a simulation or chat session
type ConversationSummary struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"` // "simulation" or "chat"
	Summary      string    `json:"summary"`
	MessageCount int       `json:"messageCount"`
	CreatedAt    time.Time `json:"createdAt"`
	// Simulation-specific fields
	EndReason   string `json:"endReason,omitempty"`
	PersonaHint string `json:"personaHint,omitempty"`
	// Chat-specific fields
	Phase        string `json:"phase,omitempty"`
	AudienceType string `json:"audienceType,omitempty"`
	CustomerName string `json:"customerName,omitempty"`
}

// HandleGetConversations returns a unified list of simulations and chat sessions.
// GET /api/v1/agent/:slug/conversations
// Returns conversations based on user permissions:
// - chat:test permission: includes simulations
// - chat:logs permission: includes chat sessions
func (h *Handler) HandleGetConversations(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permissions
	isAdmin := h.isAdmin(c)
	canViewSimulations := isAdmin || h.hasPermission(c, tenant.ID, PermChatTest)
	canViewChats := isAdmin || h.hasPermission(c, tenant.ID, PermChatLogs)

	if !canViewSimulations && !canViewChats {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires chat:test or chat:logs permission")
	}

	// Parse limit (max 50)
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 50 {
		limit = 50
	}

	var conversations []ConversationSummary

	// Fetch simulations if permitted
	if canViewSimulations {
		transcripts, _, err := h.service.ListSimulationTranscripts(ctx, tenant.ID, limit, 0)
		if err != nil {
			slog.Error("list simulations failed", "error", err)
		} else {
			for _, t := range transcripts {
				summary := t.InitialPrompt
				if len(summary) > 50 {
					summary = summary[:50] + "..."
				}
				conversations = append(conversations, ConversationSummary{
					ID:           t.ID,
					Type:         "simulation",
					Summary:      summary,
					MessageCount: len(t.Messages),
					CreatedAt:    t.CreatedAt,
					EndReason:    t.EndReason,
					PersonaHint:  t.PersonaHint,
				})
			}
		}
	}

	// Fetch chat sessions if permitted
	if canViewChats {
		sessions, err := h.store.ListAgentSessions(ctx, &store.FindAgentSession{TenantID: &tenant.ID, Limit: &limit})
		if err != nil {
			slog.Error("list sessions failed", "error", err)
		} else {
			for _, s := range sessions {
				// Build summary from customer name and intent
				summary := ""
				if s.CustomerName != "" {
					summary = s.CustomerName
				}
				if s.CurrentIntent != "" {
					if summary != "" {
						summary += " - " + s.CurrentIntent
					} else {
						summary = s.CurrentIntent
					}
				}
				if summary == "" {
					summary = "Chat session"
				}
				if len(summary) > 50 {
					summary = summary[:50] + "..."
				}

				conversations = append(conversations, ConversationSummary{
					ID:           s.ID,
					Type:         "chat",
					Summary:      summary,
					MessageCount: s.MessageCount,
					CreatedAt:    s.CreatedAt,
					Phase:        s.Phase,
					AudienceType: s.AudienceType,
					CustomerName: s.CustomerName,
				})
			}
		}
	}

	// Sort by CreatedAt descending
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].CreatedAt.After(conversations[j].CreatedAt)
	})

	// Limit to requested count
	if len(conversations) > limit {
		conversations = conversations[:limit]
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"conversations": conversations,
		"permissions": map[string]bool{
			"canViewSimulations": canViewSimulations,
			"canViewChats":       canViewChats,
		},
		"total": len(conversations),
	})
}

// HandleGetConversation returns details of a specific conversation (simulation or chat).
// GET /api/v1/agent/:slug/conversations/:conversationId
// Permission check based on conversation type.
func (h *Handler) HandleGetConversation(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	conversationID := c.Param("conversationId")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	isAdmin := h.isAdmin(c)

	// Determine type by ID prefix and fetch accordingly
	if strings.HasPrefix(conversationID, "sim-") {
		// It's a simulation - check chat:test permission
		if !isAdmin && !h.hasPermission(c, tenant.ID, PermChatTest) {
			return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires chat:test permission")
		}

		transcript, err := h.service.GetSimulationTranscript(ctx, conversationID)
		if err != nil {
			slog.Error("get simulation failed", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get conversation")
		}
		if transcript == nil || transcript.TenantID != tenant.ID {
			return echo.NewHTTPError(http.StatusNotFound, "Conversation not found")
		}

		// Convert messages to unified format
		messages := make([]map[string]interface{}, len(transcript.Messages))
		for i, m := range transcript.Messages {
			// Normalize role: human_sim -> human
			role := "human"
			if m.Role == "agent" {
				role = "agent"
			}
			msg := map[string]interface{}{
				"role":      role,
				"content":   m.Content,
				"timestamp": m.Timestamp,
				"turnNum":   m.TurnNum,
			}
			if m.Metadata != nil {
				msg["metadata"] = map[string]interface{}{
					"intent":  m.Metadata.Intent,
					"phase":   m.Metadata.Phase,
					"urgency": m.Metadata.Urgency,
				}
			}
			messages[i] = msg
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":            transcript.ID,
			"type":          "simulation",
			"initialPrompt": transcript.InitialPrompt,
			"personaHint":   transcript.PersonaHint,
			"totalTurns":    transcript.TotalTurns,
			"endReason":     transcript.EndReason,
			"messages":      messages,
			"createdAt":     transcript.CreatedAt,
		})
	}

	// It's a chat session - check chat:logs permission
	if !isAdmin && !h.hasPermission(c, tenant.ID, PermChatLogs) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires chat:logs permission")
	}

	sessions, err := h.store.ListAgentSessions(ctx, &store.FindAgentSession{ID: &conversationID, TenantID: &tenant.ID})
	if err != nil {
		slog.Error("get session failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get conversation")
	}
	if len(sessions) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Conversation not found")
	}

	session := sessions[0]

	// Convert messages to unified format
	messages := make([]map[string]interface{}, len(session.Messages))
	for i, m := range session.Messages {
		// Normalize role: user -> human, assistant -> agent
		role := "human"
		if m.Role == "assistant" {
			role = "agent"
		}
		messages[i] = map[string]interface{}{
			"role":      role,
			"content":   m.Content,
			"timestamp": m.Timestamp,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":           session.ID,
		"type":         "chat",
		"audienceType": session.AudienceType,
		"customerName": session.CustomerName,
		"phase":        session.Phase,
		"currentIntent": session.CurrentIntent,
		"messages":     messages,
		"createdAt":    session.CreatedAt,
	})
}

// ============================================================================
// SCRIPT.MD HANDLERS
// ============================================================================

// HandleImportScript imports a SCRIPT.MD file for a tenant.
// POST /api/v1/agent/:slug/script
// Permission: tenant:write or tenant:admin
func (h *Handler) HandleImportScript(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantWrite) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:write permission")
	}

	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "File required")
	}

	// Read file content
	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read file")
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read file content")
	}

	// Parse the script to generate summary
	parser := NewParser()
	parsedScript, err := parser.ParseScript(string(content))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to parse script: "+err.Error())
	}

	// Validate the script
	validation := parser.ValidateScript(parsedScript)
	if !validation.IsValid() {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"message":  "Script validation failed",
			"errors":   validation.Errors,
			"warnings": validation.Warnings,
		})
	}

	// Create hash
	contentHash := ContentHash(string(content))

	// Upsert script
	script := &store.AgentTenantScript{
		TenantID:    tenant.ID,
		Content:     string(content),
		ContentHash: contentHash,
		Summary:     parsedScript.Summary,
	}

	script, err = h.store.UpsertAgentTenantScript(ctx, script)
	if err != nil {
		slog.Error("upsert script failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save script")
	}

	// Invalidate config cache
	h.service.InvalidateConfigCache(slug)

	return c.JSON(http.StatusOK, map[string]any{
		"message":  "Script imported successfully",
		"id":       script.ID,
		"version":  script.Version,
		"sections": len(parsedScript.Sections),
		"warnings": validation.Warnings,
	})
}

// HandleGetScript returns the script for a tenant.
// GET /api/v1/agent/:slug/script
// Permission: tenant:read
func (h *Handler) HandleGetScript(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantRead) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:read permission")
	}

	script, err := h.store.GetAgentTenantScript(ctx, &store.FindAgentTenantScript{TenantID: &tenant.ID})
	if err != nil {
		slog.Error("get script failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get script")
	}

	if script == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"hasScript": false,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"hasScript":   true,
		"id":          script.ID,
		"content":     script.Content,
		"contentHash": script.ContentHash,
		"summary":     script.Summary,
		"importedAt":  script.ImportedAt,
		"version":     script.Version,
	})
}

// HandleDeleteScript deletes the script for a tenant.
// DELETE /api/v1/agent/:slug/script
// Permission: tenant:write or tenant:admin
func (h *Handler) HandleDeleteScript(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantWrite) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:write permission")
	}

	if err := h.store.DeleteAgentTenantScript(ctx, tenant.ID); err != nil {
		slog.Error("delete script failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete script")
	}

	// Invalidate config cache
	h.service.InvalidateConfigCache(slug)

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Script deleted successfully",
	})
}

// ============================================================================
// ANALYSIS HANDLERS
// ============================================================================

// HandleAnalyzeTranscript analyzes a conversation transcript against benchmarks.
// POST /api/v1/agent/:slug/analyze
// Permission: chat:test
func (h *Handler) HandleAnalyzeTranscript(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires chat:test permission")
	}

	// Parse request
	var req struct {
		ConversationID     string `json:"conversation_id"`
		IncludeSuggestions bool   `json:"include_suggestions"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if req.ConversationID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "conversation_id required")
	}

	// Get current user ID
	userID := h.getUserID(c)
	if userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	// Run analysis
	result, err := h.service.AnalyzeTranscript(ctx, tenant.ID, req.ConversationID, int32(userID), req.IncludeSuggestions)
	if err != nil {
		slog.Error("analyze transcript failed", "error", err, "conversationId", req.ConversationID)
		return echo.NewHTTPError(http.StatusInternalServerError, "Analysis failed: "+err.Error())
	}

	return c.JSON(http.StatusOK, result)
}

// HandleGetAnalysisHistory returns past analysis results for a tenant.
// GET /api/v1/agent/:slug/analysis
// Permission: chat:test
func (h *Handler) HandleGetAnalysisHistory(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermChatTest) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires chat:test permission")
	}

	// Parse limit
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	results, total, err := h.store.ListAgentAnalysisResults(ctx, &store.FindAgentAnalysisResult{
		TenantID: &tenant.ID,
		Limit:    limit,
	})
	if err != nil {
		slog.Error("list analysis results failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get analysis history")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"results": results,
		"total":   total,
	})
}

// ============================================================================
// LEARNING MEMORY HANDLERS
// ============================================================================

// HandleGetLearning returns the learning memory for a tenant.
// GET /api/v1/agent/:slug/learning
// Permission: tenant:admin
func (h *Handler) HandleGetLearning(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:admin permission")
	}

	learningService := NewLearningService(h.store)
	memory, err := learningService.GetLearningMemory(ctx, tenant.ID)
	if err != nil {
		slog.Error("get learning memory failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get learning memory")
	}

	return c.JSON(http.StatusOK, memory)
}

// HandleRegenerateLearning re-aggregates learning insights from recent analysis results.
// POST /api/v1/agent/:slug/learning/regenerate
// Permission: tenant:admin
func (h *Handler) HandleRegenerateLearning(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:admin permission")
	}

	learningService := NewLearningService(h.store)
	memory, err := learningService.AggregateFromAnalysis(ctx, tenant.ID)
	if err != nil {
		slog.Error("regenerate learning failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to regenerate learning: "+err.Error())
	}

	// Invalidate config cache
	h.service.InvalidateConfigCache(slug)

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Learning memory regenerated",
		"memory":  memory,
	})
}

// HandleApproveSuggestion approves a pending suggestion and converts it to a learned behavior.
// POST /api/v1/agent/:slug/learning/approve
// Permission: tenant:admin
func (h *Handler) HandleApproveSuggestion(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:admin permission")
	}

	// Parse request
	var req struct {
		SuggestionID   string  `json:"suggestion_id"`
		EditedBehavior *string `json:"edited_behavior,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if req.SuggestionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "suggestion_id required")
	}

	learningService := NewLearningService(h.store)
	memory, err := learningService.ApproveSuggestion(ctx, tenant.ID, req.SuggestionID, req.EditedBehavior)
	if err != nil {
		slog.Error("approve suggestion failed", "error", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Invalidate config cache
	h.service.InvalidateConfigCache(slug)

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Suggestion approved",
		"memory":  memory,
	})
}

// HandleDismissSuggestion removes a pending suggestion without approving it.
// POST /api/v1/agent/:slug/learning/dismiss
// Permission: tenant:admin
func (h *Handler) HandleDismissSuggestion(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:admin permission")
	}

	// Parse request
	var req struct {
		SuggestionID string `json:"suggestion_id"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if req.SuggestionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "suggestion_id required")
	}

	learningService := NewLearningService(h.store)
	memory, err := learningService.DismissSuggestion(ctx, tenant.ID, req.SuggestionID)
	if err != nil {
		slog.Error("dismiss suggestion failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to dismiss suggestion")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Suggestion dismissed",
		"memory":  memory,
	})
}

// HandleRemoveLearnedBehavior removes a learned behavior.
// DELETE /api/v1/agent/:slug/learning/behaviors/:behaviorId
// Permission: tenant:admin
func (h *Handler) HandleRemoveLearnedBehavior(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	behaviorID := c.Param("behaviorId")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:admin permission")
	}

	learningService := NewLearningService(h.store)
	memory, err := learningService.RemoveLearnedBehavior(ctx, tenant.ID, behaviorID)
	if err != nil {
		slog.Error("remove learned behavior failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to remove behavior")
	}

	// Invalidate config cache
	h.service.InvalidateConfigCache(slug)

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Behavior removed",
		"memory":  memory,
	})
}

// HandleToggleLearnedBehavior toggles a learned behavior's active state.
// POST /api/v1/agent/:slug/learning/behaviors/:behaviorId/toggle
// Permission: tenant:admin
func (h *Handler) HandleToggleLearnedBehavior(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")
	behaviorID := c.Param("behaviorId")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:admin permission")
	}

	learningService := NewLearningService(h.store)
	memory, err := learningService.ToggleLearnedBehavior(ctx, tenant.ID, behaviorID)
	if err != nil {
		slog.Error("toggle learned behavior failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to toggle behavior")
	}

	// Invalidate config cache
	h.service.InvalidateConfigCache(slug)

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Behavior toggled",
		"memory":  memory,
	})
}

// HandleClearLearning clears all learning memory for a tenant.
// DELETE /api/v1/agent/:slug/learning
// Permission: tenant:admin
func (h *Handler) HandleClearLearning(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:admin permission")
	}

	learningService := NewLearningService(h.store)
	if err := learningService.ClearAllLearning(ctx, tenant.ID); err != nil {
		slog.Error("clear learning failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to clear learning")
	}

	// Invalidate config cache
	h.service.InvalidateConfigCache(slug)

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Learning memory cleared",
	})
}

// HandleApplyLearnings applies selected issues and suggestions from an analysis
// directly to learned behaviors (simplified v2 workflow).
// POST /api/v1/agent/:slug/learning/apply
// Permission: tenant:admin
func (h *Handler) HandleApplyLearnings(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	// Get tenant
	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	// Check permission
	if !h.isAdmin(c) && !h.hasPermission(c, tenant.ID, PermTenantAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied: requires tenant:admin permission")
	}

	// Parse request
	var req struct {
		AnalysisID          string `json:"analysis_id"`
		SelectedIssues      []int  `json:"selected_issues"`
		SelectedSuggestions []int  `json:"selected_suggestions"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	if req.AnalysisID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "analysis_id is required")
	}

	if len(req.SelectedIssues) == 0 && len(req.SelectedSuggestions) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "No items selected to apply")
	}

	learningService := NewLearningService(h.store)
	memory, appliedCount, err := learningService.ApplySelectedLearnings(ctx, tenant.ID, req.AnalysisID, req.SelectedIssues, req.SelectedSuggestions)
	if err != nil {
		slog.Error("apply learnings failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to apply learnings: "+err.Error())
	}

	// Invalidate config cache
	h.service.InvalidateConfigCache(slug)

	return c.JSON(http.StatusOK, map[string]any{
		"applied_count":     appliedCount,
		"learned_behaviors": memory.LearnedBehaviors,
		"message":           fmt.Sprintf("%d improvements applied", appliedCount),
	})
}
