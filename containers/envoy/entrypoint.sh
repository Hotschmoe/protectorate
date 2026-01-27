#!/bin/bash
set -e

chown -R claude:claude /home/claude/workspaces 2>/dev/null || true
chown -R claude:claude /home/claude/.claude 2>/dev/null || true
mkdir -p /app/web 2>/dev/null || true

if [ -S /ssh-agent ]; then
    mkdir -p /root/.ssh
    ssh-keyscan github.com gitlab.com >> /root/.ssh/known_hosts 2>/dev/null
fi

if [ -f /etc/claude/settings.json ]; then
    cp /etc/claude/settings.json /home/claude/.claude/settings.json
    chown claude:claude /home/claude/.claude/settings.json
fi

SOCKET_DIR="/home/claude/.dtach"
SOCKET_PATH="${SOCKET_DIR}/session.sock"

mkdir -p "$SOCKET_DIR"
chown claude:claude "$SOCKET_DIR"

# Session script with TERM and PATH set, plus restart loop
cat > /usr/local/bin/envoy-session.sh << 'SCRIPT'
#!/bin/bash
export TERM=xterm-256color
export PATH="$HOME/.local/bin:$PATH"
cd /home/claude/workspaces

while true; do
    bash --login
    exit_code=$?
    echo ""
    echo "[Shell exited with code $exit_code. Restarting in 1 second...]"
    echo "[Press Ctrl+\\ to detach from session]"
    echo ""
    sleep 1
done
SCRIPT
chmod +x /usr/local/bin/envoy-session.sh

# dtach: -n creates daemon session (no attach), -z disables suspend
su - claude -c "dtach -n $SOCKET_PATH -z /usr/local/bin/envoy-session.sh"

exec envoy
