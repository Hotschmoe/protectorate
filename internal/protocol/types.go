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
	Cstack     *CstackStats      `json:"cstack,omitempty"`
}

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

// CstackInitRequest is the request body for initializing cstack
type CstackInitRequest struct {
	Mode string `json:"mode"`
}

// CstackInitResult is the response from cstack init
type CstackInitResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
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

// BranchListResponse contains available branches for a workspace
type BranchListResponse struct {
	Current string   `json:"current"`
	Local   []string `json:"local"`
	Remote  []string `json:"remote"`
}

// SwitchBranchRequest is the request body for switching branches
type SwitchBranchRequest struct {
	Workspace string `json:"workspace"`
	Branch    string `json:"branch"`
}

// FetchResult contains the result of a git fetch operation
type FetchResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// DoctorCheck represents a single diagnostic check result
type DoctorCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"` // "pass", "warning", "fail"
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}
