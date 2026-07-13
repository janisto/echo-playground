package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestCORS_PreflightRequest(t *testing.T) {
	e := echo.New()
	e.Use(CORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "traceparent,tracestate")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", acao)
	}
	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, http.MethodPatch) || strings.Contains(methods, http.MethodPut) {
		t.Fatalf("unexpected allowed methods %q", methods)
	}
	if credentials := rec.Header().Get("Access-Control-Allow-Credentials"); credentials != "" {
		t.Fatalf("credentialed CORS must remain disabled, got %q", credentials)
	}
	allowedHeaders := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers"))
	if !strings.Contains(allowedHeaders, "traceparent") || !strings.Contains(allowedHeaders, "tracestate") {
		t.Fatalf("expected W3C trace headers, got %q", allowedHeaders)
	}
}

func TestCORS_SimpleRequest(t *testing.T) {
	e := echo.New()
	e.Use(CORS())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	exposed := rec.Header().Get("Access-Control-Expose-Headers")
	if exposed == "" {
		t.Fatal("expected Access-Control-Expose-Headers to be set")
	}
}
