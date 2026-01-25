package storage

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/qdrant/go-client/qdrant"
)

// QdrantStorage wraps the Qdrant client with connection management and health checks.
type QdrantStorage struct {
	client *qdrant.Client
	host   string
	port   int
}

// NewQdrantStorage creates a new Qdrant client with health validation.
// It performs health check with retry on startup and fails fast if Qdrant is unreachable.
func NewQdrantStorage(host string, port int) (*QdrantStorage, error) {
	// Create Qdrant client using gRPC
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}

	storage := &QdrantStorage{
		client: client,
		host:   host,
		port:   port,
	}

	// Perform health check with exponential backoff retry
	ctx := context.Background()
	err = storage.healthCheckWithRetry(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("%w: %v", ErrQdrantUnreachable, err)
	}

	return storage, nil
}

// healthCheckWithRetry performs health check with exponential backoff.
// Initial interval 500ms, max interval 10s, max elapsed 30s.
func (s *QdrantStorage) healthCheckWithRetry(ctx context.Context) error {
	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.InitialInterval = 500 * time.Millisecond
	exponentialBackoff.MaxInterval = 10 * time.Second
	exponentialBackoff.MaxElapsedTime = 30 * time.Second

	operation := func() error {
		err := s.Health(ctx)
		if err != nil {
			// Permanent errors (client errors) should not be retried
			// For now, all errors are considered retryable network issues
			return err
		}
		return nil
	}

	return backoff.Retry(operation, exponentialBackoff)
}

// Health performs a single health check against Qdrant.
// Returns nil if Qdrant is healthy, error otherwise.
func (s *QdrantStorage) Health(ctx context.Context) error {
	result, err := s.client.HealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if result == nil || result.Title == "" {
		return fmt.Errorf("health check returned invalid response")
	}

	return nil
}

// EnsureCollection ensures the documents collection exists with proper configuration.
// Creates collection with 1536-dimension vectors (cosine distance) and payload indexes.
// Idempotent - safe to call multiple times.
func (s *QdrantStorage) EnsureCollection(ctx context.Context) error {
	// Check if collection already exists
	collections, err := s.client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	// Check if our collection exists
	for _, name := range collections {
		if name == CollectionName {
			// Collection already exists, nothing to do
			return nil
		}
	}

	// Collection doesn't exist, create it with named vectors
	// This allows parent documents (no vector) and chunks (with "content" vector) in same collection
	err = s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: CollectionName,
		VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
			"content": {
				Size:     VectorDimension,
				Distance: qdrant.Distance_Cosine,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	// Create payload indexes for all filterable fields
	err = s.createPayloadIndexes(ctx)
	if err != nil {
		return fmt.Errorf("failed to create payload indexes: %w", err)
	}

	return nil
}

// createPayloadIndexes creates indexes for all filterable fields.
// CRITICAL: Without these indexes, filtering becomes 10-100x slower.
func (s *QdrantStorage) createPayloadIndexes(ctx context.Context) error {
	fields := []string{
		"path",          // Filter documents by file path
		"repository",    // Filter by repository
		"commit_sha",    // Filter by commit
		"type",          // Distinguish "parent" vs "chunk"
		"parent_doc_id", // Lookup chunks by parent
	}

	for _, field := range fields {
		_, err := s.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
			CollectionName: CollectionName,
			FieldName:      field,
			FieldType:      qdrant.FieldType_FieldTypeKeyword.Enum(),
		})
		if err != nil {
			return fmt.Errorf("failed to create index for field %s: %w", field, err)
		}
	}

	return nil
}

// ClearCollection deletes all points in the collection.
// Useful for re-indexing scenarios.
func (s *QdrantStorage) ClearCollection(ctx context.Context) error {
	// Delete collection and recreate it
	err := s.client.DeleteCollection(ctx, CollectionName)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	// Recreate with proper configuration
	return s.EnsureCollection(ctx)
}

// Close closes the Qdrant client connection.
func (s *QdrantStorage) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// upsertWithRetry performs upsert operation with exponential backoff retry.
func (s *QdrantStorage) upsertWithRetry(ctx context.Context, points []*qdrant.PointStruct) error {
	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.InitialInterval = 500 * time.Millisecond
	exponentialBackoff.MaxInterval = 10 * time.Second
	exponentialBackoff.MaxElapsedTime = 30 * time.Second

	operation := func() error {
		_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: CollectionName,
			Points:         points,
		})
		return err
	}

	return backoff.Retry(operation, exponentialBackoff)
}

