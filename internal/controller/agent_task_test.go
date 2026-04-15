package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAgentPullAndReportTaskFlow(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repoDir := t.TempDir()
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
		"edge/composia-meta.yaml": "name: edge\nnodes:\n  - main\ninfra:\n  caddy:\n    compose_service: caddy\n    config_dir: /etc/caddy\n",
	})
	logDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 16, 59, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo", "edge"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	revision := currentRevision(t, repoDir)
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "demo"})
	if err != nil {
		t.Fatalf("marshal deploy task params: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "logs", "task.log")
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:       "task-remote",
		Type:         task.TypeDeploy,
		Source:       task.SourceWeb,
		ServiceName:  "demo",
		NodeID:       "main",
		CreatedAt:    time.Date(2026, 4, 4, 17, 0, 0, 0, time.UTC),
		ParamsJSON:   string(paramsJSON),
		RepoRevision: revision,
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
	reportPath, reportHandler := agentv1connect.NewAgentReportServiceHandler(&agentReportServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, logState: &taskLogAckState{confirmedBy: make(map[string]uint64)}, taskQueue: newTaskQueueNotifier(), taskResults: newTaskResultNotifier()}, connect.WithInterceptors(interceptor))
	mux.Handle(reportPath, reportHandler)
	taskPath, taskHandler := agentv1connect.NewAgentTaskServiceHandler(&agentTaskServer{db: db}, connect.WithInterceptors(interceptor))
	mux.Handle(taskPath, taskHandler)
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
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
	if pulled.Msg.GetTask().GetServiceDir() == "" {
		t.Fatalf("expected pulled task to include service_dir")
	}

	startedAt := timestamppb.New(time.Date(2026, 4, 4, 17, 1, 0, 0, time.UTC))
	finishedAt := timestamppb.New(time.Date(2026, 4, 4, 17, 2, 0, 0, time.UTC))
	if _, err := reportClient.ReportTaskStepState(ctx, connect.NewRequest(&agentv1.ReportTaskStepStateRequest{TaskId: "task-remote", StepName: "render", Status: "succeeded", StartedAt: startedAt, FinishedAt: finishedAt})); err != nil {
		t.Fatalf("report task step state: %v", err)
	}
	logStream := reportClient.UploadTaskLogs(ctx)
	if err := logStream.Send(&agentv1.UploadTaskLogsRequest{TaskId: "task-remote", Seq: 1, SentAt: finishedAt, Content: "hello from agent\n"}); err != nil {
		t.Fatalf("send task logs: %v", err)
	}
	logAck, err := logStream.Receive()
	if err != nil {
		t.Fatalf("receive log ack: %v", err)
	}
	if logAck.GetLastConfirmedSeq() != 1 {
		t.Fatalf("expected last_confirmed_seq 1, got %d", logAck.GetLastConfirmedSeq())
	}
	if err := logStream.CloseRequest(); err != nil {
		t.Fatalf("close log request: %v", err)
	}
	if err := logStream.CloseResponse(); err != nil {
		t.Fatalf("close log response: %v", err)
	}
	replayStream := reportClient.UploadTaskLogs(ctx)
	if err := replayStream.Send(&agentv1.UploadTaskLogsRequest{TaskId: "task-remote", Seq: 1, SentAt: finishedAt, Content: "hello from agent\n"}); err != nil {
		t.Fatalf("send replayed task logs: %v", err)
	}
	replayAck, err := replayStream.Receive()
	if err != nil {
		t.Fatalf("receive replay ack: %v", err)
	}
	if replayAck.GetLastConfirmedSeq() != 1 {
		t.Fatalf("expected replay ack seq 1, got %d", replayAck.GetLastConfirmedSeq())
	}
	if err := replayStream.Send(&agentv1.UploadTaskLogsRequest{TaskId: "task-remote", Seq: 2, SentAt: finishedAt, Content: "second line\n"}); err != nil {
		t.Fatalf("send second task log: %v", err)
	}
	secondAck, err := replayStream.Receive()
	if err != nil {
		t.Fatalf("receive second ack: %v", err)
	}
	if secondAck.GetLastConfirmedSeq() != 2 {
		t.Fatalf("expected second ack seq 2, got %d", secondAck.GetLastConfirmedSeq())
	}
	if err := replayStream.CloseRequest(); err != nil {
		t.Fatalf("close replay request: %v", err)
	}
	if err := replayStream.CloseResponse(); err != nil {
		t.Fatalf("close replay response: %v", err)
	}
	if _, err := reportClient.ReportTaskState(ctx, connect.NewRequest(&agentv1.ReportTaskStateRequest{TaskId: "task-remote", Status: "succeeded", FinishedAt: finishedAt})); err != nil {
		t.Fatalf("report task state: %v", err)
	}
	if _, err := reportClient.ReportServiceInstanceStatus(ctx, connect.NewRequest(&agentv1.ReportServiceInstanceStatusRequest{ServiceName: "demo", NodeId: "main", RuntimeStatus: store.ServiceRuntimeRunning, ReportedAt: finishedAt})); err != nil {
		t.Fatalf("report service instance status: %v", err)
	}

	detail, err := db.GetTask(ctx, "task-remote")
	if err != nil {
		t.Fatalf("get task detail: %v", err)
	}
	if detail.Record.Status != task.StatusSucceeded {
		t.Fatalf("expected succeeded task, got %q", detail.Record.Status)
	}
	reloadTasks, totalCount, err := db.ListTasks(ctx, []string{string(task.StatusPending)}, []string{"edge"}, []string{"main"}, []string{string(task.TypeCaddyReload)}, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("list caddy reload tasks: %v", err)
	}
	if totalCount != 1 || len(reloadTasks) != 1 {
		t.Fatalf("expected one queued caddy reload task, got total=%d tasks=%+v", totalCount, reloadTasks)
	}
	reloadDetail, err := db.GetTask(ctx, reloadTasks[0].TaskID)
	if err != nil {
		t.Fatalf("get caddy reload task: %v", err)
	}
	if reloadDetail.Record.Source != task.SourceWeb {
		t.Fatalf("expected caddy reload source web, got %q", reloadDetail.Record.Source)
	}
	syncTasks, syncCount, err := db.ListTasks(ctx, []string{string(task.StatusPending)}, []string{"edge"}, []string{"main"}, []string{string(task.TypeCaddySync)}, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("list caddy sync tasks: %v", err)
	}
	if syncCount != 0 || len(syncTasks) != 0 {
		t.Fatalf("expected no separate caddy sync tasks, got total=%d tasks=%+v", syncCount, syncTasks)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].StepName != task.StepRender {
		t.Fatalf("unexpected task steps: %+v", detail.Steps)
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read task log: %v", err)
	}
	if string(content) != "hello from agent\nsecond line\n" {
		t.Fatalf("unexpected task log content %q", string(content))
	}
	snapshot, err := db.GetServiceSnapshot(ctx, "demo")
	if err != nil {
		t.Fatalf("get service snapshot: %v", err)
	}
	if snapshot.RuntimeStatus != store.ServiceRuntimeRunning {
		t.Fatalf("expected runtime status running, got %q", snapshot.RuntimeStatus)
	}
}

