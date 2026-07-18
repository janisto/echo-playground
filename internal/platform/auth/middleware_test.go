package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/respond"
)

type MockVerifier struct {
	User  *FirebaseUser
	Error error
}

func (m *MockVerifier) Verify(context.Context, string) (*FirebaseUser, error) {
	return m.User, m.Error
}

func testUser() *FirebaseUser {
	return &FirebaseUser{UID: "test-user-123"}
}

func TestMiddleware_Success(t *testing.T) {
	user := testUser()
	verifier := &MockVerifier{User: user}

	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error {
		u, err := UserFromEchoContext(c)
		if err != nil {
			return respond.Error500("no user in context")
		}
		return c.JSON(http.StatusOK, map[string]string{"uid": u.UID})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body["uid"] != user.UID {
		t.Fatalf("expected uid %q, got %q", user.UID, body["uid"])
	}
}

func TestMiddleware_MissingAuthHeader(t *testing.T) {
	verifier := &MockVerifier{User: testUser()}

	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
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

func TestMiddleware_InvalidToken(t *testing.T) {
	verifier := &MockVerifier{Error: ErrInvalidToken}

	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	verifier := &MockVerifier{Error: ErrTokenExpired}

	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_CertificateFetchError(t *testing.T) {
	verifier := &MockVerifier{Error: ErrCertificateFetch}

	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
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

func TestMiddleware_AuthDependencyUnavailable(t *testing.T) {
	verifier := &MockVerifier{Error: ErrAuthUnavailable}
	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "30" {
		t.Fatalf("expected Retry-After 30, got %q", got)
	}
}

func TestMiddleware_RejectsMissingIdentity(t *testing.T) {
	for _, verifier := range []Verifier{
		&MockVerifier{},
		&MockVerifier{User: &FirebaseUser{}},
		&MockVerifier{User: &FirebaseUser{UID: "   "}},
		nil,
	} {
		e := echo.New()
		e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
		e.Use(Middleware(verifier))
		handled := false
		e.GET("/test", func(c *echo.Context) error {
			handled = true
			return c.NoContent(http.StatusNoContent)
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable || handled {
			t.Fatalf("expected fail-closed 503 without handler, got status=%d handled=%v", rec.Code, handled)
		}
		if got := rec.Header().Get("Retry-After"); got != "30" {
			t.Fatalf("expected Retry-After 30, got %q", got)
		}
	}
}

func TestMiddleware_BadBearerFormat(t *testing.T) {
	verifier := &MockVerifier{User: testUser()}

	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
		err    error
	}{
		{"valid", "Bearer my-token", "my-token", nil},
		{"case insensitive", "bearer my-token", "my-token", nil},
		{"empty", "", "", ErrNoToken},
		{"whitespace only", " \t", "", ErrInvalidToken},
		{"no bearer prefix", "Token abc", "", ErrInvalidToken},
		{"only bearer", "Bearer", "", ErrInvalidToken},
		{"empty token", "Bearer ", "", ErrInvalidToken},
		{"extra component", "Bearer token extra", "", ErrInvalidToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractBearerToken(tt.header)
			if !errors.Is(err, tt.err) {
				t.Fatalf("ExtractBearerToken(%q) error = %v, want %v", tt.header, err, tt.err)
			}
			if got != tt.want {
				t.Fatalf("ExtractBearerToken(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestCategorizeAuthError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{ErrTokenExpired, "token_expired"},
		{ErrTokenRevoked, "token_revoked"},
		{ErrUserDisabled, "user_disabled"},
		{ErrCertificateFetch, "certificate_fetch_failed"},
		{ErrAuthUnavailable, "dependency_unavailable"},
		{ErrInvalidToken, "invalid_token"},
		{ErrNoToken, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := categorizeAuthError(tt.err)
			if got != tt.want {
				t.Fatalf("categorizeAuthError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
