package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestRunScheduledTasksPassCreatesBackupTasksFromDefaultOverrideAndNone(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"app/composia-meta.yaml": "name: app\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: inherit\n      backup:\n        strategy: files.copy\n        include:\n          - ./inherit\n    - name: override\n      backup:\n        strategy: files.copy\n        include:\n          - ./override\n    - name: disabled\n      backup:\n        strategy: files.copy\n        include:\n          - ./disabled\nbackup:\n  data:\n    - name: inherit\n    - name: override\n      schedule: \"5 2 * * *\"\n    - name: disabled\n      schedule: none\n",
	})
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "app"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 9, 2, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	cfg := &config.ControllerConfig{
		RepoDir: repoDir,
		LogDir:  logDir,
		Nodes:   []config.NodeConfig{{ID: "main"}},
		Backup:  &config.ControllerBackupConfig{DefaultSchedule: "5 2 * * *"},
	}
	if err := runScheduledTasksPass(ctx, db, cfg, configuredNodeIDs(cfg), newTaskQueueNotifier(), time.Date(2026, 4, 9, 2, 5, 20, 0, time.UTC)); err != nil {
		t.Fatalf("run scheduled pass: %v", err)
	}
	if err := runScheduledTasksPass(ctx, db, cfg, configuredNodeIDs(cfg), newTaskQueueNotifier(), time.Date(2026, 4, 9, 2, 5, 40, 0, time.UTC)); err != nil {
		t.Fatalf("rerun scheduled pass: %v", err)
	}

	tasks, totalCount, err := db.ListTasks(ctx, nil, []string{"app"}, nil, []string{string(task.TypeBackup)}, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("list backup tasks: %v", err)
	}
	if totalCount != 1 || len(tasks) != 1 {
		t.Fatalf("expected 1 scheduled backup task, got total=%d len=%d", totalCount, len(tasks))
	}
	detail, err := db.GetTask(ctx, tasks[0].TaskID)
	if err != nil {
		t.Fatalf("get task %s: %v", tasks[0].TaskID, err)
	}
	if detail.Record.Source != task.SourceSchedule {
		t.Fatalf("expected schedule source, got %+v", detail.Record)
	}
	params := taskParams(detail.Record.ParamsJSON)
	if len(params.DataNames) != 2 {
		t.Fatalf("expected two scheduled data names, got %+v", params)
	}
	if params.DataNames[0] != "inherit" || params.DataNames[1] != "override" {
		t.Fatalf("unexpected scheduled data names: %+v", params.DataNames)
	}
}

func TestRunScheduledTasksPassCreatesRepoWideRusticMaintenanceTasks(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"rustic/composia-meta.yaml": "name: rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n",
	})
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "rustic"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 9, 3, 15, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	cfg := &config.ControllerConfig{
		RepoDir: repoDir,
		LogDir:  logDir,
		Nodes:   []config.NodeConfig{{ID: "main"}},
		Rustic: &config.ControllerRusticConfig{
			MainNodes: []string{"main"},
			Maintenance: &config.ControllerRusticMaintenanceConfig{
				ForgetSchedule: "15 3 * * *",
				PruneSchedule:  "15 3 * * *",
			},
		},
	}
	if err := runScheduledTasksPass(ctx, db, cfg, configuredNodeIDs(cfg), newTaskQueueNotifier(), time.Date(2026, 4, 9, 3, 15, 30, 0, time.UTC)); err != nil {
		t.Fatalf("run scheduled pass: %v", err)
	}
	if err := runScheduledTasksPass(ctx, db, cfg, configuredNodeIDs(cfg), newTaskQueueNotifier(), time.Date(2026, 4, 9, 3, 15, 50, 0, time.UTC)); err != nil {
		t.Fatalf("rerun scheduled pass: %v", err)
	}

	forgetTasks, forgetCount, err := db.ListTasks(ctx, nil, []string{"rustic"}, nil, []string{string(task.TypeRusticForget)}, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("list forget tasks: %v", err)
	}
	if forgetCount != 1 || len(forgetTasks) != 1 {
		t.Fatalf("expected 1 scheduled forget task, got total=%d len=%d", forgetCount, len(forgetTasks))
	}
	pruneTasks, pruneCount, err := db.ListTasks(ctx, nil, []string{"rustic"}, nil, []string{string(task.TypeRusticPrune)}, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("list prune tasks: %v", err)
	}
	if pruneCount != 1 || len(pruneTasks) != 1 {
		t.Fatalf("expected 1 scheduled prune task, got total=%d len=%d", pruneCount, len(pruneTasks))
	}
	for _, taskID := range []string{forgetTasks[0].TaskID, pruneTasks[0].TaskID} {
		detail, err := db.GetTask(ctx, taskID)
		if err != nil {
			t.Fatalf("get maintenance task %s: %v", taskID, err)
		}
		if detail.Record.Source != task.SourceSchedule {
			t.Fatalf("expected schedule source, got %+v", detail.Record)
		}
		if detail.Record.NodeID != "main" {
			t.Fatalf("expected maintenance on main, got %+v", detail.Record)
		}
		if detail.Record.ParamsJSON != `{"service_dir":"rustic","repo_wide":true}` {
			t.Fatalf("expected repo-wide params, got %q", detail.Record.ParamsJSON)
		}
	}
}
