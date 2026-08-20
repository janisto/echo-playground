package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/janisto/echo-playground/internal/platform/pagination"
)

func TestClientOwnerUsesFixedAnonymousRequestAndProjectsClosedModel(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/users/acme" || r.URL.RawQuery != "" {
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL.String())
		}
		wantHeaders := map[string]string{
			"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2026-03-10",
			"User-Agent": "echo-playground", "Accept-Encoding": "identity",
		}
		for name, want := range wantHeaders {
			if got := r.Header.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		if len(r.Header) != 4 || r.Header.Get("Authorization") != "" || r.Header.Get("X-Request-ID") != "" {
			t.Errorf("outbound header allowlist violated: %#v", r.Header)
		}
		writeJSON(w, ownerFixture(`"private_email":"secret@example.test","future_counter":1e1000`))
	}))
	defer server.Close()
	client := testClient(t, server)
	owner, err := client.GetOwner(t.Context(), "acme")
	if err != nil {
		t.Fatalf("GetOwner: %v", err)
	}
	if calls.Load() != 1 || owner.ID != 1 || owner.Login != "acme" || owner.Name != nil ||
		owner.CreatedAt.Format(time.RFC3339Nano) != "2026-07-30T12:00:00Z" {
		t.Fatalf("unexpected projection: %#v, calls=%d", owner, calls.Load())
	}
}

func TestClientRejectsInvalidLocalResourceBeforeTransport(t *testing.T) {
	var calls atomic.Int32
	client := transportClient(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("must not run")
	})
	if _, err := client.GetOwner(t.Context(), "-bad"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("invalid owner error = %v", err)
	}
	if _, err := client.GetRepository(t.Context(), "acme", "..."); !errors.Is(err, ErrUpstream) {
		t.Fatalf("dot-only repository error = %v", err)
	}
	scope := pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: "acme", Limit: 10}
	zeroPage := pagination.NewCursor(scope, "prev", "0")
	if _, err := client.ListOwnerRepositories(t.Context(), "acme", 10, &zeroPage); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("page zero cursor error = %v", err)
	}
	firstPageAsNext := pagination.NewCursor(scope, "next", "1")
	if _, err := client.ListOwnerRepositories(
		t.Context(),
		"acme",
		10,
		&firstPageAsNext,
	); !errors.Is(
		err,
		ErrInvalidCursor,
	) {
		t.Fatalf("next-to-page-one cursor error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid local values made %d requests", calls.Load())
	}
}

func TestClientTranslatesNumberedProviderLinksWithoutFollowingThem(t *testing.T) {
	var calls atomic.Int32
	var origin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		page := r.URL.Query().Get("page")
		wantQuery := "direction=asc&per_page=1&sort=full_name&type=owner"
		if page == "3" {
			wantQuery = "direction=asc&page=3&per_page=1&sort=full_name&type=owner"
		}
		if r.URL.Query().Encode() != wantQuery {
			t.Errorf("query = %q, want %q", r.URL.Query().Encode(), wantQuery)
		}
		if page == "" {
			w.Header().
				Add("Link", `<https://ignored.invalid/a>; rel="last", <`+origin+`/users/acme/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=3>; rel="next"; title="a,b"`)
		} else {
			w.Header().
				Add("Link", `<`+origin+`/user/9/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=1>; rel="prev"`)
		}
		writeJSON(w, `[`+repositorySummaryFixture("repo")+`]`)
	}))
	defer server.Close()
	origin = server.URL
	client := testClient(t, server)
	first, err := client.ListOwnerRepositories(t.Context(), "acme", 1, nil)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if first.NextCursor == "" || first.PrevCursor != "" || calls.Load() != 1 {
		t.Fatalf("first navigation = %#v, calls=%d", first, calls.Load())
	}
	cursor, err := pagination.DecodeCursor(first.NextCursor)
	if err != nil || cursor.Direction != "next" || cursor.Position != "3" {
		t.Fatalf("decoded next cursor = %#v, %v", cursor, err)
	}
	third, err := client.ListOwnerRepositories(t.Context(), "acme", 1, &cursor)
	if err != nil {
		t.Fatalf("third page: %v", err)
	}
	previous, err := pagination.DecodeCursor(third.PrevCursor)
	if err != nil || previous.Direction != "prev" || previous.Position != "1" || third.NextCursor != "" {
		t.Fatalf("third navigation = %#v, decoded=%#v, err=%v", third, previous, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("provider Link was followed; calls=%d", calls.Load())
	}
}

