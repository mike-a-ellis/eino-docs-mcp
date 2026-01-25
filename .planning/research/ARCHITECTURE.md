# Architecture Patterns

**Domain:** MCP Documentation Server
**Researched:** 2026-01-25
**Confidence:** HIGH

## Recommended Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         MCP CLIENT                               │
│                      (Claude Desktop, etc)                       │
└────────────────────────┬────────────────────────────────────────┘
                         │ stdio (JSON-RPC 2.0)
                         │
┌────────────────────────▼────────────────────────────────────────┐
│                      MCP SERVER                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Transport Layer (stdio)                                  │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                        │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │  MCP Core (github.com/modelcontextprotocol/go-sdk)       │  │
│  │  - Tool registry                                          │  │
│  │  - Resource registry                                      │  │
│  │  - Request routing                                        │  │
│  └──────────┬──────────────────────────────┬─────────────────┘  │
│             │                              │                     │
│  ┌──────────▼─────────┐        ┌──────────▼─────────────────┐  │
│  │   Search Tool      │        │   Resource Provider        │  │
│  │  (semantic_search) │        │  (list documents/chunks)   │  │
│  └──────────┬─────────┘        └──────────┬─────────────────┘  │
│             │                              │                     │
│             └──────────────┬───────────────┘                     │
│                            │                                     │
│                   ┌────────▼────────┐                           │
│                   │  Query Handler   │                           │
│                   │  - Parse query   │                           │
│                   │  - Embed query   │                           │
│                   │  - Vector search │                           │
│                   │  - Format results│                           │
│                   └────────┬────────┘                           │
└────────────────────────────┼────────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼────────┐
│  Embedding     │  │  Vector Store   │  │  Sync Worker   │
│  Service       │  │  (Qdrant)       │  │  (Background)  │
│                │  │                 │  │                │
│  - OpenAI API  │  │  - Collections  │  │  - Poll GitHub │
│  - Batch embed │  │  - Search       │  │  - Fetch docs  │
│  - Rate limit  │  │  - Persist      │  │  - Trigger     │
│                │  │                 │  │    indexing    │
└────────────────┘  └────────┬────────┘  └───────┬────────┘
                             │                   │
                    ┌────────▼───────────────────▼────────┐
                    │       Indexing Pipeline             │
                    │  1. Fetch markdown from GitHub      │
                    │  2. Parse & chunk documents         │
                    │  3. Batch embed chunks              │
                    │  4. Upsert to Qdrant                │
                    └─────────────────────────────────────┘
                                     │
                    ┌────────────────▼────────────────────┐
                    │      Persistent Volume              │
                    │      /data/qdrant                   │
                    │  (Fly.io Volume - survives restart) │
                    └─────────────────────────────────────┘
```

### Component Boundaries

| Component | Responsibility | Communicates With | Package/Module |
|-----------|----------------|-------------------|----------------|
| **MCP Server** | Protocol handling, tool/resource routing | Client via stdio | `main`, `server` |
| **Search Tool** | Handle semantic search tool calls | MCP Core, Query Handler | `server/tools` |
| **Resource Provider** | Expose document metadata as MCP resources | MCP Core, Vector Store | `server/resources` |
| **Query Handler** | Orchestrate query → embedding → search → format | Embedding Service, Vector Store | `server/handlers` |
| **Embedding Service** | Generate embeddings via OpenAI API | Query Handler, Indexing Pipeline | `embeddings` |
| **Vector Store** | Qdrant client wrapper, CRUD operations | All query/indexing components | `storage/qdrant` |
| **Sync Worker** | Periodic GitHub polling, trigger indexing | Indexing Pipeline | `sync` |
| **Indexing Pipeline** | Fetch → Parse → Chunk → Embed → Store | Embedding Service, Vector Store | `indexer` |

### Data Flow

#### Query Flow (Read Path)

```
1. Client → MCP Server
   - Tool call: semantic_search(query="how to use EINO")

2. MCP Server → Search Tool
   - Route to registered handler

3. Search Tool → Query Handler
   - Parse and validate query

4. Query Handler → Embedding Service
   - Generate query embedding: OpenAI text-embedding-3-small API
   - Input: "how to use EINO"
   - Output: [1536] float32 vector

