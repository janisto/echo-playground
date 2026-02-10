package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestVary_AddsAcceptHeader(t *testing.T) {
	e := echo.New()
	e.Use(Vary())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	vary := rec.Header().Get("Vary")
	if vary != "Accept" {
		t.Fatalf("expected Vary: Accept, got %q", vary)
	}
}

func TestVary_DoesNotDuplicateIfAlreadySet(t *testing.T) {
	e := echo.New()
	e.Use(Vary())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	values := rec.Header().Values("Vary")
	count := 0
	for _, v := range values {
		if v == "Accept" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected Vary: Accept once, got %d times", count)
	}
}
