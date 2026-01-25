# Phase 1: Storage Foundation - Research

**Researched:** 2026-01-25
**Domain:** Vector database storage with Qdrant and Go
**Confidence:** HIGH

## Summary

Phase 1 establishes vector storage infrastructure using Qdrant as a vector database with the official Go client. The standard approach is to run Qdrant as a Docker container on Fly.io with persistent volumes, using the official `github.com/qdrant/go-client` for all database operations. Collections store both document chunks (with embeddings) and full parent documents (without embeddings), linked via `parent_doc_id` fields. Embeddings are generated using OpenAI's `text-embedding-3-small` model (1536 dimensions) via the official `github.com/openai/openai-go/v3` client.

The research reveals that Qdrant has robust production patterns for health checking, payload indexing, and error handling. Critical findings include: payload fields used in filters MUST be indexed (or performance degrades catastrophically), Docker volumes on Fly.io require explicit configuration in fly.toml, and connection failures need retry logic with exponential backoff.

**Primary recommendation:** Use the official Qdrant Go client with gRPC (port 6334), create payload indexes for all filterable fields (path, repository, commit SHA), implement health checks on startup, and use the `cenkalti/backoff/v4` library for connection retry logic.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/qdrant/go-client | Latest | Official Qdrant Go client | Official SDK with gRPC support, actively maintained by Qdrant team |
| github.com/openai/openai-go/v3 | v3.16.0+ | OpenAI API client for embeddings | Official OpenAI Go SDK, supports text-embedding-3-small |
| qdrant/qdrant (Docker) | v1.16.0+ | Vector database container | Official Qdrant image, production-ready with monitoring |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/cenkalti/backoff/v4 | v4 | Exponential backoff retry logic | Connection failures, transient errors (required for production) |
| github.com/google/uuid | Latest | UUID generation for point IDs | Collision-free IDs in distributed systems (recommended over integers) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Qdrant | pgvector (PostgreSQL extension) | Simpler deployment but less optimized for vector search at scale |
| Official openai-go/v3 | sashabaranov/go-openai | Community library with more features but not official |
| UUIDs for point IDs | 64-bit integers | Integers use 8 bytes vs 16 for UUIDs but require manual ID management |

**Installation:**
```bash
go get -u github.com/qdrant/go-client
go get -u github.com/openai/openai-go/v3
go get -u github.com/cenkalti/backoff/v4
go get -u github.com/google/uuid
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── storage/             # Vector database operations
│   ├── qdrant.go       # Qdrant client wrapper
│   ├── models.go       # Point/payload structures
│   └── health.go       # Health check logic
├── embedding/           # OpenAI embedding generation
│   ├── client.go       # OpenAI client wrapper
│   └── batch.go        # Batch embedding logic
└── retry/              # Retry/backoff configuration
    └── backoff.go      # Exponential backoff config
```

### Pattern 1: Parent-Child Document Storage
**What:** Store document chunks (with embeddings) separately from full parent documents (without embeddings), linked via `parent_doc_id`
**When to use:** Always - this is the recommended RAG retrieval pattern for 2026
**Example:**
```go
// Source: https://qdrant.tech/documentation/concepts/payload/
// Parent document (no embedding, just metadata)
parentPoint := &qdrant.PointStruct{
    Id: qdrant.NewIDUUID(uuid.New().String()),
    Vectors: nil, // No vector for parent
    Payload: qdrant.NewValueMap(map[string]any{
        "type": "parent",
        "content": fullMarkdownContent,
        "path": "getting-started/installation.md",
        "repository": "cloudwego/eino",
        "commit_sha": "abc123",
        "indexed_at": "2026-01-25T10:30:00Z",
    }),
}

// Child chunk (with embedding)
chunkPoint := &qdrant.PointStruct{
    Id: qdrant.NewIDUUID(uuid.New().String()),
    Vectors: qdrant.NewVectors(embedding...), // 1536-dim vector
    Payload: qdrant.NewValueMap(map[string]any{
        "type": "chunk",
        "parent_doc_id": parentPoint.Id.GetUuid(),
        "chunk_index": 0,
        "header_path": "Installation > Prerequisites",
        "content": chunkContent,
        "path": "getting-started/installation.md",
        "repository": "cloudwego/eino",
    }),
}
```

