package protocol

import "time"

// AuthProvider represents a supported authentication provider
type AuthProvider string

const (
	AuthProviderClaude AuthProvider = "claude"
	AuthProviderGemini AuthProvider = "gemini"
	AuthProviderCodex  AuthProvider = "codex"
	AuthProviderGit    AuthProvider = "git"
)

// AuthStatus represents the authentication status for all providers
type AuthStatus struct {
	Providers map[AuthProvider]*ProviderAuthStatus `json:"providers"`
}

// ProviderAuthStatus represents the authentication status for a single provider
type ProviderAuthStatus struct {
	Authenticated bool      `json:"authenticated"`
	Method        string    `json:"method,omitempty"` // "token", "oauth", "api_key", "ssh"
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	Username      string    `json:"username,omitempty"`
}

// AuthLoginRequest is the request body for logging in to a provider
type AuthLoginRequest struct {
	Provider AuthProvider `json:"provider"`
	Token    string       `json:"token,omitempty"`
}

// AuthLoginResult is the result of a login operation
type AuthLoginResult struct {
	Success  bool   `json:"success"`
	Provider string `json:"provider"`
	Method   string `json:"method,omitempty"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

// AuthRevokeResult is the result of a revoke operation
type AuthRevokeResult struct {
	Success  bool   `json:"success"`
	Provider string `json:"provider"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}
