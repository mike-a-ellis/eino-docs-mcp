# Feature Landscape: MCP Documentation Server

**Domain:** Documentation serving via Model Context Protocol (MCP)
**Researched:** 2026-01-25
**Confidence:** HIGH (based on MCP specification, existing implementations, and 2026 best practices)

## Executive Summary

An MCP documentation server bridges AI agents with technical documentation through standardized tools. Based on analysis of the MCP specification, reference implementations (Microsoft Learn MCP, Grounded Docs MCP, GitHub MCP), and 2026 best practices, this server must provide semantic search capabilities, full document retrieval, and metadata-rich results while avoiding common anti-patterns like API mirroring and context window bloat.

---

## Table Stakes Features

Features users (AI agents) expect. Missing any = server feels incomplete or broken.

### 1. Core MCP Tools

| Tool | Purpose | Why Expected | Complexity |
|------|---------|--------------|------------|
| **search_docs** | Semantic search across EINO documentation using keywords + meaning | Every documentation MCP server provides search; agents need to find relevant docs before reading them | **Medium** - Requires vector embeddings, similarity search, chunking strategy |
| **fetch_doc** | Retrieve full markdown content for a specific document by path/URI | Agents need complete documents for context, not just snippets; MCP pattern is "find then fetch" | **Low** - Direct file read and return |
| **list_topics** | List available documentation categories/sections (e.g., "getting-started", "components", "prompts") | Agents need to discover what documentation exists; enables progressive discovery pattern | **Low** - Directory structure traversal or metadata index |

**Rationale:** These three tools match the pattern from Microsoft Learn MCP (`search` + `fetch`) and enable the recommended "workflow-based design" where agents can discover → search → retrieve in a coherent flow.

### 2. Semantic Search Capabilities

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Keyword + Vector Search** | Pure keyword matching misses synonyms and concepts (e.g., "LLM orchestration" vs "agent framework"); vector embeddings enable semantic understanding | **Medium** | Use embedding model (OpenAI, Ollama, Gemini, etc.) to create vector index; 256-512 token chunks recommended for technical docs |
| **Relevance Ranking** | Must return most relevant results first; agents have limited context windows and need best matches immediately | **Low** | Cosine similarity or dot product on vector embeddings provides natural ranking |
| **Result Limiting** | Return top N results (e.g., 5-10) to avoid context window bloat; MCP best practice is "less is more" | **Low** | Simple result slicing after ranking |

### 3. Full Document Retrieval

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Complete Markdown** | Project requirements specify "return full markdown files, not snippets"; agents need entire documents for comprehensive understanding | **Low** | Read source file and return as text content |
| **Preserve Formatting** | Markdown structure (headers, lists, code blocks) essential for comprehension | **Low** | Return raw markdown without HTML conversion |
| **Source Attribution** | Include file path and source repository info in results | **Low** | Metadata in tool response |

### 4. Basic Metadata

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Document Title** | Extracted from first H1 or filename; essential for agent to understand what document is about | **Low** | Parse markdown front matter or first heading |
| **File Path** | Enables `fetch_doc` calls; agents need to reference specific documents | **Low** | Relative path from docs root |
| **Content Summary** | 1-2 sentence description helps agents decide if document is relevant before fetching full content | **Medium** | Can extract from markdown front matter or first paragraph; consider LLM-generated summaries |

---

## Differentiators

Features that set the server apart. Not expected, but highly valued.

### 1. Advanced Search Features

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Topic Filtering** | Allow search within specific doc sections (e.g., "search only in prompts/"); reduces noise and improves precision | **Low** | Add optional `topic` parameter to search tool; filter chunks by file path prefix |
| **Multi-Query Search** | Accept multiple search queries in single call; reduces round trips for complex information needs | **Medium** | Process multiple queries, deduplicate results, merge rankings |
| **Search Result Previews** | Return 2-3 sentence excerpt showing why document matched; helps agents decide what to fetch | **Low** | Extract surrounding context from matched chunks |

### 2. Enhanced Metadata

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Entity Extraction** | Identify key entities (APIs, components, concepts) in each document; enables entity-based search ("show me all docs mentioning ComponentOrchestrator") | **High** | Requires NLP/LLM processing; consider Gemini API for extraction |
| **Topic Tags** | Categorize docs by themes (e.g., "beginner", "advanced", "API reference"); enables filtering and recommendation | **Medium** | Manual tagging in front matter or automated via LLM classification |
| **Related Documents** | Suggest related docs based on content similarity; helps agents discover connected information | **Medium** | Use vector similarity to find nearest neighbors in embedding space |
| **Code Language Tags** | Tag docs by programming languages mentioned (Go, Python, etc.); enables language-specific searches | **Low** | Detect code fence languages in markdown |

