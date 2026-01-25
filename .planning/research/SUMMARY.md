# Project Research Summary

**Project:** MCP Server for EINO Documentation
**Domain:** Model Context Protocol (MCP) documentation server with vector search
**Researched:** 2026-01-25
**Confidence:** HIGH

## Executive Summary

This project builds an MCP server in Go that serves EINO documentation to AI agents via semantic search. Research shows the recommended approach uses the official MCP Go SDK (v1.2.0) with Qdrant for vector storage, OpenAI embeddings, and deploys to Fly.io with persistent volumes. The architecture follows 2026 best practices: minimal tool count (3-5 tools, not 20+), workflow-based design, and semantic chunking at markdown boundaries.

The critical technical constraint is that Qdrant has no embedded Go mode - it must run as a separate service (Docker sidecar or external instance). This requires multi-process deployment on Fly.io with volume-backed persistence. The stack is well-supported with official clients for all services and HIGH confidence across all research areas.

Key risks include: (1) MCP security - authentication must be built from day 1, not as an afterthought; (2) Fly.io volume data loss - volumes are NOT automatically replicated and require manual snapshot configuration; (3) embedding drift - partial re-indexing can create "two worlds" of vectors with degraded retrieval quality. All risks have well-documented mitigations from production systems in 2026.

## Key Findings

### Recommended Stack

The stack uses official clients and libraries throughout, with the only significant constraint being Qdrant's architecture requirement. Unlike Python which offers embedded mode (`QdrantClient(":memory:")`), the Go client requires Qdrant to run as a separate service communicating via gRPC on port 6334.

**Core technologies:**
- **github.com/modelcontextprotocol/go-sdk v1.2.0** - Official MCP server implementation; provides StdioTransport and structured tool APIs with JSON schema support
- **Qdrant (Docker) + github.com/qdrant/go-client v1.16.0** - Vector search engine; official Go client uses gRPC; no embedded mode available, must run as separate service
- **github.com/openai/openai-go/v3 v3.16.0** - OpenAI embeddings API; official library (beta but recommended over community alternative for long-term support)
- **github.com/google/go-github/v81** - GitHub API client; industry standard for fetching EINO docs from repository
- **Fly.io Volumes** - Persistent storage for Qdrant data; native Fly.io primitive with encryption at rest, 5-day snapshot retention (manual configuration required)

**Supporting libraries:**
- zerolog for structured logging (JSON output for monitoring)
- envconfig for type-safe environment variable loading
- godotenv for local development .env files

**Deployment pattern:** Multi-process Fly.io app with MCP server + Qdrant sidecar, or separate apps with private networking (eino-qdrant.internal).

### Expected Features

The feature landscape is dominated by an anti-pattern to avoid: API-mirroring where each document gets its own tool. Microsoft Learn MCP and Grounded Docs MCP demonstrate the correct pattern: 3-5 polymorphic tools that handle all use cases through parameters.

**Must have (table stakes):**
- **search_docs** - Semantic search using vector embeddings; expected by all documentation MCP servers
- **fetch_doc** - Retrieve full markdown content by path/URI; MCP pattern is "find then fetch"
- **list_topics** - List available documentation categories; enables progressive discovery
- **Semantic chunking** - Split at markdown headers (H2/H3), not fixed token boundaries; prevents mid-sentence breaks
- **Relevance ranking** - Cosine similarity with top-N limiting (5-10 results to avoid context bloat)
- **Full document retrieval** - Return complete markdown files, not snippets (project requirement)
- **Basic metadata** - Document title, file path, content summary

**Should have (competitive):**
- **Topic filtering** - Search within specific doc sections (e.g., "prompts/"); adds `topic` parameter to search tool
- **Search result previews** - 2-3 sentence excerpt showing why document matched
- **Context-enriched chunks** - Prepend section hierarchy (breadcrumbs) to chunks for precision
- **Chunk overlap** - 10-20% overlap to prevent information loss at boundaries
- **Embedding cache** - Persist computed embeddings to disk to reduce startup time and API costs

