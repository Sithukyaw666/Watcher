package controller

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// ReconcileServices handles the reconciliation of all services defined in the compose configuration
// against the actual running containers. It creates new services or updates existing ones as needed.
func ReconcileServices(ctx context.Context, cli *client.Client, projectName string, compose *Compose, actualState map[string]container.Summary, logger *slog.Logger) error {
	for serviceName, desiredService := range compose.Services {
		logger.Info("Reconciling service", "service_name", serviceName)
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
				logger.Info("Image has changed for service. Re-creating...", "service_name", serviceName)
				logger.Info("Stopping old container", "container_id", actualContainer.ID[:12])
				if err := cli.ContainerStop(ctx, actualContainer.ID, container.StopOptions{}); err != nil {
					logger.Error("Failed to stop container", "error", err)
					continue
				}

				if err := cli.ContainerRemove(ctx, actualContainer.ID, container.RemoveOptions{}); err != nil {
					logger.Error("Failed to remove container", "error", err)
				}
				if err := createService(ctx, cli, projectName, serviceName, &desiredService, logger); err != nil {
					logger.Error("Failed to create new service", "error", err)
				}
			} else {
				logger.Info("Service is up-to-date.", "service_name", serviceName)
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
	endpointsConfig := make(map[string]*network.EndpointSettings)
	for _, netName := range service.Networks {
		endpointsConfig[netName] = &network.EndpointSettings{
			Aliases: []string{serviceName},
		}
	}
	containerName := service.ContainerName
	if containerName == "" {
		containerName = serviceName
	}
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        service.Volumes,
	}
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        service.Image,
		Env:          service.Environment,
		ExposedPorts: exposedPorts,
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
