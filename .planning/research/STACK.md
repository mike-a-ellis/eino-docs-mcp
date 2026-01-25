# Technology Stack: MCP Server for EINO Documentation

**Project:** MCP Server (Model Context Protocol) serving EINO documentation to AI agents
**Target Deployment:** Fly.io with persistent storage
**Language:** Go
**Researched:** 2026-01-25
**Overall Confidence:** HIGH

---

## Executive Summary

This stack uses the official MCP Go SDK for server implementation, official client libraries for all external services (Qdrant, OpenAI, GitHub), and follows Fly.io best practices for deployment with persistent volumes. The architecture requires running Qdrant as a separate service (sidecar or external) since no embedded Go mode exists.

**Key Decision:** Qdrant must run as a separate Docker container/service, not embedded within the Go application.

---

## Recommended Stack

### Core MCP Framework

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/modelcontextprotocol/go-sdk` | v1.2.0 | MCP server implementation | **Official SDK** maintained by Anthropic + Google. Supports MCP spec 2025-11-25. Provides `StdioTransport` for stdio communication, structured tool/resource APIs with JSON schema support. |

**Installation:**
```bash
go get github.com/modelcontextprotocol/go-sdk
```

**Rationale:** The official SDK is now stable (v1.2.0 released Dec 2025) and supports all current MCP spec versions. Third-party alternatives (mcp-go, mcp-golang) were inspirations but lack official backing.

**Basic Server Example:**
```go
package main

import (
	"context"
	"log"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name: "eino-docs",
		Version: "v1.0.0",
	}, nil)

	// Add tools and resources here

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
```

---

### Vector Database

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/qdrant/go-client` | v1.16.0 | Vector search client | **Official Qdrant Go client.** Uses gRPC for performance. Well-documented with 79+ dependent packages. |
| Qdrant Server | latest (Docker) | Vector storage engine | Industry-standard vector DB. **No Go embedded mode exists** - must run as separate service. |

**Installation:**
```bash
go get -u github.com/qdrant/go-client
```

**CRITICAL ARCHITECTURAL NOTE:**

Unlike Python's `QdrantClient(":memory:")` or `QdrantClient(path="./db")`, **the Go client does NOT support embedded mode**. You must run Qdrant as a separate service.

**Deployment Options:**

1. **Fly.io Multi-Container (Recommended):** Run Qdrant as a sidecar container
2. **External Qdrant Cloud:** Use managed Qdrant (adds latency)
3. **Docker Compose for Local Dev:** Run both containers locally

**Docker Compose (Local Development):**
```yaml
version: '3.8'
services:
  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"
    volumes:
      - ./qdrant_storage:/qdrant/storage

  mcp-server:
    build: .
    depends_on:
      - qdrant
    environment:
      - QDRANT_HOST=qdrant
      - QDRANT_PORT=6334
```

**Go Client Configuration:**
```go
import "github.com/qdrant/go-client/qdrant"

client, err := qdrant.NewClient(&qdrant.Config{
	Host: os.Getenv("QDRANT_HOST"), // "localhost" or "qdrant"
	Port: 6334,
})
if err != nil {
	log.Fatal(err)
}
defer client.Close()
```

**Persistence Configuration:**

Qdrant persists data automatically when using volume mounts. For on-disk collection storage:

```go
// When creating a collection, enable on-disk storage
vectorParams := &qdrant.VectorParams{
	Size:     1536, // OpenAI text-embedding-3-small dimension
	Distance: qdrant.Distance_Cosine,
	OnDisk:   qdrant.PtrOf(true), // Enable disk persistence
}
```

**Why NOT alternatives:**
- **Weaviate:** More complex, heavier footprint
- **Milvus:** Requires more infrastructure (etcd, MinIO)
- **Chroma:** Python-first, Go support immature

---

