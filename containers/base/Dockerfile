FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    git \
    tmux \
    && rm -rf /var/lib/apt/lists/*

ARG TTYD_VERSION=1.7.7
RUN curl -fsSL "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.x86_64" \
    -o /usr/local/bin/ttyd \
    && chmod +x /usr/local/bin/ttyd

ARG CSTACK_VERSION=1.0.1
RUN curl -fsSL "https://github.com/Hotschmoe/cstack/releases/download/v${CSTACK_VERSION}/cstack-v${CSTACK_VERSION}-linux_amd64.tar.gz" \
    -o /tmp/cstack.tar.gz \
    && tar -xzf /tmp/cstack.tar.gz -C /tmp \
    && mv /tmp/cstack /usr/local/bin/cs \
    && chmod +x /usr/local/bin/cs \
    && rm /tmp/cstack.tar.gz

RUN useradd -m -s /bin/bash -u 1000 claude \
    && mkdir -p /home/claude/.claude /etc/claude \
    && chown -R claude:claude /home/claude

# Enable tmux mouse support with proper scroll behavior
# - mouse on: enables mouse support
# - WheelUpPane binding: forces copy-mode on scroll instead of shell history
RUN printf '%s\n' \
    'set -g mouse on' \
    'bind-key -T root WheelUpPane if-shell -F -t = "#{alternate_on}" "send-keys -M" "select-pane -t =; copy-mode -e; send-keys -M"' \
    'bind-key -T root WheelDownPane if-shell -F -t = "#{alternate_on}" "send-keys -M" "select-pane -t =; send-keys -M"' \
    > /home/claude/.tmux.conf \
    && chown claude:claude /home/claude/.tmux.conf

USER claude
RUN curl -fsSL https://claude.ai/install.sh | bash \
    && echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
USER root

WORKDIR /home/claude
EXPOSE 7681
