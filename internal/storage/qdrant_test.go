// +build integration

package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestStorage creates a test storage instance and ensures collection exists.
// Skips test if Qdrant is not running.
func setupTestStorage(t *testing.T) *QdrantStorage {
	storage, err := NewQdrantStorage("localhost", 6334)
	if err != nil {
		t.Skipf("Qdrant not available: %v", err)
	}

	err = storage.EnsureCollection(context.Background())
	require.NoError(t, err, "Failed to ensure collection")

	return storage
}

func TestDocumentRoundTrip(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Create document with all metadata fields populated
	docID := uuid.New().String()
	now := time.Now().UTC().Truncate(time.Second) // Truncate to avoid microsecond precision issues

	doc := &Document{
		ID:      docID,
		Content: "# Test Document\n\nThis is test content with **markdown**.",
		Metadata: DocumentMetadata{
			Path:       "test/roundtrip.md",
			URL:        "https://raw.githubusercontent.com/test/repo/main/test/roundtrip.md",
			Repository: "test/repo",
			CommitSHA:  "abc123def456",
			IndexedAt:  now,
			Summary:    "A test document for roundtrip testing",
			Entities:   []string{"TestFunction", "TestStruct"},
		},
	}

	// Upsert document
	err := storage.UpsertDocument(ctx, doc)
	require.NoError(t, err, "Failed to upsert document")

	// Retrieve document
	retrieved, err := storage.GetDocument(ctx, docID)
	require.NoError(t, err, "Failed to get document")

	// Assert all fields match
	assert.Equal(t, doc.ID, retrieved.ID)
	assert.Equal(t, doc.Content, retrieved.Content)
	assert.Equal(t, doc.Metadata.Path, retrieved.Metadata.Path)
	assert.Equal(t, doc.Metadata.URL, retrieved.Metadata.URL)
	assert.Equal(t, doc.Metadata.Repository, retrieved.Metadata.Repository)
	assert.Equal(t, doc.Metadata.CommitSHA, retrieved.Metadata.CommitSHA)
	assert.Equal(t, doc.Metadata.Summary, retrieved.Metadata.Summary)
	assert.ElementsMatch(t, doc.Metadata.Entities, retrieved.Metadata.Entities)

	// Time comparison with tolerance for serialization
	assert.WithinDuration(t, doc.Metadata.IndexedAt, retrieved.Metadata.IndexedAt, time.Second)
}

func TestChunkSearchRoundTrip(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Use unique repository to avoid conflicts with other tests
	repo := "test/chunk-search-" + uuid.New().String()

	// Create and upsert parent document
	docID := uuid.New().String()
	doc := &Document{
		ID:      docID,
		Content: "# Parent Document\n\nFull content here.",
		Metadata: DocumentMetadata{
			Path:       "test/parent.md",
			Repository: repo,
			CommitSHA:  "parent123",
			IndexedAt:  time.Now().UTC(),
		},
	}
	err := storage.UpsertDocument(ctx, doc)
	require.NoError(t, err)

	// Create chunk with fake embedding (1536 dimensions of 0.1)
	chunkID := uuid.New().String()
	embedding := make([]float32, VectorDimension)
	for i := range embedding {
		embedding[i] = 0.1
	}

	chunk := &Chunk{
		ID:          chunkID,
		ParentDocID: docID,
		ChunkIndex:  0,
		HeaderPath:  "Parent Document > Introduction",
		Content:     "Introduction section content",
		Path:        "test/parent.md",
		Repository:  repo,
		Embedding:   embedding,
	}

	// Upsert chunk
	err = storage.UpsertChunks(ctx, []*Chunk{chunk})
	require.NoError(t, err, "Failed to upsert chunks")

	// Search chunks with same embedding and repository filter
	results, err := storage.SearchChunks(ctx, embedding, 10, repo)
	require.NoError(t, err, "Failed to search chunks")

	// Assert chunk is found
	require.Len(t, results, 1, "Expected 1 search result")

	result := results[0]
	assert.Equal(t, chunk.ParentDocID, result.ParentDocID)
	assert.Equal(t, chunk.ChunkIndex, result.ChunkIndex)
	assert.Equal(t, chunk.Content, result.Content)
	assert.Equal(t, chunk.Path, result.Path)
	assert.Equal(t, chunk.Repository, result.Repository)
}

func TestCommitSHATracking(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Use unique repository to avoid conflicts with other tests
	repo := "test/commit-sha-" + uuid.New().String()
	expectedSHA := "abc123def456"

	// Upsert document with specific commit SHA
	doc := &Document{
		ID:      uuid.New().String(),
		Content: "# Commit SHA Test",
		Metadata: DocumentMetadata{
			Path:       "test/commit.md",
			Repository: repo,
			CommitSHA:  expectedSHA,
			IndexedAt:  time.Now().UTC(),
		},
	}
	err := storage.UpsertDocument(ctx, doc)
	require.NoError(t, err)

	// Get commit SHA for repository
	commitSHA, err := storage.GetCommitSHA(ctx, repo)
	require.NoError(t, err)
	assert.Equal(t, expectedSHA, commitSHA)

	// Test non-existent repository returns empty string
	noSHA, err := storage.GetCommitSHA(ctx, "nonexistent/repo")
	require.NoError(t, err)
	assert.Equal(t, "", noSHA)
}

