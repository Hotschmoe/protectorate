# TODO - Next Session

## Claude Root User Issue

**Problem**: `--dangerously-skip-permissions` cannot be used with root/sudo privileges for security reasons.

```
root@6817665a2de8:/workspace# cd /workspace && claude --dangerously-skip-permissions
--dangerously-skip-permissions cannot be used with root/sudo privileges for security reasons
```

**Current State**:
- Sleeve containers run as root
- Claude Code refuses `--dangerously-skip-permissions` when running as root
- Without this flag, the workspace trust dialog blocks auto-launch

**Potential Solutions**:
1. Create a non-root user in the sleeve container and run Claude as that user
2. Pre-accept workspace trust via mounted config (if possible)
3. Use a different flag combination that works with root
4. Investigate if there's a way to disable the root check

**Files to Modify**:
- `containers/sleeve/Dockerfile` - Add non-root user
- `containers/sleeve/entrypoint.sh` - Switch to non-root user before launching Claude

## V0.1 Status

Working:
- Envoy web UI on :7470
- Docker containers/networks overview
- Claude auth indicator (green/red)
- Spawn/kill sleeve containers
- WebSocket terminal proxy to ttyd (fixed protocol encoding)
- Workspace management (create/select)
- Settings inheritance from host (~/.claude.json mounted)

Not Working:
- Auto-launch Claude (blocked by root user + permissions issue above)
