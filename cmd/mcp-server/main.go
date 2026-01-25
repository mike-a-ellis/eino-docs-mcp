// Package main provides the MCP server entry point for EINO documentation.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	mcpserver "github.com/bull/eino-mcp-server/internal/mcp"
)

func main() {
	// Create context that cancels on SIGTERM/SIGINT
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Create server (dependencies will be wired in Plan 03-02)
	server := mcpserver.NewServer(&mcpserver.Config{})

	// Run server (blocks until client disconnects or signal received)
	if err := server.Run(ctx); err != nil {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}