### Pattern 2: Qdrant Client Initialization with Health Check
**What:** Initialize client with connection validation and fail-fast on startup
**When to use:** Application startup - critical infrastructure must be validated early
**Example:**
```go
// Source: https://github.com/qdrant/go-client
import (
    "context"
    "fmt"
    "github.com/qdrant/go-client/qdrant"
)

func NewQdrantStorage(host string, port int) (*QdrantStorage, error) {
    client, err := qdrant.NewClient(&qdrant.Config{
        Host: host,
        Port: port, // Use 6334 for gRPC
    })
    if err != nil {
        return nil, fmt.Errorf("create client: %w", err)
    }

    // Health check - fail fast if unreachable
    ctx := context.Background()
    _, err = client.HealthCheck(ctx)
    if err != nil {
        return nil, fmt.Errorf("health check failed: %w", err)
    }

    return &QdrantStorage{client: client}, nil
}
```

### Pattern 3: Collection Creation with Payload Indexes
**What:** Create collection with vector configuration AND payload indexes for filterable fields
**When to use:** First-time setup or collection initialization
**Example:**
```go
// Source: https://qdrant.tech/documentation/concepts/collections/
err := client.CreateCollection(ctx, &qdrant.CreateCollection{
    CollectionName: "documents",
    VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
        Size:     1536, // text-embedding-3-small dimension
        Distance: qdrant.Distance_Cosine,
    }),
})

// CRITICAL: Create payload indexes for all filterable fields
// Without these, filtering performance degrades catastrophically
client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
    CollectionName: "documents",
    FieldName:      "path",
    FieldType:      qdrant.FieldType_FieldTypeKeyword.Enum(),
})

client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
    CollectionName: "documents",
    FieldName:      "repository",
    FieldType:      qdrant.FieldType_FieldTypeKeyword.Enum(),
})

client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
    CollectionName: "documents",
    FieldName:      "commit_sha",
    FieldType:      qdrant.FieldType_FieldTypeKeyword.Enum(),
})

client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
    CollectionName: "documents",
    FieldName:      "type",
    FieldType:      qdrant.FieldType_FieldTypeKeyword.Enum(),
})
```

### Pattern 4: Retry Logic with Exponential Backoff
**What:** Wrap Qdrant operations with retry logic for transient failures
**When to use:** All network operations (upsert, query, health checks)
**Example:**
```go
// Source: https://pkg.go.dev/github.com/cenkalti/backoff/v4
import (
    "github.com/cenkalti/backoff/v4"
)

func (s *QdrantStorage) UpsertWithRetry(ctx context.Context, points []*qdrant.PointStruct) error {
    operation := func() error {
        _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
            CollectionName: "documents",
            Points:         points,
        })
        if err != nil {
            // Don't retry client errors (4xx equivalent)
            if isClientError(err) {
                return backoff.Permanent(err)
            }
            return err
        }
        return nil
    }

    backoffConfig := backoff.NewExponentialBackOff(
        backoff.WithInitialInterval(500 * time.Millisecond),
        backoff.WithMaxInterval(10 * time.Second),
        backoff.WithMaxElapsedTime(30 * time.Second),
    )

    return backoff.Retry(operation, backoff.WithContext(backoffConfig, ctx))
}
```

### Pattern 5: OpenAI Embedding Generation
**What:** Generate embeddings using OpenAI's text-embedding-3-small model
**When to use:** Before storing document chunks in Qdrant
**Example:**
```go
// Source: https://github.com/openai/openai-go
import (
    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
)

func NewEmbeddingClient() *openai.Client {
    return openai.NewClient(
        option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    )
}

func GenerateEmbedding(ctx context.Context, client *openai.Client, text string) ([]float32, error) {
    resp, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
        Input: openai.EmbeddingNewParamsInputUnion{
            OfString: openai.String(text),
        },
        Model: "text-embedding-3-small", // 1536 dimensions
    })
    if err != nil {
        return nil, err
    }

    // Convert float64 to float32 for Qdrant
    embedding := make([]float32, len(resp.Data[0].Embedding))
    for i, v := range resp.Data[0].Embedding {
        embedding[i] = float32(v)
    }
    return embedding, nil
}
```

### Pattern 6: Fly.io Volume Configuration
**What:** Configure persistent storage for Qdrant data in fly.toml
**When to use:** Deploying to Fly.io
**Example:**
```toml
# fly.toml
[mounts]
  source = "qdrant_data"
  destination = "/qdrant/storage"

[env]
  QDRANT__STORAGE__STORAGE_PATH = "/qdrant/storage"
  QDRANT__LOG_LEVEL = "INFO"
  QDRANT__SERVICE__GRPC_PORT = "6334"
  QDRANT__SERVICE__HTTP_PORT = "6333"
```

