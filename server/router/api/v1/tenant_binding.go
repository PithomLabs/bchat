package v1

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/usememos/memos/store"
)

// TenantBindingMiddleware restricts all authenticated users to specific tenants.
// Super users (RoleHost, or RoleAdmin with empty AllowedTenantIDs) bypass this check.
// Scoped admins are checked against AllowedTenantIDs (GUIDs).
// RoleUser is checked via RBAC permission grants (ListUserTenantPermissions).
func TenantBindingMiddleware(s *store.Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Get(getUserIDContextKey()).(int32)
			if !ok {
				return next(c) // No user context, let auth middleware handle
			}

			user, err := s.GetUser(c.Request().Context(), &store.FindUser{ID: &userID})
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to verify tenant binding")
			}
			if user == nil {
				return echo.NewHTTPError(http.StatusForbidden, "access denied")
			}

			// Super users bypass tenant binding:
			// - RoleHost (instance owner/super-admin)
			// - RoleAdmin with empty AllowedTenantIDs (global admin)
			if user.Role == store.RoleHost || (user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0) {
				return next(c)
			}

			slug := c.Param("slug")
			if slug == "" {
				return next(c) // No slug in URL, skip check
				// NOTE: No-slug routes (e.g. HandleUpdateRoleTemplate) self-check
				// ownership from the record, so this is safe.
			}

			// Look up the tenant by slug
			tenant, err := s.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
			if err != nil || tenant == nil {
				return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
			}

			// Check if user has explicit grant for this tenant
			if user.Role == store.RoleAdmin {
				// Scoped admin: check AllowedTenantIDs (GUIDs)
				if !contains(user.AllowedTenantIDs, tenant.GUID) {
					return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
				}
			} else {
				// N2: Use existing ListUserTenantPermissions (not a new method)
				perms, err := s.ListUserTenantPermissions(c.Request().Context(), &store.FindUserTenantPermission{
					UserID:   &userID,
					TenantID: &tenant.ID,
				})
				if err != nil || len(perms) == 0 {
					return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
				}
			}

			return next(c)
		}
	}
}

// contains checks if a string slice contains a specific string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.TrimSpace(s) == item {
			return true
		}
	}
	return false
}