5. Query Handler → Vector Store (Qdrant)
   - Search request with:
     * query_vector: [1536] floats
     * limit: 5
     * score_threshold: 0.7
   - Uses cosine similarity

6. Vector Store → Query Handler
   - Returns matched chunks with:
     * text content
     * metadata (file, section, line numbers)
     * similarity scores

7. Query Handler → Search Tool
   - Format results as structured response

8. Search Tool → MCP Server → Client
   - Return formatted documentation snippets
```

#### Sync & Indexing Flow (Write Path)

```
1. Sync Worker (runs periodically, e.g., every 15min)
   - Poll GitHub API for latest commit SHA
   - Compare with stored SHA
   - If changed: trigger full re-index

2. Indexing Pipeline → GitHub
   - Fetch repository archive or use git-sync
   - Clone/pull to /tmp/docs

3. Indexing Pipeline → Document Processor
   - Walk directory tree
   - Find *.md files
   - Parse markdown with goldmark or gomarkdown

4. Document Processor → Chunker
   - Apply recursive chunking strategy:
     * Split on ## headers first (sections)
     * If section > 512 tokens: split on paragraphs
     * If paragraph > 512 tokens: split on sentences
   - Add 10-20% overlap between chunks
   - Target: 256-512 tokens per chunk

5. Chunker → Embedding Service
   - Batch chunks (max 2048 per request)
   - For each batch:
     * Call OpenAI embeddings API
     * Model: text-embedding-3-small
     * Cost: $0.00002 per 1k tokens
   - Rate limiting: respect OpenAI TPM quotas

6. Embedding Service → Vector Store (Qdrant)
   - Prepare point batches (100 points per upsert)
   - Each point:
     * id: hash(file_path + chunk_index)
     * vector: [1536] floats
     * payload: {
         file: "docs/getting-started.md",
         section: "Installation",
         text: "chunk content...",
         repo_sha: "abc123...",
         chunk_index: 0,
         total_chunks: 5
       }
   - Upsert in batches to collection "eino_docs"

7. Vector Store → Persistent Volume
   - Qdrant writes to /data/qdrant/storage
   - Persists across container restarts
   - Fly.io volume ensures durability
```

## Patterns to Follow

### Pattern 1: MCP Tool vs Resource Design

**What:** Distinguish between executable actions (tools) and contextual data (resources)

**When:** Deciding what to expose via MCP

**Implementation:**

```go
// TOOL: User/model controls invocation
func RegisterSearchTool(server *mcp.Server) {
    mcp.AddTool(server, &mcp.Tool{
        Name: "semantic_search",
        Description: "Search EINO documentation semantically",
    }, handleSemanticSearch)
}

// RESOURCE: Application controls loading (pre-conversation context)
func RegisterDocResources(server *mcp.Server) {
    // Could expose: document list, collection stats, recent updates
    // Client decides when to load this context
}
```

**Rationale:** Tools = model-controlled actions, Resources = app-controlled context. For documentation search, semantic_search is clearly a tool since the model should autonomously decide when to search based on user queries.

### Pattern 2: Embedding Batching with Rate Limiting

**What:** Batch embeddings while respecting OpenAI rate limits

**When:** Processing documents during indexing

**Implementation:**

```go
type EmbeddingService struct {
    client     *openai.Client
    batchSize  int // 2048 max
    rateLimiter *rate.Limiter // tokens per minute
}

func (e *EmbeddingService) EmbedBatch(texts []string) ([][]float32, error) {
    // 1. Validate batch size
    if len(texts) > 2048 {
        return nil, errors.New("batch too large")
    }

    // 2. Wait for rate limiter
    tokens := estimateTokens(texts)
    e.rateLimiter.WaitN(ctx, tokens)

    // 3. Call OpenAI API
    resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
        Input: texts,
        Model: openai.AdaEmbeddingV3Small,
    })

    return extractVectors(resp), err
}
```

**Benefits:** Prevents API throttling, optimizes costs, maintains predictable performance

### Pattern 3: Qdrant Point ID Determinism

**What:** Use deterministic hashing for point IDs to enable idempotent upserts

**When:** Storing document chunks in Qdrant

**Implementation:**

```go
func generatePointID(filePath string, chunkIndex int, repoSHA string) uint64 {
    // Include repo SHA to invalidate old versions
    data := fmt.Sprintf("%s:%d:%s", filePath, chunkIndex, repoSHA)
    hash := xxhash.Sum64String(data)
    return hash
}

