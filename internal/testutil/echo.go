package testutil

import (
	"github.com/labstack/echo/v5"

	appmiddleware "github.com/janisto/echo-playground/internal/platform/middleware"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/validate"
)

// NewTestEcho returns an Echo instance configured with the standard
// validator and HTTP error handler used by handler tests.
func NewTestEcho() *echo.Echo {
	e := echo.NewWithConfig(echo.Config{
		Router: echo.NewRouter(echo.RouterConfig{
			AllowOverwritingRoute:   false,
			AutoHandleHEAD:          true,
			MethodNotAllowedHandler: appmiddleware.MethodNotAllowed,
			OptionsMethodHandler:    appmiddleware.Options,
		}),
		HTTPErrorHandler:             respond.NewHTTPErrorHandler(),
		IPExtractor:                  echo.ExtractIPDirect(),
		Validator:                    validate.New(),
		NoGroupAutoRegister404Routes: true,
	})
	return e
}
