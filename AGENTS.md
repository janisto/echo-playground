# AGENTS.md

Instructions for coding agents working in this repository.

`README.md` is for human users and contributors: setup, capabilities, architecture, operations, and contribution entry points. `AGENTS.md` is for coding agents: execution rules, implementation constraints, and validation policy. Do not duplicate agent instructions into the README or turn this file into human onboarding documentation.

## Engineering priorities

- Correctness first, then readability and maintainability, then performance.
- Inspect the relevant implementation, callers, and existing tests before changing behavior.
- Prefer the smallest safe change that solves the problem.
- Reuse existing local patterns and utilities, refactoring them when needed, instead of creating parallel abstractions or adding dependencies.
- State the failure mode before architectural, security, persistence, or production-impacting changes.
- Do not declare completion until implementation, validation, and remaining risks are reported.
- Keep source comments and documentation concise. Do not add progress narration, generated banners, emojis, or speculative TODOs.

## Pull requests

- Format titles as `type[optional scope]: description`. Prefer no scope; include one only when it materially improves clarity.
- Use `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`, or `revert` as the type. Example: `feat: add response size field`.
- Keep each pull request focused. In the body, explain why the change is needed, what changed, how it was validated, and any remaining risk.
- Keep the title suitable for the final squash or merge commit.
- This repository does not maintain a `CHANGELOG.md`; do not create one or require changelog entries in pull requests.

## Commits

- Follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).
- Prefer no scope; include one only when it materially improves clarity. Write a short, imperative description. Example: `fix: preserve request ID`.
- Mark breaking changes with `!` and explain them in a `BREAKING CHANGE:` footer.
- Before committing, run `just qa` and `git diff --check`.

## GitHub automation

- Reference GitHub Actions by explicit full release tags such as
  `owner/action@v1.2.3`, not full commit SHAs or floating major-version tags.
  Dependabot updates those release tags.

## Mandatory skills

- Use `.agents/skills/adversarial-testing/SKILL.md` for every task that plans, creates, modifies, reviews, debugs, or evaluates tests. Apply it alongside any more specific framework or infrastructure testing skill.
- Use `.agents/skills/readme-maintenance/SKILL.md` for every README audit or change. Also use it to assess README impact whenever public behavior, configuration, setup, commands, architecture, deployment, CI, or supported versions change. A README edit is required only when the audit finds a stale or missing reader-facing claim.

## Project Overview

