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

func TestDeployServiceFlowReachesSucceededPlaceholderState(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	stateDir := filepath.Join(rootDir, "state")
	createGitRepoWithService(t, repoDir, "demo", "main")
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create task log dir: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := db.SyncDeclaredServices(ctx, []string{"demo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}

	cfg := &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main", Token: "main-token"}}}
	availableNodeIDs := map[string]struct{}{"main": {}}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	mux := http.NewServeMux()
	servicePath, serviceHandler := controllerv1connect.NewServiceServiceHandler(&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs}, connect.WithInterceptors(interceptor))
	mux.Handle(servicePath, serviceHandler)
	taskPath, taskHandler := controllerv1connect.NewTaskServiceHandler(&taskServer{db: db}, connect.WithInterceptors(interceptor))
	mux.Handle(taskPath, taskHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	startTaskWorker(ctx, db, func(workerCtx context.Context, record task.Record) error {
		return executeTask(workerCtx, db, record)
	})

	serviceClient := controllerv1connect.NewServiceServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	taskClient := controllerv1connect.NewTaskServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))

	deployResponse, err := serviceClient.DeployService(ctx, connect.NewRequest(&controllerv1.DeployServiceRequest{ServiceName: "demo"}))
	if err != nil {
		t.Fatalf("deploy service: %v", err)
	}

	var taskResponse *connect.Response[controllerv1.GetTaskResponse]
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		taskResponse, err = taskClient.GetTask(ctx, connect.NewRequest(&controllerv1.GetTaskRequest{TaskId: deployResponse.Msg.GetTaskId()}))
		if err == nil && taskResponse.Msg.GetStatus() == "succeeded" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("get deploy task: %v", err)
	}
	if taskResponse.Msg.GetStatus() != "succeeded" {
		t.Fatalf("expected deploy task to succeed, got %q", taskResponse.Msg.GetStatus())
	}
	if len(taskResponse.Msg.GetSteps()) != 2 {
		t.Fatalf("expected 2 task steps, got %d", len(taskResponse.Msg.GetSteps()))
	}
	stepNames := map[string]bool{}
	for _, step := range taskResponse.Msg.GetSteps() {
		stepNames[step.GetStepName()] = true
	}
	if !stepNames["render"] || !stepNames["finalize"] {
		t.Fatalf("unexpected task steps: %+v", taskResponse.Msg.GetSteps())
	}
	content, err := os.ReadFile(taskResponse.Msg.GetLogPath())
	if err != nil {
		t.Fatalf("read task log: %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("expected non-empty task log")
	}
}
