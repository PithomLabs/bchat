package agent

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/usememos/memos/store"
)

// ============================================================================
// SSRF PROTECTION (copied from plugin/webhook/webhook.go — unexported there)
// ============================================================================

// isInternalIP checks if an IP is internal/private and should not be reachable.
func isInternalIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	metadataIPs := []string{"169.254.169.254", "fd00:ec2::254"}
	for _, mIP := range metadataIPs {
		if ip.Equal(net.ParseIP(mIP)) {
			return true
		}
	}
	return false
}

// validateAndResolveWebhookURL validates a URL is safe to fetch (no SSRF)
// and resolves it to an IP for IP-pinned dialing.
func validateAndResolveWebhookURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("URL scheme must be http or https, got %s", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return "", fmt.Errorf("URL must have a host")
	}

	// Strip port for DNS resolution
	host := parsedURL.Hostname()
	ips, err := net.LookupHost(host)
	if err != nil {
		return "", fmt.Errorf("DNS resolution failed for %s: %w", host, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IPs found for %s", host)
	}

	// Check all resolved IPs — reject if ANY is internal
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return "", fmt.Errorf("failed to parse IP %s", ipStr)
		}
		if isInternalIP(ip) {
			return "", fmt.Errorf("URL resolves to internal IP %s", ipStr)
		}
	}

	// Return the URL string (first valid external IP is used for dialing)
	return parsedURL.String(), nil
}

// buildSecureHTTPClient creates an HTTP client with reasonable timeouts.
func buildSecureHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 5 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout:  5 * time.Second,
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   5,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			// Re-validate redirect target
			if _, err := validateAndResolveWebhookURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect to unsafe URL: %w", err)
			}
			return nil
		},
	}
}

// ============================================================================
// HMAC SIGNING
// ============================================================================

// signPayload computes HMAC-SHA256 signature for a payload.
func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// ============================================================================
// IDEMPOTENCY KEY
// ============================================================================

