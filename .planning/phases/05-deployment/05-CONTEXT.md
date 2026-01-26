# Phase 5: Deployment - Context

**Gathered:** 2026-01-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Production deployment to Fly.io with persistent storage and health monitoring. The MCP server and Qdrant run as separate services, data persists across restarts, and clients can connect over public HTTPS.

</domain>

<decisions>
## Implementation Decisions

### Architecture
- Sidecar container model — Qdrant runs as separate Fly.io machine, MCP server connects over internal network
- Primary region: iad (Virginia)
- MCP server size: shared-cpu-1x 256MB (smallest, can scale if needed)
- Qdrant size: shared-cpu-1x 512MB (minimum viable for vector operations)

### Persistence
- Volume size: 1GB for Qdrant data (sufficient for EINO docs corpus)
- Recovery strategy: re-index from GitHub on volume loss (source of truth is GitHub, not the index)
- No automatic volume snapshots — accept re-indexing as recovery method
- Empty start on first deploy — run sync command manually to populate index
- Manual sync only — no scheduled cron jobs, sync when docs are known to change

### Health & Monitoring
- Health check verifies: server alive + Qdrant connectivity
- Response format: JSON status object ({"status": "healthy", "qdrant": "connected"})
- No external alerting — rely on Fly.io dashboard and manual checks

### Client Access
- Public HTTPS via Fly.io TLS termination — accessible from anywhere
- No authentication required — documentation is public content
- No rate limiting — trust clients, rely on Fly.io built-in protection
- App name: eino-docs-mcp (eino-docs-mcp.fly.dev)

### Claude's Discretion
- Logging format (structured JSON vs plain text)
- Dockerfile optimization and build caching
- Fly.io configuration specifics (fly.toml)
- Health check endpoint path and timing

</decisions>

<specifics>
## Specific Ideas

- GitHub is the source of truth — losing the index is recoverable, not catastrophic
- Keep deployment simple and minimal — can scale/harden later if needed
- Two-machine model matches research insight that Qdrant has no embedded Go mode

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-deployment*
*Context gathered: 2026-01-25*
