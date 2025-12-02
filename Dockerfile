# syntax=docker/dockerfile:1.6

# Builder stage
FROM golang:1.24.10-bookworm AS builder

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
RUN go mod tidy && go mod download

# Copy source code
COPY . .

# Re-resolve modules after copying source to ensure go.sum includes new deps
RUN go mod download

# Build the binary with optimizations (multi-platform)
# Build for the builder's native platform; Buildx spawns per-arch builders.
ARG VERSION=v0.0.0
ARG BUILD_TIME
ARG COMMIT_HASH
ENV CGO_ENABLED=0
RUN go build \
    -a -installsuffix cgo \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.commitHash=${COMMIT_HASH} -X github.com/platformbuilds/mirador-core/internal/version.Version=${VERSION} -X github.com/platformbuilds/mirador-core/internal/version.CommitHash=${COMMIT_HASH} -X github.com/platformbuilds/mirador-core/internal/version.BuildTime=${BUILD_TIME}" \
    -o mirador-core cmd/server/main.go

# Final stage - lightweight runtime image based on Alpine so we can
# create directories and set permissions for the non-root user.
FROM alpine:3.18 AS runtime

# Install minimal utilities for healthcheck scripts if needed
RUN apk add --no-cache ca-certificates

# Create runtime directories and ensure proper ownership for the
# non-root user (UID 65534 used in the image below).
RUN mkdir -p /var/lib/mirador/bleve \
    && chown -R 65534:65534 /var/lib/mirador

# Copy CA certificates for HTTPS requests from the base image
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data for proper time handling
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary and configuration
COPY --from=builder /app/mirador-core /mirador-core
COPY --from=builder /app/api/openapi.yaml /api/openapi.yaml
COPY --from=builder /app/configs/ /configs/

# Expose ports
EXPOSE 8010 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/mirador-core", "healthcheck"]

# Run as non-root user for security (UID/GID 65534)
USER 65534:65534

# Start the service
ENTRYPOINT ["/mirador-core"]
CMD ["serve"]
