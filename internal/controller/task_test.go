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

func TestTaskServiceGetTaskReturnsSteps(t *testing.T) {
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
	createdAt := time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC)
	startedAt := createdAt.Add(1 * time.Minute)
	finishedAt := createdAt.Add(2 * time.Minute)
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:       "task-detail",
		Type:         task.TypeDeploy,
		Source:       task.SourceCLI,
		ServiceName:  "alpha",
		NodeID:       "main",
		Status:       task.StatusSucceeded,
		CreatedAt:    createdAt,
		StartedAt:    &startedAt,
		FinishedAt:   &finishedAt,
		RepoRevision: "deadbeef",
		LogPath:      "/tmp/task-detail.log",
	}); err != nil {
		t.Fatalf("create task detail fixture: %v", err)
	}
	if err := db.UpsertTaskStep(ctx, task.StepRecord{TaskID: "task-detail", StepName: task.StepRender, Status: task.StatusSucceeded, StartedAt: &startedAt, FinishedAt: &finishedAt}); err != nil {
		t.Fatalf("insert task step: %v", err)
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

	response, err := client.GetTask(ctx, connect.NewRequest(&controllerv1.GetTaskRequest{TaskId: "task-detail"}))
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if response.Msg.GetTaskId() != "task-detail" || response.Msg.GetRepoRevision() != "deadbeef" {
		t.Fatalf("unexpected task detail response: %+v", response.Msg)
	}
	if len(response.Msg.GetSteps()) != 1 || response.Msg.GetSteps()[0].GetStepName() != "render" {
		t.Fatalf("unexpected task step response: %+v", response.Msg.GetSteps())
	}
	if response.Msg.GetLogPath() != "/tmp/task-detail.log" {
		t.Fatalf("expected log path /tmp/task-detail.log, got %q", response.Msg.GetLogPath())
	}
}

func TestTaskServiceTailTaskLogsStreamsExistingAndNewContent(t *testing.T) {
	t.Parallel()

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "task.log")
	if err := os.WriteFile(logPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write initial log file: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-log-tail", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", LogPath: logPath, CreatedAt: time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task log tail fixture: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewTaskServiceHandler(&taskServer{db: db}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := client.TailTaskLogs(streamCtx, connect.NewRequest(&controllerv1.TailTaskLogsRequest{TaskId: "task-log-tail"}))
	if err != nil {
		t.Fatalf("tail task logs: %v", err)
	}
	defer stream.Close()

	if !stream.Receive() {
		t.Fatalf("expected first log chunk, got err=%v", stream.Err())
	}
	if stream.Msg().GetContent() != "hello\n" {
		t.Fatalf("unexpected first log chunk %q", stream.Msg().GetContent())
	}

	if err := os.WriteFile(logPath, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("append log content: %v", err)
	}
	if !stream.Receive() {
		t.Fatalf("expected second log chunk, got err=%v", stream.Err())
	}
	if stream.Msg().GetContent() != "world\n" {
		t.Fatalf("unexpected second log chunk %q", stream.Msg().GetContent())
	}
	cancel()
}
