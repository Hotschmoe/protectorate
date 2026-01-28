package sidecar

import (
	"os/exec"
	"strings"
	"sync"
)

// DHFInfo contains information about the detected CLI
type DHFInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// DHFDetector detects and caches CLI information
type DHFDetector struct {
	once sync.Once
	info *DHFInfo
}

// NewDHFDetector creates a new DHF detector
func NewDHFDetector() *DHFDetector {
	return &DHFDetector{}
}

// Info returns cached CLI information, detecting on first call
func (d *DHFDetector) Info() *DHFInfo {
	d.once.Do(func() {
		d.info = d.detect()
	})
	return d.info
}

func (d *DHFDetector) detect() *DHFInfo {
	// Try claude first
	if version := d.tryCommand("claude", "--version"); version != "" {
		return &DHFInfo{Name: "claude", Version: version}
	}

	// Try gemini
	if version := d.tryCommand("gemini", "--version"); version != "" {
		return &DHFInfo{Name: "gemini", Version: version}
	}

	// Try codex
	if version := d.tryCommand("codex", "--version"); version != "" {
		return &DHFInfo{Name: "codex", Version: version}
	}

	// Try aider
	if version := d.tryCommand("aider", "--version"); version != "" {
		return &DHFInfo{Name: "aider", Version: version}
	}

	// Try opencode
	if version := d.tryCommand("opencode", "--version"); version != "" {
		return &DHFInfo{Name: "opencode", Version: version}
	}

	return &DHFInfo{Name: "unknown", Version: ""}
}

func (d *DHFDetector) tryCommand(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(string(output))
	// Take first line only
	if idx := strings.Index(version, "\n"); idx != -1 {
		version = version[:idx]
	}
	return version
}
