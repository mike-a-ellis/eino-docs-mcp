# Phase 3: MCP Server Core - Research

**Researched:** 2026-01-25
**Domain:** Model Context Protocol (MCP) server implementation in Go
**Confidence:** HIGH

## Summary

The Model Context Protocol (MCP) Go SDK provides a mature, well-documented framework for building MCP servers that expose tools, resources, and prompts to AI agents. The official `github.com/modelcontextprotocol/go-sdk` is the standard implementation, maintained in collaboration with Google, and reached v1.0.0 stability.

For Phase 3, we need to implement an MCP server with three tools (`search_docs`, `fetch_doc`, `list_docs`) and one resource (document listing). The server will use stdio transport for local Claude Code integration, with structured input/output types that the SDK automatically converts to/from JSON schemas. The existing Qdrant storage layer integrates cleanly — handlers will call storage methods and return structured results.

The standard pattern is: create server, register tools/resources with typed handlers, run with stdio transport. The SDK handles all protocol complexity (initialization, capability negotiation, schema generation, error marshaling).

**Primary recommendation:** Use official MCP Go SDK with stdio transport, define Go structs for tool inputs/outputs (SDK auto-generates JSON schemas), implement handlers that call existing storage layer, and run with `server.Run(ctx, &mcp.StdioTransport{})`.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/modelcontextprotocol/go-sdk/mcp | v1.0.0+ | MCP protocol implementation | Official SDK, maintained by Google, comprehensive feature coverage |
| github.com/modelcontextprotocol/go-sdk | Latest | Server/client creation and lifecycle | Provides `NewServer`, transport abstractions, tool registration |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Google jsonschema-go | Latest (Jan 2026) | JSON schema generation from structs | If SDK's built-in schema gen is insufficient (unlikely) |
| context | stdlib | Context propagation, cancellation | Required for all handler signatures |
| log/slog | stdlib | Structured logging | Debugging MCP traffic and handler execution |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Official SDK | github.com/mark3labs/mcp-go | Third-party, higher-level API but less mature |
| Stdio transport | Streamable HTTP transport | HTTP enables remote access but adds complexity (auth, TLS) — defer to Phase 4 |
| Auto schema | Manual JSON schema | Full control but defeats SDK automation — unnecessary |

**Installation:**
```bash
go get github.com/modelcontextprotocol/go-sdk@latest
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/
├── mcp-server/          # MCP server entrypoint
│   └── main.go          # Server initialization, Run()
internal/
├── mcp/                 # MCP-specific layer
│   ├── server.go        # Server setup, tool/resource registration
│   ├── handlers.go      # Tool handlers (search, fetch, list)
│   └── types.go         # Input/output structs with jsonschema tags
├── storage/             # Existing Qdrant layer (unchanged)
├── indexer/             # Existing indexing pipeline (unchanged)
```

### Pattern 1: Server Initialization

**What:** Create MCP server with implementation metadata, register tools/resources, run with transport
**When to use:** Main entry point for MCP server

**Example:**
```go
// Source: https://github.com/modelcontextprotocol/go-sdk (README.md)
func main() {
    ctx := context.Background()

    // Create server with identity
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "eino-documentation-server",
        Version: "v0.1.0",
    }, nil)

    // Register tools
    mcp.AddTool(server, &mcp.Tool{
        Name:        "search_docs",
        Description: "Search EINO documentation semantically",
    }, SearchDocsHandler)

    // Register resources
    server.AddResource(&mcp.Resource{
        Name:     "doc_listing",
        URI:      "eino://docs/list",
        MIMEType: "application/json",
    }, DocListingHandler)

    // Run with stdio transport (blocks until client disconnects)
    if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
        log.Fatal(err)
    }
}
```

### Pattern 2: Tool Handler with Structured I/O

**What:** Define input/output structs with jsonschema tags, SDK auto-generates schemas and validates
**When to use:** Every tool implementation

