# Justfile for Echo Playground
# https://github.com/casey/just

set dotenv-load

PORT := env("PORT", "8080")

CONTAINER_RUNTIME := if `command -v podman 2>/dev/null || true` != "" { "podman" } else { "docker" }

@_:
    just --list

[group('build')]
build: build-app build-functions

[group('build')]
build-app:
    go build -v ./...

[group('build')]
build-functions:
    cd functions && GOWORK=off go build -v ./...

[group('run')]
functions-run port="8080":
    cd functions && GOWORK=off FUNCTION_TARGET=Hello PORT={{ port }} go run ./cmd/server

[group('build')]
clean:
    rm -f coverage.out coverage.html coverage-summary.txt \
        integration-coverage.out integration-coverage.html integration-coverage-summary.txt \
        firebase-debug.log firestore-debug.log ui-debug.log

# Run the server
[group('run')]
run:
    go run ./cmd/server

# Run the server with custom port
[group('run')]
run-port port=PORT:
    PORT={{ port }} go run ./cmd/server

# Generate OpenAPI 3.1 spec
alias docs := gen-openapi
[group('build')]
gen-openapi:
    go tool swag init --quiet --v3.1 --parseInternal --outputTypes json,yaml -g cmd/server/main.go -o api-docs >/dev/null
    go run ./cmd/openapi

# Format swag annotations
[group('build')]
fmt-openapi:
    go tool swag fmt

# Start Firebase emulators for E2E tests
[group('test')]
emulators:
    firebase emulators:start --only auth,firestore

[group('test')]
test *args:
    just test-app {{ args }}
    just test-functions {{ args }}

[group('test')]
test-app *args:
    go test ./... {{ args }}

[group('test')]
test-functions *args:
    cd functions && GOWORK=off go test ./... {{ args }}

[group('test')]
test-race: test-race-app test-race-functions

[group('test')]
test-race-app:
    go test -race ./...

[group('test')]
test-race-functions:
    cd functions && GOWORK=off go test -race ./...

[group('test')]
test-verbose *args:
    just test-app -v {{ args }}
    just test-functions -v {{ args }}

[group('test')]
test-integration-ci *args:
    REQUIRE_FIREBASE_EMULATORS=1 firebase emulators:exec --only auth,firestore --project demo-test-project \
        "just test-app -count=1 -covermode=atomic -coverpkg=./... -coverprofile=integration-coverage.out {{ args }}"
    go tool cover -func=integration-coverage.out > integration-coverage-summary.txt
    go tool cover -html=integration-coverage.out -o integration-coverage.html

[group('test')]
functions-smoke port="18081":
    #!/usr/bin/env bash
    set -euo pipefail
    tmp="$(mktemp -d)"
    cleanup() {
      result=$?
      if [[ "$result" -ne 0 ]]; then
        [[ -f "$tmp/log" ]] && tail -n 200 "$tmp/log" >&2
        [[ -f "$tmp/response" ]] && tail -n 200 "$tmp/response" >&2
      fi
      if [[ -n "${pid:-}" ]]; then kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi
      rm -rf "$tmp"
    }
    trap cleanup EXIT
    (cd functions && GOWORK=off go build -o "$tmp/server" ./cmd/server)
    FUNCTION_TARGET=Hello PORT={{ port }} "$tmp/server" >"$tmp/log" 2>&1 &
    pid=$!
    for _ in {1..30}; do
      if curl --fail --silent "http://127.0.0.1:{{ port }}/?name=Smoke" >"$tmp/response"; then
        if grep -F '"message":"Hello, Smoke!"' "$tmp/response" >/dev/null; then exit 0; fi
      fi
      sleep 0.2
    done
    exit 1

# Run tests with coverage
[group('test')]
test-coverage *args:
    go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./... {{ args }}

# Generate coverage report
[group('test')]
coverage: test-coverage
    go tool cover -func=coverage.out | tee coverage-summary.txt
    go tool cover -html=coverage.out -o coverage.html

[group('qa')]
lint: lint-app lint-functions

[group('qa')]
lint-app:
    golangci-lint run ./...

[group('qa')]
lint-functions:
    cd functions && GOWORK=off golangci-lint run ./...

[group('qa')]
fmt: fmt-app fmt-functions

[group('qa')]
fmt-app:
    golangci-lint fmt ./...

[group('qa')]
fmt-functions:
    cd functions && GOWORK=off golangci-lint fmt ./...

[group('qa')]
fmt-check: fmt-check-app fmt-check-functions

[group('qa')]
fmt-check-app:
    #!/usr/bin/env bash
    set -euo pipefail
    diff="$(golangci-lint fmt --diff ./...)"
    if [[ -n "$diff" ]]; then printf '%s\n' "$diff"; exit 1; fi

[group('qa')]
fmt-check-functions:
    #!/usr/bin/env bash
    set -euo pipefail
    diff="$(cd functions && GOWORK=off golangci-lint fmt --diff ./...)"
    if [[ -n "$diff" ]]; then printf '%s\n' "$diff"; exit 1; fi

[group('qa')]
fix: fix-app fix-functions

[group('qa')]
fix-app:
    golangci-lint run --fix ./...

