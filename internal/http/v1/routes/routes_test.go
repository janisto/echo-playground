package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/http/health"
	"github.com/janisto/echo-playground/internal/platform/auth"
	applog "github.com/janisto/echo-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/echo-playground/internal/platform/middleware"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/validate"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

func setupTestServer(verifier auth.Verifier, svc profilesvc.Service) *echo.Echo {
	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(
		appmiddleware.RequestID(),
		applog.RequestLogger(),
		respond.Recoverer(),
	)

	e.GET("/health", health.Handler)

	v1 := e.Group("/v1")
	Register(v1, verifier, svc)
	return e
}

func TestHealthEndpoint(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body health.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Status != "healthy" {
		t.Fatalf("expected 'healthy', got %q", body.Status)
	}
}

func TestHelloGetEndpoint(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/hello", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHelloPostEndpoint(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	body := `{"name":"Integration"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestItemsEndpoint(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/items?limit=5", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	link := rec.Header().Get("Link")
	if link == "" {
		t.Fatal("expected Link header")
	}
}

func TestNotFoundReturns404(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var problem respond.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", problem.Status)
	}
	if problem.Title != "Not Found" {
		t.Fatalf("expected title 'Not Found', got %q", problem.Title)
	}
}

func TestMethodNotAllowedReturns405(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/hello", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	var problem respond.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", problem.Status)
	}
}

func TestRequestIDHeader(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", "test-trace-id")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	respID := rec.Header().Get("X-Request-ID")
	if respID != "test-trace-id" {
		t.Fatalf("expected X-Request-ID 'test-trace-id', got %q", respID)
	}
}

func TestProfileRequiresAuth(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestProfileCRUD(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	// Create.
	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phoneNumber":"+358401234567","marketing":true,"terms":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Get.
	req = httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", rec.Code)
	}

	// Update.
	req = httptest.NewRequest(http.MethodPatch, "/v1/profile", strings.NewReader(`{"firstname":"Jane"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", rec.Code)
	}

	// Delete.
	req = httptest.NewRequest(http.MethodDelete, "/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rec.Code)
	}
}

func TestPanicRecovery(t *testing.T) {
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	svc := profilesvc.NewMockStore()
	e := setupTestServer(verifier, svc)

	e.GET("/panic", func(_ *echo.Context) error {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var problem respond.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", problem.Status)
	}
}