**Example:**
```go
// Source: MCP Go SDK docs/server.md
type SearchDocsInput struct {
    Query      string  `json:"query" jsonschema:"required,description=Search query"`
    MaxResults int     `json:"max_results,omitempty" jsonschema:"minimum=1,maximum=20,default=5"`
    MinScore   float64 `json:"min_score,omitempty" jsonschema:"minimum=0,maximum=1,default=0.5"`
}

type SearchDocsOutput struct {
    Results []DocumentResult `json:"results"`
    Message string          `json:"message,omitempty"`
}

type DocumentResult struct {
    Path       string    `json:"path"`
    Score      float64   `json:"score"`
    Summary    string    `json:"summary"`
    Entities   []string  `json:"entities"`
    UpdatedAt  time.Time `json:"updated_at"`
}

func SearchDocsHandler(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input SearchDocsInput,
) (*mcp.CallToolResult, SearchDocsOutput, error) {
    // SDK already validated input against schema
    // Call existing storage layer
    chunks, err := storage.SearchChunks(ctx, embedding, input.MaxResults, "cloudwego/eino")
    if err != nil {
        return nil, SearchDocsOutput{}, fmt.Errorf("search failed: %w", err)
    }

    // Return structured output (SDK marshals to JSON)
    return nil, SearchDocsOutput{Results: results}, nil
}
```

### Pattern 3: Resource Handler for Browsing

**What:** Expose data via URIs that clients can read (document listing)
**When to use:** When data should be browseable, not just callable

**Example:**
```go
// Source: MCP Go SDK docs/server.md
func DocListingHandler(
    ctx context.Context,
    req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
    // Query storage for all document paths
    paths, err := getAllDocumentPaths(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list docs: %w", err)
    }

    // Return as JSON resource content
    listing, _ := json.Marshal(paths)
    return &mcp.ReadResourceResult{
        Contents: []*mcp.ResourceContents{{
            URI:      req.Params.URI,
            MIMEType: "application/json",
            Text:     string(listing),
        }},
    }, nil
}
```

### Pattern 4: Error Handling in Handlers

**What:** Return Go errors from handlers, SDK marshals to MCP error responses
**When to use:** All error conditions in handlers

**Example:**
```go
// Return standard Go errors — SDK handles protocol details
if documentID == "" {
    return nil, Output{}, fmt.Errorf("document not found")
}

// For MCP-specific errors, use SDK error constructors
if !found {
    return nil, Output{}, mcp.ResourceNotFoundError(uri)
}
```

### Anti-Patterns to Avoid

- **Manual JSON schema generation:** SDK infers schemas from Go structs automatically. Don't hand-write schemas unless absolutely necessary.
- **Session management:** SDK handles sessions internally. Don't try to track client connections manually.
- **Blocking in handlers:** Context cancellation from client propagates to handlers. Don't ignore `ctx` — use it for storage calls.
- **Returning errors as success:** If operation fails, return Go error. Don't return success with error message in output struct.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON schema from structs | Custom reflection code | SDK's built-in schema inference with jsonschema tags | Handles nested types, validation rules, required fields automatically |
| Input validation | Manual JSON parsing and checks | SDK schema validation | Validates before handler runs, returns proper error codes |
| MCP protocol messages | Custom JSON-RPC | SDK transport layer | Handles initialize, capabilities, tool calls, errors per spec |
| Graceful shutdown | Signal handlers, cleanup code | `server.Run()` with context | SDK handles SIGTERM/SIGINT, closes transport cleanly |
| Streaming responses | Custom chunking logic | Not needed for Phase 3 | Full documents returned, no chunking required |

**Key insight:** MCP protocol is complex (initialize handshake, capability negotiation, progress tokens, cancellation). The SDK encapsulates this completely — fighting it leads to protocol violations and client incompatibility.

## Common Pitfalls

### Pitfall 1: Ignoring Context Cancellation

