package respond

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	platformvalidate "github.com/janisto/echo-playground/internal/platform/validate"
)

func TestSuccessNegotiationSpecificityQualityAndTieBreaking(t *testing.T) {
	tests := []struct {
		accept      string
		format      responseFormat
		contentType string
	}{
		{"", formatJSON, "application/json"},
		{"*/*", formatJSON, "application/json"},
		{"application/cbor", formatCBOR, "application/cbor"},
		{"application/cbor;q=0.8, application/json;q=0.8", formatJSON, "application/json"},
		{"application/json;q=0, */*;q=1", formatCBOR, "application/cbor"},
		{"application/*;q=0.9, application/json;q=0.8", formatCBOR, "application/cbor"},
		{"application/*;q=0.8, application/json;q=0.9", formatJSON, "application/json"},
		{
			"application/json;charset=utf-8;q=0.5, application/json;q=0.9",
			formatJSON,
			"application/json",
		},
		{"application/cbor;q=0.8, application/json;q=0.8", formatJSON, "application/json"},
		{"application/json; charset=utf-8", formatJSON, "application/json; charset=utf-8"},
		{"application/json; charset=iso-8859-1, application/cbor", formatCBOR, "application/cbor"},
		{"broken, application/cbor", formatCBOR, "application/cbor"},
		{"\u00a0application/json, application/cbor", formatCBOR, "application/cbor"},
		{"application/json\u2003, application/cbor", formatCBOR, "application/cbor"},
		{"application/json;q=1; note=\"a\u00a0b\"", formatJSON, "application/json"},
		{"\u00a0application/json", formatNotAcceptable, ""},
		{"application/problem+json", formatNotAcceptable, ""},
		{"application/json;q=0, application/cbor;q=0", formatNotAcceptable, ""},
	}
	for _, test := range tests {
		selected := selectRepresentation(test.accept, successCandidates)
		if selected.format != test.format || selected.contentType != test.contentType {
			t.Fatalf("Accept %q selected %#v", test.accept, selected)
		}
	}
}

func TestAcceptParsingBoundariesAndQuotedSeparators(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
		ok    bool
	}{
		{value: "0", want: 0, ok: true},
		{value: "0.", want: 0, ok: true},
		{value: "0.001", want: 1, ok: true},
		{value: "0.999", want: 999, ok: true},
		{value: "1", want: 1000, ok: true},
		{value: "1.000", want: 1000, ok: true},
		{value: ".5"},
		{value: "0.0000"},
		{value: "1.001"},
		{value: "0./"},
		{value: "2"},
		{value: "-1"},
		{value: "invalid"},
	} {
		got, ok := parseQuality(test.value)
		if got != test.want || ok != test.ok {
			t.Fatalf("parseQuality(%q) = %d/%t, want %d/%t", test.value, got, ok, test.want, test.ok)
		}
	}

	parts := splitQuoted(`application/json;profile="a,b",application/cbor`, ',')
	if len(parts) != 2 || parts[0] != `application/json;profile="a,b"` || parts[1] != "application/cbor" {
		t.Fatalf("splitQuoted = %#v", parts)
	}

	ranges := parseAccept(
		`*/json, noslash, /json, application/, application/json;broken, application/json;=x, ` +
			`application/json;level=1;q=0.7;extension=ignored, application/cbor;q=0.5, ` +
			`application/json;q=0.4;q=0.3, application/json;q="0.2"`,
	)
	if len(ranges) != 2 {
		t.Fatalf("parseAccept retained %#v, want two valid ranges", ranges)
	}
	if ranges[0].typ != "application" || ranges[0].subtype != "json" || ranges[0].quality != 700 ||
		ranges[0].specificity != 201 || ranges[0].params["level"] != "1" {
		t.Fatalf("parameterized range = %#v", ranges[0])
	}
	if ranges[1].subtype != "cbor" || ranges[1].quality != 500 || ranges[1].specificity != 200 {
		t.Fatalf("CBOR range = %#v", ranges[1])
	}

	selected := selectRepresentation(
		"application/json;q=0.2, application/json;q=0.8, application/cbor;q=0.7",
		successCandidates,
	)
	if selected.format != formatJSON || selected.contentType != MediaTypeJSON {
		t.Fatalf("equal-specificity highest quality selected %#v", selected)
	}
}

