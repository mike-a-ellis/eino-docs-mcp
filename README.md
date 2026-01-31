# EINO Documentation MCP Server

![Go Version](https://img.shields.io/badge/Go-1.24-blue)

A Model Context Protocol (MCP) server that provides semantic search over [EINO framework](https://github.com/cloudwego/eino) documentation for AI agents. Enables Claude Code and other MCP clients to retrieve relevant documentation without manual copy-paste.

**Deployed version:** https://eino-docs-mcp.fly.dev

## Why This Exists

AI agents working with EINO need access to framework documentation. Instead of manually copying docs or relying on outdated training data, this server:

- Fetches documentation directly from GitHub
- Chunks and embeds content for semantic search
- Exposes search and retrieval via MCP tools
- Keeps track of index freshness against GitHub HEAD

## How It Works

```
GitHub Docs ──> Markdown Chunking ──> OpenAI Embeddings ──> Qdrant Vector DB
                                                                    │
                                                                    v
Claude Code <──── MCP Protocol <──── Search/Fetch Tools <──── Semantic Query
```

### Architecture Overview

1. **Sync Pipeline**: Fetches EINO docs from `cloudwego/cloudwego.github.io`, splits markdown into semantic chunks, generates embeddings via OpenAI, and stores in Qdrant
2. **MCP Server**: Exposes 4 tools over MCP protocol (stdio or HTTP modes)
3. **Vector Search**: Queries use embedding similarity to find relevant documentation chunks, then returns parent document metadata

### MCP Tools

| Tool | Description |
|------|-------------|
| `search_docs` | Semantic search across all documentation. Returns metadata for matching docs. |
| `fetch_doc` | Retrieve full markdown content by document path. |
| `list_docs` | List all available document paths. |
| `get_index_status` | Get index status including document counts, last sync time, and staleness indicator. |

## Quick Start

### Prerequisites

- Go 1.24+
- Docker and Docker Compose
- OpenAI API key

### Local Setup

1. Clone the repository and set up environment:

```bash
cp .env.example .env
# Edit .env and add your OPENAI_API_KEY
```

2. Start Qdrant:

```bash
docker-compose up -d
```

3. Build and run the sync tool to index documentation:

```bash
go build -o eino-sync ./cmd/sync
./eino-sync sync
```

4. Run the MCP server (stdio mode for local development):

```bash
go build -o mcp-server ./cmd/mcp-server
./mcp-server
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENAI_API_KEY` | Yes | - | OpenAI API key for generating embeddings |
| `QDRANT_HOST` | No | `localhost` | Qdrant server hostname |
| `QDRANT_PORT` | No | `6334` | Qdrant gRPC port |
| `GITHUB_TOKEN` | No | - | GitHub token for higher rate limits (60/hr without, 5000/hr with) |
| `PORT` | No | `8080` | HTTP server port |
| `SERVER_MODE` | No | `false` | Set to `true` for HTTP mode, `false` for stdio mode |
| `LOG_LEVEL` | No | `info` | Logging verbosity |

### Example .env File

```bash
# Required
OPENAI_API_KEY=sk-your-api-key-here

# Optional
QDRANT_HOST=localhost
QDRANT_PORT=6334
GITHUB_TOKEN=ghp_your-github-token-here
```

## Running Locally (Docker)

### 1. Start Qdrant

```bash
docker-compose up -d
```

This starts Qdrant with:
- REST API on port 6333
- gRPC on port 6334 (used by this server)
- Persistent storage in `qdrant_data` volume

### 2. Build the Binaries

```bash
go build -o mcp-server ./cmd/mcp-server
go build -o eino-sync ./cmd/sync
```

### 3. Index Documentation

Run the sync tool to fetch and index all EINO docs:

```bash
./eino-sync sync
```

Output:
```
Starting sync...

Connecting to Qdrant at localhost:6334...
Qdrant healthy

Clearing existing collection...
Collection cleared

Indexing documents from GitHub...

Sync complete!
  Documents: 42/42
  Chunks: 387
  Duration: 2m15s
  Commit: abc1234

Total time: 2m20s
```

### 4. Run the MCP Server

**Stdio mode** (for local Claude Code integration):
```bash
./mcp-server
```

**HTTP mode** (for remote access):
```bash
SERVER_MODE=true ./mcp-server
```

## Claude Code Integration

### Option 1: Stdio Mode (Local)

Add to your Claude Code MCP settings (`~/.claude/mcp_servers.json`):

```json
{
  "eino-docs": {
    "command": "/path/to/mcp-server",
    "env": {
      "QDRANT_HOST": "localhost",
      "QDRANT_PORT": "6334",
      "OPENAI_API_KEY": "sk-your-key"
    }
  }
}
```

### Option 2: HTTP Mode (Remote)

For the deployed version at Fly.io:

```json
{
  "eino-docs": {
    "type": "http",
    "url": "https://eino-docs-mcp.fly.dev/mcp"
  }
}
```

### Available Tools After Integration

Once configured, Claude Code gains access to:

- **search_docs**: "Search EINO for how to create a ChatModel"
- **fetch_doc**: "Get the full content of getting-started/quickstart.md"
- **list_docs**: "What EINO documentation is available?"
- **get_index_status**: "Is the EINO docs index up to date?"

## Deployment to Fly.io

### Prerequisites

- [Fly CLI](https://fly.io/docs/hands-on/install-flyctl/) installed
- Fly.io account

### Deploy Steps

1. Create the app and volume:

```bash
fly apps create eino-docs-mcp
fly volumes create qdrant_data --region iad --size 1
```

2. Set secrets:

```bash
fly secrets set OPENAI_API_KEY=sk-your-key
fly secrets set GITHUB_TOKEN=ghp_your-token  # Optional but recommended
```

3. Deploy:

```bash
fly deploy
```

4. Trigger initial sync (via SSH or scheduled job):

```bash
fly ssh console
/app/eino-sync sync
```

### Configuration

The `fly.toml` configures:

- **Region**: `iad` (US East)
- **VM**: 512MB RAM, 1 shared CPU
- **Persistent volume**: Mounted at `/qdrant/storage`
- **Health checks**: GET `/health` every 15s
- **Auto-scaling**: Minimum 1 machine, no auto-stop

### Architecture on Fly.io

The Dockerfile runs both Qdrant and the MCP server in a single container:

1. Debian-slim base with Qdrant binary installed
2. Supervisor script starts Qdrant, waits for ready, starts MCP server
3. Persistent volume ensures index survives restarts

## Libraries Used

| Library | Purpose |
|---------|---------|
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | MCP protocol implementation |
| [qdrant/go-client](https://github.com/qdrant/go-client) | Qdrant vector database client |
| [openai/openai-go](https://github.com/openai/openai-go) | OpenAI API for embeddings |
| [google/go-github](https://github.com/google/go-github) | GitHub API for fetching docs |
| [yuin/goldmark](https://github.com/yuin/goldmark) | Markdown parsing |
| [spf13/cobra](https://github.com/spf13/cobra) | CLI framework |

## API Reference

### search_docs

Semantic search across EINO documentation. Returns metadata for matching documents (not full content).

**Input:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | - | Semantic search query |
| `max_results` | int | No | 5 | Maximum documents to return (1-20) |
| `min_score` | float | No | 0.4 | Minimum relevance threshold (0-1) |

**Output:**

```json
{
  "results": [
    {
      "path": "core-modules/model/chatmodel.md",
      "score": 0.89,
      "summary": "ChatModel interface for conversational AI...",
      "entities": ["NewChatModel", "Generate", "Stream"],
      "updated_at": "2025-01-15T10:30:00Z"
    }
  ]
}
```

### fetch_doc

Retrieve full markdown content of a specific document.

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Document path (e.g., `getting-started/quickstart.md`) |

**Output:**

```json
{
  "content": "<!-- Source: getting-started/quickstart.md -->\n\n# Quick Start...",
  "path": "getting-started/quickstart.md",
  "summary": "Getting started guide for EINO framework...",
  "updated_at": "2025-01-15T10:30:00Z",
  "found": true
}
```

### list_docs

List all available document paths in the index.

**Input:** None

**Output:**

```json
{
  "paths": [
    "getting-started/quickstart.md",
    "core-modules/model/chatmodel.md",
    "core-modules/flow/overview.md"
  ],
  "count": 42
}
```

### get_index_status

Get comprehensive index status including staleness information.

**Input:** None

**Output:**

```json
{
  "total_docs": 42,
  "total_chunks": 387,
  "indexed_paths": ["getting-started/quickstart.md", "..."],
  "last_sync_time": "2025-01-15T10:30:00Z",
  "source_commit": "abc1234def5678",
  "commits_behind": 3,
  "stale_warning": ""
}
```

When the index is >20 commits behind GitHub HEAD, `stale_warning` contains a message suggesting resync.

## Project Structure

```
.
├── cmd/
│   ├── mcp-server/          # MCP server entry point
│   │   └── main.go          # Stdio/HTTP mode switching
│   └── sync/                # Sync CLI tool
│       └── main.go          # Cobra CLI for indexing
├── internal/
│   ├── embedding/           # OpenAI embeddings
│   │   ├── client.go        # OpenAI API client
│   │   └── embedder.go      # Batch embedding generation
│   ├── github/              # GitHub integration
│   │   ├── client.go        # GitHub API client
│   │   └── fetcher.go       # Documentation fetcher
│   ├── indexer/             # Indexing pipeline
│   │   └── pipeline.go      # Orchestrates fetch->chunk->embed->store
│   ├── markdown/            # Markdown processing
│   │   └── chunker.go       # Semantic chunking
│   ├── mcp/                 # MCP server
│   │   ├── handlers.go      # Tool implementations
│   │   ├── health.go        # Health check endpoint
│   │   ├── server.go        # Server setup and tool registration
│   │   ├── transport.go     # HTTP transport wrapper
│   │   └── types.go         # Input/output types
│   ├── metadata/            # Metadata generation
│   │   └── generator.go     # LLM-powered summaries
│   └── storage/             # Vector storage
│       ├── models.go        # Document/chunk models
│       └── qdrant.go        # Qdrant operations
├── Dockerfile               # Multi-stage build
├── docker-compose.yml       # Local Qdrant setup
├── fly.toml                 # Fly.io deployment config
├── supervisor.sh            # Process supervisor for Fly.io
├── go.mod
└── go.sum
```

## Development

### Running Tests

```bash
go test ./...
```

### Building Binaries

```bash
# MCP server
go build -o mcp-server ./cmd/mcp-server

# Sync tool
go build -o eino-sync ./cmd/sync
```

### Code Organization

- **cmd/**: Entry points only, minimal logic
- **internal/**: All business logic, not importable by external packages
- **internal/mcp/**: MCP protocol handling and tool implementations
- **internal/storage/**: Data layer abstraction (currently Qdrant)
- **internal/indexer/**: Orchestrates the full indexing pipeline

### Adding New Tools

1. Define input/output types in `internal/mcp/types.go`
2. Create handler in `internal/mcp/handlers.go`
3. Register tool in `internal/mcp/server.go`

## Troubleshooting

### Qdrant Connection Failed

```
failed to connect to Qdrant: ...
```

**Solution:** Ensure Qdrant is running:
```bash
docker-compose up -d
docker-compose logs qdrant
```

### OpenAI Rate Limits

```
429 Too Many Requests
```

**Solution:** The sync tool includes exponential backoff. For large indexes, consider:
- Using a higher-tier OpenAI plan
- Adding delays between batches

### GitHub Rate Limits

```
403 rate limit exceeded
```

**Solution:** Set `GITHUB_TOKEN` environment variable:
- Without token: 60 requests/hour
- With token: 5000 requests/hour

### Index Staleness Warning

```json
{"stale_warning": "Index is 25 commits behind GitHub HEAD. Consider resyncing."}
```

**Solution:** Run sync to update the index:
```bash
./eino-sync sync
```

### Health Check Failing on Fly.io

```
Health check on port 8080 has failed
```

**Solution:** Check logs for startup errors:
```bash
fly logs
```

Common causes:
- Missing `OPENAI_API_KEY` secret
- Qdrant not starting (check volume mount)
- Port mismatch (should be 8080)

## License

MIT License

---

Built for AI agents that need EINO documentation access.