func TestPersistence(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Create and store document with unique ID
	docID := uuid.New().String()
	originalContent := "# Persistence Test\n\nThis content must survive reconnection."
	originalSHA := "persist123"

	doc := &Document{
		ID:      docID,
		Content: originalContent,
		Metadata: DocumentMetadata{
			Path:       "test/persistence.md",
			Repository: "test/persistence-repo",
			CommitSHA:  originalSHA,
			IndexedAt:  time.Now().UTC(),
			Summary:    "Testing data persistence across restarts",
			Entities:   []string{"PersistenceTest"},
		},
	}

	err := storage.UpsertDocument(ctx, doc)
	require.NoError(t, err, "Failed to upsert document")

	// Verify document exists before closing
	retrieved1, err := storage.GetDocument(ctx, docID)
	require.NoError(t, err, "Failed to get document before close")
	assert.Equal(t, originalContent, retrieved1.Content)

	// Close the connection (simulates application restart)
	err = storage.Close()
	require.NoError(t, err, "Failed to close storage")

	// Create NEW storage connection (simulates restart)
	storage2, err := NewQdrantStorage("localhost", 6334)
	require.NoError(t, err, "Failed to reconnect to Qdrant")
	defer storage2.Close()

	// Retrieve document with new connection
	retrieved2, err := storage2.GetDocument(ctx, docID)
	require.NoError(t, err, "Failed to get document after reconnection")

	// Assert all data persisted correctly
	assert.Equal(t, docID, retrieved2.ID)
	assert.Equal(t, originalContent, retrieved2.Content)
	assert.Equal(t, doc.Metadata.Path, retrieved2.Metadata.Path)
	assert.Equal(t, doc.Metadata.Repository, retrieved2.Metadata.Repository)
	assert.Equal(t, originalSHA, retrieved2.Metadata.CommitSHA)
	assert.Equal(t, doc.Metadata.Summary, retrieved2.Metadata.Summary)
	assert.ElementsMatch(t, doc.Metadata.Entities, retrieved2.Metadata.Entities)
}

func TestDimensionValidation(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Test chunk with wrong dimension
	wrongChunk := &Chunk{
		ID:          uuid.New().String(),
		ParentDocID: uuid.New().String(),
		ChunkIndex:  0,
		Content:     "Wrong dimension test",
		Path:        "test/wrong.md",
		Repository:  "test/repo",
		Embedding:   make([]float32, 512), // Wrong dimension
	}

	err := storage.UpsertChunks(ctx, []*Chunk{wrongChunk})
	assert.ErrorIs(t, err, ErrDimensionMismatch, "Should reject wrong embedding dimension")

	// Test search with wrong dimension
	wrongEmbedding := make([]float32, 512)
	_, err = storage.SearchChunks(ctx, wrongEmbedding, 10, "")
	assert.ErrorIs(t, err, ErrDimensionMismatch, "Should reject wrong query dimension")
}

func TestDocumentNotFound(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Try to get non-existent document
	nonExistentID := uuid.New().String()
	_, err := storage.GetDocument(ctx, nonExistentID)
	assert.ErrorIs(t, err, ErrDocumentNotFound)
}

func TestBatchChunkUpsert(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Use unique repository to avoid conflicts
	repo := "test/batch-" + uuid.New().String()

	// Create parent document
	docID := uuid.New().String()
	doc := &Document{
		ID:      docID,
		Content: "# Batch Test",
		Metadata: DocumentMetadata{
			Path:       "test/batch.md",
			Repository: repo,
			CommitSHA:  "batch123",
			IndexedAt:  time.Now().UTC(),
		},
	}
	err := storage.UpsertDocument(ctx, doc)
	require.NoError(t, err)

	// Create 250 chunks (more than one batch of 100)
	chunks := make([]*Chunk, 250)
	embedding := make([]float32, VectorDimension)
	for i := range embedding {
		embedding[i] = 0.5
	}

	for i := range chunks {
		chunks[i] = &Chunk{
			ID:          uuid.New().String(),
			ParentDocID: docID,
			ChunkIndex:  i,
			HeaderPath:  "Batch Test > Section",
			Content:     "Chunk content",
			Path:        "test/batch.md",
			Repository:  repo,
			Embedding:   embedding,
		}
	}

	// Upsert all chunks
	err = storage.UpsertChunks(ctx, chunks)
	require.NoError(t, err, "Failed to upsert batch of chunks")

	// Search to verify chunks were stored
	results, err := storage.SearchChunks(ctx, embedding, 300, repo)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 250, "Expected at least 250 chunks in search results")
}

