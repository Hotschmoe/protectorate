# SPEC: Cortical Stack (Memory System)

## Overview

The Cortical Stack is the memory and task management system for Protectorate sleeves. Rather than building from scratch, we fork and adapt **beads** - a git-backed graph issue tracker designed for AI agents.

**Decision**: Fork beads, adapt for sleeve lifecycle.

## Open Questions

### Q1: Go-based beads or Rust-based beads?

| Option | Repository | Notes |
|--------|------------|-------|
| Original (Go) | github.com/steveyegge/beads | Steve Yegge's original implementation |
| Rust rewrite | github.com/Dicklesworthstone/beads_rust | Community rewrite with enhancements |

**Considerations**:
- Protectorate is written in Go (easier integration with Go version?)
- Rust version may have additional features or better performance
- Maintenance and community activity
- Feature completeness
- Schema differences

**Research needed**: Deep comparison of both implementations.

### Q2: Include beads_viewer in fork?

Repository: github.com/Dicklesworthstone/beads_viewer

beads_viewer is a companion tool providing:
- Graph intelligence (PageRank, betweenness centrality, etc.)
- Robot protocol (JSON output for agents)
- Semantic search
- Triage recommendations
- History correlation with git commits

**We do NOT need**: The TUI interface

**We MAY want**: The analysis/ranking algorithms for:
- `--robot-triage` - prioritized work recommendations
- `--robot-plan` - parallel execution tracks
- `--robot-insights` - graph metrics
- `--robot-search` - semantic search

**Question**: Should these capabilities be merged into our beads fork, or kept separate?

---

## User Workflow Vision

The cortical stack enables an autonomous agent workflow:

```
INTERVIEW --> SPEC --> BEADS --> AUTONOMOUS WORK --> COMPLETION
```

### Phase 1: Interview (Human + AI)

User starts with an idea, ethos, vision. AI conducts structured interview:
- 70-100 questions covering requirements, constraints, preferences
- Captures edge cases, error handling expectations
- Documents non-functional requirements (performance, security, etc.)
- Records user's mental model and terminology

Output: `INTERVIEW.md` - comprehensive requirements capture

### Phase 2: Specification (AI generates)

AI synthesizes interview into formal specification:
- Architecture overview
- Component breakdown
- API contracts
- Data models
- Integration points

Output: `SPEC.md` - detailed technical specification

### Phase 3: Beads Generation (AI decomposes)

AI breaks specification into atomic tasks:
- Each task is a single bead with clear acceptance criteria
- Dependencies explicitly modeled (blocks/blocked-by)
- Milestones defined as parent beads
- Testing milestones interspersed throughout
- Estimate: **300-600 beads** for a substantial project

Output: `.beads/` populated with full task graph

### Phase 4: Autonomous Execution (Sleeves)

Envoy orchestrates sleeve work:
- Sleeve queries `bd ready` for next actionable task
- Works task, updates bead status
- Envoy monitors progress, health, blockers
- Resleeve on stuck/error conditions
- Route questions to other sleeves or escalate to human

### Phase 5: Completion & Notification

Envoy notifies user when:
- Major milestone completed
- Critical blocker requiring human input
- Agent appears stuck (no progress on ready tasks)
- All beads completed

---

## Sleeve Integration

### Boot Sequence

```
1. Sidecar starts, reads .beads/
2. Calls equivalent of `bd ready --json`
3. Presents actionable tasks to AI CLI
4. AI picks task, sidecar updates bead to in_progress
```

### Work Loop

```
1. AI works on current bead
2. On completion: sidecar marks bead done, queries next ready
3. On blocker: sidecar marks blocked, records reason, queries next ready
4. On error: sidecar captures context, may trigger resleeve
```

### Pre-Resleeve

```
1. Sidecar detects resleeve trigger (stuck, error, manual)
2. Auto-captures current state:
   - What was being attempted (hypothesis)
   - What was tried
   - Current errors/blockers
3. Writes to bead comments or hypothesis field
4. Resleeve proceeds
```

