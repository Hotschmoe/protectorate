# Stage 1: Build sleeve-agent
FROM golang:1.24-bookworm AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the sleeve-agent binary
RUN CGO_ENABLED=0 GOOS=linux go build -o sleeve-agent ./cmd/sleeve-agent

# Stage 2: Final image
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    git \
    tmux \
    wget \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

RUN mkdir -p /root/.claude /workspace

# Copy sleeve-agent from builder
COPY --from=builder /build/sleeve-agent /usr/local/bin/sleeve-agent
RUN chmod +x /usr/local/bin/sleeve-agent

# Copy entrypoint (assumes build context is root)
COPY containers/sleeve/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace

EXPOSE 7681

ENTRYPOINT ["/entrypoint.sh"]
