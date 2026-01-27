package envoy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

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

	readOnly := r.URL.Query().Get("mode") == "observe"
	socketPath := "/home/claude/.dtach/session.sock"

	gateway := NewTerminalGateway(s.docker, sleeve.ContainerName, socketPath, readOnly)
	gateway.Start(w, r)
}

func (s *Server) handleEnvoyTerminal(w http.ResponseWriter, r *http.Request) {
	readOnly := r.URL.Query().Get("mode") == "observe"
	socketPath := "/home/claude/.dtach/session.sock"

	envoyContainer, err := s.docker.GetContainerByName("envoy-dev")
	if err != nil || envoyContainer == nil {
		envoyContainer, err = s.docker.GetContainerByName("envoy")
	}
	if err != nil || envoyContainer == nil {
		http.Error(w, "envoy container not found", http.StatusNotFound)
		return
	}

	containerName := envoyContainer.Names[0]
	if len(containerName) > 0 && containerName[0] == '/' {
		containerName = containerName[1:]
	}

	gateway := NewTerminalGateway(s.docker, containerName, socketPath, readOnly)
	gateway.Start(w, r)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Strip "/static/" prefix to get the file path
	filePath := "web/static/" + r.URL.Path[len("/static/"):]

	// DEV_MODE: Serve from filesystem for hot-reload
	if os.Getenv("DEV_MODE") == "true" {
		devPaths := []string{
			"/app/" + filePath,                          // Mounted in container
			"./internal/envoy/" + filePath,              // Local development
		}
		for _, path := range devPaths {
			if _, err := os.Stat(path); err == nil {
				http.ServeFile(w, r, path)
				return
			}
		}
	}

	// PROD: Serve from embedded filesystem
	content, err := webFS.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type based on file extension
	switch {
	case strings.HasSuffix(filePath, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(filePath, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	}

	w.Write(content)
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

func (s *Server) handleWorkspaceCstack(w http.ResponseWriter, r *http.Request) {
	workspace := r.URL.Query().Get("workspace")
	if workspace == "" {
		http.Error(w, "workspace parameter required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		stats := getCstackInfo(workspace)
		if stats == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"exists": false})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)

	case http.MethodPost:
		action := r.URL.Query().Get("action")
		if action != "init" {
			http.Error(w, "action must be 'init'", http.StatusBadRequest)
			return
		}

		var req protocol.CstackInitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.Mode = "minimal"
		}

		result, err := s.workspaces.InitCstack(workspace, req.Mode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWorkspaceBranches(w http.ResponseWriter, r *http.Request) {
	workspace := r.URL.Query().Get("workspace")
	action := r.URL.Query().Get("action")

	// fetch-all doesn't require a workspace parameter
	if action == "fetch-all" && r.Method == http.MethodPost {
		result := s.workspaces.FetchAllRemotes()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

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

		case "commit":
			result, err := s.workspaces.CommitAll(workspace, "envoy ui commit")
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "not found") {
					http.Error(w, errMsg, http.StatusNotFound)
				} else if strings.Contains(errMsg, "in use") {
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

		case "push":
			result, err := s.workspaces.PushToRemote(workspace)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "not found") {
					http.Error(w, errMsg, http.StatusNotFound)
				} else if strings.Contains(errMsg, "in use") {
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
			http.Error(w, "invalid action: must be 'switch', 'fetch', 'pull', 'commit', or 'push'", http.StatusBadRequest)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	checks := []protocol.DoctorCheck{}

	checks = append(checks, checkSSHAgent())
	checks = append(checks, checkClaudeCredentials())
	checks = append(checks, checkGeminiCredentials())
	checks = append(checks, checkCodexCredentials())
	checks = append(checks, s.checkDocker())
	checks = append(checks, checkGitIdentity())
	checks = append(checks, s.checkRavenNetwork())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(checks)
}

func checkSSHAgent() protocol.DoctorCheck {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh-add", "-l")
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return protocol.DoctorCheck{
				Name:       "SSH Agent",
				Status:     "fail",
				Message:    fmt.Sprintf("Failed to run ssh-add: %v", err),
				Suggestion: "Ensure SSH agent is running: eval $(ssh-agent) && ssh-add",
			}
		}
	}

	switch exitCode {
	case 0:
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		keyCount := 0
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				keyCount++
			}
		}
		return protocol.DoctorCheck{
			Name:    "SSH Agent",
			Status:  "pass",
			Message: fmt.Sprintf("SSH agent running with %d key(s) loaded", keyCount),
		}
	case 1:
		return protocol.DoctorCheck{
			Name:       "SSH Agent",
			Status:     "warning",
			Message:    "SSH agent running but no keys loaded",
			Suggestion: "Run: ssh-add ~/.ssh/id_ed25519 (or your key path)",
		}
	default:
		return protocol.DoctorCheck{
			Name:       "SSH Agent",
			Status:     "fail",
			Message:    "SSH agent not running or not accessible",
			Suggestion: "Run: eval $(ssh-agent) && ssh-add",
		}
	}
}

func checkClaudeCredentials() protocol.DoctorCheck {
	credPath := "/home/claude/.claude/.credentials.json"
	if _, err := os.Stat(credPath); err == nil {
		return protocol.DoctorCheck{
			Name:    "Claude Credentials",
			Status:  "pass",
			Message: "Credentials file found",
		}
	}
	return protocol.DoctorCheck{
		Name:       "Claude Credentials",
		Status:     "fail",
		Message:    "Credentials file not found",
		Suggestion: "Run 'claude auth login' in a sleeve to authenticate",
	}
}

