# TODO - Next Session

## V0.1 Complete

All V0.1 functionality is now working:
- Envoy web UI on :7470
- Docker containers/networks overview
- Claude auth indicator (green/red)
- Spawn/kill sleeve containers
- WebSocket terminal proxy to ttyd (fixed protocol encoding)
- Workspace management (create/select)
- Settings inheritance from host (~/.claude.json mounted)
- Auto-launch Claude (fixed by running as non-root user)

## Root User Issue - RESOLVED

**Solution**: Created non-root `claude` user (UID 1000) in sleeve container.

**Changes made**:
- `containers/sleeve/Dockerfile` - Added `claude` user with home directory
- `containers/sleeve/entrypoint.sh` - Runs tmux/claude as `claude` user via `su`
- `internal/envoy/sleeve_manager.go` - Updated mount targets to `/home/claude/`

Claude now launches with `--dangerously-skip-permissions` successfully since it runs as a regular user.
