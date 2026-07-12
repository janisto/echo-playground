---
name: pagination-endpoint
description: Create or review cursor-paginated Echo v5 list endpoints in echo-playground using its opaque cursor helper, validation, filtering, RFC 8288 Link headers, and pagination tests.
---

# Pagination endpoints

Read `AGENTS.md`, `internal/platform/pagination/`, and `internal/http/v1/items/` before editing cursor-paginated list
endpoints in the root Echo application.

## Scope and architecture

The existing `pagination.Paginate` helper paginates an already ordered in-memory slice. It is appropriate for this
playground's item example and small bounded collections. It is not a database pagination abstraction.

For Firestore or another persistent store, implement pagination in the service or repository using a stable,
deterministic order, the datastore's cursor primitives, and `limit + 1`. Do not load an unbounded collection merely to
reuse the slice helper. Keep opaque HTTP cursor encoding separate from storage details when those contracts differ.

## Input and handler pattern

```go
type ListInput struct {
	Cursor   string `query:"cursor"`
	Limit    int    `query:"limit"    validate:"omitempty,min=1,max=100"`
	Category string `query:"category" validate:"omitempty,oneof=active inactive"`
}
```

Bind query parameters only, validate them, apply `pagination.DefaultLimit`, then decode and type-check the cursor:

```go
var input ListInput
if err := echo.BindQueryParams(c, &input); err != nil {
	return err
}
if err := c.Validate(&input); err != nil {
	return err
}

limit := input.Limit
if limit == 0 {
	limit = pagination.DefaultLimit
}

cursor, err := pagination.DecodeCursor(input.Cursor)
if err != nil {
	return respond.Error400("invalid cursor format")
}
if cursor.Type != "" && cursor.Type != resourceCursorType {
	return respond.Error400("cursor type mismatch")
}
```

Use a stable, endpoint-specific cursor type. Preserve active filter query parameters; `Paginate` adds the effective
limit and replaces the cursor itself:

```go
query := url.Values{}
if input.Category != "" {
	query.Set("category", input.Category)
}

result, err := pagination.Paginate(
	filtered,
	cursor,
	limit,
	resourceCursorType,
	func(resource Resource) string { return resource.ID },
	c.Request().URL.Path,
	query,
)
if err != nil {
	return respond.Error400("cursor references unknown item")
}
if result.LinkHeader != "" {
	c.Response().Header().Set("Link", result.LinkHeader)
}
return respond.Negotiate(c, http.StatusOK, ListData{
	Resources: result.Items,
	Total:     result.Total,
})
```

Malformed, cross-endpoint, and stale cursors are client input errors and return 400. Validation failures such as an
out-of-range limit return 422 through the project validator.

The `Link` header follows RFC 8288 and can include `next` and `prev`. Cursor values are opaque URL-safe Base64; clients
must not construct or interpret them. Do not set an empty Link header.

## Verification

Cover the first, middle, and last page; empty results; default, minimum, and maximum limits; stable ordering; preserved
filters and limit; next and previous links; malformed, wrong-type, and unknown cursors; and JSON or CBOR responses.
Assert decoded page contents as well as the status and Link relations.

Run the focused package tests, then `just build`, `just test`, and `just lint`.