**Defer (v2+):**
- **Entity extraction** - NLP/LLM processing to identify APIs, components, concepts (HIGH complexity, uncertain value)
- **Version awareness** - Tag docs by EINO version (only if docs are actually versioned)
- **Real-time GitHub sync** - Webhooks and incremental updates (manual/scheduled rebuild is sufficient for infrequent doc updates)
- **Hybrid search** - BM25 + vector combination (pure vector search with quality embeddings is sufficient)
- **Multi-query search** - Accept multiple queries in single call (nice optimization, not essential)

### Architecture Approach

The architecture separates concerns into layers: MCP protocol handling, query orchestration, embedding generation, and vector storage. The build order follows dependencies: storage first (testable in isolation), then embeddings, then document processing, then MCP server, finally sync worker.

**Major components:**
1. **MCP Server** - Protocol handling via go-sdk StdioTransport, tool/resource routing; talks to Query Handler
2. **Query Handler** - Orchestrates query embedding, vector search, result formatting; the "business logic" layer
3. **Embedding Service** - OpenAI API wrapper with batching (up to 2048 texts per call) and rate limiting for TPM quotas
4. **Vector Store** - Qdrant client wrapper; collection management, upsert/search operations, persistence to Fly volume
5. **Indexing Pipeline** - Fetch markdown from GitHub, parse and chunk, batch embed, store; runs as background worker
6. **Sync Worker** - Periodic GitHub polling (15min), SHA comparison for change detection, triggers indexing

**Key patterns:**
- **Deterministic point IDs** - Hash (filepath + chunk_index + repo_sha) for idempotent upserts and version tracking
- **Recursive chunking** - Split on H2 headers, then paragraphs, then sentences with 256-512 token target
- **Batch embedding with rate limiting** - Use rate.Limiter to respect OpenAI TPM, batch up to 2048 inputs
- **Collection schema** - 1536-dim vectors (text-embedding-3-small), cosine distance, on_disk=true for large datasets, indexed fields for file/repo_sha filtering

**Critical anti-pattern to avoid:** Synchronous indexing on query path - blocks users for minutes. Always run indexing in background worker with atomic index swaps.

### Critical Pitfalls

Research identified 16 domain-specific pitfalls; the top 5 must be addressed in Phase 1:

1. **Missing authentication on MCP server** - Teams skip auth because MCP spec doesn't mandate it; leads to confused deputy attacks and prompt injection vulnerabilities. Prevention: Implement from day 1, use secrets manager, log all tool invocations, run in network-limited container.

2. **Fly.io volume data loss** - Volumes are NOT automatically replicated; hardware failure = total data loss. Prevention: Enable daily snapshots (billable as of Jan 2026), design idempotent rebuild from GitHub, store sync metadata outside volume, test restore procedure.

