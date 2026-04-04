package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestRunSingleTaskMarksTaskSucceeded(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	created, err := db.CreateTask(ctx, task.Record{
		TaskID:    "task-success",
		Type:      task.TypeDeploy,
		Source:    task.SourceCLI,
		CreatedAt: time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	runSingleTask(ctx, db, func(context.Context, task.Record) error {
		return nil
	})

	row := db.SQL().QueryRowContext(ctx, `SELECT status, finished_at FROM tasks WHERE task_id = ?`, created.TaskID)
	var status string
	var finishedAt string
	if err := row.Scan(&status, &finishedAt); err != nil {
		t.Fatalf("scan completed task: %v", err)
	}
	if status != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded status, got %q", status)
	}
	if finishedAt == "" {
		t.Fatalf("expected finished_at to be set")
	}
}

func TestRunSingleTaskMarksTaskFailedOnExecutorError(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	created, err := db.CreateTask(ctx, task.Record{
		TaskID:    "task-failed",
		Type:      task.TypeDeploy,
		Source:    task.SourceCLI,
		CreatedAt: time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	runSingleTask(ctx, db, func(context.Context, task.Record) error {
		return assertError("executor failed")
	})

	row := db.SQL().QueryRowContext(ctx, `SELECT status, error_summary FROM tasks WHERE task_id = ?`, created.TaskID)
	var status string
	var errorSummary string
	if err := row.Scan(&status, &errorSummary); err != nil {
		t.Fatalf("scan failed task: %v", err)
	}
	if status != string(task.StatusFailed) {
		t.Fatalf("expected failed status, got %q", status)
	}
	if errorSummary != "executor failed" {
		t.Fatalf("expected error summary executor failed, got %q", errorSummary)
	}
}

func TestExecuteDeployTaskWritesStepsAndLogs(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "tasks", "deploy.log")
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:       "task-deploy",
		Type:         task.TypeDeploy,
		Source:       task.SourceCLI,
		ServiceName:  "demo",
		NodeID:       "main",
		Status:       task.StatusPending,
		CreatedAt:    time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC),
		RepoRevision: "deadbeef",
		LogPath:      logPath,
	}); err != nil {
		t.Fatalf("create deploy task: %v", err)
	}

	runSingleTask(ctx, db, func(execCtx context.Context, record task.Record) error {
		return executeTask(execCtx, db, record)
	})

	detail, err := db.GetTask(ctx, "task-deploy")
	if err != nil {
		t.Fatalf("get deploy task: %v", err)
	}
	if detail.Record.Status != task.StatusSucceeded {
		t.Fatalf("expected succeeded deploy task, got %q", detail.Record.Status)
	}
	if len(detail.Steps) != 2 {
		t.Fatalf("expected 2 task steps, got %d", len(detail.Steps))
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read task log: %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("expected task log content")
	}
}

func openControllerTestDB(t *testing.T) *store.DB {
	t.Helper()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}
