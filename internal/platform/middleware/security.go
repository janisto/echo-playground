package middleware

import (
	"strings"

	"github.com/labstack/echo/v5"
)

// Security returns Echo middleware that sets security headers on all responses.
// Headers follow OWASP REST Security Cheat Sheet recommendations (2025).
//
// Paths in skipPaths are excluded from security headers (e.g., "/v1/api-docs").
//
// Headers set:
//   - Cache-Control: no-store
//   - Content-Security-Policy: frame-ancestors 'none'
//   - Cross-Origin-Opener-Policy: same-origin
//   - Cross-Origin-Resource-Policy: same-origin
//   - Permissions-Policy: disables browser features not needed by REST APIs
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - X-Content-Type-Options: nosniff
//   - X-Frame-Options: DENY
func Security(skipPaths ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			for _, p := range skipPaths {
				if strings.HasPrefix(c.Request().URL.Path, p) {
					return next(c)
				}
			}

			h := c.Response().Header()
			h.Set("Cache-Control", "no-store")
			h.Set("Content-Security-Policy", "frame-ancestors 'none'")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			h.Set(
				"Permissions-Policy",
				"accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
			)
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")

			return next(c)
		}
	}
}
