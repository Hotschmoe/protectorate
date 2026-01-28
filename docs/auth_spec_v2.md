# Authentication Specification v2: Envoy-Centric Auth

## Overview

This document specifies a new authentication architecture for Protectorate that:
1. Centralizes authentication in the Envoy container
2. Eliminates dependency on host machine credentials
3. Supports multiple AI CLIs (Claude Code, Gemini CLI, Codex)
4. Enables platform-agnostic deployment (VM, local, cloud)

---

## Current State Analysis

### Problems with Host-Mounted Credentials

```
Current Flow:
                         HOST MACHINE
                    ┌─────────────────────┐
                    │  ~/.claude/         │
                    │  └─ credentials.json│
                    └─────────┬───────────┘
                              │ bind mount (RO)
              ┌───────────────┼───────────────┐
              │               │               │
              v               v               v
         ┌─────────┐    ┌─────────┐    ┌─────────┐
         │  Envoy  │    │ Sleeve1 │    │ Sleeve2 │
         └─────────┘    └─────────┘    └─────────┘
```

**Issues:**
1. **Platform coupling**: Requires host to have credentials pre-configured
2. **Installation friction**: User must authenticate on host before starting
3. **Cloud deployment blocked**: VMs/cloud containers don't have host credentials
4. **Mixed credential sources**: Some env vars, some files, some mounts
5. **Single CLI focus**: Current paths are Claude-specific (`/home/claude/`)

### Current Container User

All containers use `claude` user (UID 1000) with home at `/home/claude`. This is:
- Tool-specific naming (confusing for multi-CLI support)
- Hardcoded across Dockerfiles, entrypoints, and Go code

---

## Proposed Architecture

### Envoy as Auth Source

```
New Flow:
                         ENVOY CONTAINER
                    ┌─────────────────────────────┐
                    │  User authenticates via:    │
                    │  - CLI: `envoy auth claude` │
                    │  - WebUI: /auth page        │
                    │                             │
                    │  Credentials stored in:     │
                    │  /home/agent/.creds/        │
                    │  (named volume)             │
                    └─────────────┬───────────────┘
                                  │
                    Named Volume: agent-creds
                                  │
              ┌───────────────────┼───────────────────┐
              │                   │                   │
              v                   v                   v
         ┌─────────┐        ┌─────────┐        ┌─────────┐
         │  Envoy  │        │ Sleeve1 │        │ Sleeve2 │
         │  (RW)   │        │  (RO)   │        │  (RO)   │
         └─────────┘        └─────────┘        └─────────┘
```

### Key Changes

| Aspect | Current | Proposed |
|--------|---------|----------|
| User name | `claude` | `agent` |
| Home directory | `/home/claude` | `/home/agent` |
| Credential source | Host bind mount | Named volume |
| Auth location | Host machine | Envoy container |
| Write access | Host only | Envoy only |
| Sleeve access | Read-only bind | Read-only volume |

---

## Credential Storage Layout

### Named Volume: `agent-creds`

```
/home/agent/.creds/                    # Volume mount point
├── claude/                            # Claude Code credentials
│   ├── credentials.json               # OAuth tokens
│   ├── settings.json                  # User settings
│   └── plugins/                       # Claude plugins
├── gemini/                            # Gemini CLI credentials
│   └── credentials.json               # Google OAuth tokens
├── codex/                             # Codex CLI credentials
│   └── credentials.json               # OpenAI tokens
└── .auth-state.json                   # Combined auth status (envoy-managed)
```

### Auth State File

```json
{
  "version": 1,
  "updated_at": "2025-01-28T12:00:00Z",
  "providers": {
    "claude": {
      "authenticated": true,
      "expires_at": "2026-01-28T12:00:00Z",
      "type": "oauth",
      "scopes": ["user:inference", "user:profile"]
    },
    "gemini": {
      "authenticated": true,
      "expires_at": null,
      "type": "api_key"
    },
    "codex": {
      "authenticated": false,
      "expires_at": null,
      "type": "api_key"
    }
  }
}
```

---

## Symlink Strategy for AI CLI Compatibility

AI CLIs expect credentials in specific paths. We use symlinks to bridge:

### In Container (via entrypoint)

```bash
# Claude Code expects: ~/.claude/
ln -sf /home/agent/.creds/claude /home/agent/.claude

# Gemini CLI expects: ~/.config/gemini/
mkdir -p /home/agent/.config
ln -sf /home/agent/.creds/gemini /home/agent/.config/gemini

# Codex expects: ~/.codex/ or env var
ln -sf /home/agent/.creds/codex /home/agent/.codex
```

