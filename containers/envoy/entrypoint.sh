#!/bin/bash
set -e

# Fix ownership of mounted volumes
chown -R claude:claude /workspace 2>/dev/null || true
chown -R claude:claude /home/claude/.claude 2>/dev/null || true

# Copy read-only mounted settings to writable location
if [ -f /etc/claude/settings.json ]; then
    cp /etc/claude/settings.json /home/claude/.claude/settings.json
    chown claude:claude /home/claude/.claude/settings.json
fi

# Run envoy (needs Docker socket access, runs as root)
exec envoy --config /etc/envoy/envoy.yaml
