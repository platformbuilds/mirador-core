# Multi-stage build for optimized container
# syntax=docker/dockerfile:1.6

# Use BuildKit-aware, platform-specific builder to avoid QEMU segfaults on M1
FROM --platform=$BUILDPLATFORM golang:1.23-bookworm AS builder

# Install build dependencies
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
# Ensure pure-Go DNS resolver to avoid musl/QEMU issues during mod download
ENV GODEBUG=netdns=go \
    GOPROXY=https://proxy.golang.org,direct

# Cache go modules and build cache (requires BuildKit)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy source code
COPY . .

# Build the binary with optimizations (multi-platform)
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=v0.0.0
ARG BUILD_TIME
ARG COMMIT_HASH
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -a -installsuffix cgo \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.commitHash=${COMMIT_HASH}" \
    -o mirador-core cmd/server/main.go

# Final stage - minimal runtime image
FROM scratch

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data for proper time handling
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary and configuration
COPY --from=builder /app/mirador-core /mirador-core
COPY --from=builder /app/api/openapi.yaml /api/openapi.yaml
COPY --from=builder /app/configs/ /configs/

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/mirador-core", "healthcheck"]

# Run as non-root user for security
USER 65534:65534

# Start the service
ENTRYPOINT ["/mirador-core"]
CMD ["serve"]
