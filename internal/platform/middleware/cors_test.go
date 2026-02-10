package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestCORS_PreflightRequest(t *testing.T) {
	e := echo.New()
	e.Use(CORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", acao)
	}
}

func TestCORS_SimpleRequest(t *testing.T) {
	e := echo.New()
	e.Use(CORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", acao)
	}
}

func TestCORS_ExposedHeaders(t *testing.T) {
	e := echo.New()
	e.Use(CORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	exposed := rec.Header().Get("Access-Control-Expose-Headers")
	if exposed == "" {
		t.Fatal("expected Access-Control-Expose-Headers to be set")
	}
}
