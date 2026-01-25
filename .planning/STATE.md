# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-25)

**Core value:** AI agents can retrieve relevant EINO documentation on demand — no manual doc hunting or copy-pasting required.
**Current focus:** Phase 4 - Observability & Manual Sync

## Current Position

Phase: 4 of 5 (Observability & Manual Sync)
Plan: 2 of TBD in current phase
Status: In progress
Last activity: 2026-01-25 — Completed 04-02-PLAN.md

Progress: [████████░░] 72%

## Performance Metrics

**Velocity:**
- Total plans completed: 13
- Average duration: 4.9 min
- Total execution time: 1.1 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-storage-foundation | 3 | 21.2min | 7.1min |
| 02-document-processing | 5 | 20.2min | 4.0min |
| 03-mcp-server-core | 3 | 17.6min | 5.9min |
| 04-observability-manual-sync | 2 | 7.5min | 3.8min |

**Recent Trend:**
- Last 5 plans: 03-01 (5.6min), 03-02 (8min), 03-03 (4min), 04-01 (3min), 04-02 (4.5min)
- Trend: Consistent fast execution. Phase 4 progressing well with CLI tooling.

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

### Pending Todos

None yet.

### Blockers/Concerns

**Research Insights:**
- Qdrant has no embedded Go mode — must run as separate service (Docker sidecar on Fly.io)
- MCP authentication should be implemented in Phase 1, not deferred (security best practice)
- Fly.io volumes are NOT automatically replicated — need snapshot configuration
- Embedding drift risk — partial re-indexing can degrade retrieval quality

None blocking current work. All flagged for consideration during planning.

## Session Continuity

Last session: 2026-01-25 22:03
Stopped at: Completed 04-02-PLAN.md (CLI Sync Command)
Resume file: None

---
*State initialized: 2026-01-25*
*Last updated: 2026-01-25*
