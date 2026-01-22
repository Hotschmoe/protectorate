#!/bin/bash
set -e

TMUX_SESSION="main"

tmux new-session -d -s "$TMUX_SESSION"

# Auto-launch claude in workspace with permissions bypassed
# --dangerously-skip-permissions skips workspace trust dialog (safe in sandboxed container)
# Settings inherited from host via mounted ~/.claude.json skip setup wizard
tmux send-keys -t "$TMUX_SESSION" "cd /workspace && claude --dangerously-skip-permissions" Enter

exec ttyd \
    --port 7681 \
    --writable \
    tmux attach-session -t "$TMUX_SESSION"