### 3. Intelligent Chunking

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Semantic Chunking** | Split documents at logical boundaries (sections, subsections) rather than fixed token counts; improves retrieval accuracy | **Medium** | Parse markdown structure, split at headers while respecting 256-512 token target |
| **Chunk Overlap** | Include overlapping context between chunks (e.g., 50-100 tokens); prevents information loss at chunk boundaries | **Low** | Adjust chunking algorithm to include tail of previous chunk |
| **Context-Enriched Chunks** | Prepend section hierarchy to chunks (e.g., "Getting Started > Installation > Prerequisites: ..."); improves search precision | **Medium** | Track document structure during chunking, add breadcrumb prefix |

### 4. Version Awareness

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Version Tagging** | Tag docs by EINO version; enables version-specific queries ("show me EINO 0.4.0 getting started") | **Medium** | Parse version from file path or front matter; index versions separately |
| **Latest Version Default** | Default searches to latest version unless specified; matches user expectations | **Low** | Filter or rank by version metadata |

### 5. Performance Optimization

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **In-Memory Vector Index** | Load embeddings into memory for O(1) lookup; critical for sub-200ms response times (MCP latency concerns) | **Low** | Use simple in-memory map or FAISS index |
| **Embedding Cache** | Cache computed embeddings to avoid recomputation; improves startup time and reduces API costs | **Low** | LRU cache or disk-based persistence |
| **Parallel Chunking** | Process documents in parallel during indexing; reduces initial index build time | **Low** | Use goroutines for concurrent processing |

---

## Anti-Features

Features to explicitly NOT build. Common mistakes in this domain.

### 1. API-Mirroring Pattern

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **One Tool Per Document** | Exposing separate tools like `fetch_getting_started`, `fetch_components`, etc. inflates tool count and "dramatically drops task completion rate" per MCP best practices | **Use polymorphic design:** Single `fetch_doc` tool with `path` parameter covers all documents; single `search_docs` tool with optional `topic` filter covers all search needs |
| **Granular Operation Tools** | Separate tools for `search_by_keyword`, `search_by_semantic`, `search_by_topic` creates unnecessary complexity | **Unified search tool:** Combine all search modes into single `search_docs` with intelligent parameter handling |

**Impact:** High - This is the #1 MCP anti-pattern. Microsoft Learn MCP uses 3 tools total; Grounded Docs MCP uses minimal tool set. More tools ≠ better functionality.

### 2. Snippet-Only Responses

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Return Only Matched Chunks** | Project requirements explicitly state "return full markdown files, not snippets"; snippets force agents to make multiple calls for complete understanding | **Search returns metadata + preview, fetch returns full docs:** Use two-phase pattern where search helps find relevant docs, fetch retrieves complete content |
| **Aggressive Truncation** | Cutting off documents to save tokens creates incomplete context and forces follow-up queries | **Trust agent's context management:** Return full documents; let agent decide what to include in LLM context. Modern models have 200K+ context windows |

**Impact:** High - Breaks core use case and creates poor DX for agents.

### 3. Dynamic Tool Lists

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Change Tools Mid-Session** | MCP spec's `tools/list_changed` notification seems useful but "should be used with caution" per 2026 best practices; invalidates model provider caching and increases costs | **Static, well-designed tool set:** Design 3-5 core tools upfront that handle all use cases through parameters, not tool proliferation |

**Impact:** Medium - Adds complexity without meaningful benefit; hurts caching.

### 4. Complex Sync Mechanisms

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Real-Time GitHub Sync** | Constant polling or webhooks add complexity and failure modes; documentation updates are infrequent | **Manual/Scheduled Refresh:** Simple command to rebuild index from GitHub repo; run on deployment or on-demand. Documentation doesn't change often enough to justify real-time sync |
| **Differential Updates** | Tracking individual file changes, incremental embeddings, delta syncing creates complex state management | **Full Rebuild:** Re-index entire documentation set on update. With ~100 docs and modern embedding APIs, full rebuild takes seconds to minutes |

**Impact:** Medium - Adds significant complexity for minimal gain. Documentation updates are rare enough that full rebuilds are acceptable.

