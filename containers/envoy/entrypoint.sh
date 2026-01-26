#!/bin/bash
set -e

TMUX_SESSION="envoy"

# Fix ownership of mounted volumes
chown -R claude:claude /home/claude/workspaces 2>/dev/null || true
chown -R claude:claude /home/claude/.claude 2>/dev/null || true
mkdir -p /app/web 2>/dev/null || true

# SSH agent is mounted from host via SSH_AUTH_SOCK env var
# User must have ssh-agent running: eval $(ssh-agent) && ssh-add
if [ -S /ssh-agent ]; then
    mkdir -p /root/.ssh
    ssh-keyscan github.com gitlab.com >> /root/.ssh/known_hosts 2>/dev/null
fi

# Copy read-only mounted settings to writable location
if [ -f /etc/claude/settings.json ]; then
    cp /etc/claude/settings.json /home/claude/.claude/settings.json
    chown claude:claude /home/claude/.claude/settings.json
fi

# Create tmux session manager script that respawns on exit
cat > /usr/local/bin/tmux-session.sh << 'SCRIPT'
#!/bin/bash
SESSION="envoy"
while true; do
    if ! su - claude -c "tmux has-session -t $SESSION 2>/dev/null"; then
        su - claude -c "tmux new-session -d -s $SESSION"
    fi
    su - claude -c "tmux attach-session -t $SESSION"
    sleep 0.5
done
SCRIPT
chmod +x /usr/local/bin/tmux-session.sh

# Start initial tmux session
su - claude -c "tmux new-session -d -s $TMUX_SESSION"

# Start ttyd in background with respawning session
ttyd --port 7681 --writable /usr/local/bin/tmux-session.sh &

# Run envoy (needs Docker socket access, runs as root)
exec envoy
