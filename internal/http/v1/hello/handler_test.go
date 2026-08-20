package hello

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/testutil"
)

func setupEcho() *echo.Echo {
	e := testutil.NewTestEcho()
	Register(e.Group(""))
	return e
}

func TestGetHello(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", nil)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", nil)
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

func TestGetHello_RejectsUnsupportedSuccessRepresentations(t *testing.T) {
	for _, accept := range []string{"text/html", "application/problem+json"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		setupEcho().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotAcceptable {
			t.Fatalf("Accept %q: expected 406, got %d: %s", accept, rec.Code, rec.Body.String())
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != "application/problem+json" {
			t.Fatalf("Accept %q: expected JSON Problem Details fallback, got %q", accept, contentType)
		}
		var problem respond.ProblemDetails
		if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
			t.Fatalf("Accept %q: decode problem: %v", accept, err)
		}
		if problem.Status != http.StatusNotAcceptable {
			t.Fatalf("Accept %q: expected problem status 406, got %d", accept, problem.Status)
		}
	}
}

func TestCreateHello_Success(t *testing.T) {
	e := setupEcho()

	body := `{"name":"Alice"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(body))
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
	if problem.Errors[0].Source == nil || problem.Errors[0].Source.Pointer != "/name" {
		t.Fatalf("expected pointer '/name', got %#v", problem.Errors[0].Source)
	}
}

func TestCreateHello_NameTooLong(t *testing.T) {
	e := setupEcho()

	name := strings.Repeat("a", 101)
	body := `{"name":"` + name + `"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestCreateHello_InvalidJSON(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateHello_PreservesStreamedBodyLimitError(t *testing.T) {
	e := testutil.NewTestEcho()
	e.Use(middleware.BodyLimit(32))
	Register(e.Group(""))

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/hello",
		strings.NewReader(`{"name":"Ada"}`+strings.Repeat(" ", 32)),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.ContentLength = -1
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateHello_RejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, test := range []struct {
		body   string
		status int
	}{
		{body: `null`, status: http.StatusUnprocessableEntity},
		{body: `{"name":"Ada","unknown":true}`, status: http.StatusUnprocessableEntity},
		{body: `{"name":"Ada"} {}`, status: http.StatusBadRequest},
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(test.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		setupEcho().ServeHTTP(rec, req)
		if rec.Code != test.status {
			t.Fatalf("body %q: expected %d, got %d", test.body, test.status, rec.Code)
		}
	}
}

func TestCreateHello_CBOR(t *testing.T) {
	e := setupEcho()

	body := `{"name":"Bob"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hello", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if data.Message != "Hello, Bob!" {
		t.Fatalf("expected 'Hello, Bob!', got %q", data.Message)
	}
}
