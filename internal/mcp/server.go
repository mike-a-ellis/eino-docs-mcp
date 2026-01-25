package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with dependencies.
type Server struct {
	server *mcp.Server
	// Storage and embedder will be injected in Plan 03-02
}

// Config holds server dependencies.
type Config struct {
	// Will add Storage and Embedder in Plan 03-02
}

// NewServer creates a configured MCP server with tools registered.
func NewServer(cfg *Config) *Server {
	impl := &mcp.Implementation{
		Name:    "eino-documentation-server",
		Version: "v0.1.0",
	}

	server := mcp.NewServer(impl, nil)

	// Register tools (stub handlers for now - will implement in Plan 03-02)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_docs",
		Description: "Search EINO documentation semantically. Returns metadata for matching documents.",
	}, searchDocsStub)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "fetch_doc",
		Description: "Retrieve a specific EINO document by path. Returns full markdown content.",
	}, fetchDocStub)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_docs",
		Description: "List all available EINO documentation paths.",
	}, listDocsStub)

	return &Server{server: server}
}

// Run starts the server with stdio transport (blocks until client disconnects).
func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// Stub handlers - return placeholder responses

func searchDocsStub(ctx context.Context, req *mcp.CallToolRequest, input SearchDocsInput) (*mcp.CallToolResult, SearchDocsOutput, error) {
	return nil, SearchDocsOutput{
		Results: []SearchResult{},
		Message: "Search not yet implemented",
	}, nil
}

func fetchDocStub(ctx context.Context, req *mcp.CallToolRequest, input FetchDocInput) (*mcp.CallToolResult, FetchDocOutput, error) {
	return nil, FetchDocOutput{
		Found: false,
	}, nil
}

func listDocsStub(ctx context.Context, req *mcp.CallToolRequest, input ListDocsInput) (*mcp.CallToolResult, ListDocsOutput, error) {
	return nil, ListDocsOutput{
		Paths: []string{},
		Count: 0,
	}, nil
}
