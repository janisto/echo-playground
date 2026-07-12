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
)

func testApplication(t *testing.T, timeout time.Duration) *echo.Echo {
	t.Helper()
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.RequestTimeout = timeout
	return newEcho(cfg, zap.NewNop(), &applicationClients{
		verifier: &auth.MockVerifier{User: auth.TestUser()},
		profiles: profilesvc.NewMockStore(),
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
	if err := clients.Close(); err != nil {
		t.Fatalf("close offline clients: %v", err)
	}
}
