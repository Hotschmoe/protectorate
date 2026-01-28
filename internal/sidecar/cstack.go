package sidecar

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// CstackStats represents task statistics from cs stats --json
type CstackStats struct {
	Open       int  `json:"open"`
	Ready      int  `json:"ready"`
	InProgress int  `json:"in_progress"`
	Blocked    int  `json:"blocked"`
	Closed     int  `json:"closed"`
	Total      int  `json:"total"`
	Exists     bool `json:"exists"`
}

// CstackChecker checks cstack status with caching
type CstackChecker struct {
	workspacePath string
	cacheTTL      time.Duration

	mu         sync.RWMutex
	cached     *CstackStats
	cachedAt   time.Time
}

// NewCstackChecker creates a new cstack checker
func NewCstackChecker(workspacePath string) *CstackChecker {
	return &CstackChecker{
		workspacePath: workspacePath,
		cacheTTL:      5 * time.Second,
	}
}

// Stats returns cstack statistics, using cache if fresh
func (c *CstackChecker) Stats() *CstackStats {
	c.mu.RLock()
	if c.cached != nil && time.Since(c.cachedAt) < c.cacheTTL {
		stats := c.cached
		c.mu.RUnlock()
		return stats
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.cached != nil && time.Since(c.cachedAt) < c.cacheTTL {
		return c.cached
	}

	c.cached = c.fetch()
	c.cachedAt = time.Now()
	return c.cached
}

func (c *CstackChecker) fetch() *CstackStats {
	cstackPath := filepath.Join(c.workspacePath, ".cstack")
	if _, err := os.Stat(cstackPath); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("cs", "stats", "--json")
	cmd.Dir = c.workspacePath
	output, err := cmd.Output()
	if err != nil {
		return &CstackStats{Exists: true}
	}

	var stats CstackStats
	if err := json.Unmarshal(output, &stats); err != nil {
		return &CstackStats{Exists: true}
	}

	stats.Exists = true
	return &stats
}
