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
	return newEcho(cfg, zap.NewNop(), &applicationClients{
		verifier: &testfake.MockVerifier{User: testfake.TestUser()},
		profiles: testfake.NewProfileStore(),
	})
}

func TestApplicationRecoveryCoversLaterMiddleware(t *testing.T) {
	e := testApplication(t, time.Second)
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

func TestApplicationXFFTrustsOnlyConfiguredProxyRanges(t *testing.T) {
	values := map[string]string{
		"IP_EXTRACTOR":        "xff",
		"TRUSTED_PROXY_CIDRS": "10.0.0.0/8",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	e := testApplicationWithConfig(t, cfg)
	e.GET("/ip", func(c *echo.Context) error {
		return c.String(http.StatusOK, c.RealIP())
	})

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{
			name:       "configured proxy stops at nearest untrusted hop",
			remoteAddr: "10.1.2.3:1234",
			xff:        "203.0.113.9, 198.51.100.5",
			want:       "198.51.100.5",
		},
		{
			name:       "configured proxy walks trusted internal hops",
			remoteAddr: "10.1.2.3:1234",
			xff:        "203.0.113.9, 10.2.3.4",
			want:       "203.0.113.9",
		},
		{
			name:       "untrusted direct peer cannot supply forwarding chain",
			remoteAddr: "192.0.2.10:1234",
			xff:        "198.51.100.5",
			want:       "192.0.2.10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ip", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Forwarded-For", tt.xff)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if got := rec.Body.String(); got != tt.want {
				t.Fatalf("expected client IP %q, got %q", tt.want, got)
			}
		})
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
