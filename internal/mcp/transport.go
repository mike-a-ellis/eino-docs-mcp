package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPHandlerOptions configures the HTTP transport behavior.
type HTTPHandlerOptions struct {
	// Stateless disables session management. Use for simple tool servers
	// that don't need server-to-client requests. Default: false (stateful).
	Stateless bool
}

// NewHTTPHandler creates an HTTP handler for the MCP server using Streamable HTTP transport.
// The handler can be mounted on any http.ServeMux path (e.g., "/mcp").
//
// Example:
//
//	mux := http.NewServeMux()
//	mux.Handle("/mcp", mcpserver.NewHTTPHandler(server, nil))
//	mux.HandleFunc("/health", healthHandler)
//	http.ListenAndServe(":8080", mux)
func NewHTTPHandler(server *Server, opts *HTTPHandlerOptions) http.Handler {
	if opts == nil {
		opts = &HTTPHandlerOptions{}
	}

	sdkOpts := &mcp.StreamableHTTPOptions{
		Stateless: opts.Stateless,
	}

	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server.MCPServer()
	}, sdkOpts)
}
