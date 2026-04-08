package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestNodeQueryServiceListNodes(t *testing.T) {
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

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "node-2"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{
		NodeID:      "main",
		HeartbeatAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewNodeQueryServiceHandler(
		&nodeQueryServer{
			db: db,
			cfg: &config.ControllerConfig{
				Nodes: []config.NodeConfig{
					{ID: "main", DisplayName: "Main", Enabled: boolPtr(true)},
					{ID: "node-2"},
				},
			},
		},
		connect.WithInterceptors(interceptor),
	)

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeQueryServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")),
	)

	response, err := client.ListNodes(context.Background(), connect.NewRequest(&controllerv1.ListNodesRequest{}))
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(response.Msg.GetNodes()) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(response.Msg.GetNodes()))
	}
	if response.Msg.GetNodes()[0].GetNodeId() != "main" || !response.Msg.GetNodes()[0].GetIsOnline() {
		t.Fatalf("unexpected first node: %+v", response.Msg.GetNodes()[0])
	}
	if response.Msg.GetNodes()[0].GetDisplayName() != "Main" {
		t.Fatalf("expected explicit display name Main, got %q", response.Msg.GetNodes()[0].GetDisplayName())
	}
	if response.Msg.GetNodes()[1].GetDisplayName() != "node-2" {
		t.Fatalf("expected fallback display name node-2, got %q", response.Msg.GetNodes()[1].GetDisplayName())
	}
	if response.Msg.GetNodes()[1].GetEnabled() != true {
		t.Fatalf("expected nil enabled to default true")
	}
	if response.Msg.GetNodes()[1].GetIsOnline() {
		t.Fatalf("expected node-2 to be offline")
	}
}

func TestNodeQueryServiceGetNodeReturnsMinimalSummary(t *testing.T) {
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

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "caddy"); err != nil {
		t.Fatalf("sync declared services: %v", err)
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

	path, handler := controllerv1connect.NewNodeQueryServiceHandler(
		&nodeQueryServer{db: db, cfg: &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main", DisplayName: "Main"}}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeQueryServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")),
	)

	response, err := client.GetNode(ctx, connect.NewRequest(&controllerv1.GetNodeRequest{NodeId: "main"}))
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if response.Msg.GetNode().GetNodeId() != "main" || response.Msg.GetNode().GetDisplayName() != "Main" {
		t.Fatalf("unexpected node response: %+v", response.Msg.GetNode())
	}
	if !response.Msg.GetNode().GetIsOnline() {
		t.Fatalf("expected node to be online")
	}
}

