package agent

import (
	"context"
	"fmt"
	"io"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

type DockerClient struct {
	cli *client.Client
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &DockerClient{cli: cli}, nil
}

func (d *DockerClient) Close() error {
	if d.cli != nil {
		return d.cli.Close()
	}
	return nil
}

func (d *DockerClient) ContainerList(ctx context.Context) ([]container.Summary, error) {
	list, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return list.Items, nil
}

func (d *DockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	c, err := d.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{Size: true})
	if err != nil {
		return container.InspectResponse{}, fmt.Errorf("failed to inspect container: %w", err)
	}
	return c.Container, nil
}

func (d *DockerClient) ContainerStart(ctx context.Context, containerID string) error {
	if _, err := d.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return nil
}

func (d *DockerClient) ContainerStop(ctx context.Context, containerID string) error {
	if _, err := d.cli.ContainerStop(ctx, containerID, client.ContainerStopOptions{}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	return nil
}

func (d *DockerClient) ContainerRestart(ctx context.Context, containerID string) error {
	if _, err := d.cli.ContainerRestart(ctx, containerID, client.ContainerRestartOptions{}); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}
	return nil
}

func (d *DockerClient) ContainerRemove(ctx context.Context, containerID string, force, removeVolumes bool) error {
	if _, err := d.cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: force, RemoveVolumes: removeVolumes}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	return nil
}

func (d *DockerClient) ContainerLogs(ctx context.Context, containerID, tail string, timestamps bool) (io.ReadCloser, error) {
	logs, err := d.cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Timestamps: timestamps,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}
	return logs, nil
}

func (d *DockerClient) ImageList(ctx context.Context) ([]image.Summary, error) {
	list, err := d.cli.ImageList(ctx, client.ImageListOptions{All: true, SharedSize: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	return list.Items, nil
}

func (d *DockerClient) ImageInspect(ctx context.Context, imageID string) (client.ImageInspectResult, error) {
	inspect, err := d.cli.ImageInspect(ctx, imageID)
	if err != nil {
		return client.ImageInspectResult{}, fmt.Errorf("failed to inspect image: %w", err)
	}
	return inspect, nil
}

func (d *DockerClient) ImageRemove(ctx context.Context, imageID string, force bool) error {
	if _, err := d.cli.ImageRemove(ctx, imageID, client.ImageRemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}
	return nil
}

func (d *DockerClient) NetworkList(ctx context.Context) ([]network.Inspect, error) {
	list, err := d.cli.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var result []network.Inspect
	for _, net := range list.Items {
		inspect, err := d.cli.NetworkInspect(ctx, net.ID, client.NetworkInspectOptions{})
		if err != nil {
			continue
		}
		result = append(result, inspect.Network)
	}
	return result, nil
}

func (d *DockerClient) NetworkInspect(ctx context.Context, networkID string) (network.Inspect, error) {
	inspect, err := d.cli.NetworkInspect(ctx, networkID, client.NetworkInspectOptions{})
	if err != nil {
		return network.Inspect{}, fmt.Errorf("failed to inspect network: %w", err)
	}
	return inspect.Network, nil
}

func (d *DockerClient) NetworkRemove(ctx context.Context, networkID string) error {
	if _, err := d.cli.NetworkRemove(ctx, networkID, client.NetworkRemoveOptions{}); err != nil {
		return fmt.Errorf("failed to remove network: %w", err)
	}
	return nil
}

func (d *DockerClient) VolumeList(ctx context.Context) ([]volume.Volume, error) {
	list, err := d.cli.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}
	return list.Items, nil
}

func (d *DockerClient) VolumeInspect(ctx context.Context, volumeName string) (volume.Volume, error) {
	result, err := d.cli.VolumeInspect(ctx, volumeName, client.VolumeInspectOptions{})
	if err != nil {
		return volume.Volume{}, fmt.Errorf("failed to inspect volume: %w", err)
	}
	return result.Volume, nil
}

func (d *DockerClient) VolumeRemove(ctx context.Context, volumeName string) error {
	if _, err := d.cli.VolumeRemove(ctx, volumeName, client.VolumeRemoveOptions{}); err != nil {
		return fmt.Errorf("failed to remove volume: %w", err)
	}
	return nil
}

func (d *DockerClient) DiskUsage(ctx context.Context) (client.DiskUsageResult, error) {
	du, err := d.cli.DiskUsage(ctx, client.DiskUsageOptions{
		Volumes: true,
		Verbose: true,
	})
	if err != nil {
		return client.DiskUsageResult{}, fmt.Errorf("failed to get disk usage: %w", err)
	}
	return du, nil
}
