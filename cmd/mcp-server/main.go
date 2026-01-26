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

	// Start health check server in background
	// QdrantStorage implements HealthChecker interface via its Health(ctx) method
	healthHandler := mcpserver.NewHealthHandler(store)
	http.HandleFunc("/health", healthHandler)
	go func() {
		addr := "0.0.0.0:" + port
		log.Printf("Starting health server on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Health server error: %v", err)
		}
	}()

	// Create and run MCP server (stdio)
	server := mcpserver.NewServer(&mcpserver.Config{
		Storage:  store,
		Embedder: embedder,
		GitHub:   ghClient,
	})

	// Check if running in server mode (health endpoint only) or stdio mode
	serverMode := getEnv("SERVER_MODE", "false") == "true"

	if serverMode {
		// Server mode: keep process alive for health endpoint only
		log.Println("Running in server mode (health endpoint only)")
		<-ctx.Done()
		log.Println("Shutting down...")
	} else {
		// Stdio mode: run MCP server over stdin/stdout
		log.Println("Starting EINO Documentation MCP Server...")
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
