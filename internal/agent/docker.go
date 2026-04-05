package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	agentv1connect "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"github.com/moby/moby/api/types/mount"
)

const (
	dockerTaskResultBegin = "COMPOSIA_DOCKER_RESULT_BEGIN"
	dockerTaskResultEnd   = "COMPOSIA_DOCKER_RESULT_END"
)

type dockerTaskResult struct {
	Containers []*agentv1.ContainerInfo `json:"containers,omitempty"`
	Networks   []*agentv1.NetworkInfo   `json:"networks,omitempty"`
	Volumes    []*agentv1.VolumeInfo    `json:"volumes,omitempty"`
	Images     []*agentv1.ImageInfo     `json:"images,omitempty"`
	RawJSON    string                   `json:"raw_json,omitempty"`
}

type dockerServer struct {
	client *DockerClient
}

func newDockerServer() (*dockerServer, error) {
	cli, err := NewDockerClient()
	if err != nil {
		return nil, err
	}
	return &dockerServer{client: cli}, nil
}

func (s *dockerServer) ListContainers(ctx context.Context, _ *connect.Request[agentv1.ListContainersRequest]) (*connect.Response[agentv1.ListContainersResponse], error) {
	containers, err := s.client.ContainerList(ctx)
	if err != nil {
		return nil, err
	}

	var result []*agentv1.ContainerInfo
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
			// Remove leading slash
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}

		var ports []string
		seenPorts := make(map[string]struct{})
		for _, p := range c.Ports {
			portStr := ""
			if p.PublicPort != 0 {
				portStr = fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type)
			} else {
				portStr = fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
			}
			if _, ok := seenPorts[portStr]; ok {
				continue
			}
			seenPorts[portStr] = struct{}{}
			ports = append(ports, portStr)
		}

		var networks []string
		for n := range c.NetworkSettings.Networks {
			networks = append(networks, n)
		}

		result = append(result, &agentv1.ContainerInfo{
			Id:       c.ID,
			Name:     name,
			Image:    c.Image,
			State:    string(c.State),
			Status:   c.Status,
			Created:  time.Unix(c.Created, 0).Format(time.RFC3339),
			Labels:   c.Labels,
			Ports:    ports,
			Networks: networks,
			ImageId:  c.ImageID,
		})
	}

	return connect.NewResponse(&agentv1.ListContainersResponse{
		Containers: result,
	}), nil
}

func (s *dockerServer) InspectContainer(ctx context.Context, req *connect.Request[agentv1.InspectContainerRequest]) (*connect.Response[agentv1.InspectContainerResponse], error) {
	if req.Msg.GetContainerId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("container_id is required"))
	}

	c, err := s.client.ContainerInspect(ctx, req.Msg.GetContainerId())
	if err != nil {
		return nil, err
	}

	// Convert to JSON
	jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal container: %w", err)
	}

	return connect.NewResponse(&agentv1.InspectContainerResponse{
		RawJson: string(jsonData),
	}), nil
}

func (s *dockerServer) ListNetworks(ctx context.Context, _ *connect.Request[agentv1.ListNetworksRequest]) (*connect.Response[agentv1.ListNetworksResponse], error) {
	networks, err := s.client.NetworkList(ctx)
	if err != nil {
		return nil, err
	}

	var result []*agentv1.NetworkInfo
	for _, n := range networks {
		subnet := ""
		gateway := ""
		if len(n.IPAM.Config) > 0 {
			subnet = n.IPAM.Config[0].Subnet.String()
			gateway = n.IPAM.Config[0].Gateway.String()
		}

		result = append(result, &agentv1.NetworkInfo{
			Id:              n.ID,
			Name:            n.Name,
			Driver:          n.Driver,
			Scope:           n.Scope,
			Internal:        n.Internal,
			Attachable:      n.Attachable,
			Created:         n.Created.Format(time.RFC3339),
			Labels:          n.Labels,
			Subnet:          subnet,
			Gateway:         gateway,
			ContainersCount: uint32(len(n.Containers)),
			Ipv6Enabled:     n.EnableIPv6,
		})
	}

	return connect.NewResponse(&agentv1.ListNetworksResponse{
		Networks: result,
	}), nil
}

func (s *dockerServer) InspectNetwork(ctx context.Context, req *connect.Request[agentv1.InspectNetworkRequest]) (*connect.Response[agentv1.InspectNetworkResponse], error) {
	if req.Msg.GetNetworkId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("network_id is required"))
	}

	n, err := s.client.NetworkInspect(ctx, req.Msg.GetNetworkId())
	if err != nil {
		return nil, err
	}

	// Convert to JSON
	jsonData, err := json.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal network: %w", err)
	}

	return connect.NewResponse(&agentv1.InspectNetworkResponse{
		RawJson: string(jsonData),
	}), nil
}

