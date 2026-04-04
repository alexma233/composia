package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestClaimNextPendingTaskReturnsOldestPendingTask(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	older := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	newer := older.Add(5 * time.Minute)

	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-a", Type: task.TypeDeploy, Source: task.SourceCLI, CreatedAt: older}); err != nil {
		t.Fatalf("create older task: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-b", Type: task.TypeDeploy, Source: task.SourceCLI, CreatedAt: newer}); err != nil {
		t.Fatalf("create newer task: %v", err)
	}

	claimed, err := db.ClaimNextPendingTask(ctx, newer.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("claim next pending task: %v", err)
	}
	if claimed.TaskID != "task-a" {
		t.Fatalf("expected oldest task task-a, got %q", claimed.TaskID)
	}
	if claimed.Status != task.StatusRunning {
		t.Fatalf("expected claimed task to be running, got %q", claimed.Status)
	}
}

func TestClaimNextPendingTaskForNodeHonorsNodeAndGlobalRunningTask(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "node-2"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	createdAt := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-main", Type: task.TypeDeploy, Source: task.SourceCLI, NodeID: "main", CreatedAt: createdAt}); err != nil {
		t.Fatalf("create main task: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-node-2", Type: task.TypeDeploy, Source: task.SourceCLI, NodeID: "node-2", CreatedAt: createdAt.Add(1 * time.Minute)}); err != nil {
		t.Fatalf("create node-2 task: %v", err)
	}

	claimed, err := db.ClaimNextPendingTaskForNode(ctx, "main", createdAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("claim main task: %v", err)
	}
	if claimed.TaskID != "task-main" {
		t.Fatalf("expected task-main, got %q", claimed.TaskID)
	}

	_, err = db.ClaimNextPendingTaskForNode(ctx, "node-2", createdAt.Add(3*time.Minute))
	if !errors.Is(err, ErrNoPendingTask) {
		t.Fatalf("expected ErrNoPendingTask while another task is running, got %v", err)
	}
}

func TestClaimNextPendingTaskReturnsErrNoPendingTask(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	_, err := db.ClaimNextPendingTask(context.Background(), time.Now().UTC())
	if !errors.Is(err, ErrNoPendingTask) {
		t.Fatalf("expected ErrNoPendingTask, got %v", err)
	}
}

func TestRecoverRunningTasksMarksRunningRowsFailed(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	startedAt := time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:    "task-running",
		Type:      task.TypeDeploy,
		Source:    task.SourceSystem,
		Status:    task.StatusRunning,
		CreatedAt: startedAt.Add(-1 * time.Minute),
		StartedAt: &startedAt,
	}); err != nil {
		t.Fatalf("create running task: %v", err)
	}

	recoveredAt := startedAt.Add(10 * time.Minute)
	affected, err := db.RecoverRunningTasks(ctx, recoveredAt)
	if err != nil {
		t.Fatalf("recover running tasks: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 recovered task, got %d", affected)
	}

	row := db.sql.QueryRowContext(ctx, `SELECT status, finished_at, error_summary FROM tasks WHERE task_id = 'task-running'`)
	var status string
	var finishedAt string
	var errorSummary string
	if err := row.Scan(&status, &finishedAt, &errorSummary); err != nil {
		t.Fatalf("scan recovered task: %v", err)
	}
	if status != string(task.StatusFailed) {
		t.Fatalf("expected failed status, got %q", status)
	}
	if finishedAt != recoveredAt.Format(time.RFC3339) {
		t.Fatalf("expected finished_at %q, got %q", recoveredAt.Format(time.RFC3339), finishedAt)
	}
	if errorSummary != "controller restarted during task execution" {
		t.Fatalf("unexpected error summary %q", errorSummary)
	}
}

func TestListTasksAppliesFiltersAndCursor(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha", "bravo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeDeploy, Source: task.SourceCLI, Status: task.StatusSucceeded, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-1: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-2", Type: task.TypeBackup, Source: task.SourceCLI, Status: task.StatusFailed, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 12, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-2: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-3", Type: task.TypeRestart, Source: task.SourceCLI, Status: task.StatusFailed, ServiceName: "bravo", CreatedAt: time.Date(2026, 4, 4, 12, 10, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-3: %v", err)
	}

	tasks, nextCursor, err := db.ListTasks(ctx, string(task.StatusFailed), "", "", "", 1)
	if err != nil {
		t.Fatalf("list failed tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != "task-3" {
		t.Fatalf("unexpected failed task page: %+v", tasks)
	}
	if nextCursor != "task-3" {
		t.Fatalf("expected next cursor task-3, got %q", nextCursor)
	}

	tasks, nextCursor, err = db.ListTasks(ctx, "", "alpha", "", nextCursor, 10)
	if err != nil {
		t.Fatalf("list alpha tasks after cursor: %v", err)
	}
	if len(tasks) != 2 || tasks[0].TaskID != "task-2" || tasks[1].TaskID != "task-1" {
		t.Fatalf("unexpected alpha task page: %+v", tasks)
	}
	if nextCursor != "" {
		t.Fatalf("expected empty next cursor, got %q", nextCursor)
	}
}

func TestCompleteTaskRefreshesServiceRuntimeStatus(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-deploy", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "demo", CreatedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create deploy task: %v", err)
	}
	finishedAt := time.Date(2026, 4, 4, 12, 5, 0, 0, time.UTC)
	if err := db.CompleteTask(ctx, "task-deploy", task.StatusSucceeded, finishedAt, ""); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	row := db.sql.QueryRowContext(ctx, `SELECT runtime_status, last_task_id, updated_at FROM services WHERE service_name = 'demo'`)
	var runtimeStatus string
	var lastTaskID string
	var updatedAt string
	if err := row.Scan(&runtimeStatus, &lastTaskID, &updatedAt); err != nil {
		t.Fatalf("scan service state: %v", err)
	}
	if runtimeStatus != "running" {
		t.Fatalf("expected running runtime status, got %q", runtimeStatus)
	}
	if lastTaskID != "task-deploy" {
		t.Fatalf("expected last task task-deploy, got %q", lastTaskID)
	}
	if updatedAt != finishedAt.Format(time.RFC3339) {
		t.Fatalf("expected updated_at %q, got %q", finishedAt.Format(time.RFC3339), updatedAt)
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}
