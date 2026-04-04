package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

const taskWorkerPollInterval = 1 * time.Second

type taskExecutor func(context.Context, task.Record) error

func startTaskWorker(ctx context.Context, db *store.DB, execute taskExecutor) {
	go runTaskWorker(ctx, db, execute)
}

func runTaskWorker(ctx context.Context, db *store.DB, execute taskExecutor) {
	ticker := time.NewTicker(taskWorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runSingleTask(ctx, db, execute)
		}
	}
}

func runSingleTask(ctx context.Context, db *store.DB, execute taskExecutor) {
	record, err := db.ClaimNextPendingTask(ctx, time.Now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrNoPendingTask) {
			return
		}
		log.Printf("task worker claim failed: %v", err)
		return
	}

	status := task.StatusSucceeded
	errorSummary := ""
	if err := execute(ctx, record); err != nil {
		status = task.StatusFailed
		errorSummary = err.Error()
	}

	if err := db.CompleteTask(ctx, record.TaskID, status, time.Now().UTC(), errorSummary); err != nil {
		log.Printf("task worker completion failed for %s: %v", record.TaskID, err)
	}
}

func executeTask(ctx context.Context, db *store.DB, record task.Record) error {
	switch record.Type {
	case task.TypeDeploy:
		return executeDeployTask(ctx, db, record)
	default:
		return fmt.Errorf("task type %q is not implemented", record.Type)
	}
}

func executeDeployTask(ctx context.Context, db *store.DB, record task.Record) error {
	if err := appendTaskLog(record.LogPath, fmt.Sprintf("starting deploy task for service=%s node=%s repo_revision=%s", record.ServiceName, record.NodeID, record.RepoRevision)); err != nil {
		return err
	}

	if err := runTaskStep(ctx, db, record, task.StepRender, func() error {
		return appendTaskLog(record.LogPath, "render step completed with placeholder executor")
	}); err != nil {
		return err
	}

	if err := runTaskStep(ctx, db, record, task.StepFinalize, func() error {
		return appendTaskLog(record.LogPath, "finalize step completed with placeholder executor")
	}); err != nil {
		return err
	}

	return appendTaskLog(record.LogPath, "deploy task finished successfully")
}

func runTaskStep(ctx context.Context, db *store.DB, record task.Record, stepName task.StepName, execute func() error) error {
	startedAt := time.Now().UTC()
	if err := db.UpsertTaskStep(ctx, task.StepRecord{
		TaskID:    record.TaskID,
		StepName:  stepName,
		Status:    task.StatusRunning,
		StartedAt: &startedAt,
	}); err != nil {
		return err
	}
	if err := appendTaskLog(record.LogPath, fmt.Sprintf("step %s started", stepName)); err != nil {
		return err
	}

	if err := execute(); err != nil {
		finishedAt := time.Now().UTC()
		_ = db.UpsertTaskStep(ctx, task.StepRecord{
			TaskID:     record.TaskID,
			StepName:   stepName,
			Status:     task.StatusFailed,
			StartedAt:  &startedAt,
			FinishedAt: &finishedAt,
		})
		_ = appendTaskLog(record.LogPath, fmt.Sprintf("step %s failed: %v", stepName, err))
		return err
	}

	finishedAt := time.Now().UTC()
	if err := db.UpsertTaskStep(ctx, task.StepRecord{
		TaskID:     record.TaskID,
		StepName:   stepName,
		Status:     task.StatusSucceeded,
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	}); err != nil {
		return err
	}
	if err := appendTaskLog(record.LogPath, fmt.Sprintf("step %s succeeded", stepName)); err != nil {
		return err
	}
	return nil
}

func appendTaskLog(logPath, message string) error {
	if logPath == "" {
		return nil
	}
	return appendTaskLogRaw(logPath, fmt.Sprintf("%s %s\n", time.Now().UTC().Format(time.RFC3339), message))
}

func appendTaskLogRaw(logPath, content string) error {
	if logPath == "" || content == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("create task log directory: %w", err)
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open task log %q: %w", logPath, err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("write task log %q: %w", logPath, err)
	}
	return nil
}
