package v1

import (
	"github.com/labstack/echo/v4"

	"github.com/usememos/memos/store"
)

// getTenantFromContext extracts the tenant ID from the echo context.
// Returns nil if no tenant is set (e.g., for admin users or legacy tokens).
func getTenantFromContext(c echo.Context) *int32 {
	if v, ok := c.Get(getTenantIDContextKey()).(int32); ok {
		return &v
	}
	return nil
}

// setTenantInContext sets the tenant ID in the echo context.
func setTenantInContext(c echo.Context, tenantID int32) {
	c.Set(getTenantIDContextKey(), tenantID)
}

// ApplyTenantFilter applies tenant filtering to a FindMemo query.
// This is the defense-in-depth SQL safety net.
func ApplyTenantFilter(c echo.Context, find *store.FindMemo) {
	tenantID := getTenantFromContext(c)
	if tenantID != nil {
		find.TenantID = tenantID
	}
}

// ApplyTicketTenantFilter applies tenant filtering to a FindTicket query.
func ApplyTicketTenantFilter(c echo.Context, find *store.FindTicket) {
	tenantID := getTenantFromContext(c)
	if tenantID != nil {
		find.TenantID = tenantID
	}
}
