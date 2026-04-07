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

func TestNodeServiceListNodes(t *testing.T) {
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

	path, handler := controllerv1connect.NewNodeServiceHandler(
		&nodeServer{
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

	client := controllerv1connect.NewNodeServiceClient(
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

func TestNodeServiceGetNodeReturnsMinimalSummary(t *testing.T) {
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
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewNodeServiceHandler(
		&nodeServer{db: db, cfg: &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main", DisplayName: "Main"}}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeServiceClient(
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

func TestNodeServiceGetNodeTasksReturnsFilteredTasks(t *testing.T) {
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
	path, handler := controllerv1connect.NewNodeServiceHandler(
		&nodeServer{db: db, cfg: &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main"}, {ID: "node-2"}}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewNodeServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.GetNodeTasks(ctx, connect.NewRequest(&controllerv1.GetNodeTasksRequest{NodeId: "main", PageSize: 10}))
	if err != nil {
		t.Fatalf("get node tasks: %v", err)
	}
	if len(response.Msg.GetTasks()) != 1 || response.Msg.GetTasks()[0].GetTaskId() != "task-1" {
		t.Fatalf("unexpected node task list: %+v", response.Msg.GetTasks())
	}
}

func boolPtr(value bool) *bool {
	return &value
}
