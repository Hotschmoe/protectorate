FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    git \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

ARG ABDUCO_VERSION=0.6
RUN curl -fsSL "https://github.com/martanne/abduco/archive/refs/tags/v${ABDUCO_VERSION}.tar.gz" \
    -o /tmp/abduco.tar.gz \
    && tar -xzf /tmp/abduco.tar.gz -C /tmp \
    && cd /tmp/abduco-${ABDUCO_VERSION} \
    && make \
    && mv abduco /usr/local/bin/ \
    && chmod +x /usr/local/bin/abduco \
    && rm -rf /tmp/abduco*

ARG CSTACK_VERSION=1.0.1
RUN curl -fsSL "https://github.com/Hotschmoe/cstack/releases/download/v${CSTACK_VERSION}/cstack-v${CSTACK_VERSION}-linux_amd64.tar.gz" \
    -o /tmp/cstack.tar.gz \
    && tar -xzf /tmp/cstack.tar.gz -C /tmp \
    && mv /tmp/cstack /usr/local/bin/cs \
    && chmod +x /usr/local/bin/cs \
    && rm /tmp/cstack.tar.gz

RUN useradd -m -s /bin/bash -u 1000 claude \
    && mkdir -p /home/claude/.claude /home/claude/.abduco /etc/claude \
    && chown -R claude:claude /home/claude

USER claude
RUN curl -fsSL https://claude.ai/install.sh | bash \
    && echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
USER root

WORKDIR /home/claude
