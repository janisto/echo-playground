package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestSecurity_SetsHeaders(t *testing.T) {
	e := echo.New()
	e.Use(Security())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	expected := map[string]string{
		"Cache-Control":                "no-store",
		"Content-Security-Policy":      "frame-ancestors 'none'",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Referrer-Policy":              "strict-origin-when-cross-origin",
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
	}

	for header, want := range expected {
		got := rec.Header().Get(header)
		if got != want {
			t.Errorf("header %q: expected %q, got %q", header, want, got)
		}
	}

	pp := rec.Header().Get("Permissions-Policy")
	if pp == "" {
		t.Error("expected Permissions-Policy header to be set")
	}
}

func TestSecurity_SkipPaths(t *testing.T) {
	e := echo.New()
	e.Use(Security("/v1/api-docs"))
	e.GET("/v1/api-docs/swagger.json", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/api-docs/swagger.json", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	cc := rec.Header().Get("Cache-Control")
	if cc != "" {
		t.Fatalf("expected no Cache-Control for skipped path, got %q", cc)
	}
}

func TestSecurity_NonSkipPath(t *testing.T) {
	e := echo.New()
	e.Use(Security("/v1/api-docs"))
	e.GET("/v1/hello", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/hello", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Fatalf("expected 'no-store' for non-skipped path, got %q", cc)
	}
}