func TestReportServiceInstanceStatusRejectsMismatchedNode(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "other"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
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
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
	defer httpServer.Close()

	reportClient := agentv1connect.NewAgentReportServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))

	_, err := reportClient.ReportServiceInstanceStatus(ctx, connect.NewRequest(&agentv1.ReportServiceInstanceStatusRequest{
		ServiceName:   "demo",
		NodeId:        "other",
		RuntimeStatus: store.ServiceRuntimeRunning,
		ReportedAt:    timestamppb.New(time.Date(2026, 4, 4, 17, 2, 0, 0, time.UTC)),
	}))
	if err == nil {
		t.Fatal("expected unauthenticated error")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect error, got %T", err)
	}
	if connectErr.Code() != connect.CodeUnauthenticated {
		t.Fatalf("expected unauthenticated, got %v", connectErr.Code())
	}
	if got := connectErr.Message(); got != "node_id does not match bearer token" {
		t.Fatalf("unexpected error message %q", got)
	}
}

func TestAgentPullNextTaskLongPollWaitsForNewTask(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})
	notifier := newTaskQueueNotifier()

	mux := http.NewServeMux()
	taskPath, taskHandler := agentv1connect.NewAgentTaskServiceHandler(&agentTaskServer{db: db, taskQueue: notifier, maxWait: 700 * time.Millisecond, retryInterval: 500 * time.Millisecond}, connect.WithInterceptors(interceptor))
	mux.Handle(taskPath, taskHandler)
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
	defer httpServer.Close()

	taskClient := agentv1connect.NewAgentTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	pullCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	responseCh := make(chan *agentv1.PullNextTaskResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		response, err := taskClient.PullNextTask(pullCtx, connect.NewRequest(&agentv1.PullNextTaskRequest{NodeId: "main"}))
		if err != nil {
			errCh <- err
			return
		}
		responseCh <- response.Msg
	}()

	time.Sleep(50 * time.Millisecond)
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "demo"})
	if err != nil {
		t.Fatalf("marshal deploy task params: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:       "task-long-poll",
		Type:         task.TypeDeploy,
		Source:       task.SourceCLI,
		ServiceName:  "demo",
		NodeID:       "main",
		ParamsJSON:   string(paramsJSON),
		RepoRevision: "deadbeef",
		CreatedAt:    time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	notifier.Notify()

	select {
	case err := <-errCh:
		t.Fatalf("pull next task: %v", err)
	case response := <-responseCh:
		if !response.GetHasTask() || response.GetTask().GetTaskId() != "task-long-poll" {
			t.Fatalf("unexpected long-poll response: %+v", response)
		}
	case <-time.After(900 * time.Millisecond):
		t.Fatalf("timed out waiting for long-poll response")
	}
}

func TestAgentPullNextTaskLongPollWakesWhenRunningTaskCompletes(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "node-2"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	runningStartedAt := time.Date(2026, 4, 5, 11, 0, 10, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:      "task-running",
		Type:        task.TypeDeploy,
		Source:      task.SourceCLI,
		ServiceName: "alpha",
		NodeID:      "main",
		Status:      task.StatusRunning,
		CreatedAt:   time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC),
		StartedAt:   &runningStartedAt,
	}); err != nil {
		t.Fatalf("create running task: %v", err)
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "bravo"})
	if err != nil {
		t.Fatalf("marshal pending task params: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:       "task-pending",
		Type:         task.TypeDeploy,
		Source:       task.SourceCLI,
		ServiceName:  "bravo",
		NodeID:       "node-2",
		ParamsJSON:   string(paramsJSON),
		RepoRevision: "cafebabe",
		CreatedAt:    time.Date(2026, 4, 5, 11, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create pending task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "node-2-token" {
			return "", assertError("unexpected token")
		}
		return "node-2", nil
	})
	notifier := newTaskQueueNotifier()

	mux := http.NewServeMux()
	taskPath, taskHandler := agentv1connect.NewAgentTaskServiceHandler(&agentTaskServer{db: db, taskQueue: notifier, maxWait: time.Second, retryInterval: 700 * time.Millisecond}, connect.WithInterceptors(interceptor))
	mux.Handle(taskPath, taskHandler)
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
	defer httpServer.Close()

	taskClient := agentv1connect.NewAgentTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("node-2-token")))
	pullCtx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	responseCh := make(chan *agentv1.PullNextTaskResponse, 1)
	errCh := make(chan error, 1)
	startedAt := time.Now()
	go func() {
		response, err := taskClient.PullNextTask(pullCtx, connect.NewRequest(&agentv1.PullNextTaskRequest{NodeId: "node-2"}))
		if err != nil {
			errCh <- err
			return
		}
		responseCh <- response.Msg
	}()

	time.Sleep(50 * time.Millisecond)
	if err := db.CompleteTask(ctx, "task-running", task.StatusSucceeded, time.Date(2026, 4, 5, 11, 2, 0, 0, time.UTC), ""); err != nil {
		t.Fatalf("complete running task: %v", err)
	}
	notifier.Notify()

	select {
	case err := <-errCh:
		t.Fatalf("pull next task: %v", err)
	case response := <-responseCh:
		if elapsed := time.Since(startedAt); elapsed >= 500*time.Millisecond {
			t.Fatalf("expected notifier wake-up before retry fallback, got %v", elapsed)
		}
		if !response.GetHasTask() || response.GetTask().GetTaskId() != "task-pending" {
			t.Fatalf("unexpected response after running task completion: %+v", response)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for pending task after completion")
	}
}

