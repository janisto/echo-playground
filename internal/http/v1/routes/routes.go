package routes

import (
	"github.com/labstack/echo/v5"

	githubhttp "github.com/janisto/echo-playground/internal/http/v1/github"
	"github.com/janisto/echo-playground/internal/http/v1/hello"
	"github.com/janisto/echo-playground/internal/http/v1/items"
	"github.com/janisto/echo-playground/internal/http/v1/profile"
	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/respond"
	githubsvc "github.com/janisto/echo-playground/internal/service/github"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

// Register wires all v1 routes into the provided group.
func Register(v1 *echo.Group, verifier auth.Verifier, svc profilesvc.Service, github githubsvc.Service) {
	hello.Register(v1)
	items.Register(v1)
	githubhttp.Register(v1, github)

	protected := v1.Group("", respond.SuccessNegotiation(false), auth.Middleware(verifier))
	profile.Register(protected, svc)
}
