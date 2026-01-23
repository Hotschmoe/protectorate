# Protectorate

Container-native AI agent orchestration system written in Go.

Named after the interstellar governing body in Altered Carbon: containers are "sleeves" (bodies), AI tools are the consciousness, and Protectorate manages them all.

## TLDR

Protectorate spawns a manager container called **Envoy** that orchestrates AI agent containers called **Sleeves**. Each sleeve runs an AI CLI (Claude, Gemini, Codex, etc.) to work on tasks. Sleeves operate in workspaces where:

- **Cortical Stack** (`.cstack/`) - Long-term memory that persists across sessions
- **Needlecast** (`/needlecast/`) - Inter-agent communication system

When an agent gets stuck or needs a different approach, Envoy can **resleeve** the container to a different AI (swap Gemini for Claude) while keeping the cortical stack intact - memories and task context persist across the transition.

## Philosophy

```
WE DO NOT: Modify AI CLI tools (Claude Code, Gemini CLI, etc.)
WE DO:     Orchestrate dozens of them with shared memory and coordination
```

## Architecture

```
                    +------------------------+
                    |        ENVOY           |
                    |    (Manager Process)   |
                    |  - Spawns sleeves      |
                    |  - Routes messages     |
                    |  - Health monitoring   |
                    +----------+-------------+
                               |
         +---------------------+---------------------+
         |                     |                     |
         v                     v                     v
  +---------------+     +---------------+     +---------------+
  | Sleeve Alice  |     | Sleeve Bob    |     | Sleeve Carol  |
  | Claude Code   |     | Gemini CLI    |     | OpenCode      |
  | .cstack/      |     | .cstack/      |     | .cstack/      |
  +---------------+     +---------------+     +---------------+
```

## Components

| Component | Description |
|-----------|-------------|
| Envoy | Manager container with Docker socket access, spawns/kills sleeves, routes messages |
| Sleeve | Agent container with AI CLI, sidecar, and mounted workspace |
| Sidecar | Lightweight HTTP server exposing /health, /status, /outbox endpoints |

## Prerequisites

- Docker with BuildKit
- Go 1.24+
- `inotify-tools` (optional, for file watcher)

```bash
# Ubuntu/Debian - install file watcher support
sudo apt-get install inotify-tools
```

## Quick Start

```bash
# 1. Build base image (slow, only needed once)
make build-base

# 2. Start dev environment (fast iteration)
make dev
```

**Web UI:** http://localhost:7470

## Development (Fast Iteration)

The dev environment uses volume-mounted binaries and hot-reload for the webui:

```bash
make dev           # Start dev environment
make dev-restart   # Rebuild Go + recreate container (picks up all changes)
make dev-logs      # View container logs
make dev-down      # Stop dev environment
make watch         # Auto-rebuild on file changes (requires inotify-tools)
```

| Change Type | Action |
|-------------|--------|
| HTML/CSS/JS | Just refresh browser |
| Go code | `make dev-restart` |
| Compose/entrypoint | `make dev-restart` |

See [docs/build_optimizations.md](docs/build_optimizations.md) for details.

## Production Build

```bash
make build-base    # Build base image (~2 min, run once)
make build-sleeve  # Build sleeve image
make build-envoy   # Build envoy image (dev: local Go)
make build         # Build envoy + sleeve
make release       # Full production build (multi-stage)

make up            # Start production services
make down          # Stop services
```

## API

**Envoy Manager (port 7470)**
```
GET  /sleeves               List all sleeves
POST /sleeves               Spawn new sleeve
DELETE /sleeves/{id}        Kill sleeve
POST /sleeves/{id}/resleeve Soft or hard resleeve
```

**Sidecar (port 8080)**
```
GET  /health    Health check
GET  /status    Sleeve status from .cstack/
GET  /outbox    Read outbox messages
```

## CLI

```bash
envoy spawn --repo foo --cli claude-code    # Spawn sleeve
envoy status                                 # List sleeves
envoy resleeve agent-alice --cli gemini     # Swap CLI
envoy kill agent-bob                         # Kill sleeve
```

## Related

- [cortical-stack](../repo_cortical-stack) - Memory format used by sleeves
