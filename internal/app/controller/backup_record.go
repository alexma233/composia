package controller

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"connectrpc.com/connect"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

type backupServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
}

func (server *backupServer) ListBackups(ctx context.Context, req *connect.Request[controllerv1.ListBackupsRequest]) (*connect.Response[controllerv1.ListBackupsResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListBackupsRequest{}
	}
	backups, totalCount, err := server.db.ListBackups(ctx, req.Msg.GetServiceName(), req.Msg.GetStatus(), req.Msg.GetDataName(), req.Msg.GetNodeId(), req.Msg.GetExcludeServiceName(), req.Msg.GetExcludeStatus(), req.Msg.GetExcludeDataName(), req.Msg.GetExcludeNodeId(), req.Msg.GetPage(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.ListBackupsResponse{
		Backups:    make([]*controllerv1.BackupSummary, 0, len(backups)),
		TotalCount: totalCount,
	}
	for _, backup := range backups {
		response.Backups = append(response.Backups, backupSummaryMessage(backup))
	}
	return connect.NewResponse(response), nil
}

func (server *backupServer) GetBackup(ctx context.Context, req *connect.Request[controllerv1.GetBackupRequest]) (*connect.Response[controllerv1.GetBackupResponse], error) {
	if req.Msg == nil || req.Msg.GetBackupId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("backup_id is required"))
	}
	backup, err := server.db.GetBackup(ctx, req.Msg.GetBackupId())
	if err != nil {
		if errors.Is(err, store.ErrBackupNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetBackupResponse{
		BackupId:     backup.BackupID,
		TaskId:       backup.TaskID,
		ServiceName:  backup.ServiceName,
		NodeId:       backup.NodeID,
		DataName:     backup.DataName,
		Status:       backup.Status,
		StartedAt:    backup.StartedAt,
		FinishedAt:   backup.FinishedAt,
		ArtifactRef:  backup.ArtifactRef,
		ErrorSummary: backup.ErrorSummary,
	}
	snapshotByNodeID, err := buildNodeSnapshotMap(ctx, server.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response.Actions = buildBackupActionCapabilities(server.cfg, server.availableNodeIDs, snapshotByNodeID, backup)
	return connect.NewResponse(response), nil
}

func (server *backupServer) RestoreBackup(ctx context.Context, req *connect.Request[controllerv1.RestoreBackupRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetBackupId() == "" || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("backup_id and node_id are required"))
	}
	if server.cfg == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("controller config is required"))
	}
	backup, err := server.db.GetBackup(ctx, req.Msg.GetBackupId())
	if err != nil {
		if errors.Is(err, store.ErrBackupNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if backup.Status != string(task.StatusSucceeded) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("backup %q is not in succeeded state", backup.BackupID))
	}
	if backup.ArtifactRef == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("backup %q does not have an artifact_ref", backup.BackupID))
	}
	if err := validateTaskTargetNode(ctx, server.db, server.cfg, req.Msg.GetNodeId(), task.TypeRestore); err != nil {
		return nil, err
	}
	active, err := server.db.HasActiveServiceInstanceTask(ctx, backup.ServiceName, req.Msg.GetNodeId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if active {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service instance %q@%q already has an active task", backup.ServiceName, req.Msg.GetNodeId()))
	}
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, backup.ServiceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	var restoreDefinition *repo.DataActionConfig
	for _, item := range service.Meta.DataProtect.Data {
		if item.Name != backup.DataName {
			continue
		}
		restoreDefinition = item.Restore
		break
	}
	if restoreDefinition == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q data %q does not declare restore", service.Name, backup.DataName))
	}
	repoRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
	}
	createdTask, err := createRestoreTask(ctx, server.db, server.cfg, server.availableNodeIDs, backup.ServiceName, req.Msg.GetNodeId(), repoRevision, serviceTaskParams{ServiceDir: serviceDir, RestoreItems: []restoreTaskItem{{DataName: backup.DataName, ArtifactRef: backup.ArtifactRef, SourceTaskID: backup.TaskID}}}, requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(taskActionResponse(createdTask)), nil
}

func backupSummaryMessage(backup store.BackupSummary) *controllerv1.BackupSummary {
	return &controllerv1.BackupSummary{
		BackupId:    backup.BackupID,
		TaskId:      backup.TaskID,
		ServiceName: backup.ServiceName,
		NodeId:      backup.NodeID,
		DataName:    backup.DataName,
		Status:      backup.Status,
		StartedAt:   backup.StartedAt,
		FinishedAt:  backup.FinishedAt,
	}
}
