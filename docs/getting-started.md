# Getting Started

## Prerequisites

- Docker with `/var/run/docker.sock` accessible
- SSH access to your Git repository
- A `known_hosts` file for your Git provider

```bash
ssh-keyscan github.com > known_hosts
```

---

## Configuration

Watcher reads from a `config.yaml` file:

```yaml
repoURL: git@github.com:your-org/your-repo.git
deploymentDir: /app/deployment
composeFile: docker-compose.yaml
targetBranch: main
checkInterval: 30
stateLocation: /etc/watcher
dockerAPIVersion: "1.48"

# Optional — only if not using SSH Agent
sshKeyPath: /home/appuser/.ssh/id_rsa
```

### Parameters

| Parameter | Type | Required | Description |
|---|---|---|---|
| `repoURL` | string | yes | SSH URL of the Git repository |
| `deploymentDir` | string | yes | Path inside the container where the repo is cloned |
| `composeFile` | string | yes | Compose file to reconcile |
| `targetBranch` | string | yes | Branch to watch |
| `checkInterval` | integer | yes | Seconds between poll cycles |
| `stateLocation` | string | yes | Directory for the BoltDB state file |
| `dockerAPIVersion` | string | no | Docker Engine API version |
| `sshKeyPath` | string | no | Path to SSH private key — omit if using SSH Agent |

---

## Authentication

Watcher authenticates over SSH and verifies the server's host key using `known_hosts`.

### SSH Agent (Recommended)

```bash
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_rsa
```

Then wire it into the Compose file:

```yaml
environment:
  - SSH_AUTH_SOCK=${SSH_AUTH_SOCK}
volumes:
  - ${SSH_AUTH_SOCK}:${SSH_AUTH_SOCK}
```

### Private Key File

Mount the key and set `sshKeyPath` in `config.yaml`:

```yaml
volumes:
  - /path/to/id_rsa:/home/appuser/.ssh/id_rsa:ro
```

```yaml
sshKeyPath: /home/appuser/.ssh/id_rsa
```

---

## Docker Compose

```yaml
services:
  watcher:
    image: sithukyaw666/watcher:0.1.1
    container_name: watcher
    restart: unless-stopped
    environment:
      - SSH_AUTH_SOCK=${SSH_AUTH_SOCK}
      - SSH_KNOWN_HOSTS=/home/appuser/.ssh/known_hosts
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./config.yaml:/home/appuser/config.yaml:ro
      - ${SSH_AUTH_SOCK}:${SSH_AUTH_SOCK}
      - ./known_hosts:/home/appuser/.ssh/known_hosts:ro
      - watcher_data:/etc/watcher/
      - ./deployment:/app/deployment
    ports:
      - "8080:8080"

volumes:
  watcher_data:
```

```bash
docker compose up -d
```

Dashboard available at `http://localhost:8080`.

---

## Build from Source

```bash
git clone https://github.com/Sithukyaw666/Watcher.git
cd Watcher
go build -o watcher .
./watcher
```
