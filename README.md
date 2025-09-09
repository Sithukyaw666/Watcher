# Watcher - Native Docker Compose GitOps Engine

## Overview

**Watcher** is a lightweight GitOps tool that automates deployments for Docker Compose environments. It monitors a Git repository for changes and, upon detection, intelligently applies the desired state by communicating directly with the Docker Engine API.

Unlike tools that simply re-run `docker compose up`, Watcher parses the compose file natively in Go. It understands the relationships between services, networks, and volumes, making it a more efficient and integrated solution for continuous deployment.

## Core Features

- **Native Go Implementation**: Directly interacts with the Docker Engine API for efficient and precise control over containers, networks, and volumes.
- **Dependency-Aware Deployments**: Understands `depends_on` relationships between services to ensure they are started in the correct topological order.
- **Healthcheck-Aware Startup**: Waits for services with a defined `healthcheck` to become healthy before starting any services that depend on them. This prevents cascading failures in multi-service applications.
- **Intelligent Updates**: Detects changes to image tags and automatically re-creates services to deploy new versions, leaving unchanged services untouched.
- **Orphan Pruning**: Automatically detects and removes services that are running but are no longer defined in the compose file.

## How It Works

1.  **Monitor Git**: Clones a Git repository and monitors a specific branch for new commits.
2.  **Parse Compose File**: When a change is detected, it natively parses the `docker-compose.yaml` file.
3.  **Build Dependency Graph**: Analyzes `depends_on` relationships to determine the correct startup order.
4.  **Reconcile State**: Communicates directly with the Docker Engine API to:
    - Create or update services in the correct order.
    - Wait for dependencies to pass their health checks before starting dependent services.
    - Re-create services if their image has changed.
    - Remove orphaned services no longer in the compose file.

## Configuration

Watcher is configured via a `config.yaml` file mounted into the container.

### `config.yaml` Parameters

- `repoURL` (string, required): The SSH URL of the Git repository to monitor (e.g., `git@github.com:your-user/your-repo.git`).
- `deploymentDir` (string, required): The path _inside the container_ where the repository will be cloned (e.g., `/home/appuser/deployment`).
- `composeFile` (string, required): The name of the compose file within the repository to apply (e.g., `docker-compose.yaml`).
- `targetBranch` (string, required): The branch to monitor for new commits.
- `checkInterval` (integer, required): The frequency in seconds at which to check for new commits.
- `sshKeyPath` (string, optional): The path _inside the container_ to an SSH private key. This is used for authentication if an SSH Agent is not available. See the Authentication section below.

### Authentication

Watcher supports two methods for authenticating with your Git repository and will prioritize the SSH Agent if it is available.

#### 1. SSH Agent (Recommended)

Watcher automatically detects the `SSH_AUTH_SOCK` environment variable inside the container. If found, it will attempt to authenticate using the forwarded SSH agent. This is the most secure method as it avoids mounting private key files into the container.

#### 2. Private Key File

If an SSH agent is not detected or if agent authentication fails, Watcher will use the private key specified by the `sshKeyPath` parameter in the `config.yaml` file.

## Running with Docker

Watcher is designed to be run as a container. Below is a reference `docker-compose.yaml` demonstrating a complete configuration.

**`docker-compose.yaml` Reference:**

```yaml
services:
  watcher:
    image: sithukyaw666/watcher:v0.0.1
    container_name: my-watcher
    user: "1000:988" # user id and docker group id
    restart: unless-stopped
    environment:
      - SSH_KNOWN_HOSTS=/home/appuser/.ssh/known_hosts
      - SSH_AUTH_SOCK=${SSH_AUTH_SOCK}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./config.yaml:/home/appuser/config.yaml:ro
      - ${SSH_AUTH_SOCK}:${SSH_AUTH_SOCK}
      - ${HOME}/.ssh/id_rsa:/home/appuser/.ssh/id_rsa:ro
      - ${HOME}/deployment:/home/appuser/deployment
```
