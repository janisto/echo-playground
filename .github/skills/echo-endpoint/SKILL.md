---
name: echo-endpoint
description: Guide for creating Echo v5 API endpoints following this project's conventions including routing, input/output structs, error handling, and OpenAPI documentation.
---

# Echo v5 Endpoint Creation

Use this skill when creating new API endpoints for this Echo v5 REST API application.

For comprehensive coding guidelines, see `AGENTS.md` in the repository root.

## Router Setup

Create route handlers in `internal/http/v1/` and register them in `routes.go`:

```go
// internal/http/v1/routes/routes.go
func Register(v1 *echo.Group, verifier auth.Verifier, svc profilesvc.Service) {
    hello.Register(v1)
    items.Register(v1)
    // Add new routes here

    protected := v1.Group("", auth.Middleware(verifier))
    profile.Register(protected, svc)
}
```

Note: The health endpoint is a plain Echo handler registered at the root level in `main.go`, not via the v1 group.

## Handler Pattern

All handlers use `func(c *echo.Context) error` signature:

```go
import "github.com/janisto/echo-playground/internal/platform/respond"

func getHandler(c *echo.Context) error {
    return respond.Negotiate(c, http.StatusOK, Data{Message: "Hello, World!"})
}

func Register(g *echo.Group) {
    g.GET("/hello", getHandler)
}
```

## Input Struct Pattern

Use `json`, `query`, `param`, and `validate` tags:

```go
// Path parameters
type ResourceGetInput struct {
    ID string `param:"id" validate:"required"`
}

// Query parameters
type ResourceListInput struct {
    Status string `query:"status" validate:"omitempty,oneof=active inactive"`
    Limit  int    `query:"limit"  validate:"omitempty,min=1,max=100"`
}

// Request body
type ResourceCreateInput struct {
    Name string `json:"name" validate:"required,min=1,max=100"`
}
```

## GET Endpoint

```go
func getHandler(c *echo.Context) error {
    var input ResourceGetInput
    if err := c.Bind(&input); err != nil {
        return err
    }
    if err := c.Validate(&input); err != nil {
        return err
    }

    resource, err := getResource(input.ID)
    if err != nil {
        return respond.Error404("resource not found")
    }
    return respond.Negotiate(c, http.StatusOK, resource)
}
```

## POST Endpoint with 201 Created

```go
func createHandler(c *echo.Context) error {
    var input ResourceCreateInput
    if err := c.Bind(&input); err != nil {
        return err
    }
    if err := c.Validate(&input); err != nil {
        return err
    }

    resource := createResource(input.Name)
    c.Response().Header().Set("Location", fmt.Sprintf("/resources/%s", resource.ID))
    return respond.Negotiate(c, http.StatusCreated, resource)
}
```

## PUT/PATCH Endpoint

```go
func updateHandler(c *echo.Context) error {
    var input ResourceUpdateInput
    if err := c.Bind(&input); err != nil {
        return err
    }
    if err := c.Validate(&input); err != nil {
        return err
    }

    resource, err := updateResource(input.ID, input.Name)
    if err != nil {
        return respond.Error404("resource not found")
    }
    return respond.Negotiate(c, http.StatusOK, resource)
}
```

## DELETE Endpoint

Return 204 No Content for successful deletions:

```go
func deleteHandler(c *echo.Context) error {
    var input ResourceDeleteInput
    if err := c.Bind(&input); err != nil {
        return err
    }

    if err := deleteResource(input.ID); err != nil {
        return respond.Error404("resource not found")
    }
    return c.NoContent(http.StatusNoContent)
}
```

## Error Handling

Use custom error helpers for RFC 9457 Problem Details:

```go
import "github.com/janisto/echo-playground/internal/platform/respond"

respond.Error400("invalid request")
respond.Error401("unauthorized")
respond.Error403("access denied")
respond.Error404("resource not found")
respond.Error409("resource already exists")
respond.Error422("validation failed", fieldErrors...)
respond.Error500("internal error")
respond.NewError(http.StatusTeapot, "custom message")
```

## Logging

Use context-aware slog helpers:

```go
import (
    "log/slog"
    applog "github.com/janisto/echo-playground/internal/platform/logging"
)

func handler(c *echo.Context) error {
    ctx := c.Request().Context()
    applog.LogInfo(ctx, "processing request", slog.String("id", input.ID))

    if err != nil {
        applog.LogError(ctx, "operation failed", err, slog.String("id", input.ID))
        return respond.Error500("operation failed")
    }

    return respond.Negotiate(c, http.StatusOK, result)
}
```

## Model Struct Pattern

Use `json` and `example` tags for response models:

```go
import "github.com/janisto/echo-playground/internal/platform/timeutil"

type ResourceData struct {
    ID        string        `json:"id"        example:"res-001"`
    Name      string        `json:"name"      example:"My Resource"`
    CreatedAt timeutil.Time `json:"createdAt" example:"2024-01-15T10:30:00.000Z"`
}
```

## Status Code Reference

| Method | Success Status | Use Case |
|--------|----------------|----------|
| GET    | 200 OK         | Retrieve resource(s) |
| POST   | 201 Created    | Create resource (include Location header) |
| PUT    | 200 OK         | Replace resource |
| PATCH  | 200 OK         | Partial update |
| DELETE | 204 No Content | Remove resource |

## Error Status Codes

| Status | Use Case |
|--------|----------|
| 400    | Malformed syntax, invalid cursor |
| 401    | Missing authentication |
| 403    | Authenticated but not authorized |
| 404    | Resource not found |
| 405    | HTTP method not supported for resource |
| 409    | Conflict (duplicate resource) |
| 422    | Validation failures |
| 500    | Unexpected server error |
