package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestVary_AddsAcceptHeader(t *testing.T) {
	e := echo.New()
	e.Use(Vary())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	vary := rec.Header().Get("Vary")
	if vary != "Accept" {
		t.Fatalf("expected Vary: Accept, got %q", vary)
	}
}

func TestVary_DoesNotDuplicateIfAlreadySet(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Response().Header().Add("Vary", "accept")
			return next(c)
		}
	})
	e.Use(Vary())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	values := rec.Header().Values("Vary")
	count := 0
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), "Accept") {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected Vary: Accept once, got %d times", count)
	}
}
