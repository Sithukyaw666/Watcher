package controller

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

func ReconcileNetworks(ctx context.Context, cli *client.Client, projectName string, networks map[string]Network, logger *slog.Logger) {
	logger.Info("Reconciling networks...")

	netFilters := filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+projectName))

	actualNetworks, err := cli.NetworkList(ctx, network.ListOptions{
		Filters: netFilters,
	})

	if err != nil {
		logger.Error("Could not list networks", "error", err)
		return
	}

	if len(networks) == 0 {
		logger.Info("No networks to reconcile.")
		return
	}

	actualNetworksMap := make(map[string]struct{})

	for _, net := range actualNetworks {
		actualNetworksMap[net.Labels["com.docker.compose.network"]] = struct{}{}
	}

	for networkName, net := range networks {
		if net.External {
			logger.Info("Skipping creation for external network", "network_name", networkName)
			continue
		}
		if _, exists := actualNetworksMap[networkName]; exists {
			continue
		}
		logger.Info("Creating network...", "network_name", networkName)
		fullNetworkName := fmt.Sprintf("%s_%s", projectName, networkName)
		_, err := cli.NetworkCreate(ctx, fullNetworkName, network.CreateOptions{
			Driver: net.Driver,
			Labels: map[string]string{
				"com.docker.compose.project": projectName,
				"com.docker.compose.network": networkName,
			},
		})
		if err != nil {
			logger.Info("Could not create network", "full_network_name", fullNetworkName, "error", err)
		} else {
			logger.Info("Network created successfully.", "full_network_name", fullNetworkName)
		}

	}
	for _, actualNet := range actualNetworks {
		networkName := actualNet.Labels["com.docker.compose.network"]
		if _, existsInDesired := networks[networkName]; !existsInDesired {
			logger.Info("Found orphaned network. Removing...", "network_name", actualNet.Name)
			if err := cli.NetworkRemove(ctx, actualNet.ID); err != nil {
				logger.Error("Failed to remove the network", "network_name", actualNet.Name, "error", err)
			}
		}
	}

}
