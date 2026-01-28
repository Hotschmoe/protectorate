package envoy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hotschmoe/protectorate/internal/protocol"
)

const (
	credsBasePath = "/home/agent/.creds"
)

// AuthManager handles credential storage and authentication state
type AuthManager struct {
	mu sync.RWMutex
}

// NewAuthManager creates a new auth manager
func NewAuthManager() *AuthManager {
	return &AuthManager{}
}

// GetStatus returns the authentication status for all providers
func (m *AuthManager) GetStatus() *protocol.AuthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &protocol.AuthStatus{
		Providers: make(map[protocol.AuthProvider]*protocol.ProviderAuthStatus),
	}

	status.Providers[protocol.AuthProviderClaude] = m.getClaudeStatus()
	status.Providers[protocol.AuthProviderGemini] = m.getGeminiStatus()
	status.Providers[protocol.AuthProviderCodex] = m.getCodexStatus()
	status.Providers[protocol.AuthProviderGit] = m.getGitStatus()

	return status
}

func (m *AuthManager) getClaudeStatus() *protocol.ProviderAuthStatus {
	// Check for synced credentials (.credentials.json with dot prefix - Claude's format)
	credPath := filepath.Join(credsBasePath, "claude", ".credentials.json")
	if _, err := os.Stat(credPath); err == nil {
		return &protocol.ProviderAuthStatus{
			Authenticated: true,
			Method:        "oauth",
		}
	}

	// Fallback: check old format (credentials.json without dot)
	credPath = filepath.Join(credsBasePath, "claude", "credentials.json")
	if _, err := os.Stat(credPath); err != nil {
		return &protocol.ProviderAuthStatus{Authenticated: false}
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		return &protocol.ProviderAuthStatus{Authenticated: false}
	}

	var creds struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return &protocol.ProviderAuthStatus{Authenticated: false}
	}

	return &protocol.ProviderAuthStatus{
		Authenticated: creds.AccessToken != "",
		Method:        "token",
	}
}

func (m *AuthManager) getGeminiStatus() *protocol.ProviderAuthStatus {
	if os.Getenv("GEMINI_API_KEY") != "" {
		return &protocol.ProviderAuthStatus{
			Authenticated: true,
			Method:        "api_key",
		}
	}

	credPath := filepath.Join(credsBasePath, "gemini", "credentials.json")
	if _, err := os.Stat(credPath); err == nil {
		return &protocol.ProviderAuthStatus{
			Authenticated: true,
			Method:        "oauth",
		}
	}

	return &protocol.ProviderAuthStatus{Authenticated: false}
}

func (m *AuthManager) getCodexStatus() *protocol.ProviderAuthStatus {
	if os.Getenv("OPENAI_API_KEY") != "" {
		return &protocol.ProviderAuthStatus{
			Authenticated: true,
			Method:        "api_key",
		}
	}

	credPath := filepath.Join(credsBasePath, "codex", "auth.json")
	if _, err := os.Stat(credPath); err == nil {
		return &protocol.ProviderAuthStatus{
			Authenticated: true,
			Method:        "oauth",
		}
	}

	return &protocol.ProviderAuthStatus{Authenticated: false}
}

func (m *AuthManager) getGitStatus() *protocol.ProviderAuthStatus {
	sshKeyPath := filepath.Join(credsBasePath, "git", "id_ed25519")
	if _, err := os.Stat(sshKeyPath); err == nil {
		return &protocol.ProviderAuthStatus{
			Authenticated: true,
			Method:        "ssh",
		}
	}

	sshKeyPath = filepath.Join(credsBasePath, "git", "id_rsa")
	if _, err := os.Stat(sshKeyPath); err == nil {
		return &protocol.ProviderAuthStatus{
			Authenticated: true,
			Method:        "ssh",
		}
	}

	return &protocol.ProviderAuthStatus{Authenticated: false}
}

