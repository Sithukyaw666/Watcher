package controller

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

func ReconcileVolumes(ctx context.Context, cli *client.Client, projectName string, volumes map[string]Volume, logger *slog.Logger) {
	logger.Info("Reconciling volumes...")

	volFilters := filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+projectName))
	actualVolumes, err := cli.VolumeList(ctx, volume.ListOptions{
		Filters: volFilters,
	})

	if err != nil {
		logger.Error("Could not list volumes", "error", err)
	}

	actualVolumeMap := make(map[string]struct{})

	for _, vol := range actualVolumes.Volumes {
		actualVolumeMap[vol.Labels["com.docker.compose.project"]] = struct{}{}
	}

	if len(volumes) == 0 {
		logger.Info("No volumes to reconcile.")
		return
	}

	for volumeName, vol := range volumes {
		if vol.External {
			logger.Info("Skipping creation for external volume", "volume_name", volumeName)
			continue
		}

		if _, exists := actualVolumeMap[volumeName]; exists {
			continue
		}

		fullVolumeName := fmt.Sprintf("%s_%s", projectName, volumeName)

		_, err := cli.VolumeCreate(ctx, volume.CreateOptions{
			Name:   fullVolumeName,
			Driver: vol.Driver,
			Labels: map[string]string{
				"com.docker.compose.project": projectName,
				"com.docker.compose.volume":  volumeName,
			},
		})
		if err != nil {
			logger.Info("Could not create volume (may already exist)", "full_volume_name", fullVolumeName)
		} else {
			logger.Info("Volume created successfully", "full_volume_name", fullVolumeName)
		}
	}
	for _, actualVol := range actualVolumes.Volumes {
		volumeName := actualVol.Labels["com.docker.compose.volume"]
		if _, existsInDesired := volumes[volumeName]; !existsInDesired {
			logger.Info("Found orphaned volume. Removing...", "volume_name", actualVol.Name)

			if err := cli.VolumeRemove(ctx, actualVol.Name, true); err != nil {
				logger.Error("Failed to remove volume", "volume_name", actualVol.Name, "error", err)
			}
		}
	}
}
