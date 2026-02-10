package docs

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v5"
)

//go:embed swagger-ui.html
var swaggerUI []byte

// Register wires documentation routes.
// - GET /api-docs/openapi.json serves the generated OpenAPI 3.1 spec.
// - GET /api-docs serves an embedded Swagger UI page.
func Register(e *echo.Echo, specPath string) {
	e.GET("/api-docs/openapi.json", func(c *echo.Context) error {
		return c.File(specPath)
	})

	e.GET("/api-docs", func(c *echo.Context) error {
		return c.HTMLBlob(http.StatusOK, swaggerUI)
	})
}
