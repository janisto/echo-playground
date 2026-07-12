---
name: readme-maintenance
description: Audit or update echo-playground README.md when routes, configuration, development commands, CI, containers, Firebase behavior, OpenAPI, or function deployment guidance changes.
---

# README maintenance

Read `AGENTS.md` first, then verify every affected `README.md` claim against the current repository. Keep the README
focused on software engineers and new contributors; keep agent execution rules and detailed coding patterns in
`AGENTS.md` or task-specific skills.

## Source of truth

Read only the areas relevant to the documentation change, using these files as the primary map:

- application and configuration: `cmd/server/main.go`, `cmd/server/application.go`, and `cmd/server/config.go`;
- routes and contracts: `internal/http/health/`, `internal/http/v1/routes/`, and registered handler packages;
- API documentation: `api-docs/embed.go`, `cmd/openapi/`, and `internal/http/docs/`;
- separate function: `functions/function.go`, `functions/cmd/server/main.go`, and `functions/go.mod`;
- tooling and deployment: `Justfile`, `Dockerfile`, `firebase.json`, and `.github/workflows/`;
- dependencies and versions: `go.mod`, `functions/go.mod`, and pinned workflow or container references.

Do not copy historical claims from `plans/` into the README without re-verifying them against current code and primary
documentation.

## Required README contract

Keep the document concise and onboarding-oriented. Preserve or update these subjects when they are implemented:

- project purpose and significant capabilities;
- HTTP routes, status semantics, JSON requests, and negotiated JSON or CBOR responses;
- requirements and a working quick start;
- implemented environment variables, safe proxy defaults, CORS premise, and local Firebase behavior;
- root and function module development commands;
- the explicit distinction that Firebase CLI runs Auth and Firestore emulators but does not deploy the Go function;
- the `gcloud run deploy --source functions --function Hello --base-image go126` function path;
- emulator, OpenAPI generation, embedded Swagger UI, container, and CI behavior;
- concise project layout, contribution pointer, and license.

Organize the material for readers rather than preserving a rigid heading order. Remove stale sections instead of
maintaining compatibility with old README structure.

## Accuracy rules

- Every command must exist and use its current argument order.
- Every documented path and route must exist.
- Defaults and failure behavior must match `loadConfig` and application composition.
- Describe the two Go modules as independent; do not imply root `./...` crosses into `functions/`.
- Describe `/health` as dependency-free liveness, not Firebase readiness.
- State that CBOR is response-only unless request decoding is actually implemented.
- Keep emulator variables development-only and explain why production rejects them.
- Keep the separate function deliberately small; do not describe it as an Echo or Firebase Admin application.
- Do not claim this repository is deployed to production.
- Prefer primary sources for runtime, framework, deployment, and security claims that may change.
- Do not add agent instructions, source-level tutorials, speculative features, or duplicated `AGENTS.md` content.

## Verification

Verify named recipes without mutating dependencies:

```bash
just --dry-run build
just --dry-run test
just --dry-run lint
just --dry-run check-all
just --dry-run functions-check
just --dry-run functions-vuln
just --dry-run functions-run 8081
just --dry-run docs
just --dry-run update
```

Search route registration and configuration directly rather than trusting generated prose. If documentation changes
alongside application behavior, run `just build`, `just test`, and `just lint`; use `just check-all` for cross-module
changes. For a documentation-only correction, validate links, paths, commands, YAML where touched, and `git diff --check`.

Before finishing, reread the complete README once for contradictions, duplicated explanations, unexpanded acronyms, and
claims that are more confident than the evidence.
