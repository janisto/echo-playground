package github

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/janisto/echo-playground/internal/platform/pagination"
	"github.com/janisto/echo-playground/internal/platform/strictjson"
)

const (
	providerOrigin       = "https://api.github.com"
	providerAPIVersion   = "2026-03-10"
	providerUserAgent    = "echo-playground"
	providerTimeout      = 10 * time.Second
	maximumProviderBody  = 4_194_304
	maximumRedirectCount = 3
)

type Client struct {
	origin     *url.URL
	httpClient *http.Client
	clock      func() time.Time
	timeout    time.Duration
}

func NewClient() *Client {
	origin, err := url.Parse(providerOrigin)
	if err != nil {
		panic("invalid fixed GitHub origin: " + err.Error())
	}
	return newClient(origin, http.DefaultClient, time.Now)
}

func newClient(origin *url.URL, httpClient *http.Client, clock func() time.Time) *Client {
	copyClient := *httpClient
	copyClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{origin: cloneURL(origin), httpClient: &copyClient, clock: clock, timeout: providerTimeout}
}

func (c *Client) GetOwner(ctx context.Context, owner string) (Owner, error) {
	if !validProviderOwner(owner) {
		return Owner{}, ErrUpstream
	}
	spec := c.pointSpec("/users/"+owner, "/user/", "")
	return executeProjected(c, ctx, spec, func(document []byte, _ http.Header) (Owner, error) {
		return projectOwner(document)
	})
}

func (c *Client) ListOwnerRepositories(
	ctx context.Context,
	owner string,
	limit int,
	cursor *pagination.Cursor,
) (Page[RepositorySummary], error) {
	if !validProviderOwner(owner) {
		return Page[RepositorySummary]{}, ErrUpstream
	}
	scope := pagination.Scope{Operation: "listGitHubOwnerRepositories", Owner: owner, Limit: limit}
	query := url.Values{
		"type":      {"owner"},
		"sort":      {"full_name"},
		"direction": {"asc"},
		"per_page":  {strconv.Itoa(limit)},
	}
	current, err := applyNumberedCursor(query, scope, cursor)
	if err != nil {
		return Page[RepositorySummary]{}, err
	}
	spec := c.pageSpec("/users/"+owner+"/repos", "/user/", "/repos", query, numberedPagination, current, scope)
	return executeProjected(c, ctx, spec, func(document []byte, headers http.Header) (Page[RepositorySummary], error) {
		entries, err := projectRepositorySummaries(document, limit)
		if err != nil {
			return Page[RepositorySummary]{}, err
		}
		navigation, err := parseNavigation(headers, spec, len(entries) == 0)
		return Page[RepositorySummary]{
			Entries:    entries,
			NextCursor: navigation.nextCursor,
			PrevCursor: navigation.prevCursor,
		}, err
	})
}

func (c *Client) GetRepository(ctx context.Context, owner, repository string) (Repository, error) {
	if !validProviderOwner(owner) || !validProviderRepository(repository) {
		return Repository{}, ErrUpstream
	}
	spec := c.pointSpec("/repos/"+owner+"/"+repository, "/repositories/", "")
	return executeProjected(c, ctx, spec, func(document []byte, _ http.Header) (Repository, error) {
		return projectRepository(document)
	})
}

func (c *Client) ListRepositoryActivity(
	ctx context.Context,
	owner, repository string,
	limit int,
	cursor *pagination.Cursor,
) (Page[Activity], error) {
	if !validProviderOwner(owner) || !validProviderRepository(repository) {
		return Page[Activity]{}, ErrUpstream
	}
	scope := pagination.Scope{
		Operation:  "listGitHubRepositoryActivity",
		Owner:      owner,
		Repository: repository,
		Limit:      limit,
	}
	query := url.Values{"direction": {"desc"}, "per_page": {strconv.Itoa(limit)}}
	current, err := applyActivityCursor(query, scope, cursor)
	if err != nil {
		return Page[Activity]{}, err
	}
	spec := c.pageSpec(
		"/repos/"+owner+"/"+repository+"/activity",
		"/repositories/",
		"/activity",
		query,
		activityPagination,
		current,
		scope,
	)
	return executeProjected(c, ctx, spec, func(document []byte, headers http.Header) (Page[Activity], error) {
		entries, err := projectActivities(document, limit)
		if err != nil {
			return Page[Activity]{}, err
		}
		navigation, err := parseNavigation(headers, spec, len(entries) == 0)
		return Page[Activity]{
			Entries:    entries,
			NextCursor: navigation.nextCursor,
			PrevCursor: navigation.prevCursor,
		}, err
	})
}

func (c *Client) ListRepositoryLanguages(ctx context.Context, owner, repository string) ([]Language, error) {
	if !validProviderOwner(owner) || !validProviderRepository(repository) {
		return nil, ErrUpstream
	}
	spec := c.pointSpec("/repos/"+owner+"/"+repository+"/languages", "/repositories/", "/languages")
	return executeProjected(c, ctx, spec, func(document []byte, _ http.Header) ([]Language, error) {
		return projectLanguages(document)
	})
}

