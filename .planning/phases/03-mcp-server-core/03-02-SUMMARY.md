---
phase: 03-mcp-server-core
plan: 02
subsystem: storage
tags: [qdrant, vector-search, storage, go]

# Dependency graph
requires:
  - phase: 01-storage-foundation
    provides: QdrantStorage with SearchChunks and GetDocument methods
provides:
  - SearchChunksWithScores with similarity scores for MCP search handlers
  - ListDocumentPaths for list_docs MCP tool
  - GetDocumentByPath for fetch_doc MCP tool
  - ScoredChunk type combining chunk data with score
affects: [03-mcp-server-core, mcp-tools]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Scroll pagination with batch size termination check
    - Score extraction from Qdrant ScoredPoint.Score field

key-files:
  created: []
  modified:
    - internal/storage/models.go
    - internal/storage/qdrant.go
    - internal/storage/qdrant_test.go

key-decisions:
  - "Stop scroll pagination when results < batch size (not on empty)"
  - "ScoredChunk embeds *Chunk to reuse existing type"
  - "Score converted from float32 to float64 for Go conventions"

patterns-established:
  - "Scroll pagination: check results count < batch size to terminate"
  - "Path-based document lookup via Qdrant filter on 'path' field"

# Metrics
duration: 8min
completed: 2026-01-25
---

# Phase 03 Plan 02: Storage Query Methods Summary

**Enhanced storage layer with similarity scores, document path listing, and path-based lookup for MCP tool handlers**

## Performance

- **Duration:** 8 min
- **Started:** 2026-01-25T19:57:14Z
- **Completed:** 2026-01-25T20:05:18Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- ScoredChunk type wraps Chunk with similarity score from vector search
- SearchChunksWithScores returns relevance scores for MCP search ranking
- ListDocumentPaths scrolls all parent documents and returns sorted paths
- GetDocumentByPath retrieves document by path field with repository filter

## Task Commits

Each task was committed atomically:

1. **Task 1: Add ScoredChunk type and SearchChunksWithScores method** - `52be140` (feat)
2. **Task 2: Add ListDocumentPaths and GetDocumentByPath methods** - `51565ba` (feat)

## Files Created/Modified
- `internal/storage/models.go` - Added ScoredChunk struct with embedded *Chunk and Score field
- `internal/storage/qdrant.go` - Added SearchChunksWithScores, ListDocumentPaths, GetDocumentByPath methods
- `internal/storage/qdrant_test.go` - Added integration tests for all new methods

## Decisions Made
- **Stop scroll pagination when results < batch size:** Original plan used `len(results) == 0` check but this caused infinite loops. Changed to `len(results) < batchSize` to properly detect end of data.
- **ScoredChunk embeds *Chunk:** Rather than duplicating all fields, we embed the existing Chunk pointer and add Score field.
- **Score as float64:** Qdrant returns float32 but Go conventions prefer float64 for public APIs, so we convert.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed infinite loop in ListDocumentPaths scroll pagination**
- **Found during:** Task 2 (ListDocumentPaths implementation)
- **Issue:** Original pagination logic checked `len(results) == 0` to terminate, but Qdrant scroll API returns filtered results - with small datasets, this caused infinite loops
- **Fix:** Changed termination condition to `len(results) < batchSize` which properly detects last page
- **Files modified:** internal/storage/qdrant.go
- **Verification:** TestListDocumentPaths passes with 3 documents
- **Committed in:** 51565ba (Task 2 commit)

**2. [Rule 2 - Missing Critical] Added sleep for Qdrant eventual consistency in test**
- **Found during:** Task 2 (test verification)
- **Issue:** TestListDocumentPaths was flaky - documents not immediately visible after upsert
- **Fix:** Added 100ms sleep after document upserts before querying
- **Files modified:** internal/storage/qdrant_test.go
- **Verification:** Test passes consistently
- **Committed in:** 51565ba (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing critical)
**Impact on plan:** Both auto-fixes necessary for correct operation. No scope creep.

## Issues Encountered
None - standard implementation with minor test timing adjustment.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Storage layer now provides all methods needed by MCP tool handlers
- SearchChunksWithScores ready for search tool (scores for ranking)
- ListDocumentPaths ready for list_docs tool
- GetDocumentByPath ready for fetch_doc tool
- Next: Implement actual MCP tool handler logic

---
*Phase: 03-mcp-server-core*
*Completed: 2026-01-25*
