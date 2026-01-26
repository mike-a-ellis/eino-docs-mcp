---
phase: 05-deployment
plan: 03
subsystem: infra
tags: [fly.io, docker, qdrant, deployment, toml]

# Dependency graph
requires:
  - phase: 05-01
    provides: Multi-stage Dockerfile for MCP server
provides:
  - Complete Fly.io configuration with process groups (web + qdrant)
  - Persistent volume configuration for Qdrant storage
  - Health check configuration for reliability monitoring
  - Resource allocation (256MB web, 512MB qdrant)
affects: [05-04-deployment-execution, production-operations]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Fly.io process groups for multi-service deployment"
    - "Persistent volume mount for stateful sidecar"
    - "Health check integration for service monitoring"

key-files:
  created:
    - fly.toml
  modified: []

key-decisions:
  - "Corrected Qdrant binary path from /usr/bin/qdrant to /qdrant/qdrant via Docker inspection"
  - "MCP server allocated 256MB (minimal viable), Qdrant 512MB (vector operations requirement)"
  - "Persistent volume mounted to /qdrant/storage with 1GB initial size"
  - "QDRANT_HOST=localhost for same-host optimization on Fly.io"

patterns-established:
  - "Process group configuration: separate VM resources per service"
  - "Volume lifecycle: processes array restricts mount to specific process group"
  - "Health check grace period: 10s to allow server initialization"

# Metrics
duration: 2min
completed: 2026-01-26
---

# Phase 05 Plan 03: Fly.io Configuration Summary

**Multi-service deployment configuration with MCP server and embedded Qdrant sidecar on Fly.io**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-26T01:15:07Z
- **Completed:** 2026-01-26T01:17:31Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Complete fly.toml configuration with process groups for MCP server (web) and Qdrant (qdrant)
- Persistent volume configuration for Qdrant data storage (1GB initial size)
- Health check configuration targeting /health endpoint
- Resource allocation matching CONTEXT.md decisions (256MB/512MB split)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create fly.toml configuration** - `df53b70` (feat)

**Plan metadata:** (pending final commit)

## Files Created/Modified
- `fly.toml` - Complete Fly.io deployment configuration with process groups, volumes, health checks, and resource allocation

## Decisions Made
- **Corrected Qdrant binary path:** Docker inspection revealed actual path is `/qdrant/qdrant` (not `/usr/bin/qdrant` as initially assumed)
- **Process-specific volume mount:** `[[mounts]]` processes array ensures volume only mounts for qdrant process (not web)
- **Health check timings:** 10s grace period + 15s interval + 5s timeout balances responsiveness vs startup time
- **Auto-scaling settings:** auto_stop_machines=off + min_machines_running=1 ensures 24/7 availability

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected Qdrant binary path from /usr/bin/qdrant to /qdrant/qdrant**
- **Found during:** Task 1 (fly.toml creation)
- **Issue:** Initial configuration used `/usr/bin/qdrant` but Docker inspection revealed actual path is `/qdrant/qdrant`
- **Fix:** Updated `[processes]` section with correct binary path: `qdrant = "/qdrant/qdrant"`
- **Files modified:** fly.toml
- **Verification:** Ran `docker run --rm qdrant/qdrant:latest find / -name "qdrant" -type f` to confirm path
- **Committed in:** df53b70 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Critical fix - incorrect binary path would cause qdrant process launch failure on Fly.io

## Issues Encountered
None - execution proceeded smoothly after Docker verification

## User Setup Required

None - no external service configuration required. Fly.io secrets will be configured in plan 05-04 during deployment execution.

## Next Phase Readiness
- fly.toml ready for `fly launch` deployment
- Configuration validated: process groups, volumes, health checks all present
- Ready for plan 05-04: Deployment Execution (fly launch + secrets configuration)
- **Pending:** Fly.io volume creation (`fly volumes create qdrant_data --region iad --size 1`)
- **Pending:** Fly.io secrets configuration (OPENAI_API_KEY, GITHUB_TOKEN via `fly secrets set`)

---
*Phase: 05-deployment*
*Completed: 2026-01-26*
