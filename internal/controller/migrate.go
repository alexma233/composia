package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"github.com/google/uuid"
)

func (server *serviceCommandServer) MigrateService(ctx context.Context, req *connect.Request[controllerv1.MigrateServiceRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" || req.Msg.GetSourceNodeId() == "" || req.Msg.GetTargetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name, source_node_id, and target_node_id are required"))
	}
	if req.Msg.GetSourceNodeId() == req.Msg.GetTargetNodeId() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_node_id and target_node_id must differ"))
	}

	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if !slices.Contains(service.TargetNodes, req.Msg.GetSourceNodeId()) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q is not declared on source node %q", service.Name, req.Msg.GetSourceNodeId()))
	}
	if slices.Contains(service.TargetNodes, req.Msg.GetTargetNodeId()) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q is already declared on target node %q", service.Name, req.Msg.GetTargetNodeId()))
	}
	if err := validateTaskTargetNode(ctx, server.db, server.cfg, req.Msg.GetSourceNodeId(), task.TypeBackup); err != nil {
		return nil, err
	}
	if err := validateTaskTargetNode(ctx, server.db, server.cfg, req.Msg.GetTargetNodeId(), task.TypeRestore); err != nil {
		return nil, err
	}
	repoRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{
		ServiceDir:   serviceDir,
		DataNames:    enabledMigrateDataNames(service),
		SourceNodeID: req.Msg.GetSourceNodeId(),
		TargetNodeID: req.Msg.GetTargetNodeId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("encode migrate task params: %w", err))
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	createdTask, err := server.db.CreateTaskWithConstraints(ctx, task.Record{
		TaskID:       taskID,
		Type:         task.TypeMigrate,
		Source:       requestTaskSource(req.Header()),
		TriggeredBy:  triggeredBy,
		ServiceName:  service.Name,
		Status:       task.StatusPending,
		ParamsJSON:   string(paramsJSON),
		RepoRevision: repoRevision,
		LogPath:      filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	}, store.TaskAdmissionConstraints{
		RequireInactiveService: true,
		RequireInactiveServiceInstances: []store.ServiceInstanceTarget{
			{ServiceName: service.Name, NodeID: req.Msg.GetSourceNodeId()},
			{ServiceName: service.Name, NodeID: req.Msg.GetTargetNodeId()},
		},
	})
	if err != nil {
		return nil, connectTaskAdmissionError(err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(taskActionResponse(createdTask)), nil
}

func enabledMigrateDataNames(service repo.Service) []string {
	if service.Meta.Migrate == nil {
		return nil
	}
	names := make([]string, 0, len(service.Meta.Migrate.Data))
	for _, item := range service.Meta.Migrate.Data {
		if item.Name == "" || (item.Enabled != nil && !*item.Enabled) {
			continue
		}
		names = append(names, item.Name)
	}
	return names
}

func (executor *controllerTaskExecutor) executeMigrateTask(ctx context.Context, record task.Record) error {
	params := taskParams(record.ParamsJSON)
	if params.ServiceDir == "" || params.SourceNodeID == "" || params.TargetNodeID == "" {
		return executor.failControllerTask(ctx, record, task.StepFinalize, errors.New("migrate task is missing service_dir or source/target node ids"))
	}
	detail, err := executor.db.GetTask(ctx, record.TaskID)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}
	stepSucceeded := func(stepName task.StepName) bool {
		return hasTaskStepStatus(detail.Steps, stepName, task.StatusSucceeded)
	}
	if err := appendTaskLogRaw(record.LogPath, fmt.Sprintf("starting controller migrate task for service=%s source=%s target=%s repo_revision=%s\n", record.ServiceName, params.SourceNodeID, params.TargetNodeID, record.RepoRevision)); err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}
	service, err := repo.FindServiceAtRevision(executor.cfg.RepoDir, record.RepoRevision, params.ServiceDir, executor.availableNodeIDs)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}

	restoreItems := append([]restoreTaskItem(nil), params.RestoreItems...)
	if !stepSucceeded(task.StepComposeDown) {
		if err := executor.runMigrateStep(ctx, record, task.StepComposeDown, func() error {
			stopTask, err := createServiceTaskWithOptions(ctx, executor.db, executor.cfg, executor.availableNodeIDs, record.ServiceName, []string{params.SourceNodeID}, task.TypeStop, nil, serviceTaskCreateOptions{Source: record.Source})
			if err != nil {
				return err
			}
			return executor.waitTask(ctx, stopTask.TaskID, 5*time.Minute)
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepComposeDown, err)
		}
	}

	if len(params.DataNames) > 0 && !stepSucceeded(task.StepBackup) {
		if err := executor.runMigrateStep(ctx, record, task.StepBackup, func() error {
			backupTask, err := createServiceTaskWithOptions(ctx, executor.db, executor.cfg, executor.availableNodeIDs, record.ServiceName, []string{params.SourceNodeID}, task.TypeBackup, params.DataNames, serviceTaskCreateOptions{Source: record.Source})
			if err != nil {
				return err
			}
			if err := executor.waitTask(ctx, backupTask.TaskID, 10*time.Minute); err != nil {
				return err
			}
			backups, err := executor.db.ListBackupsForTask(ctx, backupTask.TaskID)
			if err != nil {
				return err
			}
			byName := make(map[string]store.BackupDetail, len(backups))
			for _, backup := range backups {
				if backup.Status == string(task.StatusSucceeded) {
					byName[backup.DataName] = backup
				}
			}
			for _, dataName := range params.DataNames {
				backup, ok := byName[dataName]
				if !ok || backup.ArtifactRef == "" {
					return fmt.Errorf("backup artifact for %s was not recorded", dataName)
				}
				restoreItems = append(restoreItems, restoreTaskItem{DataName: dataName, ArtifactRef: backup.ArtifactRef, SourceTaskID: backup.TaskID})
			}
			params.RestoreItems = restoreItems
			paramsJSON, err := json.Marshal(params)
			if err != nil {
				return err
			}
			if err := executor.db.UpdateTaskParamsJSON(ctx, record.TaskID, string(paramsJSON)); err != nil {
				return err
			}
			return appendTaskLogRaw(record.LogPath, fmt.Sprintf("backup step captured %d artifact(s)\n", len(restoreItems)))
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepBackup, err)
		}
	}

	if len(restoreItems) > 0 && !stepSucceeded(task.StepRestore) {
		if err := executor.runMigrateStep(ctx, record, task.StepRestore, func() error {
			restoreTask, err := createRestoreTask(ctx, executor.db, executor.cfg, executor.availableNodeIDs, record.ServiceName, params.TargetNodeID, record.RepoRevision, serviceTaskParams{ServiceDir: params.ServiceDir, RestoreItems: restoreItems}, record.Source)
			if err != nil {
				return err
			}
			notifyTaskQueue(executor.taskQueue)
			return executor.waitTask(ctx, restoreTask.TaskID, 10*time.Minute)
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepRestore, err)
		}
	}

	if !stepSucceeded(task.StepComposeUp) {
		if err := executor.runMigrateStep(ctx, record, task.StepComposeUp, func() error {
			deployTask, err := createServiceTaskWithOptions(ctx, executor.db, executor.cfg, executor.availableNodeIDs, record.ServiceName, []string{params.TargetNodeID}, task.TypeDeploy, nil, serviceTaskCreateOptions{Source: record.Source})
			if err != nil {
				return err
			}
			if err := executor.waitTask(ctx, deployTask.TaskID, 5*time.Minute); err != nil {
				return err
			}
			if repo.CaddyManaged(service) {
				reloadTask, err := createNodeCaddyReloadTask(ctx, executor.db, executor.cfg, executor.availableNodeIDs, params.TargetNodeID, record.Source)
				if err != nil {
					return err
				}
				if err := executor.waitTask(ctx, reloadTask.TaskID, 2*time.Minute); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepComposeUp, err)
		}
	}

	if !stepSucceeded(task.StepAwaitingConfirmation) {
		if err := executor.enterMigrateAwaitingConfirmation(ctx, record); err != nil {
			return executor.failControllerTask(ctx, record, task.StepAwaitingConfirmation, err)
		}
		return nil
	}

	if service.Meta.Network != nil && service.Meta.Network.DNS != nil && !stepSucceeded(task.StepDNSUpdate) {
		if err := executor.runMigrateStep(ctx, record, task.StepDNSUpdate, func() error {
			client, err := executor.dnsProviders.Cloudflare(executor.cfg)
			if err != nil {
				return err
			}
			migratedService := service
			migratedService.TargetNodes = migrateTargetNodes(service.TargetNodes, params.SourceNodeID, params.TargetNodeID)
			desired, err := buildDesiredServiceDNS(ctx, migratedService, executor.cfg, client)
			if err != nil {
				return err
			}
			return syncServiceDNS(ctx, client, desired, record.LogPath)
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
		}
	}

	if !stepSucceeded(task.StepPersistRepo) {
		if err := executor.runMigrateStep(ctx, record, task.StepPersistRepo, func() error {
			return executor.persistMigratedTargetNodes(ctx, record, service, params.SourceNodeID, params.TargetNodeID)
		}); err != nil {
			return executor.failControllerTask(ctx, record, task.StepPersistRepo, err)
		}
	}

	finishedAt := time.Now().UTC()
	if err := executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: task.StepFinalize, Status: task.StatusSucceeded, StartedAt: &finishedAt, FinishedAt: &finishedAt}); err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}
	if err := appendTaskLogRaw(record.LogPath, "migrate task finished successfully\n"); err != nil {
		return executor.failControllerTask(ctx, record, task.StepFinalize, err)
	}
	return executor.db.CompleteTask(ctx, record.TaskID, task.StatusSucceeded, finishedAt, "")
}

