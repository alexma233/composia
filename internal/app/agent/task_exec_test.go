package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	backupcfg "forgejo.alexma.top/alexma233/composia/internal/core/backup"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestResolveComposeForceRecreateAutoDetectsServiceDirBindMount(t *testing.T) {
	rootDir := t.TempDir()
	serviceDir := filepath.Join(rootDir, "repo", "demo")
	binDir := filepath.Join(rootDir, "bin")
	argsFile := filepath.Join(rootDir, "args.txt")
	if err := os.MkdirAll(serviceDir, 0o750); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\nprintf '%s' \"$TEST_COMPOSE_CONFIG\"\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_COMPOSE_CONFIG", fmt.Sprintf(`{"services":{"web":{"volumes":[{"type":"bind","source":%q,"target":"/app/config"},{"type":"volume","source":"named","target":"/data"}]}}}`, filepath.Join(serviceDir, "config")))

	var logBuilder strings.Builder
	forceRecreate, err := resolveComposeForceRecreate(context.Background(), serviceDir, composeCommandConfig{ProjectName: "demo", Files: []string{"compose.yaml", "compose.override.yaml"}}, composeRecreateAuto, func(output string) error {
		_, writeErr := logBuilder.WriteString(output)
		return writeErr
	})
	if err != nil {
		t.Fatalf("resolve compose force recreate: %v", err)
	}
	if !forceRecreate {
		t.Fatal("expected auto mode to force recreate for service-dir bind mount")
	}
	logText := logBuilder.String()
	if !strings.Contains(logText, "detected bind mounts from replaceable service directory") || !strings.Contains(logText, "using docker compose up -d --force-recreate") {
		t.Fatalf("expected recreate warning log, got %q", logText)
	}
	argsContent, err := os.ReadFile(argsFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := string(argsContent); got != "compose --project-name demo -f compose.yaml -f compose.override.yaml config --format json " {
		t.Fatalf("unexpected docker args %q", got)
	}
}

func TestRunComposeUpWithForceRecreatePassesFlag(t *testing.T) {
	rootDir := t.TempDir()
	serviceDir := filepath.Join(rootDir, "repo", "demo")
	binDir := filepath.Join(rootDir, "bin")
	argsFile := filepath.Join(rootDir, "args.txt")
	if err := os.MkdirAll(serviceDir, 0o750); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)

	if err := runComposeUpWithOptions(context.Background(), serviceDir, composeCommandConfig{ProjectName: "demo", Files: []string{"compose.yaml", "compose.override.yaml"}}, composeUpOptions{ForceRecreate: true}, func(string) error { return nil }); err != nil {
		t.Fatalf("run compose up: %v", err)
	}
	argsContent, err := os.ReadFile(argsFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if string(argsContent) != "compose --project-name demo -f compose.yaml -f compose.override.yaml up -d --force-recreate " {
		t.Fatalf("unexpected docker args %q", string(argsContent))
	}
}

func TestExecuteDeployTaskSkipsComposeForConfigInfraService(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerLogFile := filepath.Join(rootDir, "docker.log")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\nexit 97\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o750); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(cfg.CaddyGeneratedDir(), 0o750); err != nil {
		t.Fatalf("create caddy generated dir: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{
		"host-service/composia-meta.yaml": "name: host-service\nnodes:\n  - main\ninfra:\n  config: {}\nnetwork:\n  caddy:\n    enabled: true\n    source: ./host-service.caddy\n",
		"host-service/host-service.caddy": "host.example.com { reverse_proxy 127.0.0.1:8080 }\n",
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-config", responseServiceName: "host-service", responseRelativeRoot: "host-service"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" { //nolint:goconst
			return "", errString("unexpected token")
		}
		return "main", nil //nolint:goconst
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
	logUploader := newTaskLogUploader(reportClient, "task-config")
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-config", Type: protoAgentTaskType(task.TypeDeploy), ServiceName: "host-service", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "host-service"}
	if err := executeDeployTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute infra.config deploy task: %v", err)
	}

	if _, err := os.Stat(dockerLogFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected docker command to be skipped, got stat err=%v", err)
	}
	generated, err := os.ReadFile(filepath.Join(cfg.CaddyGeneratedDir(), "host-service.caddy"))
	if err != nil {
		t.Fatalf("read generated caddy file: %v", err)
	}
	if !strings.Contains(string(generated), "reverse_proxy 127.0.0.1:8080") {
		t.Fatalf("unexpected generated caddy content %q", string(generated))
	}

	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded task status, got %q", reportServer.taskStatus)
	}
	if reportServer.runtimeStatus != store.ServiceRuntimeRunning {
		t.Fatalf("expected running runtime status, got %q", reportServer.runtimeStatus)
	}
	if reportServer.stepStatuses[task.StepComposeUp] != string(task.StatusSucceeded) || reportServer.stepStatuses[task.StepCaddySync] != string(task.StatusSucceeded) {
		t.Fatalf("unexpected step statuses %+v", reportServer.stepStatuses)
	}
}

