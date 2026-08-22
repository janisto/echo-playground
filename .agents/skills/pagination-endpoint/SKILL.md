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

Parse the raw query as one closed scalar source. `request.Limit` applies the default and exact unsigned-decimal
`1..100` grammar. Distinguish an absent optional filter from a present empty value so an enum-constrained filter cannot
silently become unfiltered:

```go
query, err := request.ParseQuery(c, "cursor", "limit", "category")
if err != nil {
	return err
}
limit, err := request.Limit(query)
if err != nil {
	return err
}

category := ""
if values, present := query["category"]; present {
	category = values[0]
	if !validCategory(category) {
		return respond.ValidationFailed(respond.ErrorDetail{
			Detail: "category is not supported",
			Source: &respond.ErrorSource{Parameter: "category"},
		})
	}
}
```

Bind every result-shaping value into an endpoint-specific `Scope`. Include owner and repository for nested provider
collections. Decode only a present cursor and require its complete scope to match before pagination:

```go
scope := pagination.Scope{
	Operation: operationID,
	Filter:    category,
	Limit:     limit,
}
var cursor *pagination.Cursor
if values, present := query["cursor"]; present {
	decoded, decodeErr := pagination.DecodeCursor(values[0])
	if decodeErr != nil || !decoded.Matches(scope) {
		return respond.InvalidRequest()
	}
	cursor = &decoded
}
```

Preserve active filter query parameters; `Paginate` receives the scoped cursor, adds the effective limit, and replaces
the cursor in each generated Link target:

```go
linkQuery := url.Values{}
if category != "" {
	linkQuery.Set("category", category)
}

result, err := pagination.Paginate(
	filtered,
	cursor,
	scope,
	func(resource Resource) string { return resource.ID },
	c.Request().URL.Path,
	linkQuery,
)
if err != nil {
	return respond.InvalidRequest()
}
if result.LinkHeader != "" {
	c.Response().Header().Set("Link", result.LinkHeader)
}
return respond.Negotiate(c, http.StatusOK, ListData{
	Resources: result.Items,
	Total:     result.Total,
})
```

Malformed, empty, oversized, cross-endpoint, cross-filter, and stale cursors are client input errors and return 400.
Unknown and repeated scalar query keys also return 400. A present empty enum filter and an empty, nondecimal, or
out-of-range limit are valid-syntax validation failures and return 422.

The `Link` header follows RFC 8288 and can include `next` and `prev`. Cursor values are opaque URL-safe Base64; clients
must not construct or interpret them. Do not set an empty Link header.

## Verification

Cover the first, middle, and last page; empty results; default, minimum, and maximum limits; stable ordering; preserved
filters and limit; next and previous links; malformed, empty, noncanonical, wrong-operation, wrong-scope, stale, and
oversized cursors; unknown query keys; repeated known keys; and JSON or CBOR responses.
Assert decoded page contents as well as the status and Link relations.

Run the focused package tests, then `just build`, `just test`, and `just lint`.
