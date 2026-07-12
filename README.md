# Echo Playground

A compact, high-quality REST API example built with [Echo v5](https://github.com/labstack/echo/tree/v5) and Go 1.26.
It demonstrates HTTP contracts, structured observability, Firebase Authentication, Firestore CRUD, OpenAPI 3.1,
and production-shaped verification without pretending to be a complete production platform.

<img src="assets/gopher.svg" alt="Go Gopher mascot illustration" width="400">

<sub>Gopher illustration from [free-gophers-pack](https://github.com/MariaLetta/free-gophers-pack) by Maria Letta</sub>

## Features

- Echo v5 with strict single-object JSON decoding, bounded request bodies, request deadlines, server timeouts, panic recovery, CORS, and security headers
- Request-scoped Zap logging through [echo-observability](https://pkg.go.dev/github.com/janisto/echo-observability)
- RFC 9457 Problem Details with JSON and CBOR response negotiation
- Cursor pagination with RFC 8288 `Link` headers
- Firebase ID-token validation with revocation checks and explicit dependency-failure handling
- Firestore CRUD with atomic create, field-specific transactional PATCH, and audit events
- Generated and embedded OpenAPI 3.1 JSON, semantically matched YAML, and SRI-pinned Swagger UI
- A separate minimal Google Functions Framework example
- Required CI checks for both Go modules, emulators, race detection, vulnerabilities, generated artifacts, and the final image

## HTTP contract

JSON is the only supported request-body format. Successful and error responses use JSON by default and CBOR when
the `Accept` header prefers `application/cbor`.
Negotiation follows RFC 9110 specificity and quality rules, so an explicit `q=0` exclusion overrides a broader
range when another supported representation remains. Requests with no supported range retain the documented JSON fallback.

Errors use:

- `application/problem+json`
- `application/problem+cbor`

Malformed input returns 400. Valid input that fails field or PATCH semantics returns 422. The `/health`
endpoint is dependency-free liveness; it does not claim Firebase readiness.

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

The default development project ID is `demo-test-project`. Firebase-protected routes require the local emulators
or explicitly configured real Firebase credentials; the public health, docs, hello, and items routes run locally without them.
Demo project IDs are rejected outside development so an offline fallback cannot mask a deployed misconfiguration.
Firebase emulator variables are also rejected outside development because the Auth emulator accepts unsigned test tokens.

## Configuration

`just` loads `.env`. The binary validates only settings it actually implements:

| Variable | Default | Meaning |
|---|---|---|
| `HOST` | `0.0.0.0` | Listen host |
| `PORT` | `8080` | Listen port, 1-65535 |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `APP_ENVIRONMENT` | `development` | Environment label |
| `FIREBASE_PROJECT_ID` | `demo-test-project` in development | Firebase project |
| `IP_EXTRACTOR` | `direct` | `direct` or explicitly configured `xff` proxy mode |
| `GOOGLE_APPLICATION_CREDENTIALS` | ADC | Optional service-account file |

`direct` is deliberately safe by default and ignores forwarding headers. Use `xff` only when the service
is behind a trusted proxy topology such as Cloud Run. Client IP is observational data, never authorization input.

CORS intentionally permits all origins for this public playground API, does not permit credentialed browser requests,
and limits preflight methods and headers to the implemented contract. Narrow origins before adapting the example to a private API.

## Development commands

| Command | Scope |
|---|---|
| `just check` | Build, test, and lint the root service |
| `just functions-check` | Build, test, and lint the function module |
| `just check-all` | Check both modules |
| `just test-race` | Root race detector |
| `just functions-test-race` | Function race detector |
| `just vuln` | Root vulnerability scan |
| `just functions-vuln` | Function vulnerability scan |
| `just update` | Update root dependencies, root Go tools, and the function module |
| `just functions-update` | Update only the function module |
| `just functions-run 8081` | Run target `Hello` through the official Functions Framework |
| `just docs` | Generate, normalize, and embed-review OpenAPI artifacts |
| `just emulators` | Start Auth and Firestore emulators |
| `just container-build` | Build the final service image |

Firebase integration tests skip locally when emulators are absent. CI sets
`REQUIRE_FIREBASE_EMULATORS=1`, which converts absence into failure.

## Go function: Firebase CLI versus gcloud

The separate `functions/` module is intentionally a small standard-library HTTP example. It does not import Echo,
the root application architecture, Firebase Admin, or the root observability stack.
POST accepts exactly one known-field JSON object with `Content-Type: application/json`; GET accepts an optional `name` query parameter.

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
other environment so a deployed service cannot accidentally accept emulator-issued unsigned tokens.

`firestore.rules` denies all client SDK access. As the
[official Firestore guidance](https://firebase.google.com/docs/firestore/security/rules-conditions) explains, server
libraries bypass Security Rules and authenticate through ADC/IAM; the deny-all rule prevents this server-owned collection
from being exposed accidentally to clients.

## OpenAPI and Swagger UI

`just docs` runs swag, applies deterministic OpenAPI corrections that swag v2 RC cannot currently express, and
writes equivalent JSON and YAML. CI regenerates the artifacts and rejects any diff. The service embeds
`api-docs/swagger.json`, so documentation does not depend on its runtime working directory.

Swagger UI uses exact version 5.32.8 assets with SHA-384 integrity metadata. A docs-specific CSP permits only those pinned
assets and the embedded same-origin initialization script.

## Project layout

```text
.agents/skills/         Five portable project workflows with Codex UI metadata
.github/agents/        Evidence-based security review profile for GitHub Copilot
api-docs/              Generated OpenAPI plus embedded spec
cmd/openapi/           Deterministic generated-spec normalization
cmd/server/            Process lifecycle, typed config, and application composition
functions/             Independent Functions Framework Go module
internal/http/         Health, docs, and versioned Echo handlers
internal/platform/     Auth, middleware, pagination, responses, validation
internal/service/      Firestore and in-memory profile implementations
internal/testutil/     Echo and Firebase emulator test support
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

The distroless final image is non-root and embeds the OpenAPI document. Rebuild it for both Go standard-library and base-image
security fixes. Cloud Run automatic base-image rebasing is not a substitute for rebuilding a compiled Go binary.

This repository is not deployed to production. If publishing an image, use an immutable registry digest and an explicit
release workflow rather than `latest`.

## CI

GitHub Actions use least-privilege read tokens and explicitly pinned action versions. By project convention,
`actions/setup-go` uses the exact `v6.5.0` release tag; other third-party actions use full commit SHAs. Required jobs cover:

- root and function build/test/race checks;
- Auth and Firestore emulator tests with fail-on-missing behavior;
- both-module vulnerability scans;
- OpenAPI regeneration and semantic validation;
- final container probes for liveness, embedded docs, bearer schema, and version metadata;
- root and function linting.

Branch protection requires the stable aggregate checks `ci` and `lint`. Each aggregate runs even when a dependency fails
and succeeds only when every specialized job in its workflow succeeds; internal job names are not part of the ruleset contract.

Dependabot tracks both Go modules, GitHub Actions, and Docker base images.
Repository automation also labels application, function, and documentation changes and enables squash auto-merge for
Dependabot minor and patch updates, subject to repository branch protections and required checks.

## Contributing

See [AGENTS.md](AGENTS.md) for repository-specific engineering and verification rules.

## License

MIT
