# Phase 3: MCP Server Core - Context

**Gathered:** 2026-01-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement MCP protocol server exposing EINO documentation to AI agents through search, retrieval, and browsing tools. AI agents can query indexed documentation and receive full markdown files with metadata. This phase delivers the protocol layer — observability and deployment are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Search response design
- Result count configurable via parameter, default 5 documents
- Include relevance/similarity scores (0.0-1.0 scale) with each result
- Rich metadata per result: path, summary, entities, last updated timestamp
- Metadata only in search results — agent uses fetch_doc to get full content

### Tool edge cases
- Empty search returns empty array with helpful message ("No matching documents found. Try broader terms.")
- Invalid path in fetch_doc returns error with suggestions of similar valid paths
- list_docs returns flat list of all document paths
- Minimum relevance threshold of 0.5 (configurable) — filter low-quality results

### Server identity
- Server name: "eino-documentation-server"
- Tool descriptions: concise one-liners (e.g., "Search EINO documentation")
- Server info includes indexed commit SHA and timestamp
- Expose document listing as MCP resource (in addition to tools)

### Document formatting
- Prepend source info header: `<!-- Source: path/to/doc.md -->`
- Preserve code blocks exactly as-is from source markdown
- Convert relative links to absolute GitHub URLs
- Structured response: {content, path, summary, updated} — not just content string

### Claude's Discretion
- Exact error message wording
- How similarity suggestions are generated for invalid paths
- Resource template format for doc listing

</decisions>

<specifics>
## Specific Ideas

- Relevance threshold should be adjustable (0.5 default but allow override)
- Server should announce what commit/version it has indexed upfront in server info
- Tools plus resources approach: search/fetch as tools, doc listing as browseable resource

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-mcp-server-core*
*Context gathered: 2026-01-25*
