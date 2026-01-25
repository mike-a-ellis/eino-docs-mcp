# Phase 4: Observability & Manual Sync - Research

**Researched:** 2026-01-25
**Domain:** Go MCP server observability, Qdrant collection statistics, CLI tooling
**Confidence:** HIGH

## Summary

This phase adds observability and manual sync capabilities to the MCP server. The standard approach involves adding a `get_index_status` MCP tool using the handler factory pattern, and a `sync` CLI command using Cobra. Qdrant's Go client provides `GetCollectionInfo()` for statistics (points_count, indexed_vectors_count, segments_count), and the existing indexer pipeline can be wrapped with sync state management using atomic.Bool flags to block queries during re-indexing.

The key technical challenges are: (1) preventing queries during sync using application-level state flags rather than Qdrant locking, (2) comparing GitHub HEAD commit with indexed commit using go-github's CompareCommits API, and (3) structuring error categories for observability while keeping the implementation simple.

**Primary recommendation:** Use atomic.Bool for sync-in-progress state, GetCollectionInfo for counts, ListDocumentPaths for indexed paths, and Cobra subcommands for CLI structure. Block queries at handler level when sync is active, return structured errors with category+message format.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/qdrant/go-client | v1.16.2 | Collection statistics via GetCollectionInfo | Official Qdrant Go client, already in use |
| github.com/google/go-github/v81 | v81.0.0 | Compare commits for staleness detection | Official GitHub API client, already in use |
| sync/atomic | stdlib | Lock-free sync state flag (atomic.Bool) | Standard library, zero dependencies |
| time | stdlib | RFC3339 timestamp formatting | Standard library, ISO 8601 compatible |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/spf13/cobra | latest | CLI command structure (if adding CLI) | Most popular Go CLI framework, used by kubectl, hugo, github cli |
| log/slog | stdlib | Structured logging for sync progress | Standard library structured logger |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| atomic.Bool | sync.Mutex | Mutex is heavier, atomic.Bool is sufficient for simple boolean state |
| Cobra CLI | stdlib flag package | Cobra provides better subcommand structure and help generation |
| Application-level blocking | Qdrant collection locking | Qdrant doesn't provide collection-level read locks during writes |

**Installation:**
```bash
# Cobra only if adding CLI commands
go get github.com/spf13/cobra@latest
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/
├── mcp-server/
│   └── main.go          # MCP server entry point (existing)
└── sync/                # NEW: CLI sync command
    └── main.go          # Sync command entry point

internal/
├── mcp/
│   ├── handlers.go      # Add makeStatusHandler
│   ├── types.go         # Add StatusInput/StatusOutput
│   └── server.go        # Add sync state to Server struct
├── indexer/
│   └── pipeline.go      # Existing IndexAll method
└── storage/
    └── qdrant.go        # Existing ListDocumentPaths, GetCommitSHA
```

### Pattern 1: Atomic Boolean for Sync State
**What:** Use atomic.Bool to track whether sync is in progress, checked by query handlers to block requests
**When to use:** Lightweight state flag for concurrent access without lock contention
**Example:**
```go
// Source: https://pkg.go.dev/sync/atomic
import "sync/atomic"

type Server struct {
    server   *mcp.Server
    storage  *storage.QdrantStorage
    embedder *embedding.Embedder
    syncInProgress atomic.Bool  // NEW: sync state flag
}

// In query handlers (search, fetch, list)
func (s *Server) checkSyncState() error {
    if s.syncInProgress.Load() {
        return errors.New("sync_in_progress: index is currently being rebuilt")
    }
    return nil
}

// In sync command
func (s *Server) runSync(ctx context.Context) error {
    if !s.syncInProgress.CompareAndSwap(false, true) {
        return errors.New("sync already in progress")
    }
    defer s.syncInProgress.Store(false)
    // ... perform sync
}
```

### Pattern 2: Collection Statistics via GetCollectionInfo
**What:** Use Qdrant's GetCollectionInfo method to retrieve counts and status
**When to use:** For observability, monitoring, status reporting
**Example:**
```go
// Source: https://pkg.go.dev/github.com/qdrant/go-client/qdrant
info, err := storage.client.GetCollectionInfo(ctx, storage.CollectionName)
if err != nil {
    return nil, fmt.Errorf("failed to get collection info: %w", err)
}

// Available metrics (counts are approximate, not exact)
pointsCount := info.GetPointsCount()           // Total points (docs + chunks)
indexedVectors := info.GetIndexedVectorsCount() // Vectors in HNSW index
segmentsCount := info.GetSegmentsCount()       // Number of segments
status := info.GetStatus()                     // green/yellow/grey/red
```