func (s *dockerServer) ListVolumes(ctx context.Context, _ *connect.Request[agentv1.ListVolumesRequest]) (*connect.Response[agentv1.ListVolumesResponse], error) {
	volumes, err := s.client.VolumeList(ctx)
	if err != nil {
		return nil, err
	}

	diskUsage, err := s.client.DiskUsage(ctx)
	if err != nil {
		// Continue even if disk usage fails
		fmt.Printf("Warning: failed to get disk usage: %v\n", err)
	}

	volumeMap := make(map[string]int64)
	if diskUsage.Volumes.Items != nil {
		for _, v := range diskUsage.Volumes.Items {
			if v.UsageData != nil {
				volumeMap[v.Name] = v.UsageData.Size
			}
		}
	}

	// Get containers using these volumes
	containersUsingVolumes := make(map[string]uint32)
	containerList, err := s.client.ContainerList(ctx)
	if err == nil {
		for _, c := range containerList {
			for _, m := range c.Mounts {
				if m.Type == mount.TypeVolume {
					containersUsingVolumes[m.Name]++
				}
			}
		}
	}

	var result []*agentv1.VolumeInfo
	for _, v := range volumes {
		size := volumeMap[v.Name]
		count := containersUsingVolumes[v.Name]

		result = append(result, &agentv1.VolumeInfo{
			Name:            v.Name,
			Driver:          v.Driver,
			Mountpoint:      v.Mountpoint,
			Scope:           v.Scope,
			Created:         v.CreatedAt,
			Labels:          v.Labels,
			SizeBytes:       size,
			ContainersCount: count,
			InUse:           count > 0,
		})
	}

	return connect.NewResponse(&agentv1.ListVolumesResponse{
		Volumes: result,
	}), nil
}

func (s *dockerServer) InspectVolume(ctx context.Context, req *connect.Request[agentv1.InspectVolumeRequest]) (*connect.Response[agentv1.InspectVolumeResponse], error) {
	if req.Msg.GetVolumeName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("volume_name is required"))
	}

	vol, err := s.client.VolumeInspect(ctx, req.Msg.GetVolumeName())
	if err != nil {
		return nil, err
	}

	diskUsage, err := s.client.DiskUsage(ctx)
	if err == nil {
		for _, usageVolume := range diskUsage.Volumes.Items {
			if usageVolume.Name == vol.Name {
				vol.UsageData = usageVolume.UsageData
				break
			}
		}
	}

	// Convert to JSON
	jsonData, err := json.Marshal(vol)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal volume: %w", err)
	}

	return connect.NewResponse(&agentv1.InspectVolumeResponse{
		RawJson: string(jsonData),
	}), nil
}

func (s *dockerServer) ListImages(ctx context.Context, _ *connect.Request[agentv1.ListImagesRequest]) (*connect.Response[agentv1.ListImagesResponse], error) {
	images, err := s.client.ImageList(ctx)
	if err != nil {
		return nil, err
	}

	var result []*agentv1.ImageInfo
	for _, img := range images {
		isDangling := len(img.RepoTags) == 0

		containersCount := uint32(0)
		if img.Containers >= 0 {
			containersCount = uint32(img.Containers)
		}

		arch := ""
		os := ""
		author := ""
		virtualSize := img.Size
		if img.Labels != nil {
			arch = img.Labels["org.opencontainers.image.architecture"]
			os = img.Labels["org.opencontainers.image.os"]
		}

		result = append(result, &agentv1.ImageInfo{
			Id:              img.ID,
			RepoTags:        img.RepoTags,
			Size:            img.Size,
			VirtualSize:     virtualSize,
			Created:         time.Unix(img.Created, 0).Format(time.RFC3339),
			RepoDigests:     img.RepoDigests,
			Architecture:    arch,
			Os:              os,
			Author:          author,
			ContainersCount: containersCount,
			IsDangling:      isDangling,
		})
	}

	return connect.NewResponse(&agentv1.ListImagesResponse{
		Images: result,
	}), nil
}

func (s *dockerServer) InspectImage(ctx context.Context, req *connect.Request[agentv1.InspectImageRequest]) (*connect.Response[agentv1.InspectImageResponse], error) {
	if req.Msg.GetImageId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("image_id is required"))
	}

	inspect, err := s.client.ImageInspect(ctx, req.Msg.GetImageId())
	if err != nil {
		return nil, err
	}

	// Convert to JSON string directly from the inspect result
	jsonData, err := json.Marshal(inspect)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal image inspect: %w", err)
	}

	return connect.NewResponse(&agentv1.InspectImageResponse{
		RawJson: string(jsonData),
	}), nil
}

// DockerTaskExecutor handles docker-related task execution (deprecated - kept for compatibility)
type DockerTaskExecutor struct{}

