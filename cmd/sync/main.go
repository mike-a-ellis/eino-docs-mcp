// Package main provides the sync CLI for Eino User Manual documentation indexing.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/bull/eino-mcp-server/internal/embedding"
	ghclient "github.com/bull/eino-mcp-server/internal/github"
	"github.com/bull/eino-mcp-server/internal/indexer"
	"github.com/bull/eino-mcp-server/internal/markdown"
	"github.com/bull/eino-mcp-server/internal/metadata"
	"github.com/bull/eino-mcp-server/internal/storage"
)

var rootCmd = &cobra.Command{
	Use:   "eino-sync",
	Short: "Eino User Manual documentation indexing tool",
	Long:  "CLI tool for managing Eino User Manual documentation index in Qdrant",
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Re-index all documentation from GitHub",
	Long: `Clears existing index and rebuilds from latest GitHub commit.

This command:
1. Connects to Qdrant and verifies health
2. Clears the existing document collection
3. Fetches all Eino User Manual documentation from GitHub
4. Generates embeddings and metadata for each document
5. Stores documents and chunks in Qdrant

Environment variables:
  QDRANT_HOST    Qdrant hostname (default: localhost)
  QDRANT_PORT    Qdrant gRPC port (default: 6334)
  OPENAI_API_KEY OpenAI API key for embeddings (required)
  GITHUB_TOKEN   GitHub token for higher rate limits (optional)`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func main() {
	// Load .env file if present (local development), ignore if missing (production)
	_ = godotenv.Load()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	start := time.Now()

	fmt.Println("Starting sync...")
	fmt.Println()

	// Get environment configuration
	qdrantHost := getEnv("QDRANT_HOST", "localhost")
	qdrantPort := getEnvInt("QDRANT_PORT", 6334)

	// 1. Connect to Qdrant
	fmt.Printf("Connecting to Qdrant at %s:%d...\n", qdrantHost, qdrantPort)
	store, err := storage.NewQdrantStorage(qdrantHost, qdrantPort)
	if err != nil {
		return fmt.Errorf("Failed to connect to Qdrant: %w", err)
	}
	defer store.Close()

	// 2. Check health
	if err := store.Health(ctx); err != nil {
		return fmt.Errorf("Qdrant health check failed: %w", err)
	}
	fmt.Println("Qdrant healthy")

	// 3. Ensure collection exists
	if err := store.EnsureCollection(ctx); err != nil {
		return fmt.Errorf("Failed to ensure collection: %w", err)
	}

	// 4. Initialize embedding client
	embeddingClient, err := embedding.NewClient()
	if err != nil {
		return fmt.Errorf("Failed to create embedding client: %w", err)
	}
	embedder := embedding.NewEmbedder(embeddingClient, 0) // Use default batch size

	// 5. Initialize GitHub client
	ghClient, err := ghclient.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("Failed to create GitHub client: %w", err)
	}

	// 6. Initialize other components
	chunker := markdown.NewChunker()
	// Use the same OpenAI client from embeddings for metadata generation
	generator := metadata.NewGenerator(embeddingClient.Client())
	fetcher := ghclient.NewFetcher(ghClient, "cloudwego", "cloudwego.github.io", "content/en/docs/eino")

	// 7. Clear existing collection
	fmt.Println()
	fmt.Println("Clearing existing collection...")
	if err := store.ClearCollection(ctx); err != nil {
		return fmt.Errorf("Failed to clear collection: %w", err)
	}
	fmt.Println("Collection cleared")

	// 8. Initialize pipeline and run indexing
	fmt.Println()
	fmt.Println("Indexing documents from GitHub...")
	pipeline := indexer.NewPipeline(fetcher, chunker, embedder, generator, store, slog.Default())

	result, err := pipeline.IndexAll(ctx)
	if err != nil {
		return fmt.Errorf("Indexing failed: %w", err)
	}

	// 9. Print results
	fmt.Println()
	fmt.Println("Sync complete!")
	fmt.Printf("  Documents: %d/%d\n", result.SuccessfulDocs, result.TotalDocs)
	fmt.Printf("  Chunks: %d\n", result.TotalChunks)
	fmt.Printf("  Duration: %s\n", result.Duration.Round(time.Second))
	fmt.Printf("  Commit: %s\n", result.CommitSHA)

	// 10. Print failed documents if any
	if len(result.FailedDocs) > 0 {
		fmt.Println()
		fmt.Println("Failed documents:")
		for _, failed := range result.FailedDocs {
			fmt.Printf("  - %s: %s\n", failed.Path, failed.Reason)
		}
	}

	fmt.Println()
	fmt.Printf("Total time: %s\n", time.Since(start).Round(time.Second))

	return nil
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