**What goes wrong:** Client disconnects or times out, but handler keeps running and queries database
**Why it happens:** Go handlers often ignore `ctx` parameter when it seems unused
**How to avoid:** Pass context to all storage operations. Qdrant client respects cancellation.
**Warning signs:** Server logs show operations completing after client has disconnected

```go
// BAD: Context ignored
func Handler(ctx context.Context, req *mcp.CallToolRequest, in Input) {
    results, _ := storage.Search(context.Background(), query) // Ignores cancellation!
    return results
}

// GOOD: Context propagated
func Handler(ctx context.Context, req *mcp.CallToolRequest, in Input) {
    results, err := storage.Search(ctx, query) // Cancelled if client disconnects
    return results, err
}
```

### Pitfall 2: Large Responses Without Streaming

**What goes wrong:** Returning 10+ full markdown documents (potentially hundreds of KB) blocks until complete
**Why it happens:** Assuming stdio transport buffers infinitely, or that clients don't timeout
**How to avoid:** Limit `max_results` with jsonschema constraints, return metadata-only results for search
**Warning signs:** Client timeouts, slow tool responses

**Note:** For Phase 3, this is mitigated by CONTEXT.md decision to return metadata in search, full content via fetch_doc.

### Pitfall 3: Stdio Transport Lifecycle Confusion

**What goes wrong:** Server exits before client disconnects, or hangs after client closes
**Why it happens:** Misunderstanding `server.Run()` blocking behavior and shutdown signals
**How to avoid:** `server.Run()` blocks until stdin closes. Use SIGTERM for graceful shutdown. Context cancellation propagates.
**Warning signs:** Server process doesn't exit cleanly, orphaned processes

```go
// CORRECT: server.Run() blocks until client disconnects or SIGTERM
func main() {
    ctx := context.Background()
    server := mcp.NewServer(impl, nil)
    // ... register tools ...

    // Blocks here until stdin closes or SIGTERM
    if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
        log.Fatal(err)
    }
    // Cleanup happens after Run() returns
}
```

### Pitfall 4: Schema Mismatch with Client Expectations

**What goes wrong:** Client receives different field names or types than expected, fails silently
**Why it happens:** Changing struct field names without json tags, or mismatched types
**How to avoid:** Use explicit `json` tags on all struct fields. Test with MCP Inspector.
**Warning signs:** Tools appear in client but fail with "invalid arguments" errors

```go
// BAD: Implicit field names, Go conventions don't match JSON conventions
type Output struct {
    DocumentPath string  // Becomes "DocumentPath" in JSON
    LastUpdated  string  // Should be time.Time
}

// GOOD: Explicit JSON tags, proper types
type Output struct {
    DocumentPath string    `json:"document_path"` // snake_case for JSON
    LastUpdated  time.Time `json:"last_updated"`  // Marshals to ISO 8601
}
```

### Pitfall 5: Error Information Loss

**What goes wrong:** Generic "operation failed" errors don't help agent understand what went wrong
**Why it happens:** Returning `fmt.Errorf("search failed")` without context
**How to avoid:** Return descriptive errors with context. SDK marshals to MCP error with message.
**Warning signs:** Agent retries same failing operation, or gives up when alternative approach would work

```go
// BAD: Loses context
if len(results) == 0 {
    return nil, Output{}, fmt.Errorf("search failed")
}

// GOOD: Actionable error message
if len(results) == 0 {
    return nil, Output{
        Message: "No matching documents found. Try broader terms.",
    }, nil // Empty results are success, not error
}

// GOOD: Error with context
if err != nil {
    return nil, Output{}, fmt.Errorf("vector search failed for query %q: %w", query, err)
}
```

## Code Examples

Verified patterns from official sources:

### Complete Tool Registration Flow

