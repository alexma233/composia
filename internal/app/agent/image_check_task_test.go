package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
)

func TestExecuteImageCheckTaskSkipsConfigInfraService(t *testing.T) {
	rootDir := t.TempDir()
	cfg := &config.AgentConfig{RepoDir: rootDir, StateDir: rootDir}
	bundle := buildBundleArchive(t, map[string]string{
		"host-service/composia-meta.yaml": "name: host-service\nnodes:\n  - main\ninfra:\n  config: {}\n",
	})

	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-image-check", responseServiceName: "host-service", responseRelativeRoot: "host-service"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errors.New("unexpected token")
		}
		return "main", nil
	})))
	bundleMux.Handle(bundlePath, bundleHandler)
	bundleHTTPServer := httptest.NewServer(bundleMux)
	defer bundleHTTPServer.Close()

	reportServer := &agentExecutionTestReportServer{}
	reportMux := http.NewServeMux()
	reportPath, reportHandler := agentv1connect.NewAgentReportServiceHandler(reportServer, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errors.New("unexpected token")
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
	logUploader := newTaskLogUploader(reportClient, "task-image-check")
	defer func() { _ = logUploader.Close() }()
	pulledTask := &agentv1.AgentTask{TaskId: "task-image-check", Type: protoAgentTaskType(task.TypeImageCheck), ServiceName: "host-service", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "host-service"}

	if err := executeImageCheckTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("executeImageCheckTask returned error: %v", err)
	}

	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("task status = %q", reportServer.taskStatus)
	}
	if reportServer.stepStatuses[task.StepRender] != string(task.StatusSucceeded) || reportServer.stepStatuses[task.StepImageCheck] != string(task.StatusSucceeded) {
		t.Fatalf("unexpected step statuses: %+v", reportServer.stepStatuses)
	}
}
