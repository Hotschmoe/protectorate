package envoy

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type DockerClient struct {
	cli *client.Client
}

type ContainerInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	State   string   `json:"state"`
	Status  string   `json:"status"`
	Ports   []string `json:"ports"`
	Created int64    `json:"created"`
}

type NetworkInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &DockerClient{cli: cli}, nil
}

func (d *DockerClient) ListContainers() ([]ContainerInfo, error) {
	ctx := context.Background()

	containers, err := d.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]ContainerInfo, 0, len(containers))
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0][1:]
		}

		ports := make([]string, 0, len(c.Ports))
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, formatPort(p))
			}
		}

		result = append(result, ContainerInfo{
			ID:      c.ID[:12],
			Name:    name,
			Image:   c.Image,
			State:   c.State,
			Status:  c.Status,
			Ports:   ports,
			Created: c.Created,
		})
	}

	return result, nil
}

func formatPort(p types.Port) string {
	if p.IP != "" {
		return fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type)
	}
	return fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type)
}

func (d *DockerClient) ListNetworks() ([]NetworkInfo, error) {
	ctx := context.Background()

	networks, err := d.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]NetworkInfo, 0, len(networks))
	for _, n := range networks {
		result = append(result, NetworkInfo{
			ID:     n.ID[:12],
			Name:   n.Name,
			Driver: n.Driver,
			Scope:  n.Scope,
		})
	}

	return result, nil
}

func (d *DockerClient) CreateContainer(name, image string, config *container.Config, hostConfig *container.HostConfig, networkConfig *network.NetworkingConfig) (string, error) {
	ctx := context.Background()

	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, name)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (d *DockerClient) StartContainer(id string) error {
	ctx := context.Background()
	return d.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (d *DockerClient) StopContainer(id string) error {
	ctx := context.Background()
	return d.cli.ContainerStop(ctx, id, container.StopOptions{})
}

func (d *DockerClient) RemoveContainer(id string) error {
	ctx := context.Background()
	return d.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (d *DockerClient) GetContainerByName(name string) (*types.Container, error) {
	ctx := context.Background()

	f := filters.NewArgs()
	f.Add("name", name)

	containers, err := d.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		for _, n := range c.Names {
			if n == "/"+name {
				return &c, nil
			}
		}
	}

	return nil, nil
}

func (d *DockerClient) EnsureNetwork(name string) error {
	ctx := context.Background()

	networks, err := d.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}

	for _, n := range networks {
		if n.Name == name {
			return nil
		}
	}

	_, err = d.cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge"})
	return err
}
