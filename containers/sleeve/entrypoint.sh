#!/bin/bash
set -e

TMUX_SESSION="main"

tmux new-session -d -s "$TMUX_SESSION"

# Start with shell prompt - user can launch claude manually
# This avoids the interactive setup wizard prompts
tmux send-keys -t "$TMUX_SESSION" "cd /workspace && echo 'Sleeve ready. Type: claude --resume'" Enter

exec ttyd \
    --port 7681 \
    --writable \
    tmux attach-session -t "$TMUX_SESSION"
