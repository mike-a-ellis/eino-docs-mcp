package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/bull/eino-mcp-server/internal/embedding"
	"github.com/bull/eino-mcp-server/internal/github"
	"github.com/bull/eino-mcp-server/internal/markdown"
	"github.com/bull/eino-mcp-server/internal/metadata"
	"github.com/bull/eino-mcp-server/internal/storage"
)

// IndexResult contains statistics about an indexing operation.
type IndexResult struct {
	TotalDocs      int
	TotalChunks    int
	SuccessfulDocs int
	FailedDocs     []FailedDoc
	CommitSHA      string
	Duration       time.Duration
}

// FailedDoc represents a document that failed to index.
type FailedDoc struct {
	Path   string
	Reason string
}

// Pipeline orchestrates the full indexing process from fetching to storage.
type Pipeline struct {
	fetcher   *github.Fetcher
	chunker   *markdown.Chunker
	embedder  *embedding.Embedder
	generator *metadata.Generator
	storage   *storage.QdrantStorage
	logger    *slog.Logger
}

// NewPipeline creates a new indexing pipeline with the given components.
func NewPipeline(
	fetcher *github.Fetcher,
	chunker *markdown.Chunker,
	embedder *embedding.Embedder,
	generator *metadata.Generator,
	storage *storage.QdrantStorage,
	logger *slog.Logger,
) *Pipeline {
	if logger == nil {
		logger = slog.Default()
	}
	return &Pipeline{
		fetcher:   fetcher,
		chunker:   chunker,
		embedder:  embedder,
		generator: generator,
		storage:   storage,
		logger:    logger,
	}
}

// IndexAll fetches all documents from GitHub and indexes them in Qdrant.
// Returns detailed statistics about the indexing operation.
func (p *Pipeline) IndexAll(ctx context.Context) (*IndexResult, error) {
	start := time.Now()
	result := &IndexResult{}

	// 1. Get latest commit SHA
	commitSHA, err := p.fetcher.GetLatestCommitSHA(ctx)
	if err != nil {
		return nil, fmt.Errorf("get commit SHA: %w", err)
	}
	result.CommitSHA = commitSHA
	p.logger.Info("Starting indexing", "commit", commitSHA)

	// 2. List all docs
	paths, err := p.fetcher.ListDocs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list docs: %w", err)
	}
	result.TotalDocs = len(paths)
	p.logger.Info("Found documents", "count", len(paths))

	// 3. Process each document
	for _, path := range paths {
		chunks, err := p.processDocument(ctx, path, commitSHA)
		if err != nil {
			p.logger.Warn("Failed to process document", "path", path, "error", err)
			result.FailedDocs = append(result.FailedDocs, FailedDoc{
				Path:   path,
				Reason: err.Error(),
			})
			continue // Skip unparseable docs, continue with others
		}
		result.SuccessfulDocs++
		result.TotalChunks += chunks
	}

	result.Duration = time.Since(start)
	p.logger.Info("Indexing complete",
		"successful", result.SuccessfulDocs,
		"failed", len(result.FailedDocs),
		"chunks", result.TotalChunks,
		"duration", result.Duration,
	)

	return result, nil
}

// processDocument handles the full pipeline for a single document.
// Returns the number of chunks created for the document.
func (p *Pipeline) processDocument(ctx context.Context, path, commitSHA string) (int, error) {
	// Fetch content
	fetched, err := p.fetcher.FetchDoc(ctx, path)
	if err != nil {
		return 0, fmt.Errorf("fetch: %w", err)
	}
	p.logger.Debug("Fetched document", "path", path, "size", len(fetched.Content))

	// Generate metadata (summary, entities)
	meta, err := p.generator.GenerateMetadata(ctx, path, fetched.Content)
	if err != nil {
		p.logger.Warn("Metadata generation failed, using empty", "path", path, "error", err)
		meta = &metadata.DocumentMetadata{Summary: "", Entities: []string{}}
	}

	// Chunk document
	chunks, err := p.chunker.ChunkDocument([]byte(fetched.Content))
	if err != nil {
		return 0, fmt.Errorf("chunk: %w", err)
	}
	p.logger.Debug("Chunked document", "path", path, "chunks", len(chunks))

	// Generate embeddings for all chunks
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content // Content already has header path prepended
	}

	embeddings, err := p.embedder.GenerateEmbeddings(ctx, texts)
	if err != nil {
		return 0, fmt.Errorf("embeddings: %w", err)
	}

	// Create parent document
	docID := uuid.New().String()
	doc := &storage.Document{
		ID:      docID,
		Content: fetched.Content,
		Metadata: storage.DocumentMetadata{
			Path:       path,
			URL:        fetched.URL,
			Repository: "cloudwego/cloudwego.github.io",
			CommitSHA:  commitSHA,
			IndexedAt:  time.Now(),
			Summary:    meta.Summary,
			Entities:   meta.Entities,
		},
	}

	if err := p.storage.UpsertDocument(ctx, doc); err != nil {
		return 0, fmt.Errorf("store document: %w", err)
	}

	// Create chunks with embeddings
	storageChunks := make([]*storage.Chunk, len(chunks))
	for i, chunk := range chunks {
		storageChunks[i] = &storage.Chunk{
			ID:          uuid.New().String(),
			ParentDocID: docID,
			ChunkIndex:  chunk.Index,
			HeaderPath:  chunk.HeaderPath,
			Content:     chunk.RawContent, // Store without header prefix in payload
			Path:        path,
			Repository:  "cloudwego/cloudwego.github.io",
			Embedding:   embeddings[i],
		}
	}

	if err := p.storage.UpsertChunks(ctx, storageChunks); err != nil {
		return 0, fmt.Errorf("store chunks: %w", err)
	}

	p.logger.Info("Indexed document", "path", path, "chunks", len(chunks))
	return len(chunks), nil
}
