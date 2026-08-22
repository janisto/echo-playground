package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestCORSDisabledByDefault(t *testing.T) {
	e := echo.New()
	e.Use(CORS(nil))
	e.GET("/test", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if value := rec.Header().Get("Access-Control-Allow-Origin"); value != "" {
		t.Fatalf("disabled CORS emitted an origin: %q", value)
	}
}

func TestCORSUsesExplicitOriginAndPortableFields(t *testing.T) {
	e := echo.New()
	e.Use(CORS([]string{"https://app.example"}))
	e.GET("/test", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	req.Header.Set("Access-Control-Request-Headers", "authorization,x-request-id,traceparent,tracestate")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if value := rec.Header().Get("Access-Control-Allow-Origin"); value != "https://app.example" {
		t.Fatalf("allow origin = %q", value)
	}
	if value := rec.Header().Get("Access-Control-Allow-Credentials"); value != "" {
		t.Fatalf("credentials must remain disabled, got %q", value)
	}
	allowHeaders := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers"))
	for _, name := range []string{"authorization", "x-request-id", "traceparent", "tracestate"} {
		if !strings.Contains(allowHeaders, name) {
			t.Fatalf("allow headers %q lacks %s", allowHeaders, name)
		}
	}
	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, http.MethodPatch) || strings.Contains(methods, http.MethodPut) {
		t.Fatalf("unexpected allow methods %q", methods)
	}
}

func TestCORSExposesPortableResponseFields(t *testing.T) {
	e := echo.New()
	e.Use(CORS([]string{"https://app.example"}))
	e.GET("/test", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	exposed := strings.ToLower(rec.Header().Get("Access-Control-Expose-Headers"))
	for _, name := range []string{"link", "location", "x-request-id", "retry-after", "x-ratelimit-reset"} {
		if !strings.Contains(exposed, name) {
			t.Fatalf("exposed headers %q lacks %s", exposed, name)
		}
	}
}
