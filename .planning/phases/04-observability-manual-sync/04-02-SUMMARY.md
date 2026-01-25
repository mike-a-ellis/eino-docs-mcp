---
phase: 04-observability-manual-sync
plan: 02
subsystem: cli
tags: [cobra, cli, sync, indexing, qdrant]

# Dependency graph
requires:
  - phase: 04-01
    provides: get_index_status tool for monitoring
  - phase: 02-05
    provides: Pipeline for full document indexing
  - phase: 01-02
    provides: Qdrant storage with health check and clear collection
provides:
  - Standalone CLI tool for manual documentation re-indexing
  - Progress output during sync operations
  - Result reporting with success/failure counts
affects: [05-deployment]

# Tech tracking
tech-stack:
  added: [github.com/spf13/cobra v1.10.2]
  patterns: [Cobra CLI structure with subcommands, environment-based configuration]

key-files:
  created: [cmd/sync/main.go]
  modified: [go.mod, go.sum, internal/storage/qdrant.go]

key-decisions:
  - "Use Cobra for CLI framework (standard in Go ecosystem)"
  - "Reuse OpenAI client from embeddings for metadata generation (no duplicate clients)"
  - "Clear collection before indexing (full refresh model, not incremental)"
  - "Default to cloudwego/cloudwego.github.io repo (matches MCP server default)"

patterns-established:
  - "CLI environment variables with getEnv/getEnvInt helpers"
  - "Progress output at each pipeline stage for user visibility"
  - "Fail-fast error handling with descriptive error messages"

# Metrics
duration: 4.5min
completed: 2026-01-25
---

# Phase 04 Plan 02: CLI Sync Command Summary

**Standalone CLI tool for manual EINO documentation re-indexing with progress output and comprehensive result reporting**

## Performance

- **Duration:** 4.5 min
- **Started:** 2026-01-25T21:59:14Z
- **Completed:** 2026-01-25T22:03:46Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- CLI binary `eino-sync` with `sync` subcommand for full documentation re-indexing
- Progress output showing connection, health check, clearing, and indexing stages
- Comprehensive result reporting: success/failure counts, chunk count, duration, commit SHA
- Failed document listing with paths and error reasons
- Environment variable documentation in CLI help

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Cobra dependency and create CLI structure** - `14bdd55` (chore)
2. **Task 2: Implement sync command with full pipeline** - `e4d4f0d` (feat)

_Note: Task 2 includes bug fixes for GetCollectionInfo discovered during build_

## Files Created/Modified
- `cmd/sync/main.go` - CLI sync command entry point with full pipeline orchestration
- `go.mod` - Added Cobra dependency
- `go.sum` - Updated dependency checksums
- `internal/storage/qdrant.go` - Fixed GetCollectionInfo to use correct API method

## Decisions Made

**Use Cobra for CLI framework**
- Rationale: Standard choice in Go ecosystem, provides help generation, subcommands, flag parsing

**Reuse OpenAI client from embeddings for metadata**
- Rationale: Avoid creating duplicate OpenAI clients, use embedder.Client() accessor

**Clear collection before indexing**
- Rationale: Full refresh model ensures consistency, avoids partial state

**Default to cloudwego/cloudwego.github.io repo**
- Rationale: Matches MCP server defaults, provides consistency across components

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed GetCollectionInfo API method call**
- **Found during:** Task 2 (build after implementing sync command)
- **Issue:** Code called `s.client.GetCollection(ctx, CollectionName)` but Qdrant client method is `GetCollectionInfo`
- **Fix:** Changed method call to `s.client.GetCollectionInfo(ctx, CollectionName)`
- **Files modified:** internal/storage/qdrant.go
- **Verification:** Build succeeded after fix
- **Committed in:** e4d4f0d (Task 2 commit)

**2. [Rule 1 - Bug] Fixed PointsCount pointer type handling**
- **Found during:** Task 2 (build after API method fix)
- **Issue:** Qdrant API returns `*uint64` for PointsCount, but code assigned directly to `uint64` field
- **Fix:** Added nil check and pointer dereference: `if collection.PointsCount != nil { pointsCount = *collection.PointsCount }`
- **Files modified:** internal/storage/qdrant.go
- **Verification:** Build succeeded, type error resolved
- **Committed in:** e4d4f0d (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Bug fixes were discovered during compilation and required for correct operation. Both issues existed in code from Plan 04-01 and were exposed when sync command tried to build against storage package. No scope creep - essential correctness fixes.

## Issues Encountered

**Qdrant API method mismatch**
- Problem: Plan 04-01 implementation used incorrect method name
- Resolution: Fixed during Task 2 build verification (deviation Rule 1)

**Go toolchain path**
- Problem: `go` command not in PATH
- Resolution: Used full path `/home/bull/go1.24.0/bin/go` for all commands

## User Setup Required

None - no external service configuration required.

The sync CLI requires the same environment variables as the MCP server:
- `QDRANT_HOST` (default: localhost)
- `QDRANT_PORT` (default: 6334)
- `OPENAI_API_KEY` (required)
- `GITHUB_TOKEN` (optional, for higher rate limits)

## Next Phase Readiness

**Ready for deployment phase:**
- CLI tool builds and runs successfully
- Error handling provides clear messages for missing dependencies
- Progress output gives users visibility into sync operations
- Result reporting shows exactly what succeeded/failed

**Verification complete:**
- Build succeeds: `go build ./cmd/sync`
- Help system works: `./sync --help`, `./sync sync --help`
- Error handling verified: Missing OPENAI_API_KEY produces clear error message
- Qdrant health check runs before indexing

**Next steps:**
- Add sync command to deployment documentation
- Consider scheduling periodic syncs (cron/systemd timer)
- Add metrics/logging integration for production monitoring

---
*Phase: 04-observability-manual-sync*
*Completed: 2026-01-25*
