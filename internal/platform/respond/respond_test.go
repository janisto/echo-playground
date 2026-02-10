package respond

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/validate"
)

// --- ProblemDetails constructors ---

func TestNewError(t *testing.T) {
	p := NewError(http.StatusTeapot, "custom message")
	if p.Type != "about:blank" {
		t.Fatalf("expected about:blank, got %q", p.Type)
	}
	if p.Title != http.StatusText(http.StatusTeapot) {
		t.Fatalf("expected %q, got %q", http.StatusText(http.StatusTeapot), p.Title)
	}
	if p.Status != http.StatusTeapot {
		t.Fatalf("expected %d, got %d", http.StatusTeapot, p.Status)
	}
	if p.Detail != "custom message" {
		t.Fatalf("expected 'custom message', got %q", p.Detail)
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name   string
		fn     func(string) *ProblemDetails
		status int
	}{
		{"Error400", Error400, http.StatusBadRequest},
		{"Error401", Error401, http.StatusUnauthorized},
		{"Error403", Error403, http.StatusForbidden},
		{"Error404", Error404, http.StatusNotFound},
		{"Error409", Error409, http.StatusConflict},
		{"Error500", Error500, http.StatusInternalServerError},
		{"Error503", Error503, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.fn("detail")
			if p.Status != tt.status {
				t.Fatalf("expected status %d, got %d", tt.status, p.Status)
			}
			if p.Title != http.StatusText(tt.status) {
				t.Fatalf("expected title %q, got %q", http.StatusText(tt.status), p.Title)
			}
			if p.Detail != "detail" {
				t.Fatalf("expected detail 'detail', got %q", p.Detail)
			}
			if p.Type != "about:blank" {
				t.Fatalf("expected type about:blank, got %q", p.Type)
			}
		})
	}
}

func TestError422WithFields(t *testing.T) {
	fields := []ErrorDetail{
		{Message: "name is required", Location: "name"},
		{Message: "email must be valid", Location: "email", Value: "bad"},
	}
	p := Error422("validation failed", fields...)
	if p.Status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", p.Status)
	}
	if len(p.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(p.Errors))
	}
	if p.Errors[0].Message != "name is required" {
		t.Fatalf("unexpected error message: %s", p.Errors[0].Message)
	}
}

func TestError422WithoutFields(t *testing.T) {
	p := Error422("validation failed")
	if len(p.Errors) != 0 {
		t.Fatalf("expected 0 errors, got %d", len(p.Errors))
	}
}

func TestProblemDetailsError(t *testing.T) {
	p := Error404("resource not found")
	if p.Error() != "404 Not Found: resource not found" {
		t.Fatalf("unexpected Error(): %s", p.Error())
	}

	p2 := &ProblemDetails{Type: "about:blank", Title: "Not Found", Status: 404}
	if p2.Error() != "404 Not Found" {
		t.Fatalf("unexpected Error() without detail: %s", p2.Error())
	}
}

func TestProblemDetailsStatusCode(t *testing.T) {
	p := Error400("bad")
	if p.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", p.StatusCode())
	}
}

func TestProblemDetailsImplementsError(t *testing.T) {
	var err error = Error404("not found")
	var pd *ProblemDetails
	if !errors.As(err, &pd) {
		t.Fatal("expected ProblemDetails to be extractable via errors.As")
	}
	if pd.Status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", pd.Status)
	}
}

// --- parseAccept ---

func TestParseAcceptEmpty(t *testing.T) {
	ranges := parseAccept("")
	if ranges != nil {
		t.Fatalf("expected nil for empty header, got %v", ranges)
	}
}

func TestParseAcceptNoSlash(t *testing.T) {
	ranges := parseAccept("text")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].typ != "text" || ranges[0].subtype != "*" {
		t.Fatalf("expected text/*, got %s/%s", ranges[0].typ, ranges[0].subtype)
	}
}

func TestParseAcceptEmptyPart(t *testing.T) {
	ranges := parseAccept("application/json, , text/html")
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}
}