### Embeddings API

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/openai/openai-go/v3` | v3.16.0 | OpenAI embeddings | **Official OpenAI library.** Still beta but officially supported. Better than community alternative for long-term stability. |

**Installation:**
```bash
go get -u 'github.com/openai/openai-go/v3@v3.16.0'
```

**Basic Embeddings Usage:**
```go
import (
	"context"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func createEmbedding(text string) ([]float32, error) {
	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	embedding, err := client.Embeddings.New(context.TODO(), openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInput{
			String: text,
		},
		Model: openai.EmbeddingModelTextEmbedding3Small, // 1536 dimensions
	})

	if err != nil {
		return nil, err
	}

	return embedding.Data[0].Embedding, nil
}
```

**Recommended Model:**
- **`text-embedding-3-small`**: 1536 dimensions, $0.02/1M tokens, good quality-to-cost ratio
- **`text-embedding-3-large`**: 3072 dimensions, higher quality, 5x more expensive

**Best Practices:**
```go
// Always use environment variables
apiKey := os.Getenv("OPENAI_API_KEY")
if apiKey == "" {
	log.Fatal("OPENAI_API_KEY environment variable required")
}

// Use context with timeout for production
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Handle rate limiting errors
if err != nil {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatusCode {
		case 401:
			log.Fatal("Invalid API key")
		case 429:
			log.Println("Rate limited, backing off...")
		case 500:
			log.Println("OpenAI server error")
		}
	}
}
```

**Why NOT `sashabaranov/go-openai`:**

While `sashabaranov/go-openai` is more mature (2,822 dependent packages vs newer official SDK), the **official library** is the strategic choice:
- Long-term support guaranteed by OpenAI
- Will track API changes faster
- Beta status is acceptable for v3.16.0 (released recently, stable)

**Confidence Level:** MEDIUM - Official SDK is newer but backed by OpenAI. Monitor for breaking changes.

---

### GitHub Integration

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/google/go-github/v81` | v81 | Fetch EINO docs from GitHub | **De facto standard** Go GitHub client. Maintained by Google, 11k+ stars, comprehensive API coverage. |

**Installation:**
```bash
go get github.com/google/go-github/v81
```

**Basic File Fetching:**
```go
import (
	"context"
	"github.com/google/go-github/v81/github"
)

func fetchFile(owner, repo, path string) (string, error) {
	client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))

	fileContent, _, _, err := client.Repositories.GetContents(
		context.Background(),
		owner,
		repo,
		path,
		nil, // Optional: &github.RepositoryContentGetOptions{Ref: "main"}
	)

	if err != nil {
		return "", err
	}

	content, err := fileContent.GetContent()
	return content, err
}
```

**Fetching Directory Contents:**
```go
func fetchDirectory(owner, repo, path string) ([]*github.RepositoryContent, error) {
	client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))

	_, dirContents, _, err := client.Repositories.GetContents(
		context.Background(),
		owner,
		repo,
		path,
		nil,
	)

	return dirContents, err
}
```

**Authentication:**
```go
// Unauthenticated: 60 requests/hour
client := github.NewClient(nil)

// Authenticated: 5,000 requests/hour (recommended)
client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
```

**Best Practice:** Always authenticate for production. Generate a Personal Access Token with `public_repo` scope.

**Why NOT alternatives:**
- `octokit/go-octokit`: Less maintained, smaller community
- Direct API calls: Reinventing the wheel, no rate limit handling

---

### Deployment Infrastructure

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Fly.io Volumes | N/A | Persistent Qdrant storage | **Native Fly.io primitive** for block storage. Encryption at rest, automatic snapshots (5-day retention). |
| Docker Multi-Stage | N/A | Optimized container images | Reduces final image size, improves security, faster deploys. |

---

## Deployment Configuration

### Dockerfile (Multi-Stage Build)

```dockerfile
# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mcp-server .

# Stage 2: Runtime
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/mcp-server .

# Create data directory for logs/temp files (not Qdrant data)
RUN mkdir -p /data

EXPOSE 8080

CMD ["./mcp-server"]
```

**Why Alpine:**
- Minimal attack surface (5MB base vs 120MB+ for Debian)
- Sufficient for Go binaries (no glibc dependencies with `CGO_ENABLED=0`)
- Standard for Go deployments on Fly.io

