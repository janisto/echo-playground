---
name: pagination-endpoint
description: Guide for creating paginated list endpoints with cursor-based pagination and RFC 8288 Link headers following this project's conventions.
---

# Pagination Endpoint Creation

Use this skill when creating paginated list endpoints for this Echo v5 REST API application.

For comprehensive pagination guidelines, see `AGENTS.md` in the repository root.

## Pagination Package

The project uses cursor-based pagination via `internal/platform/pagination`:

- `pagination.Cursor` - Decoded cursor with type and value
- `pagination.Paginate` - Generic pagination helper
- `pagination.DecodeCursor` / `Cursor.Encode()` - Cursor encoding/decoding

## Input Struct Pattern

Use `query` and `validate` tags for pagination query parameters:

```go
type ListResourcesInput struct {
    Cursor   string `query:"cursor"`
    Limit    int    `query:"limit"    validate:"omitempty,min=1,max=100"`
    Category string `query:"category" validate:"omitempty,oneof=active inactive"`
}
```

## Handler Implementation

```go
const resourceCursorType = "resource"

func listHandler(c *echo.Context) error {
    var input ListResourcesInput
    if err := c.Bind(&input); err != nil {
        return err
    }
    if err := c.Validate(&input); err != nil {
        return err
    }

    // 1. Apply default limit
    limit := input.Limit
    if limit == 0 {
        limit = pagination.DefaultLimit
    }

    // 2. Decode and validate cursor
    cursor, err := pagination.DecodeCursor(input.Cursor)
    if err != nil {
        return respond.Error400("invalid cursor format")
    }

    // 3. Validate cursor type matches endpoint
    if cursor.Type != "" && cursor.Type != resourceCursorType {
        return respond.Error400("cursor type mismatch")
    }

    // 4. Apply filters
    filtered := filterResources(allResources, input.Category)

    // 5. Validate cursor references existing item
    if cursor.Value != "" && !resourceExists(filtered, cursor.Value) {
        return respond.Error400("cursor references unknown item")
    }

    // 6. Build query params for Link header
    query := url.Values{}
    if input.Category != "" {
        query.Set("category", input.Category)
    }

    // 7. Paginate using helper
    result := pagination.Paginate(
        filtered,
        cursor,
        limit,
        resourceCursorType,
        func(r Resource) string { return r.ID },
        "/resources",
        query,
    )

    // 8. Set Link header and return
    c.Response().Header().Set("Link", result.LinkHeader)
    return respond.Negotiate(c, http.StatusOK, ResourcesData{
        Resources: result.Items,
        Total:     result.Total,
    })
}
```

## Cursor Validation Rules

Invalid cursors MUST return 400 Bad Request:

```go
// Decode error (malformed base64, invalid JSON)
cursor, err := pagination.DecodeCursor(input.Cursor)
if err != nil {
    return respond.Error400("invalid cursor format")
}

// Type mismatch (cursor from different endpoint)
if cursor.Type != "" && cursor.Type != resourceCursorType {
    return respond.Error400("cursor type mismatch")
}

// Invalid reference (cursor points to deleted/nonexistent item)
if cursor.Value != "" && !resourceExists(filtered, cursor.Value) {
    return respond.Error400("cursor references unknown item")
}
```

## Cursor Type Constants

Define a constant for each paginated endpoint to prevent cursor reuse:

```go
const (
    itemCursorType     = "item"
    resourceCursorType = "resource"
    userCursorType     = "user"
)
```

## Pagination Helper

The `pagination.Paginate` function handles:
- Finding start position from cursor
- Slicing items to requested limit
- Generating next cursor
- Building RFC 8288 Link header

```go
result := pagination.Paginate(
    items,           // []T - full filtered slice
    cursor,          // Cursor - decoded cursor
    limit,           // int - items per page
    cursorType,      // string - cursor type constant
    getID,           // func(T) string - ID extractor
    basePath,        // string - endpoint path for links
    query,           // url.Values - preserved query params
)

// result.Items      - []T paginated items
// result.Total      - int total count before pagination
// result.LinkHeader - string RFC 8288 Link header
```

## Link Header Format

The Link header follows RFC 8288:

```
Link: </resources?cursor=eyJ0Ijoi...>; rel="next"
```

Multiple links are comma-separated:

```
Link: </resources?cursor=abc>; rel="next", </resources>; rel="first"
```

## Filter Helper Pattern

Create filter functions for query parameters:

```go
func filterResources(resources []Resource, category string) []Resource {
    if category == "" {
        return resources
    }
    return slices.DeleteFunc(slices.Clone(resources), func(r Resource) bool {
        return r.Category != category
    })
}
```

## Testing Paginated Endpoints

```go
func TestListResources_Pagination(t *testing.T) {
    e := setupTestServer()

    req := httptest.NewRequest(http.MethodGet, "/v1/resources?limit=5", nil)
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }

    link := rec.Header().Get("Link")
    if !strings.Contains(link, `rel="next"`) {
        t.Error("expected next link")
    }

    var body ResourcesData
    json.Unmarshal(rec.Body.Bytes(), &body)
    if len(body.Resources) != 5 {
        t.Errorf("expected 5 items, got %d", len(body.Resources))
    }
}

func TestListResources_InvalidCursor(t *testing.T) {
    e := setupTestServer()

    req := httptest.NewRequest(http.MethodGet, "/v1/resources?cursor=invalid", nil)
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req)

    if rec.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rec.Code)
    }
}
```

## Complete Example

See `internal/http/v1/items/handler.go` for a complete implementation.
