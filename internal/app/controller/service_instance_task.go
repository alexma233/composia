package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"github.com/google/uuid"
)

func (executor *controllerTaskExecutor) createServiceInstanceTask(ctx context.Context, serviceName, nodeID string, taskType task.Type, params serviceTaskParams, repoRevision string, source task.Source) (task.Record, error) {
	if serviceName == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if nodeID == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	if params.ServiceDir == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("service_dir is required"))
	}
	if err := validateTaskTargetNode(ctx, executor.db, executor.cfg, nodeID, taskType); err != nil {
		return task.Record{}, err
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode task params: %w", err))
	}
	taskSource := source
	if taskSource == "" {
		taskSource = task.SourceSystem
	}
	taskID := uuid.NewString()
	createdTask, err := executor.db.CreateTaskIfNoActiveServiceInstanceTask(ctx, task.Record{
		TaskID:       taskID,
		Type:         taskType,
		Source:       taskSource,
		ServiceName:  serviceName,
		NodeID:       nodeID,
		Status:       task.StatusPending,
		ParamsJSON:   string(paramsJSON),
		RepoRevision: repoRevision,
		LogPath:      filepath.Join(executor.cfg.LogDir, "tasks", taskID+".log"),
	})
	if err != nil {
		return task.Record{}, connectTaskAdmissionError(err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o600); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	notifyTaskQueue(executor.taskQueue)
	return createdTask, nil
}