func TestClientActivityCursorReconstructionAndReverseLink(t *testing.T) {
	var origin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Encode(); got != "after=after-token&direction=desc&per_page=2" {
			t.Errorf("activity query = %q", got)
		}
		w.Header().
			Set("Link", `<`+origin+`/repositories/7/activity?before=before-token&direction=desc&per_page=2>; rel="prev"`)
		writeJSON(w, `[]`)
	}))
	defer server.Close()
	origin = server.URL
	client := testClient(t, server)
	scope := pagination.Scope{Operation: "listGitHubRepositoryActivity", Owner: "acme", Repository: "repo", Limit: 2}
	cursor := pagination.NewCursor(scope, "next", "after-token")
	page, err := client.ListRepositoryActivity(t.Context(), "acme", "repo", 2, &cursor)
	if err != nil {
		t.Fatalf("activity page: %v", err)
	}
	previous, err := pagination.DecodeCursor(page.PrevCursor)
	if err != nil || previous.Direction != "prev" || previous.Position != "before-token" {
		t.Fatalf("activity previous cursor = %#v, err=%v", previous, err)
	}
}

func TestClientRejectsUnsafeOrNonAdvancingProviderLinks(t *testing.T) {
	tests := map[string]string{
		"cross origin":        `<https://attacker.invalid/users/acme/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=2>; rel="next"`,
		"same page":           `ORIGIN/users/acme/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=1>; rel="next"`,
		"page zero":           `ORIGIN/users/acme/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=0>; rel="prev"`,
		"missing page":        `ORIGIN/users/acme/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=>; rel="next"`,
		"wrong fixed query":   `ORIGIN/users/acme/repos?direction=desc&per_page=1&sort=full_name&type=owner&page=2>; rel="next"`,
		"wrong path":          `ORIGIN/users/acme/other?direction=asc&per_page=1&sort=full_name&type=owner&page=2>; rel="next"`,
		"empty page has next": `ORIGIN/users/acme/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=2>; rel="next"`,
		"duplicate":           `ORIGIN/users/acme/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=2>; rel="next", <ORIGIN/users/acme/repos?direction=asc&per_page=1&sort=full_name&type=owner&page=3>; rel="next"`,
	}
	for name, template := range tests {
		t.Run(name, func(t *testing.T) {
			var origin string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Link", strings.ReplaceAll(template, "ORIGIN", "<"+origin))
				writeJSON(w, `[]`)
			}))
			defer server.Close()
			origin = server.URL
			_, err := testClient(t, server).ListOwnerRepositories(t.Context(), "acme", 1, nil)
			if !errors.Is(err, ErrUpstream) {
				t.Fatalf("error = %v, want ErrUpstream", err)
			}
		})
	}
}

func TestClientIgnoresAnchoredProviderLinkValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", `<https://attacker.invalid/private>; rel="next"; anchor="/alternate"`)
		writeJSON(w, `[`+repositorySummaryFixture("repo")+`]`)
	}))
	defer server.Close()
	page, err := testClient(t, server).ListOwnerRepositories(t.Context(), "acme", 1, nil)
	if err != nil || page.NextCursor != "" || page.PrevCursor != "" {
		t.Fatalf("anchored navigation = %#v, %v", page, err)
	}
}