### Pattern 3: GitHub Commit Comparison for Staleness
**What:** Use go-github CompareCommits API to check how many commits indexed SHA is behind HEAD
**When to use:** To warn about stale data
**Example:**
```go
// Source: https://pkg.go.dev/github.com/google/go-github/v81/github
comparison, _, err := client.Repositories.CompareCommits(
    ctx,
    owner,
    repo,
    indexedCommitSHA,  // base
    "HEAD",            // head (or branch name like "main")
    nil,
)
if err != nil {
    // Handle network error
    return 0, err
}

commitsAhead := comparison.AheadBy  // How many commits HEAD is ahead
commitsBehing := comparison.BehindBy // Should be 0 if base is in history
```

### Pattern 4: Cobra CLI Subcommands
**What:** Structure CLI with root command and subcommands using Cobra
**When to use:** For CLI tools with multiple commands
**Example:**
```go
// Source: https://oneuptime.com/blog/post/2026-01-07-go-cobra-cli/view
// cmd/sync/main.go
package main

import (
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "eino-sync",
    Short: "EINO documentation sync tool",
}

var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Re-index all documentation from GitHub",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Initialize dependencies (storage, fetcher, embedder, etc.)
        // Run pipeline.IndexAll(ctx)
        return nil
    },
}

func init() {
    rootCmd.AddCommand(syncCmd)
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### Pattern 5: MCP Handler Factory with Sync State Check
**What:** Extend existing handler factory pattern to include sync state check
**When to use:** All query handlers that need to block during sync
**Example:**
```go
// Source: Existing codebase pattern in internal/mcp/handlers.go
func makeStatusHandler(
    store *storage.QdrantStorage,
    syncInProgress *atomic.Bool,
) func(context.Context, *mcp.CallToolRequest, StatusInput) (*mcp.CallToolResult, StatusOutput, error) {
    return func(ctx context.Context, req *mcp.CallToolRequest, input StatusInput) (
        *mcp.CallToolResult, StatusOutput, error,
    ) {
        // Get collection info
        info, err := store.client.GetCollectionInfo(ctx, storage.CollectionName)
        if err != nil {
            return nil, StatusOutput{}, fmt.Errorf("qdrant_error: failed to get collection info: %w", err)
        }

        // Get indexed document paths
        paths, err := store.ListDocumentPaths(ctx, defaultRepository)
        if err != nil {
            return nil, StatusOutput{}, fmt.Errorf("qdrant_error: failed to list documents: %w", err)
        }

        // Return status
        return nil, StatusOutput{
            TotalDocs:      len(paths),
            TotalChunks:    int(info.GetPointsCount()) - len(paths), // points - docs = chunks
            IndexedPaths:   paths,
            LastSyncTime:   // from GetCommitSHA + timestamp from any doc
            SourceCommit:   // from GetCommitSHA
            SyncInProgress: syncInProgress.Load(),
        }, nil
    }
}
```

### Pattern 6: RFC3339 Timestamp Formatting
**What:** Use time.RFC3339 constant for ISO 8601 timestamps
**When to use:** All timestamp output in status responses
**Example:**
```go
// Source: https://pkg.go.dev/time
import "time"

// Format timestamp for output
timestamp := time.Now().Format(time.RFC3339)  // "2026-01-25T14:30:00Z"

// Parse timestamp from storage
parsed, err := time.Parse(time.RFC3339, storedTimestamp)
```

### Anti-Patterns to Avoid
- **Storing sync state in database:** Application state belongs in memory, not persistent storage
- **Polling Qdrant for sync completion:** Sync happens in same process, use direct state flags
- **Nested mutexes for sync control:** Over-engineering, atomic.Bool is sufficient
- **Using collection optimizer status for sync state:** Optimizer status is Qdrant's internal state, doesn't reflect application-level sync

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CLI argument parsing | Manual os.Args parsing | Cobra | Subcommands, help generation, flag validation |
| Commit comparison | Git subprocess + parsing | go-github CompareCommits | Rate limiting, authentication, error handling |
| Concurrent state flag | Custom mutex wrapper | atomic.Bool | Lock-free, simpler, sufficient for boolean |
| Timestamp formatting | Manual string formatting | time.RFC3339 | Handles timezones, milliseconds, standard format |
| Collection metrics | Scrolling through all points | GetCollectionInfo | Qdrant maintains counts internally, much faster |
| Progress tracking | Manual stdout writes | log/slog with levels | Structured, filterable, standard library |

**Key insight:** Qdrant already tracks collection statistics and Cobra already handles CLI boilerplate. Focus on orchestration, not reimplementation.

## Common Pitfalls

### Pitfall 1: Using Collection Counts for "Exact" Numbers
**What goes wrong:** Documentation states GetCollectionInfo returns approximate counts during optimization
**Why it happens:** Qdrant duplicates points during optimization, delays indexing for performance
**How to avoid:** Accept that counts are approximate. For exact counts, use Count API (but adds latency)
**Warning signs:** Counts fluctuate slightly between calls, don't match insertion count exactly

### Pitfall 2: Forgetting to Release Sync Lock on Panic
**What goes wrong:** If sync panics, atomic.Bool stays true forever, queries blocked permanently
**Why it happens:** No defer to reset flag on panic
**How to avoid:** Always use defer after setting sync flag
**Warning signs:** Sync crashes, subsequent queries return "sync in progress" forever
```go
// WRONG
syncInProgress.Store(true)
doSync() // if this panics, flag stays true

