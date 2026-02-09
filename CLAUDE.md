# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go-based MCP (Model Context Protocol) server providing semantic search over EINO framework documentation. It fetches docs from GitHub, chunks and embeds them into a Qdrant vector database, and exposes search/retrieval tools via MCP for use by Claude Code and other AI agents.

**Module:** `github.com/mike-a-ellis/eino-docs-mcp`
**Go version:** 1.24
**Deployed at:** https://eino-docs-mcp.fly.dev/mcp

## Build & Run Commands

```bash
# Build binaries
go build -o mcp-server ./cmd/mcp-server
go build -o eino-sync ./cmd/sync

# Run tests (unit tests only — no external deps needed)
go test ./...

# Run a single test
go test -run TestChunkDocument_BasicHeaders ./internal/markdown

# Run integration tests (requires running Qdrant + OPENAI_API_KEY)
go test -tags=integration ./internal/storage
go test -tags=integration ./internal/indexer

# Start local Qdrant
docker-compose up -d

# Index documentation
./eino-sync sync

# Run MCP server (stdio mode for local dev)
./mcp-server

# Run MCP server (HTTP mode for remote/Fly.io)
SERVER_MODE=true ./mcp-server

# Deploy to Fly.io
fly deploy
```

## Architecture

```
GitHub (cloudwego/cloudwego.github.io)
    → Fetch docs from content/en/docs/eino/
    → Chunk markdown at H1/H2 boundaries (goldmark parser)
    → Generate embeddings (OpenAI text-embedding-3-small, 1536-dim)
    → Generate metadata summaries (GPT-4o)
    → Store in Qdrant (cosine similarity)
    → Serve via MCP (4 tools: search_docs, fetch_doc, list_docs, get_index_status)
```

### Two binaries

- **`cmd/mcp-server/main.go`** — Dual-mode MCP server. `SERVER_MODE=true` for HTTP (`/mcp` + `/health` endpoints), `false` for stdio. Both modes start an HTTP health server.
- **`cmd/sync/main.go`** — Cobra CLI with `sync` command. Fetches all docs from GitHub, clears the Qdrant collection, and rebuilds the full index.

### Internal packages

| Package | Purpose |
|---------|---------|
| `internal/mcp/` | MCP server setup, tool registration (`server.go`), handler implementations (`handlers.go`), I/O types (`types.go`), health endpoint (`health.go`), HTTP transport (`transport.go`) |
| `internal/storage/` | Qdrant gRPC client. Documents (full content, no embeddings) and chunks (sections with embeddings) stored in a single `documents` collection |
| `internal/indexer/` | Pipeline orchestration: fetch → chunk → embed → generate metadata → store |
| `internal/embedding/` | OpenAI client wrapper with batch embedding (default 500/batch) and exponential backoff on 429s |
| `internal/github/` | GitHub API client with rate limit handling. Recursively fetches `.md` files, base64-decodes content |
| `internal/markdown/` | Semantic chunking at H1/H2 boundaries. Preserves header hierarchy as context prefix (e.g. `"# Title > ## Section"`) |
| `internal/metadata/` | GPT-4o-powered summary and entity extraction. Truncates at 16k tokens |

### Search flow (in `handlers.go`)

1. Embed query via OpenAI
2. Search chunks in Qdrant (request 3x `max_results` for dedup headroom)
3. Filter by `min_score` (default 0.4)
4. Deduplicate by parent document (keep highest score per doc)
5. Fetch parent document metadata
6. Return up to `max_results` (default 5, max 20)

### Adding a new MCP tool

1. Define input/output structs in `internal/mcp/types.go`
2. Create `makeXxxHandler()` in `internal/mcp/handlers.go`
3. Register with `mcp.AddTool()` in `internal/mcp/server.go`

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENAI_API_KEY` | Yes | — | For embeddings and metadata generation |
| `QDRANT_HOST` | No | `localhost` | Qdrant gRPC host |
| `QDRANT_PORT` | No | `6334` | Qdrant gRPC port |
| `GITHUB_TOKEN` | No | — | GitHub API token (60 req/hr without, 5000/hr with) |
| `SERVER_MODE` | No | `false` | `true` for HTTP mode, `false` for stdio |
| `PORT` | No | `8080` | HTTP server port |

## Deployment

Fly.io single-container deployment running both Qdrant and the MCP server:
- `supervisor.sh` starts Qdrant, waits for gRPC port 6334, then starts MCP server
- Qdrant binary is copied from `qdrant/qdrant:v1.12.6` Docker image in multi-stage build (Fly.io's Depot builder blocks GitHub releases downloads)
- Persistent volume mounted at `/qdrant/storage`
- Runtime requires `libunwind8` package for Qdrant binary
- After deploy, SSH in to run initial sync: `fly ssh console` → `/app/eino-sync sync`

## Testing

- **Unit tests** (`internal/markdown/chunker_test.go`): Standard Go tests, no external deps
- **Integration tests** (`internal/storage/qdrant_test.go`, `internal/indexer/pipeline_test.go`): Gated behind `//go:build integration` tag, require running Qdrant and `OPENAI_API_KEY`
- Storage tests use `testify/assert` and `testify/require`; markdown tests use stdlib only
