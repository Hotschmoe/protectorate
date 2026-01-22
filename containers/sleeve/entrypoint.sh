#!/bin/bash
set -e

TMUX_SESSION="main"

tmux new-session -d -s "$TMUX_SESSION"

# Start with shell - auto-launch blocked by root user restriction
# TODO: Create non-root user to enable --dangerously-skip-permissions
tmux send-keys -t "$TMUX_SESSION" "cd /workspace && echo 'Type: claude'" Enter

exec ttyd \
    --port 7681 \
    --writable \
    tmux attach-session -t "$TMUX_SESSION"