func TestParseAcceptInvalidQValue(t *testing.T) {
	ranges := parseAccept("application/json;q=invalid")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].q != 1.0 {
		t.Fatalf("expected q=1.0 for invalid q value, got %f", ranges[0].q)
	}
}

func TestParseAcceptQValueOutOfRange(t *testing.T) {
	ranges := parseAccept("application/json;q=2.0")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].q != 1.0 {
		t.Fatalf("expected q=1.0 for out-of-range q value, got %f", ranges[0].q)
	}

	ranges = parseAccept("application/json;q=-0.5")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].q != 1.0 {
		t.Fatalf("expected q=1.0 for negative q value, got %f", ranges[0].q)
	}
}

func TestParseAcceptMultipleQParams(t *testing.T) {
	ranges := parseAccept("application/json;q=0.5;q=0.9")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].q != 0.9 {
		t.Fatalf("expected last q value (0.9), got %f", ranges[0].q)
	}
}

// --- selectFormat ---

func TestSelectFormatEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		accept     string
		expectCBOR bool
	}{
		{"empty accept defaults to JSON", "", false},
		{"wildcard defaults to JSON", "*/*", false},
		{"application wildcard defaults to JSON", "application/*", false},
		{"explicit JSON", "application/json", false},
		{"explicit CBOR", "application/cbor", true},
		{"CBOR with quality parameter", "application/cbor;q=1.0", true},
		{"multiple types with equal q-values defaults to JSON", "application/json, application/cbor", false},
		{"CBOR preferred with quality", "application/json;q=0.9, application/cbor;q=1.0", true},
		{"text/html defaults to JSON", "text/html", false},
		{"problem+cbor explicit", "application/problem+cbor", true},
		{"problem+json explicit", "application/problem+json", false},
		{
			"problem+cbor preferred over problem+json",
			"application/problem+cbor;q=1.0, application/problem+json;q=0.5",
			true,
		},
		{
			"problem+json preferred over problem+cbor",
			"application/problem+cbor;q=0.5, application/problem+json;q=1.0",
			false,
		},
		{"problem+cbor over base cbor same q", "application/cbor, application/problem+cbor", true},
		{"CBOR excluded with q=0", "application/cbor;q=0, application/json", false},
		{"JSON preferred with higher quality", "application/cbor;q=0.5, application/json;q=0.9", false},
		{"CBOR only with low quality still accepted", "application/cbor;q=0.1", true},
		{"wildcard with CBOR explicit prefers CBOR", "*/*;q=0.1, application/cbor;q=1.0", true},
		{"wildcard with JSON explicit prefers JSON", "*/*;q=0.1, application/json;q=1.0", false},
		{
			"q-value wins over specificity - JSON base over CBOR problem",
			"application/problem+cbor;q=0.1, application/json;q=1.0",
			false,
		},
		{
			"q-value wins over specificity - CBOR base over JSON problem",
			"application/problem+json;q=0.1, application/cbor;q=1.0",
			true,
		},
		{
			"equal q-values use specificity as tie-breaker - CBOR wins",
			"application/json;q=0.8, application/problem+cbor;q=0.8",
			true,
		},
		{
			"equal q-values use specificity as tie-breaker - JSON wins",
			"application/cbor;q=0.8, application/problem+json;q=0.8",
			false,
		},
		{"malformed quality defaults to 1.0", "application/cbor;q=invalid", true},
		{"whitespace handling", "  application/cbor  ;  q=1.0  ", true},
		{"case insensitive type matching", "Application/CBOR", true},
		{"both excluded with q=0", "application/json;q=0, application/cbor;q=0", false},
		{"only wildcard with q=0", "*/*;q=0", false},
		{"structured suffix wildcard +cbor", "application/*+cbor", true},
		{"structured suffix wildcard +json", "application/*+json", false},
		{"no matching type", "image/png, text/plain", false},
		{"CBOR excluded JSON accepted", "application/cbor;q=0, application/json;q=1.0", false},
		{"JSON excluded CBOR accepted", "application/json;q=0, application/cbor;q=1.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectFormat(tt.accept)
			if got != tt.expectCBOR {
				t.Fatalf("selectFormat(%q) = %v, want %v", tt.accept, got, tt.expectCBOR)
			}
		})
	}
}

