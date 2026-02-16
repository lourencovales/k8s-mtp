package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/store"
)

type Server struct {
	*http.Server
	config *config.Config
	logger *slog.Logger
	store *store.Database
}

func NewServer(cfg *config.Config, logger *slog.Logger, store *store.Database) *Server {
	httpSrv := &http.Server{
		Addr: cfg.ListenAddr,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	srvLogger := logger.With("service", "api-server")

	return &Server{
		Server: httpSrv,
		config: cfg,
		logger: srvLogger,
		store: store,
	}
}

func (s *Server) NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/health", s.loggingMiddleware(s.healthHandler()))

	return mux
}

func (s *Server) Start() error {
	s.Handler = s.NewMux()
	s.logger.Info("server starting", "address", s.config.ListenAddr)
	return s.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}

func (s *Server) loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
		)
		next(w, r)
	}
}

func (s *Server) healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}
}
