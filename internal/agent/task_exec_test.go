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
	backupcfg "forgejo.alexma.top/alexma233/composia/internal/backup"
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
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n"), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	if err := os.MkdirAll(cfg.CaddyGeneratedDir(), 0o755); err != nil {
		t.Fatalf("create caddy generated dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.CaddyGeneratedDir(), "demo.caddy"), []byte("demo"), 0o644); err != nil {
		t.Fatalf("seed generated caddy file: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":  "name: demo\nnode: main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
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
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_ARGS_FILE\"\nprintf '[INFO] snapshot abc12345 successfully saved.\\n'\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_ARGS_FILE", dockerArgsFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	serviceBundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"service_name":"backup","service_dir":"backup","compose_service":"rustic","profile":"prod","data_protect_dir":"/data-protect","node_id":"main"},"items":[{"name":"config","strategy":"files.copy","include":["./config"],"provider":"rustic","tags":["composia-service:demo","composia-data:config","composia-node:main"]}]}`,
	})
	rusticBundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n    data_protect_dir: /data-protect\n",
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
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-backup", Type: string(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"config"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute backup task: %v", err)
	}

	argsContent, err := os.ReadFile(dockerArgsFile)
	if err != nil {
		t.Fatalf("read docker args file: %v", err)
	}
	argsLog := string(argsContent)
	if !strings.Contains(argsLog, "compose run --rm rustic -P prod backup --host main") {
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
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":  "name: demo\nnode: main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
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
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-caddy-sync", Type: string(task.TypeCaddySync), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo"}
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
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "element-web"), 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "element-web", "composia-meta.yaml"), []byte("name: element\nnode: main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./site-config.caddy\n"), 0o644); err != nil {
		t.Fatalf("write service meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "element-web", "site-config.caddy"), []byte("element.alexma.top { reverse_proxy 127.0.0.1:8080 }\n"), 0o644); err != nil {
		t.Fatalf("write caddy source: %v", err)
	}
	if err := os.MkdirAll(cfg.CaddyGeneratedDir(), 0o755); err != nil {
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

func TestStageIncludeRejectsPathOutsideServiceRoot(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	stagingDir := t.TempDir()
	if err := stageInclude(context.Background(), serviceRoot, stagingDir, "../secrets"); err == nil || !strings.Contains(err.Error(), "must stay within the service root") {
		t.Fatalf("expected service root validation error, got %v", err)
	}
}

func TestRestoreIncludeRejectsPathOutsideServiceRoot(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	stagingDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(stagingDir, "paths", sanitizeStagePath("../secrets")), 0o755); err != nil {
		t.Fatalf("create staged restore source: %v", err)
	}
	if err := restoreInclude(context.Background(), serviceRoot, stagingDir, "../secrets"); err == nil || !strings.Contains(err.Error(), "must stay within the service root") {
		t.Fatalf("expected service root validation error, got %v", err)
	}
}

func TestStageIncludeUsesDockerTarExportForVolume(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerLogFile := filepath.Join(rootDir, "docker.log")
	volumeSourceDir := filepath.Join(rootDir, "volume-source")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(volumeSourceDir, "nested"), 0o755); err != nil {
		t.Fatalf("create fake volume dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(volumeSourceDir, "nested", "hello.txt"), []byte("hello from volume\n"), 0o644); err != nil {
		t.Fatalf("seed fake volume file: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\nif [ \"$1\" = \"volume\" ] && [ \"$2\" = \"inspect\" ]; then\n  printf 'unexpected docker volume inspect\\n' >&2\n  exit 91\nfi\nif [ \"$1\" = \"run\" ] && [ \"$2\" = \"--rm\" ] && [ \"$3\" = \"-v\" ] && [ \"$4\" = \"caddy_caddy_data:/source:ro\" ]; then\n  tar -C \"$TEST_VOLUME_SOURCE_DIR\" -cf - .\n  exit 0\nfi\nprintf 'unexpected docker invocation: %s\\n' \"$*\" >&2\nexit 92\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)
	t.Setenv("TEST_VOLUME_SOURCE_DIR", volumeSourceDir)

	stagingDir := t.TempDir()
	if err := stageInclude(context.Background(), t.TempDir(), stagingDir, "caddy_caddy_data"); err != nil {
		t.Fatalf("stage volume include: %v", err)
	}

	stagedFile := filepath.Join(stagingDir, "volumes", sanitizeStagePath("caddy_caddy_data"), "nested", "hello.txt")
	content, err := os.ReadFile(stagedFile)
	if err != nil {
		t.Fatalf("read staged file: %v", err)
	}
	if string(content) != "hello from volume\n" {
		t.Fatalf("unexpected staged content %q", string(content))
	}

	dockerLog, err := os.ReadFile(dockerLogFile)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	logText := string(dockerLog)
	expected := "run --rm -v caddy_caddy_data:/source:ro " + dockerVolumeTarImage + " tar -C /source -cf - ."
	if !strings.Contains(logText, expected) {
		t.Fatalf("expected docker tar export command %q, got %q", expected, logText)
	}
	if strings.Contains(logText, "volume inspect") {
		t.Fatalf("expected docker volume inspect to be unused, got %q", logText)
	}
	if strings.Contains(logText, "/var/lib/docker/volumes/") {
		t.Fatalf("expected backup flow to avoid docker mountpoints, got %q", logText)
	}
}

func TestRestoreIncludeUsesDockerTarImportForVolume(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerLogFile := filepath.Join(rootDir, "docker.log")
	volumeTargetDir := filepath.Join(rootDir, "volume-target")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.MkdirAll(volumeTargetDir, 0o755); err != nil {
		t.Fatalf("create fake volume target dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(volumeTargetDir, "stale.txt"), []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("seed stale file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(volumeTargetDir, ".stale"), []byte("hidden stale\n"), 0o644); err != nil {
		t.Fatalf("seed hidden stale file: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\nif [ \"$1\" = \"volume\" ] && [ \"$2\" = \"inspect\" ]; then\n  printf 'unexpected docker volume inspect\\n' >&2\n  exit 93\nfi\nif [ \"$1\" = \"run\" ] && [ \"$2\" = \"-i\" ] && [ \"$3\" = \"--rm\" ] && [ \"$4\" = \"-v\" ] && [ \"$5\" = \"caddy_caddy_data:/target\" ]; then\n  mkdir -p \"$TEST_VOLUME_TARGET_DIR\"\n  rm -rf \"$TEST_VOLUME_TARGET_DIR\"/..?* \"$TEST_VOLUME_TARGET_DIR\"/.[!.]* \"$TEST_VOLUME_TARGET_DIR\"/*\n  tar -C \"$TEST_VOLUME_TARGET_DIR\" -xf -\n  exit 0\nfi\nprintf 'unexpected docker invocation: %s\\n' \"$*\" >&2\nexit 94\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)
	t.Setenv("TEST_VOLUME_TARGET_DIR", volumeTargetDir)

	stagingDir := t.TempDir()
	stagedVolumeDir := filepath.Join(stagingDir, "volumes", sanitizeStagePath("caddy_caddy_data"), "nested")
	if err := os.MkdirAll(stagedVolumeDir, 0o755); err != nil {
		t.Fatalf("create staged volume dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagedVolumeDir, "hello.txt"), []byte("restored from stage\n"), 0o644); err != nil {
		t.Fatalf("seed staged volume file: %v", err)
	}

	if err := restoreInclude(context.Background(), t.TempDir(), stagingDir, "caddy_caddy_data"); err != nil {
		t.Fatalf("restore volume include: %v", err)
	}

	restoredContent, err := os.ReadFile(filepath.Join(volumeTargetDir, "nested", "hello.txt"))
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(restoredContent) != "restored from stage\n" {
		t.Fatalf("unexpected restored content %q", string(restoredContent))
	}
	if _, err := os.Stat(filepath.Join(volumeTargetDir, "stale.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale file removed, got stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(volumeTargetDir, ".stale")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected hidden stale file removed, got stat err=%v", err)
	}

	dockerLog, err := os.ReadFile(dockerLogFile)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	logText := string(dockerLog)
	expected := "run -i --rm -v caddy_caddy_data:/target " + dockerVolumeTarImage + " sh -c " + dockerVolumeImportCmd
	if !strings.Contains(logText, expected) {
		t.Fatalf("expected docker tar import command %q, got %q", expected, logText)
	}
	if strings.Contains(logText, "volume inspect") {
		t.Fatalf("expected docker volume inspect to be unused, got %q", logText)
	}
	if strings.Contains(logText, "/var/lib/docker/volumes/") {
		t.Fatalf("expected restore flow to avoid docker mountpoints, got %q", logText)
	}
}

func TestStageIncludeReturnsErrorWhenDockerTarExportFailsForVolume(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nif [ \"$1\" = \"run\" ] && [ \"$2\" = \"--rm\" ] && [ \"$3\" = \"-v\" ] && [ \"$4\" = \"caddy_caddy_data:/source:ro\" ]; then\n  printf 'export failed\\n' >&2\n  exit 44\nfi\nprintf 'unexpected docker invocation: %s\\n' \"$*\" >&2\nexit 95\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := stageInclude(context.Background(), t.TempDir(), t.TempDir(), "caddy_caddy_data")
	if err == nil {
		t.Fatalf("expected stage volume export to fail")
	}
	if !strings.Contains(err.Error(), "docker run volume export failed") {
		t.Fatalf("expected docker export failure in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "export failed") {
		t.Fatalf("expected docker stderr in error, got %v", err)
	}
}

func TestRestoreIncludeReturnsErrorWhenDockerTarImportFailsForVolume(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nif [ \"$1\" = \"run\" ] && [ \"$2\" = \"-i\" ] && [ \"$3\" = \"--rm\" ] && [ \"$4\" = \"-v\" ] && [ \"$5\" = \"caddy_caddy_data:/target\" ]; then\n  printf 'import failed\\n' >&2\n  exit 45\nfi\nprintf 'unexpected docker invocation: %s\\n' \"$*\" >&2\nexit 96\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	stagingDir := t.TempDir()
	stagedVolumeDir := filepath.Join(stagingDir, "volumes", sanitizeStagePath("caddy_caddy_data"))
	if err := os.MkdirAll(stagedVolumeDir, 0o755); err != nil {
		t.Fatalf("create staged volume dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagedVolumeDir, "hello.txt"), []byte("restore me\n"), 0o644); err != nil {
		t.Fatalf("seed staged file: %v", err)
	}

	err := restoreInclude(context.Background(), t.TempDir(), stagingDir, "caddy_caddy_data")
	if err == nil {
		t.Fatalf("expected restore volume import to fail")
	}
	if !strings.Contains(err.Error(), "docker run volume import failed") {
		t.Fatalf("expected docker import failure in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "import failed") {
		t.Fatalf("expected docker stderr in error, got %v", err)
	}
}

func TestExecuteBackupTaskStopsComposeForTarAfterStop(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	dockerLogFile := filepath.Join(rootDir, "docker.log")
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\ncase \"$*\" in *\" rustic backup \"*) printf 'snapshot aaa999 saved\\n' ;; esac\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	serviceBundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"service_name":"backup","service_dir":"backup","compose_service":"rustic","data_protect_dir":"/data-protect","node_id":"main"},"items":[{"name":"config","strategy":"files.tar_after_stop","include":["./config"],"provider":"rustic"}]}`,
	})
	rusticBundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    data_protect_dir: /data-protect\n",
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
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-tar", Type: string(task.TypeBackup), ServiceName: "demo", NodeId: "main", RepoRevision: "deadbeef", ServiceDir: "demo", DataNames: []string{"config"}}
	if err := executeBackupTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute tar_after_stop backup task: %v", err)
	}
	dockerLog, err := os.ReadFile(dockerLogFile)
	if err != nil {
		t.Fatalf("read docker log: %v", err)
	}
	if !strings.Contains(string(dockerLog), "compose --project-name demo down") || !strings.Contains(string(dockerLog), "compose --project-name demo up -d") || !strings.Contains(string(dockerLog), "compose run --rm rustic backup --host main") {
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
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\ncase \"$*\" in *\"postgres pg_dumpall\"*) printf 'dump-sql\\n' ;; *\" rustic backup \"*) printf 'snapshot abc888 saved\\n' ;; esac\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", dockerLogFile)

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	serviceBundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\n",
		"demo/.composia-backup.json": `{"rustic":{"service_name":"backup","service_dir":"backup","compose_service":"rustic","data_protect_dir":"/data-protect","node_id":"main"},"items":[{"name":"db","strategy":"database.pgdumpall","service":"postgres","provider":"rustic"}]}`,
	})
	rusticBundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    data_protect_dir: /data-protect\n",
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
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerScript := "#!/bin/sh\nprintf 'backup failed\\n' >&2\nexit 44\n"
	if err := os.WriteFile(dockerPath, []byte(dockerScript), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	serviceBundle := buildBundleArchive(t, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\n",
		"demo/config/app.env":        "HELLO=world\n",
		"demo/.composia-backup.json": `{"rustic":{"service_name":"backup","service_dir":"backup","compose_service":"rustic","data_protect_dir":"/data-protect","node_id":"main"},"items":[{"name":"config","strategy":"files.copy","include":["./config"],"provider":"rustic"}]}`,
	})
	rusticBundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    data_protect_dir: /data-protect\n",
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
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "caddy"), 0o755); err != nil {
		t.Fatalf("create repo caddy dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "caddy", "composia-meta.yaml"), []byte("name: edge-proxy\nproject_name: infra-caddy\nnode: main\ninfra:\n  caddy:\n    compose_service: edge\n    config_dir: /etc/caddy\n"), 0o644); err != nil {
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
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-caddy-reload", Type: string(task.TypeCaddyReload), ServiceName: "edge-proxy", NodeId: "main", ServiceDir: "caddy"}
	if err := executeCaddyReloadTask(context.Background(), reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute caddy reload task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if string(argsContent) != "compose --project-name infra-caddy exec -T edge caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile " {
		t.Fatalf("unexpected docker args %q", string(argsContent))
	}
	pwdContent, err := os.ReadFile(pwdFile)
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
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n"), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n",
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
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-rustic-forget", Type: string(task.TypeRusticForget), ServiceName: "backup", NodeId: "main", ParamsJson: `{"service_name":"demo","data_name":"db","service_dir":"backup"}`}
	if err := executeRusticForgetTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute rustic forget task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := string(argsContent); got != "compose run --rm rustic -P prod forget --filter-host main --filter-tags composia-service:demo --filter-tags composia-data:db " {
		t.Fatalf("unexpected docker args %q", got)
	}
	pwdContent, err := os.ReadFile(pwdFile)
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
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n"), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n",
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
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-rustic-prune", Type: string(task.TypeRusticPrune), ServiceName: "backup", NodeId: "main", ParamsJson: `{"service_dir":"backup"}`}
	if err := executeRusticPruneTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute rustic prune task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := string(argsContent); got != "compose run --rm rustic -P prod prune " {
		t.Fatalf("unexpected docker args %q", got)
	}
	pwdContent, err := os.ReadFile(pwdFile)
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
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	bundle := buildBundleArchive(t, map[string]string{
		"backup/composia-meta.yaml": "name: backup\nproject_name: infra-rustic\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n    init_args:\n      - --set-chunker\n      - rabin\n      - --set-chunk-size\n      - 1MiB\n",
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
	defer logUploader.Close()

	pulledTask := &agentv1.AgentTask{TaskId: "task-rustic-init", Type: string(task.TypeRusticInit), ServiceName: "backup", NodeId: "main", ParamsJson: `{"service_dir":"backup"}`}
	if err := executeRusticInitTask(context.Background(), bundleClient, reportClient, cfg, pulledTask, logUploader); err != nil {
		t.Fatalf("execute rustic init task: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if got := string(argsContent); got != "compose run --rm rustic -P prod init --set-chunker rabin --set-chunk-size 1MiB " {
		t.Fatalf("unexpected docker args %q", got)
	}
	pwdContent, err := os.ReadFile(pwdFile)
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

func (server *agentExecutionTestReportServer) ReportServiceInstanceStatus(_ context.Context, req *connect.Request[agentv1.ReportServiceInstanceStatusRequest]) (*connect.Response[agentv1.ReportServiceInstanceStatusResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.runtimeStatus = req.Msg.GetRuntimeStatus()
	return connect.NewResponse(&agentv1.ReportServiceInstanceStatusResponse{}), nil
}

var _ agentv1connect.AgentReportServiceHandler = (*agentExecutionTestReportServer)(nil)
