package mcp

import (
	"context"

	"github.com/bull/eino-mcp-server/internal/embedding"
	ghclient "github.com/bull/eino-mcp-server/internal/github"
	"github.com/bull/eino-mcp-server/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with dependencies.
type Server struct {
	server   *mcp.Server
	storage  *storage.QdrantStorage
	embedder *embedding.Embedder
	github   *ghclient.Client
}

// Config holds server dependencies.
type Config struct {
	Storage  *storage.QdrantStorage
	Embedder *embedding.Embedder
	GitHub   *ghclient.Client
}

// NewServer creates a configured MCP server with tools registered.
func NewServer(cfg *Config) *Server {
	impl := &mcp.Implementation{
		Name:    "eino-user-manual-server",
		Version: "v0.1.0",
	}

	server := mcp.NewServer(impl, nil)

	// Register tools with real handlers
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_docs",
		Description: "Search Eino User Manual documentation semantically. Returns metadata for matching documents. Use fetch_doc to get full content.",
	}, makeSearchHandler(cfg.Storage, cfg.Embedder))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "fetch_doc",
		Description: "Retrieve a specific Eino User Manual document by path. Returns full markdown content.",
	}, makeFetchHandler(cfg.Storage))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_docs",
		Description: "List all available Eino User Manual documentation paths.",
	}, makeListHandler(cfg.Storage))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_index_status",
		Description: "Get the current status of the Eino User Manual documentation index including document counts, last sync time, and staleness indicator.",
	}, makeStatusHandler(cfg.Storage, cfg.GitHub))

	return &Server{
		server:   server,
		storage:  cfg.Storage,
		embedder: cfg.Embedder,
		github:   cfg.GitHub,
	}
}

// Run starts the server with stdio transport (blocks until client disconnects).
func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// MCPServer returns the underlying MCP server instance.
// Used by transport handlers that need to wrap the server.
func (s *Server) MCPServer() *mcp.Server {
	return s.server
}
