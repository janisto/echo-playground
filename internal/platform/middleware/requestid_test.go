package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestRequestID_GeneratesUUID(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	reqID := rec.Header().Get(HeaderXRequestID)
	if reqID == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
	if len(reqID) != 36 {
		t.Fatalf("expected UUID format (36 chars), got %q (%d chars)", reqID, len(reqID))
	}
}

func TestRequestID_PreservesValid(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderXRequestID, "my-custom-id-123")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	reqID := rec.Header().Get(HeaderXRequestID)
	if reqID != "my-custom-id-123" {
		t.Fatalf("expected 'my-custom-id-123', got %q", reqID)
	}
}

func TestRequestID_RejectsEmpty(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderXRequestID, "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	reqID := rec.Header().Get(HeaderXRequestID)
	if reqID == "" {
		t.Fatal("expected generated UUID, got empty")
	}
}

func TestRequestID_RejectsTooLong(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	longID := strings.Repeat("a", 129)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderXRequestID, longID)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	reqID := rec.Header().Get(HeaderXRequestID)
	if reqID == longID {
		t.Fatal("expected long ID to be rejected")
	}
	if len(reqID) != 36 {
		t.Fatalf("expected UUID (36 chars), got %d chars", len(reqID))
	}
}

func TestRequestID_RejectsControlChars(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderXRequestID, "id-with-\n-newline")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	reqID := rec.Header().Get(HeaderXRequestID)
	if strings.Contains(reqID, "\n") {
		t.Fatal("expected control characters to be rejected")
	}
}

func TestRequestID_SetsInContext(t *testing.T) {
	e := echo.New()
	e.Use(RequestID())
	var ctxID string
	e.GET("/test", func(c *echo.Context) error {
		ctxID, _ = c.Get("request_id").(string)
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderXRequestID, "ctx-test-id")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if ctxID != "ctx-test-id" {
		t.Fatalf("expected context request_id 'ctx-test-id', got %q", ctxID)
	}
}

func TestIsValidRequestID(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid alphanumeric", "abc-123", true},
		{"valid UUID", "550e8400-e29b-41d4-a716-446655440000", true},
		{"empty", "", false},
		{"max length", strings.Repeat("x", 128), true},
		{"too long", strings.Repeat("x", 129), false},
		{"with space", "has space", true},
		{"with tab", "has\ttab", false},
		{"with newline", "has\nnewline", false},
		{"with null byte", "has\x00null", false},
		{"printable boundary lower", " ", true},
		{"printable boundary upper", "~", true},
		{"non-ascii", "caf\xc3\xa9", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidRequestID(tt.id)
			if got != tt.valid {
				t.Fatalf("isValidRequestID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}
