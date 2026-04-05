package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-1"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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

func TestExecuteBackupTaskRunsRusticAndReportsSnapshot(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	rusticArgsFile := filepath.Join(rootDir, "rustic-args.txt")
	rusticPath := filepath.Join(binDir, "rustic")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	rusticScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_RUSTIC_ARGS_FILE\"\nif [ \"$5\" = \"backup\" ]; then printf 'snapshot abc12345 saved\\n'; else printf 'forget done\\n'; fi\n"
	if err := os.WriteFile(rusticPath, []byte(rusticScript), 0o755); err != nil {
		t.Fatalf("write fake rustic script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_RUSTIC_ARGS_FILE", rusticArgsFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"repository":"s3:https://example.invalid/repo","password":"secret-password"},"items":[{"name":"config","strategy":"files.copy","include":["./config"],"provider":"rustic","tags":["composia-service:demo","composia-data:config"],"retain":"--keep-daily 7 --prune"}]}`,
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-backup"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	logUploader := newTaskLogUploader(reportClient, "task-backup")
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-backup", Type: string(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"config"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute backup task: %v", err)
	}

	argsContent, err := os.ReadFile(rusticArgsFile)
	if err != nil {
		t.Fatalf("read rustic args file: %v", err)
	}
	argsLog := string(argsContent)
	if !strings.Contains(argsLog, "-r s3:https://example.invalid/repo --password-file") || !strings.Contains(argsLog, "backup") {
		t.Fatalf("unexpected rustic args %q", string(argsContent))
	}
	if !strings.Contains(argsLog, "--tag composia-service:demo --tag composia-data:config") || !strings.Contains(argsLog, "forget --tag composia-service:demo --tag composia-data:config --keep-daily 7 --prune") {
		t.Fatalf("expected backup and forget commands with tags/retain, got %q", argsLog)
	}
	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded task status, got %q", reportServer.taskStatus)
	}
	if reportServer.backupArtifactRef != "abc12345" {
		t.Fatalf("expected backup artifact ref abc12345, got %q", reportServer.backupArtifactRef)
	}
	if reportServer.backupDataName != "config" {
		t.Fatalf("expected backup data name config, got %q", reportServer.backupDataName)
	}
	if reportServer.backupStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded backup status, got %q", reportServer.backupStatus)
	}
}

func TestExecuteBackupTaskStopsComposeForTarAfterStop(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerLogFile := filepath.Join(rootDir, "docker.log")
	rusticPath := filepath.Join(binDir, "rustic")
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\n"
	rusticScript := "#!/bin/sh\ntest -f \"$6/config.tar.gz\" || exit 42\nprintf 'snapshot aaa999 saved\\n'\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	if err := os.WriteFile(rusticPath, []byte(rusticScript), 0o755); err != nil {
		t.Fatalf("write fake rustic script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"repository":"local:/tmp/repo","password":"pw"},"items":[{"name":"config","strategy":"files.tar_after_stop","include":["./config"],"provider":"rustic"}]}`,
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-tar"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	logUploader := newTaskLogUploader(reportClient, "task-tar")
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-tar", Type: string(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"config"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute tar_after_stop backup task: %v", err)
	}
	dockerLog, err := os.ReadFile(dockerLogFile)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if !strings.Contains(string(dockerLog), "compose --project-name demo down") || !strings.Contains(string(dockerLog), "compose --project-name demo up -d") {
		t.Fatalf("expected compose down and up around backup, got %q", string(dockerLog))
	}
	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.backupArtifactRef != "aaa999" {
		t.Fatalf("expected tar backup artifact aaa999, got %q", reportServer.backupArtifactRef)
	}
}

func TestExecuteBackupTaskRunsPGDumpAll(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerLogFile := filepath.Join(rootDir, "docker.log")
	rusticPath := filepath.Join(binDir, "rustic")
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\nif [ \"$4\" = \"exec\" ]; then printf 'dump-sql\\n'; fi\n"
	rusticScript := "#!/bin/sh\ntest -f \"$6/db.sql\" || exit 43\nprintf 'snapshot abc888 saved\\n'\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	if err := os.WriteFile(rusticPath, []byte(rusticScript), 0o755); err != nil {
		t.Fatalf("write fake rustic script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\n",
		"demo/.composia-backup.json": `{"rustic":{"repository":"local:/tmp/repo","password":"pw"},"items":[{"name":"db","strategy":"database.pgdumpall","service":"postgres","provider":"rustic"}]}`,
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-pg"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	logUploader := newTaskLogUploader(reportClient, "task-pg")
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-pg", Type: string(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"db"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute pgdumpall backup task: %v", err)
	}
	dockerLog, err := os.ReadFile(dockerLogFile)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if !strings.Contains(string(dockerLog), "compose --project-name demo exec -T postgres pg_dumpall") {
		t.Fatalf("expected pg_dumpall compose exec, got %q", string(dockerLog))
	}
	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.backupArtifactRef != "abc888" {
		t.Fatalf("expected pg backup artifact abc888, got %q", reportServer.backupArtifactRef)
	}
}

func TestExecuteBackupTaskReportsFailedBackupItem(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	rusticPath := filepath.Join(binDir, "rustic")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	rusticScript := "#!/bin/sh\nprintf 'backup failed\\n' >&2\nexit 44\n"
	if err := os.WriteFile(rusticPath, []byte(rusticScript), 0o755); err != nil {
		t.Fatalf("write fake rustic script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"repository":"local:/tmp/repo","password":"pw"},"items":[{"name":"config","strategy":"files.copy","include":["./config"],"provider":"rustic"}]}`,
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-backup-fail"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	logUploader := newTaskLogUploader(reportClient, "task-backup-fail")
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-backup-fail", Type: string(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"config"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err == nil {
		t.Fatalf("expected backup task to fail")
	}
	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusFailed) {
		t.Fatalf("expected failed task status, got %q", reportServer.taskStatus)
	}
	if reportServer.backupStatus != string(task.StatusFailed) {
		t.Fatalf("expected failed backup status, got %q", reportServer.backupStatus)
	}
	if reportServer.backupErrorSummary == "" {
		t.Fatalf("expected backup error summary to be recorded")
	}
}

type agentExecutionTestReportServer struct {
	agentv1connect.UnimplementedAgentReportServiceHandler
	mu                 sync.Mutex
	taskStatus         string
	runtimeStatus      string
	stepStatuses       map[task.StepName]string
	confirmedSeq       uint64
	backupArtifactRef  string
	backupDataName     string
	backupStatus       string
	backupErrorSummary string
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

func (server *agentExecutionTestReportServer) ReportBackupResult(_ context.Context, req *connect.Request[agentv1.ReportBackupResultRequest]) (*connect.Response[agentv1.ReportBackupResultResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.backupArtifactRef = req.Msg.GetArtifactRef()
	server.backupDataName = req.Msg.GetDataName()
	server.backupStatus = req.Msg.GetStatus()
	server.backupErrorSummary = req.Msg.GetErrorSummary()
	return connect.NewResponse(&agentv1.ReportBackupResultResponse{}), nil
}

func (server *agentExecutionTestReportServer) ReportServiceStatus(_ context.Context, req *connect.Request[agentv1.ReportServiceStatusRequest]) (*connect.Response[agentv1.ReportServiceStatusResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.runtimeStatus = req.Msg.GetRuntimeStatus()
	return connect.NewResponse(&agentv1.ReportServiceStatusResponse{}), nil
}

var _ agentv1connect.AgentReportServiceHandler = (*agentExecutionTestReportServer)(nil)
