# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## RULE 1 - ABSOLUTE (DO NOT EVER VIOLATE THIS)

You may NOT delete any file or directory unless I explicitly give the exact command **in this session**.

- This includes files you just created (tests, tmp files, scripts, etc.).
- You do not get to decide that something is "safe" to remove.
- If you think something should be removed, stop and ask. You must receive clear written approval **before** any deletion command is even proposed.

Treat "never delete files without permission" as a hard invariant.

---

### IRREVERSIBLE GIT & FILESYSTEM ACTIONS

Absolutely forbidden unless I give the **exact command and explicit approval** in the same message:

- `git reset --hard`
- `git clean -fd`
- `rm -rf`
- Any command that can delete or overwrite code/data

Rules:

1. If you are not 100% sure what a command will delete, do not propose or run it. Ask first.
2. Prefer safe tools: `git status`, `git diff`, `git stash`, copying to backups, etc.
3. After approval, restate the command verbatim, list what it will affect, and wait for confirmation.
4. When a destructive command is run, record in your response:
   - The exact user text authorizing it
   - The command run
   - When you ran it

If that audit trail is missing, then you must act as if the operation never happened.

---

### Code Editing Discipline

- Do **not** run scripts that bulk-modify code (codemods, invented one-off scripts, giant `sed`/regex refactors).
- Large mechanical changes: break into smaller, explicit edits and review diffs.
- Subtle/complex changes: edit by hand, file-by-file, with careful reasoning.
- **NO EMOJIS** - do not use emojis or non-textual characters.
- ASCII diagrams are encouraged for visualizing flows.
- Keep in-line comments to a minimum. Use external documentation for complex logic.
- In-line commentary should be value-add, concise, and focused on info not easily gleaned from the code.

---

### No Legacy Code - Full Migrations Only

We optimize for clean architecture, not backwards compatibility. **When we refactor, we fully migrate.**

- No "compat shims", "v2" file clones, or deprecation wrappers
- When changing behavior, migrate ALL callers and remove old code **in the same commit**
- No `_legacy` suffixes, no `_old` prefixes, no "will remove later" comments
- New files are only for genuinely new domains that don't fit existing modules
- The bar for adding files is very high

**Rationale**: Legacy compatibility code creates technical debt that compounds. A clean break is always better than a gradual migration that never completes.

---

## Session Completion Checklist

```
[ ] File issues for remaining work
[ ] Run quality gates (tests, linters)
[ ] Run git push and verify success
[ ] Confirm git status shows "up to date"
```

**Work is not complete until `git push` succeeds.**

---

## Project Overview

Protectorate is a container-native AI agent orchestration system written in Go. Named after the Protectorate from Altered Carbon - the interstellar governing body that oversees humanity across settled worlds. In our system: containers are "sleeves" (bodies), AI CLI tools are the consciousness (DHF), and Protectorate orchestrates them all.

```
WE DO NOT: Modify AI CLI tools (Claude Code, Gemini CLI, etc.)
WE DO:     Orchestrate dozens of them with shared memory and coordination
```

## Build and Development Commands

