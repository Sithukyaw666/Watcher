# Watcher

**Lightweight, pull-based GitOps engine for Docker Compose environments.**

Watcher monitors a Git repository for changes and intelligently reconciles the desired state directly with the Docker Engine API — no CI agents, no SSH exposure, no overhead.

---


## Features

- **Automatic rollback** — reverts to the last stable state if a deployment fails health checks
- **Circuit breaker** — skips commits that previously failed, avoiding retry loops
- **Dependency-aware** — respects `depends_on` and waits for `healthcheck` before proceeding
- **Live metrics and logs** — CPU, memory, and container logs streamed to the browser
- **Web dashboard** — built-in UI at `http://localhost:8080`

---


## Quick Start

```bash
# 1. Generate a known_hosts file for your Git provider
ssh-keyscan github.com > known_hosts

# 2. Create config.yaml (see Configuration)

# 3. Run with Docker Compose
docker compose up -d
```

See [Getting Started](getting-started.md) for the full guide.