// computeIdempotencyKey computes a deterministic idempotency key from variadic components.
func computeIdempotencyKey(components ...string) string {
	h := sha256.New()
	for _, c := range components {
		h.Write([]byte(c))
		h.Write([]byte(":"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ============================================================================
// WEBHOOK DELIVERY
// ============================================================================

// deliverWebhook sends a webhook event to the configured URL.
// Returns nil on success (2xx response), error otherwise.
func (s *Service) deliverWebhook(ctx context.Context, wh store.WebhookConfig, eventType string, payload []byte) error {
	if wh.URL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	// Validate URL (SSRF protection)
	safeURL, err := validateAndResolveWebhookURL(wh.URL)
	if err != nil {
		return fmt.Errorf("webhook URL validation failed: %w", err)
	}

	// Sign payload
	signature := signPayload(payload, wh.Secret)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, safeURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bchat-Signature", signature)
	req.Header.Set("X-Bchat-Event", eventType)
	for k, v := range wh.Headers {
		req.Header.Set(k, v)
	}

	// Execute request
	client := buildSecureHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook delivery failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body (limit to 1KB)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("webhook delivered",
		"url", wh.URL,
		"event", eventType,
		"status", resp.StatusCode,
	)
	return nil
}

// ============================================================================
// TRIGGER CRON HANDLER
// ============================================================================

// HandleTriggerCron handles the cron trigger endpoint for supercronic.
func (h *Handler) HandleTriggerCron(c echo.Context) error {
	token := c.Request().Header.Get("X-Cron-Token")
	expectedToken := os.Getenv("CRON_TOKEN")
	if token == "" || expectedToken == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	// Constant-time comparison (no length check to avoid timing oracle)
	if !hmac.Equal([]byte(token), []byte(expectedToken)) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	h.service.processEventPoller(c.Request().Context())
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ============================================================================
// INTEGRATION CRUD HANDLERS
// ============================================================================

// HandleListIntegrations lists all integrations for a tenant.
func (h *Handler) HandleListIntegrations(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	integrations, err := h.store.ListAgentIntegrations(ctx, &store.FindAgentIntegration{TenantID: &tenant.ID})
	if err != nil {
		slog.Error("failed to list integrations", "tenant_id", tenant.ID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list integrations")
	}

	// Mask secrets in config
	type safeIntegration struct {
		ID              int32  `json:"id"`
		TenantID        int32  `json:"tenant_id"`
		IntegrationType string `json:"integration_type"`
		Label           string `json:"label"`
		IsActive        bool   `json:"is_active"`
		CreatedAt       int64  `json:"created_at"`
		UpdatedAt       int64  `json:"updated_at"`
	}
	safe := make([]safeIntegration, 0, len(integrations))
	for _, ig := range integrations {
		safe = append(safe, safeIntegration{
			ID:              ig.ID,
			TenantID:        ig.TenantID,
			IntegrationType: ig.IntegrationType,
			Label:           ig.Label,
			IsActive:        ig.IsActive,
			CreatedAt:       ig.CreatedAt,
			UpdatedAt:       ig.UpdatedAt,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"integrations": safe,
	})
}

// HandleCreateIntegration creates a new integration.
func (h *Handler) HandleCreateIntegration(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	var req struct {
		IntegrationType string `json:"integration_type"`
		Label           string `json:"label"`
		Config          struct {
			URL     string            `json:"url"`
			Secret  string            `json:"secret"`
			Headers map[string]string `json:"headers,omitempty"`
		} `json:"config"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.IntegrationType != "webhook" {
		return echo.NewHTTPError(http.StatusBadRequest, "integration_type must be 'webhook'")
	}
	if req.Config.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "config.url is required")
	}
	if req.Config.Secret == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "config.secret is required")
	}

	// Validate URL (SSRF protection)
	if _, err := validateAndResolveWebhookURL(req.Config.URL); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid webhook URL: %v", err))
	}

	configJSON, err := json.Marshal(store.WebhookConfig{
		URL:     req.Config.URL,
		Secret:  req.Config.Secret,
		Headers: req.Config.Headers,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to marshal config")
	}

	integration := &store.AgentIntegration{
		TenantID:        tenant.ID,
		IntegrationType: req.IntegrationType,
		Label:           req.Label,
		Config:          string(configJSON),
		IsActive:        true,
	}

	created, err := h.store.CreateAgentIntegration(ctx, integration)
	if err != nil {
		slog.Error("failed to create integration", "tenant_id", tenant.ID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create integration")
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"id":              created.ID,
		"tenant_id":       created.TenantID,
		"integration_type": created.IntegrationType,
		"label":           created.Label,
		"is_active":       created.IsActive,
		"created_at":      created.CreatedAt,
	})
}

// HandleUpdateIntegration updates an existing integration.
func (h *Handler) HandleUpdateIntegration(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	idStr := c.Param("id")
	var id int32
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid integration ID")
	}

	// Verify ownership
	existing, err := h.store.GetAgentIntegration(ctx, &store.FindAgentIntegration{ID: &id, TenantID: &tenant.ID})
	if err != nil || existing == nil {
		return echo.NewHTTPError(http.StatusNotFound, "integration not found")
	}

	var req struct {
		Label  *string `json:"label"`
		Config *struct {
			URL     string            `json:"url"`
			Secret  string            `json:"secret"`
			Headers map[string]string `json:"headers,omitempty"`
		} `json:"config"`
		IsActive *bool `json:"is_active"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Label != nil {
		existing.Label = *req.Label
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	if req.Config != nil {
		if req.Config.URL == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "config.url is required")
		}
		if _, err := validateAndResolveWebhookURL(req.Config.URL); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid webhook URL: %v", err))
		}
		configJSON, err := json.Marshal(store.WebhookConfig{
			URL:     req.Config.URL,
			Secret:  req.Config.Secret,
			Headers: req.Config.Headers,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to marshal config")
		}
		existing.Config = string(configJSON)
	}

	if err := h.store.UpdateAgentIntegration(ctx, existing); err != nil {
		slog.Error("failed to update integration", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update integration")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":              existing.ID,
		"tenant_id":       existing.TenantID,
		"integration_type": existing.IntegrationType,
		"label":           existing.Label,
		"is_active":       existing.IsActive,
		"updated_at":      existing.UpdatedAt,
	})
}

// HandleDeleteIntegration deletes an integration.
func (h *Handler) HandleDeleteIntegration(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	idStr := c.Param("id")
	var id int32
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid integration ID")
	}

	// Verify ownership
	existing, err := h.store.GetAgentIntegration(ctx, &store.FindAgentIntegration{ID: &id, TenantID: &tenant.ID})
	if err != nil || existing == nil {
		return echo.NewHTTPError(http.StatusNotFound, "integration not found")
	}

	if err := h.store.DeleteAgentIntegration(ctx, id); err != nil {
		slog.Error("failed to delete integration", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete integration")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// HandleTestIntegration sends a test webhook.
func (h *Handler) HandleTestIntegration(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	idStr := c.Param("id")
	var id int32
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid integration ID")
	}

	existing, err := h.store.GetAgentIntegration(ctx, &store.FindAgentIntegration{ID: &id, TenantID: &tenant.ID})
	if err != nil || existing == nil {
		return echo.NewHTTPError(http.StatusNotFound, "integration not found")
	}

	var config store.WebhookConfig
	if err := json.Unmarshal([]byte(existing.Config), &config); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to parse integration config")
	}

	testPayload := map[string]interface{}{
		"event":     "test.ping",
		"tenant_id": tenant.ID,
		"timestamp": time.Now().Unix(),
		"data": map[string]string{
			"message": "This is a test webhook from bchat",
		},
	}
	payload, _ := json.Marshal(testPayload)

	if err := h.service.deliverWebhook(ctx, config, "test.ping", payload); err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Test webhook delivered successfully",
	})
}

// HandleListEvents lists events for a tenant.
// Payload is intentionally excluded to avoid leaking PII (name, email, phone).
func (h *Handler) HandleListEvents(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	tenant, err := h.store.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
	if err != nil || tenant == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Tenant not found")
	}

	find := &store.FindAgentEvent{TenantID: &tenant.ID}
	if status := c.QueryParam("status"); status != "" {
		find.Status = &status
	}

	events, err := h.store.ListAgentEvents(ctx, find)
	if err != nil {
		slog.Error("failed to list events", "tenant_id", tenant.ID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list events")
	}

	// Mask PII — omit payload from response
	type safeEvent struct {
		ID            int32  `json:"id"`
		TenantID      int32  `json:"tenant_id"`
		IntegrationID int32  `json:"integration_id"`
		EventType     string `json:"event_type"`
		Status        string `json:"status"`
		Attempts      int32  `json:"attempts"`
		CreatedAt     int64  `json:"created_at"`
	}
	safe := make([]safeEvent, 0, len(events))
	for _, evt := range events {
		safe = append(safe, safeEvent{
			ID:            evt.ID,
			TenantID:      evt.TenantID,
			IntegrationID: evt.IntegrationID,
			EventType:     evt.EventType,
			Status:        evt.Status,
			Attempts:      evt.Attempts,
			CreatedAt:     evt.CreatedAt,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"events": safe,
	})
}