func upsertChunk(chunk DocumentChunk, embedding []float32) error {
    pointID := generatePointID(chunk.File, chunk.Index, chunk.RepoSHA)

    return qdrantClient.Upsert(ctx, &qdrant.UpsertPoints{
        CollectionName: "eino_docs",
        Points: []*qdrant.PointStruct{
            {
                Id: qdrant.NewID(pointID),
                Vectors: qdrant.NewVectors(embedding...),
                Payload: qdrant.NewValueMap(map[string]any{
                    "file": chunk.File,
                    "section": chunk.Section,
                    "text": chunk.Text,
                    "repo_sha": chunk.RepoSHA,
                    "chunk_index": chunk.Index,
                }),
            },
        },
    })
}
```

**Benefits:** Re-indexing same version = no-op, version changes = automatic replacement, supports incremental updates

### Pattern 4: Recursive Document Chunking

**What:** Split documents hierarchically (headers → paragraphs → sentences)

**When:** Processing markdown documentation for embedding

**Implementation:**

```go
type ChunkStrategy struct {
    MaxTokens int     // 512
    Overlap   float64 // 0.15 (15%)
}

func (s *ChunkStrategy) ChunkMarkdown(doc *ast.Document) []Chunk {
    chunks := []Chunk{}

    // 1. Split on H2 headers first
    sections := splitOnHeaders(doc, 2)

    for _, section := range sections {
        tokens := countTokens(section.Text)

        if tokens <= s.MaxTokens {
            // Section fits in one chunk
            chunks = append(chunks, newChunk(section))
        } else {
            // 2. Split section on paragraphs
            paras := splitOnParagraphs(section)

            for _, para := range paras {
                if countTokens(para.Text) <= s.MaxTokens {
                    chunks = append(chunks, newChunk(para))
                } else {
                    // 3. Split paragraph on sentences
                    sentences := splitOnSentences(para)
                    chunks = append(chunks, groupSentences(sentences, s.MaxTokens)...)
                }
            }
        }
    }

    // 4. Add overlap between chunks
    return addOverlap(chunks, s.Overlap)
}
```

**Benefits:** Preserves semantic boundaries, maintains context coherence, handles varied document structures

### Pattern 5: Qdrant Embedded Mode with Persistent Storage

**What:** Run Qdrant in embedded mode with volume-backed persistence

**When:** Deploying to Fly.io or any container environment

**Implementation:**

```go
import "github.com/qdrant/go-client/qdrant"

func initVectorStore() (*qdrant.Client, error) {
    // Check if Qdrant data exists
    dataDir := "/data/qdrant"
    if err := os.MkdirAll(dataDir, 0755); err != nil {
        return nil, err
    }

    // For embedded mode, we'd run Qdrant binary as subprocess
    // pointing it to the persistent volume
    // OR use Qdrant as separate service on localhost

    client, err := qdrant.NewClient(&qdrant.Config{
        Host: "localhost",
        Port: 6334,
        APIKey: "", // Not needed for local
    })

    return client, err
}
```

**Fly.io Configuration (fly.toml):**

```toml
[mounts]
  source = "qdrant_data"
  destination = "/data/qdrant"

[env]
  QDRANT_STORAGE_PATH = "/data/qdrant/storage"
  QDRANT_SNAPSHOTS_PATH = "/data/qdrant/snapshots"
