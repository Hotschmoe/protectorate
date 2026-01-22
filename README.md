# Protectorate

Container-native AI agent orchestration system written in Go.

Named after the interstellar governing body in Altered Carbon: containers are "sleeves" (bodies), AI tools are the consciousness, and Protectorate manages them all.

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

## Quick Start

```bash
# 1. Build sleeve base image (slow, only needed once or when upgrading Claude CLI)
make build-base

# 2. Build all images (fast after base exists)
make build

# 3. Start envoy
make up
```

## Build Targets

```bash
make build-base    # Build sleeve-base image (~2 min, run once)
make build-sleeve  # Build sleeve image (~30 sec, uses base)
make build-envoy   # Build envoy image (~5 sec)
make build         # Build envoy + sleeve (requires base)
make build-all     # Build everything including base

make up            # Start services via docker-compose
make down          # Stop services
```

**Web UI:** http://localhost:7470

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
