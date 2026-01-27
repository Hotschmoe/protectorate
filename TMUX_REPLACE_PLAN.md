# TMUX Replacement Plan

Replace tmux with either **Zellij** or **abduco** based on testing results.

**Goal:** Native mouse wheel scrollback without hotkeys, plus features beneficial for sleeve orchestration.

---

## Current Architecture

```
Browser (xterm.js)
    |
    v [WebSocket]
Envoy WS Proxy (:7470)
    |
    v [WebSocket]
ttyd (:7681)
    |
    v [PTY]
tmux session
    |
    v
Claude CLI / Shell
```

**Problems with tmux:**
- Requires copy-mode for scroll (even with custom bindings)
- Heavy for our use case (we use 0% of multiplexing features)
- Complex configuration

---

## Candidates

### Option A: Zellij

```
Browser
    |
    v [WebSocket]
Zellij Web Server (:7682)
    |
    v
Zellij session
    |
    v
Claude CLI / Shell
```

**Removes:** ttyd, tmux, Envoy WS proxy layer

### Option B: abduco + ttyd

```
Browser (xterm.js)
    |
    v [WebSocket]
Envoy WS Proxy (:7470)
    |
    v [WebSocket]
ttyd (:7681)
    |
    v [PTY]
abduco session
    |
    v
Claude CLI / Shell
```

**Removes:** tmux only

---

## Feature Comparison for Protectorate

| Feature | Zellij | abduco | Importance |
|---------|--------|--------|------------|
| Native mouse scroll | Yes | Yes (terminal handles) | HIGH |
| Read-only observation | No built-in | Yes (`-r` flag) | HIGH |
| Session listing | Via CLI | Built-in (no args) | MEDIUM |
| Exit status tracking | Via resurrection | Built-in | MEDIUM |
| Multiple observers | Yes | Yes | HIGH |
| Web client built-in | Yes | No (needs ttyd) | MEDIUM |
| Session resurrection | Yes (with layout) | No | LOW |
| Plugin system | Yes (WASM) | No | LOW |
| Binary size | ~15MB | ~50KB | LOW |
| Container footprint | Smaller (no ttyd) | Larger (needs ttyd) | LOW |

---

## Future Protectorate Requirements

### 1. Envoy Observation Mode

Envoy needs to observe sleeve activity without interfering.

**Use cases:**
- Health monitoring (is CLI responsive?)
- Activity detection (is sleeve working or idle?)
- Log streaming to webui
- Debugging stuck sleeves

**abduco advantage:** Native `-r` read-only flag
**Zellij approach:** Connect second client (both can type)

### 2. Command Injection

Envoy may need to send commands to sleeves.

**Use cases:**
- Trigger soft resleeve (send exit command)
- Inject context updates
- Emergency abort

**Both support:** Write to session socket/attach and send keys

### 3. Session State Detection

Detect if CLI has crashed vs is waiting for input.

**Use cases:**
- Auto-restart crashed CLIs
- Distinguish "thinking" from "crashed"
- Report accurate status to webui

**abduco advantage:** Exit status preserved and queryable
**Zellij approach:** Check session state via CLI

### 4. Multi-Tenant Observation

Multiple users watching same sleeve.

**Use cases:**
- Team debugging
- Demo/presentation mode
- Audit logging

**Both support:** Multiple clients can attach

### 5. Sidecar Integration (Future)

Per Q_TODO_NEXT.md, sidecar will eventually manage CLI as child process.

```
Future architecture:
Sidecar
    |
    +-- abduco/zellij session
    |       |
    |       +-- Claude CLI
    |
    +-- HTTP API (:8080)
```

**Requirements:**
- Sidecar spawns session programmatically
- Sidecar can query session state
- Sidecar can inject commands
- Sidecar can kill/restart CLI without killing session

**abduco advantage:** Simpler to script, clear socket-based API
**Zellij advantage:** Plugin system could embed sidecar logic

---

## Testing Plan

### Phase 1: Base Image Update

Install both candidates in base image for A/B testing.

**Dockerfile changes:**

```dockerfile
# Install abduco (Debian repos)
RUN apt-get update && apt-get install -y abduco

# Install Zellij (GitHub release)
ARG ZELLIJ_VERSION=0.41.2
RUN curl -fsSL \
    "https://github.com/zellij-org/zellij/releases/download/v${ZELLIJ_VERSION}/zellij-x86_64-unknown-linux-musl.tar.gz" \
    | tar -xzf - -C /usr/local/bin \
    && chmod +x /usr/local/bin/zellij

# Zellij config
RUN mkdir -p /home/claude/.config/zellij \
    && cat > /home/claude/.config/zellij/config.kdl << 'EOF'
mouse_mode true
scroll_buffer_size 50000
copy_on_select true
pane_frames false
EOF
```

