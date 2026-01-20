package protocol

import "time"

// SleeveStatus represents the current state of a sleeve (agent container).
type SleeveStatus string

const (
	StatusIdle    SleeveStatus = "idle"
	StatusWorking SleeveStatus = "working"
	StatusBlocked SleeveStatus = "blocked"
	StatusDone    SleeveStatus = "done"
	StatusError   SleeveStatus = "error"
)

// MessageType represents the category of a message between sleeves or envoy.
type MessageType string

const (
	MessageTask      MessageType = "task"
	MessageMilestone MessageType = "milestone"
	MessageBlocked   MessageType = "blocked"
	MessageQuestion  MessageType = "question"
	MessageDone      MessageType = "done"
	MessageDirective MessageType = "directive"
)

// Message represents a communication between sleeves or between sleeve and envoy.
type Message struct {
	ID        string      `json:"id"`
	From      string      `json:"from"`
	To        string      `json:"to"`
	Thread    string      `json:"thread,omitempty"`
	Type      MessageType `json:"type"`
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
}

// SpawnRequest represents a request to create a new sleeve (agent container).
type SpawnRequest struct {
	Name     string            `json:"name,omitempty"` // Optional, auto-assigned if empty
	Repo     string            `json:"repo"`
	Goal     string            `json:"goal"`
	CLI      string            `json:"cli"`
	Priority int               `json:"priority,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// ResleeveRequest represents a request to resleeve an agent.
type ResleeveRequest struct {
	Type string `json:"type"` // "soft" or "hard"
	CLI  string `json:"cli,omitempty"`
}

// SleeveInfo contains metadata about a sleeve (agent container).
type SleeveInfo struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	ContainerID string       `json:"container_id"`
	Status      SleeveStatus `json:"status"`
	SpawnTime   time.Time    `json:"spawn_time"`
	LastCheckIn time.Time    `json:"last_check_in"`
	Workspace   string       `json:"workspace"`
	CLI         string       `json:"cli"`
	Goal        string       `json:"goal"`
}

// Progress tracks task completion metrics.
type Progress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
}

// StatusResponse represents the current state reported by a sleeve's sidecar.
type StatusResponse struct {
	SleeveID     string       `json:"sleeve_id"`
	Status       SleeveStatus `json:"status"`
	CurrentTask  string       `json:"current_task"`
	Progress     Progress     `json:"progress"`
	Blockers     []string     `json:"blockers,omitempty"`
	LastModified time.Time    `json:"last_modified"`
}

// DockerSpawnRequest represents a request for envoy to spawn a container on behalf of a sleeve.
type DockerSpawnRequest struct {
	Image   string            `json:"image"`
	Network string            `json:"network,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Ports   []string          `json:"ports,omitempty"`
}

// DockerSpawnResponse contains info about a spawned container.
type DockerSpawnResponse struct {
	ContainerID string `json:"container_id"`
	Address     string `json:"address"`
}
