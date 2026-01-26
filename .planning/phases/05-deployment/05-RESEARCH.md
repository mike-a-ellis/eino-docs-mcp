# Phase 5: Deployment - Research

**Researched:** 2026-01-25
**Domain:** Fly.io deployment for Go MCP servers with Qdrant sidecar
**Confidence:** HIGH

## Summary

Deploying a Go MCP server with Qdrant on Fly.io requires understanding three deployment domains: (1) Fly.io's process group architecture for running multiple services within a single app, (2) Docker multi-stage builds for efficient Go application containerization, and (3) Fly.io's volume system for persistent Qdrant storage.

The standard approach uses **process groups** rather than separate apps, since both the MCP server and Qdrant need to run together and communicate over a private internal network. Fly.io provides native MCP deployment support via `fly mcp launch`, but the stdio transport that MCP uses requires special handling for remote deployment. The platform's internal networking (6PN) allows services to communicate via `.internal` DNS without additional configuration.

For this specific deployment, the key insight is that **Qdrant will run as a separate process group** using the official Docker image, while the **MCP server runs as the primary web process** that handles stdio communication. Both processes share the same private network and the Qdrant data persists via Fly.io volumes with automatic snapshots.

**Primary recommendation:** Use Fly.io process groups with multi-stage Docker builds, configure Qdrant as a sidecar process with persistent volume, and implement comprehensive health checks that verify both server availability and Qdrant connectivity.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Fly.io Platform | 2026 | Cloud deployment platform | Native MCP support, regional edge deployment, built-in TLS, volume persistence |
| Docker Multi-stage | BuildKit | Container image building | Industry standard for Go, reduces image size from 1GB+ to 20-30MB |
| Qdrant | latest (docker) | Vector database | Official Docker image with built-in persistence, gRPC support |
| Fly.io Process Groups | - | Multi-service orchestration | Built-in alternative to Docker Compose for running MCP server + Qdrant |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| distroless/static | latest | Minimal runtime base image | Production deployments requiring smallest attack surface |
| gcr.io/distroless/base-debian11 | latest | Minimal runtime with CA certs | When HTTPS client calls are needed |
| alpine | latest | Lightweight Linux | Alternative to distroless, supports shell access for debugging |
| golang:1.24 | 1.24 | Build stage base image | Compiling Go applications with full toolchain |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Process groups | Separate Fly.io apps | More complex networking setup, higher cost, unnecessary isolation |
| Fly.io volumes | External managed DB (Qdrant Cloud) | Higher cost, external dependency, network latency |
| Multi-stage Dockerfile | Single-stage build | 40x larger images (1GB vs 25MB), slower deployments, larger attack surface |
| fly mcp launch | Manual fly launch | Less automation, requires manual fly.toml configuration |

**Installation:**

```bash
# Install flyctl (Fly.io CLI)
curl -L https://fly.io/install.sh | sh

# Authenticate
fly auth login

# Deploy (from project root with fly.toml)
fly deploy

# Set secrets
fly secrets set OPENAI_API_KEY=sk-... GITHUB_TOKEN=ghp_...
```

## Architecture Patterns

### Recommended Project Structure

```
.
├── cmd/
│   ├── mcp-server/         # Main MCP server entrypoint
│   └── sync/               # Sync command for indexing
├── internal/               # Application packages
├── Dockerfile              # Multi-stage build definition
├── fly.toml                # Fly.io configuration
├── .env.example            # Environment variable template
└── prod.env                # Production variable documentation
```

### Pattern 1: Process Groups for MCP + Qdrant

**What:** Define multiple processes in fly.toml that run in separate Machines but share networking
**When to use:** When you need to run multiple services that communicate privately
**Example:**

```toml
# Source: https://fly.io/docs/launch/processes/
app = "eino-docs-mcp"
primary_region = "iad"

[processes]
  web = "/app/mcp-server"      # MCP server process
  qdrant = "qdrant"            # Qdrant sidecar process

[http_service]
  internal_port = 8080
  processes = ["web"]          # Only web process handles HTTP
  force_https = true
  auto_stop_machines = "off"   # Keep running for MCP clients
  auto_start_machines = true

[[vm]]
  memory = "256mb"
  cpus = 1
  processes = ["web"]

[[vm]]
  memory = "512mb"             # Qdrant needs more RAM for vectors
  cpus = 1
  processes = ["qdrant"]

[[mounts]]
  source = "qdrant_data"
  destination = "/qdrant/storage"
  processes = ["qdrant"]       # Only mount on Qdrant process
  initial_size = "1gb"
```

### Pattern 2: Multi-Stage Docker Build for Go