### Post-Resleeve

```
1. New sleeve boots, sidecar reads .beads/
2. Finds in_progress bead with hypothesis
3. Presents context: "Previous sleeve was attempting X, got stuck on Y"
4. New sleeve continues or pivots
```

---

## Schema Requirements

Beyond standard beads fields, sleeves need:

### Hypothesis Field

```
What the sleeve is ABOUT TO TRY, not just what was done.
Critical for resleeve continuity.
```

### Auto-Capture Hooks

Triggered automatically by sidecar:
- On error (capture stack trace, context)
- On task completion (capture summary)
- Before resleeve (capture hypothesis)

### Simplified Command Set

Sleeves don't need 40+ commands. Core operations:

```
cs ready          # What can I work on? (JSON)
cs start <id>     # Claim task, mark in_progress
cs done <id>      # Mark completed with optional note
cs block <id>     # Mark blocked with reason
cs hypothesis <id> # Record what I'm about to try
cs triage         # Prioritized recommendations (JSON)
cs search <query> # Find relevant past work (JSON)
```

---

## File Structure

```
.cstack/
  .beads/
    beads.db          # SQLite - fast local queries
    issues.jsonl      # Git-sync format (for multi-sleeve merge)
    config.yaml       # Sleeve-specific defaults
    .history/         # Timestamped backups
  MEMORY.md           # Long-form prose context (patterns, learnings)
```

**Why both SQLite and JSONL?**
- SQLite: Fast local queries, dependency resolution
- JSONL: Git-friendly sync when multiple sleeves touch same workspace

---

## Needlecast Integration

Beads handles TASKS. Needlecast handles MESSAGES.

```
.cstack/.beads/     # What to do (task graph)
.needlecast/        # What to say (inter-sleeve communication)
```

Examples:
- "Task bd-a1b2 blocked on auth module" -> bead update
- "Hey alice, I pushed auth changes, pull and retry" -> needlecast message

---

## Research Results

### beads (Go) vs beads_rust Analysis

#### beads (Go) - Steve Yegge's Original

| Aspect | Details |
|--------|---------|
| **Size** | 206K lines of code, 461 Go files, 360 test files |
| **Commits** | 5,679 total, 4,603 in last 3 months (very active) |
| **Latest** | v0.49.0 (January 22, 2026) |
| **Binary** | ~30+ MB |
| **Architecture** | SQLite + JSONL + optional Dolt backend |
| **Schema** | 125+ issue fields, complex workflow states |
| **Features** | Daemon mode, Linear/Jira sync, Dolt federation, MCP server |
| **Philosophy** | Full-featured, enterprise-ready, automatic operations |

**Strengths**:
- Extremely active development
- Production-proven at scale
- Rich integrations (Linear, Jira, GitHub)
- Daemon for background sync
- MCP server integration

**Weaknesses**:
- Complex (125+ fields, 50+ commands)
- Large binary size
- Automatic git operations (may conflict with sleeve workflow)
- Heavy dependency chain (173 direct deps)

#### beads_rust - Dicklesworthstone's Port

| Aspect | Details |
|--------|---------|
| **Size** | 26K lines Rust, 37 command modules |
| **Stars** | 366 (stable niche adoption) |
| **Latest** | 0.1.8 (January 18, 2026), commits today |
| **Binary** | 5-8 MB (static, no GC) |
| **Architecture** | SQLite + JSONL (frozen on "classic" beads) |
| **Schema** | 30 core fields, clean subset |
| **Features** | All core CRUD, deps, labels, comments, sync |
| **Philosophy** | Non-invasive - NEVER runs git automatically |

**Strengths**:
- Small, fast binary (5-8 MB vs 30+ MB)
- Non-invasive philosophy (explicit sync, no hooks)
- No daemon to manage
- Simpler schema (30 fields vs 125)
- Comprehensive tests with CI/CD
- `#![forbid(unsafe_code)]` - memory safe
- Frozen on stable architecture (no Dolt/federation complexity)

