#!/bin/bash
set -e

chown -R claude:claude /home/claude/workspace
chown -R claude:claude /home/claude/.claude 2>/dev/null || true

if [ -f /etc/claude/settings.json ]; then
    cp /etc/claude/settings.json /home/claude/.claude/settings.json
    chown claude:claude /home/claude/.claude/settings.json
fi

SOCKET_DIR="/home/claude/.dtach"
SOCKET_PATH="${SOCKET_DIR}/session.sock"

mkdir -p "$SOCKET_DIR"
chown claude:claude "$SOCKET_DIR"

# Session script: bash shell (envoy can send commands to start claude)
cat > /usr/local/bin/sleeve-session.sh << 'SCRIPT'
#!/bin/bash
export TERM=xterm-256color
export PATH="$HOME/.local/bin:$PATH"
cd /home/claude/workspace

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
chmod +x /usr/local/bin/sleeve-session.sh

# dtach: -n creates daemon session (no attach), -z disables suspend
su - claude -c "dtach -n $SOCKET_PATH -z /usr/local/bin/sleeve-session.sh"

exec sleep infinity
