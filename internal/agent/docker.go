package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	agentv1connect "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/mount"
)

type dockerTaskResult struct {
	Containers []*agentv1.ContainerInfo `json:"containers,omitempty"`
	Networks   []*agentv1.NetworkInfo   `json:"networks,omitempty"`
	Volumes    []*agentv1.VolumeInfo    `json:"volumes,omitempty"`
	Images     []*agentv1.ImageInfo     `json:"images,omitempty"`
	RawJSON    string                   `json:"raw_json,omitempty"`
	Content    string                   `json:"content,omitempty"`
	TotalCount uint32                   `json:"total_count,omitempty"`
}

type dockerServer struct {
	client *DockerClient
}

type logChunkWriter struct {
	write func(string) error
}

func (writer logChunkWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if err := writer.write(string(data)); err != nil {
		return 0, err
	}
	return len(data), nil
}

func newDockerServer() (*dockerServer, error) {
	cli, err := NewDockerClient()
	if err != nil {
		return nil, err
	}
	return &dockerServer{client: cli}, nil
}

func normalizeDockerListPage(page uint32) uint32 {
	if page == 0 {
		return 1
	}
	return page
}

func paginateDockerList[T any](items []T, page, pageSize uint32) ([]T, uint32) {
	totalCount := uint32(len(items))
	if totalCount == 0 {
		return items, 0
	}
	if pageSize == 0 || pageSize >= totalCount {
		return items, totalCount
	}
	page = normalizeDockerListPage(page)
	start := (page - 1) * pageSize
	if start >= totalCount {
		return []T{}, totalCount
	}
	end := start + pageSize
	if end > totalCount {
		end = totalCount
	}
	return items[start:end], totalCount
}

func dockerSearchMatches(search string, values ...string) bool {
	search = strings.TrimSpace(strings.ToLower(search))
	if search == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), search) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, " ")
}

func boolCompare(left, right bool) int {
	if left == right {
		return 0
	}
	if left {
		return 1
	}
	return -1
}

func int64Compare(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func uint32Compare(left, right uint32) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func stringCompare(left, right string) int {
	return strings.Compare(strings.ToLower(left), strings.ToLower(right))
}

func dockerSortResult(compare int, desc bool) bool {
	if compare == 0 {
		return false
	}
	if desc {
		return compare > 0
	}
	return compare < 0
}

func (s *dockerServer) ListContainers(ctx context.Context, req *connect.Request[agentv1.ListContainersRequest]) (*connect.Response[agentv1.ListContainersResponse], error) {
	if req.Msg == nil {
		req.Msg = &agentv1.ListContainersRequest{}
	}
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
		sort.Strings(networks)

		info := &agentv1.ContainerInfo{
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
		}
		if !dockerSearchMatches(req.Msg.GetSearch(), info.GetId(), info.GetName(), info.GetImage(), info.GetState(), info.GetStatus(), joinStrings(info.GetNetworks())) {
			continue
		}
		result = append(result, info)
	}

	sort.SliceStable(result, func(i, j int) bool {
		left := result[i]
		right := result[j]
		var compare int
		switch req.Msg.GetSortBy() {
		case "state":
			compare = stringCompare(left.GetState(), right.GetState())
		case "image":
			compare = stringCompare(left.GetImage(), right.GetImage())
		case "created":
			compare = stringCompare(left.GetCreated(), right.GetCreated())
		default:
			compare = stringCompare(firstNonEmpty(left.GetName(), left.GetId()), firstNonEmpty(right.GetName(), right.GetId()))
		}
		if compare == 0 {
			compare = stringCompare(left.GetId(), right.GetId())
		}
		return dockerSortResult(compare, req.Msg.GetSortDesc())
	})

	pageItems, totalCount := paginateDockerList(result, req.Msg.GetPage(), req.Msg.GetPageSize())

	return connect.NewResponse(&agentv1.ListContainersResponse{
		Containers: pageItems,
		TotalCount: totalCount,
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

func (s *dockerServer) RunContainerAction(ctx context.Context, req *connect.Request[agentv1.RunContainerActionRequest]) (*connect.Response[agentv1.RunContainerActionResponse], error) {
	if req.Msg.GetContainerId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("container_id is required"))
	}

	switch req.Msg.GetAction() {
	case agentv1.ContainerAction_CONTAINER_ACTION_START:
		if err := s.client.ContainerStart(ctx, req.Msg.GetContainerId()); err != nil {
			return nil, err
		}
	case agentv1.ContainerAction_CONTAINER_ACTION_STOP:
		if err := s.client.ContainerStop(ctx, req.Msg.GetContainerId()); err != nil {
			return nil, err
		}
	case agentv1.ContainerAction_CONTAINER_ACTION_RESTART:
		if err := s.client.ContainerRestart(ctx, req.Msg.GetContainerId()); err != nil {
			return nil, err
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("action is required"))
	}

	return connect.NewResponse(&agentv1.RunContainerActionResponse{}), nil
}

func (s *dockerServer) RemoveContainer(ctx context.Context, req *connect.Request[agentv1.RemoveContainerRequest]) (*connect.Response[agentv1.RemoveContainerResponse], error) {
	if req.Msg.GetContainerId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("container_id is required"))
	}

	if err := s.client.ContainerRemove(ctx, req.Msg.GetContainerId(), req.Msg.GetForce(), req.Msg.GetRemoveVolumes()); err != nil {
		return nil, err
	}

	return connect.NewResponse(&agentv1.RemoveContainerResponse{}), nil
}