### 5. Over-Engineering Search

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **BM25 + Vector Hybrid Search** | Combining multiple ranking algorithms adds complexity; vector search alone provides excellent results for technical documentation | **Pure Vector Search:** Single embedding model with cosine similarity. Simple, effective, maintainable |
| **Query Expansion / Reranking** | Techniques like query rewriting, cross-encoder reranking add latency and complexity | **Quality Embeddings + Good Chunks:** Use proven embedding model (OpenAI text-embedding-3-small, Gemini embeddings) with semantic chunking. Handles synonyms and concepts naturally |
| **Multiple Embedding Models** | Trying different models, ensemble approaches creates complexity | **Single Best-Practice Model:** Pick one proven model (OpenAI, Gemini) and optimize chunking/metadata instead |

**Impact:** Low-Medium - Diminishing returns. 90% of quality comes from good chunking and single quality embedding model.

### 6. Kitchen Sink Metadata

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Auto-Extract Everything** | Extracting every possible entity, keyword, relationship creates bloated metadata that slows indexing and rarely gets used | **Target Useful Metadata:** Focus on title, summary, file path, and 2-3 high-value fields (topic tags, code languages). Test what agents actually use |
| **Deep Content Analysis** | Running sentiment analysis, readability scores, complexity metrics on technical docs wastes computation | **Essential Fields Only:** Agents need "what is this doc about?" and "where is it?" — not literary analysis |

**Impact:** Low - Wastes effort without improving retrieval quality.

---

## Feature Dependencies

```
Core Foundation (Must build first):
├─ Document Indexing
│  ├─ Load markdown files from source
│  └─ Parse front matter and content
│
├─ Chunking Strategy
│  ├─ Semantic chunking (split at headers)
│  ├─ 256-512 token chunks
│  └─ Context enrichment (breadcrumbs)
│
└─ Vector Embeddings
   ├─ Choose embedding model (OpenAI/Gemini)
   ├─ Generate embeddings for chunks
   └─ In-memory index (map or FAISS)

MCP Tool Layer (Depends on foundation):
├─ search_docs
│  ├─ Vector similarity search
│  ├─ Relevance ranking
│  ├─ Result limiting (top 5-10)
│  └─ Return: title, summary, path, preview
│
├─ fetch_doc
│  ├─ Path-based document lookup
│  └─ Return: full markdown content
│
└─ list_topics
   └─ Enumerate doc categories

Enhancements (Optional, build after core):
├─ Topic filtering (requires topic metadata)
├─ Version awareness (requires version tagging)
├─ Related documents (requires similarity computation)
└─ Entity extraction (requires LLM/NLP processing)
```

**Critical Path:** Indexing → Chunking → Embeddings → MCP Tools → Sync/Update

---

## MVP Recommendation

### Must Have (Phase 1)

**Core Tools:**
1. `search_docs` - Semantic search with top-N results
2. `fetch_doc` - Full markdown document retrieval
3. `list_topics` - Category enumeration

**Search Capabilities:**
- Vector embeddings (OpenAI text-embedding-3-small or Gemini)
- Semantic chunking at markdown headers
- 256-512 token chunks with slight overlap
- Cosine similarity ranking
- Return top 5-10 results

**Metadata (Minimal):**
- Document title (from H1 or filename)
- File path (relative to docs root)
- Content summary (first paragraph or LLM-generated)
- Topic/category (from directory structure)

**Sync/Update:**
- Manual index rebuild command
- Load from local clone of GitHub repo
- Full re-indexing on update

### Nice to Have (Phase 2)

Defer these until core is validated:

1. **Topic Filtering** - Add `topic` parameter to `search_docs`
2. **Search Previews** - Return 2-3 sentence excerpt showing match context
3. **Related Documents** - Return 2-3 similar docs in fetch response
4. **Embedding Cache** - Persist embeddings to disk to speed up restarts
5. **Code Language Tags** - Auto-detect programming languages in code blocks

### Explicitly Defer (Post-MVP)

Do NOT build until proven necessary:

- Real-time GitHub sync (manual is fine)
- Entity extraction (complex, uncertain value)
- Version awareness (only if EINO docs are versioned)
- Hybrid search (vector search alone is sufficient)
- Multi-query search (nice to have, not essential)
- Dynamic tool lists (anti-pattern)

---

## Complexity Assessment