func TestAgentReportTaskStateSkipsCaddyReloadWhenServiceDoesNotUseCaddy(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repoDir := t.TempDir()
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\n",
		"edge/composia-meta.yaml": "name: edge\nnodes:\n  - main\ninfra:\n  caddy:\n    compose_service: caddy\n    config_dir: /etc/caddy\n",
	})
	logDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo", "edge"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	revision := currentRevision(t, repoDir)
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "demo"})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "logs", "task.log")
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-no-caddy", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", ParamsJSON: string(paramsJSON), RepoRevision: revision, LogPath: logPath}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})
	mux := http.NewServeMux()
	reportPath, reportHandler := agentv1connect.NewAgentReportServiceHandler(&agentReportServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, logState: &taskLogAckState{confirmedBy: make(map[string]uint64)}, taskQueue: newTaskQueueNotifier(), taskResults: newTaskResultNotifier()}, connect.WithInterceptors(interceptor))
	mux.Handle(reportPath, reportHandler)
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
	defer httpServer.Close()

	reportClient := agentv1connect.NewAgentReportServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	finishedAt := timestamppb.New(time.Date(2026, 4, 4, 18, 2, 0, 0, time.UTC))
	if _, err := reportClient.ReportTaskState(ctx, connect.NewRequest(&agentv1.ReportTaskStateRequest{TaskId: "task-no-caddy", Status: "succeeded", FinishedAt: finishedAt})); err != nil {
		t.Fatalf("report task state: %v", err)
	}

	reloadTasks, totalCount, err := db.ListTasks(ctx, []string{string(task.StatusPending)}, []string{"edge"}, []string{"main"}, []string{string(task.TypeCaddyReload)}, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("list caddy reload tasks: %v", err)
	}
	if totalCount != 0 || len(reloadTasks) != 0 {
		t.Fatalf("expected no queued caddy reload task, got total=%d tasks=%+v", totalCount, reloadTasks)
	}
}

