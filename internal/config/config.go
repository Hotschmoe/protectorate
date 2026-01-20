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
	Network       string `yaml:"network"`
	WorkspaceRoot string `yaml:"workspace_root"`
	SleeveImage   string `yaml:"sleeve_image"`
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

// SleeveConfig defines the configuration for a sleeve (agent container).
type SleeveConfig struct {
	Name       string          `yaml:"name"`
	Image      string          `yaml:"image"`
	CLI        string          `yaml:"cli"`
	Env        []string        `yaml:"env"`
	Resources  ResourcesConfig `yaml:"resources"`
}

// ResourcesConfig defines resource limits for a sleeve.
type ResourcesConfig struct {
	Memory string `yaml:"memory"`
	CPUs   string `yaml:"cpus"`
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

func expandEnv(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		var varName string
		if match[1] == '{' {
			varName = match[2 : len(match)-1]
		} else {
			varName = match[1:]
		}
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})
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
			Network:       "cortical-net",
			WorkspaceRoot: "/workspaces",
			SleeveImage:   "ghcr.io/hotschmoe/protectorate-sleeve:latest",
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

// LoadSleeveConfig loads sleeve configuration from a YAML file.
func LoadSleeveConfig(path string) (*SleeveConfig, error) {
	config := &SleeveConfig{}

	data, err := os.ReadFile(path)
	if err != nil {
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
