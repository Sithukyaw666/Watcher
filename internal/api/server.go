package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/moby/moby/client"
	"github.com/sithukyaw666/watcher/internal/store"
	"github.com/sithukyaw666/watcher/model"
)

type Server struct {
	store     *store.Store
	docker    *client.Client
	config    model.Config
	logger    *slog.Logger
	server    *http.Server
	uiFS      fs.FS
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex
}

type SystemEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func NewServer(port int, store *store.Store, docker *client.Client, config model.Config, logger *slog.Logger, uiFS fs.FS) *Server {
	mux := http.NewServeMux()

	s := &Server{
		store:   store,
		docker:  docker,
		config:  config,
		logger:  logger,
		uiFS:    uiFS,
		clients: make(map[*websocket.Conn]bool),
	}

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/graph", s.handleGraph)
	mux.HandleFunc("GET /api/current_deployment", s.handleCurrentDeployment)
	mux.HandleFunc("GET /api/history/view", s.handleHistoryView)

	mux.HandleFunc("GET /api/stream/metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/stream/logs", s.handleLogs)
	mux.HandleFunc("GET /api/system/events", s.handleSystemEvents)

	mux.Handle("/", s.enableCORS(s.logMiddleWare(s.spaHandler())))
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	return s
}

func (s *Server) Start() error {
	s.logger.Info("API Server starting", "adhandler,dr", s.server.Addr)

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
func (s *Server) BroadCast(eventType string, payload interface{}) {
	event := SystemEvent{
		Type:    eventType,
		Payload: payload,
	}

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for client := range s.clients {
		err := client.WriteJSON(event)
		if err != nil {
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *Server) spaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		f, err := s.uiFS.Open(path)
		if err == nil {
			f.Close()
			http.FileServer(http.FS(s.uiFS)).ServeHTTP(w, r)
			return
		}

		content, err := fs.ReadFile(s.uiFS, "index.html")
		if err != nil {
			http.Error(w, "UI not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(content)
	}
}

func (s *Server) responseJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