func TestNegotiationRejectsBeforeHandlerSideEffect(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	var calls atomic.Int32
	e.GET("/resource", func(c *echo.Context) error {
		calls.Add(1)
		return Negotiate(c, 200, map[string]string{"ok": "true"})
	}, SuccessNegotiation(false))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/resource", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assertJSONProblem(t, rec, 406, "not_acceptable")
	if calls.Load() != 0 {
		t.Fatalf("unacceptable request ran handler %d times", calls.Load())
	}
}

func TestJSONOnlyNegotiationAndBodylessDeletePrecedence(t *testing.T) {
	for _, test := range []struct {
		name      string
		method    string
		accept    string
		want      int
		wantCalls int32
	}{
		{name: "JSON-only rejects CBOR", method: http.MethodGet, accept: "application/cbor", want: 406},
		{name: "bodyless delete ignores unacceptable success", method: http.MethodDelete, accept: "text/plain", want: 204, wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := echo.New()
			e.HTTPErrorHandler = NewHTTPErrorHandler()
			var calls atomic.Int32
			e.Any("/resource", func(c *echo.Context) error {
				calls.Add(1)
				return c.NoContent(http.StatusNoContent)
			}, SuccessNegotiation(true))
			req := httptest.NewRequestWithContext(t.Context(), test.method, "/resource", nil)
			req.Header.Set("Accept", test.accept)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != test.want || calls.Load() != test.wantCalls {
				t.Fatalf("response/calls = %d/%d, want %d/%d", rec.Code, calls.Load(), test.want, test.wantCalls)
			}
		})
	}
}

func TestControlledErrorsPreserveStatusAndUseGCPErrorMedia(t *testing.T) {
	for _, test := range []struct {
		accept      string
		contentType string
		cbor        bool
	}{
		{"application/problem+json", "application/problem+json", false},
		{"application/cbor", "application/cbor", true},
		{"application/json", "application/problem+json", false},
		{"text/plain", "application/problem+json", false},
	} {
		e := echo.New()
		e.HTTPErrorHandler = NewHTTPErrorHandler()
		e.GET("/resource", func(*echo.Context) error { return GitHubUpstream() })
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/resource", nil)
		req.Header.Set("Accept", test.accept)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 502 || rec.Header().Get("Content-Type") != test.contentType {
			t.Fatalf("Accept %q status/media = %d/%q", test.accept, rec.Code, rec.Header().Get("Content-Type"))
		}
		var problem ProblemDetails
		var err error
		if test.cbor {
			err = cbor.Unmarshal(rec.Body.Bytes(), &problem)
		} else {
			err = json.Unmarshal(rec.Body.Bytes(), &problem)
		}
		if err != nil || problem.Code != "github_upstream" || problem.Status != 502 {
			t.Fatalf("problem = %#v, err=%v", problem, err)
		}
		if strings.Contains(rec.Header().Get("Content-Type"), "problem+cbor") {
			t.Fatal("nonportable Problem CBOR media emitted")
		}
	}
}

func TestProblemTaxonomyAndValidationSourcesAreSafe(t *testing.T) {
	problem := ValidationFailed(
		ErrorDetail{Detail: "name must be valid", Source: &ErrorSource{Pointer: "/name"}},
		ErrorDetail{Detail: "limit is invalid", Source: &ErrorSource{Parameter: "limit"}},
	)
	data, err := json.Marshal(problem)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"title", "status", "detail", "code"} {
		if object[field] == nil {
			t.Fatalf("problem lacks %s", field)
		}
	}
	if object["instance"] != nil || problem.Detail != "Request validation failed" ||
		problem.Code != "validation_failed" {
		t.Fatalf("unsafe problem: %#v", object)
	}
	invalid := *GitHubUpstream()
	invalid.Status = 200
	if got := problemFromError(&invalid); got.Code != "internal_error" || got.Status != 500 {
		t.Fatalf("contradictory local problem was not normalized: %#v", got)
	}
}

