package health

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/respond"
)

// Response is the payload for the health endpoint.
type Response struct {
	Status string `json:"status" cbor:"status" example:"healthy"`
}

// Handler is the health check endpoint.
func Handler(c *echo.Context) error {
	return respond.Negotiate(c, http.StatusOK, Response{Status: "healthy"})
}
