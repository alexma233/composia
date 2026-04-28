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
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestTaskServiceListTasks(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 17, 59, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
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
		if token != "access-token" {
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
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
	)

	response, err := client.ListTasks(ctx, connect.NewRequest(&controllerv1.ListTasksRequest{ServiceName: []string{"alpha"}, PageSize: 1}))
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(response.Msg.GetTasks()) != 1 {
		t.Fatalf("expected 1 task, got %d", len(response.Msg.GetTasks()))
	}
	if response.Msg.GetTasks()[0].GetTaskId() != "task-2" {
		t.Fatalf("expected newest task task-2, got %q", response.Msg.GetTasks()[0].GetTaskId())
	}
	if response.Msg.GetTotalCount() != 2 {
		t.Fatalf("expected total count 2, got %d", response.Msg.GetTotalCount())
	}

	filtered, err := client.ListTasks(ctx, connect.NewRequest(&controllerv1.ListTasksRequest{NodeId: []string{"main"}, Type: []string{string(task.TypeDeploy)}, PageSize: 10}))
	if err != nil {
		t.Fatalf("list filtered tasks: %v", err)
	}
	if len(filtered.Msg.GetTasks()) != 1 || filtered.Msg.GetTasks()[0].GetTaskId() != "task-1" {
		t.Fatalf("unexpected filtered task list: %+v", filtered.Msg.GetTasks())
	}
}