func TestClientRejectsInvalidActivityLinkState(t *testing.T) {
	tests := []struct {
		name   string
		cursor *pagination.Cursor
		link   func(string) string
	}{
		{name: "initial prev", link: func(origin string) string {
			return `<` + origin + `/repos/acme/repo/activity?before=older&direction=desc&per_page=1>; rel="prev"`
		}},
		{name: "exact replay", cursor: func() *pagination.Cursor {
			cursor := pagination.NewCursor(
				pagination.Scope{
					Operation:  "listGitHubRepositoryActivity",
					Owner:      "acme",
					Repository: "repo",
					Limit:      1,
				},
				"next",
				"same",
			)
			return &cursor
		}(), link: func(origin string) string {
			return `<` + origin + `/repos/acme/repo/activity?after=same&direction=desc&per_page=1>; rel="next"`
		}},
		{name: "wrong member", cursor: func() *pagination.Cursor {
			cursor := pagination.NewCursor(
				pagination.Scope{
					Operation:  "listGitHubRepositoryActivity",
					Owner:      "acme",
					Repository: "repo",
					Limit:      1,
				},
				"next",
				"current",
			)
			return &cursor
		}(), link: func(origin string) string {
			return `<` + origin + `/repos/acme/repo/activity?before=other&direction=desc&per_page=1>; rel="next"`
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var origin string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Link", test.link(origin))
				writeJSON(w, `[`+activityFixture("1", "null", "2026-07-30T12:01:00.000Z")+`]`)
			}))
			defer server.Close()
			origin = server.URL
			if _, err := testClient(
				t,
				server,
			).ListRepositoryActivity(t.Context(), "acme", "repo", 1, test.cursor); !errors.Is(
				err,
				ErrUpstream,
			) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestClientManualRedirectPolicy(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/users/acme" {
			w.Header().Set("Location", "/user/1")
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}
		if r.URL.Path != "/user/1" {
			t.Errorf("redirect path = %q", r.URL.Path)
		}
		writeJSON(w, ownerFixture(""))
	}))
	defer server.Close()
	if _, err := testClient(t, server).GetOwner(t.Context(), "acme"); err != nil {
		t.Fatalf("safe redirect: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("redirect calls = %d", calls.Load())
	}

	loop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/users/acme")
		w.WriteHeader(http.StatusFound)
	}))
	defer loop.Close()
	if _, err := testClient(t, loop).GetOwner(t.Context(), "acme"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("loop error = %v", err)
	}
}