// UpsertDocument stores a parent document in Qdrant.
// Parent documents have no embedding vector - they exist for full-content retrieval.
func (s *QdrantStorage) UpsertDocument(ctx context.Context, doc *Document) error {
	// Build payload map
	payload := map[string]any{
		"type":       "parent",
		"content":    doc.Content,
		"path":       doc.Metadata.Path,
		"url":        doc.Metadata.URL,
		"repository": doc.Metadata.Repository,
		"commit_sha": doc.Metadata.CommitSHA,
		"indexed_at": doc.Metadata.IndexedAt.Format(time.RFC3339),
		"summary":    doc.Metadata.Summary,
	}

	// Add entities as interface slice (NewValueMap will handle conversion)
	if len(doc.Metadata.Entities) > 0 {
		entities := make([]interface{}, len(doc.Metadata.Entities))
		for i, entity := range doc.Metadata.Entities {
			entities[i] = entity
		}
		payload["entities"] = entities
	} else {
		payload["entities"] = []interface{}{}
	}

	// Parent documents don't have vectors - use empty vector map
	point := &qdrant.PointStruct{
		Id:      qdrant.NewIDUUID(doc.ID),
		Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{}),
		Payload: qdrant.NewValueMap(payload),
	}

	return s.upsertWithRetry(ctx, []*qdrant.PointStruct{point})
}