func TestSearchChunksWithScores(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Use unique repository to avoid conflicts with other tests
	repo := "test/scored-search-" + uuid.New().String()

	// Create and upsert parent document
	docID := uuid.New().String()
	doc := &Document{
		ID:      docID,
		Content: "# Scored Search Test\n\nContent for testing scores.",
		Metadata: DocumentMetadata{
			Path:       "test/scored.md",
			Repository: repo,
			CommitSHA:  "scored123",
			IndexedAt:  time.Now().UTC(),
		},
	}
	err := storage.UpsertDocument(ctx, doc)
	require.NoError(t, err)

	// Create chunk with embedding
	chunkID := uuid.New().String()
	embedding := make([]float32, VectorDimension)
	for i := range embedding {
		embedding[i] = 0.1
	}

	chunk := &Chunk{
		ID:          chunkID,
		ParentDocID: docID,
		ChunkIndex:  0,
		HeaderPath:  "Scored Search Test",
		Content:     "Scored search content",
		Path:        "test/scored.md",
		Repository:  repo,
		Embedding:   embedding,
	}

	err = storage.UpsertChunks(ctx, []*Chunk{chunk})
	require.NoError(t, err, "Failed to upsert chunk")

	// Search with same embedding - should get high score
	results, err := storage.SearchChunksWithScores(ctx, embedding, 10, repo)
	require.NoError(t, err, "Failed to search chunks with scores")

	// Assert chunk is found with a score
	require.Len(t, results, 1, "Expected 1 search result")

	result := results[0]
	assert.Equal(t, chunk.Content, result.Content)
	assert.Greater(t, result.Score, 0.0, "Score should be greater than 0")
	assert.LessOrEqual(t, result.Score, 1.0, "Score should be at most 1.0")
}

func TestListDocumentPaths(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Use unique repository to avoid conflicts with other tests
	repo := "test/list-paths-" + uuid.New().String()

	// Create multiple documents with different paths
	paths := []string{"docs/a.md", "docs/b.md", "docs/c.md"}

	for _, path := range paths {
		doc := &Document{
			ID:      uuid.New().String(),
			Content: "# Document at " + path,
			Metadata: DocumentMetadata{
				Path:       path,
				Repository: repo,
				CommitSHA:  "list123",
				IndexedAt:  time.Now().UTC(),
			},
		}
		err := storage.UpsertDocument(ctx, doc)
		require.NoError(t, err, "Failed to upsert document at %s", path)
	}

	// Wait for Qdrant to index documents (eventual consistency)
	time.Sleep(100 * time.Millisecond)

	// List document paths
	result, err := storage.ListDocumentPaths(ctx, repo)
	require.NoError(t, err, "Failed to list document paths")

	// Assert all paths are returned (sorted)
	assert.Len(t, result, 3, "Expected 3 document paths")
	assert.Equal(t, paths, result, "Paths should be returned in sorted order")
}

func TestListDocumentPaths_EmptyRepository(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Use non-existent repository
	result, err := storage.ListDocumentPaths(ctx, "nonexistent/repo-"+uuid.New().String())
	require.NoError(t, err)
	assert.Empty(t, result, "Expected empty list for non-existent repository")
}

func TestGetDocumentByPath(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Use unique repository to avoid conflicts
	repo := "test/by-path-" + uuid.New().String()
	path := "test/by-path.md"

	// Create document
	doc := &Document{
		ID:      uuid.New().String(),
		Content: "# Get By Path Test\n\nThis is test content.",
		Metadata: DocumentMetadata{
			Path:       path,
			URL:        "https://example.com/test/by-path.md",
			Repository: repo,
			CommitSHA:  "bypath123",
			IndexedAt:  time.Now().UTC().Truncate(time.Second),
			Summary:    "Test summary for by-path document",
			Entities:   []string{"TestEntity"},
		},
	}
	err := storage.UpsertDocument(ctx, doc)
	require.NoError(t, err, "Failed to upsert document")

	// Get document by path
	result, err := storage.GetDocumentByPath(ctx, path, repo)
	require.NoError(t, err, "Failed to get document by path")

	// Assert document matches
	assert.Equal(t, doc.ID, result.ID)
	assert.Equal(t, doc.Content, result.Content)
	assert.Equal(t, doc.Metadata.Path, result.Metadata.Path)
	assert.Equal(t, doc.Metadata.URL, result.Metadata.URL)
	assert.Equal(t, doc.Metadata.Repository, result.Metadata.Repository)
	assert.Equal(t, doc.Metadata.CommitSHA, result.Metadata.CommitSHA)
	assert.Equal(t, doc.Metadata.Summary, result.Metadata.Summary)
	assert.ElementsMatch(t, doc.Metadata.Entities, result.Metadata.Entities)
}

func TestGetDocumentByPath_NotFound(t *testing.T) {
	storage := setupTestStorage(t)
	defer storage.Close()

	ctx := context.Background()

	// Try to get non-existent document by path
	_, err := storage.GetDocumentByPath(ctx, "nonexistent/path.md", "nonexistent/repo")
	assert.ErrorIs(t, err, ErrDocumentNotFound, "Expected ErrDocumentNotFound for invalid path")
}
