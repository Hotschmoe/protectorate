package envoy

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/hotschmoe/protectorate/internal/protocol"
)

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
		var req protocol.SpawnSleeveRequest
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
		workspaces, err := s.workspaces.List()
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

		ws, err := s.workspaces.Create(req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ws)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCloneWorkspace(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jobID := r.URL.Query().Get("id")
		if jobID == "" {
			http.Error(w, "job id required", http.StatusBadRequest)
			return
		}

		job, err := s.workspaces.GetJob(jobID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)

	case http.MethodPost:
		var req protocol.CloneWorkspaceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		job, err := s.workspaces.Clone(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(job)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWorkspaceBranches(w http.ResponseWriter, r *http.Request) {
	workspace := r.URL.Query().Get("workspace")
	if workspace == "" {
		http.Error(w, "workspace parameter required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		branches, err := s.workspaces.ListBranches(workspace)
		if err != nil {
			if strings.Contains(err.Error(), "not a git repository") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(branches)

	case http.MethodPost:
		action := r.URL.Query().Get("action")

		switch action {
		case "switch":
			var req protocol.SwitchBranchRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}

			if req.Workspace == "" {
				req.Workspace = workspace
			}
			if req.Branch == "" {
				http.Error(w, "branch required", http.StatusBadRequest)
				return
			}

			err := s.workspaces.SwitchBranch(req.Workspace, req.Branch)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "not found") {
					http.Error(w, errMsg, http.StatusNotFound)
				} else if strings.Contains(errMsg, "in use") || strings.Contains(errMsg, "uncommitted") {
					http.Error(w, errMsg, http.StatusConflict)
				} else if strings.Contains(errMsg, "not a git repository") {
					http.Error(w, errMsg, http.StatusBadRequest)
				} else {
					http.Error(w, errMsg, http.StatusInternalServerError)
				}
				return
			}

			// Return updated workspace info
			workspaces, err := s.workspaces.List()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			for _, ws := range workspaces {
				if ws.Path == req.Workspace {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(ws)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case "fetch":
			result, err := s.workspaces.FetchRemote(workspace)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "not found") {
					http.Error(w, errMsg, http.StatusNotFound)
				} else if strings.Contains(errMsg, "not a git repository") {
					http.Error(w, errMsg, http.StatusBadRequest)
				} else {
					http.Error(w, errMsg, http.StatusInternalServerError)
				}
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)

		case "pull":
			result, err := s.workspaces.PullRemote(workspace)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "not found") {
					http.Error(w, errMsg, http.StatusNotFound)
				} else if strings.Contains(errMsg, "in use") || strings.Contains(errMsg, "uncommitted") {
					http.Error(w, errMsg, http.StatusConflict)
				} else if strings.Contains(errMsg, "not a git repository") {
					http.Error(w, errMsg, http.StatusBadRequest)
				} else {
					http.Error(w, errMsg, http.StatusInternalServerError)
				}
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)

		default:
			http.Error(w, "invalid action: must be 'switch', 'fetch', or 'pull'", http.StatusBadRequest)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
