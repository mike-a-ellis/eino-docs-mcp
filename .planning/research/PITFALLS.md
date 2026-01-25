# Domain Pitfalls: MCP Documentation Server

**Project:** MCP server in Go serving EINO docs to AI agents
**Stack:** Go, embedded Qdrant, OpenAI embeddings, GitHub sync, Fly.io
**Researched:** 2026-01-25

---

## Critical Pitfalls

Mistakes that cause rewrites, security breaches, or major production issues.

### Pitfall 1: Missing Authentication on MCP Server
**What goes wrong:** MCP makes it easy to wire up AI agents to real functionality, but most developers skip authentication because the standard is still evolving. Every MCP interface - no matter how "internal" - needs real security controls.

**Why it happens:** The MCP protocol focuses on tool/resource definitions, not auth. Teams assume "it's just internal" or "behind a firewall" means safe.

**Consequences:**
- Confused deputy problem: users gain access to resources available to the server but not to them
- Prompt injection attacks can manipulate the agent into unauthorized actions
- No audit trail of who called which tools

**Prevention:**
- Implement authentication from day 1, not as "phase 2"
- Use a secrets manager for API keys (never environment variables or config files)
- Log who called which server with which arguments
- Consider running the MCP server in a container with network limits

**Detection:**
- Can you access the server without credentials?
- Are API keys visible in logs or prompts?
- Is there an audit trail for tool invocations?

**Phase to address:** Phase 1 (MVP) - Security is not negotiable