func TestNodeQueryServiceGetNodeTasksReturnsFilteredTasks(t *testing.T) {
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

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "node-2"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", Status: task.StatusSucceeded, CreatedAt: time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create main task: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-2", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "node-2", Status: task.StatusSucceeded, CreatedAt: time.Date(2026, 4, 4, 13, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create node-2 task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewNodeQueryServiceHandler(
		&nodeQueryServer{db: db, cfg: &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main"}, {ID: "node-2"}}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.GetNodeTasks(ctx, connect.NewRequest(&controllerv1.GetNodeTasksRequest{NodeId: "main", PageSize: 10}))
	if err != nil {
		t.Fatalf("get node tasks: %v", err)
	}
	if len(response.Msg.GetTasks()) != 1 || response.Msg.GetTasks()[0].GetTaskId() != "task-1" {
		t.Fatalf("unexpected node task list: %+v", response.Msg.GetTasks())
	}
}

func TestNodeMaintenanceServiceReloadNodeCaddyCreatesTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"edge-proxy/composia-meta.yaml": "name: edge-proxy\nnode: main\ninfra:\n  caddy:\n    compose_service: caddy\n    config_dir: /etc/caddy\n",
	})
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "edge-proxy"); err != nil {
		t.Fatalf("sync declared services: %v", err)
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
	path, handler := controllerv1connect.NewNodeMaintenanceServiceHandler(
		&nodeMaintenanceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, taskQueue: newTaskQueueNotifier(), taskResults: newTaskResultNotifier()},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeMaintenanceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.ReloadNodeCaddy(ctx, connect.NewRequest(&controllerv1.ReloadNodeCaddyRequest{NodeId: "main"}))
	if err != nil {
		t.Fatalf("reload node caddy: %v", err)
	}
	if response.Msg.GetTaskId() == "" {
		t.Fatalf("expected task_id in response")
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if detail.Record.Type != task.TypeCaddyReload {
		t.Fatalf("expected caddy_reload task type, got %q", detail.Record.Type)
	}
	if detail.Record.ServiceName != "edge-proxy" || detail.Record.NodeID != "main" {
		t.Fatalf("unexpected created task record: %+v", detail.Record)
	}
}

func TestNodeMaintenanceServiceSyncNodeCaddyFilesCreatesSyncTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnode: main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
		"demo/demo.caddy":         "demo.example.com { reverse_proxy 127.0.0.1:8080 }\n",
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
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
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
	path, handler := controllerv1connect.NewNodeMaintenanceServiceHandler(
		&nodeMaintenanceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, taskQueue: newTaskQueueNotifier(), taskResults: newTaskResultNotifier()},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeMaintenanceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.SyncNodeCaddyFiles(ctx, connect.NewRequest(&controllerv1.SyncNodeCaddyFilesRequest{NodeId: "main", ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("sync node caddy files: %v", err)
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if detail.Record.Type != task.TypeCaddySync {
		t.Fatalf("expected caddy_sync task type, got %q", detail.Record.Type)
	}
	params := taskParams(detail.Record.ParamsJSON)
	if len(params.ServiceDirs) != 1 || params.ServiceDirs[0] != "demo" || params.FullRebuild {
		t.Fatalf("unexpected caddy sync params: %+v", params)
	}
}

func TestNodeMaintenanceServicePruneNodeRusticCreatesRusticPruneTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n",
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
	if err := syncDeclaredServicesForTests(ctx, db, "backup"); err != nil {
		t.Fatalf("sync declared services: %v", err)
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
	path, handler := controllerv1connect.NewNodeMaintenanceServiceHandler(
		&nodeMaintenanceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}, Rustic: &config.ControllerRusticConfig{MainNodes: []string{"main"}}}, taskQueue: newTaskQueueNotifier(), taskResults: newTaskResultNotifier()},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeMaintenanceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.PruneNodeRustic(ctx, connect.NewRequest(&controllerv1.PruneNodeRusticRequest{ServiceName: "demo", DataName: "db"}))
	if err != nil {
		t.Fatalf("prune node rustic: %v", err)
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if detail.Record.Type != task.TypeRusticPrune {
		t.Fatalf("expected rustic_prune task type, got %q", detail.Record.Type)
	}
	if detail.Record.ServiceName != "backup" || detail.Record.NodeID != "main" {
		t.Fatalf("unexpected created task record: %+v", detail.Record)
	}
	if !strings.Contains(detail.Record.ParamsJSON, `"service_name":"demo"`) || !strings.Contains(detail.Record.ParamsJSON, `"data_name":"db"`) {
		t.Fatalf("unexpected rustic prune params %q", detail.Record.ParamsJSON)
	}
}

func TestNodeMaintenanceServiceForgetNodeRusticCreatesRusticForgetTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n",
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
	if err := syncDeclaredServicesForTests(ctx, db, "backup"); err != nil {
		t.Fatalf("sync declared services: %v", err)
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
	path, handler := controllerv1connect.NewNodeMaintenanceServiceHandler(
		&nodeMaintenanceServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}, Rustic: &config.ControllerRusticConfig{MainNodes: []string{"main"}}}, taskQueue: newTaskQueueNotifier(), taskResults: newTaskResultNotifier()},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeMaintenanceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.ForgetNodeRustic(ctx, connect.NewRequest(&controllerv1.ForgetNodeRusticRequest{ServiceName: "demo", DataName: "db"}))
	if err != nil {
		t.Fatalf("forget node rustic: %v", err)
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if detail.Record.Type != task.TypeRusticForget {
		t.Fatalf("expected rustic_forget task type, got %q", detail.Record.Type)
	}
	if detail.Record.ServiceName != "backup" || detail.Record.NodeID != "main" {
		t.Fatalf("unexpected created task record: %+v", detail.Record)
	}
	if !strings.Contains(detail.Record.ParamsJSON, `"service_name":"demo"`) || !strings.Contains(detail.Record.ParamsJSON, `"data_name":"db"`) {
		t.Fatalf("unexpected rustic forget params %q", detail.Record.ParamsJSON)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
