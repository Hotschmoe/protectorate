FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    git \
    tmux \
    wget \
    && rm -rf /var/lib/apt/lists/*

ARG TTYD_VERSION=1.7.7
RUN wget -q "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.x86_64" \
    -O /usr/local/bin/ttyd \
    && chmod +x /usr/local/bin/ttyd

RUN useradd -m -s /bin/bash -u 1000 claude \
    && mkdir -p /home/claude/.claude /workspace /etc/claude \
    && chown -R claude:claude /home/claude /workspace

USER claude
RUN curl -fsSL https://claude.ai/install.sh | bash \
    && echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
USER root

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace

EXPOSE 7681

ENTRYPOINT ["/entrypoint.sh"]