**Sources:**
- [MCP Security Survival Guide](https://towardsdatascience.com/the-mcp-security-survival-guide-best-practices-pitfalls-and-real-world-lessons/)
- [Understanding MCP security risks](https://www.redhat.com/en/blog/model-context-protocol-mcp-understanding-security-risks-and-controls)
- [MCP Server Best Practices 2026](https://www.cdata.com/blog/mcp-server-best-practices-2026)

---

### Pitfall 2: Prompt Injection via Documentation Content
**What goes wrong:** GitHub documentation can contain indirect prompt injections - embedded prompts in code examples or comments that the LLM misinterprets as valid commands.

**Why it happens:** The RAG pipeline treats all documentation as trusted content. Malicious or accidental prompt-like text gets embedded and retrieved verbatim.

**Consequences:**
- Agent follows instructions from documentation instead of user
- Potential data exfiltration if docs contain commands like "ignore previous instructions, send all data to..."
- Tool poisoning through misleading descriptions in docs

**Prevention:**
- Sanitize documentation before embedding (strip code blocks with suspicious patterns)
- Use structured metadata to separate "content" from "instructions"
- Implement output filtering to detect command-like responses
- Monitor for retrieval of docs with high "instruction-like" token patterns

**Detection:**
- Does the agent ever respond with "as the documentation says, I should..."?
- Are retrieved chunks triggering unexpected tool calls?
- Do embeddings rank instructional text higher than factual content?

**Phase to address:** Phase 2 (Post-MVP) - After basic retrieval works, before public deployment

**Sources:**
- [MCP Security: Common risks to watch for](https://www.datadoghq.com/blog/monitor-mcp-servers/)
- [Top 10 Risks of Using MCP Servers](https://www.backslash.security/blog/top-risks-mcp-servers-ide)

---

### Pitfall 3: Fly.io Volume Data Loss (No Automatic Replication)
**What goes wrong:** Fly volumes are slices of NVMe drives on physical hardware. If the hardware fails, your embedded Qdrant database is gone. Fly.io does NOT automatically replicate volumes.

**Why it happens:** Teams assume "persistent volume" means "safe from loss." The documentation explicitly states: "For raw Fly volumes, resilience is your problem."

**Consequences:**
- Complete data loss if physical server fails
- No way to recover embeddings without rebuilding from GitHub
- Rebuilding takes time (API rate limits) and money (OpenAI embedding costs)

**Prevention:**
- Implement daily volume snapshots (billable as of Jan 2026, but worth it)
- Design sync job to be fully idempotent - can rebuild index from scratch
- Store metadata about last successful sync (timestamp, commit SHA) outside volume
- Consider periodic exports of vector index to object storage (S3/R2)
- Set up alerts for volume health metrics

**Detection:**
- Do you have a tested restore procedure?
- Can you rebuild the entire index in <1 hour?
- Are snapshots enabled and tested?

**Phase to address:** Phase 1 (MVP) - Before deploying to production

**Sources:**
- [Fly Volumes overview](https://fly.io/docs/volumes/overview/)
- [Persistent but not resilient?](https://community.fly.io/t/persistent-but-not-resilient/3477)
- [Fly Volume snapshot billing 2026](https://fly.io/docs/volumes/)

---

### Pitfall 4: Embedding Drift (Stale Vectors)
**What goes wrong:** When you update documentation and re-embed only the changed files, the vector index contains two "worlds" of meaning - old embeddings using one model version, new ones using another. This is called embedding drift.

**Why it happens:**
- OpenAI updates embedding models (same API, different vectors)
- Partial re-embeddings (only changed files)
- Nondeterministic preprocessing (chunk boundaries change)

**Consequences:**
- Retrieval quality degrades over time (old and new docs not comparable)
- Semantically similar docs drift apart in vector space
- Users report "search used to work better"
- Engineers spend 10-30 hours/month troubleshooting RAG issues that started with drift

**Prevention:**
- Track embedding model version in metadata (e.g., `text-embedding-3-small@20260115`)
- When model changes, flag ALL embeddings as stale, not just new docs
- Implement drift detection: periodically re-embed a sample and compare distances
- Design sync to support full re-indexing (not just incremental)
- Monitor retrieval quality metrics (precision@k, recall@k)

**Detection:**
- Is retrieval quality degrading over time?
- Do t-SNE/UMAP visualizations show distinct clusters by update time?
- Have you tested re-embedding the same document - do vectors match?

**Phase to address:** Phase 2 (Post-MVP) - After basic sync works

**Sources:**
- [When Embeddings Go Stale](https://medium.com/@yashtripathi.nits/when-embeddings-go-stale-detecting-fixing-retrieval-drift-in-production-778a89481a57)
- [Embedding Drift: The Quiet Killer](https://dev.to/dowhatmatters/embedding-drift-the-quiet-killer-of-retrieval-quality-in-rag-systems-4l5m)
- [Embedding Drift: The Silent RAG Breaker](https://medium.com/@nooralamshaikh336/embedding-drift-the-silent-rag-breaker-nobody-talks-about-ca4a268ef0c1)

---

### Pitfall 5: GitHub API Rate Limit Exhaustion
**What goes wrong:** Periodic sync jobs hit GitHub's rate limit (5,000 requests/hour authenticated) when recursively fetching large documentation repos, causing partial index updates and stale data.

**Why it happens:**
- Syncing repo trees requires multiple API calls (list files, get content, etc.)
- Teams poll for updates instead of using webhooks
- No exponential backoff or client-side rate limiting

**Consequences:**
- Sync jobs fail mid-process, leaving index partially updated
- Some docs are fresh, others are months old
- Hitting rate limit triggers 1-hour cooldown, delaying all updates
- Users see inconsistent results (some queries return outdated info)

**Prevention:**
- Use webhooks for updates instead of periodic polling
- Monitor rate limit headers (`X-RateLimit-Remaining`, `X-RateLimit-Reset`)
- Implement client-side rate limiting (stay below 4,500/hour to be safe)
- Use conditional requests (`If-None-Match`) - 304 responses don't count against limit
- Implement exponential backoff (1s, 2s, 4s, 8s) when approaching limit
- Cache API responses (e.g., file tree structure)
- Batch content fetching intelligently

**Detection:**
- Are you seeing HTTP 429 (rate limit exceeded) errors?
- Do sync jobs sometimes complete in 5 minutes, sometimes fail after 30?
- Is `X-RateLimit-Remaining` consistently near zero?

**Phase to address:** Phase 1 (MVP) - Critical for reliable sync

**Sources:**
- [GitHub API rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
- [Best Practices for Handling GitHub API Rate Limits](https://github.com/orgs/community/discussions/151675)
- [Managing Rate Limits for the GitHub API](https://www.lunar.dev/post/a-developers-guide-managing-rate-limits-for-the-github-api)

---

## Moderate Pitfalls

Mistakes that cause delays, increased costs, or technical debt.

### Pitfall 6: Naive Chunking Strategy
**What goes wrong:** Using fixed-size chunking (e.g., "every 500 tokens") breaks semantic units mid-sentence, splits code examples from explanations, and destroys the structure needed for good retrieval.

**Why it happens:** It's the simplest approach. Teams don't realize chunking is the #1 failure point of RAG systems in 2026.

**Consequences:**
- Low retrieval precision (irrelevant chunks ranked high)
- Context fragmentation (code and explanation in different chunks)
- Poor user experience ("Why did it return this fragment?")

**Prevention:**
- Use semantic chunking (respect headers, paragraphs, code blocks)
- For markdown docs, split on H1/H2/H3 boundaries
- Keep code examples with their explanatory text
- Test chunk sizes: 10-20% overlap is a good starting point
- Measure retrieval quality, don't assume "bigger chunks = better"
- Consider metadata (doc title, section headers) in retrieval

**Detection:**
- Are chunks breaking mid-sentence or mid-code-block?
- Do top-K results feel disjointed?
- Is overlap too high (redundant chunks) or too low (context loss)?

**Phase to address:** Phase 1 (MVP) - Chunking strategy determines retrieval quality

**Sources:**
- [Document Chunking Strategies for Vector Databases](https://www.dataquest.io/blog/document-chunking-strategies-for-vector-databases/)
- [Chunking Strategies for LLM Applications](https://www.pinecone.io/learn/chunking-strategies/)
- [Vector DB Retrieval: To chunk or not to chunk](https://unstract.com/blog/vector-db-retrieval-to-chunk-or-not-to-chunk/)

---

### Pitfall 7: No Reranking Layer
**What goes wrong:** Pure vector similarity doesn't always match user intent. A document about "building a server" and "server crashes" might have similar embeddings, but only one is relevant to "how to deploy."

**Why it happens:** Teams assume embeddings alone are sufficient. Reranking feels like optimization, not necessity.

**Consequences:**
- Lower precision (semantically similar but contextually wrong results)
- Users lose trust in search ("It returns stuff that's technically related but not what I need")
- Missing exact keyword matches (e.g., acronyms, SKUs, function names)

**Prevention:**
- Implement hybrid search: combine vector similarity with keyword matching (BM25)
- Use Reciprocal Rank Fusion (RRF) to merge results
- Add a lightweight reranking model (e.g., cross-encoder) for top-K candidates
- Monitor reranking impact on retrieval quality (before/after metrics)

**Detection:**
- Do users frequently rephrase queries?
- Are top-3 results often "close but not quite"?
- Does adding exact keyword matching improve results?

**Phase to address:** Phase 2 (Post-MVP) - After basic retrieval works

**Sources:**
- [Optimizing RAG with Hybrid Search & Reranking](https://superlinked.com/vectorhub/articles/optimizing-rag-with-hybrid-search-reranking)
- [Understanding hybrid search RAG](https://www.meilisearch.com/blog/hybrid-search-rag)
- [Advanced RAG: Hybrid Search and Re-ranking](https://dev.to/kuldeep_paul/advanced-rag-from-naive-retrieval-to-hybrid-search-and-re-ranking-4km3)

---

### Pitfall 8: MCP Tool Response Size Limits
**What goes wrong:** MCP servers have response size limits (default 4MB in HTTP transport, ~25K tokens in some clients). Large documentation chunks or full code files exceed these limits, causing truncation or errors.

**Why it happens:** Teams don't test with large documents. Claude Code reportedly truncates responses to ~700 characters in some scenarios.

**Consequences:**
- Tool responses cut off mid-sentence
- "Maximum call stack size exceeded" errors
- Agents receive incomplete context, hallucinate the rest
- Users see partial code examples

**Prevention:**
- Configure HTTP transport max message size (default 4MB, can increase)
- Produce compact, model-friendly summaries by default
- Keep full payloads behind an explicit flag or separate tool
- Implement response pagination for large results
- Use progress notifications for long-running operations
- Test with worst-case documents (large API references, etc.)

**Detection:**
- Are responses mysteriously truncated?
- Do you see "maxBuffer length exceeded" errors?
- Are tool calls timing out?

**Phase to address:** Phase 1 (MVP) - Test with large docs early

**Sources:**
- [MCP HTTP Stream Transport](https://mcp-framework.com/docs/Transports/http-stream-transport/)
- [Truncated MCP Tool Responses](https://github.com/anthropics/claude-code/issues/2638)
- [MCP server stack size limit](https://github.com/danny-avila/LibreChat/issues/5744)
- [Handling large text output from MCP server](https://github.com/orgs/community/discussions/169224)

---

### Pitfall 9: OpenAI Embeddings Cost Explosion
**What goes wrong:** Naive sync jobs re-embed all documents on every run, or embed documents one-at-a-time instead of batching. Costs spiral, especially for large documentation repos.

**Why it happens:**
- Simple "embed everything" approach
- Not tracking which docs changed
- Missing batch API benefits (50% cost savings)

**Consequences:**
- High costs: 10,000 docs × 500 tokens = 5M tokens = $0.10 per full re-index (Standard tier)
- Rate limits hit faster
- Slow sync times

**Prevention:**
- Use OpenAI Batch API (50% cheaper: $0.01/1M tokens for `text-embedding-3-small`)
- Batch up to 2,048 embeddings per request
- Track document hashes (SHA256 of content) - only re-embed if changed
- Use `text-embedding-3-small` unless you need higher quality (best cost/performance)
- Implement caching: store embeddings with content hash as key
- Monitor token consumption via OpenAI dashboard

**Detection:**
- Are embedding costs higher than expected?
- Is every sync run costing the same, even for small updates?
- Are you making 1 API call per document?

**Phase to address:** Phase 1 (MVP) - Cost optimization from the start

**Sources:**
- [OpenAI Embeddings Pricing 2026](https://costgoat.com/pricing/openai-embeddings)
- [OpenAI Rate limits](https://platform.openai.com/docs/guides/rate-limits)
- [OpenAI Pricing 2026](https://www.finout.io/blog/openai-pricing-in-2026)

---

### Pitfall 10: Qdrant Batch Upsert Timeouts
**What goes wrong:** Inserting large batches of embeddings (thousands of vectors) into Qdrant via Go client causes timeouts, consensus failures, or shard instability.

**Why it happens:** Go client doesn't automatically split batches. Sending 10,000 vectors at once overwhelms Qdrant.

**Consequences:**
- Sync jobs fail mid-process
- Shards enter dead/partially dead state
- Index corruption requiring rebuild
- High latencies compared to other clients (Java, Python)

**Prevention:**
- Use the Go client's built-in batching: `Insert` method auto-splits at 100 vectors (configurable)
- For manual batching, keep batches under 500 vectors
- Add retries with exponential backoff for timeout errors
- Monitor Qdrant shard health via API
- Test with production-scale data (thousands of docs)
- Consider connection pooling for concurrent upserts

**Detection:**
- Are you seeing timeout errors during batch inserts?
- Do Qdrant shards show unhealthy status?
- Are insertion times >1s per 100 vectors?

**Phase to address:** Phase 1 (MVP) - Critical for reliable sync

**Sources:**
- [High Latencies with Qdrant Go-Client](https://github.com/qdrant/qdrant/issues/5642)
- [Qdrant Go client documentation](https://pkg.go.dev/github.com/qdrant/go-client/qdrant)

---

### Pitfall 11: Using STDIO Transport for Production MCP
**What goes wrong:** STDIO transport (standard input/output) works great locally but cannot be deployed to production. Teams build with STDIO, then realize they can't deploy.

**Why it happens:** STDIO is the simplest transport, used in examples and local testing. Docs don't emphasize "local only."

**Consequences:**
- Late-stage rewrite to HTTP/SSE transport
- Different error handling (network vs process)
- Authentication/CORS issues appear only in production
- Testing environment diverges from production

**Prevention:**
- Use **Streamable HTTP** transport from day 1 (modern standard for 2026)
- Avoid SSE (legacy, deprecated in favor of Streamable HTTP)
- STDIO is fine for local testing tools, but not the primary transport
- Design for network deployment (assume latency, failures)
- Test against remote MCP server, not just local

**Detection:**
- Is your MCP server CLI-only?
- Are you using `exec` to spawn the server process?
- Does it listen on a TCP port?

**Phase to address:** Phase 1 (MVP) - Architecture decision

**Sources:**
- [MCP Server Transports comparison](https://docs.roocode.com/features/mcp/server-transports)
- [MCP Transport Protocols: stdio vs SSE vs StreamableHTTP](https://mcpcat.io/guides/comparing-stdio-sse-streamablehttp/)
- [Running Your Server - FastMCP](https://gofastmcp.com/deployment/running-server)

---

## Minor Pitfalls

Mistakes that cause annoyance but are fixable.

### Pitfall 12: Goroutine Leaks in Sync Job
**What goes wrong:** Spawning goroutines for parallel embedding/indexing without proper cleanup causes goroutine leaks, leading to memory growth and eventual OOM.

**Why it happens:**
- Blocked channels (sender has no receiver)
- Missing context cancellation
- Deferred cleanup not called

**Consequences:**
- Memory usage grows over time
- Fly.io machine eventually OOMs (kills process)
- Slow degradation (first sync: 100 goroutines, tenth sync: 10,000)

**Prevention:**
- Use `context.Context` for cancellation (cancel on error/completion)
- Always close channels with `defer close(ch)`
- Use buffered channels to avoid blocking senders
- Implement goroutine leak tests with `goleak`
- Monitor goroutine count in production (should be stable)
- Use `runtime.NumGoroutine()` in metrics

**Detection:**
- Is `runtime.NumGoroutine()` growing over time?
- Do sync jobs slow down after running for days?
- Profiling shows thousands of blocked goroutines?

**Phase to address:** Phase 2 (Post-MVP) - After concurrency is introduced

**Sources:**
- [Common Goroutine Leaks to Avoid](https://betterprogramming.pub/common-goroutine-leaks-that-you-should-avoid-fe12d12d6ee)
- [Preventing Goroutine Leaks with Context](https://dev.to/serifcolakel/go-concurrency-mastery-preventing-goroutine-leaks-with-context-timeout-cancellation-best-1lg0)
- [Finding a 50,000 Goroutine Leak](https://skoredin.pro/blog/golang/goroutine-leak-debugging)

---

### Pitfall 13: Fly.io Cold Start Delays
**What goes wrong:** When using auto-stop/auto-start, the first request after idle time waits for machine boot, causing 2-5 second delays. MCP clients may timeout.

**Why it happens:** To save costs, Fly.io suspends/stops idle machines. Starting from `suspended` is faster than `stopped`, but still noticeable.

**Consequences:**
- First query after idle feels slow
- MCP client timeouts if health checks fail
- Poor user experience ("Is the server down?")

**Prevention:**
- Set `min_machines_running = 1` in `fly.toml` (keeps one machine always running)
- Use `suspend` instead of `stop` (faster wake-up)
- Configure health check `grace_period` to allow for boot time
- Consider keep-alive pings from monitoring service
- Trade off: always-on costs ~$2/month for shared-cpu-1x

**Detection:**
- Do first requests after idle take >2 seconds?
- Are health checks failing on cold start?
- Is `auto_stop_machines` set to `stop` or `suspend`?

**Phase to address:** Phase 2 (Post-MVP) - Performance optimization

**Sources:**
- [Fly.io cold starts and timeouts](https://lunchpaillabs.com/blog/agent-swarm-hosting-flyio)
- [Autostop/autostart Machines](https://fly.io/docs/launch/autostop-autostart/)
- [Setting minimum instances](https://community.fly.io/t/setting-a-minimum-number-of-instances-to-keep-running-when-using-auto-start-stop/12861)

---

### Pitfall 14: No Circuit Breaker for Sync Job Failures
**What goes wrong:** When GitHub or OpenAI APIs fail repeatedly, the sync job keeps retrying forever, wasting resources and causing cascading failures.

**Why it happens:** Simple retry logic ("try 3 times") doesn't account for sustained outages.

**Consequences:**
- Sync job runs for hours, burning CPU/memory
- Partial updates leave index in inconsistent state
- No visibility into "has sync been failing for 2 days?"

**Prevention:**
- Implement circuit breaker pattern (Closed → Open → Half-open states)
- After N failures, stop trying (Open state)
- Periodically test if service is back (Half-open)
- Use exponential backoff with max delay (e.g., cap at 5 minutes)
- Alert on circuit breaker transitions (sync is failing!)
- Design for graceful degradation (serve stale data during outage)

**Detection:**
- Are sync jobs running for >1 hour?
- Do logs show hundreds of failed API calls?
- Is there an alert for "sync hasn't succeeded in 24 hours"?

**Phase to address:** Phase 2 (Post-MVP) - Resilience improvement

**Sources:**
- [Error handling in distributed systems](https://temporal.io/blog/error-handling-in-distributed-systems)
- [Strategies for handling partial failure](https://learn.microsoft.com/en-us/dotnet/architecture/microservices/implement-resilient-applications/partial-failure-strategies)
- [Handling Partial Failure in Microservices](https://medium.com/@dmosyan/handling-partial-failure-in-microservices-applications-2314d3093edb)

---

### Pitfall 15: Ignoring SQL Injection in Tool Arguments
**What goes wrong:** If you add a tool that queries metadata or logs (e.g., "search by author"), failing to sanitize inputs allows SQL injection attacks.

**Why it happens:** Go's `database/sql` makes parameterized queries easy, but teams sometimes build queries with string concatenation for flexibility.

**Consequences:**
- Data exfiltration (read entire database)
- Data corruption (modify/delete records)
- Stored prompt injection (inject malicious prompts into DB that get served later)

**Prevention:**
- Always use parameterized queries: `db.Query("SELECT * FROM docs WHERE id = ?", id)`
- Never use string concatenation: `"SELECT * FROM docs WHERE id = " + id` (vulnerable)
- Validate and sanitize all tool arguments
- Use query builders or ORMs that enforce safe patterns
- Run tools with least-privilege DB user (read-only where possible)

**Detection:**
- Are you concatenating strings in SQL queries?
- Have you tested tool arguments with `'; DROP TABLE docs; --`?
- Is there input validation on all tool parameters?

**Phase to address:** Phase 1 (MVP) - If adding database queries

**Sources:**
- [MCP Security: SQL Injection risks](https://www.reco.ai/learn/mcp-security)
- [MCP Best Practices and Common Pitfalls](https://research.aimultiple.com/mcp-security/)

---

### Pitfall 16: Fly.io Memory Limits Too Low
**What goes wrong:** Embedded Qdrant + Go runtime + embeddings in-memory require more RAM than default Fly.io allocation (256MB). Machine OOMs and restarts.

**Why it happens:** Teams start with minimum resources to save costs. Qdrant embedded mode loads vectors into memory.

**Consequences:**
- Process killed mid-request (HTTP 502)
- Sync job fails partway through
- Frequent restarts, poor reliability

**Prevention:**
- Profile memory usage during development (with production-scale data)
- Start with at least 1GB RAM for embedded Qdrant (scale up as needed)
- Monitor memory metrics: `fly scale memory 1024` if approaching limits
- Use `GOMEMLIMIT` to hint Go GC about available memory
- Consider switching to Qdrant server (separate process) if memory constrained

**Detection:**
- Are you seeing OOM kills in logs?
- Does memory usage climb to 100% before restart?
- Use `fly status` to check resource usage

**Phase to address:** Phase 1 (MVP) - Before production deployment

**Sources:**
- [Fly.io OOM troubleshooting](https://fly.io/docs/getting-started/troubleshooting/)
- [Machine Sizing guide](https://fly.io/docs/machines/guides-examples/machine-sizing/)
- [OOM Error discussion](https://community.fly.io/t/oom-error/13206)

---

## Phase-Specific Warnings

Pitfalls likely to appear in each development phase.

| Phase | Likely Pitfalls | Mitigation |
|-------|----------------|------------|
| **Phase 1: MVP** | Missing auth (Pitfall 1), volume data loss (3), rate limits (5), naive chunking (6), transport choice (11), SQL injection (15), memory limits (16) | Design for security from day 1. Test with production-scale data. Choose HTTP transport early. |
| **Phase 2: Retrieval Quality** | Embedding drift (4), no reranking (7), poor chunking (6) | Implement drift detection, hybrid search, chunk overlap testing. Monitor retrieval metrics. |
| **Phase 3: Cost Optimization** | Embedding cost explosion (9), batch timeouts (10) | Use batch API, track document changes, auto-batch inserts. |
| **Phase 4: Reliability** | Goroutine leaks (12), circuit breaker missing (14), cold starts (13) | Add leak tests, implement circuit breakers, configure min machines. |
| **Phase 5: Security Hardening** | Prompt injection (2), tool response limits (8) | Sanitize content, implement output filtering, configure response size limits. |

---

## Summary: Top 5 Must-Address

If you only fix 5 things, fix these:

1. **Add authentication to MCP server** (Pitfall 1) - Security breach risk
2. **Enable Fly volume snapshots** (Pitfall 3) - Data loss risk
3. **Implement GitHub rate limit handling** (Pitfall 5) - Sync reliability
4. **Use Streamable HTTP transport** (Pitfall 11) - Can't deploy otherwise
5. **Track embedding model versions** (Pitfall 4) - Retrieval quality degrades

---

## Confidence Assessment

| Area | Level | Notes |
|------|-------|-------|
| MCP Security | HIGH | Well-documented in 2026 with real-world incidents |
| Vector DB | MEDIUM | Qdrant-specific Go client issues verified; drift patterns confirmed |
| Fly.io | HIGH | Official docs + community discussions confirm volume/OOM risks |
| Sync Patterns | MEDIUM | General distributed systems patterns; GitHub rate limits confirmed |
| Go Concurrency | HIGH | Goroutine leak patterns well-established |

---

## Research Methodology

This research combined:
- **MCP security research**: 10+ sources from 2026 covering real-world security incidents
- **Vector DB best practices**: Qdrant docs, embedding drift research, chunking strategies
- **Fly.io deployment patterns**: Official docs + community forums for volume/memory issues
- **OpenAI API**: Official pricing/rate limit docs
- **Go patterns**: Goroutine leak detection, error handling best practices

All pitfalls are sourced from 2025-2026 documentation, blog posts, and issue trackers - not hypothetical concerns.
