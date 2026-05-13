package controller

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"github.com/google/uuid"
	"net/http"
	"os"
	"path/filepath"
)

type dockerCommandServer struct {
	db            *store.DB
	cfg           *config.ControllerConfig
	taskQueue     *taskQueueNotifier
	taskResults   *taskResultNotifier
	dockerQueries *dockerQueryBroker
	execManager   *execTunnelManager
	logManager    *containerLogTunnelManager
}

func (server *dockerCommandServer) RunContainerAction(ctx context.Context, req *connect.Request[controllerv1.RunContainerActionRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id, container_id, and action are required"))
	}

	var taskType task.Type
	var action string
	switch req.Msg.GetAction() {
	case controllerv1.ContainerAction_CONTAINER_ACTION_START:
		taskType = task.TypeDockerStart
		action = "start"
	case controllerv1.ContainerAction_CONTAINER_ACTION_STOP:
		taskType = task.TypeDockerStop
		action = "stop"
	case controllerv1.ContainerAction_CONTAINER_ACTION_RESTART:
		taskType = task.TypeDockerRestart
		action = "restart"
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("action is required"))
	}

	record, err := server.createContainerTask(ctx, req.Header(), req.Msg.GetNodeId(), req.Msg.GetContainerId(), taskType, map[string]any{"action": action, "resource": "container", "id": req.Msg.GetContainerId()})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(record)), nil
}

func (server *dockerCommandServer) RemoveContainer(ctx context.Context, req *connect.Request[controllerv1.RemoveContainerRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and container_id are required"))
	}

	record, err := server.createContainerTask(ctx, req.Header(), req.Msg.GetNodeId(), req.Msg.GetContainerId(), task.TypeDockerRemove, map[string]any{
		"action":         "remove",
		"resource":       "container",
		"id":             req.Msg.GetContainerId(),
		"force":          req.Msg.GetForce(),
		"remove_volumes": req.Msg.GetRemoveVolumes(),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(record)), nil
}

func (server *dockerCommandServer) RemoveNetwork(ctx context.Context, req *connect.Request[controllerv1.RemoveNetworkRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetNetworkId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and network_id are required"))
	}

	record, err := server.createNodeDockerTask(ctx, req.Header(), req.Msg.GetNodeId(), task.TypeDockerRemove, map[string]any{
		"action":   "remove",
		"resource": "network",
		"id":       req.Msg.GetNetworkId(),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(record)), nil
}

func (server *dockerCommandServer) RemoveVolume(ctx context.Context, req *connect.Request[controllerv1.RemoveVolumeRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetVolumeName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and volume_name are required"))
	}

	record, err := server.createNodeDockerTask(ctx, req.Header(), req.Msg.GetNodeId(), task.TypeDockerRemove, map[string]any{
		"action":   "remove",
		"resource": "volume",
		"id":       req.Msg.GetVolumeName(),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(record)), nil
}

func (server *dockerCommandServer) RemoveImage(ctx context.Context, req *connect.Request[controllerv1.RemoveImageRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetImageId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and image_id are required"))
	}

	record, err := server.createNodeDockerTask(ctx, req.Header(), req.Msg.GetNodeId(), task.TypeDockerRemove, map[string]any{
		"action":   "remove",
		"resource": "image",
		"id":       req.Msg.GetImageId(),
		"force":    req.Msg.GetForce(),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(record)), nil
}

func (server *dockerCommandServer) GetContainerLogs(ctx context.Context, req *connect.Request[controllerv1.GetContainerLogsRequest], stream *connect.ServerStream[controllerv1.GetContainerLogsResponse]) error {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetContainerId() == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and container_id are required"))
	}
	if err := validateNodeForDockerQuery(ctx, server.db, server.cfg, req.Msg.GetNodeId()); err != nil {
		return err
	}
	session, err := server.openContainerLogSession(req.Msg.GetNodeId(), req.Msg.GetContainerId(), req.Msg.GetTail(), req.Msg.GetTimestamps())
	if err != nil {
		return err
	}
	defer server.logManager.closeSession(session.id)

	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-session.incoming:
			if !ok {
				return nil
			}
			switch message.GetKind() {
			case containerLogKindChunk:
				if message.GetContent() == "" {
					continue
				}
				if err := stream.Send(&controllerv1.GetContainerLogsResponse{Content: message.GetContent()}); err != nil {
					return err
				}
			case containerLogKindError:
				return containerLogStreamError(message)
			case containerLogKindClosed:
				return nil
			}
		}
	}
}

func (server *dockerCommandServer) createContainerTask(ctx context.Context, header http.Header, nodeID, containerID string, taskType task.Type, params map[string]any) (task.Record, error) {
	if nodeID == "" || containerID == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and container_id are required"))
	}
	if err := validateTaskTargetNode(ctx, server.db, server.cfg, nodeID, taskType); err != nil {
		return task.Record{}, err
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal params: %w", err))
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	createdTask, err := server.db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        taskType,
		Source:      requestTaskSource(header),
		TriggeredBy: triggeredBy,
		NodeID:      nodeID,
		Status:      task.StatusPending,
		ParamsJSON:  string(paramsJSON),
		LogPath:     filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	notifyTaskQueue(server.taskQueue)
	return createdTask, nil
}

func (server *dockerCommandServer) createNodeDockerTask(ctx context.Context, header http.Header, nodeID string, taskType task.Type, params map[string]any) (task.Record, error) {
	if nodeID == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	if err := validateTaskTargetNode(ctx, server.db, server.cfg, nodeID, taskType); err != nil {
		return task.Record{}, err
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal params: %w", err))
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	createdTask, err := server.db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        taskType,
		Source:      requestTaskSource(header),
		TriggeredBy: triggeredBy,
		NodeID:      nodeID,
		Status:      task.StatusPending,
		ParamsJSON:  string(paramsJSON),
		LogPath:     filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	notifyTaskQueue(server.taskQueue)
	return createdTask, nil
}
