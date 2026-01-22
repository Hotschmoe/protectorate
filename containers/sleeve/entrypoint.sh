#!/bin/bash
set -e

TMUX_SESSION="main"

tmux new-session -d -s "$TMUX_SESSION"

tmux send-keys -t "$TMUX_SESSION" "claude --resume" Enter

exec ttyd \
    --port 7681 \
    --writable \
    tmux attach-session -t "$TMUX_SESSION"