func (s *dockerServer) GetContainerLogs(ctx context.Context, req *connect.Request[agentv1.GetContainerLogsRequest], stream *connect.ServerStream[agentv1.GetContainerLogsResponse]) error {
	if req.Msg.GetContainerId() == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("container_id is required"))
	}
	return s.streamContainerLogs(ctx, req.Msg.GetContainerId(), req.Msg.GetTail(), req.Msg.GetTimestamps(), true, func(content string) error {
		return stream.Send(&agentv1.GetContainerLogsResponse{Content: content})
	})
}

func (s *dockerServer) collectContainerLogs(ctx context.Context, containerID, tail string, timestamps bool) (string, error) {
	var builder strings.Builder
	if err := s.streamContainerLogs(ctx, containerID, tail, timestamps, false, func(content string) error {
		builder.WriteString(content)
		return nil
	}); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func (s *dockerServer) streamContainerLogs(ctx context.Context, containerID, tail string, timestamps, follow bool, write func(string) error) error {
	inspect, err := s.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return err
	}
	reader, err := s.client.ContainerLogs(ctx, containerID, tail, timestamps, follow)
	if err != nil {
		return err
	}
	defer reader.Close()

	writer := logChunkWriter{write: write}
	if inspect.Config != nil && inspect.Config.Tty {
		if _, err := io.Copy(writer, reader); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read container logs: %w", err)
		}
		return nil
	}
	if _, err := stdcopy.StdCopy(writer, writer, reader); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("decode container logs: %w", err)
	}
	return nil
}

func (s *dockerServer) ListNetworks(ctx context.Context, req *connect.Request[agentv1.ListNetworksRequest]) (*connect.Response[agentv1.ListNetworksResponse], error) {
	if req.Msg == nil {
		req.Msg = &agentv1.ListNetworksRequest{}
	}
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

		info := &agentv1.NetworkInfo{
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
		}
		if !dockerSearchMatches(req.Msg.GetSearch(), info.GetId(), info.GetName(), info.GetDriver(), info.GetScope(), info.GetSubnet(), info.GetGateway()) {
			continue
		}
		result = append(result, info)
	}

	sort.SliceStable(result, func(i, j int) bool {
		left := result[i]
		right := result[j]
		var compare int
		switch req.Msg.GetSortBy() {
		case "driver":
			compare = stringCompare(left.GetDriver(), right.GetDriver())
		case "scope":
			compare = stringCompare(left.GetScope(), right.GetScope())
		case "created":
			compare = stringCompare(left.GetCreated(), right.GetCreated())
		case "containers_count":
			compare = uint32Compare(left.GetContainersCount(), right.GetContainersCount())
		default:
			compare = stringCompare(firstNonEmpty(left.GetName(), left.GetId()), firstNonEmpty(right.GetName(), right.GetId()))
		}
		if compare == 0 {
			compare = stringCompare(left.GetId(), right.GetId())
		}
		return dockerSortResult(compare, req.Msg.GetSortDesc())
	})

	pageItems, totalCount := paginateDockerList(result, req.Msg.GetPage(), req.Msg.GetPageSize())

	return connect.NewResponse(&agentv1.ListNetworksResponse{
		Networks:   pageItems,
		TotalCount: totalCount,
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

func (s *dockerServer) RemoveNetwork(ctx context.Context, req *connect.Request[agentv1.RemoveNetworkRequest]) (*connect.Response[agentv1.RemoveNetworkResponse], error) {
	if req.Msg.GetNetworkId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("network_id is required"))
	}

	if err := s.client.NetworkRemove(ctx, req.Msg.GetNetworkId()); err != nil {
		return nil, err
	}

	return connect.NewResponse(&agentv1.RemoveNetworkResponse{}), nil
}

