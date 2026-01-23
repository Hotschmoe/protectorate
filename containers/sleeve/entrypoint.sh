#!/bin/bash
set -e

TMUX_SESSION="main"

# Fix ownership of mounted volumes (runs as root)
chown -R claude:claude /workspace
chown -R claude:claude /home/claude/.claude 2>/dev/null || true

# Copy read-only mounted settings to writable location
if [ -f /etc/claude/settings.json ]; then
    cp /etc/claude/settings.json /home/claude/.claude.json
    chown claude:claude /home/claude/.claude.json
fi

# Create tmux session manager script that respawns on exit
cat > /usr/local/bin/tmux-session.sh << 'SCRIPT'
#!/bin/bash
SESSION="main"
FIRST_RUN=true
while true; do
    if ! su - claude -c "tmux has-session -t $SESSION 2>/dev/null"; then
        su - claude -c "tmux new-session -d -s $SESSION"
        if [ "$FIRST_RUN" = true ]; then
            su - claude -c "tmux send-keys -t $SESSION 'cd /workspace && claude --dangerously-skip-permissions' Enter"
            FIRST_RUN=false
        fi
    fi
    su - claude -c "tmux attach-session -t $SESSION"
    sleep 0.5
done
SCRIPT
chmod +x /usr/local/bin/tmux-session.sh

# Start initial tmux session with Claude
su - claude -c "tmux new-session -d -s $TMUX_SESSION"
su - claude -c "tmux send-keys -t $TMUX_SESSION 'cd /workspace && claude --dangerously-skip-permissions' Enter"

exec ttyd \
    --port 7681 \
    --writable \
    /usr/local/bin/tmux-session.sh