3. **GitHub API rate limit exhaustion** - Recursive tree fetching hits 5000 req/hour limit, causing partial index updates. Prevention: Use conditional requests (If-None-Match for 304s that don't count), client-side rate limiting (stay under 4500/hour), monitor X-RateLimit-Remaining header, implement exponential backoff.

4. **Embedding drift** - Partial re-indexing creates mixed model versions; retrieval quality degrades over time. Prevention: Track embedding model version in metadata (text-embedding-3-small@20260115), flag ALL embeddings as stale when model changes, design for full re-indexing capability.

5. **Using STDIO transport for production** - Works locally but cannot be deployed; late-stage rewrite required. Prevention: Use Streamable HTTP transport from day 1 (2026 standard), avoid deprecated SSE, design for network deployment (latency, failures).

**Additional moderate pitfalls for Phase 2:**
- **Naive chunking** - Fixed-size breaks semantic units; use recursive markdown-aware chunking
- **OpenAI cost explosion** - Re-embed everything on every sync; use Batch API (50% savings), track content hashes, only re-embed changed docs
- **Qdrant batch upsert timeouts** - Go client doesn't auto-split large batches; keep under 500 vectors per upsert
- **No reranking layer** - Pure vector similarity misses context; consider hybrid search (BM25 + vector) in Phase 2

## Implications for Roadmap

Based on research, suggested phase structure follows dependency order: storage, embeddings, processing, MCP, sync. Security and persistence must be in Phase 1, not deferred.

### Phase 1: Foundation & Core Search
**Rationale:** Storage is the foundation (everything depends on it), and core MCP tools are the deliverable. This phase produces a testable end-to-end system.

**Delivers:**
- Qdrant client wrapper with collection initialization
- OpenAI embeddings service with batching and rate limiting
- Semantic chunking at markdown boundaries (256-512 tokens)
- MCP server with 3 core tools: search_docs, fetch_doc, list_topics
- Manual index rebuild from GitHub repo
- Fly.io deployment with persistent volume

**Addresses features:**
- search_docs (table stakes)
- fetch_doc (table stakes)
- list_topics (table stakes)
- Semantic chunking (table stakes)
- Full document retrieval (table stakes)

**Avoids pitfalls:**
- Pitfall 1: Authentication implementation (not just "TODO for later")
- Pitfall 3: Fly volume snapshot configuration
- Pitfall 5: GitHub rate limit handling with backoff
- Pitfall 11: Use Streamable HTTP transport (architecture decision)
- Pitfall 6: Semantic chunking from start

**Research flag:** Standard patterns (SKIP research-phase) - MCP Go SDK is well-documented, Qdrant patterns are established, chunking strategies are proven.

### Phase 2: Search Quality & Optimization
**Rationale:** Now that core retrieval works, optimize for quality and cost. Depends on Phase 1 metrics (retrieval precision, API costs).

**Delivers:**
- Topic filtering (add `topic` parameter to search_docs)
- Search result previews (show match context)
- Chunk overlap (10-20% for continuity)
- Context-enriched chunks (breadcrumb prefixes)
- Embedding cache (persist to disk)
- GitHub conditional requests (304 optimization)
- OpenAI Batch API integration (50% cost savings)

**Uses stack:**
- Qdrant payload filtering (by topic/file)
- OpenAI Batch API endpoint
- Structured logging for cost tracking

**Implements architecture:**
- Enhanced chunking strategy with overlap
- Drift detection (re-embed sample, compare distances)
- Optimized sync job (only changed files)

**Avoids pitfalls:**
- Pitfall 4: Embedding drift detection and versioning
- Pitfall 9: Cost explosion via Batch API and hash tracking
- Pitfall 10: Batch upsert size limits (500 vectors max)

**Research flag:** Needs validation (CONSIDER research-phase) - Hybrid search patterns, reranking approaches. But defer actual implementation to post-MVP.

### Phase 3: Reliability & Monitoring
**Rationale:** Production hardening after core functionality is validated. Adds observability and resilience.

**Delivers:**
- Scheduled sync worker (every 15min with SHA comparison)
- Circuit breaker for sync failures (stop retrying after N failures)
- Goroutine leak prevention (goleak tests, context cancellation)
- Fly.io cold start optimization (min_machines_running=1)
- Structured metrics (search latency p50/p95/p99, indexing duration, costs)
- Health checks and monitoring
- Snapshot restore testing

**Avoids pitfalls:**
- Pitfall 12: Goroutine leaks in sync job
- Pitfall 13: Cold start delays
- Pitfall 14: Circuit breaker for sustained failures

**Research flag:** Standard patterns (SKIP research-phase) - Go concurrency patterns, circuit breakers, and Fly.io configuration are well-documented.

### Phase 4: Security Hardening (Post-MVP)
**Rationale:** After production validation, harden against adversarial inputs. Deferred because read-only documentation server has lower risk surface than write-enabled tools.

**Delivers:**
- Documentation content sanitization (detect prompt-like patterns)
- Output filtering (prevent instruction leakage)
- MCP tool response size limits (configure HTTP transport max)
- SQL injection prevention (if adding metadata queries)
- Audit logging and monitoring

**Avoids pitfalls:**
- Pitfall 2: Prompt injection via documentation content
- Pitfall 8: MCP tool response size limits
- Pitfall 15: SQL injection in tool arguments

**Research flag:** Needs research (DEFINITELY research-phase) - Prompt injection patterns evolve rapidly, need 2026-current detection techniques.

### Phase Ordering Rationale

- **Foundation first (Phase 1):** Storage, Embeddings, MCP tools. This order enables incremental testing: can query Qdrant directly before MCP layer exists, can test chunking before full pipeline, can validate end-to-end before optimization.

- **Quality before reliability (Phase 2 then 3):** No point hardening a system with poor retrieval quality. Get search working well, then make it reliable.

- **Security last but not skipped (Phase 4):** Read-only documentation has lower risk than write-enabled tools, so security hardening can be post-MVP. BUT authentication (Pitfall 1) is in Phase 1 - that's table stakes.

- **Grouping rationale:** Each phase delivers value independently. Phase 1 = MVP (working search), Phase 2 = production-ready (cost-effective, high-quality), Phase 3 = reliable (won't wake you up), Phase 4 = hardened (adversary-resistant).

- **Avoids pitfall grouping:** The 5 critical pitfalls are all addressed by end of Phase 1. Moderate pitfalls spread across Phases 2-3. Minor pitfalls addressed as needed.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 4 (Security Hardening):** Prompt injection detection is an active research area; patterns evolve rapidly; need current (2026) detection techniques and adversarial testing approaches.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Foundation):** MCP Go SDK has excellent docs, Qdrant patterns are established, chunking strategies proven in production RAG systems.
- **Phase 3 (Reliability):** Go concurrency patterns, circuit breakers, Fly.io deployment are well-documented with many examples.
- **Phase 2 (Quality):** Hybrid search and reranking are well-studied; can reference existing implementations (Microsoft Learn MCP, Grounded Docs MCP).

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | **HIGH** | Official clients for all services (MCP SDK, Qdrant, OpenAI, GitHub); Fly.io deployment patterns well-documented; only concern is OpenAI beta status but v3.16.0 is stable enough |
| Features | **HIGH** | Analyzed 3 reference implementations (Microsoft Learn MCP, Grounded Docs MCP, official servers); feature expectations clear; anti-patterns well-documented in 2026 best practices |
| Architecture | **HIGH** | Build order validated by dependency analysis; component boundaries match proven patterns; Qdrant "no embedded mode" constraint confirmed in official docs |
| Pitfalls | **HIGH** | 16 pitfalls sourced from 2025-2026 real-world incidents, production issues, and official documentation warnings; not hypothetical concerns |

**Overall confidence:** **HIGH**

All four research areas show HIGH confidence. The stack uses official libraries, the feature landscape has proven reference implementations, the architecture follows established patterns, and pitfalls are documented from production systems. No major unknowns remain for roadmap planning.

### Gaps to Address

Minor gaps to validate during implementation (none are blockers):

- **OpenAI official SDK stability:** The go-openai v3 library is still beta. Monitor for breaking changes during development. Mitigation: Pin to v3.16.0, watch release notes, budget for migration if needed. Community alternative (sashabaranov/go-openai) is fallback.

- **Qdrant Go client performance:** Some reports of higher latency vs Python/Java clients (Pitfall 10). Mitigation: Test with production-scale data early, use built-in batching (Insert method auto-splits at 100 vectors), consider connection pooling if needed.

- **Chunk size optimization:** Research recommends 256-512 tokens but optimal size depends on EINO doc structure. Mitigation: Start with 512 tokens, measure retrieval quality (precision@k, recall@k), adjust based on metrics.

- **MCP transport choice validation:** Streamable HTTP is 2026 standard, but go-sdk examples emphasize StdioTransport. Mitigation: Verify HTTP transport support in go-sdk v1.2.0, test with Claude Desktop/other clients early, have fallback plan to SSE if needed.

All gaps have clear mitigations and won't block Phase 1 delivery. They're tuning parameters, not architectural unknowns.

## Sources

### Primary (HIGH confidence)

**MCP Specification & SDK:**
- [Official Go SDK for Model Context Protocol](https://github.com/modelcontextprotocol/go-sdk) - v1.2.0 release, API docs
- [Model Context Protocol Tools Specification](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) - Official spec
- [MCP Server Best Practices for 2026](https://www.cdata.com/blog/mcp-server-best-practices-2026) - Transport, security, architecture

**Stack Libraries:**
- [Qdrant Go Client](https://github.com/qdrant/go-client) - Official client docs
- [Qdrant Documentation](https://qdrant.tech/documentation/) - Storage, collections, payload indexing
- [OpenAI Embeddings API](https://platform.openai.com/docs/api-reference/embeddings) - Pricing, rate limits, model specs
- [google/go-github](https://github.com/google/go-github) - v81 docs
- [Fly.io Volumes](https://fly.io/docs/volumes/overview/) - Persistence, snapshots, resilience

**Reference Implementations:**
- [Microsoft Learn MCP Server](https://github.com/MicrosoftDocs/mcp) - Semantic search implementation
- [Grounded Docs MCP Server](https://github.com/arabold/docs-mcp-server) - Multiple source types
- [Official MCP Reference Servers](https://github.com/modelcontextprotocol/servers) - Tool patterns

### Secondary (MEDIUM confidence)

**Feature Patterns:**
- [Less is More: 4 design patterns for better MCP servers](https://www.klavis.ai/blog/less-is-more-mcp-design-patterns-for-ai-agents) - Anti-patterns, workflow-based design
- [MCP Patterns & Anti-Patterns for Enterprise AI](https://medium.com/@thirugnanamk/mcp-patterns-anti-patterns-for-implementing-enterprise-ai-d9c91c8afbb3) - Tool proliferation warnings

**Chunking Strategies:**
- [Chunking Strategies for LLM Applications - Pinecone](https://www.pinecone.io/learn/chunking-strategies/) - Fixed-size, semantic, recursive
- [Mastering Chunking for RAG - Databricks](https://community.databricks.com/t5/technical-blog/the-ultimate-guide-to-chunking-strategies-for-rag-applications/ba-p/113089) - 256-512 token recommendations
- [Chunking for RAG Best Practices - Unstructured](https://unstructured.io/blog/chunking-for-rag-best-practices) - Overlap strategies

**Security & Pitfalls:**
- [MCP Security Survival Guide](https://towardsdatascience.com/the-mcp-security-survival-guide-best-practices-pitfalls-and-real-world-lessons/) - Authentication, prompt injection
- [Understanding MCP security risks - RedHat](https://www.redhat.com/en/blog/model-context-protocol-mcp-understanding-security-risks-and-controls) - Confused deputy, audit trails
- [When Embeddings Go Stale](https://medium.com/@yashtripathi.nits/when-embeddings-go-stale-detecting-fixing-retrieval-drift-in-production-778a89481a57) - Drift detection
- [Embedding Drift: The Quiet Killer](https://dev.to/dowhatmatters/embedding-drift-the-quiet-killer-of-retrieval-quality-in-rag-systems-4l5m) - Production impact

### Tertiary (LOW confidence, needs validation)

**Performance Tuning:**
- [High Latencies with Qdrant Go-Client](https://github.com/qdrant/qdrant/issues/5642) - GitHub issue, specific to batching
- [Common Goroutine Leaks to Avoid](https://betterprogramming.pub/common-goroutine-leaks-that-you-should-avoid-fe12d12d6ee) - Context cancellation patterns

---
*Research completed: 2026-01-25*
*Ready for roadmap: yes*