func TestClientAcceptsEveryExactNamedToNumericRedirect(t *testing.T) {
	tests := []struct {
		name        string
		namedPath   string
		numericPath string
		body        string
		call        func(*Client) error
	}{
		{
			name:        "owner",
			namedPath:   "/users/acme",
			numericPath: "/user/1",
			body:        ownerFixture(""),
			call: func(client *Client) error {
				_, err := client.GetOwner(t.Context(), "acme")
				return err
			},
		},
		{
			name:        "owner repositories",
			namedPath:   "/users/acme/repos",
			numericPath: "/user/1/repos",
			body:        `[]`,
			call: func(client *Client) error {
				_, err := client.ListOwnerRepositories(t.Context(), "acme", 1, nil)
				return err
			},
		},
		{
			name:        "repository",
			namedPath:   "/repos/acme/repo",
			numericPath: "/repositories/1",
			body:        repositoryDetailFixture("null", "null", ""),
			call: func(client *Client) error {
				_, err := client.GetRepository(t.Context(), "acme", "repo")
				return err
			},
		},
		{
			name:        "activity",
			namedPath:   "/repos/acme/repo/activity",
			numericPath: "/repositories/1/activity",
			body:        `[]`,
			call: func(client *Client) error {
				_, err := client.ListRepositoryActivity(t.Context(), "acme", "repo", 1, nil)
				return err
			},
		},
		{
			name:        "languages",
			namedPath:   "/repos/acme/repo/languages",
			numericPath: "/repositories/1/languages",
			body:        `{}`,
			call: func(client *Client) error {
				_, err := client.ListRepositoryLanguages(t.Context(), "acme", "repo")
				return err
			},
		},
		{
			name:        "tags",
			namedPath:   "/repos/acme/repo/tags",
			numericPath: "/repositories/1/tags",
			body:        `[]`,
			call: func(client *Client) error {
				_, err := client.ListRepositoryTags(t.Context(), "acme", "repo", 1, nil)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case test.namedPath:
					location := test.numericPath
					if r.URL.RawQuery != "" {
						location += "?" + r.URL.RawQuery
					}
					w.Header().Set("Location", location)
					w.WriteHeader(http.StatusMovedPermanently)
				case test.numericPath:
					writeJSON(w, test.body)
				default:
					t.Errorf("unexpected redirect path %q", r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			if err := test.call(testClient(t, server)); err != nil {
				t.Fatalf("redirect operation: %v", err)
			}
		})
	}
}

func TestClientStatusEncodingMediaAndSchemaFailures(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		headers  http.Header
		body     string
		want     error
		wantRate bool
	}{
		{name: "not found", status: 404, body: `provider secret`, want: ErrNotFound},
		{name: "rate without headers", status: 403, body: `provider secret`, wantRate: true},
		{name: "unexpected 201", status: 201, body: ownerFixture(""), want: ErrUpstream},
		{name: "provider 401", status: 401, body: ownerFixture(""), want: ErrUpstream},
		{name: "provider 410", status: 410, body: ownerFixture(""), want: ErrUpstream},
		{name: "provider 422", status: 422, body: ownerFixture(""), want: ErrUpstream},
		{
			name:    "provider 500 ignores quota",
			status:  500,
			headers: http.Header{"Retry-After": {"7"}, "X-RateLimit-Reset": {"2000000002"}},
			body:    ownerFixture(""),
			want:    ErrUpstream,
		},
		{
			name:    "not found ignores quota",
			status:  404,
			headers: http.Header{"Retry-After": {"7"}, "X-RateLimit-Reset": {"2000000002"}},
			body:    ownerFixture(""),
			want:    ErrNotFound,
		},
		{
			name:    "non identity before mapping",
			status:  404,
			headers: http.Header{"Content-Encoding": {"gzip"}},
			want:    ErrUpstream,
		},
		{
			name:    "wrong media",
			status:  200,
			headers: http.Header{"Content-Type": {"text/plain"}},
			body:    ownerFixture(""),
			want:    ErrUpstream,
		},
		{
			name:    "duplicate media",
			status:  200,
			headers: http.Header{"Content-Type": {"application/json", "application/json"}},
			body:    ownerFixture(""),
			want:    ErrUpstream,
		},
		{name: "duplicate JSON member", status: 200, headers: jsonHeader(), body: `{"id":1,"id":2}`, want: ErrUpstream},
		{name: "invalid projection", status: 200, headers: jsonHeader(), body: `{"id":1}`, want: ErrUpstream},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := transportClient(func(*http.Request) (*http.Response, error) {
				headers := test.headers.Clone()
				if headers == nil {
					headers = make(http.Header)
				}
				return &http.Response{
					StatusCode:    test.status,
					Header:        headers,
					Body:          io.NopCloser(strings.NewReader(test.body)),
					ContentLength: int64(len(test.body)),
				}, nil
			})
			_, err := client.GetOwner(t.Context(), "acme")
			var rate *RateLimitError
			if test.wantRate {
				if !errors.As(err, &rate) || rate.RetryAfter != "60" || rate.Reset != "" {
					t.Fatalf("rate error = %#v", err)
				}
			} else if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestClientQuotaHeaderCombinations(t *testing.T) {
	now := time.Unix(100, 250_000_000)
	tests := []struct {
		name       string
		header     http.Header
		retryAfter string
		reset      string
	}{
		{
			name:       "valid retry invalid reset",
			header:     http.Header{"Retry-After": {"7"}, "X-Ratelimit-Reset": {"01"}},
			retryAfter: "7",
		},
		{
			name:       "invalid retry valid reset",
			header:     http.Header{"Retry-After": {"bad"}, "X-Ratelimit-Reset": {"103"}},
			retryAfter: "3",
			reset:      "103",
		},
		{
			name:       "both valid retry wins",
			header:     http.Header{"Retry-After": {"7"}, "X-Ratelimit-Reset": {"103"}},
			retryAfter: "7",
			reset:      "103",
		},
		{name: "zero retry", header: http.Header{"Retry-After": {"0"}}, retryAfter: "0"},
		{
			name:       "maximum retry",
			header:     http.Header{"Retry-After": {strconv.FormatUint(maximumSafeInteger, 10)}},
			retryAfter: strconv.FormatUint(maximumSafeInteger, 10),
		},
		{
			name:       "overflow retry",
			header:     http.Header{"Retry-After": {strconv.FormatUint(maximumSafeInteger+1, 10)}},
			retryAfter: "60",
		},
		{name: "comma retry", header: http.Header{"Retry-After": {"1, 2"}}, retryAfter: "60"},
		{name: "repeated retry", header: http.Header{"Retry-After": {"1", "2"}}, retryAfter: "60"},
		{name: "reset at now", header: http.Header{"X-Ratelimit-Reset": {"100"}}, retryAfter: "60"},
	}
	client := transportClient(func(*http.Request) (*http.Response, error) { return nil, errors.New("unused") })
	client.clock = func() time.Time { return now }
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := client.rateLimitError(test.header)
			if got.RetryAfter != test.retryAfter || got.Reset != test.reset {
				t.Fatalf("rate limit = %#v, want retry/reset %q/%q", got, test.retryAfter, test.reset)
			}
		})
	}
}

