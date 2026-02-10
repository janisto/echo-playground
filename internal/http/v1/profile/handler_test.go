package profile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/validate"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

func setupEcho(verifier auth.Verifier, svc profilesvc.Service) *echo.Echo {
	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()

	g := e.Group("", auth.Middleware(verifier))
	Register(g, svc)
	return e
}

func validCreateBody() string {
	return `{"firstname":"John","lastname":"Doe","email":"john@example.com","phone_number":"+358401234567","marketing":true,"terms":true}`
}

func TestCreateProfile_Success(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if location != "/v1/profile" {
		t.Fatalf("expected Location '/v1/profile', got %q", location)
	}

	var p Profile
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if p.Firstname != "John" {
		t.Fatalf("expected firstname 'John', got %q", p.Firstname)
	}
	if p.Email != "john@example.com" {
		t.Fatalf("expected email 'john@example.com', got %q", p.Email)
	}
}

func TestCreateProfile_Duplicate(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	body := validCreateBody()

	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate create: expected 409, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProfile_ValidationError(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	body := `{"firstname":"","lastname":"","email":"bad","phone_number":"bad","terms":true}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var problem respond.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(problem.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestCreateProfile_TermsNotAccepted(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phone_number":"+358401234567","terms":false}`
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProfile_Unauthorized(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGetProfile_Success(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	// Create first.
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", rec.Code)
	}

	// Get.
	req = httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var p Profile
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if p.Firstname != "John" {
		t.Fatalf("expected firstname 'John', got %q", p.Firstname)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateProfile_Success(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	// Create first.
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", rec.Code)
	}

	// Update.
	body := `{"firstname":"Jane"}`
	req = httptest.NewRequest(http.MethodPatch, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var p Profile
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if p.Firstname != "Jane" {
		t.Fatalf("expected firstname 'Jane', got %q", p.Firstname)
	}
	if p.Lastname != "Doe" {
		t.Fatalf("expected lastname 'Doe' (unchanged), got %q", p.Lastname)
	}
}

func TestUpdateProfile_NotFound(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	body := `{"firstname":"Jane"}`
	req := httptest.NewRequest(http.MethodPatch, "/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteProfile_Success(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	// Create first.
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(validCreateBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", rec.Code)
	}

	// Delete.
	req = httptest.NewRequest(http.MethodDelete, "/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify deleted.
	req = httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestDeleteProfile_NotFound(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{User: auth.TestUser()}
	e := setupEcho(verifier, svc)

	req := httptest.NewRequest(http.MethodDelete, "/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProfile_InvalidToken(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{Error: auth.ErrInvalidToken}
	e := setupEcho(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth != "Bearer" {
		t.Fatalf("expected WWW-Authenticate: Bearer, got %q", wwwAuth)
	}
}

func TestProfile_CertificateFetchError(t *testing.T) {
	svc := profilesvc.NewMockStore()
	verifier := &auth.MockVerifier{Error: auth.ErrCertificateFetch}
	e := setupEcho(verifier, svc)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter != "30" {
		t.Fatalf("expected Retry-After: 30, got %q", retryAfter)
	}
}
