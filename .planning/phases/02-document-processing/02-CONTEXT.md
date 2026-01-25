# Phase 2: Document Processing - Context

**Gathered:** 2026-01-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Fetch, chunk, embed, and index EINO documentation from cloudwego/cloudwego.github.io into Qdrant. This phase builds the indexing pipeline — manual sync triggering and status inspection are Phase 4.

</domain>

<decisions>
## Implementation Decisions

### Chunking Strategy
- Split at major headers only (H1 and H2) — keep content with its section
- Prepend header hierarchy to each chunk for standalone context (e.g., "# Doc Title > ## Section Name")
- No size limits — let sections be their natural size
- Claude's discretion on whether chunks should overlap with adjacent sections

### Metadata Generation
- Generate per-document metadata only (not per-chunk) — cheaper, sufficient for retrieval
- Summaries should capture topic + key points (e.g., "ChatModel interface: streaming, tool calling, message types")
- Use LLM extraction to identify key EINO functions, interfaces, and types from content
- Use GPT-4o for metadata generation (higher quality for summaries and entity extraction)

### Sync & Update Behavior
- Detect changes via Git commit SHA comparison — compare current to indexed SHA
- Moved/renamed files treated as delete + add — old path removed, new path indexed fresh
- This phase builds the indexer only — manual sync trigger is Phase 4 scope
- Claude's discretion on handling deleted documents (remove vs archive)

### Error Handling
- Fail entire batch if embedding API fails — abort and report which docs succeeded
- Respect retry-after header for OpenAI rate limits — wait as instructed, then retry
- Skip unparseable markdown files with warning — log issue, continue with other files
- Detailed logging during indexing — log each file processed, chunk counts, timing

### Claude's Discretion
- Whether to include overlap between adjacent chunks
- How to handle deleted documents (remove from index vs mark archived)
- Specific retry logic implementation details
- Batch sizes for embedding API calls

</decisions>

<specifics>
## Specific Ideas

- Embedding model is text-embedding-3-small (from requirements)
- Source is cloudwego/cloudwego.github.io EINO docs directory
- LLM metadata should identify EINO-specific concepts like ChatModel, Tool, Retriever, etc.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-document-processing*
*Context gathered: 2026-01-25*