func TestExecuteStopTaskDownloadsBundleAndRunsComposeDown(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	argsFile := filepath.Join(rootDir, "args.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n" //nolint:goconst
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil {                      //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o750); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o750); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.yaml\n  - compose.backup.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n"), 0o600); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	if err := os.MkdirAll(cfg.CaddyGeneratedDir(), 0o750); err != nil {
		t.Fatalf("create caddy generated dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CaddyGeneratedDir(), "demo.caddy"), []byte("demo"), 0o600); err != nil {
		t.Fatalf("seed generated caddy file: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":  "name: demo\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
		"demo/docker-compose.yaml": "services: {}\n",
		"demo/demo.caddy":          "demo.example.com { reverse_proxy 127.0.0.1:8080 }\n",
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
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{
		TaskId:       "task-1",
		Type:         protoAgentTaskType(task.TypeStop),
		ServiceName:  "demo",
		NodeId:       "main",
		RepoRevision: "deadbeef",
		ServiceDir:   "demo",
	}
	if err := executeStopTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute stop task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if string(argsContent) != "compose --project-name demo down " {
		t.Fatalf("unexpected docker args %q", string(argsContent))
	}
	pwdContent, err := os.ReadFile(pwdFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytesTrimSpace(pwdContent)) != filepath.Join(cfg.RepoDir, "demo") {
		t.Fatalf("expected docker cwd %q, got %q", filepath.Join(cfg.RepoDir, "demo"), string(bytesTrimSpace(pwdContent)))
	}
	if _, err := os.Stat(filepath.Join(cfg.CaddyGeneratedDir(), "demo.caddy")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected generated caddy file removed, got stat err=%v", err)
	}

	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded task status, got %q", reportServer.taskStatus)
	}
	if reportServer.runtimeStatus != store.ServiceRuntimeStopped {
		t.Fatalf("expected stopped runtime status, got %q", reportServer.runtimeStatus)
	}
	if len(reportServer.stepStatuses) == 0 || reportServer.stepStatuses[task.StepRender] != string(task.StatusSucceeded) || reportServer.stepStatuses[task.StepComposeDown] != string(task.StatusSucceeded) || reportServer.stepStatuses[task.StepCaddySync] != string(task.StatusSucceeded) {
		t.Fatalf("unexpected step statuses %+v", reportServer.stepStatuses)
	}
}

func TestExecuteBackupTaskRunsRusticAndReportsSnapshot(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerArgsFile := filepath.Join(rootDir, "docker-args.txt")
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_ARGS_FILE\"\nprintf '[INFO] snapshot abc12345 successfully saved.\\n'\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_ARGS_FILE", dockerArgsFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o750); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o750); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o600); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	serviceBundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\nnodes:\n  - main\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"service_name":"backup","service_dir":"backup","compose_service":"rustic","profile":"prod","data_protect_dir":"/data-protect","node_id":"main"},"items":[{"name":"config","strategy":"files.copy","include":["./config"],"provider":"rustic","tags":["composia-service:demo","composia-data:config","composia-node:main"]}]}`,
	})
	rusticBundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.yaml\n  - compose.backup.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n    data_protect_dir: /data-protect\n",
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{expectedTaskID: "task-backup", bundlesByServiceDir: map[string]bundleTestResponse{"": {bundle: serviceBundle, serviceName: "demo", relativeRoot: "demo"}, "backup": {bundle: rusticBundle, serviceName: "backup", relativeRoot: "backup"}}}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-backup", Type: protoAgentTaskType(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"config"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute backup task: %v", err)
	}

	argsContent, err := os.ReadFile(dockerArgsFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read docker args file: %v", err)
	}
	argsLog := string(argsContent)
	if !strings.Contains(argsLog, "compose --project-name infra-rustic -f compose.yaml -f compose.backup.yaml run --rm -v") || !strings.Contains(argsLog, "rustic -P prod backup --host main") {
		t.Fatalf("unexpected rustic args %q", string(argsContent))
	}
	if !strings.Contains(argsLog, "/data-protect/") {
		t.Fatalf("expected rustic to receive mapped data-protect path, got %q", argsLog)
	}
	if strings.Contains(argsLog, cfg.StateDir+"/") {
		t.Fatalf("expected rustic args to avoid raw state_dir path, got %q", argsLog)
	}
	if !strings.Contains(argsLog, "--tag composia-service:demo --tag composia-data:config --tag composia-node:main") {
		t.Fatalf("expected backup tags in docker args, got %q", argsLog)
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

func TestExecuteCaddySyncTaskCopiesServiceCaddyFile(t *testing.T) {
	rootDir := t.TempDir()
	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o750); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o750); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o600); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":  "name: demo\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
		"demo/docker-compose.yaml": "services: {}\n",
		"demo/demo.caddy":          "demo.example.com { reverse_proxy 127.0.0.1:8080 }\n",
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-caddy-sync"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	logUploader := newTaskLogUploader(reportClient, "task-caddy-sync")
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-caddy-sync", Type: protoAgentTaskType(task.TypeCaddySync), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo"}
	if err := executeCaddySyncTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute caddy sync task: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(cfg.CaddyGeneratedDir(), "demo.caddy"))
	if err != nil {
		t.Fatalf("read generated caddy file: %v", err)
	}
	if string(content) != "demo.example.com { reverse_proxy 127.0.0.1:8080 }\n" {
		t.Fatalf("unexpected generated caddy file %q", string(content))
	}
	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.stepStatuses[task.StepCaddySync] != string(task.StatusSucceeded) {
		t.Fatalf("expected caddy_sync step succeeded, got %+v", reportServer.stepStatuses)
	}
}

func TestExecuteCaddyTasksUseServiceDirForGeneratedFileName(t *testing.T) {
	rootDir := t.TempDir()
	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o750); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "element-web"), 0o750); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "element-web", "composia-meta.yaml"), []byte("name: element\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./site-config.caddy\n"), 0o600); err != nil {
		t.Fatalf("write service meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "element-web", "site-config.caddy"), []byte("element.alexma.top { reverse_proxy 127.0.0.1:8080 }\n"), 0o600); err != nil {
		t.Fatalf("write caddy source: %v", err)
	}
	if err := os.MkdirAll(cfg.CaddyGeneratedDir(), 0o750); err != nil {
		t.Fatalf("create caddy generated dir: %v", err)
	}
	if err := syncServiceCaddyFile(context.Background(), cfg, "element-web", filepath.Join(cfg.RepoDir, "element-web"), func(string) error { return nil }); err != nil {
		t.Fatalf("sync generated caddy file by service dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.CaddyGeneratedDir(), "element-web.caddy")); err != nil {
		t.Fatalf("expected element-web.caddy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.CaddyGeneratedDir(), "element.caddy")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected element.caddy absent, got stat err=%v", err)
	}
	if err := removeServiceCaddyFile(context.Background(), cfg, "element-web", func(string) error { return nil }); err != nil {
		t.Fatalf("remove generated caddy file by service dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.CaddyGeneratedDir(), "element-web.caddy")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected element-web.caddy removed, got stat err=%v", err)
	}
}

func TestBuildBackupVolumeFlagsServicePath(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	sourceDir := filepath.Join(serviceRoot, "volumes", "data")
	if err := os.MkdirAll(sourceDir, 0o750); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "hello.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	item := backupcfg.RuntimeItem{Name: "db", Strategy: "files.copy", Include: []string{"./volumes/data"}}
	flags, err := buildBackupVolumeFlags(serviceRoot, "/data-protect/backup-xxx", item)
	if err != nil {
		t.Fatalf("build backup volume flags: %v", err)
	}
	if len(flags) != 2 || flags[0] != "-v" || !strings.HasSuffix(flags[1], ":ro") {
		t.Fatalf("unexpected flags: %v", flags)
	}
	if !strings.Contains(flags[1], sourceDir+":/data-protect/backup-xxx/paths/volumes_data:ro") {
		t.Fatalf("expected service path bind mount, got %q", flags[1])
	}
}

func TestBuildBackupVolumeFlagsDockerVolume(t *testing.T) {
	t.Parallel()

	item := backupcfg.RuntimeItem{Name: "db", Strategy: "files.copy", Include: []string{"caddy_caddy_data"}}
	flags, err := buildBackupVolumeFlags(t.TempDir(), "/data-protect/backup-xxx", item)
	if err != nil {
		t.Fatalf("build backup volume flags: %v", err)
	}
	if len(flags) != 2 || flags[0] != "-v" {
		t.Fatalf("unexpected flags: %v", flags)
	}
	expected := "caddy_caddy_data:/data-protect/backup-xxx/volumes/caddy_caddy_data:ro"
	if flags[1] != expected {
		t.Fatalf("expected %q, got %q", expected, flags[1])
	}
}

func TestBuildBackupVolumeFlagsRejectsPathOutsideServiceRoot(t *testing.T) {
	t.Parallel()

	item := backupcfg.RuntimeItem{Name: "db", Strategy: "files.copy", Include: []string{"../secrets"}}
	_, err := buildBackupVolumeFlags(t.TempDir(), "/data-protect/backup-xxx", item)
	if err == nil || !strings.Contains(err.Error(), "must stay within the service root") {
		t.Fatalf("expected service root validation error, got %v", err)
	}
}

func TestBuildBackupVolumeFlagsRejectsMissingServicePath(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	item := backupcfg.RuntimeItem{Name: "db", Strategy: "files.copy", Include: []string{"./missing"}}
	_, err := buildBackupVolumeFlags(serviceRoot, "/data-protect/backup-xxx", item)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected missing service path error, got %v", err)
	}
}

func TestPrepareRestoreVolumeFlagsServicePath(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	stagingDir := t.TempDir()
	targetDir := filepath.Join(serviceRoot, "volumes", "data")
	if err := os.MkdirAll(filepath.Join(targetDir, "nested"), 0o750); err != nil {
		t.Fatalf("create target dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "nested", "old.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write old file: %v", err)
	}

	item := backupcfg.RestoreItem{Name: "db", Strategy: "files.copy", Include: []string{"./volumes/data"}, ArtifactRef: "aaa999"}
	flags, err := prepareRestoreVolumeFlags(context.Background(), serviceRoot, stagingDir, "/data-protect/restore-xxx", item)
	if err != nil {
		t.Fatalf("prepare restore volume flags: %v", err)
	}
	if len(flags) != 2 || flags[0] != "-v" {
		t.Fatalf("unexpected flags: %v", flags)
	}
	expected := targetDir + ":/data-protect/restore-xxx/db/paths/volumes_data"
	if flags[1] != expected {
		t.Fatalf("expected %q, got %q", expected, flags[1])
	}
	if _, err := os.Stat(filepath.Join(targetDir, "nested", "old.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected old file removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(stagingDir, "db", "paths")); err != nil {
		t.Fatalf("expected restore staging path dirs: %v", err)
	}
}

func TestApplyRestoreItemRejectsFilesCopy(t *testing.T) {
	t.Parallel()

	item := backupcfg.RestoreItem{Name: "db", Strategy: "files.copy"}
	err := applyRestoreItem(context.Background(), t.TempDir(), t.TempDir(), item, nil)
	if err == nil {
		t.Fatal("expected error for files.copy strategy")
	}
}

func TestPrepareRestoreVolumeFlagsServiceFile(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	stagingDir := t.TempDir()
	targetPath := filepath.Join(serviceRoot, "config", "app.env")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
		t.Fatalf("create target parent: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write old file: %v", err)
	}

	item := backupcfg.RestoreItem{Name: "config", Strategy: "files.copy", Include: []string{"./config/app.env"}, ArtifactRef: "aaa999"}
	flags, err := prepareRestoreVolumeFlags(context.Background(), serviceRoot, stagingDir, "/data-protect/restore-xxx", item)
	if err != nil {
		t.Fatalf("prepare restore volume flags: %v", err)
	}
	expected := targetPath + ":/data-protect/restore-xxx/config/paths/config_app.env"
	if len(flags) != 2 || flags[1] != expected {
		t.Fatalf("expected %q, got %v", expected, flags)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("expected target file placeholder: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected target file placeholder, got directory")
	}
}

func TestRestoreRuntimeItemDirectMountsItemPath(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerLogFile := filepath.Join(rootDir, "docker.log")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\nexit 0\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)

	cfg := &config.AgentConfig{StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	serviceRoot := filepath.Join(rootDir, "repo", "demo")
	targetDir := filepath.Join(serviceRoot, "config")
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		t.Fatalf("create restore target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "stale.txt"), []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	rusticRoot := filepath.Join(rootDir, "repo", "backup")
	if err := os.MkdirAll(rusticRoot, 0o750); err != nil {
		t.Fatalf("create rustic root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rusticRoot, "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o600); err != nil {
		t.Fatalf("write rustic meta: %v", err)
	}

	item := backupcfg.RestoreItem{Name: "config", Strategy: "files.copy", Include: []string{"./config"}, ArtifactRef: "abc123"}
	rustic := &backupcfg.RusticConfig{ServiceName: "backup", ComposeService: "rustic", DataProtectDir: "/data-protect"}
	if err := restoreRuntimeItem(context.Background(), cfg, serviceRoot, rusticRoot, "task-restore", item, rustic, nil); err != nil {
		t.Fatalf("restore runtime item: %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "stale.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale target cleared, got %v", err)
	}
	dockerLog, err := os.ReadFile(dockerLogFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	logText := string(dockerLog)
	if !strings.Contains(logText, "run --rm -v ") || !strings.Contains(logText, targetDir+":/data-protect/restore-") || !strings.Contains(logText, "/config/paths/config") || !strings.Contains(logText, " rustic restore abc123 /data-protect/restore-") {
		t.Fatalf("expected direct restore bind mount under item path, got %q", logText)
	}
}

func TestExecuteBackupTaskStopsComposeForTarAfterStop(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerLogFile := filepath.Join(rootDir, "docker.log")
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\ncase \"$*\" in *\" rustic backup \"*) printf 'snapshot aaa999 saved\\n' ;; esac\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o750); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o750); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.backup.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o600); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	serviceBundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\ncompose_files:\n  - compose.yaml\nnodes:\n  - main\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"service_name":"backup","service_dir":"backup","compose_service":"rustic","data_protect_dir":"/data-protect","node_id":"main"},"items":[{"name":"config","strategy":"files.copy_after_stop","include":["./config"],"provider":"rustic"}]}`,
	})
	rusticBundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.backup.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    data_protect_dir: /data-protect\n",
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{expectedTaskID: "task-tar", bundlesByServiceDir: map[string]bundleTestResponse{"": {bundle: serviceBundle, serviceName: "demo", relativeRoot: "demo"}, "backup": {bundle: rusticBundle, serviceName: "backup", relativeRoot: "backup"}}}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-tar", Type: protoAgentTaskType(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"config"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute tar_after_stop backup task: %v", err)
	}
	dockerLog, err := os.ReadFile(dockerLogFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if !strings.Contains(string(dockerLog), "compose --project-name demo -f compose.yaml down") || !strings.Contains(string(dockerLog), "compose --project-name demo -f compose.yaml up -d") || !strings.Contains(string(dockerLog), "compose --project-name infra-rustic -f compose.backup.yaml run --rm -v") || !strings.Contains(string(dockerLog), "rustic backup --host main") {
		t.Fatalf("expected compose down and up around backup, got %q", string(dockerLog))
	}
	if !strings.Contains(string(dockerLog), "/data-protect/") {
		t.Fatalf("expected mapped data-protect path in rustic backup command, got %q", string(dockerLog))
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
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\ncase \"$*\" in *\"postgres pg_dumpall\"*) printf 'dump-sql\\n' ;; *\" rustic backup \"*) printf 'snapshot abc888 saved\\n' ;; esac\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o750); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o750); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o600); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	serviceBundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\nnodes:\n  - main\n",
		"demo/.composia-backup.json": `{"rustic":{"service_name":"backup","service_dir":"backup","compose_service":"rustic","data_protect_dir":"/data-protect","node_id":"main"},"items":[{"name":"db","strategy":"database.pgdumpall","service":"postgres","provider":"rustic"}]}`,
	})
	rusticBundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    data_protect_dir: /data-protect\n",
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{expectedTaskID: "task-pg", bundlesByServiceDir: map[string]bundleTestResponse{"": {bundle: serviceBundle, serviceName: "demo", relativeRoot: "demo"}, "backup": {bundle: rusticBundle, serviceName: "backup", relativeRoot: "backup"}}}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-pg", Type: protoAgentTaskType(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"db"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute pgdumpall backup task: %v", err)
	}
	dockerLog, err := os.ReadFile(dockerLogFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if !strings.Contains(string(dockerLog), "compose --project-name demo exec -T postgres pg_dumpall") {
		t.Fatalf("expected pg_dumpall compose exec, got %q", string(dockerLog))
	}
	if !strings.Contains(string(dockerLog), "/data-protect/") {
		t.Fatalf("expected mapped data-protect path in rustic backup command, got %q", string(dockerLog))
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
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf 'backup failed\\n' >&2\nexit 44\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o750); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	serviceBundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\nnodes:\n  - main\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"service_name":"backup","service_dir":"backup","compose_service":"rustic","data_protect_dir":"/data-protect","node_id":"main"},"items":[{"name":"config","strategy":"files.copy","include":["./config"],"provider":"rustic"}]}`,
	})
	rusticBundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    data_protect_dir: /data-protect\n",
	})
	reportServer := &agentExecutionTestReportServer{}
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{expectedTaskID: "task-backup-fail", bundlesByServiceDir: map[string]bundleTestResponse{"": {bundle: serviceBundle, serviceName: "demo", relativeRoot: "demo"}, "backup": {bundle: rusticBundle, serviceName: "backup", relativeRoot: "backup"}}}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
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
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-backup-fail", Type: protoAgentTaskType(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"config"}}
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

func TestRusticDataProtectPathMapsConfiguredContainerDir(t *testing.T) {
	t.Parallel()

	cfg := &config.AgentConfig{StateDir: "/srv/composia/state-agent"}
	rustic := &backupcfg.RusticConfig{DataProtectDir: "/data-protect"}
	localPath := "/srv/composia/state-agent/data-protect/backup-task-config-123"

	mappedPath, err := rusticDataProtectPath(localPath, cfg, rustic)
	if err != nil {
		t.Fatalf("map rustic data-protect path: %v", err)
	}
	if mappedPath != "/data-protect/backup-task-config-123" {
		t.Fatalf("expected mapped path %q, got %q", "/data-protect/backup-task-config-123", mappedPath)
	}
}

func TestRusticDataProtectPathRejectsPathsOutsideStageRoot(t *testing.T) {
	t.Parallel()

	cfg := &config.AgentConfig{StateDir: "/srv/composia/state-agent"}
	rustic := &backupcfg.RusticConfig{DataProtectDir: "/data-protect"}

	_, err := rusticDataProtectPath("/srv/composia/state-agent/other/file", cfg, rustic)
	if err == nil || !strings.Contains(err.Error(), "outside agent data-protect stage root") {
		t.Fatalf("expected outside stage root error, got %v", err)
	}
}

func TestExecuteCaddyReloadTaskRunsComposeExec(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	argsFile := filepath.Join(rootDir, "args.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n" //nolint:goconst
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil {                      //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "caddy"), 0o750); err != nil {
		t.Fatalf("create repo caddy dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "caddy", "composia-meta.yaml"), []byte("name: edge-proxy\nproject_name: infra-caddy\ncompose_files:\n  - compose.yaml\n  - compose.edge.yaml\nnodes:\n  - main\ninfra:\n  caddy:\n    compose_service: edge\n    config_dir: /etc/caddy\n"), 0o600); err != nil {
		t.Fatalf("write caddy meta: %v", err)
	}

	reportServer := &agentExecutionTestReportServer{}
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

	reportClient := agentv1connect.NewAgentReportServiceClient(reportHTTPServer.Client(), reportHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	logUploader := newTaskLogUploader(reportClient, "task-caddy-reload")
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-caddy-reload", Type: protoAgentTaskType(task.TypeCaddyReload), ServiceName: "edge-proxy", NodeId: "main", ServiceDir: "caddy"}
	if err := executeCaddyReloadTask(context.Background(), reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute caddy reload task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if string(argsContent) != "compose --project-name infra-caddy -f compose.yaml -f compose.edge.yaml exec -T edge caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile " {
		t.Fatalf("unexpected docker args %q", string(argsContent))
	}
	pwdContent, err := os.ReadFile(pwdFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytesTrimSpace(pwdContent)) != filepath.Join(cfg.RepoDir, "caddy") {
		t.Fatalf("expected docker cwd %q, got %q", filepath.Join(cfg.RepoDir, "caddy"), string(bytesTrimSpace(pwdContent)))
	}

	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded task status, got %q", reportServer.taskStatus)
	}
	if reportServer.stepStatuses[task.StepCaddyReload] != string(task.StatusSucceeded) {
		t.Fatalf("expected caddy_reload step succeeded, got %+v", reportServer.stepStatuses)
	}
}

func TestExecuteRusticForgetTaskRunsComposeRun(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	argsFile := filepath.Join(rootDir, "args.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n" //nolint:goconst
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil {                      //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o750); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.yaml\n  - compose.ops.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n"), 0o600); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.yaml\n  - compose.ops.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n",
	})
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-rustic-forget", responseServiceName: "backup", responseRelativeRoot: "backup"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
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
	logUploader := newTaskLogUploader(reportClient, "task-rustic-forget")
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-rustic-forget", Type: protoAgentTaskType(task.TypeRusticForget), ServiceName: "backup", NodeId: "main", ParamsJson: `{"service_name":"demo","data_name":"db","service_dir":"backup"}`}
	if err := executeRusticForgetTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute rustic forget task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := string(argsContent); got != "compose --project-name infra-rustic -f compose.yaml -f compose.ops.yaml run --rm rustic -P prod forget --filter-host main --filter-tags composia-service:demo --filter-tags composia-data:db " {
		t.Fatalf("unexpected docker args %q", got)
	}
	pwdContent, err := os.ReadFile(pwdFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytesTrimSpace(pwdContent)) != filepath.Join(cfg.RepoDir, "backup") {
		t.Fatalf("expected docker cwd %q, got %q", filepath.Join(cfg.RepoDir, "backup"), string(bytesTrimSpace(pwdContent)))
	}
	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded task status, got %q", reportServer.taskStatus)
	}
	if reportServer.stepStatuses[task.StepPrune] != string(task.StatusSucceeded) {
		t.Fatalf("expected prune step succeeded, got %+v", reportServer.stepStatuses)
	}
}

func TestExecuteRusticPruneTaskRunsComposeRun(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	argsFile := filepath.Join(rootDir, "args.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n" //nolint:goconst
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil {                      //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o750); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.yaml\n  - compose.ops.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n"), 0o600); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.yaml\n  - compose.ops.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n",
	})
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-rustic-prune", responseServiceName: "backup", responseRelativeRoot: "backup"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
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
	logUploader := newTaskLogUploader(reportClient, "task-rustic-prune")
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-rustic-prune", Type: protoAgentTaskType(task.TypeRusticPrune), ServiceName: "backup", NodeId: "main", ParamsJson: `{"service_dir":"backup"}`}
	if err := executeRusticPruneTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute rustic prune task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := string(argsContent); got != "compose --project-name infra-rustic -f compose.yaml -f compose.ops.yaml run --rm rustic -P prod prune " {
		t.Fatalf("unexpected docker args %q", got)
	}
	pwdContent, err := os.ReadFile(pwdFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytesTrimSpace(pwdContent)) != filepath.Join(cfg.RepoDir, "backup") {
		t.Fatalf("expected docker cwd %q, got %q", filepath.Join(cfg.RepoDir, "backup"), string(bytesTrimSpace(pwdContent)))
	}
	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded task status, got %q", reportServer.taskStatus)
	}
	if reportServer.stepStatuses[task.StepPrune] != string(task.StatusSucceeded) {
		t.Fatalf("expected prune step succeeded, got %+v", reportServer.stepStatuses)
	}
}

func TestExecuteRusticInitTaskRunsComposeRun(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	argsFile := filepath.Join(rootDir, "args.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n" //nolint:goconst
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil {                      //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o750); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\ncompose_files:\n  - compose.yaml\n  - compose.ops.yaml\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n    init_args:\n      - --set-chunker\n      - rabin\n      - --set-chunk-size\n      - 1MiB\n",
	})
	bundleMux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedTaskID: "task-rustic-init", responseServiceName: "backup", responseRelativeRoot: "backup"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
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
	logUploader := newTaskLogUploader(reportClient, "task-rustic-init")
	defer func() { _ = logUploader.Close() }()

	pulledTask := &agentv1.AgentTask{TaskId: "task-rustic-init", Type: protoAgentTaskType(task.TypeRusticInit), ServiceName: "backup", NodeId: "main", ParamsJson: `{"service_dir":"backup"}`}
	if err := executeRusticInitTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute rustic init task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := string(argsContent); got != "compose --project-name infra-rustic -f compose.yaml -f compose.ops.yaml run --rm rustic -P prod init --set-chunker rabin --set-chunk-size 1MiB " {
		t.Fatalf("unexpected docker args %q", got)
	}
	pwdContent, err := os.ReadFile(pwdFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytesTrimSpace(pwdContent)) != filepath.Join(cfg.RepoDir, "backup") {
		t.Fatalf("expected docker cwd %q, got %q", filepath.Join(cfg.RepoDir, "backup"), string(bytesTrimSpace(pwdContent)))
	}
	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusSucceeded) {
		t.Fatalf("expected succeeded task status, got %q", reportServer.taskStatus)
	}
	if reportServer.stepStatuses[task.StepInit] != string(task.StatusSucceeded) {
		t.Fatalf("expected init step succeeded, got %+v", reportServer.stepStatuses)
	}
}

func TestExecutePulledTaskWithTimeoutMarksTimedOutTaskFailed(t *testing.T) {
	t.Parallel()

	reportServer := &agentExecutionTestReportServer{blockLogUploads: true}
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

	reportClient := agentv1connect.NewAgentReportServiceClient(reportHTTPServer.Client(), reportHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	pulledTask := &agentv1.AgentTask{TaskId: "task-timeout", Type: protoAgentTaskType(task.TypePrune), NodeId: "main", ParamsJson: `{"target":"images_all"}`}
	err := executePulledTaskWithTimeout(context.Background(), nil, reportClient, &config.AgentConfig{}, pulledTask, time.Second)
	if err == nil || !strings.Contains(err.Error(), "task exceeded execution timeout") {
		t.Fatalf("expected timeout error, got %v", err)
	}

	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusFailed) {
		t.Fatalf("expected failed task status, got %q", reportServer.taskStatus)
	}
	if !strings.Contains(reportServer.taskErrorSummary, "task exceeded execution timeout") {
		t.Fatalf("expected timeout error summary, got %q", reportServer.taskErrorSummary)
	}
	if reportServer.confirmedSeq != 1 {
		t.Fatalf("expected initial task log upload attempt before timeout, got confirmed seq %d", reportServer.confirmedSeq)
	}
}

func TestExecutePulledTaskWithTimeoutMarksExecutionErrorFailed(t *testing.T) {
	t.Parallel()

	reportServer := &agentExecutionTestReportServer{wrongLogAckTaskID: true}
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

	reportClient := agentv1connect.NewAgentReportServiceClient(reportHTTPServer.Client(), reportHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	pulledTask := &agentv1.AgentTask{TaskId: "task-error", Type: protoAgentTaskType(task.TypePrune), NodeId: "main", ParamsJson: `{"target":"images_all"}`}
	err := executePulledTaskWithTimeout(context.Background(), nil, reportClient, &config.AgentConfig{}, pulledTask, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "unexpected log ack task_id") {
		t.Fatalf("expected log ack task id error, got %v", err)
	}

	reportServer.mu.Lock()
	defer reportServer.mu.Unlock()
	if reportServer.taskStatus != string(task.StatusFailed) {
		t.Fatalf("expected failed task status, got %q", reportServer.taskStatus)
	}
	if !strings.Contains(reportServer.taskErrorSummary, "unexpected log ack task_id") {
		t.Fatalf("expected execution error summary, got %q", reportServer.taskErrorSummary)
	}
}

func TestReportTaskCompletionWithTimeoutReturnsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	reportServer := &agentExecutionTestReportServer{blockTaskState: true}
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

	reportClient := agentv1connect.NewAgentReportServiceClient(reportHTTPServer.Client(), reportHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	err := reportTaskCompletionWithTimeout(context.Background(), reportClient, "task-timeout", task.StatusFailed, "boom", 50*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected task completion timeout, got %v", err)
	}
}

func TestReportTaskStepStateWithTimeoutReturnsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	reportServer := &agentExecutionTestReportServer{blockTaskStepState: true}
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

	reportClient := agentv1connect.NewAgentReportServiceClient(reportHTTPServer.Client(), reportHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	err := reportTaskStepStateWithTimeout(context.Background(), reportClient, &agentv1.ReportTaskStepStateRequest{TaskId: "task-timeout", StepName: protoAgentTaskStepName(task.StepPrune), Status: protoAgentTaskStatus(task.StatusRunning)}, 50*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected task step timeout, got %v", err)
	}
}

type agentExecutionTestReportServer struct {
	agentv1connect.UnimplementedAgentReportServiceHandler
	mu                 sync.Mutex
	taskStatus         string
	taskErrorSummary   string
	runtimeStatus      string
	stepStatuses       map[task.StepName]string
	confirmedSeq       uint64
	backupArtifactRef  string
	backupDataName     string
	backupStatus       string
	backupErrorSummary string
	blockLogUploads    bool
	blockTaskState     bool
	blockTaskStepState bool
	wrongLogAckTaskID  bool
}

func (server *agentExecutionTestReportServer) Heartbeat(context.Context, *connect.Request[agentv1.HeartbeatRequest]) (*connect.Response[agentv1.HeartbeatResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}

func (server *agentExecutionTestReportServer) ReportTaskState(ctx context.Context, req *connect.Request[agentv1.ReportTaskStateRequest]) (*connect.Response[agentv1.ReportTaskStateResponse], error) {
	if server.blockTaskState {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	server.taskStatus = agentTaskStatusText(req.Msg.GetStatus())
	server.taskErrorSummary = req.Msg.GetErrorSummary()
	return connect.NewResponse(&agentv1.ReportTaskStateResponse{}), nil
}

func (server *agentExecutionTestReportServer) ReportTaskStepState(ctx context.Context, req *connect.Request[agentv1.ReportTaskStepStateRequest]) (*connect.Response[agentv1.ReportTaskStepStateResponse], error) {
	if server.blockTaskStepState {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.stepStatuses == nil {
		server.stepStatuses = make(map[task.StepName]string)
	}
	server.stepStatuses[agentTaskStepNameToTask(req.Msg.GetStepName())] = agentTaskStatusText(req.Msg.GetStatus())
	return connect.NewResponse(&agentv1.ReportTaskStepStateResponse{}), nil
}

func (server *agentExecutionTestReportServer) UploadTaskLogs(ctx context.Context, stream *connect.BidiStream[agentv1.UploadTaskLogsRequest, agentv1.UploadTaskLogsResponse]) error {
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
		blockLogUploads := server.blockLogUploads
		wrongLogAckTaskID := server.wrongLogAckTaskID
		server.mu.Unlock()
		if blockLogUploads {
			<-ctx.Done()
			return ctx.Err()
		}
		ackTaskID := message.GetTaskId()
		if wrongLogAckTaskID {
			ackTaskID = "other-task"
		}
		if err := stream.Send(&agentv1.UploadTaskLogsResponse{TaskId: ackTaskID, LastConfirmedSeq: confirmedSeq}); err != nil {
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

func (server *agentExecutionTestReportServer) ReportServiceInstanceStatus(_ context.Context, req *connect.Request[agentv1.ReportServiceInstanceStatusRequest]) (*connect.Response[agentv1.ReportServiceInstanceStatusResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.runtimeStatus = req.Msg.GetRuntimeStatus()
	return connect.NewResponse(&agentv1.ReportServiceInstanceStatusResponse{}), nil
}

var _ agentv1connect.AgentReportServiceHandler = (*agentExecutionTestReportServer)(nil)
