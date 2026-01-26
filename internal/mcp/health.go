package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse represents the JSON response from the health check endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Qdrant    string `json:"qdrant"`
	Timestamp string `json:"timestamp"`
}

// HealthChecker interface defines the health check dependency.
// The storage layer implements this via its Health() method.
type HealthChecker interface {
	Health(ctx context.Context) error
}

// NewHealthHandler creates an HTTP handler for the /health endpoint.
// It checks Qdrant connectivity and returns appropriate status codes.
func NewHealthHandler(store HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create context with 3-second timeout for health check
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		// Check Qdrant health
		err := store.Health(ctx)

		// Prepare response
		response := HealthResponse{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			// Qdrant is unhealthy
			response.Status = "unhealthy"
			response.Qdrant = "disconnected"
			w.WriteHeader(http.StatusServiceUnavailable) // 503
			json.NewEncoder(w).Encode(response)
			return
		}

		// All healthy
		response.Status = "healthy"
		response.Qdrant = "connected"
		w.WriteHeader(http.StatusOK) // 200
		json.NewEncoder(w).Encode(response)
	}
}
