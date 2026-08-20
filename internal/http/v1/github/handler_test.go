package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"

	"github.com/janisto/echo-playground/internal/platform/pagination"
	"github.com/janisto/echo-playground/internal/platform/respond"
	githubsvc "github.com/janisto/echo-playground/internal/service/github"
	"github.com/janisto/echo-playground/internal/testutil"
)

type serviceDouble struct {
	calls atomic.Int32
	error error
}

func (service *serviceDouble) GetOwner(context.Context, string) (githubsvc.Owner, error) {
	service.calls.Add(1)
	if service.error != nil {
		return githubsvc.Owner{}, service.error
	}
	return githubsvc.Owner{
		ID:          1,
		Login:       "acme",
		Type:        "Organization",
		AvatarURL:   "https://example.test/avatar",
		HTMLURL:     "https://example.test/acme",
		PublicRepos: 2,
		Followers:   3,
		Following:   4,
		CreatedAt: time.Date(
			2026,
			7,
			30,
			12,
			0,
			0,
			0,
			time.UTC,
		),
		UpdatedAt: time.Date(2026, 7, 30, 12, 1, 0, 0, time.UTC),
	}, nil
}

func (service *serviceDouble) ListOwnerRepositories(
	context.Context,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.RepositorySummary], error) {
	service.calls.Add(1)
	return githubsvc.Page[githubsvc.RepositorySummary]{
		Entries: []githubsvc.RepositorySummary{
			{ID: 1, Name: "repo", FullName: "acme/repo", HTMLURL: "https://example.test/acme/repo"},
		},
		NextCursor: "next-token",
	}, service.error
}

func (service *serviceDouble) GetRepository(context.Context, string, string) (githubsvc.Repository, error) {
	service.calls.Add(1)
	return githubsvc.Repository{
		RepositorySummary: githubsvc.RepositorySummary{
			ID:       1,
			Name:     "repo",
			FullName: "acme/repo",
			HTMLURL:  "https://example.test/acme/repo",
		},
		Topics:        []string{},
		CreatedAt:     time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC),
		DefaultBranch: "main",
	}, service.error
}

func (service *serviceDouble) ListRepositoryActivity(
	context.Context,
	string,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.Activity], error) {
	service.calls.Add(1)
	return githubsvc.Page[githubsvc.Activity]{Entries: []githubsvc.Activity{}}, service.error
}

func (service *serviceDouble) ListRepositoryLanguages(context.Context, string, string) ([]githubsvc.Language, error) {
	service.calls.Add(1)
	return []githubsvc.Language{}, service.error
}

func (service *serviceDouble) ListRepositoryTags(
	context.Context,
	string,
	string,
	int,
	*pagination.Cursor,
) (githubsvc.Page[githubsvc.Tag], error) {
	service.calls.Add(1)
	return githubsvc.Page[githubsvc.Tag]{Entries: []githubsvc.Tag{}}, service.error
}

func TestOwnerJSONAndCBORHaveExactPublicShape(t *testing.T) {
	for _, accept := range []string{"application/json", "application/cbor"} {
		t.Run(accept, func(t *testing.T) {
			service := &serviceDouble{}
			echo := githubEcho(service)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/github/owners/acme", nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			echo.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != accept {
				t.Fatalf(
					"status/content-type = %d/%q: %s",
					rec.Code,
					rec.Header().Get("Content-Type"),
					rec.Body.String(),
				)
			}
			var object map[string]any
			if accept == "application/cbor" {
				if err := cbor.Unmarshal(rec.Body.Bytes(), &object); err != nil {
					t.Fatalf("decode CBOR: %v", err)
				}
			} else if err := json.Unmarshal(rec.Body.Bytes(), &object); err != nil {
				t.Fatalf("decode JSON: %v", err)
			}
			for _, field := range []string{"id", "login", "type", "name", "avatarUrl", "htmlUrl", "company", "blog", "location", "bio", "publicRepos", "followers", "following", "createdAt", "updatedAt"} {
				if _, present := object[field]; !present {
					t.Fatalf("response lacks %s: %#v", field, object)
				}
			}
			if len(object) != 15 || service.calls.Load() != 1 {
				t.Fatalf("response/calls = %#v/%d", object, service.calls.Load())
			}
		})
	}
}

