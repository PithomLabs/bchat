package agent

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/usememos/memos/store"
)

// ResolveSlugTenantMiddleware resolves the :slug URL parameter to a tenant ID
// and sets it in the Echo context. Used for public routes that don't require
// authentication but need tenant context (e.g., widget, external chat).
//
// This middleware is intentionally lightweight: no auth, no permission check.
// Downstream handlers must perform their own authorization checks.
func ResolveSlugTenantMiddleware(dbStore *store.Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			slug := c.Param("slug")
			if slug == "" {
				// Routes without :slug (e.g., /playground/catalog) skip resolution
				return next(c)
			}

			tenant, err := dbStore.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{Slug: &slug})
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve agent")
			}
			if tenant == nil {
				return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
			}
			if !tenant.IsActive {
				return echo.NewHTTPError(http.StatusNotFound, "Agent not found")
			}

			c.Set(tenantContextKey, tenant.ID)
			return next(c)
		}
	}
}
