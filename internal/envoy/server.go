package envoy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hotschmoe/protectorate/internal/config"
)

type Server struct {
	cfg         *config.EnvoyConfig
	http        *http.Server
	docker      *DockerClient
	sidecar     *SidecarClient
	sleeves     *SleeveManager
	workspaces  *WorkspaceManager
	agentDoctor *AgentDoctorManager
	hostStats   *HostStatsCollector
}

func NewServer(cfg *config.EnvoyConfig) (*Server, error) {
	docker, err := NewDockerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	sleeves := NewSleeveManager(docker, cfg)
	workspaces := NewWorkspaceManager(cfg, sleeves.List)
	agentDoctor := NewAgentDoctorManager(cfg)
	hostStats := NewHostStatsCollector(docker, 20)
	sidecar := NewSidecarClient(cfg.Docker.Network)

	if err := sleeves.RecoverSleeves(); err != nil {
		return nil, fmt.Errorf("failed to recover sleeves: %w", err)
	}

	s := &Server{
		cfg:         cfg,
		docker:      docker,
		sidecar:     sidecar,
		sleeves:     sleeves,
		workspaces:  workspaces,
		agentDoctor: agentDoctor,
		hostStats:   hostStats,
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
	mux.HandleFunc("/api/doctor", s.handleDoctor)
	mux.HandleFunc("/api/docker/containers", s.handleDockerContainers)
	mux.HandleFunc("/api/docker/networks", s.handleDockerNetworks)
	mux.HandleFunc("/api/workspaces", s.handleWorkspaces)
	mux.HandleFunc("/api/workspaces/clone", s.handleCloneWorkspace)
	mux.HandleFunc("/api/workspaces/branches", s.handleWorkspaceBranches)
	mux.HandleFunc("/api/workspaces/cstack", s.handleWorkspaceCstack)
	mux.HandleFunc("/api/sleeves", s.handleSleeves)
	mux.HandleFunc("/api/sleeves/", s.handleSleeveByName)
	mux.HandleFunc("/api/host/stats", s.handleHostStats)
	mux.HandleFunc("/api/host/limits", s.handleHostLimits)
	mux.HandleFunc("/sleeves/", s.handleSleeveTerminal)
	mux.HandleFunc("/envoy/terminal", s.handleEnvoyTerminal)
	mux.HandleFunc("/api/agent-doctor/status", s.handleAgentDoctorStatus)
	mux.HandleFunc("/api/agent-doctor/sync", s.handleAgentDoctorSync)
	mux.HandleFunc("/api/agent-doctor/init", s.handleAgentDoctorInit)
	mux.HandleFunc("/api/agent-doctor/diff", s.handleAgentDoctorDiff)
	mux.HandleFunc("/static/", s.handleStatic)
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
