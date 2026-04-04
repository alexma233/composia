package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestTaskServiceListTasks(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}

	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:      "task-1",
		Type:        task.TypeDeploy,
		Source:      task.SourceCLI,
		ServiceName: "alpha",
		NodeID:      "main",
		Status:      task.StatusSucceeded,
		CreatedAt:   time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create first task: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:      "task-2",
		Type:        task.TypeBackup,
		Source:      task.SourceCLI,
		ServiceName: "alpha",
		NodeID:      "main",
		Status:      task.StatusFailed,
		CreatedAt:   time.Date(2026, 4, 4, 16, 5, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create second task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewTaskServiceHandler(
		&taskServer{db: db},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewTaskServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")),
	)

	response, err := client.ListTasks(ctx, connect.NewRequest(&controllerv1.ListTasksRequest{ServiceName: "alpha", PageSize: 1}))
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(response.Msg.GetTasks()) != 1 {
		t.Fatalf("expected 1 task, got %d", len(response.Msg.GetTasks()))
	}
	if response.Msg.GetTasks()[0].GetTaskId() != "task-2" {
		t.Fatalf("expected newest task task-2, got %q", response.Msg.GetTasks()[0].GetTaskId())
	}
	if response.Msg.GetNextCursor() != "task-2" {
		t.Fatalf("expected next cursor task-2, got %q", response.Msg.GetNextCursor())
	}
}