func TestLocalRejectionsDoNotCallGitHub(t *testing.T) {
	tests := []struct {
		path   string
		accept string
		status int
		code   string
	}{
		{path: "/v1/github/owners/-bad", status: 422, code: "validation_failed"},
		{path: "/v1/github/owners/acme?unknown=1", status: 400, code: "invalid_request"},
		{path: "/v1/github/owners/acme", accept: "text/plain", status: 406, code: "not_acceptable"},
		{path: "/v1/github/owners/acme/repos?limit=101", status: 422, code: "validation_failed"},
		{path: "/v1/github/owners/acme/repos?cursor=", status: 400, code: "invalid_request"},
		{path: "/v1/github/repos/acme/...", status: 422, code: "validation_failed"},
	}
	for _, test := range tests {
		t.Run(test.path+test.accept, func(t *testing.T) {
			service := &serviceDouble{}
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil)
			if test.accept != "" {
				req.Header.Set("Accept", test.accept)
			}
			rec := httptest.NewRecorder()
			githubEcho(service).ServeHTTP(rec, req)
			assertProblem(t, rec, test.status, test.code)
			if service.calls.Load() != 0 {
				t.Fatalf("rejection called GitHub %d times", service.calls.Load())
			}
		})
	}
}

func TestPathAndLimitBoundariesAtFrameworkBoundary(t *testing.T) {
	validPaths := []string{
		"/v1/github/owners/A",
		"/v1/github/owners/A" + strings.Repeat("_", 37) + "Z",
		"/v1/github/repos/acme/_",
		"/v1/github/repos/acme/" + strings.Repeat("r", 100),
		"/v1/github/owners/acme/repos",
		"/v1/github/owners/acme/repos?limit=1",
		"/v1/github/owners/acme/repos?limit=100",
	}
	for _, path := range validPaths {
		service := &serviceDouble{}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		githubEcho(service).ServeHTTP(rec, req)
		if rec.Code != 200 || service.calls.Load() != 1 {
			t.Fatalf("valid GET %s = %d calls=%d: %s", path, rec.Code, service.calls.Load(), rec.Body.String())
		}
	}

	invalid := []struct {
		path   string
		status int
	}{
		{path: "/v1/github/owners/-bad", status: 422},
		{path: "/v1/github/owners/bad-", status: 422},
		{path: "/v1/github/owners/" + strings.Repeat("a", 40), status: 422},
		{path: "/v1/github/owners/café", status: 422},
		{path: "/v1/github/repos/acme/...", status: 422},
		{path: "/v1/github/repos/acme/" + strings.Repeat("r", 101), status: 422},
		{path: "/v1/github/repos/acme/répo", status: 422},
		{path: "/v1/github/owners/acme?query=1", status: 400},
		{path: "/v1/github/owners/acme/repos?limit=1&limit=2", status: 400},
		{path: "/v1/github/owners/acme/repos?limit=zero", status: 422},
		{path: "/v1/github/owners/acme/repos?limit=0", status: 422},
		{path: "/v1/github/owners/acme/repos?limit=101", status: 422},
		{path: "/v1/github/owners/acme/repos?limit=18446744073709551616", status: 422},
		{path: "/v1/github/owners/acme/repos?cursor=broken", status: 400},
		{path: "/v1/github/owners/acme/repos?cursor=" + strings.Repeat("a", pagination.MaxCursorLength+1), status: 400},
	}
	wrongScope := pagination.NewCursor(pagination.Scope{Operation: "listGitHubRepositoryTags", Owner: "acme", Repository: "repo", Limit: 20}, "next", "2").
		Encode()
	invalid = append(invalid, struct {
		path   string
		status int
	}{path: "/v1/github/owners/acme/repos?cursor=" + wrongScope, status: 400})
	for _, test := range invalid {
		service := &serviceDouble{}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil)
		rec := httptest.NewRecorder()
		githubEcho(service).ServeHTTP(rec, req)
		if rec.Code != test.status || service.calls.Load() != 0 {
			t.Fatalf(
				"invalid GET %s = %d calls=%d, want %d: %s",
				test.path,
				rec.Code,
				service.calls.Load(),
				test.status,
				rec.Body.String(),
			)
		}
	}
}

