package envoy

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

func getGitCloneProtocol() string {
	proto := os.Getenv("GIT_CLONE_PROTOCOL")
	if proto == "" {
		return "ssh"
	}
	return proto
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := map[string]interface{}{
			"server": map[string]interface{}{
				"port": s.cfg.Port,
			},
			"sleeves": map[string]interface{}{
				"max":            s.cfg.MaxSleeves,
				"poll_interval":  s.cfg.PollInterval.String(),
				"idle_threshold": s.cfg.IdleThreshold.String(),
				"image":          s.cfg.Docker.SleeveImage,
			},
			"docker": map[string]interface{}{
				"network":        s.cfg.Docker.Network,
				"workspace_root": s.cfg.Docker.WorkspaceRoot,
			},
			"git": map[string]interface{}{
				"clone_protocol": getGitCloneProtocol(),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleConfigKey(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/config/")
	key := strings.TrimSuffix(path, "/")

	switch r.Method {
	case http.MethodGet:
		value, err := s.getConfigValue(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"key": key, "value": value})

	case http.MethodPut:
		var req struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.setConfigValue(key, req.Value); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		value, _ := s.getConfigValue(key)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"key": key, "value": value})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfigValue(key string) (interface{}, error) {
	switch key {
	case "server.port":
		return s.cfg.Port, nil
	case "sleeves.max":
		return s.cfg.MaxSleeves, nil
	case "sleeves.poll_interval":
		return s.cfg.PollInterval.String(), nil
	case "sleeves.idle_threshold":
		return s.cfg.IdleThreshold.String(), nil
	case "sleeves.image":
		return s.cfg.Docker.SleeveImage, nil
	case "docker.network":
		return s.cfg.Docker.Network, nil
	case "docker.workspace_root":
		return s.cfg.Docker.WorkspaceRoot, nil
	case "git.clone_protocol":
		return getGitCloneProtocol(), nil
	default:
		return nil, &configKeyNotFoundError{key: key}
	}
}

func (s *Server) setConfigValue(key, value string) error {
	// For now, config is read-only at runtime
	// Future: persist to YAML file and reload
	return &configReadOnlyError{key: key}
}

type configKeyNotFoundError struct {
	key string
}

func (e *configKeyNotFoundError) Error() string {
	return "config key not found: " + e.key
}

type configReadOnlyError struct {
	key string
}

func (e *configReadOnlyError) Error() string {
	return "config is read-only at runtime: " + e.key
}