// UpsertChunks stores multiple chunks with embeddings in Qdrant.
// Chunks are batched in groups of 100 for performance.
func (s *QdrantStorage) UpsertChunks(ctx context.Context, chunks []*Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Validate embedding dimensions
	for i, chunk := range chunks {
		if len(chunk.Embedding) != VectorDimension {
			return fmt.Errorf("%w: chunk %d has %d dimensions, expected %d",
				ErrDimensionMismatch, i, len(chunk.Embedding), VectorDimension)
		}
	}

	// Batch upserts in groups of 100
	batchSize := 100
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		points := make([]*qdrant.PointStruct, len(batch))

		for j, chunk := range batch {
			points[j] = &qdrant.PointStruct{
				Id: qdrant.NewIDUUID(chunk.ID),
				Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{
					"content": qdrant.NewVector(chunk.Embedding...),
				}),
				Payload: qdrant.NewValueMap(map[string]any{
					"type":          "chunk",
					"parent_doc_id": chunk.ParentDocID,
					"chunk_index":   chunk.ChunkIndex,
					"header_path":   chunk.HeaderPath,
					"content":       chunk.Content,
					"path":          chunk.Path,
					"repository":    chunk.Repository,
				}),
			}
		}

		err := s.upsertWithRetry(ctx, points)
		if err != nil {
			return fmt.Errorf("failed to upsert batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// GetDocument retrieves a parent document by ID.
// Returns ErrDocumentNotFound if document doesn't exist.
func (s *QdrantStorage) GetDocument(ctx context.Context, id string) (*Document, error) {
	result, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: CollectionName,
		Ids:            []*qdrant.PointId{qdrant.NewIDUUID(id)},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if len(result) == 0 {
		return nil, ErrDocumentNotFound
	}

	point := result[0]
	payload := point.Payload

	// Verify this is a parent document
	typeVal, ok := payload["type"]
	if !ok || typeVal.GetStringValue() != "parent" {
		return nil, ErrDocumentNotFound
	}

	// Parse indexed_at timestamp
	indexedAt, err := time.Parse(time.RFC3339, payload["indexed_at"].GetStringValue())
	if err != nil {
		indexedAt = time.Time{} // Use zero time if parse fails
	}

	// Extract entities (handle as list of strings)
	var entities []string
	if entitiesVal, ok := payload["entities"]; ok && entitiesVal.GetListValue() != nil {
		for _, val := range entitiesVal.GetListValue().Values {
			entities = append(entities, val.GetStringValue())
		}
	}

	doc := &Document{
		ID:      id,
		Content: payload["content"].GetStringValue(),
		Metadata: DocumentMetadata{
			Path:       payload["path"].GetStringValue(),
			URL:        payload["url"].GetStringValue(),
			Repository: payload["repository"].GetStringValue(),
			CommitSHA:  payload["commit_sha"].GetStringValue(),
			IndexedAt:  indexedAt,
			Summary:    payload["summary"].GetStringValue(),
			Entities:   entities,
		},
	}

	return doc, nil
}

// SearchChunks performs vector similarity search on chunks.
// Returns top N chunks ordered by similarity score.
func (s *QdrantStorage) SearchChunks(ctx context.Context, embedding []float32, limit int, repository string) ([]*Chunk, error) {
	if len(embedding) != VectorDimension {
		return nil, fmt.Errorf("%w: query has %d dimensions, expected %d",
			ErrDimensionMismatch, len(embedding), VectorDimension)
	}

	// Build filter conditions
	must := []*qdrant.Condition{
		qdrant.NewMatch("type", "chunk"),
	}
	if repository != "" {
		must = append(must, qdrant.NewMatch("repository", repository))
	}

	filter := &qdrant.Filter{
		Must: must,
	}

	// Perform vector search using named vector "content"
	vectorName := "content"
	results, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: CollectionName,
		Query:          qdrant.NewQuery(embedding...),
		Using:          &vectorName,
		Filter:         filter,
		Limit:          qdrant.PtrOf(uint64(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search chunks: %w", err)
	}

	chunks := make([]*Chunk, 0, len(results))
	for _, result := range results {
		payload := result.Payload

		chunk := &Chunk{
			ID:          result.Id.GetUuid(),
			ParentDocID: payload["parent_doc_id"].GetStringValue(),
			ChunkIndex:  int(payload["chunk_index"].GetIntegerValue()),
			HeaderPath:  payload["header_path"].GetStringValue(),
			Content:     payload["content"].GetStringValue(),
			Path:        payload["path"].GetStringValue(),
			Repository:  payload["repository"].GetStringValue(),
			// Note: Embedding not returned in search results (not needed)
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// SearchChunksWithScores performs vector similarity search on chunks.
// Returns top N chunks with similarity scores, ordered by score descending.
// This replaces SearchChunks for MCP handlers that need relevance scores.
func (s *QdrantStorage) SearchChunksWithScores(ctx context.Context, embedding []float32, limit int, repository string) ([]*ScoredChunk, error) {
	if len(embedding) != VectorDimension {
		return nil, fmt.Errorf("%w: query has %d dimensions, expected %d",
			ErrDimensionMismatch, len(embedding), VectorDimension)
	}

	// Build filter conditions
	must := []*qdrant.Condition{
		qdrant.NewMatch("type", "chunk"),
	}
	if repository != "" {
		must = append(must, qdrant.NewMatch("repository", repository))
	}

	filter := &qdrant.Filter{
		Must: must,
	}

	// Perform vector search using named vector "content"
	vectorName := "content"
	results, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: CollectionName,
		Query:          qdrant.NewQuery(embedding...),
		Using:          &vectorName,
		Filter:         filter,
		Limit:          qdrant.PtrOf(uint64(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false), // Don't need vectors in response
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search chunks: %w", err)
	}

	scoredChunks := make([]*ScoredChunk, 0, len(results))
	for _, result := range results {
		payload := result.Payload

		chunk := &Chunk{
			ID:          result.Id.GetUuid(),
			ParentDocID: payload["parent_doc_id"].GetStringValue(),
			ChunkIndex:  int(payload["chunk_index"].GetIntegerValue()),
			HeaderPath:  payload["header_path"].GetStringValue(),
			Content:     payload["content"].GetStringValue(),
			Path:        payload["path"].GetStringValue(),
			Repository:  payload["repository"].GetStringValue(),
		}

		scoredChunks = append(scoredChunks, &ScoredChunk{
			Chunk: chunk,
			Score: float64(result.Score), // Qdrant returns float32, convert to float64
		})
	}

	return scoredChunks, nil
}

// GetCommitSHA retrieves the commit SHA for indexed content from a repository.
// Returns empty string if no documents found for the repository.
func (s *QdrantStorage) GetCommitSHA(ctx context.Context, repository string) (string, error) {
	// Scroll for any parent document from this repository (no vector search needed)
	results, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: CollectionName,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("type", "parent"),
				qdrant.NewMatch("repository", repository),
			},
		},
		Limit:       qdrant.PtrOf(uint32(1)),
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to scroll for commit SHA: %w", err)
	}

	if len(results) == 0 {
		return "", nil // No documents found for this repository
	}

	commitSHA := results[0].Payload["commit_sha"].GetStringValue()
	return commitSHA, nil
}

// ListDocumentPaths returns all unique document paths in the index.
// Uses Scroll API to iterate through all parent documents.
func (s *QdrantStorage) ListDocumentPaths(ctx context.Context, repository string) ([]string, error) {
	var paths []string
	var offset *qdrant.PointId

	// Build filter for parent documents
	must := []*qdrant.Condition{
		qdrant.NewMatch("type", "parent"),
	}
	if repository != "" {
		must = append(must, qdrant.NewMatch("repository", repository))
	}

	filter := &qdrant.Filter{
		Must: must,
	}

	batchSize := uint32(100)

	// Scroll through all parent documents
	for {
		results, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
			CollectionName: CollectionName,
			Filter:         filter,
			Limit:          qdrant.PtrOf(batchSize),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayloadInclude("path"), // Only need path field
		})
		if err != nil {
			return nil, fmt.Errorf("failed to scroll documents: %w", err)
		}

		for _, result := range results {
			if path := result.Payload["path"].GetStringValue(); path != "" {
				paths = append(paths, path)
			}
		}

		// Stop if we got fewer results than batch size (no more pages)
		if uint32(len(results)) < batchSize {
			break
		}

		// Get offset for next page (last point ID)
		offset = results[len(results)-1].Id
	}

	// Sort paths alphabetically for consistent ordering
	sort.Strings(paths)
	return paths, nil
}

// GetDocumentByPath retrieves a parent document by its path.
// Returns ErrDocumentNotFound if no document exists with the given path.
func (s *QdrantStorage) GetDocumentByPath(ctx context.Context, path string, repository string) (*Document, error) {
	// Build filter for parent document with matching path
	must := []*qdrant.Condition{
		qdrant.NewMatch("type", "parent"),
		qdrant.NewMatch("path", path),
	}
	if repository != "" {
		must = append(must, qdrant.NewMatch("repository", repository))
	}

	filter := &qdrant.Filter{
		Must: must,
	}

	results, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: CollectionName,
		Filter:         filter,
		Limit:          qdrant.PtrOf(uint32(1)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query document by path: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrDocumentNotFound
	}

	point := results[0]
	payload := point.Payload

	// Parse indexed_at timestamp
	indexedAt, err := time.Parse(time.RFC3339, payload["indexed_at"].GetStringValue())
	if err != nil {
		indexedAt = time.Time{} // Use zero time if parse fails
	}

	// Extract entities
	var entities []string
	if entitiesVal, ok := payload["entities"]; ok && entitiesVal.GetListValue() != nil {
		for _, val := range entitiesVal.GetListValue().Values {
			entities = append(entities, val.GetStringValue())
		}
	}

	doc := &Document{
		ID:      point.Id.GetUuid(),
		Content: payload["content"].GetStringValue(),
		Metadata: DocumentMetadata{
			Path:       payload["path"].GetStringValue(),
			URL:        payload["url"].GetStringValue(),
			Repository: payload["repository"].GetStringValue(),
			CommitSHA:  payload["commit_sha"].GetStringValue(),
			IndexedAt:  indexedAt,
			Summary:    payload["summary"].GetStringValue(),
			Entities:   entities,
		},
	}

	return doc, nil
}

// CollectionInfo contains collection statistics
type CollectionInfo struct {
	PointsCount uint64
}

// GetCollectionInfo retrieves collection statistics including total points count.
// Used for calculating total chunks in the index.
func (s *QdrantStorage) GetCollectionInfo(ctx context.Context) (*CollectionInfo, error) {
	collection, err := s.client.GetCollection(ctx, CollectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	return &CollectionInfo{
		PointsCount: collection.PointsCount,
	}, nil
}
