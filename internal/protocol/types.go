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
}

// CloneWorkspaceRequest is the request body for cloning a git repo into a workspace
type CloneWorkspaceRequest struct {
	RepoURL string `json:"repo_url"`
	Name    string `json:"name,omitempty"`
}

// CloneJob represents an async clone operation
type CloneJob struct {
	ID        string    `json:"id"`
	RepoURL   string    `json:"repo_url"`
	Workspace string    `json:"workspace"`
	Status    string    `json:"status"` // pending, cloning, completed, failed
	Error     string    `json:"error,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

// WorkspaceInfo represents a workspace directory
type WorkspaceInfo struct {
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	InUse      bool              `json:"in_use"`
	SleeveName string            `json:"sleeve_name,omitempty"`
	Git        *WorkspaceGitInfo `json:"git,omitempty"`
}

// WorkspaceGitInfo contains git repository information for a workspace
type WorkspaceGitInfo struct {
	Branch           string `json:"branch"`
	IsDetached       bool   `json:"is_detached,omitempty"`
	RemoteBranch     string `json:"remote_branch,omitempty"`
	IsDirty          bool   `json:"is_dirty"`
	UncommittedCount int    `json:"uncommitted_count"`
	AheadCount       int    `json:"ahead_count"`
	BehindCount      int    `json:"behind_count"`
	LastCommitHash   string `json:"last_commit_hash,omitempty"`
	LastCommitMsg    string `json:"last_commit_msg,omitempty"`
	LastCommitTime   string `json:"last_commit_time,omitempty"`
}
