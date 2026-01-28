package envoy

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/hotschmoe/protectorate/internal/config"
	"github.com/hotschmoe/protectorate/internal/protocol"
)

var namePool = []string{
	"quell", "virginia", "rei", "mickey", "trepp",
	"tanaka", "athena", "apollo", "hermes", "iris", "prometheus",
}

type SleeveManager struct {
	mu        sync.RWMutex
	docker    *DockerClient
	cfg       *config.EnvoyConfig
	sleeves   map[string]*protocol.SleeveInfo
	usedNames map[string]bool
}

func NewSleeveManager(docker *DockerClient, cfg *config.EnvoyConfig) *SleeveManager {
	return &SleeveManager{
		docker:    docker,
		cfg:       cfg,
		sleeves:   make(map[string]*protocol.SleeveInfo),
		usedNames: make(map[string]bool),
	}
}

func (m *SleeveManager) allocateName() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, name := range namePool {
		if !m.usedNames[name] {
			m.usedNames[name] = true
			return name
		}
	}

	name := fmt.Sprintf("sleeve-%d", time.Now().UnixNano())
	m.usedNames[name] = true
	return name
}

func (m *SleeveManager) releaseName(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.usedNames, name)
}

func (m *SleeveManager) toHostPath(containerPath string) string {
	wsRoot := m.cfg.Docker.WorkspaceRoot
	wsHostRoot := m.cfg.Docker.WorkspaceHostRoot
	if wsRoot != "" && wsHostRoot != "" && strings.HasPrefix(containerPath, wsRoot) {
		return wsHostRoot + strings.TrimPrefix(containerPath, wsRoot)
	}
	return containerPath
}

func (m *SleeveManager) Spawn(req protocol.SpawnSleeveRequest) (*protocol.SleeveInfo, error) {
	workspace := req.Workspace

	if workspace == "" {
		return nil, fmt.Errorf("workspace path required")
	}

	if _, err := os.Stat(workspace); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace %q does not exist", workspace)
	}

	name := req.Name
	if name == "" {
		name = m.allocateName()
	} else {
		m.mu.Lock()
		if m.usedNames[name] {
			m.mu.Unlock()
			return nil, fmt.Errorf("sleeve name %q already in use", name)
		}
		m.usedNames[name] = true
		m.mu.Unlock()
	}

	containerName := "sleeve-" + name

	if err := m.docker.EnsureNetwork(m.cfg.Docker.Network); err != nil {
		m.releaseName(name)
		return nil, fmt.Errorf("failed to ensure network: %w", err)
	}

	constrained := req.MemoryLimitMB > 0 || req.CPULimit > 0

	labels := map[string]string{
		"protectorate.sleeve":    "true",
		"protectorate.name":      name,
		"protectorate.workspace": workspace,
	}
	if req.MemoryLimitMB > 0 {
		labels["protectorate.constrained"] = "true"
		labels["protectorate.memory_limit_mb"] = strconv.FormatInt(req.MemoryLimitMB, 10)
	}
	if req.CPULimit > 0 {
		labels["protectorate.constrained"] = "true"
		labels["protectorate.cpu_limit"] = strconv.Itoa(req.CPULimit)
	}

	cfg := &container.Config{
		Image:  m.cfg.Docker.SleeveImage,
		Labels: labels,
	}

	workspaceHostPath := m.toHostPath(workspace)

	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   workspaceHostPath,
			Target:   "/home/claude/workspace",
			ReadOnly: false,
		},
	}

	if m.cfg.Docker.CredentialsHostPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.cfg.Docker.CredentialsHostPath,
			Target:   "/home/claude/.claude/.credentials.json",
			ReadOnly: true,
		})
	}

	if m.cfg.Docker.SettingsHostPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.cfg.Docker.SettingsHostPath,
			Target:   "/etc/claude/settings.json",
			ReadOnly: true,
		})
	}

	if m.cfg.Docker.PluginsHostPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.cfg.Docker.PluginsHostPath,
			Target:   "/home/claude/.claude/plugins",
			ReadOnly: true,
		})
	}

	hostCfg := &container.HostConfig{
		Mounts: mounts,
	}

	if req.MemoryLimitMB > 0 || req.CPULimit > 0 {
		hostCfg.Resources = container.Resources{
			Memory:   req.MemoryLimitMB * 1024 * 1024,
			NanoCPUs: int64(req.CPULimit) * 1e9,
		}
	}

	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			m.cfg.Docker.Network: {},
		},
	}

	containerID, err := m.docker.CreateContainer(containerName, m.cfg.Docker.SleeveImage, cfg, hostCfg, netCfg)
	if err != nil {
		m.releaseName(name)
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := m.docker.StartContainer(containerID); err != nil {
		m.docker.RemoveContainer(containerID)
		m.releaseName(name)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	sleeve := &protocol.SleeveInfo{
		Name:          name,
		ContainerID:   containerID[:12],
		ContainerName: containerName,
		Workspace:     workspace,
		SpawnTime:     time.Now(),
		Status:        "running",
		Constrained:   constrained,
		MemoryLimitMB: req.MemoryLimitMB,
		CPULimit:      req.CPULimit,
	}

	m.mu.Lock()
	m.sleeves[name] = sleeve
	m.mu.Unlock()

	return sleeve, nil
}

func (m *SleeveManager) Kill(name string) error {
	m.mu.RLock()
	sleeve, ok := m.sleeves[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("sleeve %q not found", name)
	}

	containerName := "sleeve-" + name

	c, err := m.docker.GetContainerByName(containerName)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	if c != nil {
		if err := m.docker.StopContainer(c.ID); err != nil {
			// Continue to remove even if stop fails
		}
		if err := m.docker.RemoveContainer(c.ID); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}

	m.mu.Lock()
	delete(m.sleeves, name)
	m.mu.Unlock()

	m.releaseName(sleeve.Name)

	return nil
}

func (m *SleeveManager) List() []*protocol.SleeveInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*protocol.SleeveInfo, 0, len(m.sleeves))
	for _, s := range m.sleeves {
		result = append(result, s)
	}
	return result
}

