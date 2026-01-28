package sidecar

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuthInfo contains credential file status
type AuthInfo struct {
	ClaudeAuth bool `json:"claude_auth"`
	GeminiAuth bool `json:"gemini_auth"`
}

// AuthChecker checks credential files with caching
type AuthChecker struct {
	cacheTTL time.Duration

	mu       sync.RWMutex
	cached   *AuthInfo
	cachedAt time.Time
}

// NewAuthChecker creates a new auth checker
func NewAuthChecker() *AuthChecker {
	return &AuthChecker{
		cacheTTL: 30 * time.Second,
	}
}

// Status returns auth status, using cache if fresh
func (a *AuthChecker) Status() *AuthInfo {
	a.mu.RLock()
	if a.cached != nil && time.Since(a.cachedAt) < a.cacheTTL {
		info := a.cached
		a.mu.RUnlock()
		return info
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after acquiring write lock
	if a.cached != nil && time.Since(a.cachedAt) < a.cacheTTL {
		return a.cached
	}

	a.cached = a.check()
	a.cachedAt = time.Now()
	return a.cached
}

func (a *AuthChecker) check() *AuthInfo {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &AuthInfo{}
	}

	return &AuthInfo{
		ClaudeAuth: a.fileExists(filepath.Join(homeDir, ".claude", "credentials.json")),
		GeminiAuth: a.fileExists(filepath.Join(homeDir, ".config", "gemini", "credentials.json")),
	}
}

func (a *AuthChecker) fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
