package storage

import (
	"context"
	"fmt"
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

// Close closes the Qdrant client connection.
func (s *QdrantStorage) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