```

**Benefits:** Data persists across restarts, survives deployments, enables stateful operation

### Pattern 6: Collection Schema for Document Search

**What:** Design Qdrant collection schema optimized for semantic document search

**When:** Initializing vector store

**Implementation:**

```go
func ensureCollection(client *qdrant.Client) error {
    collectionName := "eino_docs"

    // Check if exists
    exists, err := client.CollectionExists(ctx, collectionName)
    if err != nil {
        return err
    }

    if !exists {
        // Create with optimized schema
        err = client.CreateCollection(ctx, &qdrant.CreateCollection{
            CollectionName: collectionName,
            VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
                Size:     1536, // text-embedding-3-small
                Distance: qdrant.Distance_Cosine,
                OnDisk:   pointer(true), // Store on disk for large datasets
            }),
            OptimizersConfig: &qdrant.OptimizersConfigDiff{
                IndexingThreshold: pointer(uint64(10000)),
            },
        })

        if err != nil {
            return err
        }

        // Create payload index for filtering
        client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndex{
            CollectionName: collectionName,
            FieldName:      "file",
            FieldType:      qdrant.FieldType_Keyword,
        })

        client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndex{
            CollectionName: collectionName,
            FieldName:      "repo_sha",
            FieldType:      qdrant.FieldType_Keyword,
        })
    }

    return nil
}
```

**Schema Design Rationale:**
- **Vector on_disk: true** - Large doc sets exceed memory, disk-backed with mmap is acceptable for query latency
- **Cosine distance** - Standard for text embeddings, normalized vectors
- **Indexed fields** - Enable filtering by file path (scoped search) or version (repo_sha)

## Anti-Patterns to Avoid

### Anti-Pattern 1: Synchronous Indexing on Query Path

**What goes wrong:** Blocking user queries while re-indexing documents

**Why bad:** User waits minutes for simple search, terrible UX, violates MCP timeout expectations

**Instead:**
- Run indexing in background worker (separate goroutine)
- Serve queries from existing index during re-index
- Swap to new index atomically after completion
- Use Qdrant collection aliasing or versioned collections

### Anti-Pattern 2: In-Memory Vector Storage Without Persistence

**What goes wrong:** Using Qdrant in-memory mode (`:memory:`) or forgetting to mount Fly volume

**Why bad:** Every restart loses all indexed documents, re-indexing on every deploy is expensive (OpenAI costs + time)

**Instead:**
- Always configure persistent storage path for Qdrant
- Mount Fly.io volume at consistent path (`/data/qdrant`)
- Verify persistence with health check after restart
- Monitor storage usage and set up volume snapshots

### Anti-Pattern 3: Giant Chunks or Tiny Chunks

**What goes wrong:**
- **Giant chunks (>1000 tokens):** Lose semantic precision, return too much context
- **Tiny chunks (<100 tokens):** Lose semantic coherence, return fragmented results

**Why bad:** Poor search relevance, either too broad or too fragmented

**Instead:**
- Target 256-512 tokens per chunk
- Use recursive chunking to preserve structure
- Add 10-20% overlap for context continuity
- Benchmark retrieval quality with sample queries

### Anti-Pattern 4: Single Embedding Call Per Chunk

**What goes wrong:** Calling OpenAI API for each chunk individually

**Why bad:**
- Slow: 1000 chunks = 1000 API calls (serial)
- Expensive: Pay per-request overhead
- Rate-limited: Quickly hit API limits

**Instead:**
- Batch up to 2048 inputs per API call
- Process batches in parallel (goroutines)
- Implement rate limiter for TPM quotas
- Monitor costs with structured logging

### Anti-Pattern 5: No Version Tracking in Vector Store

**What goes wrong:** Re-indexing without clearing old chunks from previous versions

**Why bad:**
- Search returns mix of old and new documentation
- Misleading results for updated features
- Storage bloat with duplicate/stale data

**Instead:**
- Include `repo_sha` in every point payload
- Filter queries by latest SHA
- OR: Delete collection and re-create on version change
- OR: Use versioned collections (`eino_docs_v2`) and swap alias

### Anti-Pattern 6: Exposing Raw Qdrant API via MCP

**What goes wrong:** Creating MCP tools that directly expose Qdrant operations (create_collection, delete_points, etc.)

**Why bad:**
- Security risk: clients could corrupt data
- Violates abstraction: MCP should provide semantic operations, not storage primitives
- Complexity leak: clients shouldn't need to understand vector DB internals

**Instead:**
- Expose high-level semantic tools: `semantic_search`, `find_similar`, `get_document`
- Keep storage operations internal to server
- Validate and sanitize all tool inputs
- Log tool usage for monitoring

## Scalability Considerations

| Concern | At 100 docs | At 10K docs | At 1M docs |
|---------|-------------|-------------|------------|
| **Indexing Time** | <1 minute (embed + store) | ~30 minutes (batched) | ~24 hours (distributed) |
| **Storage Size** | ~10MB (vectors + metadata) | ~1GB (vectors + metadata) | ~100GB (use Qdrant on_disk) |
| **Query Latency** | <100ms (in-memory) | <200ms (disk-backed, warm) | <500ms (may need quantization) |
| **Embedding Costs** | ~$0.02 (100 docs × 200 tokens avg) | ~$20 (10K docs × 200 tokens) | ~$2000 (1M docs × 200 tokens) |
| **Sync Strategy** | Poll every 15min | Poll every hour | Webhook-triggered |
| **Qdrant Mode** | Embedded or Docker | Docker on separate machine | Distributed cluster |
| **Fly.io Resources** | shared-cpu-1x, 256MB, 1GB volume | performance-2x, 2GB, 10GB volume | Multiple machines + external Qdrant |

### Scaling Checkpoints

**Phase 1: MVP (Target: 100-500 docs)**
- Embedded Qdrant on same machine
- In-memory vectors (if fits in RAM)
- Synchronous indexing (acceptable for small doc sets)
- Simple polling sync (15min interval)

**Phase 2: Production (Target: 1K-10K docs)**
- Qdrant as separate service (Docker sidecar or separate machine)
- Disk-backed vectors with mmap
- Background indexing worker
- Smart polling (check GitHub ETag before full fetch)
- Monitor embedding costs

**Phase 3: Scale (Target: 10K+ docs)**
- Distributed Qdrant cluster (if >100GB)
- Quantized vectors (reduce storage 4x, slight quality loss)
- Incremental indexing (only changed files)
- Webhook-based sync (GitHub push notifications)
- Caching layer for frequent queries
- Consider hybrid search (sparse + dense vectors)

## Build Order (Dependency Graph)

This order minimizes rework and enables incremental testing:

```
1. Storage Layer
   └─ Qdrant client wrapper (storage/qdrant)
      └─ Collection initialization
      └─ Upsert/search operations
      └─ Persistence configuration