func (m *SleeveManager) Get(name string) (*protocol.SleeveInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sleeve, ok := m.sleeves[name]
	if !ok {
		return nil, fmt.Errorf("sleeve %q not found", name)
	}
	return sleeve, nil
}

// CalculateIntegrity returns stack integrity as a percentage
// Based on (closed/total)*100 from cstack stats
// Returns 100 if no cstack or no tasks
func (m *SleeveManager) CalculateIntegrity(workspace string) float64 {
	stats := getCstackInfo(workspace)
	if stats == nil || !stats.Exists || stats.Total == 0 {
		return 100.0
	}
	return float64(stats.Closed) / float64(stats.Total) * 100.0
}

func (m *SleeveManager) RecoverSleeves() error {
	containers, err := m.docker.ListSleeveContainers()
	if err != nil {
		return fmt.Errorf("failed to list sleeve containers: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	recovered := 0
	for _, c := range containers {
		name := c.Labels["protectorate.name"]
		workspace := c.Labels["protectorate.workspace"]

		if name == "" {
			continue
		}

		if _, exists := m.sleeves[name]; exists {
			continue
		}

		containerName := "sleeve-" + name

		var status string
		switch c.State {
		case "running":
			status = "running"
		case "exited":
			status = "stopped"
		default:
			status = c.State
		}

		constrained := c.Labels["protectorate.constrained"] == "true"
		var memoryLimitMB int64
		var cpuLimit int
		if memStr := c.Labels["protectorate.memory_limit_mb"]; memStr != "" {
			memoryLimitMB, _ = strconv.ParseInt(memStr, 10, 64)
		}
		if cpuStr := c.Labels["protectorate.cpu_limit"]; cpuStr != "" {
			cpu, _ := strconv.Atoi(cpuStr)
			cpuLimit = cpu
		}

		sleeve := &protocol.SleeveInfo{
			Name:          name,
			ContainerID:   c.ID[:12],
			ContainerName: containerName,
			Workspace:     workspace,
			SpawnTime:     time.Unix(c.Created, 0),
			Status:        status,
			Constrained:   constrained,
			MemoryLimitMB: memoryLimitMB,
			CPULimit:      cpuLimit,
		}

		m.sleeves[name] = sleeve
		m.usedNames[name] = true
		recovered++
	}

	if recovered > 0 {
		log.Printf("recovered %d existing sleeve(s) from Docker", recovered)
	}

	return nil
}
