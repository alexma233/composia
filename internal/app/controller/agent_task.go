package controller

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	if req.Msg.GetExecutionProtocol() != 1 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("agent must support task execution protocol 1"))
	}
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}

	waitCh := server.taskQueue.Subscribe()
	defer server.taskQueue.Unsubscribe(waitCh)
	deadline := time.Now().Add(server.longPollMaxWait())

	for {
		now := time.Now().UTC()
		record, err := server.db.ClaimNextPendingTaskForNode(ctx, req.Msg.GetNodeId(), now, now.Add(taskExecutionLease))
		if err == nil {
			params, err := taskParams(record.ParamsJSON)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			logControllerAssignedTask(record)
			response := &agentv1.PullNextTaskResponse{
				HasTask: true,
				Task: &agentv1.AgentTask{
					TaskId:         record.TaskID,
					Type:           protoAgentTaskType(record.Type),
					ServiceName:    record.ServiceName,
					NodeId:         record.NodeID,
					RepoRevision:   record.RepoRevision,
					ServiceDir:     params.ServiceDir,
					DataNames:      params.DataNames,
					ParamsJson:     record.ParamsJSON,
					ExecutionId:    record.ExecutionID,
					LeaseExpiresAt: timestamppb.New(*record.LeaseExpiresAt),
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

func (server *agentTaskServer) AcknowledgeTask(ctx context.Context, req *connect.Request[agentv1.AcknowledgeTaskRequest]) (*connect.Response[agentv1.AcknowledgeTaskResponse], error) {
	nodeID, err := validateTaskExecutionRequest(ctx, req.Msg.GetTaskId(), req.Msg.GetExecutionId())
	if err != nil {
		return nil, err
	}
	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if detail.Record.NodeID != nodeID {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("task does not belong to authenticated node"))
	}
	now := time.Now().UTC()
	expiresAt := now.Add(taskExecutionLease)
	if err := server.db.AcknowledgeTaskExecution(ctx, req.Msg.GetTaskId(), req.Msg.GetExecutionId(), now, expiresAt); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&agentv1.AcknowledgeTaskResponse{LeaseExpiresAt: timestamppb.New(expiresAt)}), nil
}

func (server *agentTaskServer) RenewTaskLease(ctx context.Context, req *connect.Request[agentv1.RenewTaskLeaseRequest]) (*connect.Response[agentv1.RenewTaskLeaseResponse], error) {
	nodeID, err := validateTaskExecutionRequest(ctx, req.Msg.GetTaskId(), req.Msg.GetExecutionId())
	if err != nil {
		return nil, err
	}
	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if detail.Record.NodeID != nodeID {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("task does not belong to authenticated node"))
	}
	now := time.Now().UTC()
	expiresAt := now.Add(taskExecutionLease)
	if err := server.db.RenewTaskExecution(ctx, req.Msg.GetTaskId(), req.Msg.GetExecutionId(), now, expiresAt); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&agentv1.RenewTaskLeaseResponse{LeaseExpiresAt: timestamppb.New(expiresAt)}), nil
}

func validateTaskExecutionRequest(ctx context.Context, taskID, executionID string) (string, error) {
	if taskID == "" || executionID == "" {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("task_id and execution_id are required"))
	}
	nodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || nodeID == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer subject"))
	}
	return nodeID, nil
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
