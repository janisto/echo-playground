---
name: echo-endpoint
description: Create or change Echo v5 endpoints in echo-playground, including route registration, strict JSON input, validation, authentication, Problem Details, JSON or CBOR responses, OpenAPI, and tests.
---

# Echo v5 endpoints

Read `AGENTS.md`, the neighboring handler package, `internal/http/v1/routes/routes.go`, and the relevant platform or
service contracts before editing the root Echo application.

Do not apply this skill to `functions/`. That module is intentionally a separate, minimal Functions Framework example.

## Design boundary

- Put transport DTOs and handlers under `internal/http/v1/<resource>/`.
- Keep business and persistence behavior behind focused interfaces under `internal/service/` when the endpoint needs it.
- Reuse `internal/platform/` packages; do not introduce a handler framework or generic controller layer.
- Register public routes directly on the v1 group. Register protected routes on the group wrapped by
  `auth.Middleware` in `internal/http/v1/routes/routes.go`.
- Keep health and API documentation unversioned.

## Handler patterns

Echo v5 handlers receive `*echo.Context`:

```go
func Register(g *echo.Group) {
	g.GET("/resources/:id", getHandler)
}

func getHandler(c *echo.Context) error {
	var input GetInput
	if err := echo.BindPathParams(c, &input); err != nil {
		return err
	}
	if err := c.Validate(&input); err != nil {
		return err
	}

	return respond.Negotiate(c, http.StatusOK, Resource{ID: input.ID})
}
```

Bind only the source the endpoint accepts:

- path parameters: `echo.BindPathParams`
- query parameters: reject unknown keys with `request.RejectUnknownQuery`, then use `echo.BindQueryParams`
- JSON bodies: `request.DecodeJSON`, followed by `c.Validate`

`request.DecodeJSON` enforces `application/json`, exactly one top-level object, and known fields. It rejects `null`,
arrays, scalars, malformed input, and trailing values with 400. The composed application middleware applies the global
1 MiB body limit. CBOR is response-only. Do not replace the decoder with generic `c.Bind` or add a second decoder in a
handler.

Use `respond.Negotiate` for response bodies and `c.NoContent` for 204. Set a request-derived `Location` for 201:

```go
c.Response().Header().Set("Location", c.Request().URL.Path)
return respond.Negotiate(c, http.StatusCreated, resource)
```

Return expected errors with `respond.Error400`, `Error401`, `Error403`, `Error404`, `Error409`, `Error422`, or
`Error503`. Log unexpected internal errors once, without tokens or PII, then return `Error500`. Do not leak dependency
errors to clients.

Map temporary service failures to a stable `ErrUnavailable` sentinel and generic 503 response. Preserve cancellation
and error chains, log dependency failures once with a stable operation name, and keep raw SDK details out of responses.

Use `obs.Logger(c.Request().Context())` only for useful request-scoped events. The access logger already records every
request. Startup and background work must use the explicit process logger.

## OpenAPI contract

Add operation annotations to the handler and keep them consistent with runtime behavior:

- `@Produce json,application/cbor` for any endpoint with a response body or negotiated errors.
- a stable, unique `@ID` for every operation.
- `@Param body body <Input> true ...` for JSON bodies; omit `@Accept json` because of the tracked swag v2 issue.
- Document every reachable error status with `respond.ProblemDetails`.
- Add `@Security BearerAuth` to protected operations.
- Use paths relative to the `/v1` OpenAPI server.

Run `just docs` after annotations or documented DTOs change. Generated JSON and YAML are committed and semantically
checked.

## Verification

Add colocated tests for the success path, malformed and unknown JSON, validation, authentication or authorization,
service failures, negotiated JSON and CBOR, and response headers relevant to the endpoint. Apply
`$adversarial-testing`, then use `$go-testing` for repository test conventions.

Run the narrow test first, then:

```bash
just build
just test
just lint
```

Run `just docs` when the public contract changes. The unqualified build, test, and lint recipes cover both modules;
`just check` is the aggregate repository check.
