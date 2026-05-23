package controller

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"fmt"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"github.com/google/uuid"
	"os"
	"path/filepath"
)

type nodeQueryServer struct {
	db          *store.DB
	cfg         *config.ControllerConfig
	taskQueue   *taskQueueNotifier
	taskResults *taskResultNotifier
}

type nodeMaintenanceServer struct {
	db          *store.DB
	cfg         *config.ControllerConfig
	taskQueue   *taskQueueNotifier
	taskResults *taskResultNotifier
}

func (server *nodeQueryServer) ListNodes(ctx context.Context, _ *connect.Request[controllerv1.ListNodesRequest]) (*connect.Response[controllerv1.ListNodesResponse], error) {
	snapshotByNodeID, err := buildNodeSnapshotMap(ctx, server.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.ListNodesResponse{
		Nodes: make([]*controllerv1.NodeSummary, 0, len(server.cfg.Nodes)),
	}
	for _, node := range server.cfg.Nodes {
		summary := nodeSummary(node, snapshotByNodeID[node.ID])
		summary.Actions = buildNodeActionCapabilities(server.cfg, configuredNodeIDs(server.cfg), snapshotByNodeID, node.ID)
		response.Nodes = append(response.Nodes, summary)
	}

	return connect.NewResponse(response), nil
}

func (server *nodeQueryServer) GetNode(ctx context.Context, req *connect.Request[controllerv1.GetNodeRequest]) (*connect.Response[controllerv1.GetNodeResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	snapshotByNodeID, err := buildNodeSnapshotMap(ctx, server.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, node := range server.cfg.Nodes {
		if node.ID == req.Msg.GetNodeId() {
			summary := nodeSummary(node, snapshotByNodeID[node.ID])
			summary.Actions = buildNodeActionCapabilities(server.cfg, configuredNodeIDs(server.cfg), snapshotByNodeID, node.ID)
			return connect.NewResponse(&controllerv1.GetNodeResponse{Node: summary}), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %q is not configured", req.Msg.GetNodeId()))
}

func (server *nodeQueryServer) GetNodeDockerStats(ctx context.Context, req *connect.Request[controllerv1.GetNodeDockerStatsRequest]) (*connect.Response[controllerv1.GetNodeDockerStatsResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	stats, err := server.db.GetNodeDockerStats(ctx, req.Msg.GetNodeId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&controllerv1.GetNodeDockerStatsResponse{
		Stats: &controllerv1.DockerStats{
			ContainersTotal:     stats.ContainersTotal,
			ContainersRunning:   stats.ContainersRunning,
			ContainersStopped:   stats.ContainersStopped,
			ContainersPaused:    stats.ContainersPaused,
			Images:              stats.Images,
			Networks:            stats.Networks,
			Volumes:             stats.Volumes,
			VolumesSizeBytes:    stats.VolumesSizeBytes,
			DisksUsageBytes:     stats.DisksUsageBytes,
			DockerServerVersion: stats.DockerServerVersion,
		},
	}), nil
}

func (server *nodeMaintenanceServer) ReloadNodeCaddy(ctx context.Context, req *connect.Request[controllerv1.ReloadNodeCaddyRequest]) (*connect.Response[controllerv1.ReloadNodeCaddyResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	createdTask, err := createNodeCaddyReloadTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), req.Msg.GetNodeId(), requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}

	notifyTaskQueue(server.taskQueue)

	return connect.NewResponse(&controllerv1.ReloadNodeCaddyResponse{TaskId: createdTask.TaskID}), nil
}

func (server *nodeMaintenanceServer) SyncNodeCaddyFiles(ctx context.Context, req *connect.Request[controllerv1.SyncNodeCaddyFilesRequest]) (*connect.Response[controllerv1.SyncNodeCaddyFilesResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	createdTask, err := createNodeCaddySyncTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), req.Msg.GetNodeId(), req.Msg.GetServiceName(), req.Msg.GetFullRebuild(), requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&controllerv1.SyncNodeCaddyFilesResponse{TaskId: createdTask.TaskID}), nil
}

func (server *nodeMaintenanceServer) PruneNodeDocker(ctx context.Context, req *connect.Request[controllerv1.PruneNodeDockerRequest]) (*connect.Response[controllerv1.PruneNodeDockerResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	target := req.Msg.GetTarget()
	if target == "" {
		target = "all"
	}

	snapshot, err := server.db.GetNodeSnapshot(ctx, req.Msg.GetNodeId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !snapshot.IsOnline {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q is offline", req.Msg.GetNodeId()))
	}

	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	paramsJSON := fmt.Sprintf(`{"target":%q}`, target)

	createdTask, err := server.db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        task.TypePrune,
		Source:      requestTaskSource(req.Header()),
		TriggeredBy: triggeredBy,
		NodeID:      req.Msg.GetNodeId(),
		Status:      task.StatusPending,
		ParamsJSON:  paramsJSON,
		LogPath:     filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	notifyTaskQueue(server.taskQueue)

	return connect.NewResponse(&controllerv1.PruneNodeDockerResponse{
		TaskId: taskID,
	}), nil
}

func (server *nodeMaintenanceServer) PruneNodeRustic(ctx context.Context, req *connect.Request[controllerv1.PruneNodeRusticRequest]) (*connect.Response[controllerv1.PruneNodeRusticResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}
	nodeID := req.Msg.GetNodeId()
	var err error
	if nodeID == "" {
		nodeID, err = chooseRusticMainNode(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), task.TypeRusticPrune)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	createdTask, err := createNodeRusticMaintenanceTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), nodeID, task.TypeRusticPrune, rusticMaintenanceTaskParams{ServiceName: req.Msg.GetServiceName(), DataName: req.Msg.GetDataName()}, requestTaskSource(req.Header()), nil)
	if err != nil {
		return nil, err
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&controllerv1.PruneNodeRusticResponse{TaskId: createdTask.TaskID}), nil
}

func (server *nodeMaintenanceServer) InitNodeRustic(ctx context.Context, req *connect.Request[controllerv1.InitNodeRusticRequest]) (*connect.Response[controllerv1.InitNodeRusticResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}
	nodeID := req.Msg.GetNodeId()
	var err error
	if nodeID == "" {
		nodeID, err = chooseRusticMainNode(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), task.TypeRusticInit)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	createdTask, err := createNodeRusticMaintenanceTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), nodeID, task.TypeRusticInit, rusticMaintenanceTaskParams{}, requestTaskSource(req.Header()), nil)
	if err != nil {
		return nil, err
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&controllerv1.InitNodeRusticResponse{TaskId: createdTask.TaskID}), nil
}

func (server *nodeMaintenanceServer) ForgetNodeRustic(ctx context.Context, req *connect.Request[controllerv1.ForgetNodeRusticRequest]) (*connect.Response[controllerv1.ForgetNodeRusticResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}
	nodeID := req.Msg.GetNodeId()
	var err error
	if nodeID == "" {
		nodeID, err = chooseRusticMainNode(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), task.TypeRusticForget)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	createdTask, err := createNodeRusticMaintenanceTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), nodeID, task.TypeRusticForget, rusticMaintenanceTaskParams{ServiceName: req.Msg.GetServiceName(), DataName: req.Msg.GetDataName()}, requestTaskSource(req.Header()), nil)
	if err != nil {
		return nil, err
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&controllerv1.ForgetNodeRusticResponse{TaskId: createdTask.TaskID}), nil
}

func nodeSummary(node config.NodeConfig, snapshot store.NodeSnapshot) *controllerv1.NodeSummary {
	displayName := node.DisplayName
	if displayName == "" {
		displayName = node.ID
	}
	return &controllerv1.NodeSummary{
		NodeId:        node.ID,
		DisplayName:   displayName,
		Enabled:       node.Enabled == nil || *node.Enabled,
		IsOnline:      snapshot.IsOnline,
		LastHeartbeat: snapshot.LastHeartbeat,
	}
}