2. Embedding Service
   └─ OpenAI client wrapper (embeddings)
      └─ Batch embedding with rate limiting
      └─ Error handling and retries
      └─ Cost tracking

3. Document Processing
   └─ Markdown parser integration
   └─ Chunking strategy implementation
   └─ Metadata extraction

4. Indexing Pipeline
   └─ Fetch documents (GitHub or local)
   └─ Process → Chunk → Embed → Store
   └─ Progress tracking and logging

   ↓ At this point: Can manually trigger indexing and query Qdrant directly

5. MCP Server Core
   └─ Server initialization (github.com/modelcontextprotocol/go-sdk)
   └─ Stdio transport setup
   └─ Basic health check

6. MCP Tools
   └─ semantic_search tool
      └─ Input validation
      └─ Query embedding
      └─ Vector search
      └─ Result formatting

   ↓ At this point: Can test end-to-end via MCP client

7. MCP Resources (Optional)
   └─ Document list resource
   └─ Collection stats resource

8. Sync Worker
   └─ GitHub polling logic
   └─ Change detection (SHA comparison)
   └─ Trigger indexing
   └─ Scheduling (cron or ticker)

   ↓ At this point: Full system operational

9. Deployment Configuration
   └─ Dockerfile
   └─ fly.toml with volume mounts
   └─ Environment variable configuration
   └─ Health checks and monitoring
```

### Why This Order?

- **Storage first:** Everything depends on it, easiest to test in isolation
- **Embedding second:** Needed for both indexing and queries, can test with sample texts
- **Document processing third:** Can test chunking strategy before full pipeline
- **Indexing fourth:** Validates storage + embedding + processing together
- **MCP server fifth:** Now have data to serve, can focus on protocol correctness
- **Sync worker last:** Everything else must work first, deployment concern

## Key Interfaces Between Components

### Interface 1: MCP Tool Handler → Query Handler

```go
// MCP tool signature (enforced by go-sdk)
func handleSemanticSearch(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input SearchInput,
) (*mcp.CallToolResult, SearchOutput, error)

// SearchInput is auto-parsed from JSON
type SearchInput struct {
    Query      string `json:"query" jsonschema:"required,description=Search query"`
    Limit      int    `json:"limit" jsonschema:"description=Max results (default 5)"`
    Threshold  float32 `json:"threshold" jsonschema:"description=Min similarity (0-1)"`
}

// SearchOutput is auto-serialized to JSON
type SearchOutput struct {
    Results []SearchResult `json:"results"`
}

