package mcp

import (
	"context"

	"github.com/bull/eino-mcp-server/internal/embedding"
	"github.com/bull/eino-mcp-server/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with dependencies.
type Server struct {
	server   *mcp.Server
	storage  *storage.QdrantStorage
	embedder *embedding.Embedder
}

// Config holds server dependencies.
type Config struct {
	Storage  *storage.QdrantStorage
	Embedder *embedding.Embedder
}

// NewServer creates a configured MCP server with tools registered.
func NewServer(cfg *Config) *Server {
	impl := &mcp.Implementation{
		Name:    "eino-documentation-server",
		Version: "v0.1.0",
	}

	server := mcp.NewServer(impl, nil)

	// Register tools with real handlers
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_docs",
		Description: "Search EINO documentation semantically. Returns metadata for matching documents. Use fetch_doc to get full content.",
	}, makeSearchHandler(cfg.Storage, cfg.Embedder))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "fetch_doc",
		Description: "Retrieve a specific EINO document by path. Returns full markdown content.",
	}, makeFetchHandler(cfg.Storage))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_docs",
		Description: "List all available EINO documentation paths.",
	}, makeListHandler(cfg.Storage))

	return &Server{
		server:   server,
		storage:  cfg.Storage,
		embedder: cfg.Embedder,
	}
}

// Run starts the server with stdio transport (blocks until client disconnects).
func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}
