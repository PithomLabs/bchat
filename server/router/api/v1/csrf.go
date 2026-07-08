package v1

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

// CSRFProtectionMiddleware provides CSRF protection using SameSite=Lax cookies
// and Sec-Fetch-Site header validation.
// N5: Uses Sec-Fetch-Site when available (more reliable than Origin).
// M4: Skips Bearer/PAT requests (no cookie = no CSRF risk).
func CSRFProtectionMiddleware(allowedOrigins []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip safe methods (Idempotent requests)
			if c.Request().Method == "GET" || c.Request().Method == "HEAD" || c.Request().Method == "OPTIONS" {
				return next(c)
			}

			// M4: Skip CSRF check for Bearer/PAT requests — no cookie = no CSRF risk
			authHeader := c.Request().Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				return next(c)
			}

			// Prefer Sec-Fetch-Site when available (more reliable than Origin)
			secFetchSite := c.Request().Header.Get("Sec-Fetch-Site")
			if secFetchSite != "" {
				// "cross-site" from different origin = CSRF
				if secFetchSite == "cross-site" {
					return echo.NewHTTPError(http.StatusForbidden, "CSRF validation failed: cross-site request")
				}
				// "same-origin", "same-site", and "none" (from same origin) are safe
				return next(c)
			}

			// Fallback: check Origin header
			origin := c.Request().Header.Get("Origin")
			if origin == "" {
				// N5: Treat missing Origin conservatively for state-changing methods.
				// SameSite=Lax already blocks cross-site POST, so this is safe.
				// Log for audit trail — non-browser clients may omit these headers.
				slog.Debug("CSRF: missing both Origin and Sec-Fetch-Site, relying on SameSite=Lax",
					"method", c.Request().Method,
					"path", c.Request().URL.Path,
				)
				return next(c)
			}

			// Validate origin against allowlist
			if !isAllowedOrigin(origin, allowedOrigins) {
				return echo.NewHTTPError(http.StatusForbidden, "CSRF validation failed: invalid origin")
			}

			return next(c)
		}
	}
}

// isAllowedOrigin checks if the given origin is in the allowlist.
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		// No allowlist configured — deny all origins (safer default)
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := parsed.Hostname()
	for _, allowed := range allowedOrigins {
		if originHost == allowed {
			return true
		}
	}
	return false
}
