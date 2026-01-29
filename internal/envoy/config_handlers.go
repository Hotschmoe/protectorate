package envoy

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := map[string]interface{}{
			"server": map[string]interface{}{
				"port": s.cfg.Server.Port,
			},
			"sleeves": map[string]interface{}{
				"max":            s.cfg.Sleeves.Max,
				"poll_interval":  s.cfg.Sleeves.PollInterval,
				"idle_threshold": s.cfg.Sleeves.IdleThreshold,
				"image":          s.cfg.Sleeves.Image,
			},
			"docker": map[string]interface{}{
				"network":        s.cfg.Docker.Network,
				"workspace_root": s.cfg.Docker.WorkspaceRoot,
			},
			"git": map[string]interface{}{
				"clone_protocol": s.cfg.Git.CloneProtocol,
				"committer": map[string]interface{}{
					"name":  s.cfg.Git.Committer.Name,
					"email": s.cfg.Git.Committer.Email,
				},
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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key":     key,
			"value":   value,
			"message": "saved - restart envoy to apply changes",
		})

	case http.MethodDelete:
		if err := s.resetConfigValue(key); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		value, _ := s.getConfigValue(key)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key":     key,
			"value":   value,
			"message": "reset to default - restart envoy to apply changes",
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfigValue(key string) (interface{}, error) {
	switch key {
	case "server.port":
		return s.cfg.Server.Port, nil
	case "sleeves.max":
		return s.cfg.Sleeves.Max, nil
	case "sleeves.poll_interval":
		return s.cfg.Sleeves.PollInterval, nil
	case "sleeves.idle_threshold":
		return s.cfg.Sleeves.IdleThreshold, nil
	case "sleeves.image":
		return s.cfg.Sleeves.Image, nil
	case "docker.network":
		return s.cfg.Docker.Network, nil
	case "docker.workspace_root":
		return s.cfg.Docker.WorkspaceRoot, nil
	case "git.clone_protocol":
		return s.cfg.Git.CloneProtocol, nil
	case "git.committer.name":
		return s.cfg.Git.Committer.Name, nil
	case "git.committer.email":
		return s.cfg.Git.Committer.Email, nil
	default:
		return nil, &configKeyNotFoundError{key: key}
	}
}

func (s *Server) setConfigValue(key, value string) error {
	switch key {
	case "server.port":
		v, err := strconv.Atoi(value)
		if err != nil || v < 1 || v > 65535 {
			return errors.New("server.port must be 1-65535")
		}
		s.cfg.Server.Port = v

	case "sleeves.max":
		v, err := strconv.Atoi(value)
		if err != nil || v < 1 || v > 100 {
			return errors.New("sleeves.max must be 1-100")
		}
		s.cfg.Sleeves.Max = v

	case "sleeves.poll_interval":
		if _, err := time.ParseDuration(value); err != nil {
			return errors.New("sleeves.poll_interval must be a valid duration (e.g., 1h, 30m)")
		}
		s.cfg.Sleeves.PollInterval = value

	case "sleeves.idle_threshold":
		if _, err := time.ParseDuration(value); err != nil {
			return errors.New("sleeves.idle_threshold must be a valid duration (e.g., 1h, 0)")
		}
		s.cfg.Sleeves.IdleThreshold = value

	case "sleeves.image":
		if value == "" {
			return errors.New("sleeves.image cannot be empty")
		}
		s.cfg.Sleeves.Image = value

	case "docker.network":
		if value == "" {
			return errors.New("docker.network cannot be empty")
		}
		s.cfg.Docker.Network = value

	case "docker.workspace_root":
		return errors.New("docker.workspace_root is read-only")

	case "git.clone_protocol":
		if value != "ssh" && value != "https" {
			return errors.New("git.clone_protocol must be 'ssh' or 'https'")
		}
		s.cfg.Git.CloneProtocol = value

	case "git.committer.name":
		s.cfg.Git.Committer.Name = value

	case "git.committer.email":
		s.cfg.Git.Committer.Email = value

	default:
		return &configKeyNotFoundError{key: key}
	}

	return s.cfg.Save()
}

func (s *Server) resetConfigValue(key string) error {
	switch key {
	case "server.port":
		s.cfg.Server.Port = 7470
	case "sleeves.max":
		s.cfg.Sleeves.Max = 10
	case "sleeves.poll_interval":
		s.cfg.Sleeves.PollInterval = "1h"
	case "sleeves.idle_threshold":
		s.cfg.Sleeves.IdleThreshold = "0"
	case "sleeves.image":
		s.cfg.Sleeves.Image = "ghcr.io/hotschmoe/protectorate-sleeve:latest"
	case "docker.network":
		s.cfg.Docker.Network = "raven"
	case "git.clone_protocol":
		s.cfg.Git.CloneProtocol = "ssh"
	case "git.committer.name":
		s.cfg.Git.Committer.Name = ""
	case "git.committer.email":
		s.cfg.Git.Committer.Email = ""
	default:
		return &configKeyNotFoundError{key: key}
	}

	return s.cfg.Save()
}

type configKeyNotFoundError struct {
	key string
}

func (e *configKeyNotFoundError) Error() string {
	return "config key not found: " + e.key
}