func (s *dockerServer) ListVolumes(ctx context.Context, req *connect.Request[agentv1.ListVolumesRequest]) (*connect.Response[agentv1.ListVolumesResponse], error) {
	if req.Msg == nil {
		req.Msg = &agentv1.ListVolumesRequest{}
	}
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

		info := &agentv1.VolumeInfo{
			Name:            v.Name,
			Driver:          v.Driver,
			Mountpoint:      v.Mountpoint,
			Scope:           v.Scope,
			Created:         v.CreatedAt,
			Labels:          v.Labels,
			SizeBytes:       size,
			ContainersCount: count,
			InUse:           count > 0,
		}
		if !dockerSearchMatches(req.Msg.GetSearch(), info.GetName(), info.GetDriver(), info.GetMountpoint(), info.GetScope()) {
			continue
		}
		result = append(result, info)
	}

	sort.SliceStable(result, func(i, j int) bool {
		left := result[i]
		right := result[j]
		var compare int
		switch req.Msg.GetSortBy() {
		case "driver":
			compare = stringCompare(left.GetDriver(), right.GetDriver())
		case "scope":
			compare = stringCompare(left.GetScope(), right.GetScope())
		case "created":
			compare = stringCompare(left.GetCreated(), right.GetCreated())
		case "size_bytes":
			compare = int64Compare(left.GetSizeBytes(), right.GetSizeBytes())
		case "containers_count":
			compare = uint32Compare(left.GetContainersCount(), right.GetContainersCount())
		case "in_use":
			compare = boolCompare(left.GetInUse(), right.GetInUse())
		default:
			compare = stringCompare(left.GetName(), right.GetName())
		}
		if compare == 0 {
			compare = stringCompare(left.GetName(), right.GetName())
		}
		return dockerSortResult(compare, req.Msg.GetSortDesc())
	})

	pageItems, totalCount := paginateDockerList(result, req.Msg.GetPage(), req.Msg.GetPageSize())

	return connect.NewResponse(&agentv1.ListVolumesResponse{
		Volumes:    pageItems,
		TotalCount: totalCount,
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

func (s *dockerServer) RemoveVolume(ctx context.Context, req *connect.Request[agentv1.RemoveVolumeRequest]) (*connect.Response[agentv1.RemoveVolumeResponse], error) {
	if req.Msg.GetVolumeName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("volume_name is required"))
	}

	if err := s.client.VolumeRemove(ctx, req.Msg.GetVolumeName()); err != nil {
		return nil, err
	}

	return connect.NewResponse(&agentv1.RemoveVolumeResponse{}), nil
}

func (s *dockerServer) ListImages(ctx context.Context, req *connect.Request[agentv1.ListImagesRequest]) (*connect.Response[agentv1.ListImagesResponse], error) {
	if req.Msg == nil {
		req.Msg = &agentv1.ListImagesRequest{}
	}
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

		info := &agentv1.ImageInfo{
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
		}
		if !dockerSearchMatches(req.Msg.GetSearch(), info.GetId(), joinStrings(info.GetRepoTags()), joinStrings(info.GetRepoDigests()), info.GetArchitecture(), info.GetOs(), info.GetAuthor()) {
			continue
		}
		result = append(result, info)
	}

	sort.SliceStable(result, func(i, j int) bool {
		left := result[i]
		right := result[j]
		var compare int
		switch req.Msg.GetSortBy() {
		case "size":
			compare = int64Compare(left.GetSize(), right.GetSize())
		case "virtual_size":
			compare = int64Compare(left.GetVirtualSize(), right.GetVirtualSize())
		case "created":
			compare = stringCompare(left.GetCreated(), right.GetCreated())
		case "containers_count":
			compare = uint32Compare(left.GetContainersCount(), right.GetContainersCount())
		case "dangling":
			compare = boolCompare(left.GetIsDangling(), right.GetIsDangling())
		default:
			compare = stringCompare(firstNonEmpty(joinStrings(left.GetRepoTags()), left.GetId()), firstNonEmpty(joinStrings(right.GetRepoTags()), right.GetId()))
		}
		if compare == 0 {
			compare = stringCompare(left.GetId(), right.GetId())
		}
		return dockerSortResult(compare, req.Msg.GetSortDesc())
	})

	pageItems, totalCount := paginateDockerList(result, req.Msg.GetPage(), req.Msg.GetPageSize())

	return connect.NewResponse(&agentv1.ListImagesResponse{
		Images:     pageItems,
		TotalCount: totalCount,
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

func (s *dockerServer) RemoveImage(ctx context.Context, req *connect.Request[agentv1.RemoveImageRequest]) (*connect.Response[agentv1.RemoveImageResponse], error) {
	if req.Msg.GetImageId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("image_id is required"))
	}

	if err := s.client.ImageRemove(ctx, req.Msg.GetImageId(), req.Msg.GetForce()); err != nil {
		return nil, err
	}

	return connect.NewResponse(&agentv1.RemoveImageResponse{}), nil
}