func TestProblemTaxonomyAndIssueBoundAreExact(t *testing.T) {
	tests := []struct {
		code   string
		status int
		title  string
		detail string
	}{
		{CodeInvalidRequest, 400, "Bad Request", "Request is malformed"},
		{CodeUnauthorized, 401, "Unauthorized", "Authentication is required or invalid"},
		{CodeForbidden, 403, "Forbidden", "Access is forbidden"},
		{CodeNotFound, 404, "Not Found", "Resource not found"},
		{CodeProfileNotFound, 404, "Not Found", "Profile not found"},
		{CodeGitHubNotFound, 404, "Not Found", "GitHub resource not found"},
		{CodeMethodNotAllowed, 405, "Method Not Allowed", "Method not allowed"},
		{CodeNotAcceptable, 406, "Not Acceptable", "No acceptable response representation is available"},
		{CodeProfileExists, 409, "Conflict", "Profile already exists"},
		{CodePayloadTooLarge, 413, "Content Too Large", "Request body is too large"},
		{CodeUnsupportedMediaType, 415, "Unsupported Media Type", "Request representation is not supported"},
		{CodeValidationFailed, 422, "Unprocessable Content", "Request validation failed"},
		{CodeGitHubRateLimit, 429, "Too Many Requests", "GitHub rate limit exceeded"},
		{CodeInternalError, 500, "Internal Server Error", "Internal server error"},
		{CodeGitHubUpstream, 502, "Bad Gateway", "GitHub upstream response is invalid or unavailable"},
		{CodeDependencyUnavailable, 503, "Service Unavailable", "A required dependency is unavailable"},
		{CodeGitHubTimeout, 504, "Gateway Timeout", "GitHub request timed out"},
	}
	for _, test := range tests {
		problem := Problem(test.code)
		if problem.Type != "about:blank" || problem.Code != test.code || problem.Status != test.status ||
			problem.Title != test.title || problem.Detail != test.detail || len(problem.Errors) != 0 {
			t.Fatalf("Problem(%q) = %#v", test.code, problem)
		}
	}
	if problem := Problem("unknown_code"); problem.Code != CodeInternalError || problem.Status != 500 {
		t.Fatalf("unknown code = %#v", problem)
	}

	issues := make([]ErrorDetail, 33)
	for index := range issues {
		issues[index] = ErrorDetail{Detail: "specific issue"}
	}
	if problem := ValidationFailed(issues[:32]...); len(problem.Errors) != 32 ||
		problem.Errors[31].Detail != "specific issue" {
		t.Fatalf("32 issues changed = %#v", problem.Errors)
	}
	if problem := ValidationFailed(issues...); len(problem.Errors) != 32 ||
		problem.Errors[30].Detail != "specific issue" ||
		problem.Errors[31] != (ErrorDetail{Detail: "Additional validation errors omitted"}) {
		t.Fatalf("33 issues were not bounded = %#v", problem.Errors)
	}
}

func TestValidationErrorSourcesAndFallbackMappings(t *testing.T) {
	validation := &platformvalidate.ValidationError{Message: "validation failed", Fields: []platformvalidate.FieldError{
		{Field: "a~/b", Message: "body issue", Location: platformvalidate.LocationBody},
		{Field: "limit", Message: "query issue", Location: platformvalidate.LocationQuery},
		{Field: "Authorization", Message: "header issue", Location: platformvalidate.LocationHeader},
		{Field: "owner", Message: "path issue", Location: platformvalidate.LocationPath},
	}}
	details := validationErrorDetails(validation)
	if len(details) != 4 || details[0].Source.Pointer != "/a~0~1b" ||
		details[1].Source.Parameter != "limit" || details[2].Source.Header != "Authorization" ||
		details[3].Source != nil {
		t.Fatalf("validation details = %#v", details)
	}

	for _, test := range []struct {
		err  error
		code string
	}{
		{err: validation, code: CodeValidationFailed},
		{err: echo.ErrNotFound, code: CodeNotFound},
		{err: echo.ErrMethodNotAllowed, code: CodeMethodNotAllowed},
		{err: &http.MaxBytesError{Limit: 1_000_000}, code: CodePayloadTooLarge},
		{err: echo.NewHTTPError(http.StatusBadRequest, "private"), code: CodeInvalidRequest},
		{err: echo.NewHTTPError(http.StatusUnsupportedMediaType, "private"), code: CodeUnsupportedMediaType},
		{err: echo.NewHTTPError(http.StatusUnprocessableEntity, "private"), code: CodeValidationFailed},
		{err: echo.NewHTTPError(http.StatusNotAcceptable, "private"), code: CodeNotAcceptable},
		{err: errors.New("private failure"), code: CodeInternalError},
	} {
		if problem := problemFromError(test.err); problem.Code != test.code {
			t.Fatalf("problemFromError(%T) = %#v, want %s", test.err, problem, test.code)
		}
	}
}

