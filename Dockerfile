# Multi-stage Dockerfile for go-tor
# Stage 1: Build the Go binary
FROM golang:1.24-bookworm AS builder

# Set working directory
WORKDIR /build

# Copy source code (including go.mod and go.sum)
COPY . .

# Build the binary with optimizations
# go build will automatically download dependencies
# -ldflags: inject version info and strip debug symbols for smaller binary
# CGO_ENABLED=0: build static binary (no external C dependencies)
ARG VERSION=docker-dev
ARG BUILD_TIME
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -a -installsuffix cgo \
    -o tor-client \
    ./cmd/tor-client

# Verify the binary
RUN ls -lh tor-client && file tor-client || true

# Stage 2: Create minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot

# Labels for container metadata
LABEL maintainer="go-tor project" \
      description="Pure Go Tor client implementation" \
      org.opencontainers.image.source="https://github.com/opd-ai/go-tor" \
      org.opencontainers.image.documentation="https://github.com/opd-ai/go-tor/blob/main/docs/CONTAINERIZATION.md" \
      org.opencontainers.image.title="go-tor" \
      org.opencontainers.image.description="Educational Tor client implementation in Go"

# Copy CA certificates for TLS connections
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary from builder
COPY --from=builder --chown=nonroot:nonroot /build/tor-client /usr/local/bin/tor-client

# Create data directory with proper permissions
# distroless uses uid/gid 65532 for nonroot user
USER nonroot:nonroot
WORKDIR /home/nonroot

# Create volume mount point for persistent data
VOLUME ["/home/nonroot/.tor"]

# Expose standard Tor ports
# 9050: SOCKS5 proxy
# 9051: Control protocol
# 9052: HTTP metrics (when enabled)
EXPOSE 9050 9051 9052

# Health check using HTTP metrics endpoint
# Note: Only works when -metrics-port is specified in CMD/command
# For Kubernetes, use liveness/readiness probes with: curl http://localhost:9052/health
# Uncomment when metrics are enabled:
# HEALTHCHECK --interval=30s --timeout=10s --start-period=90s --retries=3 \
#     CMD wget --no-verbose --tries=1 --spider http://localhost:9052/health || exit 1

# Use STOPSIGNAL for graceful shutdown
STOPSIGNAL SIGTERM

# Default command: run with zero-configuration mode
# Users can override with their own flags via docker run
# For health checks, add: -metrics-port 9052
ENTRYPOINT ["/usr/local/bin/tor-client"]
CMD ["-data-dir", "/home/nonroot/.tor", "-socks-port", "9050", "-control-port", "9051"]
