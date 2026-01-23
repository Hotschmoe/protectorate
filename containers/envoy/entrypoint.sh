#!/bin/bash
set -e

TMUX_SESSION="envoy"

# Fix ownership of mounted volumes
chown -R claude:claude /workspace 2>/dev/null || true
chown -R claude:claude /home/claude/.claude 2>/dev/null || true

# Copy read-only mounted settings to writable location
if [ -f /etc/claude/settings.json ]; then
    cp /etc/claude/settings.json /home/claude/.claude/settings.json
    chown claude:claude /home/claude/.claude/settings.json
fi

# Start tmux session as claude user for terminal access
su - claude -c "tmux new-session -d -s $TMUX_SESSION"
su - claude -c "tmux send-keys -t $TMUX_SESSION 'cd /workspace && claude --dangerously-skip-permissions' Enter"

# Start ttyd in background for terminal access
ttyd --port 7681 --writable su - claude -c "tmux attach-session -t $TMUX_SESSION" &

# Run envoy (needs Docker socket access, runs as root)
exec envoy --config /etc/envoy/envoy.yaml
