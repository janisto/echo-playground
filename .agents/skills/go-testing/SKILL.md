---
name: go-testing
description: Write and review tests for echo-playground using Go testing, Echo v5 echotest or httptest, Firebase emulators, race checks, coverage, and the independent function module.
---

# Go testing

Read `AGENTS.md`, the implementation under test, and nearby tests before choosing the test boundary in either Go module.
Apply `$adversarial-testing` first to rank failure modes and select mutation-resistant cases; this skill supplies Go,
Echo, fixture, emulator, and command conventions.

## Test at the narrowest useful boundary

- Pure helpers and service behavior: ordinary table-driven unit tests.
- One handler: `github.com/labstack/echo/v5/echotest` or a small Echo instance from `testutil.NewTestEcho()`.
- Routing or middleware composition: `e.ServeHTTP` against the composed test router.
- Firestore and Firebase Auth behavior: the real local emulators, not production test switches.
- `functions/`: direct handler tests and the module's Functions Framework smoke path; do not import the root app.

Tests stay in `*_test.go`. Never add `if testing` branches, environment backdoors, or mock-only hooks to production
code. Refactor toward a small interface when a real dependency boundary needs substitution.

Reusable transport/composition fakes live in `internal/testutil/fake`. Tests inside `internal/platform/auth` or
`internal/service/profile` should use package-local test stubs to avoid import cycles. Production packages must not
export mocks, and production offline composition must use dependencies that return `ErrUnavailable`.

## Echo setup

`testutil.NewTestEcho()` installs the project validator and Problem Details error handler. Add only middleware needed by
the behavior under test:

```go
func setupTestServer(verifier auth.Verifier, svc profilesvc.Service) *echo.Echo {
	logger := zap.NewNop()
	e := testutil.NewTestEcho()
	const traceContextLevel = obs.TraceContextLevel1
	e.Use(
		obs.RequestContext(obs.RequestContextConfig{
			Logger: logger, Preset: obs.PresetGCP, TraceContextLevel: traceContextLevel,
		}),
		respond.Recoverer(logger),
		obs.AccessLogger(obs.AccessLoggerConfig{
			Logger: logger, Preset: obs.PresetGCP, TraceContextLevel: traceContextLevel,
		}),
	)

	e.GET("/health", health.Handler)
	routes.Register(e.Group("/v1"), verifier, svc)
	return e
}
```

Keep recovery outside `obs.AccessLogger`; v2 classifies and rethrows panics before recovery writes the error response.

Use `httptest.NewRequestWithContext(t.Context(), ...)`. Set `Content-Type: application/json` for bodies,
`Authorization: Bearer test-token` for protected routes, and `Accept` only when exercising negotiation. Set a fixed
`X-Request-ID` only when the assertion depends on correlation.

For isolated handlers, use `echotest.ContextConfig` and `respond.NewHTTPErrorHandler()`. Do not invent a full
application fixture when one context is sufficient.

## Assertions that matter

Verify observable contracts, not implementation trivia:

- exact HTTP status and relevant headers (`Content-Type`, `Location`, `Link`, `WWW-Authenticate`, `X-Request-ID`);
- decoded response fields and RFC 9457 `respond.ProblemDetails` errors;
- JSON and CBOR negotiation, including a specific `q=0` exclusion overriding broader wildcards and 406 when no success representation is acceptable;
- strict JSON failures: missing or unsupported content type, `null`, arrays, scalars, malformed syntax, unknown fields,
  multiple values, and the 1 MiB limit where application middleware is in scope;
- service error mapping, request deadlines, and no sensitive data in observed logs;
- pagination boundaries, invalid and stale cursors, preserved filters, and empty Link headers on terminal pages.
- panic recovery before commit returns 500 Problem Details; recovery after commit re-panics with
  `http.ErrAbortHandler` so the connection is aborted.

Use `t.Helper()` for assertion helpers, `t.Setenv()` for environment isolation, `errors.Is` for sentinel errors, and
fuzz seeds at parser boundaries. Avoid sleeps; use contexts, channels, or bounded polling.

## Firebase emulator tests

The helpers in `internal/testutil` own the test emulator addresses. Tests may skip when emulators are unavailable during
ordinary local runs; the emulator CI job requires them and fails if they are missing. Emulator REST helpers must reject
unexpected statuses before decoding bodies, use the bounded package client, bound diagnostics, and validate complete
Auth identities. Firestore cleanup must report client close failures.

Start the local emulators with:

```bash
just emulators
```

## Commands

Use Just so `.env` and the pinned toolchain are applied:

```bash
just test
just test-race
just coverage
just test-integration-ci
just functions-smoke
just check
```

The unqualified build, test, race, lint, formatting, vulnerability, tidy, and check recipes cover both modules. Use the
`*-app`, `*-functions`, or compatibility `functions-*` recipes only for a deliberately narrow first pass.
