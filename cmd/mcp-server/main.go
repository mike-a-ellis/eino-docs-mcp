// Package main provides the MCP server entry point for EINO documentation.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/bull/eino-mcp-server/internal/embedding"
	ghclient "github.com/bull/eino-mcp-server/internal/github"
	mcpserver "github.com/bull/eino-mcp-server/internal/mcp"
	"github.com/bull/eino-mcp-server/internal/storage"
)

func main() {
	// Load .env file if present (local development), ignore if missing (production)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Create context that cancels on SIGTERM/SIGINT
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Configuration from environment
	qdrantHost := getEnv("QDRANT_HOST", "localhost")
	qdrantPort := getEnvInt("QDRANT_PORT", 6334)
	port := getEnv("PORT", "8080")

	// Initialize storage
	store, err := storage.NewQdrantStorage(qdrantHost, qdrantPort)
	if err != nil {
		log.Fatalf("failed to connect to Qdrant: %v", err)
	}
	defer store.Close()

	// Ensure collection exists
	if err := store.EnsureCollection(ctx); err != nil {
		log.Fatalf("failed to ensure collection: %v", err)
	}

	// Initialize embedding client
	embeddingClient, err := embedding.NewClient()
	if err != nil {
		log.Fatalf("failed to create embedding client: %v", err)
	}
	embedder := embedding.NewEmbedder(embeddingClient, 0) // Use default batch size

	// Initialize GitHub client
	ghClient, err := ghclient.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create GitHub client: %v", err)
	}

	// Create MCP server
	server := mcpserver.NewServer(&mcpserver.Config{
		Storage:  store,
		Embedder: embedder,
		GitHub:   ghClient,
	})

	// Create HTTP server with multiple endpoints
	mux := http.NewServeMux()

	// Health endpoint (for Fly.io health checks)
	healthHandler := mcpserver.NewHealthHandler(store)
	mux.HandleFunc("/health", healthHandler)

	// MCP HTTP endpoint (for remote client connections)
	mcpHTTPHandler := mcpserver.NewHTTPHandler(server, nil)
	mux.Handle("/mcp", mcpHTTPHandler)

	// Check if running in server mode (HTTP) or stdio mode (local development)
	serverMode := getEnv("SERVER_MODE", "false") == "true"

	if serverMode {
		// HTTP mode: serve MCP over HTTP for remote clients
		addr := "0.0.0.0:" + port
		log.Printf("Starting HTTP server on %s (MCP at /mcp, health at /health)", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	} else {
		// Stdio mode: run MCP server over stdin/stdout for local clients
		// Also start HTTP health endpoint in background for local testing
		go func() {
			addr := "0.0.0.0:" + port
			log.Printf("Starting health server on %s", addr)
			if err := http.ListenAndServe(addr, mux); err != nil {
				log.Printf("Health server error: %v", err)
			}
		}()

		log.Println("Starting EINO Documentation MCP Server (stdio mode)...")
		if err := server.Run(ctx); err != nil {
			log.Printf("server error: %v", err)
			os.Exit(1)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}
