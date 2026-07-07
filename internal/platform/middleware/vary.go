package middleware

import (
	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/httpheader"
)

// Vary returns Echo middleware that adds Accept to the Vary header on all responses.
// Per RFC 9110 Section 12.5.5, the Vary header lists request headers
// that influence response selection. This API uses Accept for content negotiation
// to select JSON or CBOR format.
func Vary() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			httpheader.AddVary(c.Response().Header(), "Accept")
			return next(c)
		}
	}
}
