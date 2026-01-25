---
phase: 02-document-processing
plan: 02
subsystem: document-processing
tags: [goldmark, markdown, chunking, ast, toc, header-hierarchy]

# Dependency graph
requires:
  - phase: 01-storage-foundation
    provides: Storage models (Document, Chunk) with HeaderPath field
provides:
  - Header-based markdown chunker that splits at H1/H2 boundaries
  - Chunk struct with header hierarchy preservation
  - AST-based content extraction with goldmark
affects: [03-embedding, 04-metadata, 05-indexing]

# Tech tracking
tech-stack:
  added:
    - github.com/yuin/goldmark v1.7.16
    - go.abhg.dev/goldmark/toc v0.12.0
  patterns:
    - "AST walking for semantic boundary detection"
    - "Header hierarchy prepending for retrieval context"

key-files:
  created:
    - internal/markdown/chunker.go
    - internal/markdown/chunker_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Split only at H1 and H2 boundaries (not H3+) for semantic coherence"
  - "Prepend full header path to chunk content for standalone context"
  - "No chunk overlap - header hierarchy provides sufficient context"
  - "No size limits - preserve complete sections at natural boundaries"

patterns-established:
  - "Use toc.Inspect with MinDepth(1), MaxDepth(2), Compact(true) for header extraction"
  - "Store both Content (with prepended headers) and RawContent (original)"
  - "Format header paths as '# Title > ## Section' showing hierarchy"

# Metrics
duration: 3min
completed: 2026-01-25
---

# Phase 02 Plan 02: Markdown Chunker Summary

**Goldmark-based chunker splits markdown at H1/H2 boundaries with full header hierarchy prepended to each chunk for improved retrieval context**

## Performance

- **Duration:** 3 min
- **Started:** 2026-01-25T18:56:59Z
- **Completed:** 2026-01-25T18:59:57Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Header-aware chunking using goldmark AST parser with toc package
- Automatic header hierarchy extraction and prepending for context preservation
- Comprehensive test suite covering basic headers, nested content, edge cases
- Zero truncation - complete section content preserved at natural boundaries

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement header-based markdown chunker** - `3aab481` (feat)
2. **Task 2: Add unit tests for chunker** - `9cd29bb` (test)

## Files Created/Modified
- `internal/markdown/chunker.go` - Chunker implementation with goldmark AST walking
- `internal/markdown/chunker_test.go` - 7 comprehensive unit tests
- `go.mod` - Added goldmark and goldmark/toc dependencies
- `go.sum` - Dependency checksums

## Decisions Made

**1. Split boundaries limited to H1 and H2 only**
- Rationale: H3+ sections are too granular; H1/H2 provide semantic boundaries matching human documentation structure
- Impact: Chunks represent complete logical sections without artificial fragmentation

**2. No chunk overlap strategy**
- Rationale: Full header hierarchy prepending provides context; overlap unnecessary and increases storage
- Impact: Cleaner chunk boundaries, reduced storage size, simpler implementation

**3. No maximum chunk size enforcement**
- Rationale: Cutting mid-section destroys semantic coherence; research shows boundary-preserving chunking outperforms fixed-size
- Impact: Some chunks may be large, but embedding models handle 8K+ token inputs

**4. Dual content storage (Content + RawContent)**
- Rationale: Content field has prepended headers for embedding; RawContent preserves original for display
- Impact: Slightly higher memory usage but enables clean separation of concerns

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - goldmark and toc packages worked as documented, all tests passed on first run.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for next phase:** Chunker provides the foundation for embedding generation.

**What's available:**
- ChunkDocument method splits markdown at H1/H2 boundaries
- Each chunk has Index, HeaderPath, Content (with prepended headers), RawContent
- Comprehensive test coverage validates all chunking behaviors

**Expected usage in next phase:**
```go
chunker := markdown.NewChunker()
chunks, err := chunker.ChunkDocument(docContent)
// chunks ready for embedding generation
```

**No blockers or concerns.**

---
*Phase: 02-document-processing*
*Completed: 2026-01-25*