// RIGHT
if !syncInProgress.CompareAndSwap(false, true) {
    return errors.New("sync already running")
}
defer syncInProgress.Store(false) // Always reset
doSync()
```

### Pitfall 3: Calling ListDocumentPaths on Every Status Check
**What goes wrong:** ListDocumentPaths scrolls entire collection, expensive for large indexes
**Why it happens:** Treating status check as cheap operation
**How to avoid:** Consider caching document count, or only return paths on demand
**Warning signs:** Status handler latency increases with collection size

### Pitfall 4: Not Checking Qdrant Connection Before Sync
**What goes wrong:** Sync starts, clears collection, then fails to reconnect
**Why it happens:** Assuming Qdrant stays healthy throughout operation
**How to avoid:** Call storage.Health(ctx) before ClearCollection
**Warning signs:** Half-completed syncs leave empty collections

### Pitfall 5: Blocking Queries After Sync Completes
**What goes wrong:** Sync finishes but queries still return "sync in progress"
**Why it happens:** Not testing the defer path or flag reset logic
**How to avoid:** Test that flag is false after sync completes (both success and error paths)
**Warning signs:** Manual testing works (dev watches sync complete), but automated clients see permanent blocking

### Pitfall 6: Comparing Commits Without Common Ancestor
**What goes wrong:** CompareCommits fails when branches diverged
**Why it happens:** Assuming indexed commit is always an ancestor of HEAD
**How to avoid:** Handle API errors gracefully, don't crash on comparison failure
**Warning signs:** Status handler crashes when repository history is rewritten

### Pitfall 7: Not Surfacing Qdrant Connection Errors
**What goes wrong:** Status handler returns empty response when Qdrant is down
**Why it happens:** Swallowing errors, returning zero values
**How to avoid:** Return error with category "qdrant_connection: ..." or include connection health in status
**Warning signs:** Status returns all zeros, no indication of underlying failure

## Code Examples

Verified patterns from official sources:

### Getting Collection Statistics
```go
// Source: https://pkg.go.dev/github.com/qdrant/go-client/qdrant
ctx := context.Background()
info, err := client.GetCollectionInfo(ctx, "documents")
if err != nil {
    return fmt.Errorf("failed to get collection info: %w", err)
}

// Extract counts (note: these are approximate)
totalPoints := info.GetPointsCount()        // docs + chunks
indexedVectors := info.GetIndexedVectorsCount()
segments := info.GetSegmentsCount()
status := info.GetStatus() // "green", "yellow", "grey", "red"
```

### Atomic Boolean State Management
```go
// Source: https://pkg.go.dev/sync/atomic
import "sync/atomic"

var syncInProgress atomic.Bool

// Check state (in handler)
if syncInProgress.Load() {
    return errors.New("sync in progress")
}

// Set state with CAS (in sync command)
if !syncInProgress.CompareAndSwap(false, true) {
    return errors.New("sync already running")
}
defer syncInProgress.Store(false)
// ... perform sync
```

### Comparing GitHub Commits
```go
// Source: https://pkg.go.dev/github.com/google/go-github/v81/github
comparison, _, err := client.Repositories.CompareCommits(
    ctx,
    "cloudwego",
    "cloudwego.github.io",
    indexedSHA,  // base commit
    "main",      // head ref
    nil,
)
if err != nil {
    // Handle gracefully - network error or invalid comparison
    return 0, fmt.Errorf("failed to compare commits: %w", err)
}

