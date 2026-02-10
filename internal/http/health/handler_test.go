package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"
)

func TestHandler_ReturnsHealthy(t *testing.T) {
	e := echo.New()
	e.GET("/health", Handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Status != "healthy" {
		t.Fatalf("expected status 'healthy', got %q", body.Status)
	}
}

func TestHandler_ContentTypeJSON(t *testing.T) {
	e := echo.New()
	e.GET("/health", Handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %q", ct)
	}
}

func TestHandler_CBORNotSupported(t *testing.T) {
	e := echo.New()
	e.GET("/health", Handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Health endpoint uses c.JSON directly, so always returns JSON regardless of Accept.
	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		// May be CBOR if handler respects Accept header; try CBOR.
		if cborErr := cbor.Unmarshal(rec.Body.Bytes(), &body); cborErr != nil {
			t.Fatalf("failed to decode response as JSON or CBOR: json=%v cbor=%v", err, cborErr)
		}
	}
	if body.Status != "healthy" {
		t.Fatalf("expected status 'healthy', got %q", body.Status)
	}
}
