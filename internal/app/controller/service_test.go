package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func singleQueuedServiceActionTask(t *testing.T, response *controllerv1.RunServiceActionResponse) *controllerv1.TaskActionResponse {
	t.Helper()
	tasks := response.GetTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected exactly one queued service task, got %d", len(tasks))
	}
	return tasks[0]
}

func TestServiceQueryServiceListServices(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := syncDeclaredServicesForTests(context.Background(), db, "alpha", "bravo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceQueryServiceHandler(
		&serviceQueryServer{db: db},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceQueryServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
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

func TestServiceQueryServiceListServiceWorkspaces(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnodes:\n  - main\n",
		"draft/composia-meta.yaml": "name: draft\n",
		"scratch/README.md":        "hello\n",
	})

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceQueryServiceHandler(
		&serviceQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.ListServiceWorkspaces(ctx, connect.NewRequest(&controllerv1.ListServiceWorkspacesRequest{}))
	if err != nil {
		t.Fatalf("list service workspaces: %v", err)
	}
	if len(response.Msg.GetWorkspaces()) != 3 {
		t.Fatalf("expected 3 workspaces, got %d", len(response.Msg.GetWorkspaces()))
	}
	alpha := response.Msg.GetWorkspaces()[0]
	if alpha.GetFolder() != "alpha" || !alpha.GetHasMeta() || !alpha.GetIsDeclared() || alpha.GetServiceName() != "alpha" {
		t.Fatalf("unexpected alpha workspace: %+v", alpha)
	}
	draft := response.Msg.GetWorkspaces()[1]
	if draft.GetFolder() != "draft" || !draft.GetHasMeta() || draft.GetIsDeclared() || draft.GetRuntimeStatus() != "needs_validation" {
		t.Fatalf("unexpected draft workspace: %+v", draft)
	}
	scratch := response.Msg.GetWorkspaces()[2]
	if scratch.GetFolder() != "scratch" || scratch.GetHasMeta() || scratch.GetRuntimeStatus() != "uninitialized" {
		t.Fatalf("unexpected scratch workspace: %+v", scratch)
	}
}

func TestServiceQueryServiceGetServiceWorkspaceReturnsOneWorkspace(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnodes:\n  - main\n",
	})

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceQueryServiceHandler(
		&serviceQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.GetServiceWorkspace(ctx, connect.NewRequest(&controllerv1.GetServiceWorkspaceRequest{Folder: "alpha"}))
	if err != nil {
		t.Fatalf("get service workspace: %v", err)
	}
	if response.Msg.GetWorkspace().GetFolder() != "alpha" || response.Msg.GetWorkspace().GetServiceName() != "alpha" {
		t.Fatalf("unexpected service workspace: %+v", response.Msg.GetWorkspace())
	}
}

