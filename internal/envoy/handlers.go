package envoy

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type WorkspaceInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	InUse      bool   `json:"in_use"`
	SleeveName string `json:"sleeve_name,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	credPath := "/home/claude/.claude/.credentials.json"
	_, err := os.Stat(credPath)
	authenticated := err == nil

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"authenticated": authenticated})
}

func (s *Server) handleDockerContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := s.docker.ListContainers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containers)
}

func (s *Server) handleDockerNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := s.docker.ListNetworks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(networks)
}

func (s *Server) handleSleeves(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sleeves := s.sleeves.List()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sleeves)

	case http.MethodPost:
		var req SpawnSleeveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		sleeve, err := s.sleeves.Spawn(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(sleeve)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSleeveByName(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sleeves/")
	name := strings.Split(path, "/")[0]

	if name == "" {
		http.Error(w, "sleeve name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		sleeve, err := s.sleeves.Get(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sleeve)

	case http.MethodDelete:
		if err := s.sleeves.Kill(name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSleeveTerminal(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sleeves/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[1] != "terminal" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	name := parts[0]
	sleeve, err := s.sleeves.Get(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.proxyWebSocket(w, r, sleeve.TTYDAddress)
}

func (s *Server) handleEnvoyTerminal(w http.ResponseWriter, r *http.Request) {
	s.proxyWebSocket(w, r, "localhost:7681")
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// DEV_MODE: Serve from filesystem for hot-reload (no rebuild needed)
	if os.Getenv("DEV_MODE") == "true" {
		devPaths := []string{
			"/app/web/templates/index.html",              // Mounted in container
			"./internal/envoy/web/templates/index.html",  // Local development
		}
		for _, path := range devPaths {
			if _, err := os.Stat(path); err == nil {
				http.ServeFile(w, r, path)
				return
			}
		}
	}

	// PROD: Serve from embedded filesystem
	w.Header().Set("Content-Type", "text/html")
	html, err := webFS.ReadFile("web/templates/index.html")
	if err != nil {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Write(html)
}

func (s *Server) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		workspaces, err := s.listWorkspaces()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(workspaces)

	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "workspace name required", http.StatusBadRequest)
			return
		}

		if strings.ContainsAny(req.Name, "/\\..") {
			http.Error(w, "invalid workspace name", http.StatusBadRequest)
			return
		}

		wsPath := filepath.Join(s.cfg.Docker.WorkspaceRoot, req.Name)
		if err := os.MkdirAll(wsPath, 0755); err != nil {
			http.Error(w, "failed to create workspace: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(WorkspaceInfo{
			Name:  req.Name,
			Path:  wsPath,
			InUse: false,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listWorkspaces() ([]WorkspaceInfo, error) {
	wsRoot := s.cfg.Docker.WorkspaceRoot

	if err := os.MkdirAll(wsRoot, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(wsRoot)
	if err != nil {
		return nil, err
	}

	sleeves := s.sleeves.List()
	wsToSleeve := make(map[string]string)
	for _, sl := range sleeves {
		wsToSleeve[sl.Workspace] = sl.Name
	}

	workspaces := make([]WorkspaceInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wsPath := filepath.Join(wsRoot, entry.Name())
		sleeveName := wsToSleeve[wsPath]

		workspaces = append(workspaces, WorkspaceInfo{
			Name:       entry.Name(),
			Path:       wsPath,
			InUse:      sleeveName != "",
			SleeveName: sleeveName,
		})
	}

	return workspaces, nil
}