// Internal handler interface
type QueryHandler interface {
    Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}
```

### Interface 2: Query Handler → Embedding Service

```go
type EmbeddingService interface {
    // Embed single query (for search)
    EmbedQuery(ctx context.Context, text string) ([]float32, error)

    // Embed batch (for indexing)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type SearchResult struct {
    Text       string            // Chunk content
    Score      float32           // Similarity score
    Metadata   map[string]string // File, section, etc.
}
```

### Interface 3: Query Handler → Vector Store

```go
type VectorStore interface {
    // Search by vector
    Search(ctx context.Context, req SearchRequest) ([]Point, error)

    // Upsert points (for indexing)
    Upsert(ctx context.Context, points []Point) error

    // Delete by filter (for version cleanup)
    DeleteByFilter(ctx context.Context, filter Filter) error

    // Collection management
    EnsureCollection(ctx context.Context, schema CollectionSchema) error
}

type SearchRequest struct {
    CollectionName string
    QueryVector    []float32
    Limit          int
    ScoreThreshold float32
    Filter         *Filter // Optional: filter by file, repo_sha, etc.
}

type Point struct {
    ID      uint64
    Vector  []float32
    Payload map[string]any
}
```

### Interface 4: Indexing Pipeline → Document Processor

```go
type DocumentProcessor interface {
    // Parse markdown and extract chunks
    Process(ctx context.Context, doc Document) ([]Chunk, error)
}

type Document struct {
    Path    string // Relative path in repo
    Content []byte // Raw markdown
    SHA     string // Repo commit SHA
}

type Chunk struct {
    File       string
    Section    string // H2 header text
    Text       string // Chunk content
    Index      int    // Chunk number within file
    TotalChunks int   // Total chunks in file
    RepoSHA    string // For version tracking
}
```

### Interface 5: Sync Worker → Indexing Pipeline

```go
type SyncWorker interface {
    // Start periodic sync
    Start(ctx context.Context) error

    // Manually trigger sync
    TriggerSync(ctx context.Context) error
}

type IndexingPipeline interface {
    // Index entire repository
    IndexRepository(ctx context.Context, repoPath string, sha string) error

    // Index specific files (for incremental updates)
    IndexFiles(ctx context.Context, files []string, sha string) error
}
```

## Deployment Architecture (Fly.io)

### Single Machine Deployment (MVP)

```
┌─────────────────────────────────────────────┐
│         Fly.io Machine (shared-cpu-1x)      │
│                                             │
│  ┌──────────────────────────────────────┐  │
│  │  MCP Server Process                   │  │
│  │  - Listens on stdio                   │  │
│  │  - Serves tools/resources             │  │
│  └──────────────────────────────────────┘  │
│                                             │
│  ┌──────────────────────────────────────┐  │
│  │  Qdrant Process (localhost:6334)     │  │
│  │  - Storage: /data/qdrant/storage     │  │
│  │  - Config: on_disk=true               │  │
│  └──────────────────────────────────────┘  │
│                                             │
│  ┌──────────────────────────────────────┐  │
│  │  Sync Worker (goroutine)              │  │
│  │  - Polls GitHub every 15min           │  │
│  │  - Triggers indexing on change        │  │
│  └──────────────────────────────────────┘  │
│                                             │
└─────────────────┬───────────────────────────┘
                  │
           ┌──────▼──────┐
           │ Fly Volume  │
           │ qdrant_data │
           │   (1GB)     │
           └─────────────┘
```

**fly.toml:**

```toml
app = "eino-docs-mcp"
primary_region = "sjc"

[build]
  dockerfile = "Dockerfile"

[env]
  QDRANT_STORAGE_PATH = "/data/qdrant/storage"
  GITHUB_REPO = "getzep/eino"
  SYNC_INTERVAL = "15m"

[mounts]
  source = "qdrant_data"
  destination = "/data/qdrant"

[[services]]
  internal_port = 8080
  protocol = "tcp"

  [[services.ports]]
    handlers = ["http"]
    port = 80

  [[services.ports]]
    handlers = ["tls", "http"]
    port = 443