func TestServiceQueryServiceGetServiceReturnsMinimalSummary(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-alpha", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.CompleteTask(ctx, "task-alpha", task.StatusSucceeded, time.Date(2026, 4, 4, 10, 5, 0, 0, time.UTC), ""); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if err := db.UpdateServiceInstanceRuntimeStatus(ctx, "alpha", "main", store.ServiceRuntimeRunning, time.Date(2026, 4, 4, 10, 5, 30, 0, time.UTC)); err != nil {
		t.Fatalf("update service instance runtime status: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceQueryServiceHandler(
		&serviceQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.GetService(ctx, connect.NewRequest(&controllerv1.GetServiceRequest{ServiceName: "alpha"}))
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if response.Msg.GetName() != "alpha" || len(response.Msg.GetNodes()) != 1 || response.Msg.GetNodes()[0] != "main" {
		t.Fatalf("unexpected service response: %+v", response.Msg)
	}
	if len(response.Msg.GetInstances()) != 1 || response.Msg.GetInstances()[0].GetNodeId() != "main" {
		t.Fatalf("unexpected service instances: %+v", response.Msg.GetInstances())
	}
	if !response.Msg.GetEnabled() {
		t.Fatalf("expected enabled service")
	}
	if response.Msg.GetRuntimeStatus() != "running" {
		t.Fatalf("expected runtime status running, got %q", response.Msg.GetRuntimeStatus())
	}
}

func TestServiceQueryServiceGetServiceTasksReturnsFilteredTasks(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo"); err != nil {
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
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceQueryServiceHandler(
		&serviceQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.GetServiceTasks(ctx, connect.NewRequest(&controllerv1.GetServiceTasksRequest{ServiceName: "alpha", PageSize: 10}))
	if err != nil {
		t.Fatalf("get service tasks: %v", err)
	}
	if len(response.Msg.GetTasks()) != 1 || response.Msg.GetTasks()[0].GetTaskId() != "task-1" {
		t.Fatalf("unexpected service task list: %+v", response.Msg.GetTasks())
	}
}

func TestServiceQueryServiceGetServiceBackupsReturnsFilteredBackups(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo"); err != nil {
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
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceQueryServiceHandler(
		&serviceQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.GetServiceBackups(ctx, connect.NewRequest(&controllerv1.GetServiceBackupsRequest{ServiceName: "alpha", PageSize: 10}))
	if err != nil {
		t.Fatalf("get service backups: %v", err)
	}
	if len(response.Msg.GetBackups()) != 1 || response.Msg.GetBackups()[0].GetBackupId() != "backup-1" {
		t.Fatalf("unexpected service backup list: %+v", response.Msg.GetBackups())
	}
}

func TestServiceCommandServiceUpdateServiceTargetNodesRewritesMetaAndCommits(t *testing.T) {
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
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	commandServer := &serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}, {ID: "edge"}}}, availableNodeIDs: map[string]struct{}{"main": {}, "edge": {}}, repoMu: &sync.Mutex{}}
	queryServer := &serviceQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}, {ID: "edge"}}}, availableNodeIDs: map[string]struct{}{"main": {}, "edge": {}}, repoMu: &sync.Mutex{}}
	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		commandServer,
		connect.WithInterceptors(interceptor),
	)
	queryPath, queryHandler := controllerv1connect.NewServiceQueryServiceHandler(
		queryServer,
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle(queryPath, queryHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	queryClient := controllerv1connect.NewServiceQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.UpdateServiceTargetNodes(ctx, connect.NewRequest(&controllerv1.UpdateServiceTargetNodesRequest{ServiceName: "alpha", NodeIds: []string{"main", "edge"}, BaseRevision: mustCurrentRevision(t, repoDir)}))
	if err != nil {
		t.Fatalf("update service target nodes: %v", err)
	}
	if response.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id in response")
	}
	if response.Msg.GetSyncStatus() != store.RepoSyncStatusLocalOnly {
		t.Fatalf("expected local_only sync status, got %q", response.Msg.GetSyncStatus())
	}
	metaContent, err := os.ReadFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"))
	if err != nil {
		t.Fatalf("read rewritten meta: %v", err)
	}
	if string(metaContent) != "name: alpha\nnodes:\n  - main\n  - edge\n" {
		t.Fatalf("unexpected rewritten meta content: %q", string(metaContent))
	}
	serviceResp, err := queryClient.GetService(ctx, connect.NewRequest(&controllerv1.GetServiceRequest{ServiceName: "alpha"}))
	if err != nil {
		t.Fatalf("get service after target node update: %v", err)
	}
	if len(serviceResp.Msg.GetNodes()) != 2 || serviceResp.Msg.GetNodes()[0] != "main" || serviceResp.Msg.GetNodes()[1] != "edge" {
		t.Fatalf("unexpected service nodes after update: %+v", serviceResp.Msg.GetNodes())
	}
	clean, err := repo.IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("check clean worktree: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean worktree after target node update")
	}
	currentRevision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}
	if currentRevision != response.Msg.GetCommitId() {
		t.Fatalf("expected HEAD %q, got %q", response.Msg.GetCommitId(), currentRevision)
	}
}

