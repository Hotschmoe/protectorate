#!/bin/bash
set -e

TMUX_SESSION="main"

# Create session if it doesn't exist
tmux new-session -d -s "$TMUX_SESSION"

# Start with shell
tmux send-keys -t "$TMUX_SESSION" "cd /workspace && echo 'Type: claude'" Enter

# Start sleeve-agent
exec sleeve-agent --addr :7681
