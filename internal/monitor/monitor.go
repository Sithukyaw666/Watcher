package monitor

import (
	"context"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/client"
)

type ContainerMetrics struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	MemUsage   uint64  `json:"mem_usage"`
	MemLimit   uint64  `json:"mem_limit"`
	NetRx      float64 `json:"net_rx_kb"`
	NetTx      float64 `json:"net_tx_kb"`
	Timestamp  int64   `json:"timestamp"`
}

type ServiceStatus struct {
	ServiceName string `json:"service_name"`
	ContainerID string `json:"container_id"`
	State       string `json:"state"`
	Status      string `json:"status"`
}

func GetProjectStatus(ctx context.Context, cli *client.Client, projectName string) (map[string]ServiceStatus, error) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+projectName)),
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]ServiceStatus)

	for _, c := range containers {
		if svcName, ok := c.Labels["com.docker.compose.service"]; ok {
			result[svcName] = ServiceStatus{
				ServiceName: svcName,
				ContainerID: c.ID,
				State:       c.State,
				Status:      c.Status,
			}
		}
	}

	return result, nil
}