// Login stores credentials for a provider
func (m *AuthManager) Login(provider protocol.AuthProvider, token string) (*protocol.AuthLoginResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch provider {
	case protocol.AuthProviderClaude:
		return m.loginClaude(token)
	case protocol.AuthProviderGemini:
		return m.loginGemini(token)
	case protocol.AuthProviderCodex:
		return m.loginCodex(token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// storeCredentials is a helper that stores credentials for a provider
func (m *AuthManager) storeCredentials(provider protocol.AuthProvider, subdir, filename, tokenKey, method, missingMsg string, token string) (*protocol.AuthLoginResult, error) {
	if token == "" {
		return &protocol.AuthLoginResult{
			Success:  false,
			Provider: string(provider),
			Error:    missingMsg,
		}, nil
	}

	credDir := filepath.Join(credsBasePath, subdir)
	if err := os.MkdirAll(credDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create credentials directory: %w", err)
	}

	creds := map[string]string{tokenKey: token}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	credPath := filepath.Join(credDir, filename)
	if err := os.WriteFile(credPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write credentials: %w", err)
	}

	return &protocol.AuthLoginResult{
		Success:  true,
		Provider: string(provider),
		Method:   method,
		Message:  "credentials stored successfully",
	}, nil
}

func (m *AuthManager) loginClaude(token string) (*protocol.AuthLoginResult, error) {
	return m.storeCredentials(
		protocol.AuthProviderClaude,
		"claude",
		"credentials.json",
		"accessToken",
		"token",
		"token required for Claude authentication",
		token,
	)
}

func (m *AuthManager) loginGemini(token string) (*protocol.AuthLoginResult, error) {
	return m.storeCredentials(
		protocol.AuthProviderGemini,
		"gemini",
		"credentials.json",
		"api_key",
		"api_key",
		"API key required for Gemini authentication",
		token,
	)
}

func (m *AuthManager) loginCodex(token string) (*protocol.AuthLoginResult, error) {
	return m.storeCredentials(
		protocol.AuthProviderCodex,
		"codex",
		"auth.json",
		"api_key",
		"api_key",
		"API key required for Codex authentication",
		token,
	)
}

// Revoke removes credentials for a provider
func (m *AuthManager) Revoke(provider protocol.AuthProvider) (*protocol.AuthRevokeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var credDir string
	switch provider {
	case protocol.AuthProviderClaude:
		credDir = filepath.Join(credsBasePath, "claude")
	case protocol.AuthProviderGemini:
		credDir = filepath.Join(credsBasePath, "gemini")
	case protocol.AuthProviderCodex:
		credDir = filepath.Join(credsBasePath, "codex")
	case protocol.AuthProviderGit:
		credDir = filepath.Join(credsBasePath, "git")
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	if _, err := os.Stat(credDir); os.IsNotExist(err) {
		return &protocol.AuthRevokeResult{
			Success:  true,
			Provider: string(provider),
			Message:  "no credentials found",
		}, nil
	}

	entries, err := os.ReadDir(credDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			path := filepath.Join(credDir, entry.Name())
			if err := os.Remove(path); err != nil {
				return nil, fmt.Errorf("failed to remove %s: %w", path, err)
			}
		}
	}

	return &protocol.AuthRevokeResult{
		Success:  true,
		Provider: string(provider),
		Message:  "credentials revoked successfully",
	}, nil
}

// SyncFromCLI copies credentials from CLI tool locations to the shared volume
func (m *AuthManager) SyncFromCLI(provider string) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := map[string]interface{}{
		"synced": []string{},
		"errors": []string{},
	}

	synced := []string{}
	errors := []string{}

	if provider == "all" || provider == "claude" {
		if err := m.syncClaude(); err != nil {
			errors = append(errors, fmt.Sprintf("claude: %v", err))
		} else {
			synced = append(synced, "claude")
		}
	}

	result["synced"] = synced
	result["errors"] = errors

	if len(synced) > 0 {
		result["message"] = fmt.Sprintf("Synced: %v", synced)
	} else if len(errors) > 0 {
		result["message"] = fmt.Sprintf("Sync failed: %v", errors)
	} else {
		result["message"] = "Nothing to sync"
	}

	return result, nil
}

func (m *AuthManager) syncClaude() error {
	homeDir := "/home/agent"

	// Source locations (where Claude CLI stores credentials)
	claudeCredsPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	claudeSettingsPath := filepath.Join(homeDir, ".claude.json")

	// Destination in volume
	destDir := filepath.Join(credsBasePath, "claude")
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy credentials
	if _, err := os.Stat(claudeCredsPath); err == nil {
		data, err := os.ReadFile(claudeCredsPath)
		if err != nil {
			return fmt.Errorf("failed to read credentials: %w", err)
		}
		destPath := filepath.Join(destDir, ".credentials.json")
		if err := os.WriteFile(destPath, data, 0600); err != nil {
			return fmt.Errorf("failed to write credentials: %w", err)
		}
	} else {
		return fmt.Errorf("no Claude credentials found at %s", claudeCredsPath)
	}

	// Copy settings
	if _, err := os.Stat(claudeSettingsPath); err == nil {
		data, err := os.ReadFile(claudeSettingsPath)
		if err != nil {
			return fmt.Errorf("failed to read settings: %w", err)
		}
		destPath := filepath.Join(destDir, "settings.json")
		if err := os.WriteFile(destPath, data, 0600); err != nil {
			return fmt.Errorf("failed to write settings: %w", err)
		}
	}

	return nil
}