func TestServiceCommandServiceUpdateServiceTargetNodesRejectsInvalidNode(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: &sync.Mutex{}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	_, err := client.UpdateServiceTargetNodes(ctx, connect.NewRequest(&controllerv1.UpdateServiceTargetNodesRequest{ServiceName: "alpha", NodeIds: []string{"missing"}, BaseRevision: mustCurrentRevision(t, repoDir)}))
	if err == nil {
		t.Fatalf("expected invalid node error")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

func TestServiceCommandServiceUpdateServiceTargetNodesRejectsActiveServiceTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-alpha", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", Status: task.StatusPending}); err != nil {
		t.Fatalf("create active task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}, {ID: "edge"}}}, availableNodeIDs: map[string]struct{}{"main": {}, "edge": {}}, repoMu: &sync.Mutex{}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	_, err := client.UpdateServiceTargetNodes(ctx, connect.NewRequest(&controllerv1.UpdateServiceTargetNodesRequest{ServiceName: "alpha", NodeIds: []string{"main", "edge"}, BaseRevision: mustCurrentRevision(t, repoDir)}))
	if err == nil {
		t.Fatalf("expected active task conflict")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

func TestServiceCommandServiceDeployCreatesPendingTask(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{
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

	client := controllerv1connect.NewServiceCommandServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
	)

	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}))
	if err != nil {
		t.Fatalf("deploy service: %v", err)
	}
	queuedTask := singleQueuedServiceActionTask(t, response.Msg)
	if queuedTask.GetTaskId() == "" {
		t.Fatalf("expected task ID in deploy response")
	}
	if queuedTask.GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING {
		t.Fatalf("expected pending deploy task, got %q", queuedTask.GetStatus())
	}
	if queuedTask.GetRepoRevision() == "" {
		t.Fatalf("expected repo revision in deploy response")
	}

	detail, err := db.GetTask(ctx, queuedTask.GetTaskId())
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

func TestServiceCommandServiceDeployIgnoresUnrelatedInvalidDraft(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "demo", "main")
	if err := os.MkdirAll(filepath.Join(repoDir, "draft"), 0o755); err != nil {
		t.Fatalf("create draft dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "draft", "composia-meta.yaml"), []byte("name: draft\nnodes:\n  - missing\n"), 0o644); err != nil {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{
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

	client := controllerv1connect.NewServiceCommandServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
	)

	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}))
	if err != nil {
		t.Fatalf("deploy service with unrelated invalid draft: %v", err)
	}
	if singleQueuedServiceActionTask(t, response.Msg).GetTaskId() == "" {
		t.Fatalf("expected task ID in deploy response")
	}
}

func TestServiceCommandServiceDeployReturnsAllQueuedTasks(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\n  - edge\n",
	})

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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"demo": {"main", "edge"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "edge"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	for _, nodeID := range []string{"main", "edge"} {
		if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: nodeID, HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
			t.Fatalf("record heartbeat for %s: %v", nodeID, err)
		}
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}, {ID: "edge"}}}, availableNodeIDs: map[string]struct{}{"main": {}, "edge": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}))
	if err != nil {
		t.Fatalf("deploy service: %v", err)
	}
	if len(response.Msg.GetTasks()) != 2 {
		t.Fatalf("expected 2 queued deploy tasks, got %d", len(response.Msg.GetTasks()))
	}
	seenNodes := make(map[string]struct{}, len(response.Msg.GetTasks()))
	for _, queuedTask := range response.Msg.GetTasks() {
		if queuedTask.GetTaskId() == "" || queuedTask.GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING || queuedTask.GetRepoRevision() == "" {
			t.Fatalf("unexpected queued task: %+v", queuedTask)
		}
		detail, err := db.GetTask(ctx, queuedTask.GetTaskId())
		if err != nil {
			t.Fatalf("get queued task %q: %v", queuedTask.GetTaskId(), err)
		}
		seenNodes[detail.Record.NodeID] = struct{}{}
	}
	if len(seenNodes) != 2 {
		t.Fatalf("expected queued tasks for both nodes, got %+v", seenNodes)
	}
}

func TestServiceCommandServiceDeployUsesWebSourceHeader(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{
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
	client := controllerv1connect.NewServiceCommandServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token"), requestInterceptor),
	)

	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}))
	if err != nil {
		t.Fatalf("deploy service: %v", err)
	}

	detail, err := db.GetTask(ctx, singleQueuedServiceActionTask(t, response.Msg).GetTaskId())
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if detail.Record.Source != task.SourceWeb {
		t.Fatalf("expected web task source, got %q", detail.Record.Source)
	}
}

