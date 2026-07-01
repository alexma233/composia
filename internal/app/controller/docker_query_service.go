package controller

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

type dockerQueryServer struct {
	db            *store.DB
	cfg           *config.ControllerConfig
	dockerQueries *dockerQueryBroker
}

func (server *dockerQueryServer) ListNodeContainers(ctx context.Context, req *connect.Request[controllerv1.ListNodeContainersRequest]) (*connect.Response[controllerv1.ListNodeContainersResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	result, err := server.executeDockerListQuery(ctx, req.Header(), req.Msg.GetNodeId(), dockerResourceContainers, req.Msg.GetPage(), req.Msg.GetPageSize(), req.Msg.GetSearch(), req.Msg.GetSortBy(), req.Msg.GetSortDesc())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.ListNodeContainersResponse{
		Containers: result.Containers,
		TotalCount: result.TotalCount,
	}), nil
}

func (server *dockerQueryServer) InspectNodeContainer(ctx context.Context, req *connect.Request[controllerv1.InspectNodeContainerRequest]) (*connect.Response[controllerv1.InspectNodeContainerResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetContainerId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and container_id are required"))
	}

	result, err := server.executeDockerInspectQuery(ctx, req.Header(), req.Msg.GetNodeId(), dockerResourceContainer, req.Msg.GetContainerId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.InspectNodeContainerResponse{
		RawJson: result.RawJSON,
	}), nil
}

func (server *dockerQueryServer) ListNodeNetworks(ctx context.Context, req *connect.Request[controllerv1.ListNodeNetworksRequest]) (*connect.Response[controllerv1.ListNodeNetworksResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	result, err := server.executeDockerListQuery(ctx, req.Header(), req.Msg.GetNodeId(), dockerResourceNetworks, req.Msg.GetPage(), req.Msg.GetPageSize(), req.Msg.GetSearch(), req.Msg.GetSortBy(), req.Msg.GetSortDesc())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.ListNodeNetworksResponse{
		Networks:   result.Networks,
		TotalCount: result.TotalCount,
	}), nil
}

func (server *dockerQueryServer) InspectNodeNetwork(ctx context.Context, req *connect.Request[controllerv1.InspectNodeNetworkRequest]) (*connect.Response[controllerv1.InspectNodeNetworkResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetNetworkId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and network_id are required"))
	}

	result, err := server.executeDockerInspectQuery(ctx, req.Header(), req.Msg.GetNodeId(), dockerResourceNetwork, req.Msg.GetNetworkId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.InspectNodeNetworkResponse{
		RawJson: result.RawJSON,
	}), nil
}

func (server *dockerQueryServer) ListNodeVolumes(ctx context.Context, req *connect.Request[controllerv1.ListNodeVolumesRequest]) (*connect.Response[controllerv1.ListNodeVolumesResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	result, err := server.executeDockerListQuery(ctx, req.Header(), req.Msg.GetNodeId(), dockerResourceVolumes, req.Msg.GetPage(), req.Msg.GetPageSize(), req.Msg.GetSearch(), req.Msg.GetSortBy(), req.Msg.GetSortDesc())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.ListNodeVolumesResponse{
		Volumes:    result.Volumes,
		TotalCount: result.TotalCount,
	}), nil
}

func (server *dockerQueryServer) InspectNodeVolume(ctx context.Context, req *connect.Request[controllerv1.InspectNodeVolumeRequest]) (*connect.Response[controllerv1.InspectNodeVolumeResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetVolumeName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and volume_name are required"))
	}

	result, err := server.executeDockerInspectQuery(ctx, req.Header(), req.Msg.GetNodeId(), dockerResourceVolume, req.Msg.GetVolumeName())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.InspectNodeVolumeResponse{
		RawJson: result.RawJSON,
	}), nil
}

func (server *dockerQueryServer) ListNodeImages(ctx context.Context, req *connect.Request[controllerv1.ListNodeImagesRequest]) (*connect.Response[controllerv1.ListNodeImagesResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	result, err := server.executeDockerListQuery(ctx, req.Header(), req.Msg.GetNodeId(), dockerResourceImages, req.Msg.GetPage(), req.Msg.GetPageSize(), req.Msg.GetSearch(), req.Msg.GetSortBy(), req.Msg.GetSortDesc())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.ListNodeImagesResponse{
		Images:     result.Images,
		TotalCount: result.TotalCount,
	}), nil
}

func (server *dockerQueryServer) InspectNodeImage(ctx context.Context, req *connect.Request[controllerv1.InspectNodeImageRequest]) (*connect.Response[controllerv1.InspectNodeImageResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetImageId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and image_id are required"))
	}

	result, err := server.executeDockerInspectQuery(ctx, req.Header(), req.Msg.GetNodeId(), dockerResourceImage, req.Msg.GetImageId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.InspectNodeImageResponse{
		RawJson: result.RawJSON,
	}), nil
}

type dockerListResult struct {
	Containers []*controllerv1.ContainerInfo `json:"containers,omitempty"`
	Networks   []*controllerv1.NetworkInfo   `json:"networks,omitempty"`
	Volumes    []*controllerv1.VolumeInfo    `json:"volumes,omitempty"`
	Images     []*controllerv1.ImageInfo     `json:"images,omitempty"`
	Exec       *dockerExecResult             `json:"exec,omitempty"`
	RawJSON    string                        `json:"raw_json,omitempty"`
	Content    string                        `json:"content,omitempty"`
	TotalCount uint32                        `json:"total_count,omitempty"`
}

type dockerExecResult struct {
	ExitCode        int32  `json:"exit_code"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	TimedOut        bool   `json:"timed_out"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
	Duration        string `json:"duration"`
}
