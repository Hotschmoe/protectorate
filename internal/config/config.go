package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// EnvoyConfig defines the configuration for the Envoy manager service.
type EnvoyConfig struct {
	PollInterval  time.Duration `yaml:"poll_interval"`
	IdleThreshold time.Duration `yaml:"idle_threshold"`
	MaxSleeves    int           `yaml:"max_sleeves"`
	Port          int           `yaml:"port"`
	Docker        DockerConfig  `yaml:"docker"`
	Gitea         GiteaConfig   `yaml:"gitea"`
	Mirror        MirrorConfig  `yaml:"mirror"`
}

// DockerConfig defines Docker-specific configuration.
type DockerConfig struct {
	Network             string `yaml:"network"`
	WorkspaceRoot       string `yaml:"workspace_root"`
	WorkspaceHostRoot   string `yaml:"workspace_host_root"`
	CredentialsHostPath string `yaml:"credentials_host_path"`
	SettingsHostPath    string `yaml:"settings_host_path"`
	PluginsHostPath     string `yaml:"plugins_host_path"`
	SleeveImage         string `yaml:"sleeve_image"`
}

// GiteaConfig defines Gitea configuration.
type GiteaConfig struct {
	URL      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Token    string `yaml:"token"`
}

// MirrorConfig defines GitHub mirror configuration.
type MirrorConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Frequency string `yaml:"frequency"` // "daily", "hourly", etc.
	GitHubOrg string `yaml:"github_org"`
	Token     string `yaml:"github_token"`
}

// Matches ${VAR}, ${VAR:-default}, or $VAR
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

func expandEnv(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		var varName, defaultVal string
		hasDefault := false

		if match[1] == '{' {
			// ${VAR} or ${VAR:-default}
			inner := match[2 : len(match)-1]
			if idx := indexOf(inner, ":-"); idx >= 0 {
				varName = inner[:idx]
				defaultVal = inner[idx+2:]
				hasDefault = true
			} else {
				varName = inner
			}
		} else {
			// $VAR
			varName = match[1:]
		}

		if val := os.Getenv(varName); val != "" {
			return val
		}
		if hasDefault {
			return defaultVal
		}
		return match
	})
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func expandEnvInConfig(node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag == "!!str" {
			node.Value = expandEnv(node.Value)
		}
	case yaml.MappingNode, yaml.SequenceNode, yaml.DocumentNode:
		for _, child := range node.Content {
			expandEnvInConfig(child)
		}
	}
}

// LoadEnvoyConfig loads envoy configuration from a YAML file with defaults.
func LoadEnvoyConfig(path string) (*EnvoyConfig, error) {
	config := &EnvoyConfig{
		PollInterval:  1 * time.Hour,
		IdleThreshold: 0, // 0 means never timeout
		MaxSleeves:    10,
		Port:          7470,
		Docker: DockerConfig{
			Network:             "cortical-net",
			WorkspaceRoot:       "/workspaces",
			WorkspaceHostRoot:   "/workspaces",
			CredentialsHostPath: "",
			SettingsHostPath:    "",
			SleeveImage:         "ghcr.io/hotschmoe/protectorate-sleeve:latest",
		},
		Gitea: GiteaConfig{
			URL: "http://gitea:3000",
		},
		Mirror: MirrorConfig{
			Enabled:   false,
			Frequency: "daily",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	expandEnvInConfig(&root)

	expandedData, err := yaml.Marshal(&root)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal expanded YAML: %w", err)
	}

	if err := yaml.Unmarshal(expandedData, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

