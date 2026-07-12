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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	expected := map[string]string{
		"Cache-Control":                "no-store",
		"Content-Security-Policy":      "default-src 'none'; frame-ancestors 'none'",
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

func TestSecurity_DocsPolicy(t *testing.T) {
	e := echo.New()
	e.Use(Security())
	e.GET("/api-docs", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api-docs", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp != contentSecurityPolicy("/api-docs") {
		t.Fatalf("unexpected docs CSP %q", csp)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("expected docs to retain no-store, got %q", cc)
	}
}

func TestSecurity_DoesNotTreatPrefixAsDocs(t *testing.T) {
	e := echo.New()
	e.Use(Security())
	e.GET("/api-docs-anything", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api-docs-anything", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if csp := rec.Header().Get("Content-Security-Policy"); csp != "default-src 'none'; frame-ancestors 'none'" {
		t.Fatalf("unexpected API CSP %q", csp)
	}
}
