package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/bull/eino-mcp-server/internal/embedding"
	"github.com/bull/eino-mcp-server/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultRepository = "cloudwego/cloudwego.github.io"

// makeSearchHandler creates the search_docs tool handler.
// Search flow:
// 1. Generate embedding for query text
// 2. Search chunks with vector similarity (limit * 3 to get enough parents)
// 3. Filter by minimum score threshold
// 4. Deduplicate by parent document (keep highest-scoring chunk per doc)
// 5. Fetch parent document metadata for each unique doc
// 6. Return up to MaxResults documents with metadata (not content)
func makeSearchHandler(store *storage.QdrantStorage, embedder *embedding.Embedder) func(
	context.Context, *mcp.CallToolRequest, SearchDocsInput,
) (*mcp.CallToolResult, SearchDocsOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input SearchDocsInput) (
		*mcp.CallToolResult, SearchDocsOutput, error,
	) {
		// Apply defaults
		maxResults := input.MaxResults
		if maxResults <= 0 {
			maxResults = 5
		}
		minScore := input.MinScore
		if minScore <= 0 {
			minScore = 0.5
		}

		// Generate embedding for query
		embeddings, err := embedder.GenerateEmbeddings(ctx, []string{input.Query})
		if err != nil {
			return nil, SearchDocsOutput{}, fmt.Errorf("failed to embed query: %w", err)
		}
		queryEmbedding := embeddings[0]

		// Search chunks (request 3x to ensure enough unique documents after dedup)
		chunks, err := store.SearchChunksWithScores(ctx, queryEmbedding, maxResults*3, defaultRepository)
		if err != nil {
			return nil, SearchDocsOutput{}, fmt.Errorf("search failed: %w", err)
		}

		// Deduplicate by parent document, keeping highest score per doc
		docScores := make(map[string]float64) // docID -> highest score
		docIDs := make([]string, 0)           // preserve order
		for _, chunk := range chunks {
			if chunk.Score < minScore {
				continue // Below threshold
			}
			if existing, seen := docScores[chunk.ParentDocID]; !seen || chunk.Score > existing {
				if !seen {
					docIDs = append(docIDs, chunk.ParentDocID)
				}
				docScores[chunk.ParentDocID] = chunk.Score
			}
		}

		// Limit to maxResults
		if len(docIDs) > maxResults {
			docIDs = docIDs[:maxResults]
		}

		// Fetch document metadata for each unique document
		results := make([]SearchResult, 0, len(docIDs))
		for _, docID := range docIDs {
			doc, err := store.GetDocument(ctx, docID)
			if err != nil {
				continue // Skip documents that fail to load
			}
			results = append(results, SearchResult{
				Path:      doc.Metadata.Path,
				Score:     docScores[docID],
				Summary:   doc.Metadata.Summary,
				Entities:  doc.Metadata.Entities,
				UpdatedAt: doc.Metadata.IndexedAt,
			})
		}

		if len(results) == 0 {
			return nil, SearchDocsOutput{
				Results: []SearchResult{},
				Message: "No matching documents found. Try broader search terms.",
			}, nil
		}

		return nil, SearchDocsOutput{Results: results}, nil
	}
}

// makeFetchHandler creates the fetch_doc tool handler.
// Retrieves full document content by path.
// Prepends source header: <!-- Source: path/to/doc.md -->
func makeFetchHandler(store *storage.QdrantStorage) func(
	context.Context, *mcp.CallToolRequest, FetchDocInput,
) (*mcp.CallToolResult, FetchDocOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input FetchDocInput) (
		*mcp.CallToolResult, FetchDocOutput, error,
	) {
		doc, err := store.GetDocumentByPath(ctx, input.Path, defaultRepository)
		if err != nil {
			// Return helpful response for not found
			if errors.Is(err, storage.ErrDocumentNotFound) {
				return nil, FetchDocOutput{
					Found: false,
					Path:  input.Path,
				}, nil
			}
			return nil, FetchDocOutput{}, fmt.Errorf("failed to fetch document: %w", err)
		}

		// Prepend source header
		content := fmt.Sprintf("<!-- Source: %s -->\n\n%s", doc.Metadata.Path, doc.Content)

		return nil, FetchDocOutput{
			Content:   content,
			Path:      doc.Metadata.Path,
			Summary:   doc.Metadata.Summary,
			UpdatedAt: doc.Metadata.IndexedAt,
			Found:     true,
		}, nil
	}
}

// makeListHandler creates the list_docs tool handler.
// Returns all available document paths.
func makeListHandler(store *storage.QdrantStorage) func(
	context.Context, *mcp.CallToolRequest, ListDocsInput,
) (*mcp.CallToolResult, ListDocsOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListDocsInput) (
		*mcp.CallToolResult, ListDocsOutput, error,
	) {
		paths, err := store.ListDocumentPaths(ctx, defaultRepository)
		if err != nil {
			return nil, ListDocsOutput{}, fmt.Errorf("failed to list documents: %w", err)
		}

		return nil, ListDocsOutput{
			Paths: paths,
			Count: len(paths),
		}, nil
	}
}
