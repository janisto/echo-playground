---
name: openapi-contract
description: Maintain and verify echo-playground OpenAPI 3.1 annotations, deterministic generation, Problem Details media types, bearer security, embedded artifacts, and Swagger UI when routes, DTOs, errors, or API documentation change.
---

# OpenAPI contract maintenance

Read `AGENTS.md` and the affected handler before editing the OpenAPI contract or Swagger UI integration.

## Architecture

The public contract has four deliberately separate parts:

1. Handler annotations and DTO tags provide swag input.
2. `just docs` generates OpenAPI 3.1 JSON and YAML and runs `cmd/openapi` for deterministic corrections swag cannot
   currently express correctly.
3. `api-docs/embed.go` embeds `swagger.json`; `api-docs/embed_test.go` validates semantics and JSON/YAML equivalence.
4. `internal/http/docs/` serves the embedded document and the SRI-pinned Swagger UI.

Generated `api-docs/swagger.json` and `api-docs/swagger.yaml` are committed. Do not restore `docs.go`, import a generated
registry, or add a runtime filesystem dependency for the specification.

## Annotation rules

- General API metadata and `BearerAuth` definition live in `cmd/server/main.go`.
- Operation annotations live directly above handler functions.
- Use paths relative to the OpenAPI `/v1` server.
- Add `@Produce json,application/cbor` when an operation returns a negotiated success body or negotiated errors.
- Do not add `@Accept json`: the tracked swag v2 body-parameter defect produces a broken empty schema. JSON remains the
  documented request body through the body parameter.
- Use `respond.ProblemDetails` for every documented error.
- Include every status reachable from binding, body limits, strict JSON decoding, validation, authentication, service
  mapping, and request deadlines.
- Include 406 for operations whose success or error representation is negotiated through `Accept`.
- Add `@Security BearerAuth` to every protected operation and nowhere else.
- Document `Location` and `Link` headers when the runtime emits them.
- Keep examples consistent with JSON field names and the repository's UTC millisecond timestamp contract.

Error responses must expose `application/problem+json` and `application/problem+cbor`. If swag output requires a
systematic correction, update `cmd/openapi` and its tests rather than hand-editing generated files.

## Workflow

1. Inspect the route registration, handler behavior, input and output models, and relevant error mapping.
2. Update annotations or DTO tags.
3. Run `just fmt-openapi` after annotation formatting changes.
4. Run `just docs`.
5. Review both generated artifacts. Reject unrelated churn, missing statuses, incorrect schemas, lost security, and media
   types that disagree with runtime negotiation.
6. Update the exact whole-contract matrix in `api-docs/embed_test.go`; it rejects missing or extra operations, statuses,
   IDs, security, request or response media types, and required headers.
7. If postprocessing changes, add focused coverage in `cmd/openapi/main_test.go`.
8. If serving or UI behavior changes, test `internal/http/docs/`, CSP behavior, embedded assets, and exact routes.
9. Run `just build`, `just test`, and `just lint`; these unqualified commands cover both modules.
10. Run `just docs` again and confirm regeneration is stable.

Never conceal a generator limitation with an unexplained output patch. Keep corrections small, deterministic, tested,
and documented in the source that applies them.

## Review checklist

- OpenAPI remains `3.1.0`.
- JSON and YAML describe the same document.
- Every operation ID is non-empty and unique, and the exact path/method set matches registration.
- Protected operations use HTTP bearer authentication.
- Request bodies are JSON-only and reject undocumented fields at runtime.
- Successful responses advertise JSON and CBOR where implemented.
- All documented error statuses use both Problem Details media types.
- 201 responses document `Location`; paginated 200 responses document `Link`.
- The generated JSON remains embedded and available at `/api-docs/openapi.json`.
- Swagger UI remains at `/api-docs`, uses exact SRI-pinned assets, and receives only its narrow CSP exception.
- The second generation produces no new diff.
