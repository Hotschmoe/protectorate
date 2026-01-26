package protocol

import "time"

// SleeveInfo represents a running sleeve container
type SleeveInfo struct {
	Name        string    `json:"name"`
	ContainerID string    `json:"container_id"`
	Workspace   string    `json:"workspace"`
	TTYDPort    int       `json:"ttyd_port"`
	TTYDAddress string    `json:"ttyd_address"`
	SpawnTime   time.Time `json:"spawn_time"`
	Status      string    `json:"status"`
}

// SpawnSleeveRequest is the request body for spawning a new sleeve
type SpawnSleeveRequest struct {
	Workspace string `json:"workspace"`
	Name      string `json:"name,omitempty"`
	RepoURL   string `json:"repo_url,omitempty"`
}

// WorkspaceInfo represents a workspace directory
type WorkspaceInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	InUse      bool   `json:"in_use"`
	SleeveName string `json:"sleeve_name,omitempty"`
}
