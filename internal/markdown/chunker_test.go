package markdown

import (
	"strings"
	"testing"
)

// TestChunkDocument_BasicHeaders tests chunking with H1 and multiple H2s.
func TestChunkDocument_BasicHeaders(t *testing.T) {
	input := `# Getting Started

Introduction text here.

## Installation

Install steps here.

## Configuration

Config details here.
`

	chunker := NewChunker()
	chunks, err := chunker.ChunkDocument([]byte(input))
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// Expect 3 chunks: H1, H1>H2 Installation, H1>H2 Configuration
	if len(chunks) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(chunks))
	}

	// Verify first chunk (H1)
	if chunks[0].Index != 0 {
		t.Errorf("Chunk 0 index: expected 0, got %d", chunks[0].Index)
	}
	if chunks[0].HeaderPath != "# Getting Started" {
		t.Errorf("Chunk 0 HeaderPath: expected '# Getting Started', got %q", chunks[0].HeaderPath)
	}
	if !strings.Contains(chunks[0].RawContent, "Introduction text here") {
		t.Errorf("Chunk 0 missing expected content")
	}

	// Verify second chunk (H2 Installation)
	if chunks[1].Index != 1 {
		t.Errorf("Chunk 1 index: expected 1, got %d", chunks[1].Index)
	}
	expectedPath := "# Getting Started > ## Installation"
	if chunks[1].HeaderPath != expectedPath {
		t.Errorf("Chunk 1 HeaderPath: expected %q, got %q", expectedPath, chunks[1].HeaderPath)
	}
	if !strings.Contains(chunks[1].RawContent, "Install steps here") {
		t.Errorf("Chunk 1 missing expected content")
	}

	// Verify third chunk (H2 Configuration)
	if chunks[2].Index != 2 {
		t.Errorf("Chunk 2 index: expected 2, got %d", chunks[2].Index)
	}
	expectedPath = "# Getting Started > ## Configuration"
	if chunks[2].HeaderPath != expectedPath {
		t.Errorf("Chunk 2 HeaderPath: expected %q, got %q", expectedPath, chunks[2].HeaderPath)
	}
	if !strings.Contains(chunks[2].RawContent, "Config details here") {
		t.Errorf("Chunk 2 missing expected content")
	}
}

// TestChunkDocument_NestedContent tests that complex content is preserved.
func TestChunkDocument_NestedContent(t *testing.T) {
	input := `# API Reference

Overview of the API.

## Methods

Available methods:

` + "```go" + `
func DoSomething() error {
    return nil
}
` + "```" + `

### Details

Some details here.

- List item 1
- List item 2
`

	chunker := NewChunker()
	chunks, err := chunker.ChunkDocument([]byte(input))
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// Should have 2 chunks (H1 and H2) - H3 is not a split boundary
	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunks))
	}

	// Verify second chunk contains code block and list
	methodsChunk := chunks[1]
	if !strings.Contains(methodsChunk.RawContent, "func DoSomething()") {
		t.Errorf("Methods chunk missing code block")
	}
	if !strings.Contains(methodsChunk.RawContent, "List item 1") {
		t.Errorf("Methods chunk missing list content")
	}
	if !strings.Contains(methodsChunk.RawContent, "### Details") {
		t.Errorf("Methods chunk missing H3 subsection")
	}
}

// TestChunkDocument_HeaderHierarchy tests header path formatting.
func TestChunkDocument_HeaderHierarchy(t *testing.T) {
	input := `# Installation

General info.

## Prerequisites

Need these first.

## Steps

Do this next.
`

	chunker := NewChunker()
	chunks, err := chunker.ChunkDocument([]byte(input))
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	expectedPaths := []string{
		"# Installation",
		"# Installation > ## Prerequisites",
		"# Installation > ## Steps",
	}

	if len(chunks) != len(expectedPaths) {
		t.Fatalf("Expected %d chunks, got %d", len(expectedPaths), len(chunks))
	}

	for i, expectedPath := range expectedPaths {
		if chunks[i].HeaderPath != expectedPath {
			t.Errorf("Chunk %d HeaderPath: expected %q, got %q", i, expectedPath, chunks[i].HeaderPath)
		}
	}
}