### Benefits of Symlink Approach

1. **Single volume mount**: One volume serves all CLIs
2. **Native CLI compatibility**: Each CLI sees expected paths
3. **Centralized management**: Envoy manages one directory structure
4. **Easy extension**: Add new CLI by adding subdirectory + symlink

---

## UX Flow Options

### Option A: CLI-First (Recommended)

User authenticates each provider via Envoy CLI commands:

```
┌─────────────────────────────────────────────────────────────────┐
│ $ docker run -it protectorate/envoy                             │
│                                                                 │
│ Protectorate Envoy v0.1.0                                       │
│ No AI providers authenticated. Run 'envoy auth' to configure.   │
│                                                                 │
│ $ envoy auth                                                    │
│                                                                 │
│ Authentication Status:                                          │
│   Claude Code  [ ] Not authenticated                            │
│   Gemini CLI   [ ] Not authenticated                            │
│   Codex        [ ] Not authenticated                            │
│                                                                 │
│ Which provider would you like to authenticate?                  │
│   1) Claude Code (recommended)                                  │
│   2) Gemini CLI                                                 │
│   3) Codex                                                      │
│   4) All providers                                              │
│                                                                 │
│ > 1                                                             │
│                                                                 │
│ Authenticating Claude Code...                                   │
│ Opening browser: https://claude.ai/oauth/...                    │
│ (If browser doesn't open, visit this URL manually)              │
│                                                                 │
│ Waiting for authentication... Done!                             │
│                                                                 │
│ [x] Claude Code authenticated (expires: 2026-01-28)             │
│     Subscription: Max                                           │
│     Rate limit: default_claude_max_20x                          │
│                                                                 │
│ $ envoy status                                                  │
│ Envoy: Running                                                  │
│ Auth:  Claude Code [authenticated], Gemini [not configured]     │
│ Sleeves: 0 running, 12 available                                │
└─────────────────────────────────────────────────────────────────┘
```

**Commands:**
- `envoy auth` - Interactive auth wizard
- `envoy auth claude` - Authenticate specific provider
- `envoy auth status` - Show auth status
- `envoy auth revoke <provider>` - Remove credentials

**Pros:**
- Familiar CLI workflow for developers
- Non-interactive mode for automation: `envoy auth claude --token <token>`
- Works in headless environments with token passthrough
- Clear feedback and status

**Cons:**
- Requires terminal access
- OAuth flows need browser (or token paste fallback)

---

### Option B: WebUI-First

User authenticates via browser at `http://localhost:7470/auth`:

```
┌─────────────────────────────────────────────────────────────────┐
│ PROTECTORATE                              [Status] [Auth] [Logs]│
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Authentication                                                 │
│  ─────────────────────────────────────────────────────────────  │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  CLAUDE CODE                              [Authenticate] │   │
│  │  Status: Not authenticated                               │   │
│  │  Required for: Sleeve spawning with Claude               │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  GEMINI CLI                               [Authenticate] │   │
│  │  Status: Not authenticated                               │   │
│  │  Required for: Sleeves with Gemini                       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  CODEX                                    [Authenticate] │   │
│  │  Status: Not authenticated                               │   │
│  │  Required for: Sleeves with Codex                        │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ─────────────────────────────────────────────────────────────  │
│  Tip: You can also authenticate via CLI: `envoy auth`          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Flow:**
1. User clicks "Authenticate" for provider
2. Popup/redirect to OAuth flow
3. Callback to `/auth/callback/<provider>`
4. Credentials stored, UI updates

**Pros:**
- Visual feedback
- Works when running in cloud/VM with port forwarding
- Easier for non-CLI users

**Cons:**
- Requires port exposure
- OAuth callback handling is complex
- More code to maintain

---

### Option C: Hybrid (Recommended Implementation)

Support both CLI and WebUI:

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  CLI Mode (headless, scripts, advanced users):                  │
│  $ envoy auth claude                                            │
│  $ envoy auth claude --token sk-ant-oat01-xxx                   │
│                                                                 │
│  WebUI Mode (visual, cloud, casual users):                      │
│  Open http://localhost:7470/auth                                │
│  Click provider -> OAuth flow -> Done                           │
│                                                                 │
│  Both write to same named volume: agent-creds                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Recommendation**: Implement CLI-first (Option A) as primary, add WebUI later.

---

## Implementation Details

### 1. User Rename: `claude` -> `agent`

**Files to modify:**

```
containers/base/Dockerfile
  - Change: useradd -m -s /bin/bash -u 1000 claude
  - To:     useradd -m -s /bin/bash -u 1000 agent

