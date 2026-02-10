package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestRegister_SwaggerUI(t *testing.T) {
	e := echo.New()
	Register(e, "testdata/swagger.json")

	req := httptest.NewRequest(http.MethodGet, "/api-docs", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "swagger-ui") {
		t.Fatal("expected swagger-ui content in response")
	}
}

func TestRegister_SwaggerUIContainsSpecURL(t *testing.T) {
	e := echo.New()
	Register(e, "api-docs/swagger.json")

	req := httptest.NewRequest(http.MethodGet, "/api-docs", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/api-docs/openapi.json") {
		t.Fatal("expected swagger UI to reference /api-docs/openapi.json")
	}
}

func TestRegister_OpenAPISpec(t *testing.T) {
	e := echo.New()
	Register(e, "testdata/swagger.json")

	req := httptest.NewRequest(http.MethodGet, "/api-docs/openapi.json", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "openapi") {
		t.Fatal("expected response to contain openapi spec content")
	}
}
