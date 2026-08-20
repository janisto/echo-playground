package docs

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
)

//go:embed swagger-ui.html
var swaggerUI []byte

//go:embed swagger-init.js
var swaggerInit []byte

// Register wires documentation routes.
// - GET /openapi.json serves the generated OpenAPI 3.1 spec.
// - GET /api-docs serves an embedded Swagger UI page.
func Register(e *echo.Echo, spec []byte) {
	e.GET("/openapi.json", func(c *echo.Context) error {
		if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
			return err
		}
		return respond.JSONDocument(c, http.StatusOK, spec)
	}, respond.SuccessNegotiation(true))

	e.GET("/api-docs", func(c *echo.Context) error {
		return c.HTMLBlob(http.StatusOK, swaggerUI)
	})

	e.GET("/api-docs/swagger-init.js", func(c *echo.Context) error {
		return c.Blob(http.StatusOK, "text/javascript; charset=UTF-8", swaggerInit)
	})
}
