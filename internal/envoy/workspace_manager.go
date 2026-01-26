package envoy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hotschmoe/protectorate/internal/config"
	"github.com/hotschmoe/protectorate/internal/protocol"
)

type WorkspaceManager struct {
	mu           sync.RWMutex
	cfg          *config.EnvoyConfig
	jobs         map[string]*protocol.CloneJob
	sleeveGetter func() []*protocol.SleeveInfo
}

func NewWorkspaceManager(cfg *config.EnvoyConfig, sleeveGetter func() []*protocol.SleeveInfo) *WorkspaceManager {
	wm := &WorkspaceManager{
		cfg:          cfg,
		jobs:         make(map[string]*protocol.CloneJob),
		sleeveGetter: sleeveGetter,
	}
	go wm.cleanupExpiredJobs()
	return wm
}

func (wm *WorkspaceManager) List() ([]protocol.WorkspaceInfo, error) {
	wsRoot := wm.cfg.Docker.WorkspaceRoot

	if err := os.MkdirAll(wsRoot, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(wsRoot)
	if err != nil {
		return nil, err
	}

	sleeves := wm.sleeveGetter()
	wsToSleeve := make(map[string]string)
	for _, sl := range sleeves {
		wsToSleeve[sl.Workspace] = sl.Name
	}

	workspaces := make([]protocol.WorkspaceInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wsPath := filepath.Join(wsRoot, entry.Name())
		sleeveName := wsToSleeve[wsPath]

		ws := protocol.WorkspaceInfo{
			Name:       entry.Name(),
			Path:       wsPath,
			InUse:      sleeveName != "",
			SleeveName: sleeveName,
		}

		if gitInfo := getGitInfo(wsPath); gitInfo != nil {
			ws.Git = gitInfo
		}

		workspaces = append(workspaces, ws)
	}

	return workspaces, nil
}

func (wm *WorkspaceManager) Create(name string) (*protocol.WorkspaceInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("workspace name required")
	}

	if strings.ContainsAny(name, "/\\..") {
		return nil, fmt.Errorf("invalid workspace name")
	}

	wsPath := filepath.Join(wm.cfg.Docker.WorkspaceRoot, name)

	if _, err := os.Stat(wsPath); err == nil {
		return nil, fmt.Errorf("workspace %q already exists", name)
	}

	if err := os.MkdirAll(wsPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	return &protocol.WorkspaceInfo{
		Name:  name,
		Path:  wsPath,
		InUse: false,
	}, nil
}

func (wm *WorkspaceManager) Clone(req protocol.CloneWorkspaceRequest) (*protocol.CloneJob, error) {
	if req.RepoURL == "" {
		return nil, fmt.Errorf("repo_url required")
	}

	if !strings.HasPrefix(req.RepoURL, "https://") {
		return nil, fmt.Errorf("only HTTPS URLs are supported")
	}

	wsName := req.Name
	if wsName == "" {
		wsName = repoNameFromURL(req.RepoURL)
		if wsName == "" {
			return nil, fmt.Errorf("could not derive workspace name from repo URL")
		}
	}

	wsPath := filepath.Join(wm.cfg.Docker.WorkspaceRoot, wsName)

	if _, err := os.Stat(wsPath); err == nil {
		return nil, fmt.Errorf("workspace %q already exists", wsName)
	}

	jobID := generateJobID()
	job := &protocol.CloneJob{
		ID:        jobID,
		RepoURL:   req.RepoURL,
		Workspace: wsPath,
		Status:    "cloning",
		StartTime: time.Now(),
	}

	wm.mu.Lock()
	wm.jobs[jobID] = job
	wm.mu.Unlock()

	go wm.runClone(job)

	return job, nil
}

func (wm *WorkspaceManager) GetJob(id string) (*protocol.CloneJob, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	job, ok := wm.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %q not found", id)
	}
	return job, nil
}

func (wm *WorkspaceManager) runClone(job *protocol.CloneJob) {
	err := cloneRepo(job.RepoURL, job.Workspace)

	wm.mu.Lock()
	defer wm.mu.Unlock()

	job.EndTime = time.Now()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		os.RemoveAll(job.Workspace)
	} else {
		job.Status = "completed"
	}
}

func (wm *WorkspaceManager) cleanupExpiredJobs() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		wm.mu.Lock()
		cutoff := time.Now().Add(-1 * time.Hour)
		for id, job := range wm.jobs {
			if job.Status == "completed" || job.Status == "failed" {
				if job.EndTime.Before(cutoff) {
					delete(wm.jobs, id)
				}
			}
		}
		wm.mu.Unlock()
	}
}

func repoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func cloneRepo(url, destPath string) error {
	cmd := exec.Command("git", "clone", url, destPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func generateJobID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getGitInfo(wsPath string) *protocol.WorkspaceGitInfo {
	gitDir := filepath.Join(wsPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	info := &protocol.WorkspaceGitInfo{}

	branch, isDetached := getGitBranch(wsPath)
	info.Branch = branch
	info.IsDetached = isDetached

	if !isDetached {
		info.RemoteBranch = getGitRemoteBranch(wsPath, branch)
		ahead, behind := getGitAheadBehind(wsPath, info.RemoteBranch)
		info.AheadCount = ahead
		info.BehindCount = behind
	}

	uncommitted := getGitUncommittedCount(wsPath)
	info.UncommittedCount = uncommitted
	info.IsDirty = uncommitted > 0

	hash, msg, timeAgo := getGitLastCommit(wsPath)
	info.LastCommitHash = hash
	info.LastCommitMsg = msg
	info.LastCommitTime = timeAgo

	return info
}

func getGitBranch(wsPath string) (string, bool) {
	branch, err := runGitCommand(wsPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", false
	}
	if branch == "HEAD" {
		hash, _ := runGitCommand(wsPath, "rev-parse", "--short", "HEAD")
		return hash, true
	}
	return branch, false
}

func getGitRemoteBranch(wsPath, branch string) string {
	upstream, err := runGitCommand(wsPath, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		return "origin/" + branch
	}
	return upstream
}

func getGitAheadBehind(wsPath, remoteBranch string) (int, int) {
	out, err := runGitCommand(wsPath, "rev-list", "--left-right", "--count", remoteBranch+"...HEAD")
	if err != nil {
		return 0, 0
	}

	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0
	}

	var behind, ahead int
	fmt.Sscanf(parts[0], "%d", &behind)
	fmt.Sscanf(parts[1], "%d", &ahead)
	return ahead, behind
}

func getGitUncommittedCount(wsPath string) int {
	out, err := runGitCommand(wsPath, "status", "--porcelain")
	if err != nil {
		return 0
	}
	if out == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(out), "\n"))
}

func getGitLastCommit(wsPath string) (hash, msg, timeAgo string) {
	out, err := runGitCommand(wsPath, "log", "-1", "--format=%h|%s|%cr")
	if err != nil {
		return "", "", ""
	}
	parts := strings.SplitN(out, "|", 3)
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

func runGitCommand(wsPath string, args ...string) (string, error) {
	// Use -c safe.directory to handle mounted volumes with different ownership
	fullArgs := append([]string{"-c", "safe.directory=" + wsPath, "-C", wsPath}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
