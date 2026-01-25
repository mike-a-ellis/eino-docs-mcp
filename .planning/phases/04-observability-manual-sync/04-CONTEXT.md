# Phase 4: Observability & Manual Sync - Context

**Gathered:** 2026-01-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Provide tools for inspecting index state and triggering manual re-indexing. Users can see what's indexed, when it was synced, and refresh the index when needed. Automatic/scheduled sync belongs in a future phase.

</domain>

<decisions>
## Implementation Decisions

### Status output format
- Return counts + timestamps: total docs, total chunks, last sync time, source commit SHA
- Include list of all indexed document paths alongside counts
- Use ISO 8601 for timestamps (2026-01-25T14:30:00Z)
- No storage size reporting — keep it simple with doc/chunk counts only

### Sync behavior
- Full re-index only — clear and rebuild for guaranteed consistency
- Report progress by stage: 'Fetching...' → 'Chunking...' → 'Embedding...' → 'Done'
- Block queries during sync — return 'sync in progress' error until complete
- Sync is CLI command only — not exposed as MCP tool to agents

### Error surfacing
- get_index_status includes failed document paths and error reasons
- Per-document success/failure reported during sync progress
- Error detail format: category + message (e.g., 'embedding_failed: rate limit exceeded')

### Freshness indicators
- Staleness warning when > 20 commits behind GitHub HEAD
- Threshold is commit-based, not time-based

### Claude's Discretion
- How to surface Qdrant connection issues (status field vs fail the call)
- Whether to do live GitHub HEAD comparison (performance vs freshness tradeoff)
- Staleness display format (field value vs separate warning)

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-observability-manual-sync*
*Context gathered: 2026-01-25*
