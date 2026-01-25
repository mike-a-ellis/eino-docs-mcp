// Package mcp provides MCP server implementation for EINO documentation.
package mcp

import "time"

// SearchDocsInput defines the input parameters for the search_docs tool.
// Query is required (no omitempty), MaxResults and MinScore are optional.
type SearchDocsInput struct {
	// Query is the semantic search query.
	Query string `json:"query" jsonschema:"The semantic search query for finding relevant documentation"`
	// MaxResults is the maximum number of documents to return (1-20, default 5).
	MaxResults int `json:"max_results,omitempty" jsonschema:"Maximum number of documents to return (1-20, default 5)"`
	// MinScore is the minimum relevance threshold (0-1, default 0.5).
	MinScore float64 `json:"min_score,omitempty" jsonschema:"Minimum relevance score threshold (0-1, default 0.5)"`
}

// SearchDocsOutput contains the search results.
type SearchDocsOutput struct {
	// Results is the list of matching documents with metadata.
	Results []SearchResult `json:"results"`
	// Message provides informational context (e.g., "No matching documents found").
	Message string `json:"message,omitempty"`
}

// SearchResult represents a single document match from semantic search.
type SearchResult struct {
	// Path is the document path (e.g., "getting-started/installation.md").
	Path string `json:"path"`
	// Score is the similarity score (0-1).
	Score float64 `json:"score"`
	// Summary is the LLM-generated document summary.
	Summary string `json:"summary"`
	// Entities lists extracted functions/methods from the document.
	Entities []string `json:"entities"`
	// UpdatedAt is when the document was last indexed.
	UpdatedAt time.Time `json:"updated_at"`
}

// FetchDocInput defines the input parameters for the fetch_doc tool.
// Path is required (no omitempty).
type FetchDocInput struct {
	// Path is the document path to retrieve.
	Path string `json:"path" jsonschema:"The document path to retrieve (e.g. getting-started/installation.md)"`
}

// FetchDocOutput contains the retrieved document.
type FetchDocOutput struct {
	// Content is the full markdown content with source header prepended.
	Content string `json:"content"`
	// Path is the document path.
	Path string `json:"path"`
	// Summary is the LLM-generated document summary.
	Summary string `json:"summary"`
	// UpdatedAt is when the document was indexed.
	UpdatedAt time.Time `json:"updated_at"`
	// Found indicates whether the document exists.
	Found bool `json:"found"`
}

// ListDocsInput defines the input parameters for the list_docs tool.
// This tool takes no parameters and lists all available documents.
type ListDocsInput struct {
	// No input parameters required
}

// ListDocsOutput contains the list of all available document paths.
type ListDocsOutput struct {
	// Paths is all available document paths.
	Paths []string `json:"paths"`
	// Count is the total number of documents.
	Count int `json:"count"`
}

// StatusInput defines input for get_index_status tool (no parameters required)
type StatusInput struct{}

// StatusOutput contains index status information
type StatusOutput struct {
	// TotalDocs is the count of indexed documents
	TotalDocs int `json:"total_docs"`
	// TotalChunks is the count of document chunks
	TotalChunks int `json:"total_chunks"`
	// IndexedPaths lists all document paths in the index
	IndexedPaths []string `json:"indexed_paths"`
	// LastSyncTime is when the index was last updated (RFC3339)
	LastSyncTime string `json:"last_sync_time"`
	// SourceCommit is the GitHub commit SHA of indexed content
	SourceCommit string `json:"source_commit"`
	// CommitsBehind shows how many commits the index is behind GitHub HEAD (null if check failed)
	CommitsBehind *int `json:"commits_behind"`
	// StaleWarning is set when index is >20 commits behind
	StaleWarning string `json:"stale_warning,omitempty"`
}