func TestHTTPErrorMappingMethodAndCancellation(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.GET("/resource", func(c *echo.Context) error { return c.NoContent(204) })
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/resource", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assertJSONProblem(t, rec, 405, "method_not_allowed")
	if !strings.Contains(rec.Header().Get("Allow"), "GET") {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}

	e.GET("/cancel", func(*echo.Context) error { return context.Canceled })
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/cancel", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 499 || rec.Body.Len() != 0 {
		t.Fatalf("canceled response = %d %q", rec.Code, rec.Body.String())
	}
}

func TestRecovererWritesInternalProblemBeforeCommit(t *testing.T) {
	core, recorded := observer.New(zap.ErrorLevel)
	e := echo.New()
	e.HTTPErrorHandler = NewHTTPErrorHandler()
	e.Use(Recoverer(zap.New(core)))
	e.GET("/panic", func(*echo.Context) error { panic("secret panic") })
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assertJSONProblem(t, rec, 500, "internal_error")
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("panic detail leaked: %s", rec.Body.String())
	}
	if entries := recorded.FilterMessage("panic recovered").All(); len(entries) != 1 {
		t.Fatalf("recovery log count = %d, want 1", len(entries))
	}
}

func TestRecovererPropagatesAbortAndTerminatesCommittedPanic(t *testing.T) {
	for _, test := range []struct {
		name      string
		handler   echo.HandlerFunc
		wantWrite int
	}{
		{
			name: "abort sentinel",
			handler: func(*echo.Context) error {
				panic(http.ErrAbortHandler)
			},
			wantWrite: 200,
		},
		{
			name: "panic after commit",
			handler: func(c *echo.Context) error {
				c.Response().WriteHeader(http.StatusAccepted)
				panic("private")
			},
			wantWrite: http.StatusAccepted,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			context := echo.New().NewContext(request, recorder)
			panicValue := recoverValue(func() {
				_ = Recoverer(zap.NewNop())(test.handler)(context)
			})
			panicError, ok := panicValue.(error)
			if !ok || !errors.Is(panicError, http.ErrAbortHandler) {
				t.Fatalf("panic = %#v, want http.ErrAbortHandler", panicValue)
			}
			if recorder.Code != test.wantWrite {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantWrite)
			}
		})
	}
}

func TestProblemHEADAndCommittedErrorHandlingDoNotAppendBodies(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/missing", nil)
	rec := httptest.NewRecorder()
	writeProblem(rec, req, *NotFound())
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 ||
		rec.Header().Get("Content-Type") != MediaTypeProblemJSON {
		t.Fatalf("HEAD problem = %d/%q/%q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	context := echo.New().NewContext(
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
		rec,
	)
	context.Response().WriteHeader(http.StatusAccepted)
	NewHTTPErrorHandler()(context, errors.New("private"))
	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Fatalf("committed response changed to %d %q", rec.Code, rec.Body.String())
	}
}

func recoverValue(call func()) (value any) {
	defer func() { value = recover() }()
	call()
	return nil
}

func assertJSONProblem(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status || recorder.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf(
			"status/media = %d/%q: %s",
			recorder.Code,
			recorder.Header().Get("Content-Type"),
			recorder.Body.String(),
		)
	}
	var problem ProblemDetails
	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&problem,
	); err != nil || problem.Status != status ||
		problem.Code != code {
		t.Fatalf("problem = %#v, err=%v", problem, err)
	}
}

func FuzzSelectFormat(f *testing.F) {
	f.Add("application/json")
	f.Add("application/cbor")
	f.Add("broken")
	f.Fuzz(func(t *testing.T, accept string) {
		selected := selectRepresentation(accept, successCandidates)
		if selected.format < formatNotAcceptable || selected.format > formatCBOR {
			t.Fatalf("invalid format %d", selected.format)
		}
	})
}

func FuzzSelectFormatQuality(f *testing.F) {
	f.Add("application/json;q=0, */*;q=1")
	f.Add("application/cbor;q=0.5, application/json;q=0.5")
	f.Fuzz(func(t *testing.T, accept string) {
		selected := selectRepresentation(accept, successCandidates)
		if selected.format == formatJSON && !strings.HasPrefix(selected.contentType, "application/json") {
			t.Fatalf("JSON format with %q", selected.contentType)
		}
		if selected.format == formatCBOR && selected.contentType != "application/cbor" {
			t.Fatalf("CBOR format with %q", selected.contentType)
		}
	})
}
