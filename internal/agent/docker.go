package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
)

type dockerServer struct{}

func (s *dockerServer) ListContainers(ctx context.Context, _ *connect.Request[agentv1.ListContainersRequest]) (*connect.Response[agentv1.ListContainersResponse], error) {
	output, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}

	var containers []*agentv1.ContainerInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var psData struct {
			ID      string `json:"ID"`
			Names   string `json:"Names"`
			Image   string `json:"Image"`
			State   string `json:"State"`
			Status  string `json:"Status"`
			Created string `json:"CreatedAt"`
			Labels  string `json:"Labels"`
		}
		if err := json.Unmarshal([]byte(line), &psData); err != nil {
			continue
		}
		labels := parseLabels(psData.Labels)
		containers = append(containers, &agentv1.ContainerInfo{
			Id:      psData.ID,
			Name:    psData.Names,
			Image:   psData.Image,
			State:   psData.State,
			Status:  psData.Status,
			Created: psData.Created,
			Labels:  labels,
		})
	}

	return connect.NewResponse(&agentv1.ListContainersResponse{
		Containers: containers,
	}), nil
}

func (s *dockerServer) InspectContainer(ctx context.Context, req *connect.Request[agentv1.InspectContainerRequest]) (*connect.Response[agentv1.InspectContainerResponse], error) {
	if req.Msg.GetContainerId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("container_id is required"))
	}

	output, err := exec.CommandContext(ctx, "docker", "inspect", req.Msg.GetContainerId()).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect: %w", err)
	}

	return connect.NewResponse(&agentv1.InspectContainerResponse{
		RawJson: string(output),
	}), nil
}

func (s *dockerServer) ListNetworks(ctx context.Context, _ *connect.Request[agentv1.ListNetworksRequest]) (*connect.Response[agentv1.ListNetworksResponse], error) {
	output, err := exec.CommandContext(ctx, "docker", "network", "ls", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker network ls: %w", err)
	}

	var networks []*agentv1.NetworkInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var lsData struct {
			ID         string `json:"ID"`
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Scope      string `json:"Scope"`
			Internal   string `json:"Internal"`
			Attachable string `json:"Attachable"`
			Labels     string `json:"Labels"`
			CreatedAt  string `json:"CreatedAt"`
		}
		if err := json.Unmarshal([]byte(line), &lsData); err != nil {
			continue
		}
		labels := parseLabels(lsData.Labels)
		networks = append(networks, &agentv1.NetworkInfo{
			Id:         lsData.ID,
			Name:       lsData.Name,
			Driver:     lsData.Driver,
			Scope:      lsData.Scope,
			Internal:   lsData.Internal == "true",
			Attachable: lsData.Attachable == "true",
			Created:    lsData.CreatedAt,
			Labels:     labels,
		})
	}

	return connect.NewResponse(&agentv1.ListNetworksResponse{
		Networks: networks,
	}), nil
}

func (s *dockerServer) InspectNetwork(ctx context.Context, req *connect.Request[agentv1.InspectNetworkRequest]) (*connect.Response[agentv1.InspectNetworkResponse], error) {
	if req.Msg.GetNetworkId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("network_id is required"))
	}

	output, err := exec.CommandContext(ctx, "docker", "network", "inspect", req.Msg.GetNetworkId()).Output()
	if err != nil {
		return nil, fmt.Errorf("docker network inspect: %w", err)
	}

	return connect.NewResponse(&agentv1.InspectNetworkResponse{
		RawJson: string(output),
	}), nil
}

func (s *dockerServer) ListVolumes(ctx context.Context, _ *connect.Request[agentv1.ListVolumesRequest]) (*connect.Response[agentv1.ListVolumesResponse], error) {
	output, err := exec.CommandContext(ctx, "docker", "volume", "ls", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker volume ls: %w", err)
	}

	var volumes []*agentv1.VolumeInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var volData struct {
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Mountpoint string `json:"Mountpoint"`
			Scope      string `json:"Scope"`
			Labels     string `json:"Labels"`
		}
		if err := json.Unmarshal([]byte(line), &volData); err != nil {
			continue
		}
		labels := parseLabels(volData.Labels)

		created, _ := s.volumeCreatedTime(ctx, volData.Name)

		volumes = append(volumes, &agentv1.VolumeInfo{
			Name:       volData.Name,
			Driver:     volData.Driver,
			Mountpoint: volData.Mountpoint,
			Scope:      volData.Scope,
			Created:    created,
			Labels:     labels,
		})
	}

	return connect.NewResponse(&agentv1.ListVolumesResponse{
		Volumes: volumes,
	}), nil
}