containers/envoy/entrypoint.sh
  - Change all /home/claude references to /home/agent
  - Change: su - claude
  - To:     su - agent

containers/sleeve/entrypoint.sh
  - Same changes as envoy

internal/envoy/sleeve_manager.go
  - Update mount targets from /home/claude to /home/agent

internal/envoy/handlers.go
  - Update credential path checks

internal/sidecar/auth.go
  - Update home directory expectations
```

**Migration note**: This is a breaking change. Users must rebuild images.

---

### 2. Named Volume Setup

**Create volume on first run (entrypoint.sh):**

```bash
# In envoy entrypoint
docker volume inspect agent-creds >/dev/null 2>&1 || \
    docker volume create agent-creds
```

**Mount in docker-compose.yaml:**

```yaml
services:
  envoy:
    volumes:
      - agent-creds:/home/agent/.creds:rw  # Envoy can write
      - /var/run/docker.sock:/var/run/docker.sock

volumes:
  agent-creds:
    name: agent-creds
```

**Mount for sleeves (SleeveManager.go):**

```go
mounts = append(mounts, mount.Mount{
    Type:     mount.TypeVolume,
    Source:   "agent-creds",
    Target:   "/home/agent/.creds",
    ReadOnly: true,  // Sleeves only read
})
```

---

### 3. Auth Command Implementation

**New file: `cmd/envoy/auth.go`**

```go
package main

import (
    "fmt"
    "os"
    "os/exec"

    "github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
    Use:   "auth [provider]",
    Short: "Authenticate AI providers",
    Long:  `Authenticate Claude Code, Gemini CLI, or Codex for use in sleeves.`,
    Run:   runAuth,
}

func runAuth(cmd *cobra.Command, args []string) {
    if len(args) == 0 {
        // Interactive mode - show menu
        showAuthMenu()
        return
    }

    provider := args[0]
    switch provider {
    case "claude":
        authClaude()
    case "gemini":
        authGemini()
    case "codex":
        authCodex()
    case "status":
        showAuthStatus()
    default:
        fmt.Fprintf(os.Stderr, "Unknown provider: %s\n", provider)
        os.Exit(1)
    }
}

func authClaude() {
    // Option 1: Use claude setup-token for long-lived token
    // Option 2: Run claude auth login and let it write to ~/.creds/claude/

    credsDir := "/home/agent/.creds/claude"
    os.MkdirAll(credsDir, 0700)

    // Set CLAUDE_CONFIG_DIR to redirect credential storage
    os.Setenv("CLAUDE_CONFIG_DIR", credsDir)

    // Run claude auth
    cmd := exec.Command("claude", "auth", "login")
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    // Update auth state
    updateAuthState("claude", true)
}
```

---

### 4. Symlink Setup in Entrypoint

**containers/base/entrypoint-common.sh:**

```bash
#!/bin/bash
# Common entrypoint logic for envoy and sleeves

setup_cred_symlinks() {
    local home="/home/agent"
    local creds="/home/agent/.creds"

    # Claude Code: ~/.claude -> ~/.creds/claude
    if [ -d "$creds/claude" ]; then
        ln -sfn "$creds/claude" "$home/.claude"
    fi

    # Gemini CLI: ~/.config/gemini -> ~/.creds/gemini
    if [ -d "$creds/gemini" ]; then
        mkdir -p "$home/.config"
        ln -sfn "$creds/gemini" "$home/.config/gemini"
    fi

    # Codex: ~/.codex -> ~/.creds/codex
    if [ -d "$creds/codex" ]; then
        ln -sfn "$creds/codex" "$home/.codex"
    fi
}

setup_cred_symlinks
```

---

### 5. Config Changes

**configs/envoy.yaml:**

```yaml
# Remove host credential paths (deprecated)
# docker:
#   credentials_host_path: ...   # REMOVED
#   settings_host_path: ...      # REMOVED

# Add volume configuration
docker:
  network: raven
  credentials_volume: agent-creds
  workspace_root: /home/agent/workspaces
  sleeve_image: protectorate/sleeve:latest

# Auth configuration
auth:
  # Supported providers
  providers:
    - claude
    - gemini
    - codex
  # Default provider for new sleeves
  default_provider: claude