**Weaknesses**:
- No Linear/Jira sync (out of scope)
- No daemon mode
- Smaller community
- Missing some advanced features (molecules, gates, slots)

#### Head-to-Head Comparison

| Criteria | beads (Go) | beads_rust | Winner |
|----------|-----------|------------|--------|
| Binary size | 30+ MB | 5-8 MB | **beads_rust** |
| Startup time | 200+ ms | <50 ms | **beads_rust** |
| Memory usage | 80+ MB | <30 MB | **beads_rust** |
| Feature count | 50+ commands | 37 commands | beads (Go) |
| Schema complexity | 125+ fields | 30 fields | **beads_rust** (simpler) |
| Git behavior | Auto-commits, hooks | Explicit sync only | **beads_rust** (non-invasive) |
| Maintenance | Very active | Active | Tie |
| Container-friendliness | Good | **Excellent** | **beads_rust** |
| Extension/fork effort | High (complex) | Lower (simpler) | **beads_rust** |

---

### beads_viewer Analysis

**Purpose**: Graph-aware AI agent interface built on top of beads.

| Aspect | Details |
|--------|---------|
| **Language** | Go 1.25+ |
| **Size** | 23 packages, 358 files |
| **Main entry** | 6,592 lines (monolithic dispatcher) |
| **TUI** | Charmbracelet (Bubble Tea) stack |
| **Dependencies** | gonum (graph), charmbracelet (TUI), zero external APIs |

#### Robot Protocol Commands (44+ commands)

**Core Triage**:
```
bv --robot-triage          # Mega-command: everything AI needs
bv --robot-next            # Single top recommendation
bv --robot-triage-by-track # Group by execution track
```

**Graph Intelligence**:
```
bv --robot-insights        # Full 9 metrics dashboard
bv --robot-graph           # DAG export (JSON/DOT/Mermaid)
bv --robot-plan            # Parallel execution tracks
```

**Search & History**:
```
bv --robot-search          # Semantic + hybrid search
bv --robot-history         # Bead-to-commit correlation
bv --robot-file-beads      # Beads touching a file
```

**Forecasting**:
```
bv --robot-burndown        # Sprint burndown
bv --robot-forecast        # ETA predictions
bv --robot-capacity        # Parallelization simulation
```

#### 9 Graph Metrics Computed

1. **PageRank** - Network importance (prestige)
2. **Betweenness Centrality** - Bottleneck detection
3. **Eigenvector Centrality** - Influencer detection
4. **HITS (Hubs/Authorities)** - Aggregators vs prerequisites
5. **Critical Path Score** - Longest path to terminal
6. **K-Core Decomposition** - Structural cohesion
7. **Articulation Points** - Single points of failure
8. **Slack** - Parallelizable work buffer
9. **Cycles** - Circular dependency detection

**Key Design**: Phase 1 metrics are instant. Phase 2 metrics are async with 500ms timeout. Status reporting tells agents which metrics are trustworthy.

#### Extractability Assessment: 8/10

- `pkg/analysis/` - Graph metrics, triage (fully extractable)
- `pkg/correlation/` - Git history linking (standalone)
- `pkg/search/` - Semantic search (standalone)
- `pkg/ui/` - TUI (can be removed for robot-only mode)

**Could be split into**: Library (metrics) + CLI (robot commands) + TUI (optional)

---

### Recommendation

#### Q1 Answer: Fork beads_rust

**Rationale**:

1. **Container-optimized** - 5-8 MB binary, <50ms startup, <30MB memory. Sleeves need to be lightweight.

2. **Non-invasive philosophy** - beads_rust never runs git automatically. This is critical because:
   - Sleeves shouldn't auto-commit (envoy controls git operations)
   - Explicit sync prevents race conditions between sleeves
   - No daemon = simpler container lifecycle