### Anti-Patterns to Avoid
- **Skipping payload indexes:** Filtering on non-indexed fields causes full table scans and 10-100x slowdowns
- **Using bind mounts on WSL/Windows:** Creates non-POSIX filesystems that corrupt Qdrant data - use Docker volumes
- **Creating separate collections per tenant:** Wastes resources - use single collection with tenant field filtering
- **Immediate retry without backoff:** Causes thundering herd and cascading failures
- **Storing only chunks without parent docs:** Loses full document context needed for retrieval

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exponential backoff retry | Custom sleep loops with exponential timers | `github.com/cenkalti/backoff/v4` | Handles jitter, context cancellation, max retries, and permanent errors correctly |
| UUID generation | String concatenation or hash-based IDs | `github.com/google/uuid` | RFC 4122 compliant, collision-free, battle-tested |
| Vector similarity search | Custom HNSW implementation | Qdrant's built-in HNSW index | Production-optimized with tunable parameters (m, ef_construct) |
| Payload filtering during search | Post-filter results in application code | Qdrant's filterable HNSW indexes | 10-100x faster by integrating filtering into search execution |
| Point ID management | Auto-incrementing integers | UUIDs | Avoids coordination overhead in distributed systems |
| Timestamp formatting | Custom RFC3339 string builders | Go's `time.RFC3339` constant | Handles timezones, leap seconds, and parsing correctly |

**Key insight:** Vector database operations have subtle edge cases (memory mapping, POSIX compliance, index selectivity estimation) that make custom implementations error-prone. Qdrant's production-hardened features handle these correctly.

## Common Pitfalls

### Pitfall 1: Missing Payload Indexes
**What goes wrong:** Queries with filters become 10-100x slower, timeouts occur, users think Qdrant is slow
**Why it happens:** Payload indexes are not auto-created - must be manually configured for each filterable field
**How to avoid:**
- Create payload index for EVERY field used in `Filter.Must`, `Filter.Should`, or `Filter.MustNot`
- Use `qdrant.FieldType_FieldTypeKeyword` for string fields (path, repository, commit_sha)
- Use `qdrant.FieldType_FieldTypeInteger` for numeric fields
- Create indexes immediately after collection creation
**Warning signs:** Query latency spikes when adding filters, Qdrant logs show "full scan" warnings

### Pitfall 2: Incompatible Filesystem (Data Corruption)
**What goes wrong:** Qdrant data resets to zeros after restart, "OutputTooSmall" panics occur
**Why it happens:** WSL-based Docker on Windows with bind mounts creates non-POSIX filesystems
**How to avoid:**
- Use Docker named volumes: `docker volume create qdrant_data`
- Mount as: `-v qdrant_data:/qdrant/storage` (not `/c/Users/...:/qdrant/storage`)
- For Fly.io: Always use `[mounts]` section in fly.toml, never bind mounts
**Warning signs:** Data loss after restart, Qdrant won't start with file system errors

### Pitfall 3: File Descriptor Limits Exceeded
**What goes wrong:** Qdrant crashes with "Too many files open" (OS error 24)
**Why it happens:** Each collection segment requires multiple open files, default limits are too low
**How to avoid:**
- Set `--ulimit nofile=10000:10000` in Docker run command
- For non-Docker: `ulimit -n 10000` before starting Qdrant
- Monitor file descriptor usage in production
**Warning signs:** Crashes when collection grows beyond ~1M points, "error 24" in logs

### Pitfall 4: No Health Check on Startup
**What goes wrong:** Application starts successfully but all Qdrant operations fail silently
**Why it happens:** Qdrant client constructor doesn't validate connectivity
**How to avoid:**
- Call `client.HealthCheck(ctx)` immediately after `NewClient()`
- Return error and fail startup if health check fails
- Don't start HTTP server until Qdrant is confirmed reachable
**Warning signs:** Application "works" but all vector searches return empty results

### Pitfall 5: Embedding Dimension Mismatch
**What goes wrong:** Upsert operations fail with "vector dimension mismatch" errors
**Why it happens:** Collection configured for 1536 dimensions but receiving different-sized vectors
**How to avoid:**
- Validate embedding length matches collection config (1536 for text-embedding-3-small)
- Check `len(embedding)` before creating PointStruct
- Use consistent embedding model across all operations
**Warning signs:** Upserts fail after collection creation works, errors mention vector dimensions

