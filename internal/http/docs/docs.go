package docs

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v5"

	assets "github.com/janisto/echo-playground"
	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
)

//go:embed swagger-ui.html
var swaggerUI []byte

//go:embed swagger-init.js
var swaggerInit []byte

const (
	indexLink   = `</llms.txt>; rel="describedby"; type="text/plain"`
	catalogLink = `</.well-known/api-catalog>; rel="api-catalog"; type="application/linkset+json"`
	catalogType = `application/linkset+json; profile="https://www.rfc-editor.org/info/rfc9727"`
)

var catalog = []byte(`{
  "linkset": [{
    "anchor": "/",
    "service-desc": [{"href": "/openapi.json", "type": "application/json"}],
    "service-doc": [
      {"href": "/llms.txt", "type": "text/plain"},
      {"href": "/llms-full.txt", "type": "text/plain"}
    ],
    "status": [{"href": "/health", "type": "application/json"}]
  }]
}
`)

// Register wires the embedded schema, usage guides, API catalog, and Swagger UI.
func Register(e *echo.Echo, spec []byte) {
	e.GET("/openapi.json", func(c *echo.Context) error {
		if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
			return err
		}
		c.Response().Header().Add("Link", indexLink)
		return respond.JSONDocument(c, http.StatusOK, spec)
	}, respond.SuccessNegotiation(true))

	e.GET("/.well-known/api-catalog", document(catalogType, catalog, catalogLink, indexLink))
	e.GET("/llms.txt", document("text/plain; charset=utf-8", assets.UsageIndex))
	e.GET("/llms-full.txt", document("text/plain; charset=utf-8", assets.UsageGuide, indexLink))

	e.GET("/api-docs", func(c *echo.Context) error {
		c.Response().Header().Add("Link", indexLink)
		return c.HTMLBlob(http.StatusOK, swaggerUI)
	})

	e.GET("/api-docs/swagger-init.js", func(c *echo.Context) error {
		return c.Blob(http.StatusOK, "text/javascript; charset=UTF-8", swaggerInit)
	})
}

func document(contentType string, body []byte, links ...string) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
			return err
		}
		for _, link := range links {
			c.Response().Header().Add("Link", link)
		}
		return respond.Document(c, contentType, body)
	}
}
