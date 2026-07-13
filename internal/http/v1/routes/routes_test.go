package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/janisto/echo-observability"
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/janisto/echo-playground/internal/http/health"
	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/respond"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
	"github.com/janisto/echo-playground/internal/testutil"
	testfake "github.com/janisto/echo-playground/internal/testutil/fake"
)

func setupTestServer(verifier auth.Verifier, svc profilesvc.Service) *echo.Echo {
	return setupTestServerWithLogger(verifier, svc, zap.NewNop())
}

func setupTestServerWithLogger(verifier auth.Verifier, svc profilesvc.Service, logger *zap.Logger) *echo.Echo {
	e := testutil.NewTestEcho()
	e.Use(
		obs.RequestContext(obs.RequestContextConfig{Logger: logger, Preset: obs.PresetGCP}),
		obs.AccessLogger(obs.AccessLoggerConfig{Logger: logger, Preset: obs.PresetGCP}),
		respond.Recoverer(logger),
	)

	e.GET("/health", health.Handler)

	v1 := e.Group("/v1")
	Register(v1, verifier, svc)
	return e
}

func TestHealthEndpoint(t *testing.T) {
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
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
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/hello", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHelloPostEndpoint(t *testing.T) {
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	body := `{"name":"Integration"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestItemsEndpoint(t *testing.T) {
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/items?limit=5", nil)
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
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nonexistent", nil)
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
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/hello", nil)
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
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
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

func TestInvalidRequestIDIsReplaced(t *testing.T) {
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	req.Header.Set(echo.HeaderXRequestID, "bad request id")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	responseID := rec.Header().Get(echo.HeaderXRequestID)
	if responseID == "" || responseID == "bad request id" {
		t.Fatalf("expected a generated request ID, got %q", responseID)
	}
	if !obs.DefaultValidateRequestID(responseID) {
		t.Fatalf("generated request ID is invalid: %q", responseID)
	}
}

func TestObservabilityCorrelatesHandlerAndAccessLogs(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServerWithLogger(verifier, svc, logger)

	const (
		requestID = "request-123"
		traceID   = "4bf92f3577b34da6a3ce929d0e0e4736"
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/hello", nil)
	req.Header.Set(echo.HeaderXRequestID, requestID)
	req.Header.Set("Traceparent", "00-"+traceID+"-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	assertObservedFields(t, recorded.FilterMessage("request completed").All(), map[string]any{
		"request_id":     requestID,
		"correlation_id": traceID,
		"trace_id":       traceID,
		"status":         int64(http.StatusOK),
		"path_template":  "/v1/hello",
	})
}

func TestProfileRequiresAuth(t *testing.T) {
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestProfileCRUD(t *testing.T) {
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServer(verifier, svc)

	// Create.
	body := `{"firstname":"John","lastname":"Doe","email":"john@example.com","phoneNumber":"+358401234567","marketing":true}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Get.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", rec.Code)
	}

	// Update.
	req = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPatch,
		"/v1/profile",
		strings.NewReader(`{"firstname":"Jane"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", rec.Code)
	}

	// Delete.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rec.Code)
	}
}

func TestPanicRecovery(t *testing.T) {
	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	verifier := &testfake.MockVerifier{User: testfake.TestUser()}
	svc := testfake.NewProfileStore()
	e := setupTestServerWithLogger(verifier, svc, logger)

	e.GET("/panic", func(_ *echo.Context) error {
		panic("test panic")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil)
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
	assertObservedFields(t, recorded.FilterMessage("panic recovered").All(), map[string]any{
		"request_id": rec.Header().Get(echo.HeaderXRequestID),
	})
	assertObservedFields(t, recorded.FilterMessage("request completed").All(), map[string]any{
		"status": int64(http.StatusInternalServerError),
	})
}

func assertObservedFields(t *testing.T, entries []observer.LoggedEntry, want map[string]any) {
	t.Helper()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	for key, expected := range want {
		if got := fields[key]; got != expected {
			t.Fatalf("expected %s=%v, got %v in %#v", key, expected, got, fields)
		}
	}
}