func executeDockerQuery(ctx context.Context, query *agentv1.DockerQueryTask) (dockerTaskResult, error) {
	if query == nil {
		return dockerTaskResult{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("query is required"))
	}
	server, err := newDockerServer()
	if err != nil {
		return dockerTaskResult{}, err
	}
	defer func() {
		_ = server.client.Close()
	}()

	switch query.GetAction() {
	case "list":
		switch query.GetResource() {
		case "containers":
			resp, err := server.ListContainers(ctx, connect.NewRequest(&agentv1.ListContainersRequest{
				PageSize: query.GetPageSize(),
				Page:     query.GetPage(),
				Search:   query.GetSearch(),
				SortBy:   query.GetSortBy(),
				SortDesc: query.GetSortDesc(),
			}))
			if err != nil {
				return dockerTaskResult{}, err
			}
			return dockerTaskResult{Containers: resp.Msg.GetContainers(), TotalCount: resp.Msg.GetTotalCount()}, nil
		case "networks":
			resp, err := server.ListNetworks(ctx, connect.NewRequest(&agentv1.ListNetworksRequest{
				PageSize: query.GetPageSize(),
				Page:     query.GetPage(),
				Search:   query.GetSearch(),
				SortBy:   query.GetSortBy(),
				SortDesc: query.GetSortDesc(),
			}))
			if err != nil {
				return dockerTaskResult{}, err
			}
			return dockerTaskResult{Networks: resp.Msg.GetNetworks(), TotalCount: resp.Msg.GetTotalCount()}, nil
		case "volumes":
			resp, err := server.ListVolumes(ctx, connect.NewRequest(&agentv1.ListVolumesRequest{
				PageSize: query.GetPageSize(),
				Page:     query.GetPage(),
				Search:   query.GetSearch(),
				SortBy:   query.GetSortBy(),
				SortDesc: query.GetSortDesc(),
			}))
			if err != nil {
				return dockerTaskResult{}, err
			}
			return dockerTaskResult{Volumes: resp.Msg.GetVolumes(), TotalCount: resp.Msg.GetTotalCount()}, nil
		case "images":
			resp, err := server.ListImages(ctx, connect.NewRequest(&agentv1.ListImagesRequest{
				PageSize: query.GetPageSize(),
				Page:     query.GetPage(),
				Search:   query.GetSearch(),
				SortBy:   query.GetSortBy(),
				SortDesc: query.GetSortDesc(),
			}))
			if err != nil {
				return dockerTaskResult{}, err
			}
			return dockerTaskResult{Images: resp.Msg.GetImages(), TotalCount: resp.Msg.GetTotalCount()}, nil
		default:
			return dockerTaskResult{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported list resource %q", query.GetResource()))
		}
	case "inspect":
		switch query.GetResource() {
		case "container":
			resp, err := server.InspectContainer(ctx, connect.NewRequest(&agentv1.InspectContainerRequest{ContainerId: query.GetId()}))
			if err != nil {
				return dockerTaskResult{}, err
			}
			return dockerTaskResult{RawJSON: resp.Msg.GetRawJson()}, nil
		case "network":
			resp, err := server.InspectNetwork(ctx, connect.NewRequest(&agentv1.InspectNetworkRequest{NetworkId: query.GetId()}))
			if err != nil {
				return dockerTaskResult{}, err
			}
			return dockerTaskResult{RawJSON: resp.Msg.GetRawJson()}, nil
		case "volume":
			resp, err := server.InspectVolume(ctx, connect.NewRequest(&agentv1.InspectVolumeRequest{VolumeName: query.GetId()}))
			if err != nil {
				return dockerTaskResult{}, err
			}
			return dockerTaskResult{RawJSON: resp.Msg.GetRawJson()}, nil
		case "image":
			resp, err := server.InspectImage(ctx, connect.NewRequest(&agentv1.InspectImageRequest{ImageId: query.GetId()}))
			if err != nil {
				return dockerTaskResult{}, err
			}
			return dockerTaskResult{RawJSON: resp.Msg.GetRawJson()}, nil
		default:
			return dockerTaskResult{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported inspect resource %q", query.GetResource()))
		}
	case "logs":
		if query.GetResource() != "container" {
			return dockerTaskResult{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported logs resource %q", query.GetResource()))
		}
		content, err := server.collectContainerLogs(ctx, query.GetId(), query.GetTail(), query.GetTimestamps())
		if err != nil {
			return dockerTaskResult{}, err
		}
		return dockerTaskResult{Content: content}, nil
	default:
		return dockerTaskResult{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported docker query action %q", query.GetAction()))
	}
}

func dockerQueryErrorCode(err error) string {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		switch connectErr.Code() {
		case connect.CodeInvalidArgument:
			return "invalid_argument"
		case connect.CodeNotFound:
			return "not_found"
		case connect.CodeFailedPrecondition:
			return "failed_precondition"
		case connect.CodePermissionDenied:
			return "permission_denied"
		case connect.CodeDeadlineExceeded:
			return "deadline_exceeded"
		case connect.CodeUnavailable:
			return "unavailable"
		default:
			return "internal"
		}
	}
	return "internal"
}

type dockerTaskParams struct {
	Action        string `json:"action"`
	Resource      string `json:"resource"`
	ID            string `json:"id,omitempty"`
	Force         bool   `json:"force,omitempty"`
	RemoveVolumes bool   `json:"remove_volumes,omitempty"`
}

func executeDockerTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	_ = cfg
	params, err := parseDockerTaskParams(pulledTask.GetParamsJson())
	if err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting docker task: action=%s resource=%s id=%s\n", params.Action, params.Resource, params.ID)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), dockerTaskStepName(params.Action), func() error {
		return runDockerMutation(ctx, params)
	}); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, logUploader, "docker task finished successfully\n"); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func parseDockerTaskParams(paramsJSON string) (dockerTaskParams, error) {
	var params dockerTaskParams
	if paramsJSON == "" {
		return params, fmt.Errorf("docker task params are required")
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return params, fmt.Errorf("decode docker task params: %w", err)
	}
	if params.Action == "" || params.ID == "" {
		return params, fmt.Errorf("docker task action and id are required")
	}
	return params, nil
}

