# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy source code
COPY server/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION:-dev}" \
    -o /app/zimaos-blue \
    ./cmd/blue

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 blue && \
    adduser -u 1000 -G blue -s /bin/sh -D blue

# Create directories
RUN mkdir -p /app/data /app/config && \
    chown -R blue:blue /app

# Copy binary from builder
COPY --from=builder /app/zimaos-blue /app/zimaos-blue

# Copy default config if exists
COPY --from=builder /build/config.example.yaml /app/config/config.yaml

# Set working directory
WORKDIR /app

# Switch to non-root user
USER blue

# Expose port
EXPOSE 23456

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:23456/health || exit 1

# Environment variables
ENV BLUE_ENV=production \
    BLUE_DATA_DIR=/app/data \
    BLUE_CONFIG=/app/config/config.yaml

# Volumes
VOLUME ["/app/data", "/app/config"]

# Entry point
ENTRYPOINT ["/app/zimaos-blue"]
CMD ["--config", "/app/config/config.yaml"]
