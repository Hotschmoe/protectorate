.PHONY: build build-base build-sleeve build-envoy up down clean clean-all help

# Default target
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

# Build the envoy image (uses base for runtime)
build-envoy:
	DOCKER_BUILDKIT=1 docker build \
		--provenance=false \
		-t protectorate/envoy:latest \
		-f containers/envoy/Dockerfile \
		.

# Build everything including base (for CI or fresh setup)
build-all: build-base build

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

help:
	@echo "Protectorate Build Targets"
	@echo ""
	@echo "  make build-base   Build shared base image (slow, ~2 min, run once)"
	@echo "  make build-sleeve Build sleeve image (fast, ~3 sec)"
	@echo "  make build-envoy  Build envoy image (fast, ~10 sec)"
	@echo "  make build        Build envoy + sleeve (requires base exists)"
	@echo "  make build-all    Build everything including base"
	@echo ""
	@echo "  make up           Start services via docker-compose"
	@echo "  make down         Stop services"
	@echo "  make clean        Remove all containers and networks"
	@echo "  make clean-all    Remove containers, networks, and images"
