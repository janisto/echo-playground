package middleware

import "github.com/labstack/echo/v5"

// Security returns Echo middleware that sets security headers on all responses.
// Headers follow OWASP REST Security Cheat Sheet recommendations (2025).
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
func Security() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			h := c.Response().Header()
			h.Set("Cache-Control", "no-store")
			h.Set("Content-Security-Policy", contentSecurityPolicy(c.Request().URL.Path))
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

func contentSecurityPolicy(path string) string {
	if path == "/api-docs" || path == "/api-docs/" || path == "/api-docs/swagger-init.js" {
		return "default-src 'none'; " +
			"connect-src 'self'; " +
			"img-src data:; " +
			"script-src 'self' https://unpkg.com; " +
			"style-src https://unpkg.com; " +
			"frame-ancestors 'none'"
	}
	return "default-src 'none'; frame-ancestors 'none'"
}
