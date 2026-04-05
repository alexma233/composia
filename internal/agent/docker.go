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
	agentv1connect "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

type dockerServer struct{}

func (s *dockerServer) ListContainers(ctx context.Context, _ *connect.Request[agentv1.ListContainersRequest]) (*connect.Response[agentv1.ListContainersResponse], error) {
	output, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}

	var containers []*agentv1.ContainerInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	imageIDMap := make(map[string]string)
	inspectOutput, _ := exec.CommandContext(ctx, "docker", "images", "--format", "{{.ID}}\t{{.Repository}}:{{.Tag}}").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(inspectOutput)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			imageIDMap[parts[1]] = parts[0]
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var psData struct {
			ID       string `json:"ID"`
			Names    string `json:"Names"`
			Image    string `json:"Image"`
			State    string `json:"State"`
			Status   string `json:"Status"`
			Created  string `json:"CreatedAt"`
			Labels   string `json:"Labels"`
			Ports    string `json:"Ports"`
			Networks string `json:"Networks"`
			ImageID  string `json:"ImageID"`
			Command  string `json:"Command"`
		}
		if err := json.Unmarshal([]byte(line), &psData); err != nil {
			continue
		}
		labels := parseLabels(psData.Labels)
		ports := parsePorts(psData.Ports)
		networks := parseList(psData.Networks)
		imageID := psData.ImageID
		if imageID == "" {
			imageID = imageIDMap[psData.Image]
		}

		createdTime := parseDockerCreatedAt(psData.Created)

		containers = append(containers, &agentv1.ContainerInfo{
			Id:       psData.ID,
			Name:     strings.Split(psData.Names, ",")[0],
			Image:    psData.Image,
			State:    psData.State,
			Status:   psData.Status,
			Created:  createdTime,
			Labels:   labels,
			Ports:    ports,
			Networks: networks,
			ImageId:  imageID,
		})
	}

	return connect.NewResponse(&agentv1.ListContainersResponse{
		Containers: containers,
	}), nil
}

func parseDockerCreatedAt(created string) string {
	// Try RFC3339 format first
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		return t.Format(time.RFC3339)
	}
	// Try "2006-01-02 15:04:05 -0700 MST" format
	if t, err := time.Parse("2006-01-02 15:04:05 -0700 MST", created); err == nil {
		return t.Format(time.RFC3339)
	}
	// Try "2006-01-02 15:04:05 MST" format
	if t, err := time.Parse("2006-01-02 15:04:05 MST", created); err == nil {
		return t.Format(time.RFC3339)
	}
	// Return as-is if parsing fails
	return created
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

	var networkIDs []string
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
		networkIDs = append(networkIDs, lsData.ID)
	}

	for i, network := range networks {
		inspectOutput, err := exec.CommandContext(ctx, "docker", "network", "inspect", "--format", "{{json .}}", network.Id).Output()
		if err != nil {
			continue
		}
		var inspectData struct {
			IPAM struct {
				Config []struct {
					Subnet  string `json:"Subnet"`
					Gateway string `json:"Gateway"`
				} `json:"Config"`
			} `json:"IPAM"`
			EnableIPv6 bool `json:"EnableIPv6"`
			Containers map[string]struct {
				Name string `json:"Name"`
			} `json:"Containers"`
		}
		if err := json.Unmarshal(inspectOutput, &inspectData); err == nil {
			if len(inspectData.IPAM.Config) > 0 {
				networks[i].Subnet = inspectData.IPAM.Config[0].Subnet
				networks[i].Gateway = inspectData.IPAM.Config[0].Gateway
			}
			networks[i].Ipv6Enabled = inspectData.EnableIPv6
			networks[i].ContainersCount = uint32(len(inspectData.Containers))
		}
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
		sizeBytes, containersCount, inUse := s.volumeUsageData(ctx, volData.Name)

		volumes = append(volumes, &agentv1.VolumeInfo{
			Name:            volData.Name,
			Driver:          volData.Driver,
			Mountpoint:      volData.Mountpoint,
			Scope:           volData.Scope,
			Created:         created,
			Labels:          labels,
			SizeBytes:       sizeBytes,
			ContainersCount: uint32(containersCount),
			InUse:           inUse,
		})
	}

	return connect.NewResponse(&agentv1.ListVolumesResponse{
		Volumes: volumes,
	}), nil
}

