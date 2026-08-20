package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/janisto/echo-playground/api-docs"
	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/pagination"
	"github.com/janisto/echo-playground/internal/platform/respond"
	githubsvc "github.com/janisto/echo-playground/internal/service/github"
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
	const requestID = "auth-failure-request"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil)
	req.Header.Set(echo.HeaderXRequestID, requestID)
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
	if got := fields["request_id"]; got != requestID || rec.Header().Get(echo.HeaderXRequestID) != requestID {
		t.Fatalf("request ID correlation = %v/%q, want %q", got, rec.Header().Get(echo.HeaderXRequestID), requestID)
	}
	for _, key := range []string{"status", "error"} {
		if value, ok := fields[key]; ok {
			t.Fatalf("expected %s to be absent, got %v in %#v", key, value, fields)
		}
	}
}

func TestApplicationPortableRequestIDSelection(t *testing.T) {
	e := testApplication(t, time.Second)
	generated := regexp.MustCompile(`^[0-9a-f]{32}$`)
	tests := []struct {
		name      string
		values    []string
		want      string
		generated bool
	}{
		{name: "absent", generated: true},
		{name: "minimum", values: []string{"a"}, want: "a"},
		{name: "maximum", values: []string{"A" + strings.Repeat(".", 127)}, want: "A" + strings.Repeat(".", 127)},
		{name: "punctuation", values: []string{"A0._:-z"}, want: "A0._:-z"},
		{name: "whitespace", values: []string{"bad id"}, generated: true},
		{name: "unicode", values: []string{"café"}, generated: true},
		{name: "too long", values: []string{"a" + strings.Repeat("b", 128)}, generated: true},
		{name: "leading punctuation", values: []string{".bad"}, generated: true},
		{name: "comma combined", values: []string{"first,second"}, generated: true},
		{name: "duplicate fields", values: []string{"first", "second"}, generated: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
			for _, value := range test.values {
				req.Header.Add(echo.HeaderXRequestID, value)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			values := rec.Header().Values(echo.HeaderXRequestID)
			if rec.Code != http.StatusOK || len(values) != 1 {
				t.Fatalf("response = %d request IDs=%q", rec.Code, values)
			}
			if test.generated {
				if !generated.MatchString(values[0]) {
					t.Fatalf("replacement ID = %q", values[0])
				}
				for _, rejected := range test.values {
					if values[0] == rejected {
						t.Fatalf("reflected rejected ID %q", rejected)
					}
				}
			} else if values[0] != test.want {
				t.Fatalf("selected ID = %q, want %q", values[0], test.want)
			}
		})
	}
}

func TestApplicationRequestIDGenerationIsConcurrentAndUnique(t *testing.T) {
	e := testApplication(t, time.Second)
	const requests = 128
	ids := make(chan string, requests)
	var wait sync.WaitGroup
	ctx := t.Context()
	for range requests {
		wait.Go(func() {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code == http.StatusOK {
				ids <- rec.Header().Get(echo.HeaderXRequestID)
			}
		})
	}
	wait.Wait()
	close(ids)
	generated := regexp.MustCompile(`^[0-9a-f]{32}$`)
	seen := make(map[string]struct{}, requests)
	for id := range ids {
		if !generated.MatchString(id) {
			t.Fatalf("generated ID = %q", id)
		}
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("duplicate generated ID %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != requests {
		t.Fatalf("generated %d unique IDs, want %d", len(seen), requests)
	}
}

func TestApplicationRequestIDIsStableAcrossRepresentativeOutcomes(t *testing.T) {
	e := testApplication(t, time.Second)
	e.GET("/dependency", func(*echo.Context) error { return respond.DependencyUnavailable() })
	e.GET("/panic-id", func(*echo.Context) error { panic("request-id-panic-canary") })
	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		length      int64
		status      int
	}{
		{name: "success", method: http.MethodGet, path: "/health", status: 200},
		{name: "not-found", method: http.MethodGet, path: "/missing", status: 404},
		{name: "method", method: http.MethodDelete, path: "/health", status: 405},
		{
			name:        "body-limit",
			method:      http.MethodPost,
			path:        "/v1/hello",
			body:        `{}`,
			contentType: "application/json",
			length:      1_000_001,
			status:      413,
		},
		{name: "authentication", method: http.MethodGet, path: "/v1/profile", status: 401},
		{
			name:        "validation",
			method:      http.MethodPost,
			path:        "/v1/hello",
			body:        `{"name":""}`,
			contentType: "application/json",
			status:      422,
		},
		{name: "dependency", method: http.MethodGet, path: "/dependency", status: 503},
		{name: "panic", method: http.MethodGet, path: "/panic-id", status: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestID := "case-" + test.name
			req := httptest.NewRequestWithContext(t.Context(), test.method, test.path, strings.NewReader(test.body))
			req.Header.Set(echo.HeaderXRequestID, requestID)
			if test.contentType != "" {
				req.Header.Set(echo.HeaderContentType, test.contentType)
			}
			if test.length != 0 {
				req.ContentLength = test.length
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			values := rec.Header().Values(echo.HeaderXRequestID)
			if rec.Code != test.status || len(values) != 1 || values[0] != requestID {
				t.Fatalf("response = %d request IDs=%q, want %d/%q", rec.Code, values, test.status, requestID)
			}
		})
	}
}

