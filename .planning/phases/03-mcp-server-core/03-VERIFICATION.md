---
phase: 03-mcp-server-core
verified: 2026-01-25T20:16:49Z
status: gaps_found
score: 4/5 must-haves verified

gaps:
  - truth: "AI agent can query search_docs tool and receive 5-10 relevant full markdown files"
    status: partial
    reason: "search_docs returns metadata only (path, score, summary, entities), not full markdown content"
    artifacts:
      - path: "internal/mcp/handlers.go"
        issue: "makeSearchHandler returns SearchResult without Content field"
      - path: "internal/mcp/types.go"
        issue: "SearchResult struct lacks Content field - only has Path, Score, Summary, Entities"
    missing:
      - "Add Content field to SearchResult struct in types.go"
      - "Modify makeSearchHandler to include doc.Content in search results"
      - "Update tool description to reflect that full content is returned"
    design_note: |
      Current implementation uses two-tool workflow: search_docs returns metadata,
      then fetch_doc retrieves full content. This is a scalable API pattern but
      conflicts with MCP-05 requirement "search_docs returns 5-10 full markdown files".
      
      Options:
      1. Add Content to SearchResult (matches requirement literally)
      2. Update requirement to reflect two-tool workflow (better design)
      3. Add flag to search_docs: include_content (optional, default false)
---

# Phase 3: MCP Server Core Verification Report

**Phase Goal:** AI agents can search and retrieve EINO documentation via MCP protocol  
**Verified:** 2026-01-25T20:16:49Z  
**Status:** gaps_found  
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | MCP server runs and responds to protocol requests using official Go SDK | ✓ VERIFIED | Server.go uses github.com/modelcontextprotocol/go-sdk v1.2.0, creates mcp.Server with Implementation, runs with StdioTransport |
| 2 | AI agent can query search_docs tool and receive 5-10 relevant full markdown files | ✗ FAILED | search_docs returns metadata only (SearchResult has Path, Score, Summary, Entities but NO Content field). Requirement MCP-05 says "returns 5-10 full markdown files" but implementation requires two-tool workflow (search → fetch) |
| 3 | AI agent can use fetch_doc tool to retrieve a specific document by path | ✓ VERIFIED | makeFetchHandler calls GetDocumentByPath, returns FetchDocOutput with full Content field (doc.Content prepended with source header) |
| 4 | AI agent can browse available documentation structure using list_docs tool | ✓ VERIFIED | makeListHandler calls ListDocumentPaths, returns ListDocsOutput with all paths and count |
| 5 | Returned documents are complete markdown files, not snippets or chunks | ✓ VERIFIED | fetch_doc returns doc.Content (full parent document), not chunk.Content. search_docs returns metadata pointing to parent documents, not chunks |

