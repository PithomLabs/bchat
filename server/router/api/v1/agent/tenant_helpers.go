package agent

import (
	"context"

	"github.com/labstack/echo/v4"

	"github.com/usememos/memos/store"
)

// tenantContextKey is the Echo context key for tenant ID.
// Must match v1.getTenantIDContextKey() in server/router/api/v1/ticket_service.go.
const tenantContextKey = "tenant-id"

// getTenantFromContext extracts the tenant ID from the echo context.
// Returns nil if no tenant is set.
func getTenantFromContext(c echo.Context) *int32 {
	if v, ok := c.Get(tenantContextKey).(int32); ok {
		return &v
	}
	return nil
}

// getTenantIDOrFail extracts the tenant ID from context and returns an error if not found.
// This is the recommended way for handlers to get tenant ID after TenantBindingMiddleware runs.
func getTenantIDOrFail(c echo.Context) (int32, error) {
	tenantID := getTenantFromContext(c)
	if tenantID == nil {
		return 0, echo.NewHTTPError(400, "tenant context not set - middleware may not be configured correctly")
	}
	return *tenantID, nil
}

// getTenantOrFail gets the full AgentTenant struct using the tenant ID from context.
// Use this when you need the full tenant object (CompanyName, GUID, etc).
func getTenantOrFail(ctx context.Context, s *store.Store, c echo.Context) (*store.AgentTenant, error) {
	tenantID, err := getTenantIDOrFail(c)
	if err != nil {
		return nil, err
	}
	return s.GetAgentTenant(ctx, &store.FindAgentTenant{ID: &tenantID})
}