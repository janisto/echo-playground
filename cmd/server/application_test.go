package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/respond"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
	testfake "github.com/janisto/echo-playground/internal/testutil/fake"
)

func testApplication(t *testing.T, timeout time.Duration) *echo.Echo {
	t.Helper()
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.RequestTimeout = timeout
	return testApplicationWithConfig(t, cfg)
}

func testApplicationWithConfig(t *testing.T, cfg config) *echo.Echo {
	t.Helper()
	return testApplicationWithConfigAndLogger(t, cfg, zap.NewNop())
}

func testApplicationWithConfigAndLogger(t *testing.T, cfg config, logger *zap.Logger) *echo.Echo {
	t.Helper()
	return newEcho(cfg, logger, &applicationClients{
		verifier: &testfake.MockVerifier{User: testfake.TestUser()},
		profiles: testfake.NewProfileStore(),
	})
}

func testApplicationWithObservedLogger(t *testing.T) (*echo.Echo, *observer.ObservedLogs) {
	t.Helper()
	core, recorded := observer.New(zapcore.DebugLevel)
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return testApplicationWithConfigAndLogger(t, cfg, zap.New(core)), recorded
}

func singleObservedEntry(t *testing.T, recorded *observer.ObservedLogs, message string) observer.LoggedEntry {
	t.Helper()
	entries := recorded.FilterMessage(message).All()
	if len(entries) != 1 {
		t.Fatalf("expected one %q log, got %d", message, len(entries))
	}
	return entries[0]
}

func TestApplicationRecoveryCoversLaterMiddleware(t *testing.T) {
	e, recorded := testApplicationWithObservedLogger(t)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if c.Request().URL.Path == "/panic" {
				panic("middleware panic")
			}
			return next(c)
		}
	})
	e.GET("/panic", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	var problem respond.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("unexpected problem: %#v", problem)
	}

	accessEntry := singleObservedEntry(t, recorded, "request completed")
	if accessEntry.Level != zapcore.ErrorLevel {
		t.Fatalf("expected ERROR access log, got %s", accessEntry.Level)
	}
	accessFields := accessEntry.ContextMap()
	if got := accessFields["terminal_reason"]; got != "panic" {
		t.Fatalf("expected terminal_reason=panic, got %v in %#v", got, accessFields)
	}
	if _, ok := accessFields["status"]; ok {
		t.Fatalf("uncommitted panic must not infer an access-log status: %#v", accessFields)
	}

	recoveryEntry := singleObservedEntry(t, recorded, "panic recovered")
	requestID := rec.Header().Get(echo.HeaderXRequestID)
	if requestID == "" {
		t.Fatal("expected generated response request ID")
	}
	if got := recoveryEntry.ContextMap()["request_id"]; got != requestID {
		t.Fatalf("expected recovery request_id=%q, got %v", requestID, got)
	}
}

func TestApplicationObservabilityUsesV2PrivacyDefaults(t *testing.T) {
	e, recorded := testApplicationWithObservedLogger(t)
	const (
		requestID = "request-123"
		traceID   = "4bf92f3577b34da6a3ce929d0e0e4736"
	)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/hello", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set(echo.HeaderXRequestID, requestID)
	req.Header.Set("Traceparent", "00-"+traceID+"-00f067aa0ba902b7-01")
	req.Header.Set("User-Agent", "privacy-canary")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	entry := singleObservedEntry(t, recorded, "request completed")
	fields := entry.ContextMap()
	want := map[string]any{
		"request_id":                           requestID,
		"correlation_id":                       traceID,
		"trace_id":                             traceID,
		"trace_sampled":                        true,
		"logging.googleapis.com/trace":         traceID,
		"logging.googleapis.com/trace_sampled": true,
		"status":                               int64(http.StatusOK),
		"path_template":                        "/v1/hello",
	}
	for key, expected := range want {
		if got := fields[key]; got != expected {
			t.Fatalf("expected %s=%v, got %v in %#v", key, expected, got, fields)
		}
	}
	for _, key := range []string{"path", "peer_ip", "remote_ip", "user_agent", "error", "terminal_reason"} {
		if value, ok := fields[key]; ok {
			t.Fatalf("expected %s to be absent, got %v in %#v", key, value, fields)
		}
	}
}

