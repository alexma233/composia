package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"github.com/google/uuid"
)

func (server *taskServer) CreateMigrationRollback(ctx context.Context, req *connect.Request[controllerv1.CreateMigrationRollbackRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.GetCleanupTarget() {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("cleanup_target is not implemented yet"))
	}
	if !req.Msg.GetRollbackDns() && !req.Msg.GetDeploySource() && !req.Msg.GetStopTarget() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one rollback action is required"))
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if detail.Record.Type != task.TypeMigrate {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task type %q does not support migration rollback", detail.Record.Type))
	}
	if detail.Record.Status != task.StatusAwaitingConfirmation && detail.Record.Status != task.StatusCancelled && detail.Record.Status != task.StatusFailed {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task %q is not eligible for rollback", detail.Record.TaskID))
	}

	params, err := taskParams(detail.Record.ParamsJSON)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if params.ServiceDir == "" || params.SourceNodeID == "" || params.TargetNodeID == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("migrate task is missing service_dir or source/target node ids"))
	}
	if err := validateMigrationRollbackActions(req.Msg, detail); err != nil {
		return nil, err
	}

	if detail.Record.Status == task.StatusAwaitingConfirmation {
		if err := server.cancelAwaitingMigrationForRollback(ctx, detail); err != nil {
			return nil, err
		}
	}

	rollbackParams := serviceTaskParams{
		ServiceDir:            params.ServiceDir,
		SourceNodeID:          params.SourceNodeID,
		TargetNodeID:          params.TargetNodeID,
		OriginalMigrateTaskID: detail.Record.TaskID,
		RollbackDNS:           req.Msg.GetRollbackDns(),
		DeploySource:          req.Msg.GetDeploySource(),
		StopTarget:            req.Msg.GetStopTarget(),
		CleanupTarget:         req.Msg.GetCleanupTarget(),
	}
	paramsJSON, err := json.Marshal(rollbackParams)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("encode rollback task params: %w", err))
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	createdTask, err := server.db.CreateTaskWithConstraints(ctx, task.Record{
		TaskID:          taskID,
		Type:            task.TypeMigrateRollback,
		Source:          requestTaskSource(req.Header()),
		TriggeredBy:     triggeredBy,
		ServiceName:     detail.Record.ServiceName,
		Status:          task.StatusPending,
		ParamsJSON:      string(paramsJSON),
		RepoRevision:    detail.Record.RepoRevision,
		AttemptOfTaskID: detail.Record.TaskID,
		LogPath:         filepath.Join(server.cfg.LogDir, "tasks", taskID+".log"),
	}, store.TaskAdmissionConstraints{RequireInactiveService: true})
	if err != nil {
		return nil, connectTaskAdmissionError(err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o600); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(taskActionResponse(createdTask)), nil
}

func validateMigrationRollbackActions(req *controllerv1.CreateMigrationRollbackRequest, detail store.TaskDetail) error {
	stepSucceeded := func(stepName task.StepName) bool {
		return hasTaskStepStatus(detail.Steps, stepName, task.StatusSucceeded)
	}
	if req.GetRollbackDns() && !stepSucceeded(task.StepDNSUpdate) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("rollback_dns requires a succeeded dns_update step"))
	}
	if req.GetDeploySource() && !stepSucceeded(task.StepComposeDown) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("deploy_source requires a succeeded compose_down step"))
	}
	if req.GetStopTarget() && !stepSucceeded(task.StepComposeUp) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("stop_target requires a succeeded compose_up step"))
	}
	return nil
}

func (server *taskServer) cancelAwaitingMigrationForRollback(ctx context.Context, detail store.TaskDetail) error {
	finishedAt := time.Now().UTC()
	if err := appendTaskLogRaw(detail.Record.LogPath, "migration rollback requested; cancelling awaiting migration before rollback\n"); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if err := server.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: detail.Record.TaskID, StepName: task.StepAwaitingConfirmation, Status: task.StatusCancelled, StartedAt: findTaskStepStartedAt(detail.Steps, task.StepAwaitingConfirmation), FinishedAt: &finishedAt}); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if err := server.db.CompleteTask(ctx, detail.Record.TaskID, task.StatusCancelled, finishedAt, "migration rollback requested"); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	detail.Record.Status = task.StatusCancelled
	detail.Record.FinishedAt = &finishedAt
	detail.Record.ErrorSummary = "migration rollback requested"
	notifyTaskResult(server.taskResults, detail.Record.TaskID)
	dispatchTaskRecordNotification(server.notifier, notify.EventTaskCancelled, detail.Record)
	return nil
}

