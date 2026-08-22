---
name: echo-endpoint
description: Create or change Echo v5 portable API endpoints in echo-playground, including strict JSON or CBOR input, closed queries, authentication, Problem Details, OpenAPI generation, and adversarial tests.
---

# Echo 5.3 endpoints

Read `AGENTS.md`, neighboring handlers, `internal/http/v1/routes/routes.go`, and the relevant platform and service
contracts. Do not apply this skill to the independent `functions/` module.

## Design boundary

- Put transport DTOs and handlers under `internal/http/v1/<resource>/`; keep business, provider, and persistence work
  behind focused interfaces under `internal/service/`.
- Reuse `internal/platform/`. Do not add a generic controller layer, caller-selectable provider URL, ambient credential,
  compatibility alias, or test branch in production code.
- Register public routes directly on the v1 group. Register profile routes only on the group protected by
  `auth.Middleware`. Keep health and `/openapi.json` unversioned.
- Apply `respond.SuccessNegotiation` before dependency work. Body-free handlers do not consume request content.

## Input and output

Bind only the declared source. Use `request.ParseQuery` or `RejectUnknownOrRepeatedQuery` for closed scalar queries and
the route's explicit path grammar. For the three portable write operations, use `request.Decode` followed by
normalization selected by the contract and `c.Validate`.

`request.Decode` accepts exactly one closed JSON or CBOR object, distinguishes syntax 400 from schema 422, enforces
exact media and content-coding rules, and preserves explicit null versus omission through presence-aware DTOs. The
application body-limit middleware applies the exact 1,000,000-byte limit only after a supported body-bearing method is
selected. Never replace these boundaries with generic `c.Bind`.

Use `respond.Negotiate` for JSON/CBOR success bodies and `c.NoContent` for 204. Return stable helpers such as
`InvalidRequest`, `Unauthorized`, `NotFound`, `ValidationFailed`, `DependencyUnavailable`, and `InternalError`.
Problems use `application/problem+json` or ordinary `application/cbor`. Keep raw dependency errors, rejected values,
credentials, profile data, and provider data out of responses and logs.

Preserve caller cancellation. Map controlled dependency outages to their stable sentinel and status. Log once with a
stable operation or reason and the request-scoped observability logger. Do not forward the selected request ID or any
inbound field to GitHub unless the fixed outbound allowlist explicitly permits it.

## OpenAPI and verification

Update the handler's native Swag annotations, `cmd/openapi/` normalization, and semantic tests together. Run
`just fmt-openapi`, `just docs`, and `just openapi-check` whenever portable paths, parameters, bodies, statuses, media,
headers, security, or models change.

Apply `$adversarial-testing` and `$go-testing`. At the real Echo boundary, cover success and exact output, minimum and
maximum bounds, malformed and ambiguous input, negotiation, authentication, dependency failure, cancellation or
deadline, and forbidden side effects. Assert runtime and generated OpenAPI independently. Run the narrow package first,
then `just check`; use `just contract-smoke` against an already-running local server for real-HTTP readiness.