func TestClientQuotaHintsUseInjectedFractionalClock(t *testing.T) {
	now := time.Unix(2_000_000_000, 250_000_000)
	client := transportClient(func(*http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Retry-After", "invalid")
		header.Set("X-Ratelimit-Reset", "2000000002")
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader("never read")),
		}, nil
	})
	client.clock = func() time.Time { return now }
	_, err := client.GetOwner(t.Context(), "acme")
	var rate *RateLimitError
	if !errors.As(err, &rate) || rate.RetryAfter != "2" || rate.Reset != "2000000002" {
		t.Fatalf("rate error = %#v", err)
	}
}

func TestClientBodyBoundaryAndCollectionLimit(t *testing.T) {
	exact := paddedOwnerFixture(t, maximumProviderBody)
	client := transportClient(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        jsonHeader(),
			Body:          io.NopCloser(strings.NewReader(exact)),
			ContentLength: -1,
		}, nil
	})
	if _, err := client.GetOwner(t.Context(), "acme"); err != nil {
		t.Fatalf("exact 4 MiB body: %v", err)
	}
	over := exact + " "
	client = transportClient(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        jsonHeader(),
			Body:          io.NopCloser(strings.NewReader(over)),
			ContentLength: -1,
		}, nil
	})
	if _, err := client.GetOwner(t.Context(), "acme"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("over-limit body error = %v", err)
	}

	client = transportClient(func(*http.Request) (*http.Response, error) {
		body := `[` + repositorySummaryFixture("one") + `,` + repositorySummaryFixture("two") + `]`
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        jsonHeader(),
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
		}, nil
	})
	if _, err := client.ListOwnerRepositories(t.Context(), "acme", 1, nil); !errors.Is(err, ErrUpstream) {
		t.Fatalf("over-limit collection error = %v", err)
	}
}

func TestClientDeadlineAndCallerCancellation(t *testing.T) {
	client := transportClient(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	client.timeout = 5 * time.Millisecond
	if _, err := client.GetOwner(t.Context(), "acme"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("operation timeout error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.GetOwner(ctx, "acme"); !errors.Is(err, context.Canceled) || errors.Is(err, ErrTimeout) {
		t.Fatalf("caller cancellation error = %v", err)
	}
}

func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return newClient(origin, server.Client(), time.Now)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func transportClient(function roundTripFunc) *Client {
	origin, _ := url.Parse("https://api.github.com")
	return newClient(origin, &http.Client{Transport: function}, time.Now)
}

func writeJSON(writer http.ResponseWriter, body string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(writer, body)
}

func jsonHeader() http.Header { return http.Header{"Content-Type": {"application/json"}} }

func ownerFixture(extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return `{"id":1,"login":"acme","type":"Organization","name":null,"avatar_url":"https://example.test/avatar","html_url":"https://example.test/acme","company":null,"blog":"","location":null,"bio":null,"public_repos":2,"followers":3,"following":4,"created_at":"2026-07-30T12:00:00.000Z","updated_at":"2026-07-30T12:01:00.000Z"` + extra + `}`
}

func repositorySummaryFixture(name string) string {
	return fmt.Sprintf(
		`{"id":1,"name":%q,"full_name":%q,"description":null,"html_url":"https://example.test/acme/%s","fork":false,"private":false,"visibility":"public"}`,
		name,
		"acme/"+name,
		name,
	)
}

func paddedOwnerFixture(t *testing.T, size int) string {
	t.Helper()
	base := ownerFixture(`"padding":""`)
	padding := size - len(base)
	if padding < 0 {
		t.Fatal("owner fixture exceeds requested size")
	}
	fixture := strings.Replace(base, `"padding":""`, `"padding":"`+strings.Repeat("x", padding)+`"`, 1)
	if len(fixture) != size {
		t.Fatalf("fixture size = %d, want %d", len(fixture), size)
	}
	return fixture
}

func TestCanonicalHeaderIntegerRejectsAmbiguousValues(t *testing.T) {
	for _, header := range []http.Header{
		{"Retry-After": {"01"}},
		{"Retry-After": {"1, 2"}},
		{"Retry-After": {"1", "2"}},
		{"Retry-After": {strconv.FormatUint(maximumSafeInteger+1, 10)}},
	} {
		if _, ok := canonicalHeaderInteger(header, "Retry-After"); ok {
			t.Fatalf("accepted ambiguous quota header %#v", header)
		}
	}
}