---

### fly.toml Configuration

```toml
app = "eino-mcp-server"
primary_region = "sjc"

[build]
  dockerfile = "Dockerfile"

[env]
  QDRANT_HOST = "localhost"
  QDRANT_PORT = "6334"

# Persistent volume for Qdrant data
[mounts]
  source = "qdrant_data"
  destination = "/qdrant/storage"
  auto_extend_size_threshold = 80

[[services]]
  internal_port = 8080
  protocol = "tcp"

  [[services.ports]]
    port = 80
    handlers = ["http"]

  [[services.ports]]
    port = 443
    handlers = ["tls", "http"]

[[services.tcp_checks]]
  interval = "15s"
  timeout = "2s"
  grace_period = "5s"
```

---

### Volume Creation Commands

```bash
# Create volume (10GB, expandable)
fly volumes create qdrant_data --size 10 --region sjc

# For multiple machines (HA), create one volume per machine
fly volumes create qdrant_data --size 10 --region sjc --count 2

# Check volumes
fly volumes list

# Snapshot management (automatic, but can trigger manually)
fly volumes snapshots list qdrant_data
```

**Important Constraints:**
- One volume per machine (1:1 mapping)
- No automatic replication between volumes
- Application must handle multi-volume sync if needed
- Minimum 2 machines + volumes recommended for production

---

### Fly.io Multi-Process Configuration

To run both MCP server and Qdrant in the same Fly app:

**fly.toml:**
```toml
[processes]
  mcp = "./mcp-server"
  qdrant = "/usr/local/bin/qdrant"

[mounts]
  source = "qdrant_data"
  destination = "/qdrant/storage"
  processes = ["qdrant"]  # Only mount to Qdrant process
```

**Alternative: Separate Apps (Cleaner)**

Run Qdrant as a separate Fly app with private networking:

```bash
# Deploy Qdrant app
fly launch --image qdrant/qdrant --name eino-qdrant --internal-port 6334

# Create volume for Qdrant
fly volumes create qdrant_storage --app eino-qdrant --size 10

# Connect MCP server to Qdrant via private IPv6
# In MCP server fly.toml:
[env]
  QDRANT_HOST = "eino-qdrant.internal"
  QDRANT_PORT = "6334"
```

---

## Supporting Libraries

### Recommended

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/joho/godotenv` | v1.5.1 | Load .env files | Local development environment variable management |
| `github.com/rs/zerolog` | v1.33.0 | Structured logging | Production logging with JSON output for Fly.io monitoring |
| `github.com/kelseyhightower/envconfig` | v1.4.0 | Environment config parsing | Type-safe environment variable loading with validation |

**Example envconfig usage:**
```go
type Config struct {
	QdrantHost    string `envconfig:"QDRANT_HOST" required:"true"`
	QdrantPort    int    `envconfig:"QDRANT_PORT" default:"6334"`
	OpenAIKey     string `envconfig:"OPENAI_API_KEY" required:"true"`
	GitHubToken   string `envconfig:"GITHUB_TOKEN" required:"true"`
	GitHubOwner   string `envconfig:"GITHUB_OWNER" default:"cloudwego"`
	GitHubRepo    string `envconfig:"GITHUB_REPO" default:"eino"`
}

func loadConfig() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	return &cfg, err
}
```

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| MCP SDK | `modelcontextprotocol/go-sdk` | `mark3labs/mcp-go` | Official SDK is now stable (v1.2.0), backed by Anthropic + Google |
| Vector DB | Qdrant | Weaviate | Qdrant lighter, simpler API, better Go client |
| Vector DB | Qdrant | Milvus | Milvus requires etcd + MinIO, over-engineered for this use case |
| Vector DB | Qdrant | Chroma | Python-first, Go client immature |
| OpenAI Client | `openai/openai-go` | `sashabaranov/go-openai` | Official support preferred despite beta status |
| GitHub Client | `google/go-github` | `octokit/go-octokit` | go-github is industry standard with better maintenance |
| Deployment | Fly.io | Railway | Fly.io has better volume management and multi-region support |
| Deployment | Fly.io | Render | Fly.io more flexible for sidecar/multi-process patterns |

---

## Installation Script

```bash
#!/bin/bash
# install-dependencies.sh