**What:** Use separate build and runtime stages to minimize final image size
**When to use:** Always for Go production deployments
**Example:**

```dockerfile
# Source: https://docs.docker.com/guides/golang/build-images/
# Build stage
FROM golang:1.24 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mcp-server ./cmd/mcp-server

# Runtime stage
FROM gcr.io/distroless/static-debian11
COPY --from=builder /build/mcp-server /app/mcp-server
ENTRYPOINT ["/app/mcp-server"]
```

### Pattern 3: Internal Networking with .internal DNS

**What:** Services communicate over Fly.io's private 6PN network using .internal DNS
**When to use:** When process groups or separate apps need to communicate privately
**Example:**

```go
// Source: https://fly.io/docs/networking/app-services/
// MCP server connecting to Qdrant over internal network
qdrantHost := os.Getenv("QDRANT_HOST")
if qdrantHost == "" {
    // Connect to Qdrant process via internal DNS
    qdrantHost = "qdrant.internal:6334"
}

client, err := qdrant.NewClient(&qdrant.Config{
    Host: qdrantHost,
    Port: 6334,
    UseTLS: false, // Internal network, no TLS needed
})
```

### Pattern 4: Health Checks for Multi-Service Apps

**What:** HTTP health check endpoint that verifies both server and dependencies
**When to use:** Always for production deployments, especially with databases
**Example:**

```go
// Source: Fly.io best practices
type HealthStatus struct {
    Status string `json:"status"`
    Qdrant string `json:"qdrant"`
}

func healthHandler(store storage.VectorStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        status := HealthStatus{Status: "healthy"}

        // Check Qdrant connectivity
        if err := store.Ping(r.Context()); err != nil {
            status.Qdrant = "disconnected"
            status.Status = "unhealthy"
            w.WriteHeader(http.StatusServiceUnavailable)
        } else {
            status.Qdrant = "connected"
            w.WriteHeader(http.StatusOK)
        }

        json.NewEncoder(w).Encode(status)
    }
}

// In fly.toml:
[[http_service.checks]]
  grace_period = "10s"        # Wait for startup
  interval = "15s"
  timeout = "5s"
  method = "GET"
  path = "/health"
```

### Pattern 5: Graceful Shutdown for Go Services

**What:** Handle SIGINT/SIGTERM signals to shut down cleanly
**When to use:** Always for production Go applications
**Example:**

```go
// Source: https://oneuptime.com/blog/post/2026-01-07-go-graceful-shutdown-kubernetes/view
func main() {
    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer stop()

    server := &http.Server{Addr: ":8080"}

    go func() {
        if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Fatalf("Server error: %v", err)
        }
    }()

    <-ctx.Done()
    log.Println("Shutting down gracefully...")

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
    defer cancel()

    if err := server.Shutdown(shutdownCtx); err != nil {
        log.Printf("Forced shutdown: %v", err)
    }
}
```

### Anti-Patterns to Avoid

- **Separate apps for MCP + Qdrant:** Process groups provide simpler networking and configuration management within a single app context
- **Buildpacks instead of Dockerfile:** Docker provides more control, faster builds, and doesn't break on upstream buildpack changes
- **Binding to 127.0.0.1:** Fly.io requires apps to bind to `0.0.0.0` or `[::]` to receive traffic from the proxy
- **No grace period on health checks:** Slow-starting apps will fail health checks if grace_period is too short
- **Protocol = "https" in fly.toml:** This requires the app to serve TLS directly; use the default HTTP with force_https for Fly.io TLS termination
- **Forgetting kill_timeout:** Default 5 seconds may not be enough for graceful shutdown; set to 20-30s

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| MCP stdio remote deployment | Custom WebSocket proxy | `fly mcp launch` | Fly.io provides built-in stdio wrapping and proxying for MCP servers |
| Service health monitoring | Custom health check daemon | Fly.io health checks | Built-in routing decisions, deployment blocking, automatic configuration |
| Secret management | Config files in image | `fly secrets` | Encrypted vault, never logged, injected at runtime, not baked into images |
| Log aggregation | Custom logging service | Fly.io logs + external export | Built-in log collection, can export to Datadog/Sentry/etc. |
| TLS certificate management | Manual cert generation | Fly.io TLS termination | Automatic certificate provisioning and renewal |
| Volume snapshots | Custom backup scripts | Fly.io volume snapshots | Automatic daily snapshots with configurable retention (1-60 days) |

**Key insight:** Fly.io provides platform-level solutions for deployment concerns that would otherwise require custom tooling. Use these built-in features rather than reimplementing them in application code.

## Common Pitfalls