// --- ensureVary ---

func TestEnsureVaryAddsValues(t *testing.T) {
	h := make(http.Header)
	ensureVary(h, "Origin", "Accept")
	values := h.Values("Vary")
	set := headerSet(values)
	if _, ok := set["Origin"]; !ok {
		t.Fatal("expected Vary to contain Origin")
	}
	if _, ok := set["Accept"]; !ok {
		t.Fatal("expected Vary to contain Accept")
	}
}

func TestEnsureVaryNoDuplicates(t *testing.T) {
	h := make(http.Header)
	h.Add("Vary", "Accept")
	ensureVary(h, "Accept", "Origin")
	count := countInHeader(h.Values("Vary"), "Accept")
	if count != 1 {
		t.Fatalf("expected Accept once, got %d", count)
	}
}

func TestEnsureVaryMergesCommaSeparated(t *testing.T) {
	h := make(http.Header)
	h.Set("Vary", "Accept-Encoding, Accept-Language")
	ensureVary(h, "Origin", "Accept")
	set := headerSet(h.Values("Vary"))
	for _, v := range []string{"Accept-Encoding", "Accept-Language", "Origin", "Accept"} {
		if _, ok := set[v]; !ok {
			t.Fatalf("expected Vary to contain %q", v)
		}
	}
}

func TestEnsureVaryEmptyInput(t *testing.T) {
	h := make(http.Header)
	ensureVary(h)
	if len(h.Values("Vary")) != 0 {
		t.Fatalf("expected no Vary header, got %v", h.Values("Vary"))
	}
}

func TestEnsureVaryDuplicateInSingleCall(t *testing.T) {
	h := make(http.Header)
	ensureVary(h, "Accept", "Accept", "Origin")
	count := countInHeader(h.Values("Vary"), "Accept")
	if count != 1 {
		t.Fatalf("expected Accept once, got %d", count)
	}
}

// --- writeProblem ---

func TestWriteProblemJSON(t *testing.T) {
	problem := ProblemDetails{
		Type:   "about:blank",
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: "resource not found",
	}
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	writeProblem(rec, req, problem)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}

	var got ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if got.Status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", got.Status)
	}
	if got.Detail != "resource not found" {
		t.Fatalf("expected detail 'resource not found', got %q", got.Detail)
	}
}

func TestWriteProblemCBOR(t *testing.T) {
	problem := ProblemDetails{
		Type:   "about:blank",
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: "resource not found",
	}
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()

	writeProblem(rec, req, problem)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var got ProblemDetails
	if err := cbor.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if got.Status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", got.Status)
	}
}

func TestWriteProblemVaryHeaders(t *testing.T) {
	problem := ProblemDetails{Type: "about:blank", Title: "Not Found", Status: 404}
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	writeProblem(rec, req, problem)

	set := headerSet(rec.Header().Values("Vary"))
	if _, ok := set["Origin"]; !ok {
		t.Fatal("expected Vary to contain Origin")
	}
	if _, ok := set["Accept"]; !ok {
		t.Fatal("expected Vary to contain Accept")
	}
}

func TestWriteProblemNoHTMLEscaping(t *testing.T) {
	problem := ProblemDetails{
		Type:   "about:blank",
		Title:  "Bad Request",
		Status: 400,
		Detail: "param foo=<bar>&baz",
	}
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	writeProblem(rec, req, problem)

	body := rec.Body.String()
	if strings.Contains(body, `\u003c`) || strings.Contains(body, `\u003e`) || strings.Contains(body, `\u0026`) {
		t.Fatalf("should not contain HTML-escaped characters: %s", body)
	}
}

// failWriter is an http.ResponseWriter that accepts WriteHeader but fails on Write.
type failWriter struct {
	header http.Header
	status int
}

