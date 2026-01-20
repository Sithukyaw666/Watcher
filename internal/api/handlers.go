package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/sithukyaw666/watcher/internal/monitor"
	"github.com/sithukyaw666/watcher/operations"
	"github.com/sithukyaw666/watcher/operations/controller"
)

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSystemEvents(w http.ResponseWriter, r *http.Request) {
	setSSEHeaders(w)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	msgChan := make(chan SystemEvent, 10)

	s.clientsMu.Lock()
	s.clients[msgChan] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, msgChan)
		s.clientsMu.Unlock()
		close(msgChan)
	}()

	ctx := r.Context()

	for {
		select {
		case event := <-msgChan:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) handleCurrentDeployment(w http.ResponseWriter, r *http.Request) {
	lastSuccess, err := s.store.GetLastSuccessfulDeployment()
	if err != nil {
		s.logger.Warn("Failed to fetch last successful deployment", "error", err)
		s.responseJSON(w, http.StatusNotFound, map[string]string{"error": "no active deployment found"})
		return
	}
	s.responseJSON(w, http.StatusOK, lastSuccess)
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

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	targetService := r.URL.Query().Get("service")
	if targetService == "" {
		http.Error(w, "service parameter required", http.StatusBadRequest)
		return
	}

	setSSEHeaders(w)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
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
			data, err := json.Marshal(metrics)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
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

	setSSEHeaders(w)
	flush, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
	}

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
			data, err := json.Marshal(logs)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flush.Flush()
		case <-ctx.Done():
			return
		}
	}
}
