# Multi-stage build for optimized container
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags="-w -s -X main.version=v2.1.3 -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
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