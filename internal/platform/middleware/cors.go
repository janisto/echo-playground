package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// CORS returns deployment-configured CORS middleware. An empty allowlist keeps
// browser cross-origin access disabled.
func CORS(origins []string) echo.MiddlewareFunc {
	if len(origins) == 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: origins,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Request-ID",
			"Traceparent",
			"Tracestate",
		},
		ExposeHeaders: []string{
			"Link",
			"Location",
			"X-Request-ID",
			"Retry-After",
			"X-RateLimit-Reset",
		},
		MaxAge: 300,
	})
}
