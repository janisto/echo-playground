package health

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// Response is the payload for the health endpoint.
type Response struct {
	Status string `json:"status"`
}

// Handler is the health check endpoint.
func Handler(c *echo.Context) error {
	return c.JSON(http.StatusOK, Response{Status: "healthy"})
}
