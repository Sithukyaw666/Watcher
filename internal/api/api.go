package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/sithukyaw666/watcher/internal/store"
)

type Server struct {
	logger *slog.Logger
	store  store.DeploymentQuerier
	http.Handler
}

type response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

const ResponseContentType = "application/json"

func NewServer(store store.DeploymentQuerier, logger *slog.Logger) *Server {
	s := new(Server)
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", s.handleHealthCheck)
	mux.HandleFunc("/api/deployment/history", s.handleGetAllDeployment)
	mux.HandleFunc("/api/deployment/current", s.handleGetCurrentDeployment)

	s.logger = logger
	s.store = store
	s.Handler = mux
	return s

}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetAllDeployment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", ResponseContentType)
	deployments, err := s.store.GetAllDeployments()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		s.logger.Error("Server: cannot get deployment list", "error", err)
		resp := response{
			Message: "Cannot get the deployment list",
			Data:    nil,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}
	w.WriteHeader(http.StatusOK)
	if deployments == nil {
		resp := response{
			Message: "Deployment list is empty",
			Data:    deployments,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := response{
		Message: "Deployment list retrieved successfully",
		Data:    deployments,
	}

	json.NewEncoder(w).Encode(resp)

}

func (s *Server) handleGetCurrentDeployment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", ResponseContentType)
	deployment, err := s.store.GetLastSuccessfulDeployment()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		s.logger.Error("Server: cannot get the last deployment", "error", err)
		resp := response{
			Message: "Cannot get the last successful deployment",
			Data:    nil,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}
	if deployment == nil {
		resp := response{
			Message: "No last successful deployment found",
			Data:    nil,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := response{
		Message: "Last successful deployment retrieved  successfully",
		Data:    deployment,
	}
	json.NewEncoder(w).Encode(resp)
}
