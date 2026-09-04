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
	triggerChan chan<- struct{}
}

type response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

const ResponseContentType = "application/json"

func NewServer(store store.DeploymentQuerier, logger *slog.Logger, triggerChan chan<- struct{}) *Server {
	s := new(Server)
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", s.handleHealthCheck)
	mux.HandleFunc("/api/deployment/history", s.handleGetAllDeployment)
	mux.HandleFunc("/api/deployment/current", s.handleGetCurrentDeployment)
	mux.HandleFunc("POST /api/webhook", s.handleWebhookTrigger)

	s.logger = logger
	s.store = store
	s.Handler = mux
	s.triggerChan = triggerChan
	return s

}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", ResponseContentType)
	resp := response{
		Message: "OK",
		Data:    nil,
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetAllDeployment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", ResponseContentType)
	deployments, err := s.store.GetAllDeployments()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Warn("Server: cannot get deployment list", "error", err)
		resp := response{
			Message: "Cannot get the deployment list",
			Data:    nil,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	if deployments == nil {
		w.WriteHeader(http.StatusNotFound)
		resp := response{
			Message: "Deployment list is empty",
			Data:    deployments,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}
	w.WriteHeader(http.StatusOK)
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
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Warn("Server: cannot get the last deployment", "error", err)
		resp := response{
			Message: "Cannot get the last successful deployment",
			Data:    nil,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}
	if deployment == nil {
		w.WriteHeader(http.StatusNotFound)
		resp := response{
			Message: "No last successful deployment found",
			Data:    nil,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}
	w.WriteHeader(http.StatusOK)
	resp := response{
		Message: "Last successful deployment retrieved  successfully",
		Data:    deployment,
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleWebhookTrigger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", ResponseContentType)
	//todo: hmac verification for all git provider

	select {
	case s.triggerChan <- struct{}{}:
		s.logger.Info("Server: webhook received: queued deployment")
	default:
		s.logger.Info("Server: webhook received: deployment already running/queue, coalescing request")
	}

	resp := response{
		Message: "Webhook received",
		Data:    nil,
	}
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)

}
