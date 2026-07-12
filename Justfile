# Justfile for Echo Playground
# https://github.com/casey/just

set dotenv-load

PORT := env("PORT", "8080")

# Container runtime: prefer podman, fallback to docker
CONTAINER_RUNTIME := if `command -v podman 2>/dev/null || true` != "" { "podman" } else { "docker" }

@_:
    just --list

# Build the application
[group('build')]
build:
    go build -v ./...

[group('build')]
functions-build:
    cd functions && GOWORK=off go build -v ./...

[group('run')]
functions-run port="8080":
    cd functions && GOWORK=off FUNCTION_TARGET=Hello PORT={{ port }} go run ./cmd/server

# Clean build artifacts and coverage files
[group('build')]
clean:
    rm -f coverage.out coverage.html

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

# Run all tests
[group('test')]
test *args:
    go test ./... {{ args }}

[group('test')]
functions-test *args:
    cd functions && GOWORK=off go test ./... {{ args }}

[group('test')]
test-race:
    go test -race ./...

[group('test')]
functions-test-race:
    cd functions && GOWORK=off go test -race ./...

# Run tests with verbose output
[group('test')]
test-verbose *args:
    go test -v ./... {{ args }}

# Run tests with coverage
[group('test')]
test-coverage *args:
    go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./... {{ args }}

# Generate coverage report
[group('test')]
coverage: test-coverage
    go tool cover -func=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Run linter
[group('qa')]
lint:
    golangci-lint run ./...

[group('qa')]
functions-lint:
    cd functions && GOWORK=off golangci-lint run ./...

# Apply formatters (gci, gofumpt, golines)
[group('qa')]
fmt:
    golangci-lint fmt ./...

# Run linter and apply formatters
[group('qa')]
fix:
    golangci-lint run --fix ./...

# Check for vulnerabilities
[group('qa')]
vuln:
    go tool govulncheck ./...

[group('qa')]
functions-vuln:
    cd functions && GOWORK=off go tool -modfile=../go.mod govulncheck ./...

# Quality assurance: tidy, fix, build, and test
[group('qa')]
qa: tidy fix build test

[group('qa')]
qa-all: qa functions-tidy functions-fix functions-build functions-test

# Full check: lint, build, and test
[group('qa')]
check: lint build test

[group('qa')]
functions-check: functions-lint functions-build functions-test

[group('qa')]
check-all: check functions-check

# Download module dependencies
alias install := download
[group('lifecycle')]
download:
    go mod download

# Tidy go.mod and go.sum
[group('lifecycle')]
tidy:
    go mod tidy

[group('lifecycle')]
functions-tidy:
    cd functions && GOWORK=off go mod tidy

[group('qa')]
functions-fix:
    cd functions && GOWORK=off golangci-lint run --fix ./...

# Update both modules and root Go tools to their latest compatible versions
[group('lifecycle')]
update: update-root functions-update

[group('lifecycle')]
update-root:
    go get -u -t ./... tool
    go mod tidy

[group('lifecycle')]
functions-update:
    cd functions && GOWORK=off go get -u -t ./...
    cd functions && GOWORK=off go mod tidy

# Recreate the root module from clean state
[group('lifecycle')]
fresh: clean download build

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
container-logs name="echo-playground":
    {{ CONTAINER_RUNTIME }} logs -f {{ name }}

[group('container')]
container-down name="echo-playground":
    -{{ CONTAINER_RUNTIME }} stop {{ name }}
