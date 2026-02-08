# Multi-stage build for minimal production image
# Build stage for MCP server
FROM golang:1.24 AS builder
WORKDIR /build

# Copy dependency files first (for layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o mcp-server ./cmd/mcp-server && \
    go build -ldflags="-w -s" -o eino-sync ./cmd/sync

# Pull Qdrant binary from official Docker image (avoids GitHub releases download)
FROM qdrant/qdrant:v1.12.6 AS qdrant

# Runtime stage - use Debian 12 for Qdrant binary (requires GLIBC 2.34)
FROM debian:12-slim

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates netcat-openbsd libunwind8 && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy Qdrant binary from official image
COPY --from=qdrant /qdrant/qdrant /usr/local/bin/qdrant

# Copy binaries from build stage
COPY --from=builder /build/mcp-server /app/mcp-server
COPY --from=builder /build/eino-sync /app/eino-sync

# Create qdrant directory for process execution
RUN mkdir -p /qdrant && ln -s /usr/local/bin/qdrant /qdrant/qdrant

# Create storage directory with proper permissions
RUN mkdir -p /qdrant/storage && chmod 755 /qdrant/storage

# Copy supervisor script to run both processes
COPY supervisor.sh /supervisor.sh
RUN chmod +x /supervisor.sh

# Expose ports (documentation only, Fly.io uses internal_port)
EXPOSE 8080 6334

# Use supervisor script to run both Qdrant and MCP server
ENTRYPOINT ["/supervisor.sh"]