func dockerTaskStepName(action string) task.StepName {
	switch action {
	case "start":
		return task.StepDockerStart
	case "stop":
		return task.StepDockerStop
	case "restart":
		return task.StepDockerRestart
	default:
		return task.StepDockerRemove
	}
}

func runDockerMutation(ctx context.Context, params dockerTaskParams) error {
	server, err := newDockerServer()
	if err != nil {
		return err
	}
	defer func() {
		_ = server.client.Close()
	}()

	switch params.Action {
	case "start":
		_, err = server.RunContainerAction(ctx, connect.NewRequest(&agentv1.RunContainerActionRequest{ContainerId: params.ID, Action: agentv1.ContainerAction_CONTAINER_ACTION_START}))
	case "stop":
		_, err = server.RunContainerAction(ctx, connect.NewRequest(&agentv1.RunContainerActionRequest{ContainerId: params.ID, Action: agentv1.ContainerAction_CONTAINER_ACTION_STOP}))
	case "restart":
		_, err = server.RunContainerAction(ctx, connect.NewRequest(&agentv1.RunContainerActionRequest{ContainerId: params.ID, Action: agentv1.ContainerAction_CONTAINER_ACTION_RESTART}))
	case "remove":
		switch params.Resource {
		case "container":
			_, err = server.RemoveContainer(ctx, connect.NewRequest(&agentv1.RemoveContainerRequest{ContainerId: params.ID, Force: params.Force, RemoveVolumes: params.RemoveVolumes}))
		case "network":
			_, err = server.RemoveNetwork(ctx, connect.NewRequest(&agentv1.RemoveNetworkRequest{NetworkId: params.ID}))
		case "volume":
			_, err = server.RemoveVolume(ctx, connect.NewRequest(&agentv1.RemoveVolumeRequest{VolumeName: params.ID}))
		case "image":
			_, err = server.RemoveImage(ctx, connect.NewRequest(&agentv1.RemoveImageRequest{ImageId: params.ID, Force: params.Force}))
		default:
			return fmt.Errorf("unknown docker resource for remove: %s", params.Resource)
		}
	default:
		return fmt.Errorf("unknown docker action: %s", params.Action)
	}
	return err
}