### Pitfall 1: Port Binding Misconfiguration

**What goes wrong:** App binds to `127.0.0.1` or wrong port, Fly.io proxy can't route traffic, deployment succeeds but app is unreachable
**Why it happens:** Local development uses localhost, developers forget to change to `0.0.0.0` for containerized deployment
**How to avoid:**
- Always bind to `0.0.0.0` or `[::]` in production
- Match the port in code to `internal_port` in fly.toml
- Check logs for: "WARNING The app is not listening on the expected address"
**Warning signs:**
- Deployment succeeds but app returns 502 Bad Gateway
- `fly status` shows Machines running but health checks fail
- Logs show server starting but no incoming requests

### Pitfall 2: Health Check Grace Period Too Short

**What goes wrong:** App takes 15s to start (Qdrant connection, initialization), health checks begin after 5s, deployment fails with "unhealthy" status
**Why it happens:** Default grace_period is very short, Go apps with external dependencies need more time
**How to avoid:**
- Set `grace_period = "10s"` or higher for apps with database connections
- Measure actual startup time in development
- Add startup logging to identify slow initialization steps
**Warning signs:**
- Deployments fail with "health check never passed"
- Logs show successful startup after health check timeout
- Manual requests work but automated health checks fail

### Pitfall 3: Volume Not Mounted to Correct Process

**What goes wrong:** Volume configured in fly.toml but mounted to wrong process group, Qdrant data doesn't persist
**Why it happens:** Forgetting to specify `processes = ["qdrant"]` in [[mounts]] section
**How to avoid:**
- Always specify `processes` in [[mounts]] when using process groups
- Verify volume mounting with `fly ssh console -s` and check `/qdrant/storage`
- Test persistence by stopping/starting Machines and checking data survival
**Warning signs:**
- Qdrant collections disappear after deployment
- Volume shows 0 bytes used even though data was indexed
- Re-syncing required after every deployment

### Pitfall 4: Secrets vs Environment Variables Confusion

**What goes wrong:** Sensitive values stored in fly.toml [env] section, committed to git, exposed in public repositories
**Why it happens:** Unclear distinction between `fly secrets` (encrypted) and [env] (plaintext configuration)
**How to avoid:**
- Use `fly secrets set` for API keys, tokens, credentials
- Use [env] section only for non-sensitive configuration (PORT, LOG_LEVEL)
- Never commit .env files with real secrets
- Document expected secrets in .env.example
**Warning signs:**
- API keys visible in `fly config show`
- Git warnings about committed secrets
- Environment variables not available despite being in fly.toml

### Pitfall 5: Qdrant gRPC Performance Degradation with Large Payloads

**What goes wrong:** Using gRPC port 6334 expecting 2-3x performance improvement, but queries with large payloads (document text) are actually 3-4x slower than REST
**Why it happens:** Recent Qdrant versions (late 2025) have a gRPC performance bug with large string payloads
**How to avoid:**
- Test both REST (6333) and gRPC (6334) with realistic payloads
- For document search with full text returns, REST may be faster
- Monitor query latency in production and switch ports if needed
- gRPC is still faster for metadata-only queries
**Warning signs:**
- Slow query responses (200ms+ when expecting 50ms)
- Performance degrades with longer documents
- REST client outperforms gRPC client in benchmarks
**Source:** https://github.com/qdrant/qdrant/issues/7366

### Pitfall 6: Insufficient Memory for Qdrant Process

**What goes wrong:** Qdrant crashes with OOM errors, vector operations fail, index corruption
**Why it happens:** Default Fly.io Machine size (256MB) is too small for vector operations
**How to avoid:**
- Allocate minimum 512MB for Qdrant process
- Use separate [[vm]] configs for different process groups
- Monitor memory usage with `fly metrics` command
- Consider 1GB for larger document collections
**Warning signs:**
- Qdrant process repeatedly crashes
- Logs show "out of memory" or "killed"
- `fly status` shows Qdrant Machine in failed state

### Pitfall 7: Network Filesystem for Qdrant Volume

**What goes wrong:** Volume created on NFS/network storage, Qdrant fails compatibility check, data corruption
**Why it happens:** Qdrant requires POSIX-compliant block storage, Fly.io volumes are block storage but external mounts may not be
**How to avoid:**
- Use Fly.io volumes (built-in block storage)
- Never mount NFS, EFS, or object storage for Qdrant
- Qdrant v1.15.0+ performs runtime filesystem compatibility check
**Warning signs:**
- Qdrant fails to start with "incompatible filesystem" error
- Data corruption messages in logs
- Write operations fail intermittently

## Code Examples

Verified patterns from official sources:

