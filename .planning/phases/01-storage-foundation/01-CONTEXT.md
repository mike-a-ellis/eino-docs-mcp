# Phase 1: Storage Foundation - Context

**Gathered:** 2026-01-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Vector storage infrastructure for storing and retrieving document embeddings with full content and metadata. Qdrant database stores embeddings, full markdown content, and queryable metadata. Data persists across restarts. This is backend plumbing — downstream phases (processing, MCP tools) build on this foundation.

</domain>

<decisions>
## Implementation Decisions

### Qdrant Configuration
- Claude decides deployment mode (Docker sidecar vs Fly.io service) based on simplest Fly.io setup
- Vector dimension: 1536 (OpenAI text-embedding-3-small default)
- Single collection for all documents, filter by metadata
- Default search limit: 10 results
- Validate embedding dimension only before storing
- Retry with exponential backoff (3 attempts) on connection failures
- Expose health check method to verify Qdrant is reachable
- Auto-create collection if missing on first use

### Document Payload Structure
- Full markdown content stored inline in Qdrant point payload
- Store both full documents and chunks — chunks have embeddings, full docs stored for retrieval
- Each chunk has parent_doc_id field linking to full document
- Track both chunk_index (0, 1, 2...) and header path (section hierarchy) for each chunk

### Metadata Schema
- Timestamps in RFC3339 format ('2026-01-25T10:30:00Z')
- Source repository as full path ('cloudwego/eino', 'cloudwego/eino-ext')
- File path relative to docs root ('getting-started/installation.md')
- Store GitHub raw URL for document source

### Persistence
- Data path via QDRANT_DATA_PATH environment variable, default ./qdrant_data
- Verify collection exists and is accessible on startup
- Fail startup if Qdrant unreachable (no degraded mode)
- Provide ClearAll() admin method for re-indexing scenarios

### Claude's Discretion
- Qdrant deployment mode (Docker sidecar vs managed service)
- Exact retry timing and backoff multipliers
- Collection index configuration and optimization settings
- Internal error handling patterns

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches for Qdrant Go client usage.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 01-storage-foundation*
*Context gathered: 2026-01-25*