```

---

## Migration Path

### Phase 1: User Rename (Breaking Change)

1. Update all Dockerfiles: `claude` -> `agent`
2. Update all Go code paths
3. Update entrypoints
4. Document breaking change in CHANGELOG
5. Bump major version

### Phase 2: Named Volume (Non-Breaking)

1. Add volume support alongside existing bind mounts
2. Deprecation warning for host credential paths
3. Auto-migrate: if host credentials exist, copy to volume

### Phase 3: Auth Commands (Additive)

1. Add `envoy auth` subcommand
2. Add auth status to webui
3. Add auth health checks

### Phase 4: Remove Host Dependencies (Breaking)

1. Remove bind mount credential support
2. Remove deprecated config options
3. Update documentation

---

## Pros and Cons

### Pros

1. **Platform agnostic**: No host dependencies, runs anywhere Docker runs
2. **Self-contained**: Single `docker run` to start everything
3. **Multi-CLI support**: One volume for all providers
4. **Security**: Clear permission model (envoy writes, sleeves read)
5. **Debuggable**: Named volumes are inspectable (`docker volume inspect`)
6. **Cloud-ready**: Works in VM, cloud container services
7. **Consistent naming**: `agent` user is provider-agnostic

### Cons

1. **Breaking change**: User rename requires full rebuild
2. **Volume lifecycle**: Must manage volume separately from containers
3. **OAuth complexity**: Envoy must handle OAuth callbacks
4. **Symlink maintenance**: Must keep symlinks in sync with CLI expectations
5. **Initial setup**: User must authenticate inside container (can't just mount host creds)

---

## Platform Agnosticism Assessment

### Is Keeping Everything in Envoy a Good Idea?

**YES** - with caveats:

**Strengths:**
- Single deployment unit (one docker-compose or one `docker run`)
- No host configuration required
- Portable across environments
- Clear data boundaries (workspace volume, credential volume)

**Caveats:**
- **Volume persistence**: Named volumes persist independently. Users must manage:
  - Backup: `docker run --rm -v agent-creds:/data alpine tar czf - /data > backup.tar.gz`
  - Restore: `docker run --rm -v agent-creds:/data alpine tar xzf - < backup.tar.gz`
- **Multi-host**: Named volumes are node-local. For multi-host, need:
  - Volume driver plugins (NFS, EFS, etc.)
  - Or: re-authenticate on each host
- **Credential renewal**: If OAuth tokens expire, user must re-auth in envoy

**Recommendation**: The envoy-centric approach is correct for the target use case (single machine with 12+ sleeves). For multi-host clusters, document the volume driver approach but don't implement initially.

---

## Workspace Considerations

The same pattern applies to workspaces:

```
Current:  Host bind mount  -> Sleeves
Proposed: Named volume     -> Envoy -> Sleeves
```

**Benefits:**
- Envoy can manage workspace lifecycle
- Clone repos inside envoy, sleeves just mount
- Easy to snapshot/backup entire workspace volume

**Implementation:**
```yaml
volumes:
  agent-creds:
    name: agent-creds
  agent-workspaces:
    name: agent-workspaces
```

This keeps Protectorate fully isolated from host filesystem.

---

## Open Questions

1. **Token refresh**: Should envoy have a background job to refresh OAuth tokens proactively?
2. **Multi-provider sleeves**: Can a sleeve use multiple CLIs? (Probably yes, with all symlinks)
3. **API key support**: For Gemini/Codex, support `GEMINI_API_KEY` env var as alternative to OAuth?
4. **Credential encryption**: Should credentials be encrypted at rest in the volume?
5. **Audit logging**: Log credential access/refresh events?

---

## Appendix: AI CLI Credential Paths

| CLI | Default Path | Env Override | Notes |
|-----|--------------|--------------|-------|
| Claude Code | `~/.claude/` | `CLAUDE_CONFIG_DIR` | OAuth, long-lived tokens |
| Gemini CLI | `~/.config/gemini/` | `GEMINI_CONFIG_DIR`? | Google OAuth |
| Codex | `~/.codex/` | `OPENAI_API_KEY` | API key or OAuth |
| Aider | `~/.aider/` | Various env vars | Multiple provider support |
| OpenCode | `~/.opencode/` | Unknown | TBD |

---

## Appendix: Docker Command Examples

### Start Envoy (First Time)

```bash
docker run -d \
  --name protectorate-envoy \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v agent-creds:/home/agent/.creds \
  -v agent-workspaces:/home/agent/workspaces \
  -p 7470:7470 \
  protectorate/envoy:latest
```

### Authenticate

```bash
docker exec -it protectorate-envoy envoy auth claude
```

### Check Status

```bash
docker exec protectorate-envoy envoy auth status
```

### Backup Credentials

```bash
docker run --rm \
  -v agent-creds:/creds:ro \
  -v $(pwd):/backup \
  alpine tar czf /backup/creds.tar.gz -C /creds .
```
