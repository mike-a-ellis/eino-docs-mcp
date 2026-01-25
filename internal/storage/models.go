package storage

import "time"

// Document represents a full markdown document stored in Qdrant.
// Documents have no embedding vector - they exist for full-content retrieval.
type Document struct {
	ID       string           // UUID
	Content  string           // Full markdown content
	Metadata DocumentMetadata
}

// DocumentMetadata contains indexing metadata for a document.
type DocumentMetadata struct {
	Path       string    // Relative path: "getting-started/installation.md"
	URL        string    // GitHub raw URL for source
	Repository string    // Full repo path: "cloudwego/eino"
	CommitSHA  string    // Git commit SHA when indexed
	IndexedAt  time.Time // When this version was indexed
	Summary    string    // LLM-generated summary (populated in Phase 2)
	Entities   []string  // Extracted functions/methods (populated in Phase 2)
}

// Chunk represents a document section with an embedding vector.
// Chunks are used for semantic search, then parent document is retrieved.
type Chunk struct {
	ID          string    // UUID
	ParentDocID string    // Links to parent Document.ID
	ChunkIndex  int       // Position in document (0, 1, 2...)
	HeaderPath  string    // Section hierarchy: "Installation > Prerequisites"
	Content     string    // Chunk text content
	Path        string    // Same as parent document path (for filtering)
	Repository  string    // Same as parent (for filtering)
	Embedding   []float32 // 1536-dim vector (text-embedding-3-small)
}

// CollectionName is the single Qdrant collection for all documents.
const CollectionName = "documents"

// VectorDimension is the embedding size for text-embedding-3-small.
const VectorDimension = 1536
