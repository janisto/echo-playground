# syntax=docker/dockerfile:1.19.0
# Dockerfile for a Go Echo application
#
# Uses multi-stage build with official Go and distroless images.
# See: https://docs.docker.com/language/golang/build-images/
#
# Rebuild this image for Go standard-library and base-image security updates.

# Builder image: includes Go toolchain for compilation
ARG GO_IMAGE=golang:1.26-trixie
# Runtime image: minimal distroless image (no shell, no package manager)
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot
# Build arguments for version injection
ARG VERSION=dev

FROM ${GO_IMAGE} AS builder

ARG VERSION=dev

# CGO_ENABLED=0: Static binary required for distroless
# GOOS=linux: Explicit target OS for cross-build environments (Cloud Run is Linux)
ENV CGO_ENABLED=0 GOOS=linux

WORKDIR /app

# Install dependencies first so source-only changes reuse this layer.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .

# Build with optimizations and version injection
# -mod=readonly: Catch dirty module state early
# -buildvcs=false: Avoid VCS info embedding (cleaner CI builds without .git)
# -trimpath: Remove file system paths from binary for reproducibility
# -ldflags: Inject version and strip debug info for smaller binary
RUN go build -mod=readonly -buildvcs=false -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/server ./cmd/server

# Runtime stage: uses minimal distroless image
FROM ${RUNTIME_IMAGE} AS runtime

# OCI labels for image metadata
ARG RUNTIME_IMAGE
ARG VERSION
LABEL org.opencontainers.image.base.name="${RUNTIME_IMAGE}" \
      org.opencontainers.image.source="https://github.com/janisto/echo-playground" \
      org.opencontainers.image.version="${VERSION}"

# Copy the compiled binary with explicit permissions
COPY --from=builder --chmod=0555 /app/server /server

# Run as non-root user (UID 65532 = nonroot in distroless)
# Numeric UID for Kubernetes runAsNonRoot policy compatibility
USER 65532:65532

# Cloud Run expects the server to listen on $PORT (default 8080)
ENV PORT=8080
EXPOSE 8080

# Run the application (exec form for proper signal handling)
ENTRYPOINT ["/server"]
