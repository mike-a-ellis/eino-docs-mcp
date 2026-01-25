package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/openai/openai-go"
)

// DefaultMaxTokens is the maximum content length before truncation (in tokens).
const DefaultMaxTokens = 16000

// DocumentMetadata contains LLM-generated metadata for a document.
type DocumentMetadata struct {
	Summary  string   `json:"summary"`
	Entities []string `json:"entities"`
}

// Generator produces metadata using GPT-4o.
type Generator struct {
	client    *openai.Client
	maxTokens int
}

// NewGenerator creates a metadata generator with the given OpenAI client.
// Optional maxTokens parameter sets truncation limit (defaults to DefaultMaxTokens).
func NewGenerator(client *openai.Client, maxTokens ...int) *Generator {
	max := DefaultMaxTokens
	if len(maxTokens) > 0 && maxTokens[0] > 0 {
		max = maxTokens[0]
	}
	return &Generator{
		client:    client,
		maxTokens: max,
	}
}

// GenerateMetadata analyzes document content and produces a summary and entity list.
func (g *Generator) GenerateMetadata(ctx context.Context, path, content string) (*DocumentMetadata, error) {
	// Truncate if too long
	truncated := g.truncateContent(content)

	prompt := fmt.Sprintf(`Analyze this EINO framework documentation and provide:
1. A concise summary (1-2 sentences) capturing the main topic and key points
2. A list of key EINO functions, interfaces, classes, or types mentioned

Document path: %s

Document content:
%s

Respond in JSON format:
{"summary": "Brief description of what this document covers", "entities": ["Entity1", "Entity2"]}

Focus on EINO-specific concepts like:
- Components: ChatModel, Retriever, Embedding, Tool, Callback
- Interfaces: their methods and purposes
- Configuration: options, parameters, settings
- Patterns: chains, agents, flows`, path, truncated)

	resp, err := g.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Model: openai.ChatModelGPT4o,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &openai.ResponseFormatJSONObjectParam{
				Type: "json_object",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	var metadata DocumentMetadata
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &metadata, nil
}

// truncateContent truncates content to fit within token limits.
// Uses rough estimate of 4 characters per token.
func (g *Generator) truncateContent(content string) string {
	// Rough estimate: 1 token ≈ 4 characters
	maxChars := g.maxTokens * 4

	if len(content) <= maxChars {
		return content
	}

	log.Printf("Warning: Truncating content from %d to %d characters (estimated %d tokens)",
		len(content), maxChars, g.maxTokens)

	return content[:maxChars]
}
