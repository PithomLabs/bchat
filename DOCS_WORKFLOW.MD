# Bchat Unified Workflow Guide (Development, Testing & Deployment)

This document outlines the standard, unified workflow for developing, containerizing, testing, and deploying Bchat with **RAG support enabled by default**. It addresses critical strategy gaps and provides a step-by-step pipeline from local code to Fly.io production.

---

## 1. Deep-Dive: Strategy Gaps Identified & Resolved

Our audit of your three listed commands (`task build:all`, `sudo docker build -f Dockerfile.fly -t bchat:rag .`, and `fly deploy`) uncovered **three critical gaps** that must be bridged for RAG to work reliably:

### Gap A: Dockerfile Configuration Drift (Critical)
* **The Disparity:** 
  * Your local command builds using `Dockerfile.fly` (`LANCEDB_STORAGE_PROVIDER = 's3'`).
  * Your production command `fly deploy` uses `fly.toml`, which points to `Dockerfile.local.fly` (`LANCEDB_STORAGE_PROVIDER = 'local'`).
* **The Risk:** You are testing an S3-based configuration locally inside Docker, but deploying a Local Volume-based configuration to Fly.io. This breaks development-to-production parity.
* **The Unification:** **Unify on `Dockerfile.local.fly` for both environments**. Use the local volume variant everywhere to ensure identical behaviors.

### Gap B: Native CGO Library Context Dependency
* **The Disparity:** Both `Dockerfile.local.fly` and `Dockerfile.fly` are **multi-stage builds**. They download npm packages, build frontend/widget bundles, and compile the Go binary **inside the container**. Therefore, the host-compiled outputs of `task build:all` are ignored.
* **The True Dependency:** The only host files the Docker builds actually depend on are the **native LanceDB CGO libraries** (`lib/linux_amd64/` and `include/`). Because these libraries are not in Git (~542MB), they must exist locally on the host before running any docker/deploy command.
* **The Unification:** Instead of running the slow host-compiling `task build:all`, you can run a much faster `task setup:lancedb` locally. This guarantees the native shared libraries are present for the Docker build context.

### Gap C: Local Container Runtime Parity
* **The Disparity:** If you run the locally-built image `bchat:rag` without mounting a local volume or passing secrets, the container will fall back to `NoOpVectorDB` because it lacks a database and OpenRouter keys.
* **The Unification:** Run the local container using `--env-file` and a local volume mount to guarantee identical parities with the remote Fly volume.

---

## 2. The Unified RAG Workflow

Follow this step-by-step pipeline to develop, test, and deploy Bchat with CGO LanceDB RAG support.

```
┌────────────────────────┐      ┌────────────────────────┐      ┌────────────────────────┐
│   1. LOCAL HOST DEV    │     │   2. LOCAL DOCKER TEST │     │   3. FLY.IO DEPLOYMENT │
├────────────────────────┤      ├────────────────────────┤      ├────────────────────────┤
│ • task setup:lancedb   │ ───▶ │ • docker build         │ ───▶ │ • task fly:check       │
│ • task run:rag         │      │ • docker run (w/ vol)  │      │ • fly deploy           │
└────────────────────────┘      └────────────────────────┘      └────────────────────────┘
```

### Phase 1: Local Host Development
Develop and test natively on your Linux host. This is fast and leverages the pre-populated local development database `memos_dev.db`.

```bash
# 1. Download native LanceDB libraries (run once on new checkout)
task setup:lancedb

# 2. Build backend natively with CGO and run Bchat in dev mode
task run:rag
```

---

### Phase 2: Local Docker Container Testing
Containerize your code locally using the same volume-based configuration that runs in production to ensure perfect parity.

```bash
# 1. Build the local Docker image using the Local Volume Dockerfile
sudo docker build -f Dockerfile.local.fly -t bchat:rag .

# 2. Create a local persistent volume directory (mirrors the Fly volume)
mkdir -p build/docker_memos_data

# 3. Run the container with local volumes and host secrets
sudo docker run -d \
  --name bchat_test \
  -p 5230:5230 \
  --env-file .env \
  -v $(pwd)/build/docker_memos_data:/var/opt/memos \
  bchat:rag

# 4. Stream RAG container logs
sudo docker logs -f bchat_test | grep -E "RAG|LanceDB|Embedding|vector database|Reindex"
```

---

### Phase 3: Fly.io Production Deployment
Deploy safely to Fly.io, knowing that the environment, code, and container structures are identical.

```bash
# 1. Run pre-deployment pipeline checks (verifies .env -> Dockerfile -> fly.toml parity)
task fly:check

# 2. Deploy to production
fly deploy

# 3. Stream live remote logs to watch the auto-bootstrap indexer (uses new Taskfile utility)
task fly:logs:rag
```

---

## 3. Reference Table of Commands

| Context | Action | Command |
|---------|--------|---------|
| **Setup** | Download CGO libraries | `task setup:lancedb` |
| **Local Host** | Run natively with RAG | `task run:rag` |
| **Local Host** | Rebuild all local assets | `task build:all` |
| **Local Docker** | Build image | `sudo docker build -f Dockerfile.local.fly -t bchat:rag .` |
| **Local Docker** | Run container | `sudo docker run --env-file .env -v $(pwd)/build/docker_memos_data:/var/opt/memos -p 5230:5230 bchat:rag` |
| **Deployment** | Parity Chain Check | `task fly:check` |
| **Deployment** | Push to Production | `fly deploy` |
| **Diagnostics** | Watch Remote RAG Init | `task fly:logs:rag` |
| **Diagnostics** | Production DB Shell | `task fly:ssh:db` |