func (w *failWriter) Header() http.Header        { return w.header }
func (w *failWriter) WriteHeader(statusCode int) { w.status = statusCode }
func (w *failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteProblem_JSONEncodeError(t *testing.T) {
	problem := ProblemDetails{
		Type:   "about:blank",
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: "test",
	}
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := &failWriter{header: make(http.Header)}

	writeProblem(w, req, problem)

	if w.status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.status)
	}
}

func TestWriteProblem_CBOREncodeError(t *testing.T) {
	problem := ProblemDetails{
		Type:   "about:blank",
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: "test",
	}
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "application/cbor")
	w := &failWriter{header: make(http.Header)}

	writeProblem(w, req, problem)

	if w.status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.status)
	}
}

// --- HTTPErrorHandler ---

func TestHTTPErrorHandler_ProblemDetails(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return Error404("item not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", problem.Status)
	}
	if problem.Detail != "item not found" {
		t.Fatalf("expected detail 'item not found', got %q", problem.Detail)
	}
}

func TestHTTPErrorHandler_EchoHTTPError(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "bad input")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", problem.Status)
	}
	if problem.Detail != "bad input" {
		t.Fatalf("expected detail 'bad input', got %q", problem.Detail)
	}
}

func TestHTTPErrorHandler_ValidationError(t *testing.T) {
	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()

	type input struct {
		Name string `json:"name" validate:"required"`
	}

	e.POST("/test", func(c *echo.Context) error {
		var in input
		if err := c.Validate(&in); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, in)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", problem.Status)
	}
	if len(problem.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(problem.Errors))
	}
	if problem.Errors[0].Location != "name" {
		t.Fatalf("expected location 'name', got %q", problem.Errors[0].Location)
	}
}

func TestHTTPErrorHandler_BareError(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return errors.New("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", problem.Status)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("expected detail 'internal server error', got %q", problem.Detail)
	}
}

func TestHTTPErrorHandler_NotFound(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", problem.Status)
	}
	if problem.Detail != "resource not found" {
		t.Fatalf("expected detail 'resource not found', got %q", problem.Detail)
	}
}

func TestHTTPErrorHandler_MethodNotAllowed(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", problem.Status)
	}
	if !strings.Contains(problem.Detail, "DELETE") {
		t.Fatalf("expected detail to mention DELETE, got %q", problem.Detail)
	}
}

func TestHTTPErrorHandler_CBORResponse(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return Error400("bad request")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var problem ProblemDetails
	if err := cbor.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if problem.Status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", problem.Status)
	}
}

func TestHTTPErrorHandler_NotFoundCBOR(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var problem ProblemDetails
	if err := cbor.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", problem.Status)
	}
}

func TestHTTPErrorHandler_MethodNotAllowedCBOR(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, nil)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var problem ProblemDetails
	if err := cbor.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if problem.Status != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", problem.Status)
	}
}

// --- Recoverer ---

