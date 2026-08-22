package docs

import (
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestRegister_SwaggerUI(t *testing.T) {
	e := echo.New()
	Register(e, []byte(`{"openapi":"3.1.2"}`))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api-docs", nil)
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
	for _, expected := range []string{
		"https://unpkg.com/swagger-ui-dist@5.32.11/swagger-ui.css",
		"https://unpkg.com/swagger-ui-dist@5.32.11/swagger-ui-bundle.js",
		`crossorigin="anonymous"`,
	} {
		if !strings.Contains(rec.Body.String(), expected) {
			t.Fatalf("Swagger UI HTML lacks %q", expected)
		}
	}
	assertIntegrity(t, rec.Body.String(), "sha384-9Q2fpS+xeS4ffJy6CagnwoUl+4ldAYhOs9pgZuEKxypVModhmZFzeMlvVsAjf7uT")
	assertIntegrity(t, rec.Body.String(), "sha384-vfl/klfTFrIz5urj0HnhcXLAbzPdRHezizfy+XgFB6GqcKkhlk0lS3bIbyB39NLA")
}

func assertIntegrity(t *testing.T, document, value string) {
	t.Helper()
	if !strings.Contains(document, `integrity="`+value+`"`) {
		t.Fatalf("Swagger UI HTML lacks integrity %q", value)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "sha384-"))
	if err != nil {
		t.Fatalf("decode integrity %q: %v", value, err)
	}
	if len(decoded) != sha512.Size384 {
		t.Fatalf("integrity %q decodes to %d bytes, want %d", value, len(decoded), sha512.Size384)
	}
}

func TestRegister_SwaggerInit(t *testing.T) {
	e := echo.New()
	Register(e, []byte(`{"openapi":"3.1.2"}`))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api-docs/swagger-init.js", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Fatalf("expected JavaScript content type, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "/openapi.json") {
		t.Fatal("expected swagger UI to reference /openapi.json")
	}
}

func TestRegister_OpenAPISpec(t *testing.T) {
	e := echo.New()
	Register(e, []byte(`{"openapi":"3.1.2"}`))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
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
