package metadata

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestParseMetadataResponse verifies JSON parsing of valid response.
func TestParseMetadataResponse(t *testing.T) {
	jsonResponse := `{"summary": "Test summary", "entities": ["Entity1", "Entity2"]}`

	var metadata DocumentMetadata
	err := json.Unmarshal([]byte(jsonResponse), &metadata)
	if err != nil {
		t.Fatalf("Failed to parse valid JSON response: %v", err)
	}

	// Verify summary
	if metadata.Summary != "Test summary" {
		t.Errorf("Expected summary 'Test summary', got '%s'", metadata.Summary)
	}

	// Verify entities
	if len(metadata.Entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(metadata.Entities))
	}
	if metadata.Entities[0] != "Entity1" {
		t.Errorf("Expected first entity 'Entity1', got '%s'", metadata.Entities[0])
	}
	if metadata.Entities[1] != "Entity2" {
		t.Errorf("Expected second entity 'Entity2', got '%s'", metadata.Entities[1])
	}
}

// TestTruncateContent verifies truncation works correctly for very long content.
func TestTruncateContent(t *testing.T) {
	// Create a generator with default max tokens (16000)
	g := &Generator{
		maxTokens: DefaultMaxTokens,
	}

	// Create very long string (100k chars, well over 16k tokens)
	longContent := strings.Repeat("This is a test content. ", 4000) // ~100k chars

	truncated := g.truncateContent(longContent)

	// Expected max chars: 16000 tokens * 4 chars/token = 64000 chars
	expectedMaxChars := DefaultMaxTokens * 4
	if len(truncated) != expectedMaxChars {
		t.Errorf("Expected truncated length %d, got %d", expectedMaxChars, len(truncated))
	}

	// Verify it's a prefix of the original
	if !strings.HasPrefix(longContent, truncated) {
		t.Error("Truncated content should be a prefix of original content")
	}
}

// TestTruncateContent_Short verifies short content is not truncated.
func TestTruncateContent_Short(t *testing.T) {
	g := &Generator{
		maxTokens: DefaultMaxTokens,
	}

	// Short content (1000 chars, well under limit)
	shortContent := strings.Repeat("Short. ", 140) // ~1000 chars

	truncated := g.truncateContent(shortContent)

	// Content should be unchanged
	if truncated != shortContent {
		t.Error("Short content should not be truncated")
	}
	if len(truncated) != len(shortContent) {
		t.Errorf("Expected length %d, got %d", len(shortContent), len(truncated))
	}
}

// TestTruncateContent_CustomMaxTokens verifies custom max tokens setting.
func TestTruncateContent_CustomMaxTokens(t *testing.T) {
	// Create generator with custom max tokens
	customMaxTokens := 1000
	g := &Generator{
		maxTokens: customMaxTokens,
	}

	// Create content that exceeds custom limit
	content := strings.Repeat("Content. ", 1000) // ~9000 chars

	truncated := g.truncateContent(content)

	// Expected max chars: 1000 tokens * 4 chars/token = 4000 chars
	expectedMaxChars := customMaxTokens * 4
	if len(truncated) != expectedMaxChars {
		t.Errorf("Expected truncated length %d, got %d", expectedMaxChars, len(truncated))
	}
}
