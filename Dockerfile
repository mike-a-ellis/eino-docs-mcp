# Multi-stage build for minimal production image
# Build stage
FROM golang:1.24 AS builder
WORKDIR /build

# Copy dependency files first (for layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o mcp-server ./cmd/mcp-server

# Runtime stage (distroless for minimal attack surface)
FROM gcr.io/distroless/static-debian11

# Copy binary from build stage
COPY --from=builder /build/mcp-server /app/mcp-server

# Expose port (documentation only, Fly.io uses internal_port)
EXPOSE 8080

# Run as non-root (distroless has nonroot user built-in)
USER nonroot:nonroot

ENTRYPOINT ["/app/mcp-server"]
