package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestBackupRecordServiceListAndGetBackup(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-1: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-2", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "bravo", CreatedAt: time.Date(2026, 4, 4, 14, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-2: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: "backup-1", TaskID: "task-1", ServiceName: "alpha", DataName: "config", Status: "succeeded", StartedAt: "2026-04-04T14:00:00Z", FinishedAt: "2026-04-04T14:01:00Z", ArtifactRef: "snapshot-1"}); err != nil {
		t.Fatalf("insert backup-1: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: "backup-2", TaskID: "task-2", ServiceName: "bravo", DataName: "db", Status: "failed", StartedAt: "2026-04-04T14:05:00Z", FinishedAt: "2026-04-04T14:06:00Z"}); err != nil {
		t.Fatalf("insert backup-2: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewBackupRecordServiceHandler(&backupRecordServer{db: db}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewBackupRecordServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	listResponse, err := client.ListBackups(ctx, connect.NewRequest(&controllerv1.ListBackupsRequest{ServiceName: "alpha"}))
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(listResponse.Msg.GetBackups()) != 1 || listResponse.Msg.GetBackups()[0].GetBackupId() != "backup-1" {
		t.Fatalf("unexpected backup list response: %+v", listResponse.Msg.GetBackups())
	}

	getResponse, err := client.GetBackup(ctx, connect.NewRequest(&controllerv1.GetBackupRequest{BackupId: "backup-1"}))
	if err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if getResponse.Msg.GetArtifactRef() != "snapshot-1" || getResponse.Msg.GetServiceName() != "alpha" {
		t.Fatalf("unexpected backup detail response: %+v", getResponse.Msg)
	}
}

func TestBackupRecordServiceRestoreBackupCreatesPendingRestoreTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml":  "name: alpha\nnode: main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config\n      restore:\n        strategy: files.copy\n        include:\n          - ./config\nbackup:\n  data:\n    - name: config\n",
		"backup/composia-meta.yaml": "name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    data_protect_dir: /data-protect\n",
	})
	logDir := filepath.Join(rootDir, "logs")
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"alpha": {"main"}, "backup": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-backup", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", Status: task.StatusSucceeded, CreatedAt: time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create backup task: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: "backup-1", TaskID: "task-backup", ServiceName: "alpha", DataName: "config", Status: string(task.StatusSucceeded), StartedAt: "2026-04-04T14:00:00Z", FinishedAt: "2026-04-04T14:01:00Z", ArtifactRef: "snapshot-1"}); err != nil {
		t.Fatalf("insert backup record: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewBackupRecordServiceHandler(
		&backupRecordServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, taskQueue: newTaskQueueNotifier()},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	requestInterceptor := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("X-Composia-Source", "web")
			return next(ctx, req)
		}
	})
	client := controllerv1connect.NewBackupRecordServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token"), requestInterceptor))

	response, err := client.RestoreBackup(ctx, connect.NewRequest(&controllerv1.RestoreBackupRequest{BackupId: "backup-1", NodeId: "main"}))
	if err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	if response.Msg.GetTaskId() == "" {
		t.Fatalf("expected restore task ID")
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get restore task: %v", err)
	}
	if detail.Record.Type != task.TypeRestore {
		t.Fatalf("expected restore task type, got %q", detail.Record.Type)
	}
	if detail.Record.Source != task.SourceWeb {
		t.Fatalf("expected restore task source web, got %q", detail.Record.Source)
	}
	params := taskParams(detail.Record.ParamsJSON)
	if params.ServiceDir != "alpha" {
		t.Fatalf("unexpected restore service_dir: %+v", params)
	}
	if len(params.RestoreItems) != 1 {
		t.Fatalf("expected one restore item, got %+v", params.RestoreItems)
	}
	if params.RestoreItems[0].DataName != "config" || params.RestoreItems[0].ArtifactRef != "snapshot-1" || params.RestoreItems[0].SourceTaskID != "task-backup" {
		t.Fatalf("unexpected restore item: %+v", params.RestoreItems[0])
	}
	if detail.Record.NodeID != "main" {
		t.Fatalf("unexpected restore node_id: %q", detail.Record.NodeID)
	}
}
