# WatcherCD - Docker Compose CI/CD Automation

## Overview

**WatcherCD** is a simple CD tool that automates CI/CD pipelines for Docker Compose environments. It is designed to fill the gap where Docker Compose lacks advanced CI/CD tools like ArgoCD. While tools like **Watchtower** exist for monitoring Docker container images, Watchtower only monitors static tag image tags from the registry. **WatcherCD**, on the other hand, checks for changes in the GitHub repository for Docker Compose YAML file updates. When a change is detected, it pulls the updated files and deploys the changes using Docker Compose.

## Features

- **Monitor GitHub Repositories**: Watch for changes in the Docker Compose YAML file within a specified GitHub repository.
- **Automated Deployment**: When changes are detected, WatcherCD automatically pulls the updates and deploys them using Docker Compose.
- **Configurable Check Intervals**: Set the interval at which WatcherCD checks for changes in the repository.
- **SSH Key Authentication**: Securely access private GitHub repositories using SSH keys.

## Requirements

- **Docker Compose**: WatcherCD interacts with Docker Compose to deploy the services.

## Installation

comming soon

## Configuration

You can configure WatcherCD by editing the `config.yaml` file. Here’s an example configuration:

```yaml
repoURL: git@github.com:<repository>.git
deploymentDir: /path/to/your/directory
composeFile: docker-compose.yaml
targetBranch: main
sshKeyPath: /path/to/your/private/key
checkInterval: 30
```

### Configuration Fields

- **repoURL**: The SSH URL to your GitHub repository where your Docker Compose YAML file is stored.
- **deploymentDir**: The directory to store the docker compose file locally.
- **composeFile**: The name of the Docker Compose YAML file to be deployed (typically `docker-compose.yaml`).
- **targetBranch**: The branch to monitor for changes in the repository.
- **sshKeyPath**: The path to your private SSH key used to authenticate with GitHub (useful for private repositories).
- **checkInterval**: The frequency (in seconds) at which WatcherCD will check for changes in the repository.

## How It Works

1.  WatcherCD checks for changes in the specified GitHub repository at the set interval.
2.  If a change in the Docker Compose YAML file is detected, WatcherCD pulls the latest changes from the repository.
3.  After pulling the latest changes, WatcherCD deploys the updated configuration.