**Keep tmux temporarily** for fallback during testing.

### Phase 2: Switchable Entrypoints

Add `TERM_MODE` environment variable.

```bash
TERM_MODE=abduco   # abduco + ttyd
TERM_MODE=zellij   # zellij web client
TERM_MODE=tmux     # current behavior (fallback)
```

### Phase 3: Test Matrix

| Test | abduco | Zellij |
|------|--------|--------|
| Mouse wheel scroll | [ ] | [ ] |
| Scroll in vim/less | [ ] | [ ] |
| Session survives disconnect | [ ] | [ ] |
| Session survives container restart | [ ] | [ ] |
| Multiple browser tabs | [ ] | [ ] |
| Read-only observation | [ ] | [ ] |
| Inject command from outside | [ ] | [ ] |
| Detect CLI exit status | [ ] | [ ] |
| Input latency (subjective) | [ ] | [ ] |
| Copy/paste works | [ ] | [ ] |

### Phase 4: Prototype Sidecar Integration

Test programmatic control from Go:

```go
// abduco approach
func (s *Sidecar) AttachReadOnly(session string) (io.Reader, error) {
    cmd := exec.Command("abduco", "-r", session)
    return cmd.StdoutPipe()
}

func (s *Sidecar) InjectCommand(session string, command string) error {
    cmd := exec.Command("abduco", "-a", session)
    stdin, _ := cmd.StdinPipe()
    stdin.Write([]byte(command + "\n"))
    return stdin.Close()
}

// zellij approach
func (s *Sidecar) AttachReadOnly(session string) (io.Reader, error) {
    cmd := exec.Command("zellij", "attach", session)
    return cmd.StdoutPipe()
}

func (s *Sidecar) InjectCommand(session string, command string) error {
    return exec.Command("zellij", "action", "write-chars", command).Run()
}
```

---

## Decision Criteria

**Choose Zellij if:**
- Web client simplification outweighs other factors
- Plugin system becomes valuable for future features
- Session resurrection is needed

**Choose abduco if:**
- Read-only observation is critical path
- Simplicity and scriptability matter more
- Want minimal attack surface

**Hybrid option:**
- Use abduco for sleeves (read-only observation)
- Use Zellij for envoy terminal (nicer UX for operators)

---

## Migration Plan (Post-Decision)

### If Zellij Wins

1. Remove ttyd from base image
2. Remove tmux from base image
3. Update entrypoints to use zellij only
4. Update Envoy WS proxy to connect to Zellij web server
   - Or remove proxy entirely, expose Zellij ports directly
5. Update webui to use Zellij's xterm.js integration

### If abduco Wins

1. Remove tmux from base image
2. Keep ttyd
3. Update entrypoints to use abduco
4. Add read-only endpoint to sidecar API
5. Add `/sleeves/{name}/observe` route for read-only view

### Common Steps

1. Update CLAUDE.md terminology (remove tmux references)
2. Update docs/build_optimizations.md
3. Remove .tmux.conf from base image
4. Test full sleeve lifecycle
5. Update CI/CD if applicable

---

## Files to Modify

| File | Changes |
|------|---------|
| `containers/base/Dockerfile` | Add zellij, abduco; remove tmux after decision |
| `containers/sleeve/entrypoint.sh` | Add TERM_MODE switch |
| `containers/envoy/entrypoint.sh` | Add TERM_MODE switch |
| `docker-compose.dev.yaml` | Add TERM_MODE env, expose 7682 |
| `internal/envoy/ws_proxy.go` | Support Zellij protocol (if chosen) |
| `.env.example` | Document TERM_MODE |

---

## Open Questions

1. **Zellij web auth:** How to handle tokens in container environment?
2. **Zellij HTTPS:** Required for public interface - cert management?
3. **abduco socket location:** Use /tmp or dedicated directory?
4. **Multi-sleeve observation:** Can Envoy observe all sleeves simultaneously?
5. **Resource overhead:** Memory usage of Zellij vs abduco under load?

---

## Next Steps

1. [ ] Update base Dockerfile with both tools
2. [ ] Create switchable entrypoint scripts
3. [ ] Test mouse scroll in both modes
4. [ ] Test read-only observation in both modes
5. [ ] Prototype sidecar integration
6. [ ] Make final decision
7. [ ] Execute full migration
