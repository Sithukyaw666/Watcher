package monitor

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
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

type LogMessage struct {
	ServiceName string `json:"service"`
	Stream      string `json:"stream"`
	Data        string `json:"data"`
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

func StreamStats(ctx context.Context, cli *client.Client, containerID, name string, out chan<- ContainerMetrics, logger *slog.Logger) {
	info, err := cli.Info(ctx)
	hostMemTotal := int64(0)
	if err == nil {
		hostMemTotal = info.MemTotal
	} else {
		logger.Warn("Failed to get host info, memory percentages might be off for unlimited containers", "error", err)
	}

	stats, err := cli.ContainerStats(ctx, containerID, true)
	if err != nil {
		logger.Error("Failed to start stats stream", "id", containerID, "error", err)
		return
	}

	defer stats.Body.Close()

	dec := json.NewDecoder(stats.Body)
	var v container.StatsResponse

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := dec.Decode(&v); err != nil {
			return
		}

		metrics := calculateMetrics(v, uint64(hostMemTotal))
		metrics.ID = containerID
		metrics.Name = name
		select {
		case out <- metrics:
		case <-ctx.Done():
			return
		}
	}
}

func calculateMetrics(v container.StatsResponse, hostMemTotal uint64) ContainerMetrics {
	var cpuPercent = 0.0
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		onlineCPUs := float64(v.CPUStats.OnlineCPUs)
		if onlineCPUs == 0 {
			onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
		}
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	memUsage := v.MemoryStats.Usage

	if cache, ok := v.MemoryStats.Stats["cache"]; ok {
		memUsage -= cache
	}

	memLimit := v.MemoryStats.Limit

	if hostMemTotal > 0 && memLimit > hostMemTotal {
		memLimit = hostMemTotal
	}

	memPercent := 0.0
	if memLimit != 0.0 {
		memPercent = float64(memUsage) / float64(memLimit) * 100.0
	}

	var rx, tx float64
	for _, network := range v.Networks {
		rx += float64(network.RxBytes)
		tx += float64(network.TxBytes)
	}

	return ContainerMetrics{
		CPUPercent: cpuPercent,
		MemPercent: memPercent,
		MemUsage:   memUsage,
		MemLimit:   memLimit,
		NetRx:      rx / 1024,
		NetTx:      tx / 1024,
		Timestamp:  time.Now().Unix(),
	}

}

func StreamLogs(ctx context.Context, cli *client.Client, containerID, serviceName string, out chan<- LogMessage, logger *slog.Logger) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
	}

	resp, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return
	}
	defer resp.Close()

	outR, outW := io.Pipe()
	errR, errW := io.Pipe()

	go func() {
		defer outW.Close()
		defer errW.Close()
		stdcopy.StdCopy(outW, errW, resp)
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	readPipe := func(r io.Reader, streamName string) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case out <- LogMessage{ServiceName: serviceName, Stream: streamName, Data: scanner.Text()}:
			}
		}
	}

	go readPipe(outR, "stdout")
	go readPipe(errR, "stderr")
	wg.Wait()
}
