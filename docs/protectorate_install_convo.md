# Protectorate Installation Design

This document describes the one-line installer for Protectorate.

---

## Goal

```bash
curl -fsSL https://raw.githubusercontent.com/hotschmoe/protectorate/master/install.sh | bash
```

User runs one command, answers prompts, and ends up with:
- Docker installed (if needed)
- Claude Code installed and authenticated
- Protectorate repo cloned (latest release)
- Pre-built container images pulled from ghcr.io
- Envoy container running
- Ready to spawn sleeves

---

## Installation Flow

```
+------------------+
| Check Docker     |
| installed?       |
+--------+---------+
         |
    No   |   Yes
    v    |    |
+--------+    |
| Install     |
| Docker      |
+--------+----+
         |
         v
+------------------+
| Check Claude CLI |
| installed?       |
+--------+---------+
         |
    No   |   Yes
    v    |    |
+--------+    |
| Install     |
| Claude CLI  |
+--------+----+
         |
         v
+------------------+
| Check Claude     |
| authenticated?   |
+--------+---------+
         |
    No   |   Yes
    v    |    |
+--------+    |
| Run claude  |
| login flow  |
+--------+----+
         |
         v
+------------------+
| Generate long-   |
| lived token      |
| (setup-token)    |
+--------+---------+
         |
         v
+------------------+
| Get latest       |
| release tag      |
+--------+---------+
         |
         v
+------------------+
| Clone repo at    |
| release tag      |
+--------+---------+
         |
         v
+------------------+
| Setup onboarding |
| flag             |
+--------+---------+
         |
         v
+------------------+
| Create .env      |
| with token       |
+--------+---------+
         |
         v
+------------------+
| Pull pre-built   |
| images from      |
| ghcr.io          |
+--------+---------+
         |
         v
+------------------+
| Start Envoy      |
| (docker compose) |
+------------------+
         |
         v
    [Ready!]
```

---

## Release Flow

When a version is tagged, GitHub Actions builds and pushes images:

```
git tag v0.1.0 && git push --tags
         |
         v
GitHub Actions triggers
         |
         +---> Build ghcr.io/hotschmoe/protectorate-base:v0.1.0
         +---> Build ghcr.io/hotschmoe/protectorate-envoy:v0.1.0
         +---> Build ghcr.io/hotschmoe/protectorate-sleeve:v0.1.0
         +---> Tag all as :latest
```

Users pull pre-built images - no local building required.

---

## Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/hotschmoe/protectorate/master/uninstall.sh | bash
```

Removes:
- All sleeve containers
- Envoy container
- Raven network
- Optionally: container images
- Optionally: ~/protectorate directory

Does NOT remove:
- Docker (may be used by other apps)
- Claude CLI (may be used standalone)
- ~/.claude/ credentials

---

## Current Implementation

### install.sh

Located at repo root. Key functions:

| Function | Purpose |
|----------|---------|
| `check_docker` | Install Docker if missing, handle sudo/group issues |
| `check_claude` | Install Claude CLI if missing |
| `check_claude_auth` | Check for existing credentials |
| `claude_login` | Interactive OAuth login |
| `generate_token` | Generate long-lived OAuth token (1 year) |
| `get_latest_release` | Query GitHub API for latest release tag |
| `setup_repo` | Clone repo at specific tag |
| `setup_onboarding` | Set hasCompletedOnboarding flag |
| `create_env` | Generate .env file with token |
| `prompt_optional_vars` | Ask for optional API keys |
| `pull_images` | Pull pre-built images from ghcr.io |
| `start_envoy` | Start envoy via docker compose |

### uninstall.sh

Located at repo root. Stops containers, removes network, prompts for image/directory removal.

### .github/workflows/release.yaml

Triggered by version tags (v*). Builds and pushes:
- protectorate-base (internal dependency)
- protectorate-envoy
- protectorate-sleeve

### docker-compose.yaml

Production compose file. Uses pre-built ghcr.io images:
```yaml
image: ghcr.io/hotschmoe/protectorate-envoy:latest
```

### docker-compose.dev.yaml

Development compose file. Uses local base image + volume-mounted binary for fast iteration.

### configs/envoy.yaml

Envoy configuration. Uses env var with default for sleeve image:
```yaml
sleeve_image: ${SLEEVE_IMAGE:-ghcr.io/hotschmoe/protectorate-sleeve:latest}
```

### .env.example

Documents all environment variables.

---

## Decisions Made

| Question | Decision |
|----------|----------|
| Install location | Always `~/protectorate` |
| Version selection | Always latest release (queries GitHub API) |
| Image registry | ghcr.io/hotschmoe (public, no auth needed) |
| Compose files | Two: production (ghcr.io) and dev (local build) |
| Config source | envoy.yaml with env var overrides |
| Uninstall script | Yes, provided |

---

## Files

| File | Purpose | Status |
|------|---------|--------|
| `install.sh` | Main installer script | DONE |
| `uninstall.sh` | Uninstaller script | DONE |
| `.github/workflows/release.yaml` | GitHub Actions for image builds | DONE |
| `.env.example` | Environment variable documentation | DONE |
| `docker-compose.yaml` | Production compose (ghcr.io images) | DONE |
| `docker-compose.dev.yaml` | Development compose (local build) | DONE |
| `configs/envoy.yaml` | Envoy configuration | DONE |

---

## Future Improvements

See `docs/CONFIG_TODO.md` for full roadmap. Key items:

**Short-term:**
- [ ] `--version` flag to install specific version
- [ ] `--dry-run` flag
- [ ] Better error messages

**Medium-term:**
- [ ] macOS support
- [ ] WSL2 support
- [ ] `protectorate` CLI wrapper
- [ ] Systemd integration

**Long-term:**
- [ ] TUI installer (gum/whiptail)
- [ ] Custom domain (protectorate.dev/install.sh)
- [ ] Auto-updates

---

## Security Considerations

1. **Piping to bash:** Standard practice, users can review with `curl ... | less` first

2. **Token storage:** `.env` contains sensitive token
   - In `.gitignore`
   - User warned not to commit

3. **Docker group:** Adding user to docker group grants root-equivalent access
   - Necessary for container management
   - Alternative: rootless docker (more complex)

4. **Public images:** ghcr.io packages are public, no auth needed to pull