func TestOpenAPIDiscoveryUsesEmbeddedLocalStateAndDoesNotReadBody(t *testing.T) {
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	e := newEcho(cfg, zap.NewNop(), &applicationClients{
		verifier: neverVerifier{}, profiles: neverProfileService{}, github: neverGitHubService{},
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
	req.Body = unreadBody{}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" ||
		rec.Body.String() != string(apidocs.OpenAPIJSON) {
		t.Fatalf("discovery = %d/%q bytes=%d", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}

	for _, test := range []struct {
		name   string
		method string
		path   string
		accept string
		status int
	}{
		{name: "query", method: http.MethodGet, path: "/openapi.json?unknown=1", status: 400},
		{name: "negotiation", method: http.MethodGet, path: "/openapi.json", accept: "application/cbor", status: 406},
		{name: "method", method: http.MethodPost, path: "/openapi.json", status: 405},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
			req.Body = unreadBody{}
			req.Header.Set("Accept", test.accept)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != test.status {
				t.Fatalf("response = %d: %s", rec.Code, rec.Body.String())
			}
			if test.status == 405 && !strings.Contains(rec.Header().Get("Allow"), "GET") {
				t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
			}
		})
	}
}

type unreadBody struct{}

func (unreadBody) Read([]byte) (int, error) { panic("discovery read request body") }
func (unreadBody) Close() error             { return nil }

type neverVerifier struct{}

func (neverVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	panic("discovery invoked authentication")
}

type neverProfileService struct{}

func (neverProfileService) Create(context.Context, string, profilesvc.CreateParams) (*profilesvc.Profile, error) {
	panic("discovery invoked profile persistence")
}

func (neverProfileService) Get(context.Context, string) (*profilesvc.Profile, error) {
	panic("discovery invoked profile persistence")
}

func (neverProfileService) Update(context.Context, string, profilesvc.UpdateParams) (*profilesvc.Profile, error) {
	panic("discovery invoked profile persistence")
}

func (neverProfileService) Delete(context.Context, string) error {
	panic("discovery invoked profile persistence")
}

type neverGitHubService struct{}

func (neverGitHubService) GetOwner(context.Context, string) (githubsvc.Owner, error) {
	panic("discovery invoked GitHub")
}

func (neverGitHubService) ListOwnerRepositories(
	context.Context,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.RepositorySummary], error) {
	panic("discovery invoked GitHub")
}

func (neverGitHubService) GetRepository(context.Context, string, string) (githubsvc.Repository, error) {
	panic("discovery invoked GitHub")
}

func (neverGitHubService) ListRepositoryActivity(
	context.Context,
	string,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.Activity], error) {
	panic("discovery invoked GitHub")
}

func (neverGitHubService) ListRepositoryLanguages(context.Context, string, string) ([]githubsvc.Language, error) {
	panic("discovery invoked GitHub")
}

func (neverGitHubService) ListRepositoryTags(
	context.Context,
	string,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.Tag], error) {
	panic("discovery invoked GitHub")
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
		{accept: "application/cbor", contentType: "application/cbor"},
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