func hasTaskStepStatus(steps []task.StepRecord, stepName task.StepName, status task.Status) bool {
	for _, step := range steps {
		if step.StepName == stepName && step.Status == status {
			return true
		}
	}
	return false
}

func (executor *controllerTaskExecutor) enterMigrateAwaitingConfirmation(ctx context.Context, record task.Record) error {
	startedAt := time.Now().UTC()
	if err := executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: task.StepAwaitingConfirmation, Status: task.StatusAwaitingConfirmation, StartedAt: &startedAt}); err != nil {
		return err
	}
	if err := appendTaskLogRaw(record.LogPath, "migrate task is awaiting manual confirmation before dns_update\n"); err != nil {
		return err
	}
	return executor.db.TransitionTaskStatus(ctx, record.TaskID, task.StatusRunning, task.StatusAwaitingConfirmation, "")
}

func createRestoreTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, serviceName, nodeID, repoRevision string, params serviceTaskParams, source task.Source) (task.Record, error) {
	if err := validateTaskTargetNode(ctx, db, cfg, nodeID, task.TypeRestore); err != nil {
		return task.Record{}, err
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode restore task params: %w", err))
	}
	taskID := uuid.NewString()
	createdTask, err := db.CreateTaskIfNoActiveServiceInstanceTask(ctx, task.Record{TaskID: taskID, Type: task.TypeRestore, Source: source, ServiceName: serviceName, NodeID: nodeID, Status: task.StatusPending, ParamsJSON: string(paramsJSON), RepoRevision: repoRevision, LogPath: filepath.Join(cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID))})
	if err != nil {
		return task.Record{}, connectTaskAdmissionError(err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	return createdTask, nil
}

func (executor *controllerTaskExecutor) waitTask(ctx context.Context, taskID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	waitCh := executor.taskResults.Subscribe(taskID)
	defer executor.taskResults.Unsubscribe(taskID, waitCh)
	for {
		detail, err := executor.db.GetTask(ctx, taskID)
		if err == nil {
			switch detail.Record.Status {
			case task.StatusSucceeded:
				return nil
			case task.StatusFailed, task.StatusCancelled:
				return fmt.Errorf("task %s failed: %s", taskID, detail.Record.ErrorSummary)
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timeout waiting for task %s", taskID)
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-waitCh:
			timer.Stop()
		case <-timer.C:
			return fmt.Errorf("timeout waiting for task %s", taskID)
		}
	}
}

func (executor *controllerTaskExecutor) runMigrateStep(ctx context.Context, record task.Record, stepName task.StepName, run func() error) error {
	startedAt := time.Now().UTC()
	if err := executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: stepName, Status: task.StatusRunning, StartedAt: &startedAt}); err != nil {
		return err
	}
	if err := run(); err != nil {
		return err
	}
	finishedAt := time.Now().UTC()
	return executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: stepName, Status: task.StatusSucceeded, StartedAt: &startedAt, FinishedAt: &finishedAt})
}