func (c *Client) ListRepositoryTags(
	ctx context.Context,
	owner, repository string,
	limit int,
	cursor *pagination.Cursor,
) (Page[Tag], error) {
	if !validProviderOwner(owner) || !validProviderRepository(repository) {
		return Page[Tag]{}, ErrUpstream
	}
	scope := pagination.Scope{Operation: "listGitHubRepositoryTags", Owner: owner, Repository: repository, Limit: limit}
	query := url.Values{"per_page": {strconv.Itoa(limit)}}
	current, err := applyNumberedCursor(query, scope, cursor)
	if err != nil {
		return Page[Tag]{}, err
	}
	spec := c.pageSpec(
		"/repos/"+owner+"/"+repository+"/tags",
		"/repositories/",
		"/tags",
		query,
		numberedPagination,
		current,
		scope,
	)
	return executeProjected(c, ctx, spec, func(document []byte, headers http.Header) (Page[Tag], error) {
		entries, err := projectTags(document, limit)
		if err != nil {
			return Page[Tag]{}, err
		}
		navigation, err := parseNavigation(headers, spec, len(entries) == 0)
		return Page[Tag]{Entries: entries, NextCursor: navigation.nextCursor, PrevCursor: navigation.prevCursor}, err
	})
}

func (c *Client) pointSpec(namedPath, numericPrefix, numericSuffix string) providerSpec {
	return providerSpec{
		origin:        c.origin,
		namedPath:     namedPath,
		numericPrefix: numericPrefix,
		numericSuffix: numericSuffix,
		query:         url.Values{},
	}
}

func (c *Client) pageSpec(
	namedPath, numericPrefix, numericSuffix string,
	query url.Values,
	paginationKind paginationKind,
	current string,
	scope pagination.Scope,
) providerSpec {
	return providerSpec{
		origin: c.origin, namedPath: namedPath, numericPrefix: numericPrefix, numericSuffix: numericSuffix,
		query: query, pagination: paginationKind, currentValue: current, scope: scope,
	}
}

func executeProjected[T any](
	c *Client,
	parent context.Context,
	spec providerSpec,
	project func([]byte, http.Header) (T, error),
) (T, error) {
	var zero T
	operationContext, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()
	document, headers, err := c.fetch(operationContext, spec)
	if err == nil {
		result, projectionError := project(document, headers)
		if projectionError == nil && parent.Err() == nil && operationContext.Err() == nil {
			return result, nil
		}
		err = projectionError
	}
	if parent.Err() != nil {
		return zero, parent.Err()
	}
	if errors.Is(operationContext.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return zero, ErrTimeout
	}
	var rateLimit *RateLimitError
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUpstream) || errors.As(err, &rateLimit) {
		return zero, err
	}
	return zero, ErrUpstream
}

func (c *Client) fetch(ctx context.Context, spec providerSpec) ([]byte, http.Header, error) {
	target := cloneURL(c.origin)
	target.Path = spec.namedPath
	target.RawPath = ""
	target.RawQuery = spec.query.Encode()
	visited := map[string]struct{}{target.String(): {}}

	for redirects := 0; ; redirects++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, nil, ErrUpstream
		}
		request.Header = http.Header{
			"Accept":               {"application/vnd.github+json"},
			"X-Github-Api-Version": {providerAPIVersion},
			"User-Agent":           {providerUserAgent},
			"Accept-Encoding":      {"identity"},
		}
		response, err := c.httpClient.Do(request)
		if err != nil {
			return nil, nil, err
		}
		if !identityEncoded(response.Header) {
			closeResponse(response)
			return nil, nil, ErrUpstream
		}
		if isRedirect(response.StatusCode) {
			if redirects >= maximumRedirectCount {
				closeResponse(response)
				return nil, nil, ErrUpstream
			}
			next, err := redirectTarget(target, response.Header, spec)
			closeResponse(response)
			if err != nil {
				return nil, nil, err
			}
			if _, loop := visited[next.String()]; loop {
				return nil, nil, ErrUpstream
			}
			visited[next.String()] = struct{}{}
			target = next
			continue
		}
		switch response.StatusCode {
		case http.StatusOK:
			document, err := readSuccess(response)
			return document, response.Header.Clone(), err
		case http.StatusNotFound:
			closeResponse(response)
			return nil, nil, ErrNotFound
		case http.StatusForbidden, http.StatusTooManyRequests:
			rateLimit := c.rateLimitError(response.Header)
			closeResponse(response)
			return nil, nil, rateLimit
		default:
			closeResponse(response)
			return nil, nil, ErrUpstream
		}
	}
}

func (c *Client) rateLimitError(header http.Header) *RateLimitError {
	now := c.clock()
	retryValue, retryOK := canonicalHeaderInteger(header, "Retry-After")
	resetValue, resetOK := canonicalHeaderInteger(header, "X-RateLimit-Reset")
	var resetDelay uint64
	if resetOK {
		nowSeconds := now.Unix()
		if nowSeconds >= 0 && resetValue <= uint64(nowSeconds) {
			resetOK = false
		} else {
			//nolint:gosec // Modulo conversion makes subtraction exact for negative Unix seconds.
			resetDelay = resetValue - uint64(nowSeconds)
		}
	}

	retryAfter := "60"
	if retryOK {
		retryAfter = strconv.FormatUint(retryValue, 10)
	} else if resetOK {
		retryAfter = strconv.FormatUint(resetDelay, 10)
	}
	reset := ""
	if resetOK {
		reset = strconv.FormatUint(resetValue, 10)
	}
	return &RateLimitError{RetryAfter: retryAfter, Reset: reset}
}