func (e *DockerTaskExecutor) ExecuteTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask) error {
	// Docker tasks are handled differently - they're for CLI execution
	// This is deprecated as we now use Docker SDK directly
	return nil
}

// executeDockerTask handles docker-related tasks from the controller
// This is called via the task queue system
func executeDockerTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params := parseDockerListParams(pulledTask.GetParamsJson())

	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting docker task: action=%s resource=%s id=%s\n", params.Action, params.Resource, params.ID)); err != nil {
		return err
	}

	var stepName string
	var execute func() error

	if params.Action == "list" {
		stepName = "docker_list"
		execute = func() error {
			return runDockerList(ctx, params.Resource, func(output string) error {
				return uploadTaskLog(ctx, logUploader, output)
			})
		}
	} else {
		stepName = "docker_inspect"
		execute = func() error {
			return runDockerInspect(ctx, params.Resource, params.ID, func(output string) error {
				return uploadTaskLog(ctx, logUploader, output)
			})
		}
	}

	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepName(stepName), execute); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}

	if err := uploadTaskLog(ctx, logUploader, "docker task finished successfully\n"); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

type dockerListParams struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	ID       string `json:"id,omitempty"`
}

func parseDockerListParams(paramsJSON string) dockerListParams {
	if paramsJSON == "" {
		return dockerListParams{Action: "list", Resource: "containers"}
	}
	var params dockerListParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return dockerListParams{Action: "list", Resource: "containers"}
	}
	if params.Action == "" {
		params.Action = "list"
	}
	if params.Resource == "" {
		params.Resource = "containers"
	}
	return params
}

func runDockerList(ctx context.Context, resource string, uploadLog func(string) error) error {
	server, err := newDockerServer()
	if err != nil {
		return err
	}
	defer func() {
		_ = server.client.Close()
	}()

	var payload dockerTaskResult
	switch resource {
	case "containers":
		resp, err := server.ListContainers(ctx, connect.NewRequest(&agentv1.ListContainersRequest{}))
		if err != nil {
			return err
		}
		payload.Containers = resp.Msg.Containers
	case "networks":
		resp, err := server.ListNetworks(ctx, connect.NewRequest(&agentv1.ListNetworksRequest{}))
		if err != nil {
			return err
		}
		payload.Networks = resp.Msg.Networks
	case "volumes":
		resp, err := server.ListVolumes(ctx, connect.NewRequest(&agentv1.ListVolumesRequest{}))
		if err != nil {
			return err
		}
		payload.Volumes = resp.Msg.Volumes
	case "images":
		resp, err := server.ListImages(ctx, connect.NewRequest(&agentv1.ListImagesRequest{}))
		if err != nil {
			return err
		}
		payload.Images = resp.Msg.Images
	default:
		return fmt.Errorf("unknown resource type: %s", resource)
	}

	return uploadDockerTaskResult(uploadLog, payload)
}

func runDockerInspect(ctx context.Context, resource, id string, uploadLog func(string) error) error {
	if id == "" {
		return fmt.Errorf("inspect requires an id")
	}

	server, err := newDockerServer()
	if err != nil {
		return err
	}
	defer func() {
		_ = server.client.Close()
	}()

	var payload dockerTaskResult
	switch resource {
	case "container":
		resp, err := server.InspectContainer(ctx, connect.NewRequest(&agentv1.InspectContainerRequest{ContainerId: id}))
		if err != nil {
			return err
		}
		payload.RawJSON = resp.Msg.GetRawJson()
	case "network":
		resp, err := server.InspectNetwork(ctx, connect.NewRequest(&agentv1.InspectNetworkRequest{NetworkId: id}))
		if err != nil {
			return err
		}
		payload.RawJSON = resp.Msg.GetRawJson()
	case "volume":
		resp, err := server.InspectVolume(ctx, connect.NewRequest(&agentv1.InspectVolumeRequest{VolumeName: id}))
		if err != nil {
			return err
		}
		payload.RawJSON = resp.Msg.GetRawJson()
	case "image":
		resp, err := server.InspectImage(ctx, connect.NewRequest(&agentv1.InspectImageRequest{ImageId: id}))
		if err != nil {
			return err
		}
		payload.RawJSON = resp.Msg.GetRawJson()
	default:
		return fmt.Errorf("unknown resource type: %s", resource)
	}

	return uploadDockerTaskResult(uploadLog, payload)
}

func uploadDockerTaskResult(uploadLog func(string) error, payload dockerTaskResult) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal docker task result: %w", err)
	}
	if err := uploadLog(dockerTaskResultBegin + "\n"); err != nil {
		return err
	}
	if err := uploadLog(string(encoded) + "\n"); err != nil {
		return err
	}
	if err := uploadLog(dockerTaskResultEnd + "\n"); err != nil {
		return err
	}
	return nil
}
