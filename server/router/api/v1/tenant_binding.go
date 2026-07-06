package v1

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/usememos/memos/store"
)

// TenantBindingMiddleware restricts admin users to specific tenants based on their AllowedTenantIDs.
// Super users (empty AllowedTenantIDs) bypass this check.
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

			// Super users bypass tenant binding
			if user.Role == store.RoleAdmin && len(user.AllowedTenantIDs) == 0 {
				return next(c)
			}

			// Non-admin users don't need tenant binding (they're scoped differently)
			if user.Role != store.RoleAdmin {
				return next(c)
			}

			slug := c.Param("slug")
			if slug == "" {
				return next(c) // No slug in URL, skip check
			}

			// Look up the tenant by slug
			tenant, err := s.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
			if err != nil || tenant == nil {
				return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
			}

			// Check if user has access to this tenant
			if !contains(user.AllowedTenantIDs, tenant.GUID) {
				return echo.NewHTTPError(http.StatusForbidden, "access denied to this tenant")
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