func (s *dockerServer) volumeUsageData(ctx context.Context, volumeName string) (int64, int, bool) {
	inspectOutput, err := exec.CommandContext(ctx, "docker", "volume", "inspect", "--format", "{{.UsageData.Size}}|{{len .UsageData.RefCount}}", volumeName).Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(inspectOutput)), "|")
		if len(parts) == 2 {
			var size int64
			fmt.Sscanf(parts[0], "%d", &size)
			var count int
			fmt.Sscanf(parts[1], "%d", &count)
			return size, count, count > 0
		}
	}

	containersOutput, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", "volume="+volumeName, "--format", "{{.ID}}").Output()
	if err == nil {
		count := len(strings.Split(strings.TrimSpace(string(containersOutput)), "\n"))
		if count == 0 || (count == 1 && strings.TrimSpace(string(containersOutput)) == "") {
			return 0, 0, false
		}
		return 0, count, true
	}
	return 0, 0, false
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

	imageUsage := make(map[string]int)
	containersOutput, _ := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.Image}}").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(containersOutput)), "\n") {
		imageName := strings.TrimSpace(line)
		if imageName != "" {
			imageUsage[imageName]++
		}
	}

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
			Digest     string `json:"Digest"`
		}
		if err := json.Unmarshal([]byte(line), &imgData); err != nil {
			continue
		}
		var repoTags []string
		var repoDigests []string
		isDangling := false

		if imgData.Repository == "<none>" {
			isDangling = true
		} else {
			if imgData.Tag != "<none>" {
				repoTags = append(repoTags, imgData.Repository+":"+imgData.Tag)
			}
			if imgData.Digest != "" && imgData.Digest != "<none>" {
				repoDigests = append(repoDigests, imgData.Repository+"@"+imgData.Digest)
			}
		}

		size := parseDockerSize(imgData.Size)
		created, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", imgData.CreatedAt)
		if created.IsZero() {
			created, _ = time.Parse(time.RFC3339, imgData.CreatedAt)
		}

		var containersCount uint32
		for _, tag := range repoTags {
			containersCount += uint32(imageUsage[tag])
		}

		images = append(images, &agentv1.ImageInfo{
			Id:              imgData.ID,
			RepoTags:        repoTags,
			Size:            size,
			Created:         created.Format(time.RFC3339),
			RepoDigests:     repoDigests,
			VirtualSize:     0,
			Architecture:    "",
			Os:              "",
			ContainersCount: containersCount,
			IsDangling:      isDangling,
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

func parsePorts(portsStr string) []string {
	if portsStr == "" {
		return nil
	}
	var ports []string
	for _, p := range strings.Split(portsStr, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			ports = append(ports, p)
		}
	}
	return ports
}

func parseList(listStr string) []string {
	if listStr == "" {
		return nil
	}
	var items []string
	for _, item := range strings.Split(listStr, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
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

type dockerListParams struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	ID       string `json:"id,omitempty"`
}

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
	var args []string
	switch resource {
	case "containers":
		args = []string{"ps", "-a", "--format", "{{json .}}"}
	case "networks":
		args = []string{"network", "ls", "--format", "{{json .}}"}
	case "volumes":
		args = []string{"volume", "ls", "--format", "{{json .}}"}
	case "images":
		args = []string{"images", "--format", "{{json .}}"}
	default:
		return fmt.Errorf("unknown resource type: %s", resource)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if outStr := string(output); outStr != "" {
		if logErr := uploadLog(outStr); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker %s list failed: %w", resource, err)
	}
	return nil
}

func runDockerInspect(ctx context.Context, resource, id string, uploadLog func(string) error) error {
	if id == "" {
		return fmt.Errorf("inspect requires an id")
	}

	var args []string
	switch resource {
	case "container":
		args = []string{"inspect", id}
	case "network":
		args = []string{"network", "inspect", id}
	case "volume":
		args = []string{"volume", "inspect", id}
	case "image":
		args = []string{"image", "inspect", id}
	default:
		return fmt.Errorf("unknown resource type: %s", resource)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if outStr := string(output); outStr != "" {
		if logErr := uploadLog(outStr); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker %s inspect failed: %w", resource, err)
	}
	return nil
}