func (executor *controllerTaskExecutor) persistMigratedTargetNodes(ctx context.Context, record task.Record, service repo.Service, sourceNodeID, targetNodeID string) error {
	if executor.repoMu == nil {
		return errors.New("repo mutex is not configured")
	}
	currentService, err := repo.FindService(executor.cfg.RepoDir, executor.availableNodeIDs, service.Name)
	if err != nil {
		return err
	}
	updatedContent, err := repo.RewriteServiceTargetNodes(currentService.MetaPath, migrateTargetNodes(service.TargetNodes, sourceNodeID, targetNodeID), executor.availableNodeIDs)
	if err != nil {
		return err
	}
	relativeMetaPath, err := filepath.Rel(executor.cfg.RepoDir, currentService.MetaPath)
	if err != nil {
		return fmt.Errorf("resolve service meta path: %w", err)
	}
	repoSrv := &repoCommandServer{db: executor.db, cfg: executor.cfg, availableNodeIDs: executor.availableNodeIDs, repoMu: executor.repoMu}
	executor.repoMu.Lock()
	defer executor.repoMu.Unlock()
	if err := repoSrv.syncRepoBeforeWrite(ctx); err != nil {
		return err
	}
	if err := repoSrv.verifyRepoWriteBaseRevision(record.RepoRevision); err != nil {
		return err
	}
	if err := repoSrv.ensureCleanWorktree(); err != nil {
		return err
	}
	_, err = repoSrv.updateRepoFileTransaction(ctx, relativeMetaPath, updatedContent, fmt.Sprintf("migrate %s from %s to %s", service.Name, sourceNodeID, targetNodeID), store.RepoSyncState{})
	return err
}

func migrateTargetNodes(nodes []string, sourceNodeID, targetNodeID string) []string {
	updated := append([]string(nil), nodes...)
	for index, nodeID := range updated {
		if nodeID == sourceNodeID {
			updated[index] = targetNodeID
			return updated
		}
	}
	return updated
}
