# Protectorate - V1 Specification

## Overview

V1 is single-machine deployment with core sleeve management, message routing, and local git infrastructure.

## Repository Structure

```
protectorate/
  cmd/
    envoy/           # Manager binary entry point
    sidecar/         # Sidecar binary
  internal/
    envoy/           # Manager implementation
    sidecar/         # Sidecar implementation
    protocol/        # Shared types
    config/          # Configuration loading
  containers/
    envoy/           # Manager Dockerfile
    sleeve/          # Sleeve Dockerfile
  configs/
    envoy.yaml       # Default manager config
```

## Containers

### Envoy Manager Container

Single container with two modes:

```
+--------------------------------------------------+
|              ENVOY CONTAINER                      |
|                                                   |
|  +-------------------+   +-------------------+   |
|  | BOOTSTRAP MODE    |   | MANAGER MODE      |   |
|  | (First run)       |   | (Normal operation)|   |
|  |                   |   |                   |   |
|  | - Check env       |   | - Spawn sleeves   |   |
|  | - Setup wizard    |   | - Route messages  |   |
|  | - Create Gitea    |   | - Health checks   |   |
|  | - Init config     |   | - API server      |   |
|  | - Create networks |   | - UI server       |   |
|  +-------------------+   +-------------------+   |
+--------------------------------------------------+
```

### Sleeve Container

```dockerfile
FROM debian:bookworm-slim

# System dependencies
RUN apt-get update && apt-get install -y \
    curl git nodejs npm python3 python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Node.js (for AI CLI tools)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs

# Bun
RUN curl -fsSL https://bun.sh/install | bash
ENV PATH="/root/.bun/bin:$PATH"

# Rust
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | \
    sh -s -- -y --default-toolchain stable
ENV PATH="/root/.cargo/bin:$PATH"

# Zig
RUN curl -fsSL https://ziglang.org/download/0.11.0/zig-linux-x86_64-0.11.0.tar.xz | \
    tar -xJ -C /usr/local && \
    ln -s /usr/local/zig-linux-x86_64-0.11.0/zig /usr/local/bin/zig

# AI CLI tools
RUN npm install -g @anthropic/claude-code

# Sidecar binary
COPY sidecar /usr/local/bin/sidecar

ENTRYPOINT ["/usr/local/bin/sidecar"]
```

Image size: ~1.1GB (spawn time >> image size)

## API

### Envoy Manager API (port 7470)

```
GET  /health                 # Health check
GET  /sleeves               # List all sleeves
POST /sleeves               # Spawn new sleeve
GET  /sleeves/{id}          # Get sleeve info
DELETE /sleeves/{id}        # Kill sleeve
POST /sleeves/{id}/resleeve # Resleeve (soft or hard)
POST /docker/spawn          # Proxy Docker spawn for sleeves
```

### Sidecar API (port 8080)

```
GET  /health    # Health check
GET  /status    # Sleeve status from .cstack/
GET  /outbox    # Read OUTBOX.md messages
POST /resleeve  # Soft resleeve (CLI swap)
```

## Message Routing

```
Sleeve A writes to OUTBOX.md
       |
       v
Envoy reads all OUTBOX files on polling cycle
       |
       v
Envoy routes message to Sleeve B's INBOX.md
       |
       v
Envoy clears processed messages from OUTBOX
       |
       v
Sleeve B reads INBOX.md on next cycle
```

## CLI Commands

```bash
envoy spawn --repo foo --cli claude-code    # Spawn sleeve
envoy status                                 # List sleeves
envoy resleeve agent-alice --cli gemini     # Soft resleeve
envoy resleeve agent-alice --hard           # Hard resleeve
envoy kill agent-bob                         # Kill sleeve
```

## Configuration

### envoy.yaml

```yaml
poll_interval: 1h
idle_threshold: 0  # 0 = never timeout
max_sleeves: 10
port: 7470

docker:
  network: cortical-net
  workspace_root: /workspaces
  sleeve_image: ghcr.io/hotschmoe/protectorate-sleeve:latest

gitea:
  url: http://gitea:3000
  user: cortical
  # token from env: ${GITEA_TOKEN}

mirror:
  enabled: true
  frequency: daily
  github_org: hotschmoe
  # token from env: ${GITHUB_TOKEN}
```

## V1 Scope

### Included

- [x] Envoy manager container (with bootstrap mode)
- [x] Gitea container (spawned by envoy)
- [x] Sleeve container (single image, all CLIs + languages)
- [x] Setup wizard (TUI in manager)
- [x] INBOX/OUTBOX message routing
- [x] Sleeve spawning and lifecycle
- [x] Sidecar health/status API
- [x] Daily GitHub mirror (cron)
- [x] Daily summary agent
- [x] QMD search integration
- [x] Minimal dashboard UI
- [x] Soft resleeve (CLI swap)
- [x] Hard resleeve (container respawn)
- [x] Docker proxy for sleeve testing (via manager)

### Excluded (V2+)

- [ ] Multi-machine (MASTER/SLAVE topology)
- [ ] Traefik reverse proxy
- [ ] Ralphing / agent loops
- [ ] Shared arena messaging
- [ ] Messaging integration (Telegram/Slack)
- [ ] Warm container pool
- [ ] Sleeve deployment to production
- [ ] Beads integration

## Policies

### One Sleeve Per Stack

Strictly enforced - no concurrent access to same .cstack/ directory.

If parallelism needed:
1. Use subagents within the CLI harness (Claude Code supports this)
2. Use separate repos with separate sleeves

### Host Auth Inheritance

AI CLI credentials inherited via volume mount:

```bash
docker run -it \
  -v ~/.claude:/root/.claude \
  -v $(pwd):/workspace \
  protectorate-sleeve
```

User authenticates on host once, all containers inherit.

## Sleeve Naming

Curated list of memorable names:

```
quell, virginia, rei, mickey, trepp, tanaka,
athena, apollo, hermes, iris, prometheus,
hal, samantha, jarvis, cortana, shodan
```

Manager assigns next available name on spawn.

## Implementation Phases

### Phase 1: Foundation
- Create repos and project structure
- Basic manager skeleton
- Sleeve and envoy Dockerfiles
- Push to ghcr.io

### Phase 2: Bootstrap and Gitea
- Setup wizard TUI
- First-run detection
- Gitea spawning and configuration
- Config repo initialization

### Phase 3: Sleeve Lifecycle
- Spawn API endpoint
- Sidecar implementation
- Polling loop and state tracking
- Timeout handling

### Phase 4: Communication and UI
- OUTBOX polling
- INBOX writing
- Dashboard UI
- Manager chat interface

### Phase 5: Mirror and Summary
- GitHub mirror cron
- Summary agent
- QMD search integration