func applyNumberedCursor(query url.Values, scope pagination.Scope, cursor *pagination.Cursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	if !cursor.Matches(scope) || !canonicalPageDecimal(cursor.Position) ||
		cursor.Direction == "next" && cursor.Position == "1" {
		return "", ErrInvalidCursor
	}
	query.Set("page", cursor.Position)
	return cursor.Position, nil
}

func applyActivityCursor(query url.Values, scope pagination.Scope, cursor *pagination.Cursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	if !cursor.Matches(scope) || !printableASCII(cursor.Position, pagination.MaxCursorLength) {
		return "", ErrInvalidCursor
	}
	member := "after"
	if cursor.Direction == "prev" {
		member = "before"
	}
	query.Set(member, cursor.Position)
	return cursor.Position, nil
}

func redirectTarget(current *url.URL, header http.Header, spec providerSpec) (*url.URL, error) {
	locations := header.Values("Location")
	if len(locations) != 1 || strings.TrimSpace(locations[0]) == "" {
		return nil, ErrUpstream
	}
	reference, err := url.Parse(strings.TrimSpace(locations[0]))
	if err != nil {
		return nil, ErrUpstream
	}
	target := current.ResolveReference(reference)
	if !validProviderTarget(target, spec) {
		return nil, ErrUpstream
	}
	return target, nil
}

func validProviderTarget(target *url.URL, spec providerSpec) bool {
	if target.Scheme != spec.origin.Scheme || target.Host != spec.origin.Host || target.User != nil ||
		target.Fragment != "" ||
		target.ForceQuery {
		return false
	}
	if target.Path != spec.namedPath && !matchesNumericPath(target.Path, spec.numericPrefix, spec.numericSuffix) {
		return false
	}
	if target.EscapedPath() != (&url.URL{Path: target.Path}).EscapedPath() {
		return false
	}
	query, err := url.ParseQuery(target.RawQuery)
	if err != nil {
		return false
	}
	for _, values := range query {
		if len(values) != 1 {
			return false
		}
	}
	return query.Encode() == spec.query.Encode()
}

func readSuccess(response *http.Response) ([]byte, error) {
	defer func() { _ = response.Body.Close() }()
	contentTypes := response.Header.Values("Content-Type")
	if len(contentTypes) != 1 || strings.Contains(contentTypes[0], ",") {
		return nil, ErrUpstream
	}
	mediaType, _, err := mime.ParseMediaType(contentTypes[0])
	if err != nil || mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
		return nil, ErrUpstream
	}
	if response.ContentLength > maximumProviderBody {
		return nil, ErrUpstream
	}
	document, err := io.ReadAll(io.LimitReader(response.Body, maximumProviderBody+1))
	if err != nil || len(document) > maximumProviderBody {
		return nil, ErrUpstream
	}
	if err := strictjson.Validate(document); err != nil {
		return nil, ErrUpstream
	}
	return document, nil
}

func closeResponse(response *http.Response) {
	_ = response.Body.Close()
}

func identityEncoded(header http.Header) bool {
	values := header.Values("Content-Encoding")
	return len(values) == 0 || len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "identity")
}

func canonicalHeaderInteger(header http.Header, name string) (uint64, bool) {
	values := header.Values(name)
	if len(values) != 1 {
		return 0, false
	}
	value := strings.TrimSpace(values[0])
	if !canonicalSafeDecimal(value) {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil && parsed <= maximumSafeInteger
}

func isRedirect(status int) bool {
	return status == http.StatusMovedPermanently || status == http.StatusFound || status == http.StatusSeeOther ||
		status == http.StatusTemporaryRedirect || status == http.StatusPermanentRedirect
}

func validProviderOwner(value string) bool {
	if len(value) < 1 || len(value) > 39 || !providerAlphaNumeric(value[0]) {
		return false
	}
	if len(value) == 1 {
		return true
	}
	if !providerAlphaNumeric(value[len(value)-1]) {
		return false
	}
	for index := 1; index < len(value)-1; index++ {
		if !providerAlphaNumeric(value[index]) && value[index] != '_' && value[index] != '-' {
			return false
		}
	}
	return true
}

func validProviderRepository(value string) bool {
	if len(value) < 1 || len(value) > 100 ||
		!strings.ContainsAny(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") {
		return false
	}
	for index := range len(value) {
		if !providerAlphaNumeric(value[index]) && value[index] != '.' && value[index] != '_' && value[index] != '-' {
			return false
		}
	}
	return true
}

func providerAlphaNumeric(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func cloneURL(source *url.URL) *url.URL {
	if source == nil {
		return nil
	}
	clone := *source
	return &clone
}

var _ Service = (*Client)(nil)
