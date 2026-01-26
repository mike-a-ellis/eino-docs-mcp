# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-25)

**Core value:** AI agents can retrieve relevant EINO documentation on demand — no manual doc hunting or copy-pasting required.
**Current focus:** Phase 5 - Deployment

## Current Position

Phase: 5 of 5 (Deployment)
Plan: 4 of 4 in current phase
Status: Phase complete - All phases complete
Last activity: 2026-01-26 — Completed 05-04-PLAN.md

Progress: [██████████] 100%

## Performance Metrics

**Velocity:**
- Total plans completed: 18
- Average duration: 4.6 min
- Total execution time: 1.5 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-storage-foundation | 3 | 21.2min | 7.1min |
| 02-document-processing | 5 | 20.2min | 4.0min |
| 03-mcp-server-core | 3 | 17.6min | 5.9min |
| 04-observability-manual-sync | 2 | 7.5min | 3.8min |
| 04.1-env-file-configuration | 1 | 2min | 2min |
| 05-deployment | 4 | 23.6min | 5.9min |

**Recent Trend:**
- Last 5 plans: 04.1-01 (2min), 05-02 (4.6min), 05-01 (6min), 05-03 (2min), 05-04 (11min)
- Trend: All phases complete. Production deployment successful at eino-docs-mcp.fly.dev.

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Full document retrieval over snippets — users need complete context
- LLM-generated metadata for better summaries and entity extraction
- Embedded Qdrant with persistent volume — no external DB dependency
- Periodic sync over webhooks — simpler implementation, docs change infrequently
- Use gRPC port 6334 for Qdrant (2-3x faster than REST) — Plan 01-01
- Named Docker volume over bind mount (WSL filesystem performance) — Plan 01-01
- Duplicate path/repository in chunks for efficient filtering — Plan 01-01
- Fail-fast startup for Qdrant connection (no degraded mode) — Plan 01-02
- Exponential backoff retry: 500ms initial, 10s max, 30s total — Plan 01-02
- Payload indexes created at collection setup time (not lazily) — Plan 01-02
- Use Qdrant named vectors for optional embeddings — Plan 01-03
- Split only at H1/H2 boundaries (not H3+) for semantic coherence — Plan 02-02
- No chunk overlap - header hierarchy provides sufficient context — Plan 02-02
- No size limits on chunks - preserve complete sections — Plan 02-02
- Use go-github-ratelimit middleware instead of hand-rolled rate limiting — Plan 02-01
- Default to cloudwego/cloudwego.github.io repo with content/en/docs/eino path — Plan 02-01
- Use GPT-4o for metadata generation (higher quality than 3.5-turbo) — Plan 02-04
- Truncate at 16k tokens for cost efficiency and error prevention — Plan 02-04
- Per-document metadata generation (not per-chunk) for 10-20x cost reduction — Plan 02-04
- Batch size 500 texts for embedding requests (balances RPM vs TPM) — Plan 02-03
- Validate OPENAI_API_KEY on client creation (fail-fast for better errors) — Plan 02-03
- Retry only HTTP 429 rate limits (treat other errors as permanent) — Plan 02-03
- Use float32 for embeddings (50% memory reduction, negligible precision loss) — Plan 02-03
- Metadata failures are non-fatal (continue with empty values) — Plan 02-05
- Unparseable documents skipped with warning (don't fail entire indexing) — Plan 02-05
- Embedding failures are fatal for that document (required for search) — Plan 02-05
- jsonschema tag = description only (Google jsonschema-go format) — Plan 03-01
- Stop scroll pagination when results < batch size (not on empty) — Plan 03-02
- ScoredChunk embeds *Chunk to reuse existing type — Plan 03-02
- Handler factory pattern: makeXxxHandler returns closure over dependencies — Plan 03-03
- Search deduplication: keep highest-scoring chunk per parent document — Plan 03-03
- Request 3x limit for search to ensure enough unique docs after dedup — Plan 03-03
- Source header prepended to fetched content for attribution — Plan 03-03
- Two-tool workflow: search_docs returns metadata, fetch_doc retrieves content (scalable design) — Phase 3 verification
- Chunk count calculated as PointsCount - document count (1 parent + N chunks per doc) — Plan 04-01
- GitHub API failures return nil for commits_behind (graceful degradation) — Plan 04-01
- Stale warning threshold set at >20 commits behind HEAD — Plan 04-01
- Qdrant errors prefixed with "qdrant_error:" for caller disambiguation — Plan 04-01
- Use Cobra for CLI framework (standard in Go ecosystem) — Plan 04-02
- Reuse OpenAI client from embeddings for metadata generation (no duplicate clients) — Plan 04-02
- Clear collection before indexing (full refresh model, not incremental) — Plan 04-02
- Graceful .env fallback - missing file logs warning, doesn't fail — Plan 04.1-01
- sync CLI silent fallback - no log noise for production — Plan 04.1-01
- prod.env is documentation only - fly secrets for actual values — Plan 04.1-01
- Health endpoint returns 200/healthy when Qdrant connected, 503/unhealthy when disconnected — Plan 05-02
- 3-second timeout for health checks to prevent hanging Fly.io health probes — Plan 05-02
- Bind to 0.0.0.0 instead of localhost for container compatibility — Plan 05-02
- Run HTTP server in background goroutine while MCP server continues on stdio — Plan 05-02
- Use distroless/static-debian11 for minimal runtime image — Plan 05-01
- CGO_ENABLED=0 for static binary with no external dependencies — Plan 05-01
- Strip debug symbols with -ldflags="-w -s" for smaller binary size — Plan 05-01
- Run as non-root user (nonroot:nonroot) for security — Plan 05-01
- Qdrant binary path is /qdrant/qdrant (not /usr/bin/qdrant) verified via Docker inspection — Plan 05-03
- Process-specific volume mounts using processes array in [[mounts]] — Plan 05-03
- MCP server 256MB / Qdrant 512MB resource allocation for Fly.io — Plan 05-03
- Health check timings: 10s grace + 15s interval + 5s timeout — Plan 05-03
- Supervisor script for single-machine deployment (process groups require multiple machines) — Plan 05-04
- Debian 12 base image required for Qdrant GLIBC 2.34+ compatibility — Plan 05-04
- SERVER_MODE environment variable keeps MCP server alive for health endpoint — Plan 05-04

### Pending Todos

None yet.

### Roadmap Evolution

- Phase 4.1 inserted after Phase 4: Environment Configuration - .env file support for local.env and prod.env (URGENT)

### Blockers/Concerns

**Research Insights:**
- Qdrant has no embedded Go mode — must run as separate service (Docker sidecar on Fly.io)
- MCP authentication should be implemented in Phase 1, not deferred (security best practice)
- Fly.io volumes are NOT automatically replicated — need snapshot configuration
- Embedding drift risk — partial re-indexing can degrade retrieval quality

None blocking current work. All flagged for consideration during planning.

## Session Continuity

Last session: 2026-01-26
Stopped at: Completed 05-04-PLAN.md (Production Deployment) - ALL PHASES COMPLETE
Resume file: None

## Project Status

**DEPLOYMENT COMPLETE**

The EINO Documentation MCP Server is now live in production:
- URL: https://eino-docs-mcp.fly.dev
- Health: https://eino-docs-mcp.fly.dev/health
- Status: Running (MCP server + Qdrant on Fly.io)
- Storage: 1GB persistent volume for Qdrant data

All 5 phases completed successfully. System ready for use.

---
*State initialized: 2026-01-25*
*Last updated: 2026-01-26*
