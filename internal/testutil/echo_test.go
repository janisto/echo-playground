package testutil

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestNewTestEchoRejectsDuplicateRoutes(t *testing.T) {
	e := NewTestEcho()
	handler := func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) }
	e.GET("/duplicate", handler)

	defer func() {
		if recover() == nil {
			t.Fatal("expected duplicate route registration to panic")
		}
	}()
	e.GET("/duplicate", handler)
}