```go
// Source: github.com/modelcontextprotocol/go-sdk README.md
package main

import (
    "context"
    "log"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Define input/output types
type SearchInput struct {
    Query string `json:"query" jsonschema:"required"`
    Limit int    `json:"limit,omitempty" jsonschema:"minimum=1,maximum=20,default=5"`
}

type SearchOutput struct {
    Results []Result `json:"results"`
    Count   int      `json:"count"`
}

type Result struct {
    Path    string  `json:"path"`
    Score   float64 `json:"score"`
    Summary string  `json:"summary"`
}

// Tool handler
func SearchDocs(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input SearchInput,
) (*mcp.CallToolResult, SearchOutput, error) {
    // Implementation using existing storage layer
    results := performSearch(ctx, input.Query, input.Limit)

    return nil, SearchOutput{
        Results: results,
        Count:   len(results),
    }, nil
}

func main() {
    // Create server
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "eino-documentation-server",
        Version: "v0.1.0",
    }, nil)

    // Register tool
    mcp.AddTool(server, &mcp.Tool{
        Name:        "search_docs",
        Description: "Search EINO documentation",
    }, SearchDocs)

    // Run with stdio (blocks until client disconnects)
    if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
        log.Fatal(err)
    }
}
```

### Multiple Tools Registration

```go
// Source: MCP Go SDK examples/server/everything
func setupServer(storage *storage.QdrantStorage) *mcp.Server {
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "eino-documentation-server",
        Version: "v0.1.0",
    }, nil)

    // Register all three tools
    mcp.AddTool(server, &mcp.Tool{
        Name:        "search_docs",
        Description: "Search EINO documentation semantically",
    }, makeSearchHandler(storage))

    mcp.AddTool(server, &mcp.Tool{
        Name:        "fetch_doc",
        Description: "Retrieve a specific document by path",
    }, makeFetchHandler(storage))

    mcp.AddTool(server, &mcp.Tool{
        Name:        "list_docs",
        Description: "List all available documentation paths",
    }, makeListHandler(storage))

    // Register resource for browseable doc listing
    server.AddResource(&mcp.Resource{
        Name:     "documentation",
        URI:      "eino://docs",
        MIMEType: "application/json",
    }, makeResourceHandler(storage))

    return server
}
```

### Handler with Storage Integration

