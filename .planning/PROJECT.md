# EINO Documentation MCP Server

## What This Is

An MCP server written in Go that provides EINO framework documentation to AI agents through semantic search. Instead of manually browsing cloudwego.io and pasting relevant docs into AI conversations, agents can query this server to get the documentation they need for any EINO-related task.

## Core Value

AI agents can retrieve relevant EINO documentation on demand — no manual doc hunting or copy-pasting required.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] MCP server implemented in Go using the MCP protocol
- [ ] Fetches documentation from GitHub (cloudwego/cloudwego.github.io/content/en/docs/eino)
- [ ] Periodic sync to keep docs current (configurable interval)
- [ ] Embedded Qdrant for vector storage with persistent volume (survives restarts)
- [ ] OpenAI text-embedding-3-small for semantic search
- [ ] LLM-generated summaries and entity extraction during indexing
- [ ] `search_docs` tool — semantic search returning 5-10 full markdown files
- [ ] `fetch_doc` tool — get a specific document by path/name
- [ ] `list_docs` tool — browse available documentation structure
- [ ] Returns full markdown files, not just matching chunks
- [ ] Deployed to Fly.io with persistent volume for Qdrant data

### Out of Scope

- Real-time webhook updates — periodic sync is sufficient
- Multiple documentation sources — EINO docs only for v1
- User authentication — server is for personal/team use
- Web UI — MCP interface only

## Context

EINO is a Go framework for building AI applications (part of CloudWeGo ecosystem). The documentation lives at:
- Source: https://github.com/cloudwego/cloudwego.github.io/tree/main/content/en/docs/eino
- Rendered: https://www.cloudwego.io/docs/eino/

The pain point: when working on EINO tasks with AI agents, finding and pasting relevant documentation is manual and repetitive. This server automates that — the AI queries for what it needs.

## Constraints

- **Language**: Go — matches the EINO ecosystem and user preference
- **Vector DB**: Embedded Qdrant — must persist across server restarts on Fly.io
- **Embeddings**: OpenAI API — requires API key, incurs per-token cost
- **Deployment**: Fly.io — simple Go deployment with persistent volumes
- **Storage**: Must not re-query GitHub constantly — local storage with periodic sync

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Full document retrieval | User needs complete context, not snippets | — Pending |
| LLM-generated metadata | Better summaries and entity extraction than parsing | — Pending |
| Embedded Qdrant | No external DB dependency, simpler deployment | — Pending |
| Periodic sync over webhooks | Simpler to implement, docs don't change frequently | — Pending |

---
*Last updated: 2025-01-25 after initialization*
