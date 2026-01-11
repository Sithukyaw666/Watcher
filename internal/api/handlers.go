package api

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/gorilla/websocket"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/sithukyaw666/watcher/internal/monitor"
	"github.com/sithukyaw666/watcher/operations"
	"github.com/sithukyaw666/watcher/operations/controller"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	deployments, err := s.store.GetAllDeployments()
	if err != nil {
		s.logger.Error("Failed to fetch history", "error", err)
		s.responseJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch history"})
		return
	}
	s.responseJSON(w, http.StatusOK, deployments)
}

type GraphNode struct {
	ID          string   `json:"id"`
	Image       string   `json:"image"`
	DependsOn   []string `json:"depends_on"`
	Status      string   `json:"status"`
	ContainerID string   `json:"container_id"`
	State       string   `json:"state"`
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	composePath := filepath.Join(s.config.DeploymentDir, s.config.ComposeFile)
	compose, err := controller.ParseComposeFile(composePath)
	if err != nil {
		s.responseJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse compose file"})
		return
	}

	projectName := filepath.Base(s.config.DeploymentDir)
	statusMap, err := monitor.GetProjectStatus(r.Context(), s.docker, projectName)
	if err != nil {
		s.logger.Warn("Failed to get container statuses", "error", err)
	}

	var graph []GraphNode
	for name, service := range compose.Services {
		node := GraphNode{
			ID:          name,
			Image:       service.Image,
			DependsOn:   service.DependsOn,
			ContainerID: "unknown",
			Status:      "unknown",
			State:       "unknown",
		}
		if info, ok := statusMap[name]; ok {
			node.Status = info.Status
			node.ContainerID = info.ContainerID
			node.State = info.State
		}
		graph = append(graph, node)
	}

	s.responseJSON(w, http.StatusOK, graph)
}

func (s *Server) handleHistoryView(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		s.responseJSON(w, http.StatusBadRequest, map[string]string{"error": "hash is required"})
		return
	}

	content, err := operations.GetComposeContent(s.config, hash)
	if err != nil {
		s.logger.Error("Failed to fetch git snapshot", "hash", hash, "error", err)
		s.responseJSON(w, http.StatusNotFound, map[string]string{"error": "could not find the config for this commit"})
		return
	}

	s.responseJSON(w, http.StatusOK, map[string]interface{}{
		"hash":    hash,
		"content": content,
	})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	targetService := r.URL.Query().Get("service")
	if targetService == "" {
		http.Error(w, "service parameter required", http.StatusBadRequest)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Upgrade failed", "error", err)
		return
	}
	defer c.Close()

	projectName := filepath.Base(s.config.DeploymentDir)
	containers, _ := s.docker.ContainerList(r.Context(), container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+projectName)),
	})

	statsCh := make(chan monitor.ContainerMetrics)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	for _, container := range containers {
		svcName := container.Labels["com.docker.compose.service"]
		if svcName == targetService && container.State == "running" {
			go monitor.StreamStats(ctx, s.docker, container.ID, svcName, statsCh, s.logger)
		}
	}

	for {
		select {
		case metrics := <-statsCh:
			if err := c.WriteJSON(metrics); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}

}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	targetService := r.URL.Query().Get("service")
	if targetService == "" {
		http.Error(w, "service parameter required", http.StatusBadRequest)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	projectName := filepath.Base(s.config.DeploymentDir)
	containers, _ := s.docker.ContainerList(r.Context(), container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+projectName)),
	})

	logCh := make(chan monitor.LogMessage)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	for _, container := range containers {
		svcName := container.Labels["com.docker.compose.service"]
		if svcName == targetService {
			go monitor.StreamLogs(ctx, s.docker, container.ID, svcName, logCh, s.logger)
		}
	}

	for {
		select {
		case logs := <-logCh:
			if err := c.WriteJSON(logs); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
