# Requirements: EINO Documentation MCP Server

**Defined:** 2025-01-25
**Core Value:** AI agents can retrieve relevant EINO documentation on demand — no manual doc hunting or copy-pasting required.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### MCP Server

- [ ] **MCP-01**: MCP server implemented in Go using official MCP Go SDK
- [ ] **MCP-02**: Server exposes `search_docs` tool for semantic search
- [ ] **MCP-03**: Server exposes `fetch_doc` tool to get specific document by path
- [ ] **MCP-04**: Server exposes `list_docs` tool to browse documentation structure
- [ ] **MCP-05**: `search_docs` returns 5-10 full markdown files per query (not snippets)
- [ ] **MCP-06**: Server exposes `get_index_status` tool returning indexed URLs, timestamps, stats, and source commit SHA

### Document Processing

- [ ] **PROC-01**: Fetch markdown files from GitHub repo (cloudwego/cloudwego.github.io/content/en/docs/eino)
- [ ] **PROC-02**: Chunk documents at markdown header boundaries (semantic chunking)
- [ ] **PROC-03**: Generate OpenAI embeddings (text-embedding-3-small) for document chunks
- [ ] **PROC-04**: Generate LLM summary for each document during indexing
- [ ] **PROC-05**: Extract key EINO functions/methods/classes from each document during indexing

### Storage

- [ ] **STOR-01**: Store embeddings in Qdrant vector database
- [ ] **STOR-02**: Store full document content alongside vectors (for full-file retrieval)
- [ ] **STOR-03**: Store document metadata (summary, entities, path, URL, indexed timestamp)
- [ ] **STOR-04**: Data persists across server restarts (Fly.io persistent volume)
- [ ] **STOR-05**: Track source GitHub commit SHA for indexed content

### Deployment

- [ ] **DEPL-01**: Deploy to Fly.io with Dockerfile
- [ ] **DEPL-02**: Configure persistent volume for Qdrant data
- [ ] **DEPL-03**: Implement health check endpoint
- [ ] **DEPL-04**: Manual sync trigger (endpoint or CLI command to re-index)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Search Enhancements

- **SRCH-01**: Topic/category filtering on search queries
- **SRCH-02**: Related document suggestions based on current context

### Sync & Maintenance

- **SYNC-01**: Incremental sync (only re-index changed files)
- **SYNC-02**: Periodic auto-sync on configurable schedule
- **SYNC-03**: Embedding version tracking (detect drift when model changes)
- **SYNC-04**: Automated snapshot backups

### Observability

- **OBS-01**: Metrics endpoint (request counts, latencies, index size)
- **OBS-02**: Structured logging
- **OBS-03**: Alerting on sync failures

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Real-time webhook updates | Periodic/manual sync sufficient for doc updates |
| Multiple documentation sources | EINO docs only for v1, can extend later |
| User authentication | Personal/team use, not public API |
| Web UI | MCP interface only, AI agents are the users |
| Rate limiting | Personal use, not expecting high traffic |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| MCP-01 | TBD | Pending |
| MCP-02 | TBD | Pending |
| MCP-03 | TBD | Pending |
| MCP-04 | TBD | Pending |
| MCP-05 | TBD | Pending |
| MCP-06 | TBD | Pending |
| PROC-01 | TBD | Pending |
| PROC-02 | TBD | Pending |
| PROC-03 | TBD | Pending |
| PROC-04 | TBD | Pending |
| PROC-05 | TBD | Pending |
| STOR-01 | TBD | Pending |
| STOR-02 | TBD | Pending |
| STOR-03 | TBD | Pending |
| STOR-04 | TBD | Pending |
| STOR-05 | TBD | Pending |
| DEPL-01 | TBD | Pending |
| DEPL-02 | TBD | Pending |
| DEPL-03 | TBD | Pending |
| DEPL-04 | TBD | Pending |

**Coverage:**
- v1 requirements: 20 total
- Mapped to phases: 0
- Unmapped: 20 ⚠️

---
*Requirements defined: 2025-01-25*
*Last updated: 2025-01-25 after initial definition*
