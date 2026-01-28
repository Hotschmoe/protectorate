# Sidecar Specification v1

> Lightweight HTTP server running inside each sleeve container for self-reporting and local operations.

## Overview

The sidecar eliminates the need for envoy to exec into containers for status information. Each sleeve reports its own state via a simple HTTP API.

```
+------------------+          +------------------------+
|      ENVOY       |   HTTP   |   SLEEVE CONTAINER     |
|  (orchestrator)  | <------> |  +------------------+  |
|                  |  :8080   |  |     SIDECAR      |  |
|  - spawn/kill    |          |  |  - /health       |  |
|  - workspace ops |          |  |  - /status       |  |
|  - terminal proxy|          |  |  - /terminal     |  |
+------------------+          |  +------------------+  |
                              |  |   CLAUDE CODE    |  |
                              |  |   (dtach session)|  |
                              +------------------------+
```

## Design Principles

1. **Simple** - Minimal dependencies, single Go binary
2. **Fast startup** - Ready within 1 second of container start
3. **Low overhead** - Minimal CPU/memory footprint
4. **Stateless** - No persistent state, restarts cleanly
5. **Discoverable** - Self-detects CLI tool and version at startup

## API Specification

### GET /health

Basic liveness check.

```json
{"status": "ok"}
```

### GET /status

Comprehensive sleeve status. Envoy calls this instead of exec'ing into containers.

```json
{
  "dhf": {
    "name": "Claude Code",
    "version": "2.1.20"
  },
  "workspace": {
    "path": "/home/claude/workspace",
    "cstack": {
      "exists": true,
      "open": 5,
      "ready": 2,
      "in_progress": 1,
      "blocked": 0,
      "closed": 42,
      "total": 50
    }
  },
  "process": {
    "pid": 7,
    "uptime_seconds": 3600,
    "memory_bytes": 524288000,
    "memory_percent": 12.5
  },
  "auth": {
    "claude": true,
    "gemini": false
  }
}
```

**Implementation notes:**
- DHF: Detect at startup, cache forever (try claude, gemini, codex in order)
- Cstack: Run `cs stats --json` in workspace, cache 5 seconds
- Process: Read from `/proc/self/status` and `/proc/self/stat`
- Auth: Check file existence (credentials.json, .gemini/, etc.)

### GET /terminal (WebSocket)

Optional: Terminal access via WebSocket. Connects to existing dtach session.

Query params:
- `mode=observe` - Read-only mode (no input sent to terminal)

**Current approach:** Envoy handles terminal gateway via docker exec to dtach.
**Future option:** Sidecar owns dtach session, envoy proxies WebSocket.

For v1, terminal stays in envoy. Consider moving in v2 if benefits justify complexity.

## What Stays in Envoy

These operations require Docker socket or cross-sleeve coordination:

| Operation | Reason |
|-----------|--------|
| Spawn/Kill containers | Docker API access |
| Container recovery | Enumerate containers on boot |
| Resource limits | Set at container creation |
| Workspace mutex | Prevent concurrent mounts |
| Name allocation | Global pool management |
| Git operations | Host SSH keys required |
| Agent doctor sync | Central file distribution |
| System diagnostics | Host-level checks |

## What Moves to Sidecar

| Operation | Before | After |
|-----------|--------|-------|
| DHF detection | Exec `claude --version` | GET /status |
| Cstack stats | Exec `cs stats --json` | GET /status |
| Memory/CPU | Docker stats API | GET /status |
| Auth status | Check host files | GET /status |

**Benefits:**
- Faster /api/sleeves response (no exec per sleeve)
- More accurate resource stats (from inside cgroup)
- Reduced Docker API load
- Sleeve self-awareness for future features

## Implementation

### Directory Structure

```
cmd/sidecar/
  main.go           # Entry point, starts HTTP server

internal/sidecar/
  server.go         # HTTP handlers
  dhf.go            # CLI detection logic
  cstack.go         # Cstack stats integration
  process.go        # Process stats from /proc
  auth.go           # Credential checks
```

### Startup Sequence

```
1. Parse environment (SLEEVE_NAME, WORKSPACE_PATH)
2. Detect DHF (try each CLI, cache result)
3. Start HTTP server on :8080
4. Log "sidecar ready"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| SIDECAR_PORT | HTTP listen port | 8080 |
| WORKSPACE_PATH | Mounted workspace | /home/claude/workspace |
| SLEEVE_NAME | Container name (from label) | (required) |

### Caching Strategy

| Data | TTL | Reason |
|------|-----|--------|
| DHF name/version | Forever | Doesn't change during sleeve lifetime |
| Cstack stats | 5 seconds | Balance freshness vs cs invocation cost |
| Process stats | None | Always fresh from /proc |
| Auth status | 30 seconds | Files rarely change |

## Integration with Envoy

### Before (Current)

```go
// handlers.go - list sleeves
for _, sleeve := range sleeves {
    // Exec into container for DHF
    dhf, _ := s.docker.GetDHFInfo(ctx, sleeve.ContainerID)
    sleeve.DHF = dhf.Name
    sleeve.DHFVersion = dhf.Version
}
```

### After (With Sidecar)

```go
// handlers.go - list sleeves
for _, sleeve := range sleeves {
    // HTTP call to sidecar
    status, _ := s.getSidecarStatus(ctx, sleeve.ContainerName)
    sleeve.DHF = status.DHF.Name
    sleeve.DHFVersion = status.DHF.Version
    sleeve.Integrity = calculateIntegrity(status.Workspace.Cstack)
}
```

### Fallback

If sidecar is unreachable (starting up, crashed), envoy should:
1. Return partial data (omit DHF/cstack)
2. Mark sleeve status as "degraded"
3. Retry on next poll cycle

## Future Extensions (v2+)

### Outbox Endpoint
```
GET /outbox
POST /outbox/clear
```
For needlecast message retrieval (inter-sleeve communication).

### Task Status
```
GET /tasks
```
Real-time task list from cstack for richer UI.

### File Browser
```
GET /files?path=/home/claude/workspace
```
List/read workspace files for web UI file browser.

### Metrics Endpoint
```
GET /metrics
```
Prometheus-format metrics for observability stack.

### Resleeve Support
```
POST /resleeve
```
Trigger CLI switch (soft resleeve) from within container.

## Security Considerations

1. **Network isolation** - Sidecar only accessible on raven network
2. **No secrets in API** - Auth endpoint reports boolean, not credentials
3. **Read-only filesystem access** - Sidecar doesn't modify workspace
4. **Resource limits** - Sidecar respects container cgroup limits

## Testing

```bash
# Unit tests
go test ./internal/sidecar/...

# Integration test (requires running sleeve)
curl http://sleeve-quell:8080/health
curl http://sleeve-quell:8080/status | jq
```

## Rollout Plan

1. **Phase 1:** Build sidecar binary, add to sleeve image
2. **Phase 2:** Start sidecar in entrypoint.sh alongside CLI
3. **Phase 3:** Update envoy to prefer sidecar /status over exec
4. **Phase 4:** Remove exec-based DHF detection from envoy
5. **Phase 5:** Add optional terminal endpoint (v2)
