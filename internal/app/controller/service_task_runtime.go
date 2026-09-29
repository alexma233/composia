package controller

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func (server *bundleServer) GetServiceTaskRuntime(ctx context.Context, req *connect.Request[agentv1.GetServiceTaskRuntimeRequest]) (*connect.Response[agentv1.GetServiceTaskRuntimeResponse], error) {
	record, params, err := server.serviceTask(ctx, req.Msg.GetTaskId(), req.Msg.GetExecutionId())
	if err != nil {
		return nil, err
	}
	if record.Type != task.TypeBackup && record.Type != task.TypeRestore {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("runtime parameters require a backup or restore task"))
	}
	if record.RepoRevision == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("runtime parameters require a pinned repo revision"))
	}
	params.ServiceDir, err = authorizedBundleServiceDir(server.cfg, record, params, "")
	if err != nil {
		return nil, err
	}
	service, err := repo.FindServiceAtRevision(server.cfg.RepoDir, record.RepoRevision, params.ServiceDir, configuredNodeIDs(server.cfg))
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if service.Name != record.ServiceName {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("service directory does not match task service"))
	}
	if service.Meta.DataProtect == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("service does not declare data protection"))
	}
	var payload string
	if record.Type == task.TypeBackup {
		payload, err = buildBackupRuntimePayload(server.cfg, record.ServiceName, record.NodeID, record.RepoRevision, params)
	} else {
		payload, err = buildRestoreRuntimePayload(server.cfg, record.ServiceName, record.NodeID, record.RepoRevision, params)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&agentv1.GetServiceTaskRuntimeResponse{RepoRevision: record.RepoRevision, ServiceDir: params.ServiceDir, ConfigJson: payload}), nil
}