func TestApplicationObservabilityDoesNotGuessUncommittedErrorStatus(t *testing.T) {
	e, recorded := testApplicationWithObservedLogger(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected wire status 401, got %d: %s", rec.Code, rec.Body.String())
	}
	entry := singleObservedEntry(t, recorded, "request completed")
	if entry.Level != zapcore.ErrorLevel {
		t.Fatalf("expected ERROR access log, got %s", entry.Level)
	}
	fields := entry.ContextMap()
	if got := fields["terminal_reason"]; got != "service_error" {
		t.Fatalf("expected terminal_reason=service_error, got %v in %#v", got, fields)
	}
	if got := fields["path_template"]; got != "/v1/profile" {
		t.Fatalf("expected path_template=/v1/profile, got %v in %#v", got, fields)
	}
	for _, key := range []string{"status", "error"} {
		if value, ok := fields[key]; ok {
			t.Fatalf("expected %s to be absent, got %v in %#v", key, value, fields)
		}
	}
}

func TestApplicationRequestTimeout(t *testing.T) {
	e := testApplication(t, time.Millisecond)
	e.GET("/slow", func(c *echo.Context) error {
		<-c.Request().Context().Done()
		return c.Request().Context().Err()
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApplicationUsesDirectIPByDefault(t *testing.T) {
	e := testApplication(t, time.Second)
	e.GET("/ip", func(c *echo.Context) error {
		return c.String(http.StatusOK, c.RealIP())
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ip", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.5")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Body.String() != "192.0.2.10" {
		t.Fatalf("expected direct peer IP, got %q", rec.Body.String())
	}
}

func TestApplicationAutoHandlesHEADWithoutResponseBody(t *testing.T) {
	e := testApplication(t, time.Second)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected HEAD response without body, got %q", rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("expected GET content type to be preserved, got %q", contentType)
	}
}

func TestApplicationAutoHEADStillRunsAuthentication(t *testing.T) {
	for _, tt := range []struct {
		accept      string
		contentType string
	}{
		{contentType: "application/problem+json"},
		{accept: "application/cbor", contentType: "application/problem+cbor"},
	} {
		e := testApplication(t, time.Second)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/v1/profile", nil)
		req.Header.Set("Accept", tt.accept)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("Accept %q: expected 401, got %d", tt.accept, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("Accept %q: expected HEAD error without body, got %q", tt.accept, rec.Body.String())
		}
		if challenge := rec.Header().Get("WWW-Authenticate"); challenge != "Bearer" {
			t.Fatalf("Accept %q: expected authentication challenge, got %q", tt.accept, challenge)
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != tt.contentType {
			t.Fatalf("Accept %q: expected content type %q, got %q", tt.accept, tt.contentType, contentType)
		}
	}
}

func TestApplicationRejectsDuplicateRoutes(t *testing.T) {
	e := testApplication(t, time.Second)
	defer func() {
		if recover() == nil {
			t.Fatal("expected duplicate route registration to panic")
		}
	}()
	e.GET("/health", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
}

func TestApplicationUnknownVersionedRouteDoesNotRequireAuth(t *testing.T) {
	e := testApplication(t, time.Second)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/unknown", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected public 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApplicationRejectsOversizedBody(t *testing.T) {
	e := testApplication(t, time.Second)
	body := `{"name":"` + strings.Repeat("a", (1<<20)+1) + `"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/hello", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNewFirebaseClientsOfflineMode(t *testing.T) {
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	clients, err := newFirebaseClients(t.Context(), cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("create offline clients: %v", err)
	}
	if clients.Clients != nil {
		t.Fatal("offline mode unexpectedly initialized Firebase SDK clients")
	}
	_, err = clients.verifier.Verify(t.Context(), "token")
	if !errors.Is(err, auth.ErrAuthUnavailable) {
		t.Fatalf("expected ErrAuthUnavailable, got %v", err)
	}
	_, err = clients.profiles.Get(t.Context(), "user")
	if !errors.Is(err, profilesvc.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if err := clients.Close(); err != nil {
		t.Fatalf("close offline clients: %v", err)
	}
}