// Check staleness
if comparison.AheadBy > 20 {
    // Warn: indexed commit is >20 commits behind HEAD
}
```

### Status Handler with Error Categories
```go
// Source: User requirements + MCP error handling patterns
func makeStatusHandler(store *storage.QdrantStorage, syncState *atomic.Bool) func(
    context.Context, *mcp.CallToolRequest, StatusInput,
) (*mcp.CallToolResult, StatusOutput, error) {
    return func(ctx context.Context, req *mcp.CallToolRequest, input StatusInput) (
        *mcp.CallToolResult, StatusOutput, error,
    ) {
        // Get collection info
        info, err := store.client.GetCollectionInfo(ctx, storage.CollectionName)
        if err != nil {
            // Categorized error: qdrant_error
            return nil, StatusOutput{}, fmt.Errorf("qdrant_error: %w", err)
        }

        // Get commit SHA
        commitSHA, err := store.GetCommitSHA(ctx, defaultRepository)
        if err != nil {
            return nil, StatusOutput{}, fmt.Errorf("qdrant_error: %w", err)
        }

        // Get one doc for last sync timestamp
        paths, err := store.ListDocumentPaths(ctx, defaultRepository)
        if err != nil {
            return nil, StatusOutput{}, fmt.Errorf("qdrant_error: %w", err)
        }

        var lastSync time.Time
        if len(paths) > 0 {
            doc, err := store.GetDocumentByPath(ctx, paths[0], defaultRepository)
            if err == nil {
                lastSync = doc.Metadata.IndexedAt
            }
        }

        // Count chunks: total points - document count
        totalChunks := int(info.GetPointsCount()) - len(paths)

        return nil, StatusOutput{
            TotalDocs:      len(paths),
            TotalChunks:    totalChunks,
            IndexedPaths:   paths,
            LastSyncTime:   lastSync.Format(time.RFC3339),
            SourceCommit:   commitSHA,
            SyncInProgress: syncState.Load(),
        }, nil
    }
}
```

### CLI Sync Command Structure
```go
// Source: https://cobra.dev/docs/how-to-guides/working-with-commands/
// cmd/sync/main.go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/spf13/cobra"
    "github.com/bull/eino-mcp-server/internal/indexer"
    "github.com/bull/eino-mcp-server/internal/storage"
    // ... other imports
)

var rootCmd = &cobra.Command{
    Use:   "eino-sync",
    Short: "EINO documentation indexing tool",
    Long:  "CLI tool for managing EINO documentation index in Qdrant",
}

var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Re-index all documentation from GitHub",
    Long:  "Clears existing index and rebuilds from latest GitHub commit",
    RunE: func(cmd *cobra.Command, args []string) error {
        ctx := context.Background()

        // Initialize dependencies
        store, err := storage.NewQdrantStorage("localhost", 6334)
        if err != nil {
            return fmt.Errorf("failed to connect to Qdrant: %w", err)
        }
        defer store.Close()

        // Check health before proceeding
        if err := store.Health(ctx); err != nil {
            return fmt.Errorf("Qdrant health check failed: %w", err)
        }

        // Clear collection
        fmt.Println("Clearing existing collection...")
        if err := store.ClearCollection(ctx); err != nil {
            return fmt.Errorf("failed to clear collection: %w", err)
        }

        // Initialize pipeline
        // ... create fetcher, chunker, embedder, generator

        pipeline := indexer.NewPipeline(fetcher, chunker, embedder, generator, store, logger)

        // Run indexing
        fmt.Println("Starting sync...")
        result, err := pipeline.IndexAll(ctx)
        if err != nil {
            return fmt.Errorf("sync failed: %w", err)
        }

        // Print results
        fmt.Printf("Sync complete!\n")
        fmt.Printf("  Documents: %d/%d successful\n", result.SuccessfulDocs, result.TotalDocs)
        fmt.Printf("  Chunks: %d\n", result.TotalChunks)
        fmt.Printf("  Duration: %s\n", result.Duration)
        fmt.Printf("  Commit: %s\n", result.CommitSHA)

        if len(result.FailedDocs) > 0 {
            fmt.Printf("\nFailed documents:\n")
            for _, failed := range result.FailedDocs {
                fmt.Printf("  - %s: %s\n", failed.Path, failed.Reason)
            }
        }

        return nil
    },
}

