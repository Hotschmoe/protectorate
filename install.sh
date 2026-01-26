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
#   4. Clones Protectorate repo (latest release)
#   5. Creates .env configuration
#   6. Pulls pre-built container images
#   7. Starts Envoy
#
set -e

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------

PROTECTORATE_DIR="$HOME/protectorate"
REPO_URL="https://github.com/hotschmoe/protectorate.git"
GHCR_REGISTRY="ghcr.io/hotschmoe"

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
    # Handle piped input - reopen stdin from tty if available
    if [[ -t 0 ]]; then
        read -p "Press Enter to continue (Ctrl+C to abort)..."
    else
        # When piped, try to read from tty
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

            # Try to use docker with sudo for this session
            DOCKER_CMD="sudo docker"
        elif [[ "$OSTYPE" == "darwin"* ]]; then
            error "Please install Docker Desktop from https://docker.com/products/docker-desktop"
        else
            error "Unsupported OS: $OSTYPE"
        fi
    else
        DOCKER_CMD="docker"
    fi

    # Verify docker works
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

    # Verify claude works
    if ! claude --version &> /dev/null; then
        # Try adding to PATH
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

get_latest_release() {
    info "Fetching latest release version..."

    # Query GitHub API for latest release
    LATEST=$(curl -fsSL "https://api.github.com/repos/hotschmoe/protectorate/releases/latest" 2>/dev/null | \
        grep '"tag_name"' | \
        sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')

    if [[ -z "$LATEST" ]]; then
        warn "Could not fetch latest release. Using master branch."
        LATEST="master"
    fi

    echo "$LATEST"
}

setup_repo() {
    local VERSION="$1"

    if [[ -d "$PROTECTORATE_DIR" ]]; then
        info "Protectorate directory exists. Updating..."
        cd "$PROTECTORATE_DIR"

        # Stash any local changes
        git stash --quiet 2>/dev/null || true

        # Fetch and checkout version
        git fetch --tags
        if [[ "$VERSION" == "master" ]]; then
            git checkout master
            git pull origin master
        else
            git checkout "$VERSION"
        fi
    else
        info "Cloning Protectorate ($VERSION)..."
        if [[ "$VERSION" == "master" ]]; then
            git clone "$REPO_URL" "$PROTECTORATE_DIR"
        else
            git clone --branch "$VERSION" "$REPO_URL" "$PROTECTORATE_DIR"
        fi
        cd "$PROTECTORATE_DIR"
    fi

    success "Repository ready at $PROTECTORATE_DIR"
}

setup_onboarding() {
    info "Setting up Claude Code configuration..."

    CLAUDE_JSON="$HOME/.claude.json"

    if [[ -f "$CLAUDE_JSON" ]]; then
        # Check if hasCompletedOnboarding is already set
        if grep -q "hasCompletedOnboarding" "$CLAUDE_JSON"; then
            success "Onboarding flag already set"
            return 0
        fi

        # Add hasCompletedOnboarding to existing config
        if command -v jq &> /dev/null; then
            jq '.hasCompletedOnboarding = true' "$CLAUDE_JSON" > "$CLAUDE_JSON.tmp"
            mv "$CLAUDE_JSON.tmp" "$CLAUDE_JSON"
        else
            # Fallback: insert before closing brace
            sed -i 's/}$/,"hasCompletedOnboarding":true}/' "$CLAUDE_JSON"
        fi
    else
        # Create minimal config
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

# Host paths (auto-detected)
WORKSPACE_HOST_ROOT=$PROTECTORATE_DIR/workspaces
CREDENTIALS_HOST_PATH=\${HOME}/.claude/.credentials.json
SETTINGS_HOST_PATH=\${HOME}/.claude.json
PLUGINS_HOST_PATH=\${HOME}/.claude/plugins

# Docker settings
COMPOSE_PROJECT_NAME=protectorate

# Optional: Long-lived OAuth token (run 'claude setup-token' to generate)
# CLAUDE_CODE_OAUTH_TOKEN=
EOF

    success "Created $ENV_FILE"
}


# -----------------------------------------------------------------------------
# Container Setup
# -----------------------------------------------------------------------------

pull_images() {
    info "Pulling container images..."

    # Always pull :latest since docker-compose.yaml uses :latest
    # The release workflow tags every release as both :vX.X.X and :latest
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

    # Create workspaces directory if it doesn't exist
    mkdir -p workspaces

    # Stop existing containers (if updating)
    $DOCKER_CMD compose down 2>/dev/null || true

    # Start with docker compose (--force-recreate ensures new images are used)
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

    # 3. Get latest release and setup repo
    VERSION=$(get_latest_release)
    info "Installing version: $VERSION"
    setup_repo "$VERSION"

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
    echo "Envoy API:  http://localhost:7470"
    echo "Logs:       docker logs -f envoy-poe"
    echo "Stop:       cd ~/protectorate && make down"
    echo ""
    echo "Installed:  $PROTECTORATE_DIR"
    echo "Version:    $VERSION"
    echo ""
    echo "Claude Code extensions:"
    echo "  Agents:   .claude/agents/"
    echo "  Skills:   .claude/skills/"
    echo ""
    echo "Next steps:"
    echo "  - Open http://localhost:7470 in your browser"
    echo "  - Spawn a sleeve from the web UI"
    echo ""
}

main "$@"
