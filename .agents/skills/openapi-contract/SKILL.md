---
name: openapi-contract
description: Maintain and verify echo-playground's Swag v2 registration, deterministic OpenAPI 3.1.2 normalization, embedded discovery route, runtime agreement, and Swagger UI.
---

# OpenAPI contract maintenance

Read `AGENTS.md`, the affected runtime route and models, and `cmd/openapi/` before changing the public contract.

## Contract architecture

Handler annotations are the native operation-registration source. `cmd/openapi` validates that Swag output against the
portable inventory and deterministically normalizes the exact OpenAPI 3.1.2 contract into committed
`api-docs/swagger.json` and `api-docs/swagger.yaml`; `api-docs/embed.go` embeds the JSON; `internal/http/docs/` serves
those exact bytes at `/openapi.json` and points the optional `/api-docs` UI at that route.

Do not hand-edit generated files, bypass native annotation registration, fetch references at runtime, or introduce
another description. The normalizer must reject native method, path, operation-ID, response-status, parameter,
request-body, and security drift before it supplies semantics Swag cannot express. Runtime code and OpenAPI remain
independent evidence: inspect and test both.

## Required projection

- Keep `openapi: 3.1.2`, Draft 2020-12-compatible schemas, fourteen exact portable operations, globally unique operation
  IDs, canonical paths, and resolving same-document references.
- Every operation documents the optional portable `X-Request-ID` input. Every included response documents the shared
  request ID and security headers, plus `Vary` where runtime selection uses `Accept`.
- Public operations use explicit `security: []`; all four profile operations use the single Firebase bearer scheme.
- GCP request bodies advertise JSON and CBOR. Success bodies advertise JSON and CBOR. Problems advertise
  `application/problem+json` and the same schema as ordinary `application/cbor`; never `application/problem+cbor`.
- Include every controlled status reachable while processing the supported method, but do not project path-level 405
  as a response of that method. Empty 204 responses have no content.
- Application objects are closed and express exact requiredness, omission versus null, bounds, patterns, literals,
  formats, and collection shapes. Examples, when present, must validate and contain no credential, personal, or live
  provider data.
- `/openapi.json` may omit itself. If included, use the exact discovery tuple and JSON-only success.

## Workflow

1. Trace route selection, negotiation, parsing, validation, authentication, service errors, headers, and response model.
2. Update native handler annotations, the focused normalization definitions, and semantic tests in `cmd/openapi/`.
3. Run `just fmt-openapi`, `just docs`, review both artifacts for only intended semantic changes, then run
   `just openapi-check`.
4. Test runtime discovery for exact embedded bytes, JSON-only negotiation, closed query, 405 behavior, body non-consumption,
   and independence from authentication, persistence, GitHub, DNS, and filesystem state.
5. Run the narrow affected tests and `just check`. Regenerate once more and confirm `just openapi-check` remains clean.

Use `api-docs/embed_test.go` for embedding and JSON/YAML equivalence. Keep detailed operation/schema semantics and
generated drift in `cmd/openapi/main_test.go`; keep runtime discovery behavior in `cmd/server` or `internal/http/docs`
tests. A version-prefix assertion or successful generation alone is not sufficient.
