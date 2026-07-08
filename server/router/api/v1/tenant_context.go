package v1

import (
	"context"
	"log/slog"

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

// getUserFromContext extracts the user from the echo context.
// Returns nil if no user is set.
func getUserFromContext(c echo.Context) *store.User {
	if v, ok := c.Get("user").(*store.User); ok {
		return v
	}
	return nil
}

// deriveTenantIDsForScopedAdmin resolves a scoped admin's AllowedTenantIDs (GUIDs) to tenant IDs.
// Returns nil for super users (see all) or if no resolution is needed.
func deriveTenantIDsForScopedAdmin(ctx context.Context, s *store.Store, user *store.User) []int32 {
	if user == nil {
		return nil
	}
	// Super users see all tenants — no filter needed (delegates to store.IsSuperUser)
	if store.IsSuperUser(user) {
		return nil
	}
	// Scoped admin: resolve GUIDs to tenant IDs
	if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) > 0 {
		var tenantIDs []int32
		for _, guid := range user.AllowedTenantIDs {
			tenant, err := s.GetAgentTenant(ctx, &store.FindAgentTenant{GUID: &guid})
			if err != nil || tenant == nil {
				slog.Warn("failed to resolve tenant GUID for scoped admin filter",
					"guid", guid,
					"user_id", user.ID,
					"error", err,
				)
				continue
			}
			tenantIDs = append(tenantIDs, tenant.ID)
		}
		if len(tenantIDs) == 0 {
			// No valid tenants found — deny all by returning a sentinel value
			// that matches no real tenant ID, ensuring no results are returned.
			return []int32{-1}
		}
		return tenantIDs
	}
	// RoleUser: caller must handle via RBAC permission check (not filter-based)
	return nil
}

// ApplyTenantFilter applies tenant filtering to a FindMemo query.
// This is the defense-in-depth SQL safety net.
// For scoped admins, derives TenantIDs from AllowedTenantIDs.
func ApplyTenantFilter(c echo.Context, s *store.Store, find *store.FindMemo) {
	tenantID := getTenantFromContext(c)
	if tenantID != nil {
		find.TenantID = tenantID
		return
	}
	// H2 Part B: For scoped admins, derive filter from AllowedTenantIDs
	user := getUserFromContext(c)
	if user != nil {
		tenantIDs := deriveTenantIDsForScopedAdmin(c.Request().Context(), s, user)
		if tenantIDs != nil {
			find.TenantIDs = tenantIDs
		}
	}
}

// ApplyTicketTenantFilter applies tenant filtering to a FindTicket query.
// For scoped admins, derives TenantIDs from AllowedTenantIDs.
func ApplyTicketTenantFilter(c echo.Context, s *store.Store, find *store.FindTicket) {
	tenantID := getTenantFromContext(c)
	if tenantID != nil {
		find.TenantID = tenantID
		return
	}
	// H2 Part B: For scoped admins, derive filter from AllowedTenantIDs
	user := getUserFromContext(c)
	if user != nil {
		tenantIDs := deriveTenantIDsForScopedAdmin(c.Request().Context(), s, user)
		if tenantIDs != nil {
			find.TenantIDs = tenantIDs
		}
	}
}
