package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/moby/moby/client"
	"github.com/sithukyaw666/watcher/internal/store"
	"github.com/sithukyaw666/watcher/model"
)

type Server struct {
	store  *store.Store
	docker *client.Client
	config model.Config
	logger *slog.Logger
	server *http.Server
}

func NewServer(port int, store *store.Store, docker *client.Client, config model.Config, logger *slog.Logger) *Server {
	mux := http.NewServeMux()

	s := &Server{
		store:  store,
		docker: docker,
		config: config,
		logger: logger,
	}

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/graph", s.handleGraph)

	mux.HandleFunc("GET /api/stream/metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/stream/logs", s.handleLogs)

	handler := s.enableCORS(s.logMiddleWare(mux))

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
	return s
}

func (s *Server) Start() error {
	s.logger.Info("API Server starting", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("API Request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (s *Server) responseJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