```bash
# Build all binaries
go build ./cmd/envoy
go build ./cmd/sidecar

# Run tests
go test -race ./...

# Run linter (if configured)
golangci-lint run

# Start envoy
./envoy --config configs/envoy.yaml
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

### Core Components

- **Envoy** (`cmd/envoy/`, `internal/envoy/`): Central coordinator running as a container with Docker socket access. Spawns/kills sleeves, performs hourly check-ins, routes inter-sleeve messages, and manages lifecycle.

- **Sidecar** (`cmd/sidecar/`, `internal/sidecar/`): Lightweight HTTP server baked into every sleeve. Exposes `/health`, `/status`, `/outbox`, `/resleeve` endpoints. Parses `.cstack/` files to report sleeve state.

- **Sleeves** (`containers/sleeve/`): Docker containers running AI CLIs (Claude Code, Gemini CLI, OpenCode, etc.) with the sidecar. Contains base OS, AI CLI tool, sidecar binary, and mounted workspace with `.cstack/`.

### Shared Libraries (`internal/`)

- `protocol/`: Shared types (SleeveStatus, Message, SpawnRequest, ResleeveRequest)
- `config/`: YAML configuration loading with environment variable substitution

### State Management

**Cortical Stack (Memory)** - Sleeve's own state in `.cstack/`:
- `CURRENT.md`: Active task, current focus, next steps
- `PLAN.md`: Task list, backlog, notes
- `MEMORY.md`: Long-term context, decisions, learnings

See: [cortical-stack](https://github.com/hotschmoe/cortical-stack) for the memory format specification.

**Needlecast (Communication)** - Inter-sleeve messaging in `/needlecast/`:
- `{sleeve}/INBOX.md`: Messages TO this sleeve
- `{sleeve}/OUTBOX.md`: Messages FROM this sleeve (read and cleared by envoy)
- `arena/GLOBAL.md`: Broadcast messages for all sleeves

### API Ports

- Envoy API: `:7470`
- Sidecar API: `:8080` (per sleeve)

## Key Design Decisions

1. **Containers per sleeve** for isolation, resource limits, failure domains, and scaling
2. **Filesystem as database** - state in plain markdown files for debuggability and crash resilience
3. **Envoy polls sleeves** (not push) for simpler implementation and graceful degradation
4. **Protocol over framework** - simple HTTP APIs and file conventions that any AI CLI can follow
5. **One sleeve per stack** - no concurrent access to same `.cstack/` directory (prevents corruption)

## Configuration

- Envoy config: `configs/envoy.yaml`
- Environment variables: `.env` (copy from `.env.example`)

---

## Terminology

| Term | Meaning |
|------|---------|
| Protectorate | The orchestration framework (this repo) |
| Envoy | The manager process/CLI tool |
| Sleeve | Agent container (the body) |
| Cortical Stack | Agent memory format (.cstack/) - what the sleeve knows |
| Needlecast | Inter-sleeve communication (/needlecast/) - what sleeves say/hear |
| DHF | The AI consciousness (Claude Code, Gemini CLI, etc.) |
| Resleeve | Switch CLI (soft) or respawn container (hard) |

---

## Claude Agents

Specialized agents are available in `.claude/agents/`. Agents use YAML frontmatter format:

```yaml
---
name: agent-name
description: What this agent does
model: sonnet|haiku|opus
tools:
  - Bash
  - Read
  - Edit
---
```

### Available Agents

| Agent | Model | Purpose |
|-------|-------|---------|
| coder-sonnet | sonnet | Fast, precise code changes with atomic commits |
| gemini-analyzer | sonnet | Large-context analysis via Gemini CLI (1M+ context) |
| build-verifier | sonnet | Pre-merge validation for Go build and Docker images |

### Disabling Agents

To disable specific agents in `settings.json` or `--disallowedTools`:
```json
{
  "disallowedTools": ["Task(build-verifier)", "Task(gemini-analyzer)"]
}
```

---

## Go Best Practices

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to spawn agent %s: %w", agentID, err)
}

// Use errors.Is/As for type checking
if errors.Is(err, ErrAgentNotFound) {
    // handle specific error
}
```

### Concurrency

```go
// Use context for cancellation
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

// Prefer channels over shared state
// Use sync.WaitGroup for goroutine coordination
```

### Testing

```go
// Table-driven tests
func TestSpawnAgent(t *testing.T) {
    tests := []struct {
        name    string
        request SpawnRequest
        wantErr bool
    }{
        // test cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

---

## Bug Severity

### Critical - Must Fix Immediately

- Nil pointer dereference
- Data races (use `-race` flag)
- Resource leaks (goroutines, file handles, Docker containers)
- Security vulnerabilities (command injection, path traversal)

### Important - Fix Before Merge

- Missing error handling
- Improper context usage
- Missing Docker resource cleanup (sleeves, networks)
- Unhandled edge cases in API handlers

### Contextual - Address When Convenient

- TODO/FIXME comments
- Suboptimal performance
- Missing test coverage
- Code style inconsistencies