Echo Playground is a minimal REST API skeleton built with [Echo 5.3](https://github.com/labstack/echo/tree/v5). It demonstrates structured logging, RFC 9457 Problem Details for errors, and a modular route layout.

### Key Features

- Echo 5.3 middleware stack with security headers, CORS, request correlation, structured access logging, and panic recovery
- Request-scoped Zap logger through echo-observability with Google Cloud trace metadata enrichment
- Plain response bodies with RFC 9457 Problem Details for errors
- Content negotiation supporting JSON and CBOR formats
- Cursor-based pagination with RFC 8288 Link headers
- Firebase Authentication with JWT validation via Echo middleware
- Firestore integration with transaction-safe CRUD operations and audit logging
- go-playground/validator for request validation
- Swag v2 OpenAPI registration with deterministic 3.1.2 normalization and semantic drift tests

### Tech & Tooling

- Language/runtime: Go 1.26.7+
- Frameworks/libs: Echo v5.3+, go-playground/validator, fxamacker/cbor, Firebase Admin SDK
- Logging: Zap via github.com/janisto/echo-observability/v2
- Testing: Go standard `testing` package, echotest, Firebase Emulators
- OpenAPI: Swag v2 plus the deterministic `cmd/openapi` normalizer (OAS 3.1.2)
- Task runner: [Just](https://github.com/casey/just) (required for pinned Go toolchain)
- Firebase CLI: Required for emulators (`just emulators`)

---

## Justfile

The project includes a `Justfile` for common development tasks. Run `just` to list available commands.

Key recipes:
- `just build` - Build both Go modules
- `just run` - Run the server
- `just test` - Test both Go modules
- `just coverage` - Run both modules and generate separate coverage reports
- `just lint` - Lint both Go modules
- `just fmt` - Format both Go modules
- `just fix` - Run lint fixes for both Go modules
- `just check` - Whole-repository OpenAPI drift, format, lint, build, and test check (`check-all` is a compatibility alias)
- `just functions-check` - Build, test, and lint the separate function module
- `just test-race` - Race-test both modules
- `just mutation` - Mutation-test both modules (`mutation-app` and `mutation-functions` are narrow variants)
- `just fuzz` - Fuzz one named root-module target in its package for a bounded duration
- `just fuzz-functions` - Fuzz the separate function module
- `just fuzz-all` - Run every curated fuzz target for the requested per-target duration
- `just qa` - Quality assurance (tidy + fix + build + test)
- `just vuln` - Check both modules for vulnerabilities
- `just functions-vuln` - Check the function module for vulnerabilities
- `just update` - Update root dependencies, root Go tools, and the function module
- `just functions-update` - Update only the function module
- `just install` - Download module dependencies (alias for download)
- `just fresh` - Clean local artifacts, download dependencies, and build both modules
- `just emulators` - Start Firebase emulators (Auth + Firestore)
- `just test-integration-ci` - Require emulators and generate separate integration coverage
- `just functions-smoke` / `just container-smoke` - Probe the registered function and final image
- `just contract-smoke` - Probe an already-running local server; anonymous live GitHub calls are opt-in
- `just fmt-check` / `just tidy-check` / `just modernize-check` / `just workflow-check` - Non-mutating quality gates
- `just workflow-security-check` - Audit GitHub Actions with the repository's zizmor policy
- `just docs` - Generate OpenAPI 3.1.2 JSON and YAML (alias for gen-openapi)
- `just gen-openapi` - Generate OpenAPI 3.1.2 JSON and YAML
- `just fmt-openapi` - Format native Swag handler annotations
- `just openapi-check` - Non-mutating generated-contract and runtime-discovery drift gate

The Justfile uses `set dotenv-load` so all recipes automatically load `.env`. The `.env` sets `GOTOOLCHAIN` to pin the Go version, preventing automatic upgrades from a newer local Go installation. Always prefer `just` recipes over raw `go` or `golangci-lint` commands.

`FIREBASE_MODE` is `offline`, `emulator`, or `live`. Offline and emulator modes are development-only. Emulator mode
requires both strict `host:port` variables and a `demo-*` project; live mode requires a non-demo project and rejects
emulator hosts. Tests use hardcoded emulator addresses via `internal/testutil` and auto-skip when unreachable. CI sets
`REQUIRE_FIREBASE_EMULATORS=1`, which makes missing emulators a hard failure.

---

## Setup

### Requirements

- Go 1.26.7+
- Firebase CLI (for emulators): `npm install -g firebase-tools`

### Install Dependencies

```bash
go mod download
```

### Build

```bash
go build -v ./...
```

### Run

```bash
go run ./cmd/server
```

The server starts on port 8080 with endpoints:
- `http://localhost:8080/health` - health probe
- `http://localhost:8080/api-docs` - Swagger UI
- `http://localhost:8080/openapi.json` - OpenAPI 3.1.2 spec

---

## Testing

Run all tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run tests with coverage:

```bash
go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
```

Generate coverage report:

```bash
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

---

## Linting

The project uses [golangci-lint](https://golangci-lint.run/) v2 for static analysis and code formatting. Configuration is defined in `.golangci.yml`.

Run linter:

```bash
golangci-lint run ./...
```

Apply formatters (gci, gofumpt, golines) automatically:

```bash
golangci-lint fmt ./...
```

Run linter and apply formatters in one step:

```bash
golangci-lint run --fix ./...
```

---

## Project Structure

```
.agents/skills/        # Reusable project-specific agent skills
.github/agents/       # GitHub Copilot custom-agent profiles
cmd/server/            # Application entrypoint and HTTP server bootstrap
cmd/openapi/           # Deterministic OpenAPI 3.1.2 normalization and semantic checks
cmd/profile-migrate/   # Audit-first one-time profile persistence cutover
api-docs/              # Embedded generated OpenAPI 3.1.2 JSON and YAML
functions/             # Independent Functions Framework Go module
internal/http/         # HTTP transport layer
  docs/                # Swagger UI serving and spec route registration
  health/              # Health check handler (unversioned)
  v1/                  # Versioned API (v1)
    hello/             # Hello endpoint handlers
    items/             # Items endpoint handlers
    github/            # Anonymous fixed-origin GitHub projection handlers
    profile/           # Profile endpoint handlers (requires auth)
    routes/            # Route registration
internal/platform/     # Cross-cutting infrastructure
  audit/                # Request-scoped structured audit events
  auth/                # Firebase Auth middleware and JWT validation
  firebase/            # Firebase Admin SDK initialization
  middleware/          # Security headers, CORS, vary
  pagination/          # Cursor-based pagination
  respond/             # Panic recovery, Problem Details, content negotiation
  timeutil/            # Time formatting utilities
  validate/            # go-playground/validator integration
internal/service/      # GitHub transport/projection and Firestore profile persistence
internal/service/      # Business logic and data access
  profile/             # Profile service with Firestore backend
internal/testutil/     # Test utilities (shared Echo fixture, emulator helpers)
```

Repository-wide instructions live in this file. Task-specific, portable skills live at
`.agents/skills/<skill-name>/SKILL.md`. GitHub Copilot custom-agent profiles remain in `.github/agents/` because that is
the product's supported discovery location.

The repository follows these upstream conventions:

- [AGENTS.md](https://github.com/agentsmd/agents.md) is the canonical open format for repository agent instructions,
  stewarded by the Agentic AI Foundation under the Linux Foundation.
- [Agent Skills](https://github.com/agentskills/agentskills) is the canonical specification and documentation source for
  the required `SKILL.md` filename and `name` and `description` frontmatter. The rendered
  [format specification](https://agentskills.io/specification) provides the detailed schema and validation rules, while
  [GitHub's Agent Skills documentation](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills) lists
  `.agents/skills/` as a supported project discovery location.

Use `readme-maintenance` for README audits and `openapi-contract` for generated API contract work. Do not duplicate those
workflows in custom-agent profiles. Each skill also keeps Codex UI metadata in `agents/openai.yaml`; regenerate it with
the `skill-creator` tooling whenever a skill name, description, or default prompt changes.

Current task-specific guidance:

| Name | Location | Purpose |
|---|---|---|
| `adversarial-testing` | `.agents/skills/adversarial-testing/` | Enforce risk-driven, mutation-resistant testing across every test layer |
| `echo-endpoint` | `.agents/skills/echo-endpoint/` | Implement or change Echo 5.3 endpoints |
| `go-testing` | `.agents/skills/go-testing/` | Write and review tests in either Go module |
| `pagination-endpoint` | `.agents/skills/pagination-endpoint/` | Implement cursor-paginated list endpoints |
| `readme-maintenance` | `.agents/skills/readme-maintenance/` | Reconcile README claims with the repository |
| `openapi-contract` | `.agents/skills/openapi-contract/` | Maintain generated OpenAPI and Swagger UI contracts |
| `security-review` | `.github/agents/security-review.agent.md` | Run an evidence-based GitHub Copilot security audit with a prompt-level read-only boundary |

Repository automation under `.github/` independently checks both Go modules, required Firebase emulators,
vulnerabilities, generated OpenAPI drift, the final container, and root and function lint. Use exact release tags for
every GitHub Action, for example `uses: actions/checkout@v7.0.1` and `uses: actions/setup-go@v7.0.0`, so the GitHub
Actions updater in `.github/dependabot.yml` proposes direct, readable version updates in the same form. This repository
accepts mutable upstream release tags in exchange for that consistent convention; do not replace them with commit SHAs.
Keep `ci` and `lint` as the stable ruleset-required aggregate job names. Each aggregate must use
`if: ${{ always() }}` and fail unless every job it needs completed successfully, so a failed or cancelled dependency
cannot make the required check disappear or pass. Dependabot covers both Go modules, GitHub Actions, and Docker; labeler configuration treats
`.agents/**/*.md` and `.github/**/*.md` as documentation.

Run `just workflow-security-check` when changing GitHub Actions. Keep `.github/zizmor.yml` aligned with the repository's
exact-tag policy and one-day Dependabot cooldown. Suppress a finding only at the narrowest affected workflow step and
document why the flagged behavior is intentional.

Every Go setup step intentionally uses this baseline configuration:

```yaml
- name: Set up Go
  uses: actions/setup-go@v7.0.0
  with:
    go-version: '1.26.x'
    check-latest: true
    cache: true
```

`1.26.x` tracks the newest patch release without crossing the Go 1.26 runtime boundary. `check-latest: true` prevents
a matching but stale runner-cached patch from taking precedence over the latest available Go 1.26 patch, which matters
when the modules raise their minimum patch version. `cache: true` retains setup-go's module and build caches for faster
repeat CI runs. Jobs may add `cache-dependency-path` for module-specific cache invalidation, but must retain this baseline.

---

## Architecture Principles

### Platform Layer

The `internal/platform/` packages provide shared infrastructure used by HTTP handlers. These packages are organized by concern rather than transport:

| Package | Purpose | Dependencies |
|---------|---------|--------------|
| `audit` | Request-scoped structured security and compliance events | echo-observability, Zap |
| `auth` | Firebase JWT validation, user context, Echo security middleware | Firebase Admin SDK, Echo |
| `firebase` | Firebase Admin SDK initialization (Auth + Firestore clients) | Firebase Admin SDK |
| `middleware` | HTTP middleware (CORS, security headers, vary) | Echo |
| `pagination` | Cursor encoding/decoding, link header generation | Standard library only |
| `request` | Strict single-object JSON request decoding | Echo, standard library |
| `respond` | Panic recovery, Problem Details error responses, content negotiation | Echo, fxamacker/cbor |
| `timeutil` | Time formatting constants | Standard library only |
| `validate` | Request validation via go-playground/validator | go-playground/validator, Echo |

**Truly transport-agnostic packages:**
- `pagination` - Cursor logic works for any transport
- `timeutil` - Time formatting has no transport coupling

**HTTP-coupled packages (by design):**
- `audit` - Logs through the request context installed by echo-observability
- `middleware` - HTTP-specific (CORS, headers, vary)
- `respond` - HTTP error handling with RFC 9457 Problem Details

**Key rule:** Platform packages must not import from `internal/http/` (no circular dependencies). HTTP handlers import platform packages, never the reverse.

---

## Coding Conventions

### Handler Signature

All handlers use Echo 5.3's pointer-to-concrete-struct signature:

```go
func handler(c *echo.Context) error {
    // ...
}
```

### Response Format

Responses use `respond.Negotiate()` for content negotiation (JSON/CBOR):

```go
func getHandler(c *echo.Context) error {
    return respond.Negotiate(c, http.StatusOK, Data{Message: "Hello, World!"})
}
```

### Input Binding and Validation

Use a source-specific decoder plus `c.Validate()`. Use `request.Decode` for exactly one top-level JSON or CBOR object,
`request.RejectUnknownOrRepeatedQuery` before `echo.BindQueryParams` for closed scalar query contracts, and the corresponding path binder for path DTOs. Avoid
generic `c.Bind`, which can merge multiple sources.

```go
func createHandler(c *echo.Context) error {
    var input CreateInput
    if err := request.Decode(c, &input); err != nil {
        return err
    }
    if err := c.Validate(&input); err != nil {
        return err
    }
    // process input...
}
```

### Input Struct Tags

- Body fields: `json:"name" validate:"required,min=1,max=100"`
- Query params: `query:"limit" validate:"omitempty,min=1,max=100"`
- Path params: `param:"id" validate:"required"`

### Error Handling

Errors follow RFC 9457 Problem Details and honor content negotiation:
- `application/problem+json` when JSON is requested (default, RFC 9457 registered)
- `application/cbor` with the same Problem Details members when CBOR is requested

Use custom error helpers:

```go
import "github.com/janisto/echo-playground/internal/platform/respond"

respond.InvalidRequest()
respond.Unauthorized()
respond.Forbidden()
respond.NotFound()
respond.ProfileExists()
respond.ValidationFailed(fieldErrors...)
respond.InternalError()
```

Panic recovery and Echo-level handlers use Problem Details via `internal/platform/respond`.

### Logging

Use the request-scoped logger installed by echo-observability:

```go
import (
	"github.com/janisto/echo-observability/v2"
	"go.uber.org/zap"
)

obs.Logger(ctx).Info("message", zap.String("key", "value"))
obs.Logger(ctx).Warn("message", zap.String("key", "value"))
obs.Logger(ctx).Error("message", zap.Error(err), zap.String("key", "value"))
```

`obs.Logger(ctx)` includes request IDs and valid W3C trace metadata. It intentionally returns a no-op logger outside
an observability request context. Use the explicit process logger returned by `obs.NewLogger` for startup, shutdown,
background jobs, `net/http` server errors, and other non-request paths. Use `internal/platform/audit.LogEvent` for audit events.

Install `obs.RequestContext` first, recovery middleware second, and `obs.AccessLogger` third. Echo makes the first
listed middleware outermost, so the access logger classifies and rethrows panics before recovery writes the response.

### Adding New Routes

1. Create a new directory under `internal/http/v1/` (e.g., `users/`)
2. Create `handler.go` with `Register(g *echo.Group)` function
3. Create `input.go` with input structs using `validate` tags
4. Create `model.go` with response model structs
5. Add native Swag annotations and the route's semantic normalization and tests under `cmd/openapi/`
6. Call `Register()` from `routes.Register()`
7. Log within handlers using context-aware helpers
8. Return errors using respond error helpers
9. Regenerate the OpenAPI spec: `just docs`

### POST 201 Created Pattern

```go
func createHandler(c *echo.Context) error {
    var input CreateInput
    if err := request.Decode(c, &input); err != nil {
        return err
    }
    if err := c.Validate(&input); err != nil {
        return err
    }

    resource := createResource(input)
    c.Response().Header().Set("Location", fmt.Sprintf("/resources/%s", resource.ID))
    return respond.Negotiate(c, http.StatusCreated, resource)
}
```

### JSON Encoding

- JSON responses are UTF-8
- CBOR responses use `application/cbor` content type

---

## REST API Implementation Guidelines

### URI Design

- Use plural nouns for collections (`/users`, not `/user`)
- Avoid verbs in URIs; let HTTP methods convey the action
- Nest resources to express relationships (`/posts/{postId}/comments`); limit nesting to one level
- Use lowercase with hyphens for multi-word segments (`/user-profiles`)

### Input Validation

- Validate all input; never sanitize (reject invalid input, don't transform it)
- Use go-playground/validator tags (`required`, `min`, `max`, `oneof`, `email`, `e164`)
- Return 400 for malformed syntax; 422 for validation failures on valid syntax

### HTTP Methods

| Method | Purpose | Success Status |
|--------|---------|----------------|
| GET | Retrieve resource(s) | 200 OK |
| POST | Create a resource | 201 Created |
| PUT | Replace a resource entirely | 200 OK or 204 No Content |
| PATCH | Partial update | 200 OK or 204 No Content |
| DELETE | Remove a resource | 204 No Content |

### Status Codes

| Status | Use Case |
|--------|----------|
| 200 OK | Successful GET, PUT, PATCH |
| 201 Created | Successful POST (include Location header) |
| 204 No Content | Successful DELETE |
| 400 Bad Request | Malformed syntax, missing required fields |
| 401 Unauthorized | Missing or invalid authentication |
| 403 Forbidden | Authenticated but not authorized |
| 404 Not Found | Resource does not exist |
| 405 Method Not Allowed | HTTP method not supported for resource |
| 406 Not Acceptable | No supported success representation matches `Accept` |
| 413 Content Too Large | Request body exceeds the global 1 MiB limit |
| 415 Unsupported Media Type | Body is not `application/json` |
| 422 Unprocessable Entity | Validation failures on specific fields |
| 499 Client Closed Request | Observational status for canceled requests; do not write an error body |
| 500 Internal Server Error | Unexpected server error |
| 503 Service Unavailable | Authentication dependency or request deadline failure |

### Error Responses

All errors use RFC 9457 Problem Details format:

```json
{
  "type": "about:blank",
  "title": "Not Found",
  "status": 404,
  "detail": "Resource not found",
  "code": "not_found"
}
```

### Request ID

- `X-Request-ID` header tracks requests end-to-end
- Use it in request-scoped logs and explicitly allowlisted internal calls; never forward it to GitHub
- Validated or generated automatically by echo-observability `RequestContext`

### Content Types

**Requests:**
- The three body-bearing portable operations accept `application/json` (optionally `charset=utf-8`) or parameterless `application/cbor`
- All other portable operations advertise no request body and their handlers do not consume one

**Responses:**
- Default: `application/json` (RFC 8259)
- Alternate: `application/cbor` (RFC 8949)
- Errors: `application/problem+json` (RFC 9457) or ordinary `application/cbor` with the same members
- Format selected via `Accept` header
- Apply the most-specific matching media range before comparing effective quality values; a specific `q=0` overrides a broader range when another supported representation remains
- Keep success and Problem Details negotiation separate. Problem media types do not opt a successful response into their base format.
- Return 406 when no success representation is acceptable. If neither Problem Details format is acceptable, use JSON only as the final 406 diagnostic fallback.
- Error format is controlled by `Accept` header, not request `Content-Type`

Echo 5.3 is the minimum framework baseline. Configure `AutoHandleHEAD` and reject duplicate route registration through
`RouterConfig`; retain `NoGroupAutoRegister404Routes` so authenticated group middleware cannot intercept unmatched
public routes. HEAD runs the corresponding GET handler but Echo suppresses the response body.

### Timestamps

- Use ISO 8601 / RFC 3339 format with UTC timezone and millisecond precision: `2024-01-15T10:30:00.000Z`
- Use `timeutil.Time` wrapper for JSON responses to ensure consistent `.000Z` output
- Use `timeutil.RFC3339Millis` constant for formatting: `time.Now().UTC().Format(timeutil.RFC3339Millis)`
- Go uses a reference time for format strings: `2006-01-02T15:04:05.000Z` (Jan 2, 2006 15:04:05)
- Store and transmit in UTC; convert for display only

### Pagination

Use cursor-based pagination via `internal/platform/pagination`. Bound cursor input length. Every non-empty cursor must
carry the exact endpoint type; malformed, empty-type, cross-endpoint, and stale cursors return 400 Bad Request.

Links provided via HTTP `Link` header per RFC 8288.

---

## Testing Guidelines

### Mutation and fuzzing

- Run `just mutation` when changing production logic or its tests. Investigate meaningful `LIVED` mutants and add a
  behavioral test only when the mutant exposes a real contract gap; equivalent transformations do not justify brittle assertions.
- Keep logical and bitwise inversion enabled in `.gremlins.yaml`. They target the compound authorization,
  configuration, request, and negotiation guards plus the CBOR masks, encoded lengths, and body-size shifts that the
  default Gremlins operators miss.
- Gremlins tests only covered mutations. When changing Firebase Auth or Firestore behavior, start `just emulators`
  before `just mutation-app`; otherwise emulator-only paths are `NOT COVERED` and provide no mutation evidence.
- Leave Gremlins `workers` unset in `.gremlins.yaml` and do not hard-code `--workers` in the Justfile. Its default
  scales to the machine's logical CPUs. Use a one-off lower value only for a constrained machine or if concurrent
  mutation runs contend for shared Firebase emulator state.
- Keep mutation testing separate from `just fuzz`: Gremlins mutates covered production code, while Go fuzzing mutates
  inputs to one target in one package.
- Use fuzz targets for parsers, decoders, negotiators, and other bounded input surfaces with precise invariants. A
  no-panic assertion alone is appropriate only when malformed input is explicitly allowed to produce any non-panicking result.
- During development, run the narrow root target with `just fuzz <target> <duration> <package>` or the function target
  with `just fuzz-functions`. Use `just fuzz-all <duration>` when changes affect multiple strict-JSON, pagination,
  content-negotiation, CBOR time, or function-input boundaries.
- Leave Go fuzz `-parallel` unset in the Justfile. Its default follows `GOMAXPROCS`; use a one-off lower value only when
  the available CPU or memory makes the default impractical.
- Pass the target, duration, and package to `just fuzz` when using a non-default target. Preserve minimized regression
  inputs under the package's `testdata/fuzz/<target>` directory when they represent required behavior.

### Test Structure

- Tests are colocated with source files using `_test.go` suffix
- Use Go's standard `testing` package
- Use `echotest` package for handler unit tests
- Use `e.ServeHTTP()` for integration tests

### Integration Test Pattern

Use `testutil.NewTestEcho()` to create a pre-configured Echo instance with validator and error handler:

```go
func TestMyFeature(t *testing.T) {
    e := testutil.NewTestEcho()
    e.Use(middlewares...)
    v1 := e.Group("/v1")
    routes.Register(v1, verifier, svc)

    req := httptest.NewRequest(http.MethodGet, "/v1/hello", nil)
    req.Header.Set("X-Request-ID", "test-trace-id")
    rec := httptest.NewRecorder()

    e.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200 OK, got %d", rec.Code)
    }
}
```

### Handler Unit Test Pattern (echotest)

```go
rec := echotest.ContextConfig{
    Request:     httptest.NewRequest(http.MethodGet, "/items?limit=10", nil),
    QueryValues: url.Values{"limit": {"10"}},
}.ServeWithHandler(t, handler, respond.NewHTTPErrorHandler())
```

### Error Response Testing

```go
var problem respond.ProblemDetails
if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
    t.Fatalf("failed to unmarshal problem: %v", err)
}
if problem.Status != http.StatusNotFound {
    t.Fatalf("expected 404, got %d", problem.Status)
}
```

### Coverage Requirements

- Tests should cover success paths, error paths, and edge cases
- Verify error responses use Problem Details format
- Test trace ID propagation through the request context

### Firebase Emulator Testing

Firestore integration tests require running emulators. Start them before running tests:

```bash
just emulators
```

Emulator configuration (from `firebase.json`):
| Service | Port |
|---------|------|
| Auth | 7110 |
| Firestore | 7130 |
| Emulator UI | 4000 |

Tests auto-skip when emulators are unreachable (TCP dial check). The `internal/testutil` package provides hardcoded emulator addresses and `t.Setenv()` helpers, so no `.env` changes are needed for testing. In development with `demo-test-project` and no emulator hosts, the public application starts but protected routes return 503. Firebase clients are initialized only when both Auth and Firestore emulator hosts are configured together.
Configuration must reject emulator hosts outside development. Emulator HTTP helpers must validate response status before decoding or assuming cleanup succeeded.

`firestore.rules` deliberately denies all client SDK access. The server uses the Firebase Admin SDK, which bypasses
Security Rules and is authorized through IAM; do not loosen the rules merely to make server-side emulator tests pass.

To run emulator tests, start emulators:

```bash
just emulators
```

---

## Profile persistence migration

`cmd/profile-migrate` is audit-only by default. Do not execute it against any project unless the current request
explicitly authorizes that project and mode. Applying requires an exact `--confirm-project`, a closed version-1 manifest
with approved per-document terms evidence, `--confirm-rollback-reference` for the verified provider backup, and
`--confirm-profile-writes-quiesced` for the write freeze. Never invent terms acceptance, log document IDs or profile
data, or treat a partially applied collection as globally atomic. See README.md for the operator sequence.

---

## Go Function Deployment

The `functions/` module is intentionally a minimal, independent Google Functions Framework example.
Keep its handler at the module root beside `functions/go.mod`. Do not import Echo or the root application
architecture into it.
Its POST handler requires `Content-Type: application/json`, one JSON object, and known fields only.

Firebase CLI cannot deploy Go source functions. `firebase deploy` is not a valid deployment path for this
module; Firebase CLI is used only for Auth and Firestore emulators. Deploy the function with Google Cloud CLI:

```bash
gcloud run deploy echo-playground-hello \
  --source functions \
  --function Hello \
  --base-image go126 \
  --region REGION