func TestAgentReportTaskStateQueuesCaddyReloadAfterStop(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repoDir := t.TempDir()
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
		"edge/composia-meta.yaml": "name: edge\nnodes:\n  - main\ninfra:\n  caddy:\n    compose_service: caddy\n    config_dir: /etc/caddy\n",
	})
	logDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 4, 16, 59, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo", "edge"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	revision := currentRevision(t, repoDir)
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "demo"})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "logs", "task.log")
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-stop-caddy", Type: task.TypeStop, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", ParamsJSON: string(paramsJSON), RepoRevision: revision, LogPath: logPath}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})
	mux := http.NewServeMux()
	reportPath, reportHandler := agentv1connect.NewAgentReportServiceHandler(&agentReportServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, logState: &taskLogAckState{confirmedBy: make(map[string]uint64)}, taskQueue: newTaskQueueNotifier(), taskResults: newTaskResultNotifier()}, connect.WithInterceptors(interceptor))
	mux.Handle(reportPath, reportHandler)
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
	defer httpServer.Close()

	reportClient := agentv1connect.NewAgentReportServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	finishedAt := timestamppb.New(time.Date(2026, 4, 4, 18, 2, 0, 0, time.UTC))
	if _, err := reportClient.ReportTaskState(ctx, connect.NewRequest(&agentv1.ReportTaskStateRequest{TaskId: "task-stop-caddy", Status: "succeeded", FinishedAt: finishedAt})); err != nil {
		t.Fatalf("report task state: %v", err)
	}

	reloadTasks, totalCount, err := db.ListTasks(ctx, []string{string(task.StatusPending)}, []string{"edge"}, []string{"main"}, []string{string(task.TypeCaddyReload)}, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("list caddy reload tasks: %v", err)
	}
	if totalCount != 1 || len(reloadTasks) != 1 {
		t.Fatalf("expected one queued caddy reload task after stop, got total=%d tasks=%+v", totalCount, reloadTasks)
	}
	reloadDetail, err := db.GetTask(ctx, reloadTasks[0].TaskID)
	if err != nil {
		t.Fatalf("get caddy reload task: %v", err)
	}
	if reloadDetail.Record.Source != task.SourceCLI {
		t.Fatalf("expected caddy reload source cli, got %q", reloadDetail.Record.Source)
	}
}
