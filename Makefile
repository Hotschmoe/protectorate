.PHONY: build build-base build-sleeve build-envoy clean help

# Default target
build: build-envoy build-sleeve

# Build the sleeve base image (slow, run once)
build-base:
	DOCKER_BUILDKIT=1 docker build \
		--provenance=false \
		-t protectorate/sleeve-base:latest \
		-f containers/sleeve-base/Dockerfile \
		containers/sleeve-base/

# Build the sleeve image (fast, uses base)
build-sleeve:
	DOCKER_BUILDKIT=1 docker build \
		--provenance=false \
		-t protectorate/sleeve:latest \
		-f containers/sleeve/Dockerfile \
		containers/sleeve/

# Build the envoy image
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

# Show build times
time-base:
	time $(MAKE) build-base

time-sleeve:
	time $(MAKE) build-sleeve

help:
	@echo "Protectorate Build Targets"
	@echo ""
	@echo "  make build-base   Build sleeve-base image (slow, ~2 min, run once)"
	@echo "  make build-sleeve Build sleeve image (fast, ~3 sec)"
	@echo "  make build-envoy  Build envoy image (fast, ~5 sec)"
	@echo "  make build        Build envoy + sleeve (requires base exists)"
	@echo "  make build-all    Build everything including base"
	@echo ""
	@echo "  make up           Start services via docker-compose"
	@echo "  make down         Stop services"
