package config

import (
	"os"
	"strconv"
	"time"
)

// EnvoyConfig defines the configuration for the Envoy manager service.
type EnvoyConfig struct {
	PollInterval  time.Duration
	IdleThreshold time.Duration
	MaxSleeves    int
	Port          int
	Docker        DockerConfig
	Gitea         GiteaConfig
	Mirror        MirrorConfig
}

// DockerConfig defines Docker-specific configuration.
type DockerConfig struct {
	Network         string
	WorkspaceRoot   string
	SleeveImage     string
	WorkspaceVolume string
	CredsVolume     string
}

// GiteaConfig defines Gitea configuration.
type GiteaConfig struct {
	URL      string
	User     string
	Password string
	Token    string
}

// MirrorConfig defines GitHub mirror configuration.
type MirrorConfig struct {
	Enabled   bool
	Frequency string
	GitHubOrg string
	Token     string
}

// getEnv returns the environment variable value or a default.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt returns the environment variable as int or a default.
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// getEnvDuration returns the environment variable as duration or a default.
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

// getEnvBool returns the environment variable as bool or a default.
func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

// LoadEnvoyConfig loads configuration from environment variables with sensible defaults.
// All settings can be overridden via environment variables.
//
// Environment variables:
//
//	ENVOY_PORT              - HTTP server port (default: 7470)
//	ENVOY_POLL_INTERVAL     - Sleeve poll interval (default: 1h)
//	ENVOY_IDLE_THRESHOLD    - Idle timeout, 0 = never (default: 0)
//	ENVOY_MAX_SLEEVES       - Maximum concurrent sleeves (default: 10)
//
//	DOCKER_NETWORK          - Docker network name (default: raven)
//	WORKSPACE_ROOT          - Container path for workspaces (default: /home/agent/workspaces)
//	SLEEVE_IMAGE            - Docker image for sleeves (default: ghcr.io/hotschmoe/protectorate-sleeve:latest)
//	WORKSPACE_VOLUME        - Docker volume for workspaces (default: agent-workspaces)
//	CREDS_VOLUME            - Docker volume for credentials (default: agent-creds)
//
//	GITEA_URL               - Gitea server URL (default: http://gitea:3000)
//	GITEA_USER              - Gitea username
//	GITEA_PASSWORD          - Gitea password
//	GITEA_TOKEN             - Gitea API token
//
//	MIRROR_ENABLED          - Enable GitHub mirroring (default: false)
//	MIRROR_FREQUENCY        - Mirror frequency (default: daily)
//	MIRROR_GITHUB_ORG       - GitHub organization to mirror
//	MIRROR_GITHUB_TOKEN     - GitHub API token for mirroring
func LoadEnvoyConfig() *EnvoyConfig {
	return &EnvoyConfig{
		Port:          getEnvInt("ENVOY_PORT", 7470),
		PollInterval:  getEnvDuration("ENVOY_POLL_INTERVAL", 1*time.Hour),
		IdleThreshold: getEnvDuration("ENVOY_IDLE_THRESHOLD", 0),
		MaxSleeves:    getEnvInt("ENVOY_MAX_SLEEVES", 10),
		Docker: DockerConfig{
			Network:         getEnv("DOCKER_NETWORK", "raven"),
			WorkspaceRoot:   getEnv("WORKSPACE_ROOT", "/home/agent/workspaces"),
			SleeveImage:     getEnv("SLEEVE_IMAGE", "ghcr.io/hotschmoe/protectorate-sleeve:latest"),
			WorkspaceVolume: getEnv("WORKSPACE_VOLUME", "agent-workspaces"),
			CredsVolume:     getEnv("CREDS_VOLUME", "agent-creds"),
		},
		Gitea: GiteaConfig{
			URL:      getEnv("GITEA_URL", "http://gitea:3000"),
			User:     getEnv("GITEA_USER", ""),
			Password: getEnv("GITEA_PASSWORD", ""),
			Token:    getEnv("GITEA_TOKEN", ""),
		},
		Mirror: MirrorConfig{
			Enabled:   getEnvBool("MIRROR_ENABLED", false),
			Frequency: getEnv("MIRROR_FREQUENCY", "daily"),
			GitHubOrg: getEnv("MIRROR_GITHUB_ORG", ""),
			Token:     getEnv("MIRROR_GITHUB_TOKEN", ""),
		},
	}
}