func TestServiceCommandServiceDeployUsesOthersSourceHeader(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{
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
			req.Header().Set("X-Composia-Source", "others")
			return next(ctx, req)
		}
	})
	client := controllerv1connect.NewServiceCommandServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token"), requestInterceptor),
	)

	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}))
	if err != nil {
		t.Fatalf("deploy service: %v", err)
	}

	detail, err := db.GetTask(ctx, singleQueuedServiceActionTask(t, response.Msg).GetTaskId())
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if detail.Record.Source != task.SourceOthers {
		t.Fatalf("expected others task source, got %q", detail.Record.Source)
	}
}

func TestServiceCommandServiceCaddySyncCreatesPendingTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
		"demo/demo.caddy":         "demo.example.com { reverse_proxy 127.0.0.1:8080 }\n",
	})

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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()
	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_CADDY_SYNC}))
	if err != nil {
		t.Fatalf("caddy sync service: %v", err)
	}
	queuedTask := singleQueuedServiceActionTask(t, response.Msg)
	if queuedTask.GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING {
		t.Fatalf("expected pending caddy sync task, got %q", queuedTask.GetStatus())
	}
	detail, err := db.GetTask(ctx, queuedTask.GetTaskId())
	if err != nil {
		t.Fatalf("get caddy sync task: %v", err)
	}
	if detail.Record.Type != task.TypeCaddySync {
		t.Fatalf("expected caddy_sync task type, got %q", detail.Record.Type)
	}
	params := mustTaskParams(t, detail.Record.ParamsJSON)
	if params.ServiceDir != "demo" || len(params.ServiceDirs) != 0 || params.FullRebuild {
		t.Fatalf("unexpected caddy sync params: %+v", params)
	}
}

func TestServiceCommandServiceDeployRejectsOfflineOrDisabledNode(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	makeClient := func(nodes []config.NodeConfig) controllerv1connect.ServiceCommandServiceClient {
		path, handler := controllerv1connect.NewServiceCommandServiceHandler(
			&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: nodes}, availableNodeIDs: map[string]struct{}{"main": {}}},
			connect.WithInterceptors(interceptor),
		)
		mux := http.NewServeMux()
		mux.Handle(path, handler)
		httpServer := httptest.NewServer(mux)
		t.Cleanup(httpServer.Close)
		return controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	}

	offlineClient := makeClient([]config.NodeConfig{{ID: "main"}})
	_, err = offlineClient.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}))
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
	_, err = disabledClient.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}))
	if err == nil {
		t.Fatalf("expected disabled node deploy to fail")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition for disabled node, got %v", err)
	}
}

func TestServiceCommandServiceStopAndRestartCreatePendingTasks(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))

	stopResponse, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_STOP}))
	if err != nil {
		t.Fatalf("stop service: %v", err)
	}
	stopTask := singleQueuedServiceActionTask(t, stopResponse.Msg)
	if stopTask.GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING {
		t.Fatalf("expected pending stop task, got %q", stopTask.GetStatus())
	}
	if err := db.CompleteTask(ctx, stopTask.GetTaskId(), task.StatusSucceeded, time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC), ""); err != nil {
		t.Fatalf("complete stop task: %v", err)
	}

	restartResponse, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_RESTART}))
	if err != nil {
		t.Fatalf("restart service: %v", err)
	}
	if singleQueuedServiceActionTask(t, restartResponse.Msg).GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING {
		t.Fatalf("expected pending restart task, got %q", singleQueuedServiceActionTask(t, restartResponse.Msg).GetStatus())
	}
}

