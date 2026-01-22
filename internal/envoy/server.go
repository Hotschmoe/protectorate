package envoy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hotschmoe/protectorate/internal/config"
)

type Server struct {
	cfg       *config.EnvoyConfig
	http      *http.Server
	docker    *DockerClient
	sleeves   *SleeveManager
}

func NewServer(cfg *config.EnvoyConfig) (*Server, error) {
	docker, err := NewDockerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	sleeves := NewSleeveManager(docker, cfg)

	s := &Server{
		cfg:     cfg,
		docker:  docker,
		sleeves: sleeves,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.http = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s, nil
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/docker/containers", s.handleDockerContainers)
	mux.HandleFunc("/api/docker/networks", s.handleDockerNetworks)
	mux.HandleFunc("/api/sleeves", s.handleSleeves)
	mux.HandleFunc("/api/sleeves/", s.handleSleeveByName)
	mux.HandleFunc("/sleeves/", s.handleSleeveTerminal)
	mux.HandleFunc("/", s.handleIndex)
}

func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}
