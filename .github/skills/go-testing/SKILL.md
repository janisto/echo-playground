---
name: go-testing
description: Guide for writing Go tests following this project's patterns including httptest, echotest, test organization, and coverage requirements.
---

# Go Testing

Use this skill when writing tests for this Echo v5 REST API application.

For comprehensive testing guidelines, see `AGENTS.md` in the repository root.

## Test Organization

Tests are colocated with source files using `_test.go` suffix:

```
internal/
    http/
        v1/
            routes/
                routes.go
                routes_test.go
            items/
                handler.go
                handler_test.go
    platform/
        logging/
            middleware.go
            middleware_test.go
```

## Integration Test Server Setup

Create test servers using Echo:

```go
package routes_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/labstack/echo/v5"

    "github.com/janisto/echo-playground/internal/http/health"
    "github.com/janisto/echo-playground/internal/http/v1/routes"
    applog "github.com/janisto/echo-playground/internal/platform/logging"
    appmiddleware "github.com/janisto/echo-playground/internal/platform/middleware"
    "github.com/janisto/echo-playground/internal/platform/respond"
    "github.com/janisto/echo-playground/internal/platform/validate"
)

func setupTestServer() *echo.Echo {
    e := echo.New()
    e.Validator = validate.New()
    e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
    e.Use(
        appmiddleware.RequestID(),
        applog.RequestLogger(),
        respond.Recoverer(),
    )

    e.GET("/health", health.Handler)

    v1 := e.Group("/v1")
    routes.Register(v1, verifier, svc)
    return e
}
```

## Handler Unit Tests (echotest)

Use Echo's `echotest` package for isolated handler tests:

```go
import "github.com/labstack/echo/v5/echotest"

func TestGetHandler(t *testing.T) {
    rec := echotest.ContextConfig{
        Request: httptest.NewRequest(http.MethodGet, "/items?limit=10", nil),
        QueryValues: url.Values{"limit": {"10"}, "category": {"electronics"}},
    }.ServeWithHandler(t, handler, respond.NewHTTPErrorHandler())

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }
}
```

## Basic Integration Test Pattern

```go
func TestHealthEndpoint(t *testing.T) {
    e := setupTestServer()

    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    req.Header.Set("X-Request-ID", "test-trace-id")
    rec := httptest.NewRecorder()

    e.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }

    var body health.Response
    if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
        t.Fatalf("failed to decode response: %v", err)
    }
    if body.Status != "healthy" {
        t.Fatalf("unexpected status: %s", body.Status)
    }
}
```

## Testing Error Responses

Verify RFC 9457 Problem Details format:

```go
import "github.com/janisto/echo-playground/internal/platform/respond"

func TestNotFoundReturns404(t *testing.T) {
    e := setupTestServer()

    req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
    rec := httptest.NewRecorder()

    e.ServeHTTP(rec, req)

    if rec.Code != http.StatusNotFound {
        t.Fatalf("expected 404, got %d", rec.Code)
    }

    var problem respond.ProblemDetails
    if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
        t.Fatalf("failed to unmarshal problem: %v", err)
    }
    if problem.Status != http.StatusNotFound {
        t.Fatalf("expected status 404, got %d", problem.Status)
    }
    if problem.Title != "Not Found" {
        t.Fatalf("unexpected title: %s", problem.Title)
    }
}
```

## Testing POST Requests

```go
func TestCreateResource(t *testing.T) {
    e := setupTestServer()

    body := `{"name": "Test Resource"}`
    req := httptest.NewRequest(http.MethodPost, "/v1/resources", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()

    e.ServeHTTP(rec, req)

    if rec.Code != http.StatusCreated {
        t.Fatalf("expected 201, got %d", rec.Code)
    }

    location := rec.Header().Get("Location")
    if location == "" {
        t.Fatal("expected Location header")
    }
}
```

## Testing Validation Errors

```go
func TestValidationReturns422(t *testing.T) {
    e := setupTestServer()

    body := `{"name": ""}` // Empty name should fail validation
    req := httptest.NewRequest(http.MethodPost, "/v1/resources", strings.NewReader(body))
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
    if len(problem.Errors) == 0 {
        t.Fatal("expected validation errors")
    }
}
```

## Table-Driven Tests

Use subtests for comprehensive coverage:

```go
func TestListItems(t *testing.T) {
    e := setupTestServer()

    tests := []struct {
        name       string
        query      string
        wantStatus int
        wantItems  int
    }{
        {"default limit", "", http.StatusOK, 20},
        {"custom limit", "?limit=5", http.StatusOK, 5},
        {"filter category", "?category=electronics", http.StatusOK, 10},
        {"invalid cursor", "?cursor=invalid", http.StatusBadRequest, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest(http.MethodGet, "/v1/items"+tt.query, nil)
            rec := httptest.NewRecorder()

            e.ServeHTTP(rec, req)

            if rec.Code != tt.wantStatus {
                t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
            }
        })
    }
}
```

## Testing Link Headers

For paginated endpoints:

```go
func TestPaginationLinkHeader(t *testing.T) {
    e := setupTestServer()

    req := httptest.NewRequest(http.MethodGet, "/v1/items?limit=5", nil)
    rec := httptest.NewRecorder()

    e.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }

    link := rec.Header().Get("Link")
    if link == "" {
        t.Fatal("expected Link header for pagination")
    }

    if !strings.Contains(link, `rel="next"`) {
        t.Error("expected next link in Link header")
    }
}
```

## Testing Content Negotiation

```go
func TestCBORResponse(t *testing.T) {
    e := setupTestServer()

    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    req.Header.Set("Accept", "application/cbor")
    rec := httptest.NewRecorder()

    e.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }

    contentType := rec.Header().Get("Content-Type")
    if !strings.Contains(contentType, "application/cbor") {
        t.Errorf("expected CBOR content type, got %s", contentType)
    }
}
```

## Test Fixture Loading (echotest)

```go
expected := echotest.LoadBytes(t, "testdata/expected.json", echotest.TrimNewlineEnd)
assert.JSONEq(t, string(expected), rec.Body.String())
```

## Test Naming Convention

Pattern: `Test<Function>_<Scenario>` or `Test<Endpoint>Returns<Status><Condition>`

```go
func TestHealthEndpoint(t *testing.T) { ... }
func TestCreateResource_Returns201OnSuccess(t *testing.T) { ... }
func TestGetResource_Returns404WhenNotFound(t *testing.T) { ... }
func TestListItems_WithInvalidCursor_Returns400(t *testing.T) { ... }
```

## Running Tests

```bash
# Run all tests
go test ./...

# Verbose output
go test -v ./...

# With coverage
go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...

# Coverage report
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Coverage Requirements

Tests should cover:
- Success paths (200, 201, 204)
- Error paths (400, 404, 422, 500)
- Edge cases (empty input, boundary values)
- Problem Details format verification
- Trace ID propagation
- Content negotiation (JSON/CBOR)

## Important Notes

- Always set `X-Request-ID` header for trace testing
- Verify response Content-Type matches Accept header
- Check Problem Details structure for all error responses
- Test both valid and invalid enum values
- Verify Location header for 201 Created responses
