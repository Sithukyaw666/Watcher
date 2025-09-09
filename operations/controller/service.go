package controller

import (
	"context"
	"fmt"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/sithukyaw666/watcher/utils"
	"io"
	"log/slog"
	"strings"
	"time"
)

// ReconcileServices handles the reconciliation of all services defined in the compose configuration
// against the actual running containers. It creates new services or updates existing ones as needed.
func ReconcileServices(ctx context.Context, cli *client.Client, projectName string, compose *Compose, actualState map[string]container.Summary, logger *slog.Logger) error {
	depMap := make(map[string][]string)
	for name, service := range compose.Services {
		depMap[name] = service.DependsOn
	}

	orderServices, err := utils.ResolveDependencyOrder(depMap)
	if err != nil {
		logger.Error("Failed to resolve service dependency order", "error", err)
		return err
	}
	logger.Info("Service reconciliation order", "order", orderServices)
	for _, serviceName := range orderServices {
		desiredService := compose.Services[serviceName]
		logger.Info("Reconciling service", "service_name", serviceName)

		for _, depName := range desiredService.DependsOn {
			depService := compose.Services[depName]
			depContainer, ok := actualState[depName]
			if !ok {
				logger.Error("Dependency container not found in actual state.", "service", serviceName, "dependency", depName)
				return fmt.Errorf("dependency '%s' for service '%s' not found", depName, serviceName)
			}
			if depService.HealthCheck != nil && len(depService.HealthCheck.Test) > 0 {
				logger.Info("Waiting for dependency to be healthy", "service", serviceName, "dependency", depName)
				if err := waitForHealthCheck(ctx, cli, depContainer.ID, logger); err != nil {
					logger.Error("Dependency failed health check", "service", serviceName, "dependency", depName, "error", err)
				}
			}
		}

		if actualContainer, ok := actualState[serviceName]; ok {
			logger.Info("Service exists. Checking for image updates...", "service_name", serviceName)
			if err := pullImage(ctx, cli, desiredService.Image, logger); err != nil {
				logger.Warn("Could not pull image. Skipping update check.", "image", desiredService.Image, "error", err)
				continue
			}
			desiredImg, err := cli.ImageInspect(ctx, desiredService.Image)
			if err != nil {
				logger.Warn("Could not inspect image. Skipping update check.", "image", desiredService.Image, "error", err)
				continue
			}
			if actualContainer.ImageID != desiredImg.ID {
				logger.Info("Image has change for service. Re-creating...", "service_name", serviceName)
				logger.Info("Stopping old container", "container_id", actualContainer.ID[:12])
				if err := cli.ContainerStop(ctx, actualContainer.ID, container.StopOptions{}); err != nil {
					logger.Error("Failed to stop container", "error", err)
					continue
				}
				if err := cli.ContainerRemove(ctx, actualContainer.ID, container.RemoveOptions{}); err != nil {
					logger.Error("Failed to remove container", "error", err)
					continue
				}
				if err := createService(ctx, cli, projectName, serviceName, &desiredService, logger); err != nil {
					logger.Error("Failed to create new service", "error", err)

				}
			} else {
				if actualContainer.State != "running" {
					logger.Warn("Container exists but is not running. Starting...", "service_name", serviceName, "container_id", actualContainer.ID[:12], "current_status", actualContainer.State)
					if err := cli.ContainerStart(ctx, actualContainer.ID, container.StartOptions{}); err != nil {
						logger.Error("Failed to start the container", "service_name", serviceName)
					} else {
						logger.Info("Container started successfully.", "service_name", serviceName)
					}
				} else {
					logger.Info("Service is up-to-date and running", "service_name", serviceName)
				}
			}

		} else {
			logger.Info("Service not found. Creating...", "service_name", serviceName)
			if err := createService(ctx, cli, projectName, serviceName, &desiredService, logger); err != nil {
				logger.Error("Failed to create new service", "error", err)
			}
		}
	}

	logger.Info("Checking for orphan services to prune...")
	for serviceName, serviceContainer := range actualState {
		if _, existsInDesired := compose.Services[serviceName]; !existsInDesired {
			logger.Info("Found orphaned service. Removing...", "service_name", serviceName)

			logger.Info("Stopping container", "container_id", serviceContainer.ID[:12])
			if err := cli.ContainerStop(ctx, serviceContainer.ID, container.StopOptions{}); err != nil {
				logger.Error("Failed to stop orphaned container", "error", err)
				continue
			}
			logger.Info("Removing container", "container_id", serviceContainer.ID[:12])
			if err := cli.ContainerRemove(ctx, serviceContainer.ID, container.RemoveOptions{}); err != nil {
				logger.Error("Failed to remove orphaned container", "error", err)
				continue
			}
		}
	}
	return nil
}