### Pitfall 6: Fly.io Volume Not Configured
**What goes wrong:** Data disappears after deployments or restarts
**Why it happens:** Root filesystem is ephemeral, volumes must be explicitly mounted in fly.toml
**How to avoid:**
- Create volume: `fly volumes create qdrant_data --size 10`
- Add `[mounts]` section to fly.toml with correct destination
- Verify QDRANT__STORAGE__STORAGE_PATH environment variable matches mount destination
**Warning signs:** Fresh database after every deploy, no data persistence

### Pitfall 7: Retrying Client Errors (4xx)
**What goes wrong:** Application retries forever on invalid requests (bad API key, malformed payload)
**Why it happens:** Retry logic doesn't distinguish permanent errors from transient failures
**How to avoid:**
- Wrap client errors with `backoff.Permanent(err)` to stop retries immediately
- Only retry 5xx-equivalent errors (connection failures, timeouts)
- Check error types before retrying
**Warning signs:** Logs show repeated identical errors, retries never succeed

## Code Examples

Verified patterns from official sources:

### Health Check Endpoint Pattern
```go
// Source: https://qdrant.tech/documentation/guides/monitoring/
func (s *QdrantStorage) Health(ctx context.Context) error {
    _, err := s.client.HealthCheck(ctx)
    if err != nil {
        return fmt.Errorf("qdrant health check failed: %w", err)
    }
    return nil
}

// Use in HTTP handler
http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
    if err := storage.Health(r.Context()); err != nil {
        http.Error(w, "unhealthy", http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ok"))
})
```

### Filtered Vector Search
```go
// Source: https://qdrant.tech/documentation/concepts/payload/
searchResult, err := client.Query(ctx, &qdrant.QueryPoints{
    CollectionName: "documents",
    Query:          qdrant.NewQuery(queryEmbedding...),
    Filter: &qdrant.Filter{
        Must: []*qdrant.Condition{
            qdrant.NewMatch("repository", "cloudwego/eino"),
            qdrant.NewMatch("type", "chunk"),
        },
    },
    Limit:       10,
    WithPayload: qdrant.NewWithPayload(true),
})
```

### Batch Upsert Pattern
```go
// Source: https://github.com/qdrant/go-client
// Batch operations for better performance
const batchSize = 100

for i := 0; i < len(allPoints); i += batchSize {
    end := i + batchSize
    if end > len(allPoints) {
        end = len(allPoints)
    }

    batch := allPoints[i:end]
    _, err := client.Upsert(ctx, &qdrant.UpsertPoints{
        CollectionName: "documents",
        Points:         batch,
    })
    if err != nil {
        return fmt.Errorf("batch upsert failed: %w", err)
    }
}
```

### RFC3339 Timestamp Handling
```go
// Source: https://pkg.go.dev/time
import "time"

// Store timestamps in RFC3339 format
indexedAt := time.Now().UTC().Format(time.RFC3339)
// Result: "2026-01-25T10:30:00Z"

// Parse from payload
payload := point.Payload["indexed_at"].GetStringValue()
timestamp, err := time.Parse(time.RFC3339, payload)
```