[group('qa')]
fix-functions:
    cd functions && GOWORK=off golangci-lint run --fix ./...

[group('qa')]
vuln: vuln-app vuln-functions

[group('qa')]
vuln-app:
    go tool govulncheck ./...

[group('qa')]
vuln-functions:
    cd functions && GOWORK=off go tool -modfile=../go.mod govulncheck ./...

[group('qa')]
workflow-check:
    go tool actionlint

[group('qa')]
modernize-check:
    go fix -diff ./...
    cd functions && GOWORK=off go fix -diff ./...

[group('qa')]
qa: tidy fix build test

[group('qa')]
check: fmt-check lint build test

[group('qa')]
functions-check: lint-functions build-functions test-functions

alias check-all := check
alias qa-all := qa
alias functions-build := build-functions
alias functions-test := test-functions
alias functions-test-race := test-race-functions
alias functions-lint := lint-functions
alias functions-fix := fix-functions
alias functions-vuln := vuln-functions

alias install := download
[group('lifecycle')]
download: download-app download-functions

[group('lifecycle')]
download-app:
    go mod download

[group('lifecycle')]
download-functions:
    cd functions && GOWORK=off go mod download

[group('lifecycle')]
tidy: tidy-app tidy-functions

[group('lifecycle')]
tidy-app:
    go mod tidy

[group('lifecycle')]
tidy-functions:
    cd functions && GOWORK=off go mod tidy

[group('lifecycle')]
tidy-check:
    go mod tidy -diff
    cd functions && GOWORK=off go mod tidy -diff

# Update both modules and root Go tools to their latest compatible versions
[group('lifecycle')]
update: update-app update-functions

[group('lifecycle')]
update-app:
    go get -u -t ./... tool
    go mod tidy

[group('lifecycle')]
update-functions:
    cd functions && GOWORK=off go get -u -t ./...
    cd functions && GOWORK=off go mod tidy

[group('lifecycle')]
fresh: clean download build

alias functions-tidy := tidy-functions
alias functions-update := update-functions
alias update-root := update-app

# Container tasks
[group('container')]
container-build image="echo-playground:latest" version="dev" runtime_img="":
    {{ CONTAINER_RUNTIME }} build \
        --build-arg VERSION={{ version }} \
        {{ if runtime_img != "" { "--build-arg RUNTIME_IMAGE=" + runtime_img } else { "" } }} \
        -t {{ image }} .

[group('container')]
container-up image="echo-playground:latest" name="echo-playground" host_port=PORT:
    {{ CONTAINER_RUNTIME }} run -d --rm --name {{ name }} \
        {{ if path_exists(".env") == "true" { "--env-file .env" } else { "" } }} \
        -e PORT=8080 -p {{ host_port }}:8080 {{ image }}

[group('container')]
container-smoke image="echo-playground:smoke" name="echo-playground-smoke" host_port="18080":
    #!/usr/bin/env bash
    set -euo pipefail
    runtime="{{ CONTAINER_RUNTIME }}"
    cleanup() {
      result=$?
      if [[ "$result" -ne 0 ]]; then "$runtime" logs {{ name }} 2>&1 | tail -n 200 >&2 || true; fi
      "$runtime" stop {{ name }} >/dev/null 2>&1 || true
    }
    trap cleanup EXIT
    just container-build {{ image }} ci-smoke
    test "$("$runtime" image inspect --format '{{ "{{.Config.User}}" }}' {{ image }})" = "65532:65532"
    test "$("$runtime" image inspect --format '{{ "{{index .Config.Labels \"org.opencontainers.image.version\"}}" }}' {{ image }})" = "ci-smoke"
    test -n "$("$runtime" image inspect --format '{{ "{{index .Config.Labels \"org.opencontainers.image.base.name\"}}" }}' {{ image }})"
    test "$("$runtime" image inspect --format '{{ "{{index .Config.Labels \"org.opencontainers.image.source\"}}" }}' {{ image }})" = "https://github.com/janisto/echo-playground"
    ! "$runtime" image inspect --format '{{ "{{json .Config.Labels}}" }}' {{ image }} | \
      grep -q 'org.opencontainers.image.revision'
    "$runtime" run -d --rm --name {{ name }} \
      -e APP_ENVIRONMENT=development \
      -e FIREBASE_MODE=offline \
      -e PORT=8080 \
      -p {{ host_port }}:8080 {{ image }} >/dev/null
    for _ in {1..60}; do
      if curl --fail --silent "http://127.0.0.1:{{ host_port }}/health" >/dev/null; then break; fi
      sleep 0.25
    done
    curl --fail --silent "http://127.0.0.1:{{ host_port }}/api-docs" >/dev/null
    curl --fail --silent "http://127.0.0.1:{{ host_port }}/api-docs/openapi.json" | \
      grep -E '"openapi":[[:space:]]*"3\.1\.0"' >/dev/null
    "$runtime" logs {{ name }} 2>&1 | grep -F '"version":"ci-smoke"' >/dev/null

[group('container')]
container-logs name="echo-playground":
    {{ CONTAINER_RUNTIME }} logs -f {{ name }}

[group('container')]
container-down name="echo-playground":
    -{{ CONTAINER_RUNTIME }} stop {{ name }}