```

Validate the module independently with `just functions-check`, `just functions-test-race`, and
`just functions-vuln`. Run the registered target locally with `just functions-run 8081`.

---

## Testing & Testability

- **NEVER add test-related code to production code.** No `if testing` branches, no test flags, no mock injection points.
- If code is not unit testable, refactor it. Use dependency injection, extract interfaces, or restructure. Do not pollute production code with test scaffolding.
- Tests belong in `*_test.go` files; production code must remain test-agnostic.
- Reusable application-boundary fakes live under `internal/testutil/fake`; production packages do not export mocks.

---

## Secrets & Environment Variables

- Never commit secrets. Use environment variables for configuration.
- Access config through environment variables; don't hardcode secrets in business logic.
- Don't log secrets or PII; ensure logs redact sensitive fields.
- Typical env vars:
  - `FIREBASE_PROJECT_ID` (`demo-test-project` is the local development default)
  - `FIREBASE_MODE` (`offline`, `emulator`, or `live`)
  - `FIRESTORE_EMULATOR_HOST` (only needed when running the server against emulators; tests use hardcoded addresses)
  - `FIREBASE_AUTH_EMULATOR_HOST` (only needed when running the server against emulators; tests use hardcoded addresses)
  - `GOOGLE_APPLICATION_CREDENTIALS` (path to service account JSON; uses ADC if not set)
  - `GOOGLE_CLOUD_PROJECT`, `GCP_PROJECT`, `GCLOUD_PROJECT`, or `PROJECT_ID` (for Cloud Trace correlation)

---

## Authentication

### Protected Endpoints

Use Echo group-level middleware for protected routes:

```go
import "github.com/janisto/echo-playground/internal/platform/auth"

