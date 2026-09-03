# Watcher - Native Docker Compose GitOps Engine

## Overview

**Watcher** is a lightweight, GitOps tool that automates and observes deployments for Docker Compose environments. It monitors a Git repository for changes and intelligently reconciles the desired state directly with the Docker Engine API.

## Motivation

While using Docker Compose, we wanted a tag-based container release workflow—every new image tag should trigger a deployment.

We explored tools like **Watchtower**, but found it wasn't truly GitOps-oriented. It focuses on tracking image digests/updates rather than managing the entire deployment lifecycle via version control, making it unsuitable for strict tag-driven release flows.

Our initial solution was to deploy via CI by copying `docker-compose` files to the server over SSH and running `docker compose up` manually. This approach quickly revealed significant limitations:

- **Security Risk**: Deployment servers had to be exposed (via SSH) to CI runners.
- **Secrets Management**: Private SSH keys had to be stored in CI variables.
- **Operational Burden**: Running self-hosted runners or CI agents on deployment servers wasted resources and added maintenance overhead.

**A deployment server should focus on running applications, not executing CI jobs.**

These challenges led to the creation of **Watcher**: a pull-based GitOps controller that runs _inside_ your environment, eliminating the need for incoming SSH connections or external CI agents.

## API

For advanced integration, Watcher exposes a clean REST and SSE API.

| Endpoint                      | Type | Purpose                                          |
| :---------------------------- | :--- | :----------------------------------------------- |
| `GET /api/deployment/history` | REST | List of past deployments and statuses.           |
| `GET /api/deployment/current` | REST | Returns the last successful (stable) deployment. |

## Configuration

Watcher is configured via a `config.yaml` file.

### `config.yaml` Parameters

- `repoURL` (string): SSH URL of the Git repository.
- `deploymentDir` (string): Path where the repo is cloned inside the container.
- `composeFile` (string): Name of the target file (e.g., `docker-compose.yaml`).
- `targetBranch` (string): Branch to monitor (e.g., `main`).
- `checkInterval` (integer): Seconds between checks (e.g., `30`).
- `stateLocation` (string): Directory path for the BoltDB state file (e.g., `/etc/watcher`). Watcher will create `watcher.db` inside this directory.
- `sshKeyPath` (string, optional): Path to the SSH private key (if not using SSH Agent).
- `endpoint` (string, optional): To configure the rest endpoint for deployment data (default 0.0.0.0:7777)

## Git Authentication

Watcher supports two methods for authenticating with your Git repository. To prevent Man-in-the-Middle attacks, Watcher strictly verifies the Git server's identity using a `known_hosts` file.

### 1. SSH Agent (Recommended)

This is the **safest method** as it avoids mounting private key files directly. Watcher automatically detects the `SSH_AUTH_SOCK` environment variable.

**Setup Instructions:**

1. Ensure your SSH agent is running and your key is added:
   ```bash
   eval "$(ssh-agent -s)"
   ssh-add ~/.ssh/id_rsa
   ```

### 2. Private Key File

If you cannot use an SSH Agent, you can mount a private key file and specify `sshKeyPath` in `config.yaml`.

```yaml
# config.yaml
sshKeyPath: /home/appuser/.ssh/id_rsa
```

## Running with Docker

Comming soon
