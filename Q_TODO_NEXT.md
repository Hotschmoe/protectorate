# TODO: Next Session Topics

## 1. CLI vs WebAPI + jq / Envoy Container vs Native Binary

### Current State
- Envoy runs inside a Docker container
- Doctor (and other features) are webui-only
- API exists at `/api/doctor` but no CLI wrapper

### Questions to Discuss

**Should we add `envoy --doctor` CLI?**
- Currently users would need: `docker exec envoy-poe envoy --doctor`
- Or use the API: `curl -s localhost:7470/api/doctor | jq`
- Is CLI convenience worth the added code?

**Should envoy run on host instead of in container?**

Arguments for native binary on host:
- Direct access to Docker socket (no socket mounting)
- CLI tools like `envoy --doctor` work naturally
- Simpler debugging
- Can manage SSH agent, git config directly
- No container-in-container complexity

Arguments for containerized envoy:
- Consistent environment across platforms
- Single `docker compose up` to run everything
- No Go build required on user's machine
- Isolation from host system
- Easier updates (just pull new image)

**Hybrid approach?**
- Thin CLI wrapper on host that calls containerized envoy API
- Best of both worlds but more moving parts

### Decision Needed
This affects architecture significantly. Worth dedicated discussion.

---

## 2. Consolidate .env.example and Install Script

### Problem
Git identity defaults are hardcoded in two places:
- `.env.example` - has `GIT_COMMITTER_NAME=Protectorate`
- `install.sh` create_env() - also sets these values

This is a maintenance burden and source of drift.

### Options

**Option A: Single source in .env.example (Preferred)**
- Change `.env.example` to use `${HOME}/protectorate/workspaces` instead of `${PWD}`
- Install script just downloads and copies: `curl ... .env.example -o .env`
- Pros: Single source of truth, less code
- Cons: .env.example becomes less "example-y", more "production default"
- Tradeoff is acceptable since Doctor guides customization

**Option B: Download + sed replacements**
- Install downloads .env.example
- Uses sed to replace ${PWD} with actual path
- Pros: .env.example stays generic
- Cons: Fragile sed scripts, more complexity

**Option C: Keep separate (current state)**
- Maintain both files independently
- Pros: None
- Cons: Duplication, drift, maintenance burden

### Recommendation
Implement Option A. The .env.example file becomes the canonical default configuration. Users who clone the repo for development can still use it directly. Install script just copies it.

### Implementation Steps
1. Update `.env.example`:
   - Change `WORKSPACE_HOST_ROOT=${PWD}/workspaces` to `${HOME}/protectorate/workspaces`
   - Ensure all defaults are production-ready
2. Update `install.sh`:
   - Remove `create_env()` function body
   - Replace with: `curl -fsSL "$RAW_GITHUB/.env.example" -o .env`
3. Test fresh install
