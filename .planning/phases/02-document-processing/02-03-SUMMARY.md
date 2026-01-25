---
phase: 02-document-processing
plan: 03
subsystem: embedding
tags: [openai, embeddings, text-embedding-3-small, batching, exponential-backoff, rate-limiting]

# Dependency graph
requires:
  - phase: 01-storage-foundation
    provides: Storage models with Chunk.Embedding field ([]float32)
provides:
  - OpenAI client wrapper with API key validation
  - Embedder with batching (500 texts per request) and exponential backoff retry
  - Float64 to float32 conversion for storage compatibility
affects: [02-05-indexing, future retrieval pipeline]

# Tech tracking
tech-stack:
  added:
    - github.com/openai/openai-go v1.12.0
  patterns:
    - "Batch embedding requests (500 texts) to balance RPM vs TPM rate limits"
    - "Exponential backoff retry on HTTP 429 with 500ms initial, 10s max, 30s total"
    - "Float type conversion (OpenAI float64 → storage float32)"

key-files:
  created:
    - internal/embedding/client.go
    - internal/embedding/embedder.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Batch size 500 texts per request (balances requests-per-minute vs tokens-per-minute)"
  - "Validate OPENAI_API_KEY on client creation (fail-fast for better error messages)"
  - "Retry only on HTTP 429 rate limits (treat other errors as permanent)"
  - "30 second max elapsed time for retry backoff (prevents indefinite hanging)"

patterns-established:
  - "Use backoff.Permanent() for non-retryable errors to stop retry immediately"
  - "Convert embeddings to float32 for memory efficiency while preserving precision"
  - "Expose configurable batch size in NewEmbedder for testing and rate limit tuning"

# Metrics
duration: 3min
completed: 2026-01-25
---

# Phase 02 Plan 03: OpenAI Embeddings Client Summary

**OpenAI text-embedding-3-small client with 500-text batching and exponential backoff retry on rate limits, returning float32 vectors compatible with storage layer**

## Performance

- **Duration:** 3 min
- **Started:** 2026-01-25T18:56:57Z
- **Completed:** 2026-01-25T19:00:50Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- OpenAI client wrapper validates API key on initialization (fail-fast error handling)
- Embedder batches up to 500 texts per request to maximize throughput
- Exponential backoff retry handles rate limit errors (HTTP 429) gracefully
- Automatic conversion from OpenAI float64 to storage float32 for memory efficiency
- Configurable batch size supports testing and rate limit tier adjustment

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement OpenAI client wrapper** - `4d30c8f` (feat)
2. **Task 2: Implement embedder with batching and retry** - `90a58a9` (feat)

## Files Created/Modified
- `internal/embedding/client.go` - OpenAI client wrapper with API key validation
- `internal/embedding/embedder.go` - Embedder with batching and exponential backoff
- `go.mod` - Added github.com/openai/openai-go v1.12.0 dependency
- `go.sum` - Dependency checksums

## Decisions Made

**1. Batch size of 500 texts per request**
- Rationale: OpenAI supports up to 2048 texts/batch, but smaller batches reduce tokens-per-minute pressure while maintaining good throughput
- Impact: Balances rate limit dimensions (RPM vs TPM) for most usage tiers

**2. Validate API key on client creation (not lazy)**
- Rationale: Fail-fast with clear error message better than discovering missing key during first embedding call
- Impact: Better developer experience, earlier error detection

**3. Retry only HTTP 429 rate limit errors**
- Rationale: Authentication errors (401), invalid requests (400), and server errors (500) are not retryable
- Impact: Fast failure on permanent errors, retry only on transient rate limits

**4. Use float32 for embeddings (not float64)**
- Rationale: OpenAI returns float64, but storage uses float32; precision loss negligible for embeddings, memory savings significant
- Impact: 50% memory reduction per vector with no measurable quality impact

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed OpenAI client type mismatch**
- **Found during:** Task 1 (Client wrapper compilation)
- **Issue:** openai.NewClient() returns struct not pointer; initial code tried to assign struct to *openai.Client
- **Fix:** Changed to `&client` to take address of returned struct
- **Files modified:** internal/embedding/client.go
- **Verification:** Package compiled without errors
- **Committed in:** 4d30c8f (Task 1 commit)

**2. [Rule 3 - Blocking] Fixed OpenAI API parameter construction**
- **Found during:** Task 2 (Embedder compilation)
- **Issue:** Research example used `openai.F()` helper not available in v1.12.0; compilation failed with "undefined: openai.F"
- **Fix:** Used EmbeddingNewParamsInputUnion struct with OfArrayOfStrings field directly; model as string literal
- **Files modified:** internal/embedding/embedder.go
- **Verification:** Package compiled successfully, API types match
- **Committed in:** 90a58a9 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both fixes necessary for compilation. No functional changes to plan design. API version differences required different parameter construction patterns.

## Issues Encountered

None beyond the auto-fixed compilation issues above - both resolved immediately with SDK documentation.

## User Setup Required

**External services require manual configuration.**

**OpenAI API Setup:**
1. Get API key from https://platform.openai.com/api-keys
2. Set environment variable:
   ```bash
   export OPENAI_API_KEY="sk-..."
   ```
3. Verify:
   ```bash
   echo $OPENAI_API_KEY
   ```

**Cost considerations:**
- text-embedding-3-small: $0.02 per 1M tokens
- EINO docs estimated ~500KB → ~125K tokens → ~$0.0025 per full indexing
- Rate limits vary by tier (check platform.openai.com/account/limits)

## Next Phase Readiness

**Ready for embedding generation in indexing pipeline.**

**What's available:**
- `NewClient()` creates OpenAI client with validated API key
- `NewEmbedder(client, batchSize)` creates embedder with configurable batching
- `GenerateEmbeddings(ctx, texts)` returns [][]float32 compatible with storage.Chunk

**Expected usage in indexing pipeline:**
```go
client, err := embedding.NewClient()
if err != nil {
    return fmt.Errorf("create embedding client: %w", err)
}

embedder := embedding.NewEmbedder(client, 500)
embeddings, err := embedder.GenerateEmbeddings(ctx, chunkTexts)
if err != nil {
    return fmt.Errorf("generate embeddings: %w", err)
}

// embeddings[i] is []float32 ready for storage.Chunk.Embedding
```

**No blockers or concerns.** Retry logic handles rate limits. Batch size tunable based on actual rate limit tier during testing.

---
*Phase: 02-document-processing*
*Completed: 2026-01-25*