| Feature Category | Estimated Effort | Risk Level | Priority |
|-----------------|------------------|------------|----------|
| **Document Loading & Parsing** | 2-3 days | Low | P0 (Required) |
| **Chunking Strategy** | 3-5 days | Medium | P0 (Required) |
| **Vector Embeddings & Index** | 3-5 days | Medium | P0 (Required) |
| **Core MCP Tools (3 tools)** | 2-3 days | Low | P0 (Required) |
| **Basic Metadata Extraction** | 1-2 days | Low | P0 (Required) |
| **Manual Sync/Update** | 1-2 days | Low | P0 (Required) |
| **Topic Filtering** | 1 day | Low | P1 (Phase 2) |
| **Search Previews** | 1-2 days | Low | P1 (Phase 2) |
| **Related Documents** | 1 day | Low | P1 (Phase 2) |
| **Entity Extraction** | 5-7 days | High | P2 (Defer) |
| **Version Awareness** | 3-5 days | Medium | P2 (Defer) |

**Total MVP Estimate:** 12-18 days for core functionality

---

## Design Principles (from Research)

### 1. Less is More (MCP Best Practice)
- Minimize context window usage
- Return focused, relevant results
- 3-5 tools total, not 20+
- Top 5-10 search results, not 100

### 2. Workflow-Based Tools (MCP Pattern)
- Design tools around agent workflows: "I need to find info about X" → search → fetch
- Not around API operations: "list all docs", "get doc by ID", "get doc metadata", etc.

### 3. Model-Controlled Discovery (MCP Philosophy)
- Let agents discover tools via `tools/list`
- Provide rich tool descriptions so LLM understands when to use each
- No manual agent configuration required

### 4. Human in the Loop (MCP Security)
- Even though these tools are read-only, follow best practices
- Log all tool invocations
- Return clear source attribution in results

### 5. Performance Matters (Production MCP)
- Target <200ms tool response time
- In-memory vector index for speed
- Cache embeddings to reduce startup time
- Parallel processing where possible

---

## Sources

**MCP Specification & Best Practices:**
- [Tools - Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) - Official MCP tools specification
- [Less is More: 4 design patterns for building better MCP servers](https://www.klavis.ai/blog/less-is-more-mcp-design-patterns-for-ai-agents) - Semantic search, workflow-based design, code mode, progressive discovery patterns
- [MCP Server Best Practices for 2026](https://www.cdata.com/blog/mcp-server-best-practices-2026) - Architecture, security, transport recommendations
- [MCP Patterns & Anti-Patterns for implementing Enterprise AI](https://medium.com/@thirugnanamk/mcp-patterns-anti-patterns-for-implementing-enterprise-ai-d9c91c8afbb3) - Anti-patterns to avoid

**Reference Implementations:**
- [GitHub - MicrosoftDocs/mcp](https://github.com/MicrosoftDocs/mcp) - Microsoft Learn MCP Server with semantic search
- [GitHub - arabold/docs-mcp-server](https://github.com/arabold/docs-mcp-server) - Grounded Docs MCP Server with multiple source types
- [GitHub - modelcontextprotocol/servers](https://github.com/modelcontextprotocol/servers) - Official MCP reference servers

**Semantic Search & Chunking:**
- [Chunking Strategies for LLM Applications | Pinecone](https://www.pinecone.io/learn/chunking-strategies/) - Fixed-size, semantic, recursive chunking strategies
- [Mastering Chunking Strategies for RAG: Best Practices & Code Examples](https://community.databricks.com/t5/technical-blog/the-ultimate-guide-to-chunking-strategies-for-rag-applications/ba-p/113089) - 2026 best practices, 256-512 token recommendations
- [OpenSearch semantic search: The basics and a quick tutorial [2026 guide]](https://www.instaclustr.com/education/opensearch/opensearch-semantic-search-the-basics-and-a-quick-tutorial-2026-guide/) - Implementation patterns

**Metadata & Documentation Management:**
- [12 Metadata Tagging Best Practices for Developers](https://strapi.io/blog/metadata-tagging-best-practices) - Automation, consistency, taxonomy recommendations
- [Metadata Standards: Importance, Types, and Best Practices](https://www.acceldata.io/blog/metadata-standards-made-simple-essential-types-and-best-practices) - Standards and governance

**Sync & Updates:**
- [GitBook Git Sync](https://www.gitbook.com/features/git-sync) - Documentation sync patterns
- [GitHub Document Management](https://technicalwriterhq.com/documentation/document-management/github-document-management/) - Repository-based documentation workflows
