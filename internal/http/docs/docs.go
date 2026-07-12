package docs

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v5"
)

//go:embed swagger-ui.html
var swaggerUI []byte

//go:embed swagger-init.js
var swaggerInit []byte

// Register wires documentation routes.
// - GET /api-docs/openapi.json serves the generated OpenAPI 3.1 spec.
// - GET /api-docs serves an embedded Swagger UI page.
func Register(e *echo.Echo, spec []byte) {
	e.GET("/api-docs/openapi.json", func(c *echo.Context) error {
		return c.Blob(http.StatusOK, "application/json; charset=UTF-8", spec)
	})

	e.GET("/api-docs", func(c *echo.Context) error {
		return c.HTMLBlob(http.StatusOK, swaggerUI)
	})

	e.GET("/api-docs/swagger-init.js", func(c *echo.Context) error {
		return c.Blob(http.StatusOK, "text/javascript; charset=UTF-8", swaggerInit)
	})
}