func init() {
    rootCmd.AddCommand(syncCmd)
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual mutex locking | atomic.Bool | Go 1.19 (2022) | Simpler, lock-free, sufficient for boolean flags |
| CLI with flag package | Cobra with subcommands | Established 2015+ | Better UX, help generation, standard in Go ecosystem |
| Custom progress bars | log/slog with structured logging | Go 1.21 (2023) | Standard library, no dependencies |
| time.Format(RFC3339) string | time.RFC3339 constant | Always available | More explicit, prevents typos |
| Scroll all points for count | GetCollectionInfo | Qdrant v0.8+ | Much faster, Qdrant maintains internal counts |

**Deprecated/outdated:**
- Using third-party atomic boolean libraries: sync/atomic now has native Bool type
- Building CLI with just flag package: Cobra is now standard for multi-command CLIs
- Using log package for structured output: log/slog is now standard library

## Open Questions

1. **How to surface Qdrant connection issues**
   - What we know: GetCollectionInfo returns error if Qdrant unreachable
   - What's unclear: Should status tool fail, or return partial status with connection warning?
   - Recommendation: Fail the tool call with categorized error "qdrant_connection: ..." - MCP spec says tools should fail on exceptional conditions, and missing DB connection is exceptional

2. **Whether to do live GitHub HEAD comparison**
   - What we know: CompareCommits costs 1 API call per status check, subject to rate limits
   - What's unclear: Performance vs freshness tradeoff - check on every status call or cache?
   - Recommendation: Check on every call with error handling for rate limits. Status checks are infrequent (user-initiated), and knowing real-time staleness is valuable. If rate limit hit, return staleness field as null with note "rate_limited"

3. **Staleness display format**
   - What we know: User wants "> 20 commits behind" warning
   - What's unclear: Separate boolean "stale" field, or integer "commits_behind" field, or warning string?
   - Recommendation: Return integer "commits_behind" field (null if comparison failed), client can interpret. More flexible than boolean, provides exact information

4. **Whether to cache document paths count**
   - What we know: ListDocumentPaths scrolls all parent documents
   - What's unclear: Performance impact on large collections (1000+ docs)
   - Recommendation: Start with real-time scroll, optimize later if slow. Qdrant scroll with filters is fast (<100ms for 1000s of docs), and status checks are infrequent

5. **Error categorization granularity**
   - What we know: User wants "category + message" format
   - What's unclear: How many categories? embedding_failed, fetch_failed, qdrant_error, network_error?
   - Recommendation: Start simple with 3 categories: "qdrant_error" (storage failures), "github_error" (API failures), "embedding_error" (OpenAI failures). Can expand based on real usage patterns

## Sources

### Primary (HIGH confidence)
- [Qdrant Go Client (pkg.go.dev)](https://pkg.go.dev/github.com/qdrant/go-client/qdrant) - GetCollectionInfo API
- [go-github v81 (pkg.go.dev)](https://pkg.go.dev/github.com/google/go-github/v81/github) - CompareCommits API
- [Go sync/atomic package](https://pkg.go.dev/sync/atomic) - atomic.Bool
- [Go time package](https://pkg.go.dev/time) - RFC3339 constant
- [Qdrant Collections Documentation](https://qdrant.tech/documentation/concepts/collections/) - Collection info fields and behavior
- [Qdrant Count Points API](https://api.qdrant.tech/api-reference/points/count-points) - Exact vs approximate counts

### Secondary (MEDIUM confidence)
- [How to Build a CLI Tool in Go with Cobra (2026)](https://oneuptime.com/blog/post/2026-01-07-go-cobra-cli/view) - Recent Cobra patterns
- [Cobra Official Docs](https://cobra.dev/docs/how-to-guides/working-with-commands/) - Command structure
- [Go Error Handling Guidelines](https://jayconrod.com/posts/116/error-handling-guidelines-for-go) - Sentinel errors, wrapping
- [MCP Error Handling (Stainless)](https://www.stainless.com/mcp/error-handling-and-debugging-mcp-servers) - Tool vs protocol errors
- [Go Concurrency Control (Leapcell)](https://leapcell.io/blog/concurrency-control-in-go-mastering-mutex-and-rwmutex-for-critical-sections) - Mutex vs atomic patterns
- [Qdrant Indexing Optimization](https://qdrant.tech/articles/indexing-optimization/) - Bulk upload and indexing behavior

### Tertiary (LOW confidence)
- [GitHub API ahead/behind discussion](https://github.com/orgs/community/discussions/42292) - Community patterns, not official docs
- [CLI UX Best Practices (Evil Martians)](https://evilmartians.com/chronicles/cli-ux-best-practices-3-patterns-for-improving-progress-displays) - General patterns, not Go-specific

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries verified via pkg.go.dev and official docs
- Architecture: HIGH - Patterns verified in existing codebase and official library docs
- Pitfalls: MEDIUM - Based on documentation warnings and common Go patterns, not project-specific experience

**Research date:** 2026-01-25
**Valid until:** 2026-02-25 (30 days) - Go libraries stable, Qdrant API mature
