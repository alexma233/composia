package controller

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"time"
)

type agentTaskServer struct {
	db            *store.DB
	taskQueue     *taskQueueNotifier
	dockerQueries *dockerQueryBroker
	maxWait       time.Duration
	retryInterval time.Duration
}

func (server *agentTaskServer) PullNextTask(ctx context.Context, req *connect.Request[agentv1.PullNextTaskRequest]) (*connect.Response[agentv1.PullNextTaskResponse], error) {
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}

	waitCh := server.taskQueue.Subscribe()
	defer server.taskQueue.Unsubscribe(waitCh)
	deadline := time.Now().Add(server.longPollMaxWait())

	for {
		record, err := server.db.ClaimNextPendingTaskForNode(ctx, req.Msg.GetNodeId(), time.Now().UTC())
		if err == nil {
			params, err := taskParams(record.ParamsJSON)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			logControllerAssignedTask(record)
			response := &agentv1.PullNextTaskResponse{
				HasTask: true,
				Task: &agentv1.AgentTask{
					TaskId:       record.TaskID,
					Type:         protoAgentTaskType(record.Type),
					ServiceName:  record.ServiceName,
					NodeId:       record.NodeID,
					RepoRevision: record.RepoRevision,
					ServiceDir:   params.ServiceDir,
					DataNames:    params.DataNames,
					ParamsJson:   record.ParamsJSON,
				},
			}
			return connect.NewResponse(response), nil
		}
		if !errors.Is(err, store.ErrNoPendingTask) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return connect.NewResponse(&agentv1.PullNextTaskResponse{HasTask: false}), nil
		}
		waitFor := minDuration(remaining, server.longPollRetryInterval())
		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-waitCh:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (server *agentTaskServer) longPollMaxWait() time.Duration {
	if server.maxWait > 0 {
		return server.maxWait
	}
	return pullNextTaskMaxWait
}

func (server *agentTaskServer) longPollRetryInterval() time.Duration {
	if server.retryInterval > 0 {
		return server.retryInterval
	}
	return pullNextTaskRetryWait
}
