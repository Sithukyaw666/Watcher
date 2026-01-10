package api

import (
	"net/http"
	"path/filepath"

	"github.com/sithukyaw666/watcher/internal/monitor"
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
		}
		if info, ok := statusMap[name]; ok {
			node.Status = info.Status
			node.ContainerID = info.ContainerID
		}
		graph = append(graph, node)
	}

	s.responseJSON(w, http.StatusOK, graph)
}
