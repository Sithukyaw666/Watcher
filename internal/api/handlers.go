package api

import (
	"net/http"
	"path/filepath"

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
	ID        string   `json:"id"`
	Image     string   `json:"image"`
	DependsOn []string `json:"depends_on"`
	Status    string   `json:"status"`
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	composePath := filepath.Join(s.config.DeploymentDir, s.config.ComposeFile)
	compose, err := controller.ParseComposeFile(composePath)
	if err != nil {
		s.responseJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse compose file"})
		return
	}

	var graph []GraphNode
	for name, service := range compose.Services {
		node := GraphNode{
			ID:        name,
			Image:     service.Image,
			DependsOn: service.DependsOn,
			Status:    "unknown",
		}
		graph = append(graph, node)
	}

	s.responseJSON(w, http.StatusOK, graph)
}
