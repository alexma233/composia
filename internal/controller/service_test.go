package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func TestServiceServiceListServices(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	if err := db.SyncDeclaredServices(context.Background(), []string{"alpha", "bravo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")),
	)

	response, err := client.ListServices(context.Background(), connect.NewRequest(&controllerv1.ListServicesRequest{PageSize: 1}))
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(response.Msg.GetServices()) != 1 {
		t.Fatalf("expected 1 service in paged response, got %d", len(response.Msg.GetServices()))
	}
	if response.Msg.GetServices()[0].GetName() != "alpha" {
		t.Fatalf("expected first service alpha, got %q", response.Msg.GetServices()[0].GetName())
	}
	if response.Msg.GetServices()[0].GetRuntimeStatus() != "unknown" {
		t.Fatalf("expected runtime_status unknown, got %q", response.Msg.GetServices()[0].GetRuntimeStatus())
	}
	if response.Msg.GetTotalCount() != 2 {
		t.Fatalf("expected total count 2, got %d", response.Msg.GetTotalCount())
	}
}

func TestServiceServiceGetServiceReturnsMinimalSummary(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-alpha", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.CompleteTask(ctx, "task-alpha", task.StatusSucceeded, time.Date(2026, 4, 4, 10, 5, 0, 0, time.UTC), ""); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if err := db.UpdateServiceRuntimeStatus(ctx, "alpha", store.ServiceRuntimeRunning, time.Date(2026, 4, 4, 10, 5, 30, 0, time.UTC)); err != nil {
		t.Fatalf("update service runtime status: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.GetService(ctx, connect.NewRequest(&controllerv1.GetServiceRequest{ServiceName: "alpha"}))
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if response.Msg.GetName() != "alpha" || response.Msg.GetNode() != "main" {
		t.Fatalf("unexpected service response: %+v", response.Msg)
	}
	if !response.Msg.GetEnabled() {
		t.Fatalf("expected enabled service")
	}
	if response.Msg.GetRuntimeStatus() != "running" {
		t.Fatalf("expected runtime status running, got %q", response.Msg.GetRuntimeStatus())
	}
}

func TestServiceServiceGetServiceTasksReturnsFilteredTasks(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha", "bravo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", Status: task.StatusSucceeded, CreatedAt: time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create alpha task: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-2", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "bravo", NodeID: "main", Status: task.StatusSucceeded, CreatedAt: time.Date(2026, 4, 4, 11, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create bravo task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.GetServiceTasks(ctx, connect.NewRequest(&controllerv1.GetServiceTasksRequest{ServiceName: "alpha", PageSize: 10}))
	if err != nil {
		t.Fatalf("get service tasks: %v", err)
	}
	if len(response.Msg.GetTasks()) != 1 || response.Msg.GetTasks()[0].GetTaskId() != "task-1" {
		t.Fatalf("unexpected service task list: %+v", response.Msg.GetTasks())
	}
}

func TestServiceServiceGetServiceBackupsReturnsFilteredBackups(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha", "bravo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create alpha backup task: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-2", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "bravo", CreatedAt: time.Date(2026, 4, 4, 11, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create bravo backup task: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: "backup-1", TaskID: "task-1", ServiceName: "alpha", DataName: "config", Status: "succeeded", StartedAt: "2026-04-04T11:00:00Z", FinishedAt: "2026-04-04T11:01:00Z"}); err != nil {
		t.Fatalf("insert alpha backup: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: "backup-2", TaskID: "task-2", ServiceName: "bravo", DataName: "db", Status: "failed", StartedAt: "2026-04-04T11:05:00Z", FinishedAt: "2026-04-04T11:06:00Z"}); err != nil {
		t.Fatalf("insert bravo backup: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.GetServiceBackups(ctx, connect.NewRequest(&controllerv1.GetServiceBackupsRequest{ServiceName: "alpha", PageSize: 10}))
	if err != nil {
		t.Fatalf("get service backups: %v", err)
	}
	if len(response.Msg.GetBackups()) != 1 || response.Msg.GetBackups()[0].GetBackupId() != "backup-1" {
		t.Fatalf("unexpected service backup list: %+v", response.Msg.GetBackups())
	}
}

func TestServiceServiceDeployServiceCreatesPendingTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "demo", "main")

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
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{
			db:  db,
			cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}},
			availableNodeIDs: map[string]struct{}{
				"main": {},
			},
		},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")),
	)

	response, err := client.DeployService(ctx, connect.NewRequest(&controllerv1.DeployServiceRequest{ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("deploy service: %v", err)
	}
	if response.Msg.GetTaskId() == "" {
		t.Fatalf("expected task ID in deploy response")
	}
	if response.Msg.GetStatus() != "pending" {
		t.Fatalf("expected pending deploy task, got %q", response.Msg.GetStatus())
	}
	if response.Msg.GetRepoRevision() == "" {
		t.Fatalf("expected repo revision in deploy response")
	}

	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if detail.Record.ServiceName != "demo" || detail.Record.NodeID != "main" {
		t.Fatalf("unexpected created task record: %+v", detail.Record)
	}
	if detail.Record.Source != task.SourceCLI {
		t.Fatalf("expected default CLI task source, got %q", detail.Record.Source)
	}
	if detail.Record.LogPath == "" {
		t.Fatalf("expected task log path to be set")
	}
	if _, err := os.Stat(detail.Record.LogPath); err != nil {
		t.Fatalf("expected task log file to exist: %v", err)
	}
}

func TestServiceServiceDeployServiceIgnoresUnrelatedInvalidDraft(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "demo", "main")
	if err := os.MkdirAll(filepath.Join(repoDir, "draft"), 0o755); err != nil {
		t.Fatalf("create draft dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "draft", "composia-meta.yaml"), []byte("name: draft\nnode: missing\n"), 0o644); err != nil {
		t.Fatalf("write invalid draft meta: %v", err)
	}

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
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{
			db:  db,
			cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}},
			availableNodeIDs: map[string]struct{}{
				"main": {},
			},
		},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")),
	)

	response, err := client.DeployService(ctx, connect.NewRequest(&controllerv1.DeployServiceRequest{ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("deploy service with unrelated invalid draft: %v", err)
	}
	if response.Msg.GetTaskId() == "" {
		t.Fatalf("expected task ID in deploy response")
	}
}

func TestServiceServiceDeployServiceUsesWebSourceHeader(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "demo", "main")

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
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{
			db:  db,
			cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}},
			availableNodeIDs: map[string]struct{}{
				"main": {},
			},
		},
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
	client := controllerv1connect.NewServiceServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token"), requestInterceptor),
	)

	response, err := client.DeployService(ctx, connect.NewRequest(&controllerv1.DeployServiceRequest{ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("deploy service: %v", err)
	}

	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if detail.Record.Source != task.SourceWeb {
		t.Fatalf("expected web task source, got %q", detail.Record.Source)
	}
}

func TestServiceServiceDeployServiceRejectsOfflineOrDisabledNode(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "demo", "main")

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
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	makeClient := func(nodes []config.NodeConfig) controllerv1connect.ServiceServiceClient {
		path, handler := controllerv1connect.NewServiceServiceHandler(
			&serviceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: nodes}, availableNodeIDs: map[string]struct{}{"main": {}}},
			connect.WithInterceptors(interceptor),
		)
		mux := http.NewServeMux()
		mux.Handle(path, handler)
		httpServer := httptest.NewServer(mux)
		t.Cleanup(httpServer.Close)
		return controllerv1connect.NewServiceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	}

	offlineClient := makeClient([]config.NodeConfig{{ID: "main"}})
	_, err = offlineClient.DeployService(ctx, connect.NewRequest(&controllerv1.DeployServiceRequest{ServiceName: "demo"}))
	if err == nil {
		t.Fatalf("expected offline node deploy to fail")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition for offline node, got %v", err)
	}

	trueValue := true
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	disabledClient := makeClient([]config.NodeConfig{{ID: "main", Enabled: boolPtr(false), Token: "main-token"}, {ID: "other", Enabled: &trueValue}})
	_, err = disabledClient.DeployService(ctx, connect.NewRequest(&controllerv1.DeployServiceRequest{ServiceName: "demo"}))
	if err == nil {
		t.Fatalf("expected disabled node deploy to fail")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition for disabled node, got %v", err)
	}
}