func TestMalformedAndInvalidUTF8QueryAreRejectedWithoutGitHubCall(t *testing.T) {
	for _, rawQuery := range []string{"%zz", "cursor=%FF", "limit=1&limit=2", "unknown=1"} {
		service := &serviceDouble{}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/github/owners/acme/repos", nil)
		req.URL.RawQuery = rawQuery
		req.RequestURI = req.URL.Path + "?" + rawQuery
		rec := httptest.NewRecorder()
		githubEcho(service).ServeHTTP(rec, req)
		assertProblem(t, rec, 400, "invalid_request")
		if service.calls.Load() != 0 {
			t.Fatalf("query %q called GitHub %d times", rawQuery, service.calls.Load())
		}
	}
}

func TestPaginatedSuccessBuildsOnlyPublicRelativeLink(t *testing.T) {
	service := &serviceDouble{}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/github/owners/acme/repos?limit=1", nil)
	req.Host = "attacker.invalid"
	req.Header.Set("Forwarded", "host=attacker.invalid")
	rec := httptest.NewRecorder()
	githubEcho(service).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	link := rec.Header().Get("Link")
	if link != `</v1/github/owners/acme/repos?cursor=next-token&limit=1>; rel="next"` ||
		strings.Contains(link, "attacker") {
		t.Fatalf("Link = %q", link)
	}
	var body RepositoryPage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Count != 1 || len(body.Repos) != 1 {
		t.Fatalf("page = %#v, err=%v", body, err)
	}
}

func TestGitHubControlledFailuresAreSafe(t *testing.T) {
	tests := []struct {
		error      error
		status     int
		code       string
		retryAfter string
		reset      string
	}{
		{error: githubsvc.ErrNotFound, status: 404, code: "github_not_found"},
		{
			error:      &githubsvc.RateLimitError{RetryAfter: "7", Reset: "2000000000"},
			status:     429,
			code:       "github_rate_limit",
			retryAfter: "7",
			reset:      "2000000000",
		},
		{error: githubsvc.ErrUpstream, status: 502, code: "github_upstream"},
		{error: githubsvc.ErrTimeout, status: 504, code: "github_timeout"},
	}
	for _, test := range tests {
		service := &serviceDouble{error: test.error}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/github/owners/acme", nil)
		rec := httptest.NewRecorder()
		githubEcho(service).ServeHTTP(rec, req)
		assertProblem(t, rec, test.status, test.code)
		if rec.Header().Get("Retry-After") != test.retryAfter || rec.Header().Get("X-RateLimit-Reset") != test.reset {
			t.Fatalf("quota headers = %q/%q", rec.Header().Get("Retry-After"), rec.Header().Get("X-RateLimit-Reset"))
		}
		if strings.Contains(rec.Body.String(), "provider") {
			t.Fatalf("public problem leaked internal text: %s", rec.Body.String())
		}
	}
}

func TestUnsupportedMethodDoesNotReadBody(t *testing.T) {
	service := &serviceDouble{}
	reader := &readCounter{value: strings.NewReader(strings.Repeat("x", 100))}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/github/owners/acme", reader)
	rec := httptest.NewRecorder()
	githubEcho(service).ServeHTTP(rec, req)
	assertProblem(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
	if reader.reads.Load() != 0 || service.calls.Load() != 0 {
		t.Fatalf("unsupported method read=%d calls=%d", reader.reads.Load(), service.calls.Load())
	}
	allow := rec.Header().Get("Allow")
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "HEAD") {
		t.Fatalf("Allow = %q", allow)
	}
}

type readCounter struct {
	value *strings.Reader
	reads atomic.Int32
}

func (reader *readCounter) Read(buffer []byte) (int, error) {
	reader.reads.Add(1)
	return reader.value.Read(buffer)
}

func githubEcho(service githubsvc.Service) http.Handler {
	e := testutil.NewTestEcho()
	Register(e.Group("/v1"), service)
	return e
}

func assertProblem(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, status, recorder.Body.String())
	}
	var problem respond.ProblemDetails
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Status != status || problem.Code != code {
		t.Fatalf("problem = %#v", problem)
	}
}

var _ githubsvc.Service = (*serviceDouble)(nil)
