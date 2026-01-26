#!/usr/bin/env bash
#
# Protectorate Installer
# https://github.com/hotschmoe/protectorate
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/hotschmoe/protectorate/master/install.sh | bash
#
# This script:
#   1. Checks/installs Docker
#   2. Checks/installs Claude CLI
#   3. Handles Claude authentication
#   4. Downloads docker-compose.yaml
#   5. Creates configuration
#   6. Pulls container images
#   7. Starts Envoy
#
set -e

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------

PROTECTORATE_DIR="$HOME/protectorate"
GHCR_REGISTRY="ghcr.io/hotschmoe"
RAW_GITHUB="https://raw.githubusercontent.com/hotschmoe/protectorate/master"

# Colors (if terminal supports them)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# -----------------------------------------------------------------------------
# Helper Functions
# -----------------------------------------------------------------------------

info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

success() {
    echo -e "${GREEN}[OK]${NC} $1" >&2
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" >&2
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

prompt() {
    echo ""
    echo -e "${YELLOW}$1${NC}"
    if [[ -t 0 ]]; then
        read -p "Press Enter to continue (Ctrl+C to abort)..."
    else
        read -p "Press Enter to continue (Ctrl+C to abort)..." </dev/tty 2>/dev/null || true
    fi
}


# -----------------------------------------------------------------------------
# Dependency Checks
# -----------------------------------------------------------------------------

check_docker() {
    info "Checking Docker..."

    if ! command -v docker &> /dev/null; then
        warn "Docker not found. Installing..."

        if [[ "$OSTYPE" == "linux-gnu"* ]]; then
            curl -fsSL https://get.docker.com | sh
            sudo usermod -aG docker "$USER"
            warn "Added $USER to docker group. You may need to log out and back in."
            warn "Trying to continue with sudo for now..."
            DOCKER_CMD="sudo docker"
        elif [[ "$OSTYPE" == "darwin"* ]]; then
            error "Please install Docker Desktop from https://docker.com/products/docker-desktop"
        else
            error "Unsupported OS: $OSTYPE"
        fi
    else
        DOCKER_CMD="docker"
    fi

    if ! $DOCKER_CMD info &> /dev/null; then
        if [[ "$OSTYPE" == "linux-gnu"* ]]; then
            warn "Docker not accessible. Trying with sudo..."
            DOCKER_CMD="sudo docker"
            if ! $DOCKER_CMD info &> /dev/null; then
                error "Docker installed but not running or accessible. Please start Docker and try again."
            fi
        else
            error "Docker installed but not running. Please start Docker and try again."
        fi
    fi

    success "Docker is available"
}

check_claude() {
    info "Checking Claude CLI..."

    if ! command -v claude &> /dev/null; then
        warn "Claude Code not found. Installing..."
        curl -fsSL https://claude.ai/install.sh | bash
        export PATH="$HOME/.local/bin:$PATH"
    fi

    if ! claude --version &> /dev/null; then
        export PATH="$HOME/.local/bin:$PATH"
        if ! claude --version &> /dev/null; then
            error "Claude CLI installed but not in PATH. Please restart your terminal and try again."
        fi
    fi

    success "Claude CLI is available ($(claude --version 2>/dev/null | head -1))"
}

# -----------------------------------------------------------------------------
# Authentication
# -----------------------------------------------------------------------------

check_claude_auth() {
    CREDS_FILE="$HOME/.claude/.credentials.json"
    if [[ -f "$CREDS_FILE" ]]; then
        if grep -q "accessToken" "$CREDS_FILE" 2>/dev/null; then
            return 0
        fi
    fi
    return 1
}

claude_login() {
    prompt "You need to log in to Claude Code. This will open a browser window."

    claude auth login

    if ! check_claude_auth; then
        error "Authentication failed. Please try again."
    fi

    success "Claude authentication complete"
}


# -----------------------------------------------------------------------------
# Setup
# -----------------------------------------------------------------------------

get_latest_version() {
    # Query GitHub API for tags and get the highest version number
    LATEST=$(curl -fsSL "https://api.github.com/repos/hotschmoe/protectorate/tags" 2>/dev/null | \
        grep '"name":' | \
        grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | \
        sort -V | \
        tail -1)

    if [[ -z "$LATEST" ]]; then
        LATEST="latest"
    fi

    echo "$LATEST"
}

setup_directory() {
    info "Setting up Protectorate directory..."

    mkdir -p "$PROTECTORATE_DIR/workspaces"
    cd "$PROTECTORATE_DIR"

    # Download docker-compose.yaml
    info "Downloading docker-compose.yaml..."
    curl -fsSL "$RAW_GITHUB/docker-compose.yaml" -o docker-compose.yaml

    success "Directory ready at $PROTECTORATE_DIR"
}

setup_onboarding() {
    info "Setting up Claude Code configuration..."

    CLAUDE_JSON="$HOME/.claude.json"

    if [[ -f "$CLAUDE_JSON" ]]; then
        if grep -q "hasCompletedOnboarding" "$CLAUDE_JSON"; then
            success "Onboarding flag already set"
            return 0
        fi

        if command -v jq &> /dev/null; then
            jq '.hasCompletedOnboarding = true' "$CLAUDE_JSON" > "$CLAUDE_JSON.tmp"
            mv "$CLAUDE_JSON.tmp" "$CLAUDE_JSON"
        else
            sed -i 's/}$/,"hasCompletedOnboarding":true}/' "$CLAUDE_JSON"
        fi
    else
        echo '{"hasCompletedOnboarding":true}' > "$CLAUDE_JSON"
    fi

    success "Claude Code configuration ready"
}

create_env() {
    local VERSION="$1"
    local ENV_FILE="$PROTECTORATE_DIR/.env"

    if [[ -f "$ENV_FILE" ]]; then
        info "Existing .env found. Backing up to .env.backup"
        cp "$ENV_FILE" "$ENV_FILE.backup"
    fi

    cat > "$ENV_FILE" << EOF
# Protectorate Configuration
# Generated by install.sh on $(date)
# Version: $VERSION

# Host paths
WORKSPACE_HOST_ROOT=$PROTECTORATE_DIR/workspaces
CREDENTIALS_HOST_PATH=\${HOME}/.claude/.credentials.json
SETTINGS_HOST_PATH=\${HOME}/.claude.json
PLUGINS_HOST_PATH=\${HOME}/.claude/plugins

# Docker settings
COMPOSE_PROJECT_NAME=protectorate

# Git identity (defaults - customize in Doctor tab or edit this file)
GIT_COMMITTER_NAME=Protectorate
GIT_COMMITTER_EMAIL=protectorate@local
GIT_CLONE_PROTOCOL=ssh
EOF

    success "Created $ENV_FILE"
}


# -----------------------------------------------------------------------------
# Container Setup
# -----------------------------------------------------------------------------

pull_images() {
    info "Pulling container images..."

    $DOCKER_CMD pull "$GHCR_REGISTRY/protectorate-envoy:latest" || {
        error "Could not pull envoy image"
    }

    $DOCKER_CMD pull "$GHCR_REGISTRY/protectorate-sleeve:latest" || {
        error "Could not pull sleeve image"
    }

    success "Container images pulled"
}

start_envoy() {
    info "Starting Envoy..."

    cd "$PROTECTORATE_DIR"

    # Stop existing containers (if updating)
    $DOCKER_CMD compose down 2>/dev/null || true

    # Start with docker compose
    $DOCKER_CMD compose up -d --force-recreate

    # Wait for health check
    info "Waiting for Envoy to be ready..."
    for i in {1..30}; do
        if curl -s http://localhost:7470/health > /dev/null 2>&1; then
            success "Envoy is running!"
            return 0
        fi
        sleep 1
    done

    warn "Envoy didn't respond to health check in time."
    warn "Check logs with: docker logs envoy-poe"
    return 1
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------

main() {
    echo ""
    echo "========================================"
    echo "  Protectorate Installer"
    echo "========================================"
    echo ""

    # 1. Check/install dependencies
    check_docker
    check_claude

    # 2. Handle authentication
    if ! check_claude_auth; then
        claude_login
    else
        success "Found existing Claude credentials"
    fi

    # 3. Get version and setup directory
    VERSION=$(get_latest_version)
    info "Installing version: $VERSION"
    setup_directory

    # 4. Setup Claude Code configuration
    setup_onboarding

    # 5. Create .env file
    create_env "$VERSION"

    # 6. Pull pre-built images
    pull_images

    # 7. Start Envoy
    start_envoy

    echo ""
    echo "========================================"
    echo "  Protectorate is ready!"
    echo "========================================"
    echo ""
    echo "WebUI:      http://localhost:7470"
    echo "Logs:       docker logs -f envoy-poe"
    echo "Stop:       cd ~/protectorate && docker compose down"
    echo "Update:     Re-run this install script"
    echo ""
    echo "Installed:  $PROTECTORATE_DIR"
    echo "Version:    $VERSION"
    echo ""
    echo "Next steps:"
    echo "  1. Open http://localhost:7470"
    echo "  2. Check the Doctor tab for system health"
    echo "  3. Clone a repo or spawn a sleeve"
    echo ""
}

main "$@"