### Dockerfile for Go MCP Server

```dockerfile
# Source: https://docs.docker.com/guides/golang/build-images/
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
```

### fly.toml Configuration

```toml
# Source: https://fly.io/docs/reference/configuration/
app = "eino-docs-mcp"
primary_region = "iad"

# Kill settings for graceful shutdown
kill_signal = "SIGINT"
kill_timeout = "20s"

[build]
  dockerfile = "Dockerfile"

[processes]
  web = "/app/mcp-server"
  qdrant = "/usr/bin/qdrant"

[env]
  PORT = "8080"
  QDRANT_HOST = "localhost"
  QDRANT_PORT = "6334"
  LOG_LEVEL = "info"

# HTTP service on web process
[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = "off"
  auto_start_machines = true
  min_machines_running = 1
  processes = ["web"]

  [http_service.concurrency]
    type = "requests"
    soft_limit = 20
    hard_limit = 25

  [[http_service.checks]]
    grace_period = "10s"
    interval = "15s"
    timeout = "5s"
    method = "GET"
    path = "/health"

# VM resources for web process
[[vm]]
  memory = "256mb"
  cpus = 1
  cpu_kind = "shared"
  processes = ["web"]

# VM resources for Qdrant (needs more RAM)
[[vm]]
  memory = "512mb"
  cpus = 1
  cpu_kind = "shared"
  processes = ["qdrant"]

# Persistent volume for Qdrant data
[[mounts]]
  source = "qdrant_data"
  destination = "/qdrant/storage"
  processes = ["qdrant"]
  initial_size = "1gb"
  snapshot_retention = 5
```

### Health Check Endpoint Implementation

```go
// Source: Fly.io production best practices
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

type HealthResponse struct {
    Status    string `json:"status"`
    Qdrant    string `json:"qdrant"`
    Timestamp string `json:"timestamp"`
}

func NewHealthHandler(store VectorStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
        defer cancel()

        response := HealthResponse{
            Status:    "healthy",
            Timestamp: time.Now().UTC().Format(time.RFC3339),
        }

        // Verify Qdrant connectivity
        if err := store.HealthCheck(ctx); err != nil {
            response.Status = "unhealthy"
            response.Qdrant = "disconnected"
            w.WriteHeader(http.StatusServiceUnavailable)
        } else {
            response.Qdrant = "connected"
            w.WriteHeader(http.StatusOK)
        }

        json.NewEncoder(w).Encode(response)
    }
}
```

### Environment Variable Loading

