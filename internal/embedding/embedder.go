package embedding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/openai/openai-go"
)

const (
	// EmbeddingModel is the OpenAI model used for generating embeddings.
	EmbeddingModel = "text-embedding-3-small"

	// EmbeddingDimension is the vector dimension for text-embedding-3-small.
	// This matches storage.VectorDimension (1536).
	EmbeddingDimension = 1536

	// DefaultBatchSize balances requests-per-minute vs tokens-per-minute rate limits.
	// OpenAI supports up to 2048 texts per batch, but smaller batches reduce TPM pressure.
	DefaultBatchSize = 500
)

// Embedder generates embeddings for text using OpenAI's text-embedding-3-small model.
// It batches requests for efficiency and implements exponential backoff on rate limit errors.
type Embedder struct {
	client    *Client
	batchSize int
}

// NewEmbedder creates a new Embedder with the given client and optional batch size.
// If batchSize is 0, DefaultBatchSize (500) is used.
func NewEmbedder(client *Client, batchSize int) *Embedder {
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	return &Embedder{
		client:    client,
		batchSize: batchSize,
	}
}

// GenerateEmbeddings generates embeddings for the given texts.
// Returns [][]float32 to match storage.Chunk.Embedding type.
// Batches requests and retries with exponential backoff on rate limit errors.
func (e *Embedder) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	var allEmbeddings [][]float32

	// Process in batches
	for i := 0; i < len(texts); i += e.batchSize {
		end := min(i+e.batchSize, len(texts))
		batch := texts[i:end]

		embeddings, err := e.embedBatchWithRetry(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d: %w", i, end, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// embedBatchWithRetry generates embeddings for a single batch with retry logic.
// Retries with exponential backoff on rate limit errors (HTTP 429).
// Other errors are treated as permanent and fail immediately.
func (e *Embedder) embedBatchWithRetry(ctx context.Context, texts []string) ([][]float32, error) {
	var embeddings [][]float32

	operation := func() error {
		resp, err := e.client.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
			Input: openai.EmbeddingNewParamsInputUnion{
				OfArrayOfStrings: texts,
			},
			Model: "text-embedding-3-small",
		})
		if err != nil {
			// Check if retryable (rate limit error)
			if isRateLimitError(err) {
				return err // Will retry with backoff
			}
			return backoff.Permanent(err) // Don't retry
		}

		// Convert float64 to float32 for storage compatibility
		embeddings = make([][]float32, len(resp.Data))
		for i, data := range resp.Data {
			embeddings[i] = toFloat32(data.Embedding)
		}
		return nil
	}

	// Configure exponential backoff
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 500 * time.Millisecond
	b.MaxInterval = 10 * time.Second
	b.MaxElapsedTime = 30 * time.Second

	err := backoff.Retry(operation, backoff.WithContext(b, ctx))
	return embeddings, err
}

// isRateLimitError checks if the error is a rate limit error (HTTP 429).
func isRateLimitError(err error) bool {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429
	}
	return false
}

// toFloat32 converts []float64 to []float32.
// OpenAI API returns float64, but storage uses float32 for memory efficiency.
func toFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}
