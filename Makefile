.PHONY: build build-base build-sleeve build-envoy build-envoy-release up down clean clean-all help

# Default target (dev build)
build: build-envoy build-sleeve

# Build the shared base image (slow, run once)
build-base:
	DOCKER_BUILDKIT=1 docker build \
		--provenance=false \
		-t protectorate/base:latest \
		-f containers/base/Dockerfile \
		containers/base/

# Build the sleeve image (fast, uses base)
build-sleeve:
	DOCKER_BUILDKIT=1 docker build \
		--provenance=false \
		-t protectorate/sleeve:latest \
		-f containers/sleeve/Dockerfile \
		containers/sleeve/

# Build envoy for dev (fast: local Go build + copy binary)
build-envoy: bin/envoy
	DOCKER_BUILDKIT=1 docker build \
		--provenance=false \
		-t protectorate/envoy:latest \
		-f containers/envoy/Dockerfile.dev \
		.

# Build Go binary locally
bin/envoy: $(shell find . -name '*.go' -type f)
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -o bin/envoy ./cmd/envoy

# Build envoy for release (slow: multi-stage, self-contained)
build-envoy-release:
	DOCKER_BUILDKIT=1 docker build \
		--provenance=false \
		-t protectorate/envoy:latest \
		-f containers/envoy/Dockerfile \
		.

# Build everything including base (for CI or fresh setup)
build-all: build-base build-envoy-release build-sleeve

# Run docker-compose
up:
	docker compose up -d

down:
	docker compose down

# Remove all protectorate containers and networks
clean:
	@echo "Stopping and removing all sleeve containers..."
	@docker ps -aq --filter "name=sleeve-" | xargs -r docker rm -f 2>/dev/null || true
	@echo "Stopping envoy..."
	@docker compose down 2>/dev/null || true
	@echo "Removing raven network..."
	@docker network rm raven 2>/dev/null || true
	@echo "Clean complete."

# Nuclear option: clean + remove all protectorate images
clean-all: clean
	@echo "Removing protectorate images..."
	@docker rmi protectorate/sleeve:latest 2>/dev/null || true
	@docker rmi protectorate/base:latest 2>/dev/null || true
	@docker rmi protectorate/envoy:latest 2>/dev/null || true
	@echo "Clean-all complete."

# Show build times
time-base:
	time $(MAKE) build-base

time-sleeve:
	time $(MAKE) build-sleeve

time-envoy:
	time $(MAKE) build-envoy

# =============================================================================
# Development Targets (fast iteration with volume mounts)
# =============================================================================

.PHONY: dev dev-down dev-logs dev-restart dev-rebuild watch

# Start development environment with volume-mounted binary and hot-reload webui
dev: bin/envoy
	@echo "Starting dev environment..."
	@echo "  - Binary mounted from ./bin/envoy"
	@echo "  - Webui hot-reload enabled (just refresh browser)"
	@echo "  - API available at http://localhost:7470"
	@echo ""
	docker compose -f docker-compose.dev.yaml up -d
	@echo ""
	@echo "Dev environment ready!"
	@echo "  Webui: http://localhost:7470"
	@echo "  Logs:  make dev-logs"
	@echo ""
	@echo "Workflow:"
	@echo "  Webui changes: Edit HTML -> Refresh browser (instant)"
	@echo "  Go changes:    make dev-restart (~5 sec)"

# Stop development environment
dev-down:
	docker compose -f docker-compose.dev.yaml down

# View development logs
dev-logs:
	docker compose -f docker-compose.dev.yaml logs -f

# Restart envoy process after Go code changes
dev-restart: bin/envoy
	docker compose -f docker-compose.dev.yaml restart envoy
	@echo "Envoy restarted with new binary"

# Full rebuild and restart (rarely needed)
dev-rebuild: bin/envoy
	docker compose -f docker-compose.dev.yaml up -d --force-recreate
	@echo "Dev environment rebuilt"

# Auto-rebuild on Go file changes (Phase 2 - requires inotify-tools)
watch:
	@chmod +x ./scripts/watch.sh
	@./scripts/watch.sh

# =============================================================================
# Production/Release Targets
# =============================================================================

.PHONY: release

# Build production containers (full multi-stage builds)
release: build-all
	@echo "Production build complete"

# =============================================================================
# Help
# =============================================================================

help:
	@echo "Protectorate Build Targets"
	@echo ""
	@echo "Development (fast iteration):"
	@echo "  make dev                 Start dev environment (volume-mounted)"
	@echo "  make dev-restart         Rebuild Go binary and restart (~5 sec)"
	@echo "  make dev-logs            View dev container logs"
	@echo "  make dev-down            Stop dev environment"
	@echo "  make watch               Auto-rebuild on file changes (requires inotify-tools)"
	@echo ""
	@echo "Container Builds:"
	@echo "  make build-base          Build shared base image (slow, ~2 min, run once)"
	@echo "  make build-sleeve        Build sleeve image (fast, ~3 sec)"
	@echo "  make build-envoy         Build envoy for dev (fast, ~3 sec, local Go)"
	@echo "  make build-envoy-release Build envoy for release (slow, multi-stage)"
	@echo "  make build               Build envoy + sleeve for dev"
	@echo "  make build-all           Build everything for release (includes base)"
	@echo ""
	@echo "Production:"
	@echo "  make release             Full production build"
	@echo "  make up                  Start services via docker-compose"
	@echo "  make down                Stop services"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean               Remove all containers and networks"
	@echo "  make clean-all           Remove containers, networks, and images"