# Core dependencies
go get github.com/modelcontextprotocol/go-sdk@v1.2.0
go get github.com/qdrant/go-client@v1.16.0
go get github.com/openai/openai-go/v3@v3.16.0
go get github.com/google/go-github/v81

# Supporting libraries
go get github.com/joho/godotenv@v1.5.1
go get github.com/rs/zerolog@v1.33.0
go get github.com/kelseyhightower/envconfig@v1.4.0

# Development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "Dependencies installed successfully"
```

---

## Environment Variables Reference

```bash
# Required
export OPENAI_API_KEY="sk-..."
export GITHUB_TOKEN="ghp_..."

# Qdrant Configuration
export QDRANT_HOST="localhost"  # or "eino-qdrant.internal" on Fly.io
export QDRANT_PORT="6334"

# GitHub Source
export GITHUB_OWNER="cloudwego"
export GITHUB_REPO="eino"

# Optional
export LOG_LEVEL="info"
export MCP_SERVER_PORT="8080"
```

---

## Confidence Assessment

| Component | Confidence | Notes |
|-----------|------------|-------|
| MCP Go SDK | **HIGH** | Official SDK, v1.2.0 stable release, excellent docs |
| Qdrant Client | **HIGH** | Official client, well-documented, 79+ dependents |
| OpenAI Client | **MEDIUM** | Official but beta; monitor for breaking changes |
| GitHub Client | **HIGH** | Industry standard, maintained by Google |
| Fly.io Deployment | **HIGH** | Well-documented, proven pattern for Go + volumes |
| Qdrant Architecture | **HIGH** | Confirmed no embedded mode; sidecar pattern required |

---

## Sources

### MCP Go SDK
- [Official Repository](https://github.com/modelcontextprotocol/go-sdk)
- [Go Package Documentation](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp)

### Qdrant
- [Go Client Repository](https://github.com/qdrant/go-client)
- [Go Package Documentation](https://pkg.go.dev/github.com/qdrant/go-client/qdrant)
- [Qdrant Installation Guide](https://qdrant.tech/documentation/guides/installation/)
- [Qdrant Docker Hub](https://hub.docker.com/r/qdrant/qdrant)
- [Qdrant Storage Documentation](https://qdrant.tech/documentation/concepts/storage/)

### OpenAI
- [Official Go Library](https://github.com/openai/openai-go)
- [OpenAI API Documentation](https://platform.openai.com/docs/libraries)
- [Go Package Documentation](https://pkg.go.dev/github.com/openai/openai-go)

### GitHub
- [google/go-github Repository](https://github.com/google/go-github)
- [Go Package Documentation](https://pkg.go.dev/github.com/google/go-github/github)

### Fly.io
- [Fly Volumes Overview](https://fly.io/docs/volumes/overview/)
- [Add Volume Storage to a Fly Launch App](https://fly.io/docs/launch/volume-storage/)
- [Working with Docker on Fly.io](https://fly.io/docs/blueprints/working-with-docker/)
- [Multi-stage Builds](https://fly.io/docs/python/the-basics/multi-stage-builds/)

---

## Next Steps

1. **Initialize Go Module:**
   ```bash
   go mod init github.com/yourorg/eino-mcp-server
   ```

2. **Run Installation Script:**
   ```bash
   bash install-dependencies.sh
   ```

3. **Set Up Local Development:**
   ```bash
   # Create .env file
   cp .env.example .env

   # Start Qdrant locally
   docker-compose up -d qdrant
   ```

4. **Implement Core MCP Server:**
   - Create server with `mcp.NewServer()`
   - Add tools for documentation search
   - Add resources for document access
   - Implement embedding pipeline

5. **Deploy to Fly.io:**
   ```bash
   fly launch
   fly volumes create qdrant_data --size 10
   fly deploy
   ```