func (executor *controllerTaskExecutor) executeMigrateRollbackTask(ctx context.Context, record task.Record) error {
	params, err := taskParams(record.ParamsJSON)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}
	if params.ServiceDir == "" || params.SourceNodeID == "" || params.TargetNodeID == "" || params.OriginalMigrateTaskID == "" {
		return executor.failControllerTask(ctx, record, task.StepFinalize, errors.New("migrate rollback task is missing service_dir, source/target node ids, or original task id"))
	}
	service, err := repo.FindServiceAtRevision(executor.cfg.RepoDir, record.RepoRevision, params.ServiceDir, executor.availableNodeIDs)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}
	if err := appendTaskLogRaw(record.LogPath, fmt.Sprintf("starting migrate rollback task for service=%s source=%s target=%s original_task=%s rollback_dns=%t deploy_source=%t stop_target=%t\n", record.ServiceName, params.SourceNodeID, params.TargetNodeID, params.OriginalMigrateTaskID, params.RollbackDNS, params.DeploySource, params.StopTarget)); err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}

	if params.RollbackDNS {
		if err := executor.runMigrateStep(ctx, record, task.StepDNSUpdate, func() error {
			if service.Meta.Network == nil || service.Meta.Network.DNS == nil {
				return errors.New("service does not declare network.dns")
			}
			client, err := executor.dnsProviders.ForService(executor.cfg, service.Meta.Network.DNS.Provider)
			if err != nil {
				return err
			}
			desired, err := buildDesiredServiceDNS(ctx, service, executor.cfg, client)
			if err != nil {
				return err
			}
			return syncServiceDNS(ctx, client, desired, record.LogPath)
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
		}
	}

	if params.DeploySource {
		if err := executor.runMigrateStep(ctx, record, task.StepComposeUp, func() error {
			deployTask, err := executor.createServiceInstanceTask(ctx, record.ServiceName, params.SourceNodeID, task.TypeDeploy, serviceTaskParams{ServiceDir: params.ServiceDir}, record.RepoRevision, record.Source)
			if err != nil {
				return err
			}
			if err := executor.waitTask(ctx, deployTask.TaskID, 5*time.Minute); err != nil {
				return err
			}
			if repo.CaddyManaged(service) {
				reloadTask, err := createNodeCaddyReloadTask(ctx, executor.db, executor.cfg, executor.availableNodeIDs, params.SourceNodeID, record.Source)
				if err != nil {
					return err
				}
				return executor.waitTask(ctx, reloadTask.TaskID, 2*time.Minute)
			}
			return nil
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepComposeUp, err)
		}
	}

	if params.StopTarget {
		if err := executor.runMigrateStep(ctx, record, task.StepComposeDown, func() error {
			stopTask, err := executor.createServiceInstanceTask(ctx, record.ServiceName, params.TargetNodeID, task.TypeStop, serviceTaskParams{ServiceDir: params.ServiceDir}, record.RepoRevision, record.Source)
			if err != nil {
				return err
			}
			if err := executor.waitTask(ctx, stopTask.TaskID, 5*time.Minute); err != nil {
				return err
			}
			if repo.CaddyManaged(service) {
				reloadTask, err := createNodeCaddyReloadTask(ctx, executor.db, executor.cfg, executor.availableNodeIDs, params.TargetNodeID, record.Source)
				if err != nil {
					return err
				}
				return executor.waitTask(ctx, reloadTask.TaskID, 2*time.Minute)
			}
			return nil
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepComposeDown, err)
		}
	}

	startedAt := time.Now().UTC()
	logControllerTaskStepStarted(record, task.StepFinalize)
	finishedAt := startedAt
	if err := executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: task.StepFinalize, Status: task.StatusSucceeded, StartedAt: &startedAt, FinishedAt: &finishedAt}); err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}
	logControllerTaskStepSucceeded(record, task.StepFinalize, startedAt, finishedAt)
	if err := appendTaskLogRaw(record.LogPath, "migrate rollback task finished successfully\n"); err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}
	if err := executor.db.CompleteTask(ctx, record.TaskID, task.StatusSucceeded, finishedAt, ""); err != nil {
		return err
	}
	record.Status = task.StatusSucceeded
	record.FinishedAt = &finishedAt
	notifyTaskResult(executor.taskResults, record.TaskID)
	dispatchTaskRecordNotification(executor.notifier, notify.EventTaskCompleted, record)
	logControllerTaskFinished(record, finishedAt)
	return nil
}