func TestTaskServiceGetTaskReturnsSteps(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	createdAt := time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC)
	startedAt := createdAt.Add(1 * time.Minute)
	finishedAt := createdAt.Add(2 * time.Minute)
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:      "task-origin",
		Type:        task.TypeDeploy,
		Source:      task.SourceCLI,
		ServiceName: "alpha",
		NodeID:      "main",
		Status:      task.StatusSucceeded,
		CreatedAt:   createdAt.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("create origin task fixture: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:          "task-detail",
		Type:            task.TypeDeploy,
		Source:          task.SourceCLI,
		TriggeredBy:     "test-client",
		ServiceName:     "alpha",
		NodeID:          "main",
		Status:          task.StatusSucceeded,
		CreatedAt:       createdAt,
		StartedAt:       &startedAt,
		FinishedAt:      &finishedAt,
		RepoRevision:    "deadbeef",
		ResultRevision:  "feedface",
		AttemptOfTaskID: "task-origin",
		LogPath:         "/tmp/task-detail.log",
	}); err != nil {
		t.Fatalf("create task detail fixture: %v", err)
	}
	if err := db.UpsertTaskStep(ctx, task.StepRecord{TaskID: "task-detail", StepName: task.StepRender, Status: task.StatusSucceeded, StartedAt: &startedAt, FinishedAt: &finishedAt}); err != nil {
		t.Fatalf("insert task step: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
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
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
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
	if response.Msg.GetTriggeredBy() != "test-client" {
		t.Fatalf("expected triggered_by test-client, got %q", response.Msg.GetTriggeredBy())
	}
	if response.Msg.GetResultRevision() != "feedface" {
		t.Fatalf("expected result_revision feedface, got %q", response.Msg.GetResultRevision())
	}
	if response.Msg.GetAttemptOfTaskId() != "task-origin" {
		t.Fatalf("expected attempt_of_task_id task-origin, got %q", response.Msg.GetAttemptOfTaskId())
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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-log-tail", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", LogPath: logPath, CreatedAt: time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task log tail fixture: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewTaskServiceHandler(&taskServer{db: db}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := client.TailTaskLogs(streamCtx, connect.NewRequest(&controllerv1.TailTaskLogsRequest{TaskId: "task-log-tail"}))
	if err != nil {
		t.Fatalf("tail task logs: %v", err)
	}
	defer func() { _ = stream.Close() }()

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

func TestTaskServiceRunTaskAgainCreatesNewPendingTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 17, 59, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-old", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", Status: task.StatusSucceeded, CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create old task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewTaskServiceHandler(&taskServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.RunTaskAgain(ctx, connect.NewRequest(&controllerv1.RunTaskAgainRequest{TaskId: "task-old"}))
	if err != nil {
		t.Fatalf("run task again: %v", err)
	}
	if response.Msg.GetTaskId() == "" || response.Msg.GetTaskId() == "task-old" {
		t.Fatalf("expected new task ID, got %q", response.Msg.GetTaskId())
	}
	if response.Msg.GetStatus() != "pending" {
		t.Fatalf("expected pending rerun task, got %q", response.Msg.GetStatus())
	}
	detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get rerun task: %v", err)
	}
	if detail.Record.AttemptOfTaskID != "task-old" {
		t.Fatalf("expected attempt_of_task_id task-old, got %q", detail.Record.AttemptOfTaskID)
	}
}

func TestTaskServiceRunTaskAgainSupportsBackup(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 17, 59, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-backup", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", Status: task.StatusSucceeded, ParamsJSON: `{"service_dir":"alpha","data_names":["config"]}`, CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create backup task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewTaskServiceHandler(&taskServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.RunTaskAgain(ctx, connect.NewRequest(&controllerv1.RunTaskAgainRequest{TaskId: "task-backup"}))
	if err != nil {
		t.Fatalf("rerun backup task: %v", err)
	}
	if response.Msg.GetTaskId() == "" || response.Msg.GetTaskId() == "task-backup" {
		t.Fatalf("expected new backup task ID, got %q", response.Msg.GetTaskId())
	}
	backupDetail, err := db.GetTask(ctx, response.Msg.GetTaskId())
	if err != nil {
		t.Fatalf("get rerun backup task: %v", err)
	}
	if backupDetail.Record.Type != task.TypeBackup {
		t.Fatalf("expected rerun type backup, got %q", backupDetail.Record.Type)
	}
	if backupDetail.Record.AttemptOfTaskID != "task-backup" {
		t.Fatalf("expected attempt_of_task_id task-backup, got %q", backupDetail.Record.AttemptOfTaskID)
	}
}

func TestTaskServiceResolveTaskConfirmationApproveRequeuesMigrateTask(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "task.log")
	startedAt := time.Date(2026, 4, 4, 19, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-migrate", Type: task.TypeMigrate, Source: task.SourceCLI, ServiceName: "alpha", Status: task.StatusAwaitingConfirmation, LogPath: logPath, CreatedAt: startedAt.Add(-time.Minute), StartedAt: &startedAt}); err != nil {
		t.Fatalf("create migrate task: %v", err)
	}
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatalf("create log file: %v", err)
	}
	if err := db.UpsertTaskStep(ctx, task.StepRecord{TaskID: "task-migrate", StepName: task.StepAwaitingConfirmation, Status: task.StatusAwaitingConfirmation, StartedAt: &startedAt}); err != nil {
		t.Fatalf("create awaiting step: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewTaskServiceHandler(&taskServer{db: db, taskQueue: newTaskQueueNotifier()}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.ResolveTaskConfirmation(ctx, connect.NewRequest(&controllerv1.ResolveTaskConfirmationRequest{TaskId: "task-migrate", Decision: "approve"}))
	if err != nil {
		t.Fatalf("resolve task confirmation: %v", err)
	}
	if response.Msg.GetStatus() != string(task.StatusPending) {
		t.Fatalf("expected pending status, got %q", response.Msg.GetStatus())
	}
	detail, err := db.GetTask(ctx, "task-migrate")
	if err != nil {
		t.Fatalf("get migrate task: %v", err)
	}
	if detail.Record.Status != task.StatusPending {
		t.Fatalf("expected pending task, got %q", detail.Record.Status)
	}
	if !hasTaskStepStatus(detail.Steps, task.StepAwaitingConfirmation, task.StatusSucceeded) {
		t.Fatalf("expected awaiting_confirmation step to be succeeded, got %+v", detail.Steps)
	}
	logContent, err := os.ReadFile(detail.Record.LogPath)
	if err != nil {
		t.Fatalf("read task log: %v", err)
	}
	if string(logContent) == "" {
		t.Fatalf("expected confirmation log entry")
	}
}

func TestTaskServiceResolveTaskConfirmationRejectCancelsMigrateTask(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "task.log")
	startedAt := time.Date(2026, 4, 4, 19, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-migrate-reject", Type: task.TypeMigrate, Source: task.SourceCLI, ServiceName: "alpha", Status: task.StatusAwaitingConfirmation, LogPath: logPath, CreatedAt: startedAt.Add(-time.Minute), StartedAt: &startedAt}); err != nil {
		t.Fatalf("create migrate task: %v", err)
	}
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatalf("create log file: %v", err)
	}
	if err := db.UpsertTaskStep(ctx, task.StepRecord{TaskID: "task-migrate-reject", StepName: task.StepAwaitingConfirmation, Status: task.StatusAwaitingConfirmation, StartedAt: &startedAt}); err != nil {
		t.Fatalf("create awaiting step: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewTaskServiceHandler(&taskServer{db: db, taskQueue: newTaskQueueNotifier()}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	response, err := client.ResolveTaskConfirmation(ctx, connect.NewRequest(&controllerv1.ResolveTaskConfirmationRequest{TaskId: "task-migrate-reject", Decision: "reject"}))
	if err != nil {
		t.Fatalf("resolve task confirmation: %v", err)
	}
	if response.Msg.GetStatus() != string(task.StatusCancelled) {
		t.Fatalf("expected cancelled status, got %q", response.Msg.GetStatus())
	}
	detail, err := db.GetTask(ctx, "task-migrate-reject")
	if err != nil {
		t.Fatalf("get migrate task: %v", err)
	}
	if detail.Record.Status != task.StatusCancelled {
		t.Fatalf("expected cancelled task, got %q", detail.Record.Status)
	}
	if detail.Record.ErrorSummary != "manual verification rejected" {
		t.Fatalf("unexpected error summary %q", detail.Record.ErrorSummary)
	}
	if !hasTaskStepStatus(detail.Steps, task.StepAwaitingConfirmation, task.StatusCancelled) {
		t.Fatalf("expected awaiting_confirmation step to be cancelled, got %+v", detail.Steps)
	}
}
