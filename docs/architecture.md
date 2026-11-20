# Architecture

Watcher is a single Go binary that embeds the frontend and talks directly to the Docker Engine API. There are no external dependencies at runtime.

---

## Components

### Monitor (`internal/monitor`)

The reconciliation loop. Polls the Git remote over SSH on a configurable `checkInterval` and compares the latest commit SHA against the last known state. When a new commit is detected it hands off to the Controller.

### Controller (`operations/controller`)

Executes the deployment plan:

1. Pull the new commit.
2. Parse the target Compose file.
3. Determine which services changed.
4. Apply changes via the Docker Engine API (pull images, recreate containers).
5. Wait for `healthcheck` results before marking dependents as ready.

If any service fails health checks, the Controller triggers an automatic rollback.

### Circuit Breaker

Before deploying a commit, the Controller checks the Store for a previous failure record matching that SHA. If found, the commit is skipped. This prevents the infinite `failure → rollback → re-detect → failure` cycle.

### Store (`internal/store`)

Backed by [BoltDB](https://github.com/etcd-io/bbolt) — an embedded key/value store. Persists:

- Deployment history (commit, status, timestamp, author)
- Compose YAML snapshots (used for rollback and the `/api/history/view` endpoint)
- Failed commit registry (circuit breaker state)

### API Server (`internal/api`)

Serves the embedded frontend SPA, REST endpoints, and SSE streams — all from one Go HTTP server, no reverse proxy needed.

### Frontend (`web/`)

A Vite-built SPA embedded into the binary at compile time via Go's `embed` package.

---

## Rollback Flow

1. A new commit is deployed.
2. One or more services fail their health checks.
3. The Controller loads the last stable Compose snapshot from the Store.
4. The previous config is re-applied via the Docker Engine API.
5. The failed commit SHA is recorded in the circuit breaker registry.

The rollback uses the exact YAML snapshot that was active during the last successful deployment, so it is deterministic.

---

## Security Considerations

- **No inbound connections** — Watcher uses a pull model; your Git server and CI never need to reach into your network.
- **Host key verification** — All SSH connections verify the server identity against a `known_hosts` file.
- **Read-only mounts** — Config files and SSH keys should be mounted `:ro`.
- **Docker socket access** — Watcher requires `/var/run/docker.sock`. Treat this with the same care as root access to the host.
