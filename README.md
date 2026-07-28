# Echo Playground

[![Application CI](https://img.shields.io/github/actions/workflow/status/janisto/echo-playground/app-ci.yml?branch=main&label=application%20CI&logo=githubactions&logoColor=white)](https://github.com/janisto/echo-playground/actions/workflows/app-ci.yml)
[![Code quality](https://img.shields.io/github/actions/workflow/status/janisto/echo-playground/app-lint.yml?branch=main&label=code%20quality&logo=go&logoColor=white)](https://github.com/janisto/echo-playground/actions/workflows/app-lint.yml)
[![Go 1.26.5](https://img.shields.io/badge/Go-1.26.5-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MIT license](https://img.shields.io/github/license/janisto/echo-playground)](LICENSE)

A compact, high-quality REST API example built with [Echo 5.3](https://github.com/labstack/echo/tree/v5) and Go 1.26.
It demonstrates HTTP contracts, structured observability, Firebase Authentication, Firestore CRUD, OpenAPI 3.1,
and production-shaped verification without pretending to be a complete production platform.

<img src="assets/gopher.svg" alt="Go Gopher mascot illustration" width="400">

<sub>Gopher illustration from [free-gophers-pack](https://github.com/MariaLetta/free-gophers-pack) by Maria Letta</sub>

## Features

- Echo 5.3 with strict single-object JSON decoding, bounded request bodies, request deadlines, server timeouts, panic recovery, CORS, and security headers
- Request-scoped Zap logging and W3C trace correlation through
  [echo-observability v2](https://pkg.go.dev/github.com/janisto/echo-observability/v2), without raw paths, peer IPs,
  user agents, or returned error text in access logs
- RFC 9457 Problem Details with JSON and CBOR response negotiation
- Cursor pagination with RFC 8288 `Link` headers
- Firebase ID-token validation with revocation checks and explicit dependency-failure handling
- Firestore CRUD with atomic create, field-specific transactional PATCH, existence-preconditioned delete, transient
  dependency classification, and audit events
- Generated and embedded OpenAPI 3.1 JSON with an exact seven-operation semantic contract, matched YAML, and SRI-pinned Swagger UI
- A separate minimal Google Functions Framework example
- Required CI checks for both Go modules, emulators, race detection, vulnerabilities, generated artifacts, and the final image

## HTTP contract

JSON is the only supported request-body format. Successful and error responses use JSON by default and CBOR when
the `Accept` header prefers `application/cbor`.
Negotiation follows RFC 9110 specificity and quality rules, so an explicit `q=0` exclusion overrides a broader
range when another supported representation remains. A request with no acceptable success representation returns 406;
when the client also rejects both Problem Details formats, the 406 explanation uses JSON as a final diagnostic fallback.
Echo 5.3 automatically serves bodyless HEAD responses through the corresponding GET route.

Errors use:

- `application/problem+json`
- `application/problem+cbor`

Malformed input returns 400. Valid input that fails field or PATCH semantics returns 422. The `/health`
endpoint is dependency-free liveness; it does not claim Firebase readiness. Query contracts are closed and reject
repeated scalar parameters instead of choosing an arbitrary value.

| Method | Path | Result |
|---|---|---|
| GET | `/health` | Liveness |
| GET | `/v1/hello` | Default greeting |
| POST | `/v1/hello` | Computed personalized greeting, 200 |
| GET | `/v1/items` | Filtered cursor-paginated sample data |
| POST | `/v1/profile` | Create authenticated profile, 201 |
| GET | `/v1/profile` | Read authenticated profile |
| PATCH | `/v1/profile` | Update at least one supplied field |
| DELETE | `/v1/profile` | Delete authenticated profile, 204 |

## Requirements

- Go 1.26.5+
- [Just](https://github.com/casey/just)
- [golangci-lint](https://golangci-lint.run/) v2
- [Firebase CLI](https://firebase.google.com/docs/cli) for Auth and Firestore emulators
- Docker or Podman for the final-image checks
- Google Cloud CLI only when deploying the function example
- [Gremlins](https://github.com/go-gremlins/gremlins) only when running mutation tests
- [actionlint](https://github.com/rhysd/actionlint) only when validating workflows locally (`brew install actionlint` on macOS)
- [zizmor](https://docs.zizmor.sh/) only when auditing GitHub Actions locally

The root service and `functions/` are independent Go modules. An optional ignored `go.work` is useful
for editor navigation, but repository recipes set `GOWORK=off` for nested-module checks so a clean checkout behaves
the same as CI.

## Quick start

```bash
cp .env.example .env
just run
```

Open:

- http://localhost:8080/health
- http://localhost:8080/api-docs
- http://localhost:8080/api-docs/openapi.json

The default `FIREBASE_MODE=offline` keeps public health, docs, hello, and items routes available while protected routes
return 503. Use explicit `emulator` mode for local Auth and Firestore, or `live` mode with a real project and ADC.
Offline and emulator modes are rejected outside development. Live mode rejects demo projects and emulator hosts so a
deployed misconfiguration cannot accept unsigned emulator tokens or silently fall back offline.

## Configuration

`just` loads `.env`. The binary validates only settings it actually implements:

| Variable | Default | Meaning |
|---|---|---|
| `HOST` | `0.0.0.0` | Listen host |
| `PORT` | `8080` | Listen port, 1-65535 |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `APP_ENVIRONMENT` | `development` | `development`, `staging`, or `production` |
| `FIREBASE_MODE` | `offline` | `offline`, `emulator`, or `live` |
| `FIREBASE_PROJECT_ID` | `demo-test-project` outside live mode | Firebase project |
| `GOOGLE_APPLICATION_CREDENTIALS` | ADC | Optional service-account file |

CORS intentionally permits all origins for this public playground API, does not permit credentialed browser requests,
and permits the paired W3C `Traceparent` and `Tracestate` headers. Narrow origins before adapting the example to a private API.

## Development commands

| Command | Scope |
|---|---|
| `just check` | Format-check, lint, build, and test both modules |
| `just build`, `just test`, `just lint` | Run the named check for both modules |
| `just coverage` | Test both modules and generate separate coverage reports |
| `just test-race`, `just vuln` | Race-test or vulnerability-scan both modules |
| `just mutation` | Mutation-test both modules with Gremlins |
| `just mutation-app`, `just mutation-functions` | Mutation-test one module |
| `just fuzz [target] [duration] [package]` | Fuzz one root-module target for a bounded duration |
| `just fuzz-functions [target] [duration]` | Fuzz the separate function module |
| `just fuzz-all [duration]` | Run every curated fuzz target; duration applies to each target |
| `just functions-check` | Narrow build, test, and lint of the function module |
| `just update` | Update root dependencies, root Go tools, and the function module |
| `just functions-update` | Update only the function module |
| `just functions-run 8081` | Run target `Hello` through the official Functions Framework |
| `just docs` | Generate, normalize, and embed-review OpenAPI artifacts |
| `just emulators` | Start Auth and Firestore emulators |
| `just test-integration-ci` | Require emulators and generate separate integration coverage |
| `just workflow-check`, `just workflow-security-check` | Run local actionlint or zizmor against GitHub Actions |
| `just modernize-check` | Report available Go modernizations without changing files |
| `just container-smoke` | Build and verify the final service image |

Firebase integration tests skip locally when emulators are absent. CI sets
`REQUIRE_FIREBASE_EMULATORS=1`, which converts absence into failure.

## Mutation testing

Install Gremlins with Homebrew on macOS before running mutation tests:

```bash
brew tap go-gremlins/tap
brew install gremlins
```

Then run its mutation campaigns against covered production code in both Go modules:

```bash
just mutation
```

The unqualified command runs separate root and `functions/` campaigns; use `just mutation-app` or
`just mutation-functions` for a deliberately narrow run. Gremlins changes expressions and conditions, then checks
whether the existing tests detect each behavioral change. In addition to Gremlins' default mutations, this repository
enables logical-operator inversion for compound guards and bitwise inversion for CBOR masks, encoded lengths, and size
limits. Review `LIVED` mutants as possible test gaps; equivalent transformations do not need artificial assertions.

Gremlins mutates only code covered by the tests available to that run. With no Firebase emulators running, Auth and
Firestore integration tests skip and their unexecuted mutations are reported as `NOT COVERED`, not `LIVED`. Start
`just emulators` in another terminal before `just mutation-app` when changing those paths. Mutation testing
intentionally runs outside `just qa` and may take several minutes. The configured per-mutant safety timeout does not
limit the total campaign time.

## Fuzz testing

Go's native fuzzing engine targets the input spaces where examples alone are least convincing:

| Target | Package | Invariant |
|---|---|---|
| `FuzzDecodeJSON` | `./internal/platform/request` | Content type, object shape, unknown fields, and trailing values follow the strict request contract |
| `FuzzRejectUnknownOrRepeatedQuery` | `./internal/platform/request` | Malformed, unknown, and repeated scalar query parameters fail closed |
| `FuzzDecodeCursor` | `./internal/platform/pagination` | Every encoded cursor round-trips; arbitrary accepted cursors remain stable |
| `FuzzPaginate` | `./internal/platform/pagination` | Page bounds, item order, next/previous cursors, filters, and caller-owned query values stay consistent |
| `FuzzSelectFormat` | `./internal/platform/respond` | Media-type selection is invariant to token casing and surrounding whitespace |
| `FuzzSelectFormatQuality` | `./internal/platform/respond` | Exact JSON/CBOR quality ordering, ties, exclusions, and header order select the documented format |
| `FuzzTimeUnmarshalCBOR` | `./internal/platform/timeutil` | Accepted CBOR timestamps canonicalize to the same millisecond on encode/decode |
| `FuzzTimeCBORRoundTrip` | `./internal/platform/timeutil` | Canonically encoded timestamps always decode to the same millisecond |
| `FuzzHelloHandler` | `./functions` | GET/POST precedence, Unicode rune counting, and the 100-rune boundary remain exact |

Run the default ten-second strict-JSON session, select one target while developing, or exercise the complete curated
set. `just fuzz-all` applies its duration to each target, so its total run is longer:

```bash
just fuzz
just fuzz FuzzPaginate 1m ./internal/platform/pagination
just fuzz-functions
just fuzz-all 30s
```

The equivalent native Go command for the default target is:

```bash
go test -fuzz='^FuzzDecodeJSON$' -fuzztime=10s ./internal/platform/request
```

Go first replays the seed corpus and then generates new inputs. When fuzzing finds a failure, it minimizes the input and
writes it below the target package at `testdata/fuzz/<target>`; `just test` or the corresponding module's
`go test ./...` runs saved corpus inputs as regression tests. Review and commit a failing input together with the fix
when it represents behavior the boundary must preserve.

See the [Go fuzzing documentation](https://go.dev/doc/security/fuzz/) for the engine's workflow and additional flags.

## Go function: Firebase CLI versus gcloud

The separate `functions/` module is intentionally a small standard-library HTTP example. It does not import Echo,
the root application architecture, Firebase Admin, or the root observability stack.
POST accepts exactly one known-field JSON object with `Content-Type: application/json`; GET accepts one optional
`name` query parameter. Unknown, repeated, or malformed query parameters are rejected.

**Firebase CLI cannot deploy this Go module.** Its
[supported function runtime type](https://github.com/firebase/firebase-tools/blob/main/src/deploy/functions/runtimes/supported/types.ts)
contains Node.js, Python, and Dart, not Go. Firebase CLI is used in this repository only for the Auth and Firestore emulators.

The module uses the official Google Functions Framework for Go. Deploy it as a
[Cloud Run function](https://docs.cloud.google.com/run/docs/deploy-functions) with Google Cloud CLI and the Go 1.26 runtime:

```bash
gcloud run deploy echo-playground-hello \
  --source functions \
  --function Hello \
  --base-image go126 \
  --region REGION
```

The registered handler is at the `functions/` source root beside `go.mod`, as required by the Go function
build contract.

Run the same registry and target path locally with `just functions-run 8081`.

## Firebase emulators

```bash
just emulators
```

| Emulator | Address |
|---|---|
| Auth | `127.0.0.1:7110` |
| Firestore | `127.0.0.1:7130` |
| UI | http://localhost:4000 |

Tests configure emulator addresses themselves. Emulator variables are development-only; startup rejects them in every
other environment so a deployed service cannot accidentally accept emulator-issued unsigned tokens. Application
emulator mode requires both variables as strict `host:port` authorities and a `demo-*` project ID.

`firestore.rules` denies all client SDK access. As the
[official Firestore guidance](https://firebase.google.com/docs/firestore/security/rules-conditions) explains, server
libraries bypass Security Rules and authenticate through ADC/IAM; the deny-all rule prevents this server-owned collection
from being exposed accidentally to clients.

## OpenAPI and Swagger UI

`just docs` runs swag, applies deterministic OpenAPI corrections that swag v2 RC cannot currently express, and
writes equivalent JSON and YAML. Semantic tests enforce the exact path, method, operation ID, status, security, request
media, response media, and required-header matrix. CI regenerates the artifacts and rejects any diff. The service embeds
`api-docs/swagger.json`, so documentation does not depend on its runtime working directory.

Swagger UI uses exact version 5.32.11 assets with SHA-384 integrity metadata. A docs-specific CSP permits scripts and
styles from `unpkg.com`; SRI pins the selected external bytes, while the initialization script is embedded and same-origin.

## Project layout

```text
.agents/skills/         Six portable project workflows with Codex UI metadata
.github/agents/        Evidence-based security review profile for GitHub Copilot
api-docs/              Generated OpenAPI plus embedded spec
cmd/openapi/           Deterministic generated-spec normalization
cmd/server/            Process lifecycle, typed config, and application composition
functions/             Independent Functions Framework Go module
internal/http/         Health, docs, and versioned Echo handlers
internal/platform/     Auth, middleware, pagination, responses, validation
internal/service/      Firestore profile implementation and service contracts
internal/testutil/     Echo, reusable test fakes, and Firebase emulator support
```

Repository guidance follows the canonical [AGENTS.md format](https://github.com/agentsmd/agents.md). Portable skills use
the canonical [Agent Skills specification and documentation](https://github.com/agentskills/agentskills), with the
detailed [format specification](https://agentskills.io/specification), under `.agents/skills/`. See
[AGENTS.md](AGENTS.md) for the working rules.

## Container

```bash
just container-build echo-playground:local local
just container-up echo-playground:local
```

`container-up` treats its optional third argument as the host port and always runs the application on container port 8080,
so a local `.env` `PORT` value cannot desynchronize the port mapping.

The distroless final image is non-root and embeds the OpenAPI document. Its OCI version label is distinct from source
revision metadata; no revision label is emitted unless a real source revision is added deliberately. Rebuild it for both
Go standard-library and base-image security fixes. Cloud Run automatic base-image rebasing is not a substitute for
rebuilding a compiled Go binary.

This repository is not deployed to production. If publishing an image, use an immutable registry digest and an explicit
release workflow rather than `latest`.

## CI

GitHub Actions use least-privilege read tokens and exact release tags, such as `actions/checkout@v7.0.1` and
`actions/setup-go@v7.0.0`, for consistent, readable Dependabot updates. This convention accepts mutable upstream tags
instead of immutable commit pins. Required jobs cover:

- root and function build/test/race/coverage checks plus bounded pull-request fuzzing;
- Auth and Firestore emulator tests with fail-on-missing behavior and separate downloadable coverage;
- both-module vulnerability scans;
- OpenAPI regeneration and semantic validation;
- final container probes for liveness, embedded docs, non-root execution, and honest OCI metadata;
- both-module formatting, linting, module tidiness, Go modernization, and pinned zizmor checks.

Branch protection requires the stable aggregate checks `ci` and `lint`. Each aggregate runs even when a dependency fails
and succeeds only when every specialized job in its workflow succeeds; internal job names are not part of the ruleset contract.

Dependabot checks both Go modules, GitHub Actions, and Docker base images quarterly after a one-day release cooldown.
Repository automation also labels application, function, documentation, and tooling changes and enables squash auto-merge for
Dependabot minor and patch updates, subject to repository branch protections and required checks.

## Contributing

See [AGENTS.md](AGENTS.md) for repository-specific engineering and verification rules.

## License

MIT
