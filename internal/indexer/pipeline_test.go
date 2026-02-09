//go:build integration

package indexer

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mike-a-ellis/eino-docs-mcp/internal/embedding"
	"github.com/mike-a-ellis/eino-docs-mcp/internal/github"
	"github.com/mike-a-ellis/eino-docs-mcp/internal/markdown"
	"github.com/mike-a-ellis/eino-docs-mcp/internal/metadata"
	"github.com/mike-a-ellis/eino-docs-mcp/internal/storage"
)

func TestPipeline_IndexAll_Integration(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// Setup
	storage, err := storage.NewQdrantStorage("localhost", 6334)
	require.NoError(t, err)
	defer storage.Close()

	// Clear existing data for clean test
	err = storage.ClearCollection(context.Background())
	require.NoError(t, err)
	err = storage.EnsureCollection(context.Background())
	require.NoError(t, err)

	// Create components
	ghClient := github.NewClient()
	fetcher := github.NewFetcher(ghClient, "cloudwego", "cloudwego.github.io", "content/en/docs/eino")
	chunker := markdown.NewChunker()

	openaiClient, err := embedding.NewClient()
	require.NoError(t, err)
	embedder := embedding.NewEmbedder(openaiClient, 500)
	generator := metadata.NewGenerator(openaiClient.Client())

	pipeline := NewPipeline(fetcher, chunker, embedder, generator, storage, slog.Default())

	// Run indexing
	// Note: Full indexing tested manually, this validates wiring
	ctx := context.Background()
	result, err := pipeline.IndexAll(ctx)
	require.NoError(t, err)

	// Verify results
	assert.Greater(t, result.TotalDocs, 0, "Should find documents")
	assert.Greater(t, result.SuccessfulDocs, 0, "Should successfully index documents")
	assert.NotEmpty(t, result.CommitSHA, "Should capture commit SHA")
	assert.Greater(t, result.TotalChunks, 0, "Should create chunks")

	// Log any failures for debugging
	if len(result.FailedDocs) > 0 {
		t.Logf("Failed to index %d documents:", len(result.FailedDocs))
		for _, fail := range result.FailedDocs {
			t.Logf("  - %s: %s", fail.Path, fail.Reason)
		}
	}

	// Verify searchable
	testQuery := make([]float32, 1536) // Zero vector for simple test
	chunks, err := storage.SearchChunks(ctx, testQuery, 5, "cloudwego/cloudwego.github.io")
	require.NoError(t, err)
	assert.Greater(t, len(chunks), 0, "Should find indexed chunks")

	// Verify chunk structure
	if len(chunks) > 0 {
		chunk := chunks[0]
		assert.NotEmpty(t, chunk.ID, "Chunk should have ID")
		assert.NotEmpty(t, chunk.ParentDocID, "Chunk should have parent doc ID")
		assert.NotEmpty(t, chunk.Path, "Chunk should have path")
		assert.NotEmpty(t, chunk.Content, "Chunk should have content")
	}
}