func TestServiceCommandServiceRejectsConfigInfraRestart(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"host-service/composia-meta.yaml": "name: host-service\nnodes:\n  - main\ninfra:\n  config: {}\nnetwork:\n  caddy:\n    enabled: true\n    source: ./host-service.caddy\n",
	})

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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "host-service"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	_, err = client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "host-service", Action: controllerv1.ServiceAction_SERVICE_ACTION_RESTART}))
	if err == nil {
		t.Fatalf("expected infra.config restart to fail")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

func TestServiceInstanceServiceRejectsConfigInfraRestart(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"host-service/composia-meta.yaml": "name: host-service\nnodes:\n  - main\ninfra:\n  config: {}\nnetwork:\n  caddy:\n    enabled: true\n    source: ./host-service.caddy\n",
	})

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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "host-service"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceInstanceServiceHandler(
		&serviceInstanceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceInstanceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	_, err = client.RunServiceInstanceAction(ctx, connect.NewRequest(&controllerv1.RunServiceInstanceActionRequest{ServiceName: "host-service", NodeId: "main", Action: controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_RESTART}))
	if err == nil {
		t.Fatalf("expected infra.config restart to fail")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

func TestServiceInstanceServiceInvalidActionTakesPrecedenceOverMissingService(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	stateDir := filepath.Join(rootDir, "state")
	logDir := filepath.Join(rootDir, "logs")
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
	defer func() { _ = db.Close() }()

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceInstanceServiceHandler(
		&serviceInstanceServer{db: db, cfg: &config.ControllerConfig{RepoDir: filepath.Join(rootDir, "repo"), LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceInstanceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	_, err = client.RunServiceInstanceAction(context.Background(), connect.NewRequest(&controllerv1.RunServiceInstanceActionRequest{ServiceName: "missing", NodeId: "main"}))
	if err == nil {
		t.Fatalf("expected invalid action to fail")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestServiceCommandServiceUpdateCreatesPendingTask(t *testing.T) {
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_UPDATE}))
	if err != nil {
		t.Fatalf("update service: %v", err)
	}
	if singleQueuedServiceActionTask(t, response.Msg).GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING {
		t.Fatalf("expected pending update task, got %q", singleQueuedServiceActionTask(t, response.Msg).GetStatus())
	}
}

func TestServiceCommandServiceUpdateWithImageSelectionsReturnsRepoWriteResult(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\nupdate:\n  images:\n    api:\n      image: ghcr.io/example/api\n      current:\n        env:\n          file: .env\n          key: API_VERSION\n      discovery:\n        sources:\n          - type: auto\n      filter:\n        type: semver\n",
		"demo/.env":               "API_VERSION=1.2.3\n",
	})

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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if err := db.UpsertServiceImageUpdateChecks(ctx, []store.ServiceImageUpdateCheck{{
		ServiceName:       "demo",
		NodeID:            "main",
		ImageName:         "api",
		ImageRef:          "ghcr.io/example/api",
		PolicyType:        "semver",
		CurrentValue:      "1.2.3@sha256:old",
		CurrentTag:        "1.2.3",
		CurrentDigest:     "sha256:old",
		CandidateTag:      "1.3.0",
		CandidateDigest:   "sha256:new",
		CandidateTagsJSON: `["1.3.0"]`,
		UpdateAvailable:   true,
		CheckStatus:       store.ImageCheckStatusOK,
		CheckedAt:         time.Date(2026, 4, 4, 11, 59, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatalf("upsert image update checks: %v", err)
	}
	baseRevision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("current repo revision: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{
		ServiceName:   "demo",
		Action:        controllerv1.ServiceAction_SERVICE_ACTION_UPDATE,
		ImageUpdates:  []*controllerv1.ImageUpdateSelection{{ImageName: "api", UseDetected: true}},
		BaseRevision:  baseRevision,
		CommitMessage: "update images for demo",
	}))
	if err != nil {
		t.Fatalf("update service with image selection: %v", err)
	}
	queuedTask := singleQueuedServiceActionTask(t, response.Msg)
	if queuedTask.GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING {
		t.Fatalf("expected pending update task, got %q", queuedTask.GetStatus())
	}
	if response.Msg.GetRepoWrite() == nil || response.Msg.GetRepoWrite().GetCommitId() == "" {
		t.Fatalf("expected repo write result, got %+v", response.Msg.GetRepoWrite())
	}
	updatedEnv, err := os.ReadFile(filepath.Join(repoDir, "demo", ".env"))
	if err != nil {
		t.Fatalf("read updated env file: %v", err)
	}
	if string(updatedEnv) != "API_VERSION=1.3.0@sha256:new\n" {
		t.Fatalf("unexpected updated env file:\n%s", string(updatedEnv))
	}
}

func TestServiceCommandServiceUpdateDNSCreatesPendingTaskWithoutOnlineNode(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\nnetwork:\n  dns:\n    provider: cloudflare\n    hostname: demo.example.com\n",
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{
			db: db,
			cfg: &config.ControllerConfig{
				RepoDir: repoDir,
				LogDir:  logDir,
				Nodes:   []config.NodeConfig{{ID: "main"}},
				DNS:     &config.ControllerDNSConfig{Cloudflare: &config.CloudflareDNSConfig{APIToken: "dns-token"}},
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
	client := controllerv1connect.NewServiceCommandServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token"), requestInterceptor),
	)

	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "demo", Action: controllerv1.ServiceAction_SERVICE_ACTION_DNS_UPDATE}))
	if err != nil {
		t.Fatalf("update service dns: %v", err)
	}
	queuedTask := singleQueuedServiceActionTask(t, response.Msg)
	if queuedTask.GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING {
		t.Fatalf("expected pending dns update task, got %q", queuedTask.GetStatus())
	}
	detail, err := db.GetTask(ctx, queuedTask.GetTaskId())
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

func TestReplaceEnvFileValueUpdatesExistingKey(t *testing.T) {
	updated, err := replaceEnvFileValue("API_VERSION=1.2.3\nOTHER=value\n", "API_VERSION", "1.3.0@sha256:new")
	if err != nil {
		t.Fatalf("replace env file value: %v", err)
	}
	if updated != "API_VERSION=1.3.0@sha256:new\nOTHER=value\n" {
		t.Fatalf("unexpected updated env:\n%s", updated)
	}
}

func TestReplaceYAMLPathImageValueUpdatesImage(t *testing.T) {
	updated, err := replaceYAMLPathImageValue("services:\n  api:\n    image: ghcr.io/example/api:1.2.3\n", "services.api.image", "ghcr.io/example/api", "1.3.0@sha256:new")
	if err != nil {
		t.Fatalf("replace yaml path image value: %v", err)
	}
	if !strings.Contains(updated, "image: ghcr.io/example/api:1.3.0@sha256:new") {
		t.Fatalf("unexpected updated yaml:\n%s", updated)
	}
}

func TestServiceCommandServiceBackupCreatesPendingTaskWithDefaultDataNames(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config\n    - name: db\n      backup:\n        strategy: files.copy\n        include:\n          - ./db\nbackup:\n  data:\n    - name: config\n    - name: db\n      enabled: false\n",
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "alpha", Action: controllerv1.ServiceAction_SERVICE_ACTION_BACKUP}))
	if err != nil {
		t.Fatalf("backup service: %v", err)
	}
	queuedTask := singleQueuedServiceActionTask(t, response.Msg)
	if queuedTask.GetStatus() != controllerv1.TaskStatus_TASK_STATUS_PENDING {
		t.Fatalf("expected pending backup task, got %q", queuedTask.GetStatus())
	}
	detail, err := db.GetTask(ctx, queuedTask.GetTaskId())
	if err != nil {
		t.Fatalf("get backup task: %v", err)
	}
	params := mustTaskParams(t, detail.Record.ParamsJSON)
	if len(params.DataNames) != 1 || params.DataNames[0] != "config" {
		t.Fatalf("unexpected backup task params: %+v", params)
	}
}

