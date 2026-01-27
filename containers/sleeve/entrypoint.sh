#!/bin/bash
set -e

chown -R claude:claude /home/claude/workspace
chown -R claude:claude /home/claude/.claude 2>/dev/null || true

if [ -f /etc/claude/settings.json ]; then
    cp /etc/claude/settings.json /home/claude/.claude/settings.json
    chown claude:claude /home/claude/.claude/settings.json
fi

SOCKET_DIR="/home/claude/.abduco"
SOCKET_PATH="${SOCKET_DIR}/claude.sock"

mkdir -p "$SOCKET_DIR"
chown claude:claude "$SOCKET_DIR"

cat > /usr/local/bin/claude-session.sh << 'SCRIPT'
#!/bin/bash
cd /home/claude/workspace
exec claude --dangerously-skip-permissions
SCRIPT
chmod +x /usr/local/bin/claude-session.sh

su - claude -c "abduco -e '^]' -c $SOCKET_PATH /usr/local/bin/claude-session.sh" &

exec sleep infinity
