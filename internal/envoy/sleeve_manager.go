package envoy

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/hotschmoe/protectorate/internal/config"
)

var namePool = []string{
	"quell", "virginia", "rei", "mickey", "trepp",
	"tanaka", "athena", "apollo", "hermes", "iris", "prometheus",
}

type SleeveInfo struct {
	Name         string    `json:"name"`
	ContainerID  string    `json:"container_id"`
	Workspace    string    `json:"workspace"`
	TTYDPort     int       `json:"ttyd_port"`
	TTYDAddress  string    `json:"ttyd_address"`
	SpawnTime    time.Time `json:"spawn_time"`
	Status       string    `json:"status"`
}

type SpawnSleeveRequest struct {
	Workspace string `json:"workspace"`
	Name      string `json:"name,omitempty"`
}

type SleeveManager struct {
	mu       sync.RWMutex
	docker   *DockerClient
	cfg      *config.EnvoyConfig
	sleeves  map[string]*SleeveInfo
	usedNames map[string]bool
	nextPort int
}

func NewSleeveManager(docker *DockerClient, cfg *config.EnvoyConfig) *SleeveManager {
	return &SleeveManager{
		docker:    docker,
		cfg:       cfg,
		sleeves:   make(map[string]*SleeveInfo),
		usedNames: make(map[string]bool),
		nextPort:  7681,
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

func (m *SleeveManager) allocatePort() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	port := m.nextPort
	m.nextPort++
	return port
}

func (m *SleeveManager) toHostPath(containerPath string) string {
	wsRoot := m.cfg.Docker.WorkspaceRoot
	wsHostRoot := m.cfg.Docker.WorkspaceHostRoot
	if wsRoot != "" && wsHostRoot != "" && strings.HasPrefix(containerPath, wsRoot) {
		return wsHostRoot + strings.TrimPrefix(containerPath, wsRoot)
	}
	return containerPath
}

func (m *SleeveManager) Spawn(req SpawnSleeveRequest) (*SleeveInfo, error) {
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
	port := m.allocatePort()

	if err := m.docker.EnsureNetwork(m.cfg.Docker.Network); err != nil {
		m.releaseName(name)
		return nil, fmt.Errorf("failed to ensure network: %w", err)
	}

	cfg := &container.Config{
		Image: m.cfg.Docker.SleeveImage,
		ExposedPorts: nat.PortSet{
			"7681/tcp": struct{}{},
		},
	}

	workspaceHostPath := m.toHostPath(req.Workspace)

	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   workspaceHostPath,
			Target:   "/workspace",
			ReadOnly: false,
		},
	}

	if m.cfg.Docker.CredentialsHostPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.cfg.Docker.CredentialsHostPath,
			Target:   "/root/.claude/.credentials.json",
			ReadOnly: true,
		})
	}

	if m.cfg.Docker.SettingsHostPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.cfg.Docker.SettingsHostPath,
			Target:   "/root/.claude.json",
			ReadOnly: true,
		})
	}

	hostCfg := &container.HostConfig{
		Mounts: mounts,
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

	sleeve := &SleeveInfo{
		Name:        name,
		ContainerID: containerID[:12],
		Workspace:   req.Workspace,
		TTYDPort:    port,
		TTYDAddress: fmt.Sprintf("%s:7681", containerName),
		SpawnTime:   time.Now(),
		Status:      "running",
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

func (m *SleeveManager) List() []*SleeveInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SleeveInfo, 0, len(m.sleeves))
	for _, s := range m.sleeves {
		result = append(result, s)
	}
	return result
}

func (m *SleeveManager) Get(name string) (*SleeveInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sleeve, ok := m.sleeves[name]
	if !ok {
		return nil, fmt.Errorf("sleeve %q not found", name)
	}
	return sleeve, nil
}
