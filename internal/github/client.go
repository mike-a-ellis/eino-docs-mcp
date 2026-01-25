package github

import (
	"context"
	"os"

	"github.com/gofri/go-github-ratelimit/github_ratelimit"
	"github.com/google/go-github/v81/github"
)

// Client wraps the GitHub API client with rate limiting support
type Client struct {
	*github.Client
}

// NewClient creates a new GitHub client with optional authentication and rate limiting.
// If GITHUB_TOKEN environment variable is set, the client will be authenticated.
// Rate limiting is automatically handled using exponential backoff.
func NewClient(ctx context.Context) (*Client, error) {
	// Create rate limit handler with default configuration
	// This handles both primary rate limits (5000 req/hour authenticated, 60 unauthenticated)
	// and secondary rate limits (abuse detection) with automatic retry
	rateLimiter, err := github_ratelimit.NewRateLimitWaiterClient(nil)
	if err != nil {
		return nil, err
	}

	// Create GitHub client with rate limiting
	ghClient := github.NewClient(rateLimiter)

	// If GITHUB_TOKEN is set, use authenticated client for higher rate limits
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		ghClient = ghClient.WithAuthToken(token)
	}

	return &Client{Client: ghClient}, nil
}