// createService creates and starts a new Docker container for the specified service
func createService(ctx context.Context, cli *client.Client, projectName string, serviceName string, service *Service, logger *slog.Logger) error {
	logger.Info("Creating service", "service_name", serviceName)

	if err := pullImage(ctx, cli, service.Image, logger); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", service.Image, err)
	}
	logger.Info("Image pulled successfully.", "image", service.Image)

	exposedPorts, portBindings, err := nat.ParsePortSpecs(service.Ports)
	if err != nil {
		return fmt.Errorf("failed to parse port specs: %w", err)
	}

	// The keys in this map must be the FULL network names.
	endpointsConfig := make(map[string]*network.EndpointSettings)
	for _, netName := range service.Networks {
		fullNetworkName := fmt.Sprintf("%s_%s", projectName, netName)
		endpointsConfig[fullNetworkName] = &network.EndpointSettings{
			Aliases: []string{serviceName},
		}
	}

	containerName := service.ContainerName
	if containerName == "" {
		containerName = serviceName
	}

	// We need to process the Binds to prefix named volumes.
	var processedBinds []string
	for _, v := range service.Volumes {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) == 2 {
			source := parts[0]
			// Check if it's a named volume (and not a host path bind mount)
			if !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".") {
				// It's a named volume, so prefix the source with the project name.
				prefixedSource := fmt.Sprintf("%s_%s", projectName, source)
				processedBinds = append(processedBinds, fmt.Sprintf("%s:%s", prefixedSource, parts[1]))
			} else {
				// It's a bind mount (e.g., /path/on/host:/path/in/container), so use it as-is.
				processedBinds = append(processedBinds, v)
			}
		} else {
			logger.Warn("Skipping malformed volume definition", "volume_string", v)
		}
	}

	var healthConfig *container.HealthConfig
	if service.HealthCheck != nil && len(service.HealthCheck.Test) > 0 {
		var interval, timeout, startPeriod time.Duration
		var err error
		if service.HealthCheck.Interval != "" {
			interval, err = time.ParseDuration(service.HealthCheck.Interval)
			if err != nil {
				return fmt.Errorf("invalid healthcheck interval format '%s': %w", service.HealthCheck.Interval, err)
			}
		}
		if service.HealthCheck.Timeout != "" {
			timeout, err = time.ParseDuration(service.HealthCheck.Timeout)
			if err != nil {
				return fmt.Errorf("invalid healthcheck timeout format '%s': %w", service.HealthCheck.Timeout, err)
			}
		}
		if service.HealthCheck.StartPeriod != "" {
			startPeriod, err = time.ParseDuration(service.HealthCheck.StartPeriod)
			if err != nil {
				return fmt.Errorf("invalid healthcheck start_period format '%s': %w", service.HealthCheck.StartPeriod, err)
			}
		}

		healthConfig = &container.HealthConfig{
			Test:        service.HealthCheck.Test,
			Interval:    interval,
			Timeout:     timeout,
			Retries:     service.HealthCheck.Retries,
			StartPeriod: startPeriod,
		}
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        processedBinds, // Use the processed list of binds
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        service.Image,
		Env:          service.Environment,
		Cmd:          service.Command,
		ExposedPorts: exposedPorts,
		Healthcheck:  healthConfig,
		Labels: map[string]string{
			"com.docker.compose.project": projectName,
			"com.docker.compose.service": serviceName,
		},
	}, hostConfig, &network.NetworkingConfig{
		EndpointsConfig: endpointsConfig,
	}, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	logger.Info("Successfully created and started service", "service_name", serviceName, "container_id", resp.ID[:12])
	return nil
}

// pullImage pulls the specified Docker image from the registry
func pullImage(ctx context.Context, cli *client.Client, imageName string, logger *slog.Logger) error {
	out, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()
	io.Copy(io.Discard, out)
	return nil
}

func waitForHealthCheck(ctx context.Context, cli *client.Client, containerID string, logger *slog.Logger) error {
	logger.Info("Waiting for container to be healthy", "container_id", containerID[:12])

	for i := 0; i < 60; i++ {
		inspect, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			return fmt.Errorf("failed to inspect container %s: %w", containerID, err)
		}
		if inspect.State != nil && inspect.State.Health != nil {
			status := inspect.State.Health.Status
			logger.Info("Container health status", "container_id", containerID[:12], "status", status)
			switch status {
			case "healthy":
				logger.Info("Container is healthy", "container_id", containerID[:12])
				return nil
			case "unhealthy":
				return fmt.Errorf("container %s is unhealthy", containerID)

			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for container %s to become healthy", containerID)
}
