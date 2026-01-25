# Roadmap: EINO Documentation MCP Server

## Overview

This roadmap delivers an MCP server that provides AI agents with on-demand access to EINO framework documentation through semantic search. The journey moves from storage infrastructure (Phase 1), through document processing and indexing (Phase 2), to MCP protocol implementation with search tools (Phase 3), then observability features (Phase 4), and finally production deployment on Fly.io (Phase 5). Each phase builds on the previous, delivering independently verifiable capabilities that culminate in a production-ready documentation server.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Storage Foundation** - Vector database infrastructure with persistent storage
- [ ] **Phase 2: Document Processing** - Fetch, chunk, embed, and index EINO documentation
- [ ] **Phase 3: MCP Server Core** - Implement MCP protocol with search and retrieval tools
- [ ] **Phase 4: Observability & Manual Sync** - Index status inspection and manual re-indexing
- [ ] **Phase 5: Deployment** - Production deployment to Fly.io with persistent volumes

## Phase Details

### Phase 1: Storage Foundation
**Goal**: Vector storage infrastructure works and persists across restarts
**Depends on**: Nothing (first phase)
**Requirements**: STOR-01, STOR-02, STOR-03, STOR-04, STOR-05
**Success Criteria** (what must be TRUE):
  1. Qdrant database stores document embeddings and retrieves them via vector similarity search
  2. Full markdown document content is stored alongside vectors and returned with search results
  3. Document metadata (summary, entities, path, URL, timestamp, commit SHA) is stored and queryable
  4. Data persists across server restarts without re-indexing
  5. Current GitHub commit SHA is tracked and retrievable for indexed content
**Plans**: TBD

Plans:
- [ ] TBD during planning

### Phase 2: Document Processing
**Goal**: Documentation is fetched, chunked, embedded, and indexed in vector database
**Depends on**: Phase 1
**Requirements**: PROC-01, PROC-02, PROC-03, PROC-04, PROC-05
**Success Criteria** (what must be TRUE):
  1. All markdown files are fetched from cloudwego/cloudwego.github.io EINO docs directory
  2. Documents are chunked at markdown header boundaries preserving semantic units
  3. OpenAI embeddings are generated for each chunk using text-embedding-3-small
  4. LLM-generated summaries capture the main topic and purpose of each document
  5. Key EINO functions, methods, and classes are extracted from each document during indexing
**Plans**: TBD

Plans:
- [ ] TBD during planning

### Phase 3: MCP Server Core
**Goal**: AI agents can search and retrieve EINO documentation via MCP protocol
**Depends on**: Phase 2
**Requirements**: MCP-01, MCP-02, MCP-03, MCP-04, MCP-05
**Success Criteria** (what must be TRUE):
  1. MCP server runs and responds to protocol requests using official Go SDK
  2. AI agent can query search_docs tool and receive 5-10 relevant full markdown files
  3. AI agent can use fetch_doc tool to retrieve a specific document by path
  4. AI agent can browse available documentation structure using list_docs tool
  5. Returned documents are complete markdown files, not snippets or chunks
**Plans**: TBD

Plans:
- [ ] TBD during planning

### Phase 4: Observability & Manual Sync
**Goal**: Users can inspect index status and trigger manual re-indexing
**Depends on**: Phase 3
**Requirements**: MCP-06, DEPL-04
**Success Criteria** (what must be TRUE):
  1. User can query get_index_status tool to see indexed URLs, timestamps, stats, and source commit SHA
  2. User can trigger manual sync to re-index documentation from GitHub
  3. Index statistics show total documents, chunks, last sync time, and data freshness
**Plans**: TBD

Plans:
- [ ] TBD during planning

### Phase 5: Deployment
**Goal**: Server runs reliably on Fly.io with persistent storage and health monitoring
**Depends on**: Phase 4
**Requirements**: DEPL-01, DEPL-02, DEPL-03
**Success Criteria** (what must be TRUE):
  1. Server deploys to Fly.io using Dockerfile and runs without errors
  2. Qdrant data persists across deployments and server restarts via Fly.io volume
  3. Health check endpoint returns server status and catches deployment failures
  4. Server is accessible to MCP clients for production use
**Plans**: TBD

Plans:
- [ ] TBD during planning

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Storage Foundation | 0/TBD | Not started | - |
| 2. Document Processing | 0/TBD | Not started | - |
| 3. MCP Server Core | 0/TBD | Not started | - |
| 4. Observability & Manual Sync | 0/TBD | Not started | - |
| 5. Deployment | 0/TBD | Not started | - |

---
*Roadmap created: 2026-01-25*
*Last updated: 2026-01-25*