// TestChunkDocument_SingleSection tests document with no headers.
func TestChunkDocument_SingleSection(t *testing.T) {
	input := `This is a document with no headers.

Just plain text content.
`

	chunker := NewChunker()
	chunks, err := chunker.ChunkDocument([]byte(input))
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// Should return single chunk with empty header path
	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0].HeaderPath != "" {
		t.Errorf("Expected empty HeaderPath, got %q", chunks[0].HeaderPath)
	}

	if !strings.Contains(chunks[0].RawContent, "This is a document") {
		t.Errorf("Chunk missing expected content")
	}
}

// TestChunkDocument_PrependedContent verifies header path is prepended to Content field.
func TestChunkDocument_PrependedContent(t *testing.T) {
	input := `# Title

Some content.

## Section

Section content.
`

	chunker := NewChunker()
	chunks, err := chunker.ChunkDocument([]byte(input))
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// Check first chunk has prepended header
	if !strings.HasPrefix(chunks[0].Content, "# Title\n\n") {
		t.Errorf("Chunk 0 Content doesn't start with header path")
	}
	if !strings.Contains(chunks[0].Content, "Some content") {
		t.Errorf("Chunk 0 Content missing actual content")
	}

	// Check second chunk has prepended hierarchy
	expectedPrefix := "# Title > ## Section\n\n"
	if !strings.HasPrefix(chunks[1].Content, expectedPrefix) {
		t.Errorf("Chunk 1 Content doesn't start with expected header path")
		t.Logf("Expected prefix: %q", expectedPrefix)
		t.Logf("Actual content: %q", chunks[1].Content[:50])
	}
	if !strings.Contains(chunks[1].Content, "Section content") {
		t.Errorf("Chunk 1 Content missing actual content")
	}

	// Verify RawContent doesn't have prepended header
	if strings.HasPrefix(chunks[1].RawContent, "# Title") {
		t.Errorf("RawContent should not have prepended header")
	}
}

// TestChunkDocument_MultipleH1s tests multiple top-level sections.
func TestChunkDocument_MultipleH1s(t *testing.T) {
	input := `# First Section

First content.

## First Subsection

First subsection content.

# Second Section

Second content.

## Second Subsection

Second subsection content.
`

	chunker := NewChunker()
	chunks, err := chunker.ChunkDocument([]byte(input))
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// Should have 4 chunks total
	if len(chunks) != 4 {
		t.Errorf("Expected 4 chunks, got %d", len(chunks))
	}

	// Verify hierarchy paths
	expectedPaths := []string{
		"# First Section",
		"# First Section > ## First Subsection",
		"# Second Section",
		"# Second Section > ## Second Subsection",
	}

	for i, expectedPath := range expectedPaths {
		if chunks[i].HeaderPath != expectedPath {
			t.Errorf("Chunk %d: expected path %q, got %q", i, expectedPath, chunks[i].HeaderPath)
		}
	}
}

// TestChunkDocument_EmptySections tests handling of headers with no content.
func TestChunkDocument_EmptySections(t *testing.T) {
	input := `# Title

## Empty Section

## Another Section

Some content here.
`

	chunker := NewChunker()
	chunks, err := chunker.ChunkDocument([]byte(input))
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// With toc.Compact(true), empty sections should still be included
	// but may have minimal content
	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify we have the sections we expect
	foundAnother := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.HeaderPath, "Another Section") {
			foundAnother = true
			if !strings.Contains(chunk.RawContent, "Some content here") {
				t.Errorf("'Another Section' chunk missing expected content")
			}
		}
	}

	if !foundAnother {
		t.Error("Did not find 'Another Section' chunk")
	}
}