### Collection Auto-Creation Pattern
```go
// Source: https://qdrant.tech/documentation/concepts/collections/
func (s *QdrantStorage) EnsureCollection(ctx context.Context) error {
    // Check if collection exists
    collections, err := s.client.ListCollections(ctx)
    if err != nil {
        return fmt.Errorf("list collections: %w", err)
    }

    for _, coll := range collections.Collections {
        if coll.Name == "documents" {
            return nil // Already exists
        }
    }

    // Create collection
    err = s.client.CreateCollection(ctx, &qdrant.CreateCollection{
        CollectionName: "documents",
        VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
            Size:     1536,
            Distance: qdrant.Distance_Cosine,
        }),
    })
    if err != nil {
        return fmt.Errorf("create collection: %w", err)
    }

    // Create payload indexes
    return s.createPayloadIndexes(ctx)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Store chunks only | Parent-child document storage | 2025-2026 | Preserves full context while enabling granular search |
| Separate embedding + vector DB | Unified vector DB (Qdrant) | 2024+ | Simpler architecture, atomic updates |
| REST API for Qdrant | gRPC client (port 6334) | v1.0+ | 2-3x faster, better streaming support |
| Auto-increment IDs | UUIDs | 2025+ | Eliminates coordination in distributed systems |
| Post-filter search results | Filterable HNSW indexes | Qdrant v1.7+ | 10-100x faster filtered search |
| Manual retry logic | Structured backoff libraries | 2024+ | Handles edge cases (jitter, context, permanent errors) |
| text-embedding-ada-002 | text-embedding-3-small | Feb 2024 | Same 1536 dims, 5x cheaper ($0.02/1M tokens) |

**Deprecated/outdated:**
- **sashabaranov/go-openai:** Community library - official `openai-go/v3` now exists (as of 2024)
- **text-embedding-ada-002:** Superseded by text-embedding-3-small (same dimensions, cheaper, better multilingual)
- **Qdrant REST API for production:** gRPC is now standard (faster, better error handling)
- **Storing vectors in PostgreSQL (pgvector):** Viable but Qdrant is optimized for vector search workloads

## Open Questions

Things that couldn't be fully resolved:

1. **Fly.io Volume Snapshots Configuration**
   - What we know: Fly.io takes daily automatic snapshots with 5-day retention
   - What's unclear: How to configure custom retention (1-60 days) in fly.toml vs CLI
   - Recommendation: Use default 5-day retention initially, investigate custom retention in Phase 4 (deployment)

2. **Qdrant Sidecar vs Separate Service on Fly.io**
   - What we know: Fly.io supports both Docker sidecar and separate Fly apps
   - What's unclear: Performance tradeoffs, networking complexity differences
   - Recommendation: Start with separate Fly app (simpler debugging), consider sidecar if networking latency becomes issue

3. **Optimal HNSW Parameters for Documentation Search**
   - What we know: Default `m=16`, `ef_construct=100` work for most cases
   - What's unclear: Whether documentation corpus (10k-100k vectors) benefits from tuning
   - Recommendation: Use defaults initially, add tuning task if P99 latency exceeds 100ms

4. **Chunking Strategy Specifics**
   - What we know: 512 tokens with 25% overlap is 2026 best practice
   - What's unclear: Whether markdown headers should force chunk boundaries
   - Recommendation: Implement header-aware chunking in Phase 2 (processing), start with fixed 512-token chunks

## Sources

### Primary (HIGH confidence)
- [Qdrant Go Client GitHub](https://github.com/qdrant/go-client) - Official client installation and usage patterns
- [Qdrant Go Client Docs](https://pkg.go.dev/github.com/qdrant/go-client/qdrant) - API reference
- [Qdrant Collections Documentation](https://qdrant.tech/documentation/concepts/collections/) - Collection configuration and setup
- [Qdrant Payload Documentation](https://qdrant.tech/documentation/concepts/payload/) - Payload types, indexing, filtering
- [Qdrant Configuration Guide](https://qdrant.tech/documentation/guides/configuration/) - Environment variables, storage config
- [Qdrant Monitoring Documentation](https://qdrant.tech/documentation/guides/monitoring/) - Health checks, metrics endpoints
- [Qdrant Common Errors Guide](https://qdrant.tech/documentation/guides/common-errors/) - Troubleshooting and solutions
- [OpenAI Go Client GitHub](https://github.com/openai/openai-go) - Official embedding client
- [Backoff Library Docs](https://pkg.go.dev/github.com/cenkalti/backoff/v4) - Exponential backoff patterns
- [Fly.io Volumes Documentation](https://fly.io/docs/volumes/overview/) - Volume configuration and persistence

### Secondary (MEDIUM confidence)
- [Qdrant Production Best Practices](https://qdrant.tech/articles/vector-search-production/) - Operational guidance
- [Qdrant Filtering Guide](https://qdrant.tech/articles/vector-search-filtering/) - Advanced filtering patterns
- [Parent Document Retrieval (MongoDB)](https://www.mongodb.com/docs/atlas/ai-integrations/langchain/parent-document-retrieval/) - Pattern explanation
- [Document Chunking Strategies (Dataquest)](https://www.dataquest.io/blog/document-chunking-strategies-for-vector-databases/) - Chunking best practices 2026
- [Go Retry with Exponential Backoff (OneUptime)](https://oneuptime.com/blog/post/2026-01-07-go-retry-exponential-backoff/view) - Implementation patterns

### Tertiary (LOW confidence)
- [Qdrant on Fly.io Community Thread](https://community.fly.io/t/setting-up-qdrant-on-fly-io/24341) - Deployment discussion (March 2025)
- [Qdrant Point ID Best Practices Discussion](https://github.com/orgs/qdrant/discussions/3461) - Community recommendations
- [Vector Search Resource Optimization](https://qdrant.tech/articles/vector-search-resource-optimization/) - Performance tuning

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official libraries, well-documented, production-proven
- Architecture: HIGH - Patterns verified from official docs and community consensus
- Pitfalls: HIGH - Documented in official troubleshooting guides

**Research date:** 2026-01-25
**Valid until:** 2026-02-25 (30 days - stable domain with infrequent breaking changes)