func TestServiceCommandServiceMigrateCreatesPendingControllerTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config\n      restore:\n        strategy: files.copy\n        include:\n          - ./config\nbackup:\n  data:\n    - name: config\nmigrate:\n  data:\n    - name: config\n",
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"alpha": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "edge"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	for _, nodeID := range []string{"main", "edge"} {
		if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: nodeID, HeartbeatAt: time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)}); err != nil {
			t.Fatalf("record heartbeat for %s: %v", nodeID, err)
		}
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}, {ID: "edge"}}}, availableNodeIDs: map[string]struct{}{"main": {}, "edge": {}}},
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
	client := controllerv1connect.NewServiceCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token"), requestInterceptor))
	response, err := client.MigrateService(ctx, connect.NewRequest(&controllerv1.MigrateServiceRequest{ServiceName: "alpha", SourceNodeId: "main", TargetNodeId: "edge"}))
	if err != nil {
		t.Fatalf("migrate service: %v", err)
	}
	if response.Msg.GetTaskId() == "" {
		t.Fatalf("expected migrate task ID")
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get migrate task: %v", err)
	}
	if detail.Record.Type != task.TypeMigrate {
		t.Fatalf("expected migrate task type, got %q", detail.Record.Type)
	}
	if detail.Record.Source != task.SourceWeb {
		t.Fatalf("expected migrate task source web, got %q", detail.Record.Source)
	}
	params := mustTaskParams(t, detail.Record.ParamsJSON)
	if params.SourceNodeID != "main" || params.TargetNodeID != "edge" {
		t.Fatalf("unexpected migrate params: %+v", params)
	}
	if len(params.DataNames) != 1 || params.DataNames[0] != "config" {
		t.Fatalf("unexpected migrate data names: %+v", params.DataNames)
	}
}

