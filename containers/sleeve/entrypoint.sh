#!/bin/bash
set -e

TMUX_SESSION="main"

# Fix ownership of mounted volumes (runs as root)
chown -R claude:claude /workspace
chown -R claude:claude /home/claude/.claude 2>/dev/null || true

# Start tmux session as claude user
su - claude -c "tmux new-session -d -s $TMUX_SESSION"

# Auto-launch Claude with dangerously-skip-permissions (safe as non-root)
su - claude -c "tmux send-keys -t $TMUX_SESSION 'cd /workspace && claude --dangerously-skip-permissions' Enter"

exec ttyd \
    --port 7681 \
    --writable \
    su - claude -c "tmux attach-session -t $TMUX_SESSION"