func Register(v1 *echo.Group, verifier auth.Verifier, svc profilesvc.Service) {
    protected := v1.Group("", auth.Middleware(verifier))
    profile.Register(protected, svc)
}
```

### Accessing User in Handlers

The auth middleware sets the user in Echo context for secured endpoints:

```go
func handleGetProfile(c *echo.Context) error {
    user, err := auth.UserFromEchoContext(c)
    if err != nil {
        return respond.Unauthorized()
    }
    // user is guaranteed non-nil for secured endpoints because the auth
    // middleware rejects unauthenticated requests before reaching the handler
    return respond.Negotiate(c, http.StatusOK, Profile{ID: user.UID})
}
```

---

## OpenAPI Documentation

The repository uses native Swag v2 handler annotations for operation registration and `cmd/openapi` for deterministic
OpenAPI 3.1.2 normalization. The normalizer validates the native method, path, operation ID, response-status,
parameter, request-body, and security projection before supplying exact Draft 2020-12 schemas, media, and shared
headers that Swag cannot express. Do not hand-edit generated artifacts or add another contract description.

### Generated files

| File | Purpose |
|------|---------|
| `api-docs/swagger.json` | OpenAPI 3.1.2 JSON embedded and served at `/openapi.json` |
| `api-docs/swagger.yaml` | Semantically equivalent YAML projection |

### Generating the spec

```bash
just docs
```

Use `just openapi-check` for the non-mutating native-generation, normalization, and runtime-discovery drift gate.

### When to regenerate

Regenerate after any of these changes:
- adding, removing, or renaming a portable route or operation ID;
- changing parameters, body media, success or error media, statuses, security, headers, or public models;
- changing schema requiredness, nullability, bounds, literals, patterns, collection shapes, or examples; or
- changing generated document metadata.

### Generator conventions

- Keep exact native operation registration in handler annotations. Keep Draft 2020-12 schemas, shared parameters,
  response media, security, and headers in the focused normalizer files under `cmd/openapi/`.
- The normalizer must fail when native routes, operation IDs, status sets, parameters, request bodies, or security drift;
  it must not silently replace an unrelated generated surface.
- GCP request bodies use JSON and CBOR; successes use JSON and CBOR; failures use Problem Details JSON and ordinary CBOR.
- Every required response documents the portable request ID and security headers; add conditional `Link`, `Location`,
  `WWW-Authenticate`, and quota headers where the runtime emits them.
- Keep every reachable reference local and resolving, every application object closed, and all profile operations
  protected by the one Firebase bearer scheme. Public operations use explicit `security: []`.
- Detailed semantic tests belong in `cmd/openapi/main_test.go`. Runtime discovery tests must compare the served embedded
  bytes and prove that auth, persistence, and GitHub dependencies are not invoked.
- Commit both generated files and run `just docs` followed by `just openapi-check`.

### Swagger UI

Swagger UI and its initialization script are embedded in `internal/http/docs/`. Generated OpenAPI JSON is embedded
by `api-docs/embed.go` and registered by application composition with `docs.Register(e, apidocs.OpenAPIJSON)`.

| URL | Purpose |
|-----|---------|
| `/api-docs` | Swagger UI |
| `/openapi.json` | Raw OpenAPI 3.1.2 spec |

---

## Repository restrictions

- Do not modify `go.mod` or `go.sum` unless a dependency change is explicitly in scope.
- Use `internal/platform/respond` helpers for application error responses.
- Verify repository paths, APIs, and commands from the checkout or current tooling before relying on them.
