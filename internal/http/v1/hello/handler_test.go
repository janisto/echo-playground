package hello

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/validate"
)

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	Register(e.Group(""))
	return e
}

func TestGetHello(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var data Data
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if data.Message != "Hello, World!" {
		t.Fatalf("expected 'Hello, World!', got %q", data.Message)
	}
}

func TestGetHello_CBOR(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("expected application/cbor, got %q", ct)
	}

	var data Data
	if err := cbor.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if data.Message != "Hello, World!" {
		t.Fatalf("expected 'Hello, World!', got %q", data.Message)
	}
}

func TestCreateHello_Success(t *testing.T) {
	e := setupEcho()

	body := `{"name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var data Data
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if data.Message != "Hello, Alice!" {
		t.Fatalf("expected 'Hello, Alice!', got %q", data.Message)
	}
}

func TestCreateHello_MissingName(t *testing.T) {
	e := setupEcho()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	var problem respond.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", problem.Status)
	}
	if len(problem.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
	if problem.Errors[0].Location != "name" {
		t.Fatalf("expected location 'name', got %q", problem.Errors[0].Location)
	}
}

func TestCreateHello_NameTooLong(t *testing.T) {
	e := setupEcho()

	name := strings.Repeat("a", 101)
	body := `{"name":"` + name + `"}`
	req := httptest.NewRequest(http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestCreateHello_InvalidJSON(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodPost, "/hello", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateHello_CBOR(t *testing.T) {
	e := setupEcho()

	body := `{"name":"Bob"}`
	req := httptest.NewRequest(http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("expected application/cbor, got %q", ct)
	}

	var data Data
	if err := cbor.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if data.Message != "Hello, Bob!" {
		t.Fatalf("expected 'Hello, Bob!', got %q", data.Message)
	}
}