func checkGeminiCredentials() protocol.DoctorCheck {
	// Check for API key first
	if os.Getenv("GEMINI_API_KEY") != "" {
		return protocol.DoctorCheck{
			Name:    "Gemini Credentials",
			Status:  "pass",
			Message: "Authenticated via API key",
		}
	}

	// Check for OAuth tokens
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/claude"
	}
	oauthPaths := []string{
		homeDir + "/.config/gemini-cli/oauth_tokens.json",
		homeDir + "/.gemini/oauth_tokens.json",
	}

	for _, path := range oauthPaths {
		if _, err := os.Stat(path); err == nil {
			return protocol.DoctorCheck{
				Name:    "Gemini Credentials",
				Status:  "pass",
				Message: "Authenticated via OAuth",
			}
		}
	}

	return protocol.DoctorCheck{
		Name:       "Gemini Credentials",
		Status:     "warning",
		Message:    "Not authenticated (optional)",
		Suggestion: "Set GEMINI_API_KEY or run 'gemini' to authenticate",
	}
}

func checkCodexCredentials() protocol.DoctorCheck {
	// Check for API key first
	if os.Getenv("OPENAI_API_KEY") != "" {
		return protocol.DoctorCheck{
			Name:    "Codex Credentials",
			Status:  "pass",
			Message: "Authenticated via API key",
		}
	}

	// Check for cached auth
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/claude"
	}
	authPaths := []string{
		homeDir + "/.codex/auth.json",
		homeDir + "/.config/codex/auth.json",
	}

	for _, path := range authPaths {
		if _, err := os.Stat(path); err == nil {
			return protocol.DoctorCheck{
				Name:    "Codex Credentials",
				Status:  "pass",
				Message: "Authenticated via cached auth",
			}
		}
	}

	return protocol.DoctorCheck{
		Name:       "Codex Credentials",
		Status:     "warning",
		Message:    "Not authenticated (optional)",
		Suggestion: "Set OPENAI_API_KEY or run 'codex' to authenticate",
	}
}

func (s *Server) checkDocker() protocol.DoctorCheck {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := s.docker.cli.Ping(ctx); err != nil {
		return protocol.DoctorCheck{
			Name:       "Docker",
			Status:     "fail",
			Message:    fmt.Sprintf("Docker daemon unreachable: %v", err),
			Suggestion: "Ensure Docker is running and accessible",
		}
	}
	return protocol.DoctorCheck{
		Name:    "Docker",
		Status:  "pass",
		Message: "Docker daemon reachable",
	}
}

func checkGitIdentity() protocol.DoctorCheck {
	name := os.Getenv("GIT_COMMITTER_NAME")
	email := os.Getenv("GIT_COMMITTER_EMAIL")

	if name == "" || email == "" {
		missing := []string{}
		if name == "" {
			missing = append(missing, "GIT_COMMITTER_NAME")
		}
		if email == "" {
			missing = append(missing, "GIT_COMMITTER_EMAIL")
		}
		return protocol.DoctorCheck{
			Name:       "Git Identity",
			Status:     "fail",
			Message:    fmt.Sprintf("%s not set", strings.Join(missing, " and ")),
			Suggestion: "Set GIT_COMMITTER_NAME and GIT_COMMITTER_EMAIL in .env",
		}
	}

	if name == "Protectorate" || email == "protectorate@local" {
		return protocol.DoctorCheck{
			Name:       "Git Identity",
			Status:     "warning",
			Message:    fmt.Sprintf("Using default identity: %s <%s>", name, email),
			Suggestion: "Set GIT_COMMITTER_NAME and GIT_COMMITTER_EMAIL to your real identity in .env",
		}
	}

	return protocol.DoctorCheck{
		Name:    "Git Identity",
		Status:  "pass",
		Message: fmt.Sprintf("Identity configured: %s <%s>", name, email),
	}
}

func (s *Server) checkRavenNetwork() protocol.DoctorCheck {
	networks, err := s.docker.ListNetworks()
	if err != nil {
		return protocol.DoctorCheck{
			Name:       "Raven Network",
			Status:     "fail",
			Message:    fmt.Sprintf("Failed to list networks: %v", err),
			Suggestion: "Check Docker connectivity",
		}
	}

	for _, n := range networks {
		if n.Name == "raven" {
			return protocol.DoctorCheck{
				Name:    "Raven Network",
				Status:  "pass",
				Message: "Network 'raven' exists",
			}
		}
	}

	return protocol.DoctorCheck{
		Name:       "Raven Network",
		Status:     "fail",
		Message:    "Network 'raven' not found",
		Suggestion: "Run: docker network create raven",
	}
}

func (s *Server) handleAgentDoctorStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workspace := r.URL.Query().Get("workspace")
	status, err := s.agentDoctor.GetStatus(workspace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleAgentDoctorSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.AgentDoctorSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = protocol.AgentDoctorSyncRequest{}
	}

	result, err := s.agentDoctor.Sync(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleAgentDoctorInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.AgentDoctorInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Workspace == "" {
		http.Error(w, "workspace required", http.StatusBadRequest)
		return
	}

	result, err := s.agentDoctor.Init(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleAgentDoctorDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workspace := r.URL.Query().Get("workspace")
	if workspace == "" {
		http.Error(w, "workspace parameter required", http.StatusBadRequest)
		return
	}

	result, err := s.agentDoctor.Diff(workspace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