**Score:** 4/5 truths verified (truth 2 is PARTIAL — workflow achieves goal via two calls but doesn't match literal requirement)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/mcp-server/main.go` | MCP server entry point | ✓ VERIFIED | 73 lines, substantive. Creates context with signal handling, initializes QdrantStorage, creates Embedder, calls NewServer with Config, runs with stdio transport |
| `internal/mcp/server.go` | Server setup with tool registration | ✓ VERIFIED | 59 lines, substantive. NewServer creates mcp.Server, registers 3 tools (search_docs, fetch_doc, list_docs) with real handlers, Run() starts stdio transport |
| `internal/mcp/types.go` | Input/output structs with jsonschema tags | ✓ VERIFIED | 72 lines, substantive. All 6 types defined (SearchDocsInput/Output, FetchDocInput/Output, ListDocsInput/Output, SearchResult) with json and jsonschema tags |
| `internal/mcp/handlers.go` | Tool handler implementations | ⚠️ PARTIAL | 151 lines, substantive. All 3 handlers implemented with real storage/embedder integration. makeSearchHandler returns metadata only (no Content field in SearchResult) |
| `internal/storage/qdrant.go` (enhanced) | Storage methods with scores and path operations | ✓ VERIFIED | SearchChunksWithScores (lines 390-447), ListDocumentPaths (476-527), GetDocumentByPath (529-591) all present and substantive |
| `internal/storage/models.go` (enhanced) | ScoredChunk type | ✓ VERIFIED | ScoredChunk defined (lines 38-41) with embedded *Chunk and Score float64 field |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| cmd/mcp-server/main.go | internal/storage/qdrant.go | Storage initialization | ✓ WIRED | Line 27: `storage.NewQdrantStorage(qdrantHost, qdrantPort)` |
| cmd/mcp-server/main.go | internal/embedding/embedder.go | Embedder initialization | ✓ WIRED | Lines 39-43: `embedding.NewClient()` then `embedding.NewEmbedder(client, 0)` |
| cmd/mcp-server/main.go | internal/mcp/server.go | Server creation | ✓ WIRED | Lines 46-49: `mcpserver.NewServer(&mcpserver.Config{Storage: store, Embedder: embedder})` |
| internal/mcp/server.go | internal/mcp/handlers.go | Tool handler registration | ✓ WIRED | Lines 34-47: `mcp.AddTool` called 3 times with makeSearchHandler, makeFetchHandler, makeListHandler |
| internal/mcp/handlers.go | internal/storage/qdrant.go | Storage method calls | ✓ WIRED | Line 47: `SearchChunksWithScores`, Line 75: `GetDocument`, Line 108: `GetDocumentByPath`, Line 141: `ListDocumentPaths` |
| internal/mcp/handlers.go | internal/embedding/embedder.go | Query embedding generation | ✓ WIRED | Line 40: `embedder.GenerateEmbeddings(ctx, []string{input.Query})` |

### Requirements Coverage

| Requirement | Status | Blocking Issue |
|-------------|--------|----------------|
| MCP-01: MCP server using official Go SDK | ✓ SATISFIED | N/A |
| MCP-02: search_docs tool for semantic search | ✓ SATISFIED | Tool registered and functional |
| MCP-03: fetch_doc tool to get document by path | ✓ SATISFIED | Tool registered and functional |
| MCP-04: list_docs tool to browse structure | ✓ SATISFIED | Tool registered and functional |
| MCP-05: search_docs returns 5-10 full markdown files (not snippets) | ✗ BLOCKED | SearchResult lacks Content field — returns metadata only |

### Anti-Patterns Found

No blocker anti-patterns found. Code is clean:
- No TODO/FIXME comments
- No placeholder content
- No stub implementations (all handlers have real logic)
- No empty returns
- Proper error handling with fmt.Errorf and context wrapping
- No orphaned code (all functions are called/wired)

**Line count verification:**
- handlers.go: 151 lines (well above 15-line minimum for components)
- server.go: 59 lines (substantive)
- types.go: 72 lines (substantive)
- main.go: 73 lines (substantive)

### Human Verification Required

#### 1. End-to-End MCP Protocol Test

**Test:** Start MCP server with indexed documents, connect with Claude Code or MCP Inspector, run search query  
**Steps:**
1. Ensure Qdrant is running: `docker compose up -d`
2. Ensure documents are indexed (check with: `curl http://localhost:6333/collections/documents | jq '.result.points_count'`)
3. Build server: `go build -o mcp-server ./cmd/mcp-server/`
4. Run: `OPENAI_API_KEY=<key> ./mcp-server`
5. Test with MCP Inspector: `npx @anthropics/mcp-inspector ./mcp-server`
6. Or manual JSON-RPC:
   ```bash
   echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"search_docs","arguments":{"query":"how to create a ChatModel"}},"id":1}' | ./mcp-server
   ```

**Expected:**
- Server initializes without errors
- search_docs returns 5-10 results with scores, paths, summaries
- fetch_doc returns full markdown content with source header
- list_docs returns all indexed document paths

**Why human:** Requires running server with dependencies (Qdrant, OpenAI API key, indexed documents) and verifying protocol-level communication

#### 2. Search Result Relevance

**Test:** Run semantic search with domain-specific queries (e.g., "ChatModel initialization", "streaming responses", "error handling")  
**Expected:**
- Results are semantically relevant (not just keyword matches)
- Scores correlate with relevance (higher scores = better matches)
- Minimum score threshold filters out irrelevant results

**Why human:** Requires domain knowledge to judge relevance quality

#### 3. Workflow Validation: Search → Fetch

**Test:** Use search_docs to find relevant documents, then use fetch_doc to retrieve full content  
**Expected:**
- Search returns paths that can be passed to fetch_doc
- Fetched documents contain the information suggested by search summaries
- Source headers correctly identify document paths

**Why human:** Validates user workflow and verifies that two-tool pattern achieves the goal

### Gaps Summary

**Critical Gap:** search_docs returns metadata only, not full markdown content

**What's missing:**
1. SearchResult struct needs a Content field (string)
2. makeSearchHandler needs to include doc.Content when building SearchResult
3. Tool description should be updated if full content is included

**Impact on goal:**
- The phase goal "AI agents can search and retrieve EINO documentation" IS achievable via the two-tool workflow (search → fetch)
- However, requirement MCP-05 literally states "search_docs returns 5-10 full markdown files" which is NOT satisfied
- Current implementation is arguably BETTER design (scalable, follows API best practices) but doesn't match the stated requirement

**Design tension:**
- **Current implementation:** search_docs returns lightweight metadata (1-2KB response), fetch_doc retrieves full content (10-50KB per doc)
- **Literal requirement:** search_docs returns 5-10 full files in one response (50-500KB response)
- **Tradeoff:** Current design is more scalable and flexible; requirement design is simpler (one call instead of two)

**Recommendation:**
1. **Option A (match requirement):** Add Content to SearchResult, include full markdown in search responses
2. **Option B (update requirement):** Revise MCP-05 to reflect two-tool workflow design
3. **Option C (hybrid):** Add optional `include_content` parameter to search_docs (default: false for metadata only)

---

_Verified: 2026-01-25T20:16:49Z_  
_Verifier: Claude (gsd-verifier)_