func TestPlanRequestedServiceImageUpdatesIncludesMutableAllDetected(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.UpsertServiceImageUpdateChecks(ctx, []store.ServiceImageUpdateCheck{{
		ServiceName:     "app",
		NodeID:          "main",
		ImageName:       "web",
		ImageRef:        "nginx",
		PolicyType:      "digest",
		CurrentTag:      "latest",
		CurrentDigest:   "sha256:old",
		CandidateTag:    "latest",
		CandidateDigest: "sha256:new",
		UpdateAvailable: true,
		CheckStatus:     store.ImageCheckStatusOK,
		CheckedAt:       time.Date(2026, 5, 8, 4, 0, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatalf("upsert image update checks: %v", err)
	}

	backupBeforeUpdate := true
	service := repo.Service{
		Name:        "app",
		TargetNodes: []string{"main"},
		Meta: repo.ServiceMeta{Update: &repo.UpdateConfig{Images: map[string]repo.ImageUpdateConfig{
			"web": {
				Image:              "nginx",
				BackupBeforeUpdate: &backupBeforeUpdate,
				Current:            repo.ImageUpdateCurrent{Tag: "latest"},
				Discovery:          repo.ImageUpdateDiscovery{Sources: []repo.ImageUpdateDiscoverySource{{Type: "digest"}}},
			},
		}}},
	}
	server := &serviceCommandServer{db: db, cfg: &config.ControllerConfig{}}
	planned, err := server.planRequestedServiceImageUpdates(ctx, service, []string{"main"}, nil, true)
	if err != nil {
		t.Fatalf("plan image updates: %v", err)
	}
	if len(planned) != 1 || planned[0].ImageName != "web" || planned[0].RepoBacked {
		t.Fatalf("unexpected planned mutable update: %+v", planned)
	}
	if !serviceImageUpdatesNeedBackup(server.cfg, service.Meta.Update, service.Meta.Update.Images, planned, nil, nil) {
		t.Fatalf("expected mutable all-detected update to require backup")
	}
}

func TestPlanRequestedServiceImageUpdatesRejectsEmptyAllDetected(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	service := repo.Service{
		Name:        "app",
		TargetNodes: []string{"main"},
		Meta: repo.ServiceMeta{Update: &repo.UpdateConfig{Images: map[string]repo.ImageUpdateConfig{
			"api": {
				Image:     "ghcr.io/example/api",
				Current:   repo.ImageUpdateCurrent{Env: &repo.ImageUpdateCurrentEnv{File: ".env", Key: "API_TAG"}},
				Discovery: repo.ImageUpdateDiscovery{Sources: []repo.ImageUpdateDiscoverySource{{Type: "auto"}}},
				Filter:    &repo.ImageUpdateFilter{Type: "semver"},
			},
		}}},
	}
	server := &serviceCommandServer{db: db, cfg: &config.ControllerConfig{}}
	_, err := server.planRequestedServiceImageUpdates(ctx, service, []string{"main"}, nil, true)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

func createGitRepoWithService(t *testing.T, repoDir, serviceName, nodeID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repoDir, serviceName), 0o755); err != nil {
		t.Fatalf("create repo directory: %v", err)
	}
	metaPath := filepath.Join(repoDir, serviceName, "composia-meta.yaml")
	content := "name: " + serviceName + "\nnodes:\n  - " + nodeID + "\n"
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
