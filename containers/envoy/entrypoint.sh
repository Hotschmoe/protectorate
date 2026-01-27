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

SOCKET_DIR="/home/claude/.abduco"
SOCKET_PATH="${SOCKET_DIR}/envoy.sock"

mkdir -p "$SOCKET_DIR"
chown claude:claude "$SOCKET_DIR"

cat > /usr/local/bin/envoy-session.sh << 'SCRIPT'
#!/bin/bash
cd /home/claude/workspaces
exec bash
SCRIPT
chmod +x /usr/local/bin/envoy-session.sh

su - claude -c "abduco -e '^]' -c $SOCKET_PATH /usr/local/bin/envoy-session.sh" &

exec envoy