```go
// Pattern: Handler calls existing storage layer
func makeSearchHandler(storage *storage.QdrantStorage) func(
    context.Context,
    *mcp.CallToolRequest,
    SearchInput,
) (*mcp.CallToolResult, SearchOutput, error) {
    return func(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (
        *mcp.CallToolResult, SearchOutput, error,
    ) {
        // Generate embedding for query (use existing embedder)
        embedding, err := embedder.Embed(ctx, input.Query)
        if err != nil {
            return nil, SearchOutput{}, fmt.Errorf("embedding failed: %w", err)
        }

        // Search using existing storage layer
        chunks, err := storage.SearchChunks(ctx, embedding, input.Limit*3, "cloudwego/eino")
        if err != nil {
            return nil, SearchOutput{}, fmt.Errorf("search failed: %w", err)
        }

        // Get parent documents for top chunks
        docMap := make(map[string]*storage.Document)
        for _, chunk := range chunks {
            if _, seen := docMap[chunk.ParentDocID]; !seen {
                doc, err := storage.GetDocument(ctx, chunk.ParentDocID)
                if err == nil && doc != nil {
                    docMap[chunk.ParentDocID] = doc
                }
            }
        }

        // Convert to output format with scores
        results := make([]SearchResult, 0, len(docMap))
        for _, doc := range docMap {
            results = append(results, SearchResult{
                Path:      doc.Metadata.Path,
                Score:     0.85, // From vector search
                Summary:   doc.Metadata.Summary,
                Entities:  doc.Metadata.Entities,
                UpdatedAt: doc.Metadata.IndexedAt,
            })
        }

        // Return metadata only (per CONTEXT.md decision)
        return nil, SearchOutput{Results: results}, nil
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| SSE (Server-Sent Events) transport | Streamable HTTP transport | Deprecated 2025 | Use stdio for local, HTTP for remote |
| Manual JSON schema definitions | Struct tags with auto-generation | SDK v1.0 | Less boilerplate, type-safe |
| Custom transport implementations | Official transports (stdio, HTTP) | SDK stabilization | Better compatibility |
| OAuth 2.0 for HTTP | OAuth 2.1 mandatory | March 2025 | Not applicable to stdio (Phase 3) |

**Deprecated/outdated:**
- **SSE transport:** Replaced by Streamable HTTP. Don't use for new servers.
- **EventStore.Open:** No-op API artifact from pre-v1.0, implement as empty method if required.
- **Custom transports:** Unless specific need, use stdio (local) or HTTP (remote).

## Open Questions

Things that couldn't be fully resolved:

1. **Similarity score access from Qdrant**
   - What we know: `storage.SearchChunks()` returns chunks but doesn't expose similarity scores from Qdrant Query API
   - What's unclear: Whether go-client returns scores in response, or if we need to modify storage layer
   - Recommendation: Check `qdrant.ScoredPoint` in Query response, update `SearchChunks()` to return `(chunk, score)` tuples

2. **Document path listing from Qdrant**
   - What we know: No existing method to get all document paths efficiently
   - What's unclear: Best approach — Scroll API with filter, or maintain separate index
   - Recommendation: Use Scroll with `type="parent"` filter, extract paths from payload. Cache if performance issue.

3. **Server info metadata exposure**
   - What we know: CONTEXT.md requires exposing indexed commit SHA in server info
   - What's unclear: How to set server info beyond `Implementation.Name/Version`
   - Recommendation: Research `ServerOptions` or use tool to query metadata, not server info

## Sources

### Primary (HIGH confidence)

- [MCP Go SDK Official Repository](https://github.com/modelcontextprotocol/go-sdk) - Official SDK implementation
- [MCP Go SDK Documentation - Server](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/docs/server.md) - Server features and patterns
- [MCP Go SDK Documentation - Protocol](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/docs/protocol.md) - Lifecycle and transports
- [MCP Go SDK Documentation - Troubleshooting](https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/docs/troubleshooting.md) - Debugging techniques
- [MCP Go SDK Package Docs](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) - API reference
- [MCP Specification - Lifecycle](https://modelcontextprotocol.io/specification/2025-03-26/basic/lifecycle) - Protocol lifecycle

### Secondary (MEDIUM confidence)

- [Architecture Overview - Model Context Protocol](https://modelcontextprotocol.io/docs/learn/architecture) - Transport mechanisms
- [MCP Transport Future Blog](http://blog.modelcontextprotocol.io/posts/2025-12-19-mcp-transport-future/) - Stdio vs HTTP guidance
- [Google's JSON Schema Package](https://opensource.googleblog.com/2026/01/a-json-schema-package-for-go.html) - Schema generation (if needed)
- [Building MCP Server in Go Tutorial](https://navendu.me/posts/mcp-server-go/) - Implementation walkthrough
- [MCP Server Best Practices 2026](https://www.cdata.com/blog/mcp-server-best-practices-2026) - Production patterns
- [15 Best Practices for MCP Servers](https://thenewstack.io/15-best-practices-for-building-mcp-servers-in-production/) - Security and operations
- [MCP Security Guide](https://towardsdatascience.com/the-mcp-security-survival-guide-best-practices-pitfalls-and-real-world-lessons/) - Security patterns

### Tertiary (LOW confidence)

- [Qdrant MCP Server](https://github.com/qdrant/mcp-server-qdrant) - Example vector search MCP server (TypeScript)
- [MCP + Milvus Documentation](https://milvus.io/docs/milvus_and_mcp.md) - Vector DB integration patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official SDK is well-documented, stable v1.0+, maintained by Google
- Architecture: HIGH - Multiple official examples, clear patterns from docs and examples
- Pitfalls: MEDIUM - Derived from troubleshooting docs and general Go practices, not phase-specific

**Research date:** 2026-01-25
**Valid until:** February 2026 (30 days - SDK is stable, slow-moving)
