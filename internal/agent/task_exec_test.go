package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestExecuteStopTaskDownloadsBundleAndRunsComposeDown(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	argsFile := filepath.Join(rootDir, "args.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":  "name: demo\n",
		"demo/docker-compose.yaml": "services: {}\n",
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
		}
		return "main", nil
	})))
	bundleMux.Handle(bundlePath, bundleHandler)
	bundleHTTPServer := httptest.NewServer(bundleMux)
	defer bundleHTTPServer.Close()

	reportMux := http.NewServeMux()
	reportPath, reportHandler := agentv1connect.NewAgentReportServiceHandler(reportServer, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
		}
		return "main", nil
	})))
	reportMux.Handle(reportPath, reportHandler)
	reportHTTPServer := httptest.NewUnstartedServer(reportMux)
	reportHTTPServer.EnableHTTP2 = true
	reportHTTPServer.StartTLS()
	defer reportHTTPServer.Close()

	bundleClient := agentv1connect.NewBundleServiceClient(bundleHTTPServer.Client(), bundleHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	reportClient := agentv1connect.NewAgentReportServiceClient(reportHTTPServer.Client(), reportHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	logUploader := newTaskLogUploader(reportClient, "task-1")
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{
		TaskId:       "task-1",
		Type:         string(task.TypeStop),
		ServiceName:  "demo",
		NodeId:       "main",
		RepoRevision: "deadbeef",
		ServiceDir:   "demo",
	}
	if err := executeStopTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute stop task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if string(argsContent) != "compose --project-name demo down " {
		t.Fatalf("unexpected docker args %q", string(argsContent))
	}
	pwdContent, err := os.ReadFile(pwdFile)
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytesTrimSpace(pwdContent)) != filepath.Join(cfg.RepoDir, "demo") {
		t.Fatalf("expected docker cwd %q, got %q", filepath.Join(cfg.RepoDir, "demo"), string(bytesTrimSpace(pwdContent)))
	}

	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded task status, got %q", reportServer.taskStatus)
	}
	if reportServer.runtimeStatus != store.ServiceRuntimeStopped {
		t.Fatalf("expected stopped runtime status, got %q", reportServer.runtimeStatus)
	}
	if len(reportServer.stepStatuses) == 0 || reportServer.stepStatuses[task.StepRender] != string(task.StatusSucceeded) || reportServer.stepStatuses[task.StepComposeDown] != string(task.StatusSucceeded) {
		t.Fatalf("unexpected step statuses %+v", reportServer.stepStatuses)
	}
}

type agentExecutionTestReportServer struct {
	agentv1connect.UnimplementedAgentReportServiceHandler
	mu            sync.Mutex
	taskStatus    string
	runtimeStatus string
	stepStatuses  map[task.StepName]string
	confirmedSeq  uint64
}

func (server *agentExecutionTestReportServer) Heartbeat(context.Context, *connect.Request[agentv1.HeartbeatRequest]) (*connect.Response[agentv1.HeartbeatResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}

func (server *agentExecutionTestReportServer) ReportTaskState(_ context.Context, req *connect.Request[agentv1.ReportTaskStateRequest]) (*connect.Response[agentv1.ReportTaskStateResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.taskStatus = req.Msg.GetStatus()
	return connect.NewResponse(&agentv1.ReportTaskStateResponse{}), nil
}

func (server *agentExecutionTestReportServer) ReportTaskStepState(_ context.Context, req *connect.Request[agentv1.ReportTaskStepStateRequest]) (*connect.Response[agentv1.ReportTaskStepStateResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.stepStatuses == nil {
		server.stepStatuses = make(map[task.StepName]string)
	}
	server.stepStatuses[task.StepName(req.Msg.GetStepName())] = req.Msg.GetStatus()
	return connect.NewResponse(&agentv1.ReportTaskStepStateResponse{}), nil
}

func (server *agentExecutionTestReportServer) UploadTaskLogs(_ context.Context, stream *connect.BidiStream[agentv1.UploadTaskLogsRequest, agentv1.UploadTaskLogsResponse]) error {
	for {
		message, err := stream.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		server.mu.Lock()
		if message.GetSeq() > server.confirmedSeq {
			server.confirmedSeq = message.GetSeq()
		}
		confirmedSeq := server.confirmedSeq
		server.mu.Unlock()
		if err := stream.Send(&agentv1.UploadTaskLogsResponse{TaskId: message.GetTaskId(), LastConfirmedSeq: confirmedSeq}); err != nil {
			return err
		}
	}
}

func (server *agentExecutionTestReportServer) ReportBackupResult(context.Context, *connect.Request[agentv1.ReportBackupResultRequest]) (*connect.Response[agentv1.ReportBackupResultResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}

func (server *agentExecutionTestReportServer) ReportServiceStatus(_ context.Context, req *connect.Request[agentv1.ReportServiceStatusRequest]) (*connect.Response[agentv1.ReportServiceStatusResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.runtimeStatus = req.Msg.GetRuntimeStatus()
	return connect.NewResponse(&agentv1.ReportServiceStatusResponse{}), nil
}

var _ agentv1connect.AgentReportServiceHandler = (*agentExecutionTestReportServer)(nil)