func TestRecovererReturnsProblemDetails(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.Use(Recoverer())
	e.GET("/panic", func(c *echo.Context) error {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", problem.Status)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("expected detail 'internal server error', got %q", problem.Detail)
	}
}

func TestRecovererReturnsCBOR(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.Use(Recoverer())
	e.GET("/panic", func(c *echo.Context) error {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var problem ProblemDetails
	if err := cbor.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", problem.Status)
	}
}

func TestRecovererWithErrorPanic(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.Use(Recoverer())
	e.GET("/panic-error", func(c *echo.Context) error {
		panic(errors.New("wrapped error"))
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-error", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("expected detail 'internal server error', got %q", problem.Detail)
	}
}

func TestRecovererWithNonErrorPanic(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.Use(Recoverer())
	e.GET("/panic-int", func(c *echo.Context) error {
		panic(42)
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-int", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("expected detail 'internal server error', got %q", problem.Detail)
	}
}

func TestRecovererRePanicsOnErrAbortHandler(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.Use(Recoverer())
	e.GET("/abort", func(c *echo.Context) error {
		panic(http.ErrAbortHandler)
	})

	defer func() {
		rec := recover()
		err, ok := rec.(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("expected http.ErrAbortHandler re-panic, got %v", rec)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	resp := httptest.NewRecorder()
	e.ServeHTTP(resp, req)

	t.Fatal("expected panic to propagate")
}

// --- Negotiate ---

func TestNegotiateJSON(t *testing.T) {
	e := echo.New()
	e.GET("/test", func(c *echo.Context) error {
		return Negotiate(c, http.StatusOK, map[string]string{"msg": "hello"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body["msg"] != "hello" {
		t.Fatalf("expected 'hello', got %q", body["msg"])
	}
}

func TestNegotiateCBOR(t *testing.T) {
	e := echo.New()
	e.GET("/test", func(c *echo.Context) error {
		return Negotiate(c, http.StatusOK, map[string]string{"msg": "hello"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("expected application/cbor, got %q", ct)
	}

	var body map[string]string
	if err := cbor.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if body["msg"] != "hello" {
		t.Fatalf("expected 'hello', got %q", body["msg"])
	}
}

func TestWriteProblemPreservesInstance(t *testing.T) {
	problem := ProblemDetails{
		Type:     "about:blank",
		Title:    "Not Found",
		Status:   http.StatusNotFound,
		Detail:   "resource not found",
		Instance: "/custom/instance",
	}
	req := httptest.NewRequest(http.MethodGet, "/other-path", nil)
	rec := httptest.NewRecorder()

	writeProblem(rec, req, problem)

	var got ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if got.Instance != "/custom/instance" {
		t.Fatalf("expected instance '/custom/instance', got %q", got.Instance)
	}
}

func TestNegotiateJSON_Status(t *testing.T) {
	e := echo.New()
	e.GET("/test", func(c *echo.Context) error {
		return Negotiate(c, http.StatusCreated, map[string]string{"id": "123"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestHTTPErrorHandler_EchoHTTPErrorNonStandard(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusTooManyRequests, "rate limited")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Detail != "rate limited" {
		t.Fatalf("expected detail 'rate limited', got %q", problem.Detail)
	}
}

func TestHTTPErrorHandler_ValidationErrorCBOR(t *testing.T) {
	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()

	type input struct {
		Name string `json:"name" validate:"required"`
	}

	e.POST("/test", func(c *echo.Context) error {
		var in input
		if err := c.Validate(&in); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, in)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+cbor" {
		t.Fatalf("expected application/problem+cbor, got %q", ct)
	}

	var problem ProblemDetails
	if err := cbor.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if len(problem.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(problem.Errors))
	}
}

func TestHTTPErrorHandler_BareErrorCBOR(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return errors.New("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var problem ProblemDetails
	if err := cbor.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if problem.Detail != "internal server error" {
		t.Fatalf("expected detail 'internal server error', got %q", problem.Detail)
	}
}

func TestNegotiateCBOR_MarshalError(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		return Negotiate(c, http.StatusOK, make(chan int))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unmarshalable type, got %d", rec.Code)
	}
}

func TestRecoverer_CommittedResponse(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.Use(Recoverer())
	e.GET("/test", func(c *echo.Context) error {
		c.Response().WriteHeader(http.StatusOK)
		_, _ = c.Response().Write([]byte("partial"))
		panic("late panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (committed), got %d", rec.Code)
	}
}

func TestHTTPErrorHandler_CommittedResponse(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/test", func(c *echo.Context) error {
		c.Response().WriteHeader(http.StatusOK)
		_, _ = c.Response().Write([]byte("partial"))
		return errors.New("late error")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (committed), got %d", rec.Code)
	}
}

// --- helpers ---

func headerSet(values []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			set[strings.TrimSpace(part)] = struct{}{}
		}
	}
	return set
}

func countInHeader(values []string, target string) int {
	count := 0
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			if strings.TrimSpace(part) == target {
				count++
			}
		}
	}
	return count
}
