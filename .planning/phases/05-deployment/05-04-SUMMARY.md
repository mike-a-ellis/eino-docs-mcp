---
phase: 05-deployment
plan: 04
subsystem: deployment
tags: [flyio, production, supervisor, qdrant, deployment, health-checks]

dependencies:
  requires: [05-01, 05-02, 05-03]
  provides: [production-deployment, live-mcp-server]
  affects: []

tech-stack:
  added: [supervisor-script, single-machine-orchestration]
  patterns: [process-supervision, container-orchestration, health-monitoring]

file-tracking:
  created: [supervisor.sh, entrypoint.sh]
  modified: [Dockerfile, fly.toml, cmd/mcp-server/main.go]

decisions:
  - slug: supervisor-single-machine
    title: Use supervisor script for single-machine deployment
    rationale: Fly.io process groups require multiple machines. Single machine is cost-effective for this use case. Supervisor script manages both MCP server and Qdrant in one container.
    alternatives: ["Process groups with 2 machines", "External Qdrant service"]

  - slug: debian12-glibc-upgrade
    title: Upgrade base image to Debian 12
    rationale: Qdrant binary requires GLIBC 2.34+. Debian 11 has 2.31. Debian 12 provides 2.36.
    alternatives: ["Build Qdrant from source", "Use Alpine with glibc compatibility"]

  - slug: server-mode-env-var
    title: Add SERVER_MODE environment variable
    rationale: MCP server needs to keep running in production to serve health endpoint. Without this, container exits after startup.
    alternatives: ["Separate health check service", "Modify supervisor to handle exit"]

metrics:
  duration: 11min
  completed: 2026-01-26
---

# Phase 05 Plan 04: Production Deployment Summary

**One-liner:** Deployed MCP server with embedded Qdrant to Fly.io using supervisor for process orchestration, live at eino-docs-mcp.fly.dev

## What Was Built

Deployed the complete EINO documentation MCP server to production on Fly.io with:

1. **Single-machine deployment** - Supervisor script orchestrates both MCP server and Qdrant processes
2. **Qdrant binary integration** - Added Qdrant 1.12.5 binary to Docker image
3. **Health monitoring** - HTTP health endpoint accessible at /health
4. **Persistent storage** - 1GB Fly.io volume mounted for Qdrant data
5. **Production configuration** - Secrets management via Fly.io secrets

## Tasks Completed

| Task | Type | Description | Commit | Duration |
|------|------|-------------|--------|----------|
| 1 | auto | Authenticate with Fly.io CLI | N/A | Verification |
| 2 | auto | Create Fly.io app and set secrets | 4fb18db | 2min |
| 3 | auto | Deploy to Fly.io | 446ade1 | 8min |
| 4 | checkpoint | Human verification | N/A | User verified |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added Qdrant binary to Docker image**
- **Found during:** Task 3 (Initial deployment)
- **Issue:** Deployment failed because Qdrant binary was not included in Docker image
- **Fix:** Added multi-stage build step to download Qdrant v1.12.5 binary from GitHub releases and copy to final image at /qdrant/qdrant
- **Files modified:** Dockerfile
- **Commit:** 88cb614

**2. [Rule 1 - Bug] Upgraded to Debian 12 for GLIBC compatibility**
- **Found during:** Task 3 (Qdrant startup)
- **Issue:** Qdrant binary failed with "GLIBC_2.34 not found". Debian 11 only has GLIBC 2.31.
- **Fix:** Changed base image from debian:11-slim to debian:12-slim (GLIBC 2.36)
- **Files modified:** Dockerfile
- **Commit:** 977bdbd

**3. [Rule 2 - Missing Critical] Created supervisor script for single-machine deployment**
- **Found during:** Task 3 (Process orchestration)
- **Issue:** Fly.io process groups require separate machines. Single machine more cost-effective for this use case.
- **Fix:** Created supervisor.sh to start Qdrant in background, wait for readiness, then exec MCP server. Added to Docker image.
- **Files created:** supervisor.sh
- **Files modified:** Dockerfile, fly.toml (changed cmd to /app/supervisor.sh)
- **Commit:** 765f3c2

**4. [Rule 1 - Bug] Fixed supervisor to keep container alive**
- **Found during:** Task 3 (Container lifecycle)
- **Issue:** Container exited immediately after starting Qdrant because supervisor script completed
- **Fix:** Changed supervisor.sh to wait on Qdrant process instead of just sleeping briefly
- **Files modified:** supervisor.sh
- **Commit:** 45bacca

**5. [Rule 2 - Missing Critical] Added SERVER_MODE environment variable**
- **Found during:** Task 3 (HTTP server lifecycle)
- **Issue:** MCP server exits after setup when running on stdio. Health endpoint needs server to stay alive.
- **Fix:** Added SERVER_MODE env var in fly.toml. Modified main.go to check this flag and keep running when set.
- **Files modified:** cmd/mcp-server/main.go, fly.toml
- **Commit:** c9c7f36

## Technical Implementation

### Deployment Architecture

