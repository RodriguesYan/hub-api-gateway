# Multi-stage build for Hub API Gateway
# This Dockerfile follows best practices for Go microservices:
# - Multi-stage build for smaller image size
# - Non-root user for security
# - Health checks for container orchestration
# - Optimized layer caching

# ============================================================================
# Stage 1: Build
# ============================================================================
FROM golang:1.25.1-alpine AS builder

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD_DATE
ARG GIT_COMMIT

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    file

# Set working directory
WORKDIR /build

# Copy proto contracts first (required for local replace directive)
# Note: Build context must be parent directory for this to work
COPY hub-proto-contracts /hub-proto-contracts

# Copy dependency files first (better layer caching)
COPY hub-api-gateway/go.mod hub-api-gateway/go.sum ./

# Download dependencies (cached if go.mod/go.sum unchanged)
RUN go mod download && go mod verify

# Copy source code
COPY hub-api-gateway/ .

# Build binary with optimizations
# -ldflags explanation:
#   -w: Omit DWARF symbol table (reduces size)
#   -s: Omit symbol table and debug info (reduces size)
#   -X: Set version information
# Note: TARGETARCH is automatically set by Docker buildx for multi-arch builds
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build \
    -ldflags="-w -s \
    -X main.version=${VERSION} \
    -X main.buildDate=${BUILD_DATE} \
    -X main.gitCommit=${GIT_COMMIT}" \
    -a -installsuffix cgo \
    -o gateway \
    cmd/server/main.go

# Verify binary was created
RUN ls -lh gateway && file gateway

# ============================================================================
# Stage 2: Runtime
# ============================================================================
FROM alpine:3.19

# Install runtime dependencies
# - ca-certificates: For HTTPS connections
# - tzdata: For timezone support
# - wget: For health checks
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    wget

# Create non-root user for security
# Using specific UID/GID for consistency across environments
RUN addgroup -g 1000 gateway && \
    adduser -D -u 1000 -G gateway gateway

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder --chown=gateway:gateway /build/gateway .

# Copy configuration directory structure
# Note: Actual config files should be mounted or provided via env vars
COPY --from=builder --chown=gateway:gateway /build/config ./config

# Create directories for logs and temp files
RUN mkdir -p /app/logs /app/tmp && \
    chown -R gateway:gateway /app

# Switch to non-root user
USER gateway

# Expose HTTP port
EXPOSE 8080

# Add labels for metadata (OCI standard)
LABEL org.opencontainers.image.title="Hub API Gateway" \
      org.opencontainers.image.description="API Gateway for Hub Investments microservices" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.vendor="Hub Investments" \
      maintainer="Hub Investments Platform Team"

# Health check configuration
# - interval: Check every 30 seconds
# - timeout: Fail if check takes longer than 3 seconds
# - start-period: Wait 10 seconds before first check (startup time)
# - retries: Mark unhealthy after 3 consecutive failures
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run gateway
# Using exec form to ensure proper signal handling
CMD ["./gateway"]

