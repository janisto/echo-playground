package routes

import (
	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/http/v1/hello"
	"github.com/janisto/echo-playground/internal/http/v1/items"
	"github.com/janisto/echo-playground/internal/http/v1/profile"
	"github.com/janisto/echo-playground/internal/platform/auth"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

// Register wires all v1 routes into the provided group.
func Register(v1 *echo.Group, verifier auth.Verifier, svc profilesvc.Service) {
	hello.Register(v1)
	items.Register(v1)

	protected := v1.Group("", auth.Middleware(verifier))
	profile.Register(protected, svc)
}