```
Fly.io Machine (shared-cpu-1x, 512MB)
├── supervisor.sh (PID 1)
├── Qdrant (background process)
│   └── Data: /var/lib/qdrant (mounted from fly volume)
└── MCP Server (foreground process)
    ├── Stdio: MCP protocol (primary interface)
    └── HTTP :8080: Health endpoint (monitoring)
```

### Key Files

**supervisor.sh:**
- Starts Qdrant in background
- Polls http://localhost:6333/health until ready (max 30s)
- Execs MCP server process (becomes PID 1)
- Waits on Qdrant process to keep container alive

**fly.toml:**
- Single process group (removed web/qdrant separation)
- Internal port 8080 for health checks
- Volume mounted at /var/lib/qdrant
- Environment: SERVER_MODE=true

**Dockerfile:**
- Multi-stage: builder (Go) + qdrant-downloader + final (Debian 12)
- Downloads Qdrant v1.12.5 from GitHub releases
- Copies both MCP server binary and Qdrant binary
- Sets supervisor.sh as CMD

### Health Check Configuration

- **Endpoint:** https://eino-docs-mcp.fly.dev/health
- **Grace period:** 10 seconds (allow startup time)
- **Interval:** 15 seconds
- **Timeout:** 5 seconds
- **Returns:** {"status":"healthy","qdrant":"connected","timestamp":"..."}

### Resource Allocation

- **CPU:** shared-cpu-1x (shared cores)
- **Memory:** 512MB total
- **Volume:** 1GB (qdrant_data in IAD region)

## Decisions Made

**Use supervisor script for single-machine deployment:**
- Fly.io process groups require multiple machines (2x cost)
- Single machine sufficient for this use case (low traffic MCP server)
- Supervisor script provides simple process orchestration
- Alternative considered: External Qdrant service (more complex, higher cost)

**Upgrade to Debian 12 for GLIBC compatibility:**
- Qdrant binary requires GLIBC 2.34+
- Debian 11 has GLIBC 2.31 (too old)
- Debian 12 provides GLIBC 2.36 (compatible)
- Alternative considered: Build Qdrant from source (much slower builds)

**Add SERVER_MODE environment variable:**
- MCP server normally exits after setup on stdio
- Health endpoint requires server to keep running
- SERVER_MODE flag prevents exit in production
- Alternative considered: Separate health check service (overcomplicated)

## Test Results

**Deployment verification (all passed):**
- ✓ `fly status` shows 1 machine running
- ✓ Health endpoint returns 200 OK at https://eino-docs-mcp.fly.dev/health
- ✓ Response: `{"status":"healthy","qdrant":"connected","timestamp":"2026-01-26T02:15:42Z"}`
- ✓ Fly.io logs show both Qdrant and MCP server started successfully
- ✓ Volume mounted and accessible by Qdrant process
- ✓ User verification: "Deployment verified and working"

## Files Modified

**Created:**
- supervisor.sh - Process orchestration script for single-machine deployment
- entrypoint.sh - (Initially created, then replaced by supervisor.sh)

**Modified:**
- Dockerfile - Added Qdrant binary download, upgraded to Debian 12, changed CMD to supervisor.sh
- fly.toml - Removed process groups, added SERVER_MODE, configured single machine deployment
- cmd/mcp-server/main.go - Added SERVER_MODE check to keep server alive in production

## Next Steps

1. **Index initial data** - Run `fly ssh console -s -C "/app/mcp-server sync"` to populate Qdrant with EINO docs
2. **Configure MCP client** - Update Claude Desktop config to use deployed server
3. **Monitor health** - Check Fly.io dashboard for uptime and resource usage
4. **Set up periodic sync** - Consider Fly.io scheduled machines for automated doc syncing

## Dependencies for Future Work

**Provides:**
- Production MCP server endpoint at eino-docs-mcp.fly.dev
- Health monitoring at /health
- Persistent Qdrant storage on Fly.io volume

**Potential impacts:**
- Future sync automation will need to use `fly ssh console` or scheduled machines
- Scaling to multiple machines will require refactoring supervisor approach back to process groups
- Volume backups should be configured for data persistence

## Lessons Learned

1. **Fly.io process groups require multiple machines** - Single machine deployments need custom orchestration
2. **GLIBC version matters for binaries** - Check library requirements when using pre-built binaries
3. **Container lifecycle needs careful management** - Supervisor script must keep container alive correctly
4. **SERVER_MODE flag pattern** - Clean way to handle dual stdio/HTTP modes in same binary
5. **Iterative debugging on Fly.io** - Quick deploy cycle (fly deploy) makes debugging feasible

## Performance Notes

**Execution time:** 11 minutes total
- App/volume creation: 2min
- Initial deployment attempts: 8min (multiple iterations for fixes)
- Final verification: 1min

**Deployment characteristics:**
- Build time: ~2min (Go compilation + Qdrant download)
- Startup time: ~8s (Qdrant initialization + MCP server setup)
- Health check response time: <100ms

---

*Phase 05 complete. Production deployment successful.*