func (s *dockerServer) volumeCreatedTime(ctx context.Context, volumeName string) (string, error) {
	output, err := exec.CommandContext(ctx, "docker", "volume", "inspect", "-f", "{{.CreatedAt}}", volumeName).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *dockerServer) InspectVolume(ctx context.Context, req *connect.Request[agentv1.InspectVolumeRequest]) (*connect.Response[agentv1.InspectVolumeResponse], error) {
	if req.Msg.GetVolumeName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("volume_name is required"))
	}

	output, err := exec.CommandContext(ctx, "docker", "volume", "inspect", req.Msg.GetVolumeName()).Output()
	if err != nil {
		return nil, fmt.Errorf("docker volume inspect: %w", err)
	}

	return connect.NewResponse(&agentv1.InspectVolumeResponse{
		RawJson: string(output),
	}), nil
}

func (s *dockerServer) ListImages(ctx context.Context, _ *connect.Request[agentv1.ListImagesRequest]) (*connect.Response[agentv1.ListImagesResponse], error) {
	output, err := exec.CommandContext(ctx, "docker", "images", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker images: %w", err)
	}

	var images []*agentv1.ImageInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var imgData struct {
			ID         string `json:"ID"`
			Repository string `json:"Repository"`
			Tag        string `json:"Tag"`
			Size       string `json:"Size"`
			CreatedAt  string `json:"CreatedAt"`
		}
		if err := json.Unmarshal([]byte(line), &imgData); err != nil {
			continue
		}
		var repoTags []string
		if imgData.Repository != "<none>" && imgData.Tag != "<none>" {
			repoTags = append(repoTags, imgData.Repository+":"+imgData.Tag)
		} else if imgData.Repository != "<none>" {
			repoTags = append(repoTags, imgData.Repository)
		}

		size := parseDockerSize(imgData.Size)
		created, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", imgData.CreatedAt)
		if created.IsZero() {
			created, _ = time.Parse(time.RFC3339, imgData.CreatedAt)
		}

		images = append(images, &agentv1.ImageInfo{
			Id:       imgData.ID,
			RepoTags: repoTags,
			Size:     size,
			Created:  created.Format(time.RFC3339),
		})
	}

	return connect.NewResponse(&agentv1.ListImagesResponse{
		Images: images,
	}), nil
}

func (s *dockerServer) InspectImage(ctx context.Context, req *connect.Request[agentv1.InspectImageRequest]) (*connect.Response[agentv1.InspectImageResponse], error) {
	if req.Msg.GetImageId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("image_id is required"))
	}

	output, err := exec.CommandContext(ctx, "docker", "image", "inspect", req.Msg.GetImageId()).Output()
	if err != nil {
		return nil, fmt.Errorf("docker image inspect: %w", err)
	}

	return connect.NewResponse(&agentv1.InspectImageResponse{
		RawJson: string(output),
	}), nil
}

func parseLabels(labelsStr string) map[string]string {
	labels := make(map[string]string)
	if labelsStr == "" {
		return labels
	}
	parts := strings.Split(labelsStr, ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			labels[kv[0]] = kv[1]
		}
	}
	return labels
}

func parseDockerSize(sizeStr string) int64 {
	sizeStr = strings.TrimSpace(sizeStr)
	sizeStr = strings.ToUpper(sizeStr)

	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "B") {
		sizeStr = strings.TrimSuffix(sizeStr, "B")
	}

	sizeStr = strings.TrimSpace(sizeStr)
	var size int64
	for _, c := range sizeStr {
		if c >= '0' && c <= '9' {
			size = size*10 + int64(c-'0')
		}
	}
	return size * multiplier
}