3. **Simpler to extend** - 26K lines vs 206K lines. Adding hypothesis field, resleeve hooks, sidecar integration is straightforward.

4. **Frozen architecture** - beads_rust is intentionally frozen on SQLite + JSONL. No Dolt federation, no molecules, no gates. This stability is a feature for us.

5. **Schema fits our needs** - 30 fields covers everything Kovac requested. We add hypothesis field, that's it.

**What we give up**: Linear/Jira sync (don't need), daemon mode (don't need), Dolt federation (don't need), advanced molecules/gates (don't need).

#### Q2 Answer: Fork beads_viewer as Separate Tool

**Rationale**:

1. **Don't merge - keep separate** - beads_viewer is Go, beads_rust is Rust. Different languages, different build chains.

2. **Include in sleeve containers** - Both `br` (beads_rust) and `bv` (beads_viewer) installed in sleeves:
   - `br` for CRUD operations (create, update, close, sync)
   - `bv --robot-*` for intelligence (triage, insights, plan)

3. **Strip the TUI** - Fork beads_viewer, remove Charmbracelet TUI dependencies, keep only `--robot-*` commands. Results in smaller binary.

4. **Standardize JSON schema** - Ensure `br --json` and `bv --robot-*` output compatible formats for sidecar consumption.

#### Final Architecture

```
Sleeve Container
  |
  +-- br (beads_rust fork)
  |     - CRUD: create, update, close, sync
  |     - Query: ready, list, search, dep tree
  |     - ~8 MB binary
  |
  +-- bv (beads_viewer fork, robot-only)
  |     - Intelligence: triage, insights, plan
  |     - Correlation: history, file-beads
  |     - ~15 MB binary (no TUI)
  |
  +-- Sidecar
        - Calls br/bv, exposes /status, /health
        - Auto-capture hooks
        - Hypothesis management
```

#### Fork Names

| Upstream | Fork Name | Purpose |
|----------|-----------|---------|
| beads_rust | `cortical-stack` or `cs` | Task CRUD, sync |
| beads_viewer | `cortical-viewer` or `cv` | Graph intelligence |

Or keep the `br`/`bv` names for familiarity.

---

## Implementation Checklist

### Phase 1: Fork & Baseline

- [ ] Fork beads_rust to hotschmoe/cortical-stack
- [ ] Fork beads_viewer to hotschmoe/cortical-viewer
- [ ] Verify both build and pass tests
- [ ] Document upstream commit hashes

### Phase 2: Customize beads_rust (cs)

- [ ] Add `hypothesis` field to Issue struct
- [ ] Add `cs hypothesis <id> "text"` command
- [ ] Add `--sleeve-mode` flag (JSON-only output, no colors)
- [ ] Simplify to ~15 core commands (remove rarely-used)
- [ ] Update README for Protectorate use case

### Phase 3: Customize beads_viewer (cv)

- [ ] Remove TUI (Charmbracelet stack)
- [ ] Keep only `--robot-*` commands
- [ ] Add `--hypothesis` flag to triage output
- [ ] Ensure JSON schemas match cs output
- [ ] Reduce binary size

### Phase 4: Sidecar Integration

- [ ] Sidecar calls `cs ready --json` for actionable work
- [ ] Sidecar calls `cv --robot-triage` for prioritization
- [ ] Auto-capture: `cs hypothesis` before resleeve
- [ ] Auto-capture: `cs comment` on errors
- [ ] Expose via `/status` endpoint

### Phase 5: Envoy Integration

- [ ] Envoy queries sleeve sidecars for triage data
- [ ] Envoy aggregates cross-sleeve insights
- [ ] Envoy detects stuck sleeves (no progress on ready)
- [ ] Envoy notifies user on milestones/blockers

---

## References

- [beads (Go)](https://github.com/steveyegge/beads)
- [beads_rust](https://github.com/Dicklesworthstone/beads_rust)
- [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer)
- [Kovac Conversation](./kovac_convo.md) - Early sleeve feedback on memory needs
