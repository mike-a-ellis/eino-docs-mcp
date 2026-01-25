package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"
)

// Chunk represents a section of a markdown document with header context.
type Chunk struct {
	Index      int    // Position in document (0, 1, 2...)
	HeaderPath string // Hierarchy: "# Doc Title > ## Section Name"
	Content    string // Chunk content WITH header path prepended
	RawContent string // Original content without header prefix
}

// Chunker splits markdown documents at header boundaries while preserving context.
type Chunker struct {
	parser goldmark.Markdown
}

// NewChunker creates a new markdown chunker configured with goldmark parser.
func NewChunker() *Chunker {
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)
	return &Chunker{
		parser: md,
	}
}

// ChunkDocument splits markdown at H1 and H2 boundaries with header hierarchy preservation.
// Each chunk includes prepended header path for context during retrieval.
func (c *Chunker) ChunkDocument(source []byte) ([]Chunk, error) {
	// Parse markdown to AST
	reader := text.NewReader(source)
	doc := c.parser.Parser().Parse(reader)

	// Extract TOC with hierarchy
	tree, err := toc.Inspect(doc, source,
		toc.MinDepth(1),   // Include H1
		toc.MaxDepth(2),   // Split at H1 and H2 only
		toc.Compact(true), // Remove empty items
	)
	if err != nil {
		return nil, fmt.Errorf("inspect TOC: %w", err)
	}

	// If no headers found, return entire content as single chunk
	if len(tree.Items) == 0 {
		return []Chunk{
			{
				Index:      0,
				HeaderPath: "",
				Content:    string(source),
				RawContent: string(source),
			},
		}, nil
	}

	// Extract chunks with header context
	var chunks []Chunk
	c.extractChunks(doc, source, tree.Items, nil, &chunks)

	return chunks, nil
}

// extractChunks recursively walks TOC items to extract content with header paths.
func (c *Chunker) extractChunks(doc ast.Node, source []byte, items toc.Items, ancestors []string, chunks *[]Chunk) {
	for i, item := range items {
		// Build header path for this item
		currentPath := append(ancestors, string(item.Title))
		headerPath := formatHeaderPath(currentPath)

		// Find the header node in AST
		headerNode := findHeaderByID(doc, string(item.ID))
		if headerNode == nil {
			continue
		}

		// Determine content boundaries
		startLine := headerNode.Lines().At(0)
		var endLine text.Segment

		// Find end boundary: next sibling header or parent's next sibling
		if i+1 < len(items) {
			// Next sibling exists
			nextHeader := findHeaderByID(doc, string(items[i+1].ID))
			if nextHeader != nil {
				endLine = nextHeader.Lines().At(0)
			}
		} else {
			// This is last item at current level, find end by walking to next H1/H2
			endLine = findNextHeaderBoundary(doc, headerNode, headerNode.(*ast.Heading).Level)
		}

		// Extract content
		content := extractContent(source, startLine, endLine)

		// Create chunk with prepended header path
		chunk := Chunk{
			Index:      len(*chunks),
			HeaderPath: headerPath,
			RawContent: content,
			Content:    fmt.Sprintf("%s\n\n%s", headerPath, content),
		}
		*chunks = append(*chunks, chunk)

		// Process children (H2 under H1)
		if len(item.Items) > 0 {
			c.extractChunks(doc, source, item.Items, currentPath, chunks)
		}
	}
}

// formatHeaderPath builds a header hierarchy string.
// Example: ["Installation", "Prerequisites"] -> "# Installation > ## Prerequisites"
func formatHeaderPath(path []string) string {
	if len(path) == 0 {
		return ""
	}

	var parts []string
	for i, segment := range path {
		// Add appropriate number of # based on depth
		prefix := strings.Repeat("#", i+1)
		parts = append(parts, fmt.Sprintf("%s %s", prefix, segment))
	}

	return strings.Join(parts, " > ")
}

// findHeaderByID locates a heading node by its auto-generated ID.
func findHeaderByID(node ast.Node, id string) ast.Node {
	var found ast.Node
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			// Check if this heading has the target ID
			headingID, ok := heading.AttributeString("id")
			if ok && string(headingID.([]byte)) == id {
				found = n
				return ast.WalkStop, nil
			}
		}
		return ast.WalkContinue, nil
	})
	return found
}

// findNextHeaderBoundary finds the next H1 or H2 header after the given node.
func findNextHeaderBoundary(root ast.Node, current ast.Node, currentLevel int) text.Segment {
	var nextHeader ast.Node
	foundCurrent := false

	ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)

			// Skip until we find current node
			if !foundCurrent {
				if n == current {
					foundCurrent = true
				}
				return ast.WalkContinue, nil
			}

			// Found a header after current - check if it's a boundary (same or higher level)
			if heading.Level <= currentLevel {
				nextHeader = n
				return ast.WalkStop, nil
			}
		}

		return ast.WalkContinue, nil
	})

	if nextHeader != nil {
		return nextHeader.Lines().At(0)
	}

	// No next header found - return empty segment (will extract to EOF)
	return text.Segment{}
}

// extractContent extracts text between start and end line segments.
func extractContent(source []byte, start text.Segment, end text.Segment) string {
	var buf bytes.Buffer

	// If no end boundary, extract to end of document
	if end.Start == 0 && end.Stop == 0 {
		buf.Write(source[start.Start:])
	} else {
		buf.Write(source[start.Start:end.Start])
	}

	return strings.TrimSpace(buf.String())
}
