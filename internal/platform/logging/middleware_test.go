package logging

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestRequestLogger_EnrichesContext(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("request_id", "test-req-id")
			return next(c)
		}
	})
	e.Use(RequestLogger())

	var hasLogger bool
	e.GET("/test", func(c *echo.Context) error {
		l := LoggerFromContext(c.Request().Context())
		hasLogger = l != nil
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !hasLogger {
		t.Fatal("expected logger to be set in context")
	}
}

func TestAccessLogger_LogsRequest(t *testing.T) {
	e := echo.New()
	e.Use(RequestLogger())
	e.Use(AccessLogger())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequestLogger_TraceparentHeader(t *testing.T) {
	e := echo.New()
	e.Use(RequestLogger())

	e.GET("/test", func(c *echo.Context) error {
		_ = TraceIDFromContext(c.Request().Context())
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAccessLogger_ErrorPropagation(t *testing.T) {
	e := echo.New()
	e.Use(RequestLogger())
	e.Use(AccessLogger())
	e.GET("/error", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "bad")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Error should still be returned to client.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