```go
// Source: Go best practices with Fly.io deployment
package config

import (
    "fmt"
    "os"
    "strconv"

    "github.com/joho/godotenv"
)

type Config struct {
    Port         int
    QdrantHost   string
    QdrantPort   int
    OpenAIKey    string
    GitHubToken  string
    LogLevel     string
}

func Load() (*Config, error) {
    // Load .env file if it exists (development)
    // Production uses fly secrets (already in environment)
    _ = godotenv.Load()

    cfg := &Config{
        Port:         getEnvInt("PORT", 8080),
        QdrantHost:   getEnv("QDRANT_HOST", "localhost"),
        QdrantPort:   getEnvInt("QDRANT_PORT", 6334),
        LogLevel:     getEnv("LOG_LEVEL", "info"),
    }

    // Required secrets
    cfg.OpenAIKey = os.Getenv("OPENAI_API_KEY")
    if cfg.OpenAIKey == "" {
        return nil, fmt.Errorf("OPENAI_API_KEY is required")
    }

    cfg.GitHubToken = os.Getenv("GITHUB_TOKEN")
    if cfg.GitHubToken == "" {
        return nil, fmt.Errorf("GITHUB_TOKEN is required")
    }

    return cfg, nil
}

func getEnv(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}

func getEnvInt(key string, fallback int) int {
    if value := os.Getenv(key); value != "" {
        if i, err := strconv.Atoi(value); err == nil {
            return i
        }
    }
    return fallback
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| SSE transport for MCP | stdio transport | Nov 2024 | SSE deprecated, stdio is now standard and most interoperable |
| Buildpacks for Fly.io | Dockerfile | 2024-2025 | Buildpacks unreliable due to upstream changes, Dockerfiles more stable |
| Single Machine deployment | Process groups | 2023-2024 | Better resource isolation, independent scaling, simpler than multiple apps |
| Volume auto-snapshots free | Volume snapshot billing | Jan 2026 | $0.08/GB/month for snapshots (first 10GB free) |
| Qdrant gRPC always faster | REST faster for large payloads | Late 2025 | gRPC performance bug with large strings, REST recommended for document search |

**Deprecated/outdated:**

- **SSE transport for MCP servers:** Deprecated in favor of stdio (most common) and Streaming HTTP (alternative)
- **fly apps create:** Replaced by `fly launch` for new applications
- **Separate apps for multi-service:** Process groups are now the recommended pattern within a single app
- **Free volume snapshots:** As of January 2026, snapshots cost $0.08/GB/month (first 10GB free)

## Open Questions

Things that couldn't be fully resolved:

1. **Optimal Qdrant connection pooling for MCP workload**
   - What we know: MCP servers are stateful and async, Qdrant supports connection pooling
   - What's unclear: Best pool size for low-latency queries with occasional bulk operations
   - Recommendation: Start with default pool size, monitor connection usage, adjust based on metrics

2. **fly mcp launch vs manual fly launch for this use case**
   - What we know: `fly mcp launch` automates stdio MCP deployment, but we have a custom Qdrant sidecar
   - What's unclear: Whether fly mcp launch supports process groups with custom services
   - Recommendation: Use standard `fly launch` for full control over process groups and volumes

3. **Snapshot retention vs re-indexing strategy**
   - What we know: Snapshots cost $0.08/GB/month, re-indexing from GitHub is free but time-consuming
   - What's unclear: Break-even point where snapshot cost > re-indexing cost for this corpus size
   - Recommendation: Start with default 5-day retention, measure actual snapshot size, adjust based on 1GB corpus

4. **Internal networking performance (6PN) vs localhost for Qdrant**
   - What we know: Process groups can communicate via .internal DNS, unclear if same host optimization applies
   - What's unclear: Whether Qdrant running in separate process can bind to localhost for same-host optimization
   - Recommendation: Use `QDRANT_HOST=localhost` since process groups share the same Machine/host

## Sources

### Primary (HIGH confidence)

- Fly.io Official Documentation (https://fly.io/docs/)
  - Languages & Frameworks: Go deployment guide
  - Volumes: Persistence, snapshots, pricing
  - Health Checks: Configuration and types
  - Process Groups: Multi-service architecture
  - App Configuration: fly.toml reference
  - Networking: Internal .internal DNS and 6PN
  - MCP: Model Context Protocol deployment
  - Troubleshooting: Common deployment issues
  - Production Checklist: Going to production guide

- Docker Official Documentation (https://docs.docker.com/)
  - Go Build Guide: Multi-stage builds
  - JSON File Logging: Docker logging drivers

- Qdrant Official Documentation (https://qdrant.tech/documentation/)
  - Installation: Docker deployment
  - Storage: Persistent storage requirements
  - Configuration: Ports and settings
  - API & SDKs: gRPC vs REST interfaces

- Go SDK for MCP (https://github.com/modelcontextprotocol/go-sdk)
  - Protocol documentation: Lifecycle and transports
  - Server features: Implementation guide
  - Troubleshooting: Debugging MCP applications

### Secondary (MEDIUM confidence)

- Fly.io Blog: "Launching MCP Servers on Fly.io" (Jan 2025)
- oneuptime.com: "How to Containerize Go Apps with Multi-Stage Dockerfiles" (Jan 2026)
- oneuptime.com: "How to Implement Graceful Shutdown in Go for Kubernetes" (Jan 2026)
- victoriametrics.com: "Graceful Shutdown in Go: Practical Patterns" (2025)
- CData Blog: "MCP Server Best Practices for 2026" (2026)
- Kuberns Blog: "What Is Fly.io? Complete Guide 2026" (2026)
- dash0.com: "JSON Logging: A Quick Guide for Engineers" (2025)

### Tertiary (LOW confidence - flagged for validation)

- GitHub Issue #7366: Qdrant gRPC performance bug with payloads (Oct 2025) - Real performance issue but specific to certain workloads
- Fly.io Community Forums: Process groups vs separate apps discussions - Community consensus, not official guidance
- Third-party MCP deployment tools (NakulRajan/mcp-fly-deployer) - Useful reference but not official

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official Fly.io and Docker documentation, widely used patterns
- Architecture patterns: HIGH - Direct from official Fly.io configuration reference and examples
- Pitfalls: MEDIUM-HIGH - Mix of official troubleshooting docs and recent community findings (gRPC issue)
- Code examples: HIGH - Extracted from official documentation with cited sources

**Research date:** 2026-01-25
**Valid until:** 2026-02-25 (30 days) - Fly.io is stable platform, configuration patterns unlikely to change

**Note:** The Qdrant gRPC performance issue (Pitfall 5) is based on a recent GitHub issue from late 2025 and should be tested in the actual environment during implementation.
