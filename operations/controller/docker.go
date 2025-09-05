package controller

import (
	"context"
	"log/slog"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/client"
)

// Apply is the main entry point for Docker operations. It lists running containers,
// builds the actual state map, and then delegates service reconciliation to ReconcileServices.
func Apply(ctx context.Context, cli *client.Client, projectName string, compose *Compose, logger *slog.Logger) error {

	ReconcileVolumes(ctx, cli, projectName, compose.Volumes, logger)
	ReconcileNetworks(ctx, cli, projectName, compose.Networks, logger)
	projectFilter := filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+projectName))
	runningContainers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: projectFilter,
	})

	if err != nil {
		return err
	}

	actualState := make(map[string]container.Summary)

	for _, c := range runningContainers {
		serviceName := c.Labels["com.docker.compose.service"]
		logger.Info("Found existing container for service", "service_name", serviceName, "container_id", c.ID[:12], "image", c.Image)
		if serviceName != "" {
			actualState[serviceName] = c
		}
	}

	logger.Info("Found containers for project", "container_count", len(actualState), "project_name", projectName)

	// Delegate service reconciliation to the dedicated function
	return ReconcileServices(ctx, cli, projectName, compose, actualState, logger)
}
