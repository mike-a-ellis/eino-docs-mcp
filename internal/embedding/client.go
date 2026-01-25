package embedding

import (
	"fmt"
	"os"

	"github.com/openai/openai-go"
)

// Client wraps the OpenAI client for embedding generation.
type Client struct {
	client *openai.Client
}

// NewClient creates a new OpenAI client for embedding generation.
// It reads the OPENAI_API_KEY from the environment and returns an error if not set.
func NewClient() (*Client, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	// openai-go automatically reads OPENAI_API_KEY from environment
	client := openai.NewClient()

	return &Client{client: &client}, nil
}

// Client returns the underlying OpenAI client for use in other packages (e.g., metadata generation).
func (c *Client) Client() *openai.Client {
	return c.client
}
