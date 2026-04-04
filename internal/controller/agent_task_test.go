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
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAgentPullAndReportTaskFlow(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "logs", "task.log")
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:       "task-remote",
		Type:         task.TypeDeploy,
		Source:       task.SourceCLI,
		ServiceName:  "demo",
		NodeID:       "main",
		CreatedAt:    time.Date(2026, 4, 4, 17, 0, 0, 0, time.UTC),
		RepoRevision: "deadbeef",
		LogPath:      logPath,
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})

	mux := http.NewServeMux()
	reportPath, reportHandler := agentv1connect.NewAgentReportServiceHandler(&agentReportServer{db: db}, connect.WithInterceptors(interceptor))
	mux.Handle(reportPath, reportHandler)
	taskPath, taskHandler := agentv1connect.NewAgentTaskServiceHandler(&agentTaskServer{db: db}, connect.WithInterceptors(interceptor))
	mux.Handle(taskPath, taskHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	taskClient := agentv1connect.NewAgentTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	reportClient := agentv1connect.NewAgentReportServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))

	pulled, err := taskClient.PullNextTask(ctx, connect.NewRequest(&agentv1.PullNextTaskRequest{NodeId: "main"}))
	if err != nil {
		t.Fatalf("pull next task: %v", err)
	}
	if !pulled.Msg.GetHasTask() || pulled.Msg.GetTask().GetTaskId() != "task-remote" {
		t.Fatalf("unexpected pulled task: %+v", pulled.Msg)
	}

	startedAt := timestamppb.New(time.Date(2026, 4, 4, 17, 1, 0, 0, time.UTC))
	finishedAt := timestamppb.New(time.Date(2026, 4, 4, 17, 2, 0, 0, time.UTC))
	if _, err := reportClient.ReportTaskStepState(ctx, connect.NewRequest(&agentv1.ReportTaskStepStateRequest{TaskId: "task-remote", StepName: "render", Status: "succeeded", StartedAt: startedAt, FinishedAt: finishedAt})); err != nil {
		t.Fatalf("report task step state: %v", err)
	}
	if _, err := reportClient.UploadTaskLogs(ctx, connect.NewRequest(&agentv1.UploadTaskLogsRequest{TaskId: "task-remote", Content: "hello from agent\n"})); err != nil {
		t.Fatalf("upload task logs: %v", err)
	}
	if _, err := reportClient.ReportTaskState(ctx, connect.NewRequest(&agentv1.ReportTaskStateRequest{TaskId: "task-remote", Status: "succeeded", FinishedAt: finishedAt})); err != nil {
		t.Fatalf("report task state: %v", err)
	}

	detail, err := db.GetTask(ctx, "task-remote")
	if err != nil {
		t.Fatalf("get task detail: %v", err)
	}
	if detail.Record.Status != task.StatusSucceeded {
		t.Fatalf("expected succeeded task, got %q", detail.Record.Status)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].StepName != task.StepRender {
		t.Fatalf("unexpected task steps: %+v", detail.Steps)
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read task log: %v", err)
	}
	if string(content) != "hello from agent\n" {
		t.Fatalf("unexpected task log content %q", string(content))
	}
}