func TestServiceServiceStopAndRestartCreatePendingTasks(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "demo", "main")

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
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))

	stopResponse, err := client.StopService(ctx, connect.NewRequest(&controllerv1.StopServiceRequest{ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("stop service: %v", err)
	}
	if stopResponse.Msg.GetStatus() != "pending" {
		t.Fatalf("expected pending stop task, got %q", stopResponse.Msg.GetStatus())
	}
	if err := db.CompleteTask(ctx, stopResponse.Msg.GetTaskId(), task.StatusSucceeded, time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC), ""); err != nil {
		t.Fatalf("complete stop task: %v", err)
	}

	restartResponse, err := client.RestartService(ctx, connect.NewRequest(&controllerv1.RestartServiceRequest{ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("restart service: %v", err)
	}
	if restartResponse.Msg.GetStatus() != "pending" {
		t.Fatalf("expected pending restart task, got %q", restartResponse.Msg.GetStatus())
	}
}

func TestServiceServiceUpdateCreatesPendingTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "demo", "main")

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
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.UpdateService(ctx, connect.NewRequest(&controllerv1.UpdateServiceRequest{ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("update service: %v", err)
	}
	if response.Msg.GetStatus() != "pending" {
		t.Fatalf("expected pending update task, got %q", response.Msg.GetStatus())
	}
}

func TestServiceServiceUpdateServiceDNSCreatesPendingTaskWithoutOnlineNode(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnode: main\nnetwork:\n  dns:\n    provider: cloudflare\n    hostname: demo.example.com\n",
	})
	logDir := filepath.Join(rootDir, "logs")
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	dnsTokenPath := filepath.Join(rootDir, "cloudflare.token")
	if err := os.WriteFile(dnsTokenPath, []byte("test-token\n"), 0o644); err != nil {
		t.Fatalf("write cloudflare token: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{
			db: db,
			cfg: &config.ControllerConfig{
				RepoDir: repoDir,
				LogDir:  logDir,
				Nodes:   []config.NodeConfig{{ID: "main"}},
				DNS:     &config.ControllerDNSConfig{Cloudflare: &config.CloudflareDNSConfig{APITokenFile: dnsTokenPath}},
			},
			availableNodeIDs: map[string]struct{}{"main": {}},
		},
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
	client := controllerv1connect.NewServiceServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token"), requestInterceptor),
	)

	response, err := client.UpdateServiceDNS(ctx, connect.NewRequest(&controllerv1.UpdateServiceDNSRequest{ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("update service dns: %v", err)
	}
	if response.Msg.GetStatus() != "pending" {
		t.Fatalf("expected pending dns update task, got %q", response.Msg.GetStatus())
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get dns update task: %v", err)
	}
	if detail.Record.Type != task.TypeDNSUpdate {
		t.Fatalf("expected dns_update task type, got %q", detail.Record.Type)
	}
	if detail.Record.Source != task.SourceWeb {
		t.Fatalf("expected web task source, got %q", detail.Record.Source)
	}
}

func TestServiceServiceBackupCreatesPendingTaskWithDefaultDataNames(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnode: main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config\n    - name: db\n      backup:\n        strategy: files.copy\n        include:\n          - ./db\nbackup:\n  data:\n    - name: config\n    - name: db\n      enabled: false\n",
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
	if err := db.SyncDeclaredServices(ctx, []string{"alpha"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.BackupService(ctx, connect.NewRequest(&controllerv1.BackupServiceRequest{ServiceName: "alpha"}))
	if err != nil {
		t.Fatalf("backup service: %v", err)
	}
	if response.Msg.GetStatus() != "pending" {
		t.Fatalf("expected pending backup task, got %q", response.Msg.GetStatus())
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get backup task: %v", err)
	}
	params := taskParams(detail.Record.ParamsJSON)
	if len(params.DataNames) != 1 || params.DataNames[0] != "config" {
		t.Fatalf("unexpected backup task params: %+v", params)
	}
}

func createGitRepoWithService(t *testing.T, repoDir, serviceName, nodeID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repoDir, serviceName), 0o755); err != nil {
		t.Fatalf("create repo directory: %v", err)
	}
	metaPath := filepath.Join(repoDir, serviceName, "composia-meta.yaml")
	content := "name: " + serviceName + "\nnode: " + nodeID + "\n"
	if err := os.WriteFile(metaPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write service meta: %v", err)
	}
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
}

func createGitRepoWithContent(t *testing.T, repoDir string, files map[string]string) {
	t.Helper()
	for relativePath, content := range files {
		absolutePath := filepath.Join(repoDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
			t.Fatalf("create directory for %s: %v", relativePath, err)
		}
		if err := os.WriteFile(absolutePath, []byte(content), 0o644); err != nil {
			t.Fatalf("write file %s: %v", relativePath, err)
		}
	}
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
}

func runGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", repoDir, "-c", "commit.gpgsign=false"}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

type assertError string

func (value assertError) Error() string {
	return string(value)
}