```

### Process Supervision

The main Go binary should supervise both the MCP server and Qdrant process:

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // 1. Start Qdrant process
    qdrantCmd := startQdrant(ctx, "/data/qdrant")
    defer qdrantCmd.Process.Kill()

    // Wait for Qdrant to be ready
    waitForQdrant(ctx, "localhost:6334")

    // 2. Initialize services
    qdrantClient := initVectorStore()
    embeddingService := initEmbeddings()
    queryHandler := initQueryHandler(qdrantClient, embeddingService)

    // 3. Start sync worker in background
    syncWorker := initSyncWorker()
    go syncWorker.Start(ctx)

    // 4. Start MCP server (blocks)
    mcpServer := initMCPServer(queryHandler)
    mcpServer.Run(ctx, &mcp.StdioTransport{})
}
```

## Monitoring and Observability

Key metrics to track:

1. **Query Metrics**
   - Search latency (p50, p95, p99)
   - Results returned per query
   - Embedding API latency
   - Qdrant search latency

2. **Indexing Metrics**
   - Documents indexed per sync
   - Indexing duration
   - Embedding API costs
   - Failed chunks (errors)

3. **Storage Metrics**
   - Qdrant collection size
   - Volume disk usage
   - Point count
   - Memory usage

4. **Sync Metrics**
   - Sync frequency
   - Detected changes
   - Failed syncs
   - Time since last successful sync

Implement structured logging with these metrics for debugging and cost tracking.

## Sources

### MCP Protocol & Go SDK
- [Official Go SDK for Model Context Protocol](https://github.com/modelcontextprotocol/go-sdk)
- [Building a Model Context Protocol Server in Go - Navendu Pottekkat](https://navendu.me/posts/mcp-server-go/)
- [Model Context Protocol: Go Implementation Tutorial](https://prasanthmj.github.io/ai/mcp-go/)
- [MCP Resources vs Tools Comparison - Medium](https://medium.com/@laurentkubaski/mcp-resources-explained-and-how-they-differ-from-mcp-tools-096f9d15f767)
- [Understanding MCP Features Guide - WorkOS](https://workos.com/blog/mcp-features-guide)

### Qdrant Vector Database
- [Qdrant Go Client - GitHub](https://github.com/qdrant/go-client)
- [Qdrant Configuration Documentation](https://qdrant.tech/documentation/guides/configuration/)
- [Qdrant Collections Documentation](https://qdrant.tech/documentation/concepts/collections/)
- [Qdrant Storage Documentation](https://qdrant.tech/documentation/concepts/storage/)
- [Qdrant Payload Indexing](https://qdrant.tech/documentation/concepts/payload/)

### OpenAI Embeddings
- [OpenAI Embeddings API Reference](https://platform.openai.com/docs/api-reference/embeddings)
- [go-openai Client Library](https://github.com/sashabaranov/go-openai/blob/master/embeddings.go)
- [New Embedding Models Announcement - OpenAI](https://openai.com/index/new-embedding-models-and-api-updates/)

### Document Chunking Strategies
- [Chunking Strategies for LLM Applications - Pinecone](https://www.pinecone.io/learn/chunking-strategies/)
- [Chunking Strategies for RAG - Weaviate](https://weaviate.io/blog/chunking-strategies-for-rag)
- [RAG Document Chunking Best Practices - Airbyte](https://airbyte.com/agentic-data/ag-document-chunking-best-practices)
- [Chunking for RAG Best Practices - Unstructured](https://unstructured.io/blog/chunking-for-rag-best-practices)

### Markdown Processing in Go
- [goldmark - Markdown Parser](https://github.com/yuin/goldmark)
- [gomarkdown/markdown - Parser and Renderer](https://github.com/gomarkdown/markdown)
- [Advanced Markdown Processing in Go](https://blog.kowalczyk.info/article/cxn3/advanced-markdown-processing-in-go.html)

### Fly.io Deployment
- [Fly Volumes Overview](https://fly.io/docs/volumes/overview/)
- [Add Volume Storage to Fly Launch App](https://fly.io/docs/launch/volume-storage/)
- [Fly Volumes Management](https://fly.io/docs/volumes/volume-manage/)

### GitHub Repository Sync
- [gpoll - Go Git Repository Polling Library](https://github.com/eddieowens/gpoll)
- [git-sync - Kubernetes Git Sync Sidecar](https://github.com/kubernetes/git-sync)
