package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/respond"
)

func TestMiddleware_Success(t *testing.T) {
	user := TestUser()
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

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
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
	verifier := &MockVerifier{User: TestUser()}

	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
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

func TestMiddleware_BadBearerFormat(t *testing.T) {
	verifier := &MockVerifier{User: TestUser()}

	e := echo.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.Use(Middleware(verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{"valid", "Bearer my-token", "my-token", false},
		{"case insensitive", "bearer my-token", "my-token", false},
		{"empty", "", "", true},
		{"no bearer prefix", "Token abc", "", true},
		{"only bearer", "Bearer", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractBearerToken(tt.header)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractBearerToken(%q): err=%v, wantErr=%v", tt.header, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ExtractBearerToken(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestUserFromContext_Standard(t *testing.T) {
	// Without context value, should return nil.
	ctx := context.Background()
	got := UserFromContext(ctx)
	if got != nil {
		t.Fatal("expected nil for context without user")
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
