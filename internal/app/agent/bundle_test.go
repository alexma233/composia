package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
)

func TestDownloadServiceBundleExtractsIntoRepoDir(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{"demo/composia-meta.yaml": "name: demo\n"})
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
		}
		return "main", nil
	})))
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	result, err := downloadServiceBundle(context.Background(), client, cfg, "task-1", "")
	if err != nil {
		t.Fatalf("download service bundle: %v", err)
	}
	if result.RelativeRoot != "demo" || result.RootPath != filepath.Join(cfg.RepoDir, "demo") {
		t.Fatalf("unexpected bundle result: %+v", result)
	}

	content, err := os.ReadFile(filepath.Join(cfg.RepoDir, "demo", "composia-meta.yaml"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(content) != "name: demo\n" {
		t.Fatalf("unexpected extracted content %q", string(content))
	}
}

func TestDownloadServiceBundleReplacesExistingDirectoryAtomically(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "demo"), 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "demo", "composia-meta.yaml"), []byte("name: old\n"), 0o644); err != nil {
		t.Fatalf("write old file: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{"demo/composia-meta.yaml": "name: new\n", "demo/docker-compose.yaml": "services: {}\n"})
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
		}
		return "main", nil
	})))
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	if _, err := downloadServiceBundle(context.Background(), client, cfg, "task-1", ""); err != nil {
		t.Fatalf("download service bundle: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(cfg.RepoDir, "demo", "composia-meta.yaml"))
	if err != nil {
		t.Fatalf("read replaced file: %v", err)
	}
	if string(content) != "name: new\n" {
		t.Fatalf("unexpected replaced content %q", string(content))
	}
}

func TestDownloadServiceBundlePreservesExistingDirectoryOnInvalidArchive(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(filepath.Join(cfg.RepoDir, "demo"), 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.RepoDir, "demo", "composia-meta.yaml"), []byte("name: old\n"), 0o644); err != nil {
		t.Fatalf("write old file: %v", err)
	}

	invalidBundle := buildBundleArchive(t, map[string]string{"other/composia-meta.yaml": "name: other\n"})
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: invalidBundle}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
		}
		return "main", nil
	})))
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	if _, err := downloadServiceBundle(context.Background(), client, cfg, "task-1", ""); err == nil {
		t.Fatalf("expected invalid bundle download to fail")
	}
	content, err := os.ReadFile(filepath.Join(cfg.RepoDir, "demo", "composia-meta.yaml"))
	if err != nil {
		t.Fatalf("read preserved file: %v", err)
	}
	if string(content) != "name: old\n" {
		t.Fatalf("expected old content to remain, got %q", string(content))
	}
}

func TestDownloadServiceBundleIgnoresPAXHeaders(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	bundle := buildBundleArchiveWithPAXHeader(t, map[string]string{"demo/composia-meta.yaml": "name: demo\n"})
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
		}
		return "main", nil
	})))
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	result, err := downloadServiceBundle(context.Background(), client, cfg, "task-1", "")
	if err != nil {
		t.Fatalf("download service bundle with PAX header: %v", err)
	}
	if result.RootPath != filepath.Join(cfg.RepoDir, "demo") {
		t.Fatalf("unexpected bundle result: %+v", result)
	}
}

func TestDownloadServiceBundleSendsServiceDirOverride(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := &config.AgentConfig{RepoDir: filepath.Join(rootDir, "repo"), StateDir: filepath.Join(rootDir, "state")}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	bundle := buildBundleArchive(t, map[string]string{"bravo/composia-meta.yaml": "name: bravo\n"})
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewBundleServiceHandler(bundleTestServer{bundle: bundle, expectedServiceDir: "bravo", responseServiceName: "bravo", responseRelativeRoot: "bravo"}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", errString("unexpected token")
		}
		return "main", nil
	})))
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	result, err := downloadServiceBundle(context.Background(), client, cfg, "task-1", "bravo")
	if err != nil {
		t.Fatalf("download overridden service bundle: %v", err)
	}
	if result.RelativeRoot != "bravo" || result.RootPath != filepath.Join(cfg.RepoDir, "bravo") {
		t.Fatalf("unexpected overridden bundle result: %+v", result)
	}
}

type bundleTestServer struct {
	bundle               []byte
	bundlesByServiceDir  map[string]bundleTestResponse
	expectedTaskID       string
	expectedServiceDir   string
	responseServiceName  string
	responseRelativeRoot string
}

type bundleTestResponse struct {
	bundle       []byte
	serviceName  string
	relativeRoot string
}

func (server bundleTestServer) GetServiceBundle(_ context.Context, req *connect.Request[agentv1.GetServiceBundleRequest], stream *connect.ServerStream[agentv1.GetServiceBundleResponse]) error {
	expectedTaskID := server.expectedTaskID
	if expectedTaskID == "" {
		expectedTaskID = "task-1"
	}
	if req.Msg.GetTaskId() != expectedTaskID {
		return errString("unexpected task id")
	}
	if server.expectedServiceDir != "" && req.Msg.GetServiceDir() != server.expectedServiceDir {
		return errString("unexpected service dir")
	}
	bundle := server.bundle
	serviceName := server.responseServiceName
	relativeRoot := server.responseRelativeRoot
	if len(server.bundlesByServiceDir) > 0 {
		response, ok := server.bundlesByServiceDir[req.Msg.GetServiceDir()]
		if !ok {
			return errString("unexpected service dir")
		}
		bundle = response.bundle
		serviceName = response.serviceName
		relativeRoot = response.relativeRoot
	}
	if serviceName == "" {
		serviceName = "demo"
	}
	if relativeRoot == "" {
		relativeRoot = "demo"
	}
	firstChunk := &agentv1.GetServiceBundleResponse{ServiceName: serviceName, RepoRevision: "deadbeef", RelativeRoot: relativeRoot, Data: bundle[:len(bundle)/2]}
	secondChunk := &agentv1.GetServiceBundleResponse{Data: bundle[len(bundle)/2:]}
	if err := stream.Send(firstChunk); err != nil {
		return err
	}
	return stream.Send(secondChunk)
}

func buildBundleArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buffer := bytes.Buffer{}
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		body := []byte(content)
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buffer.Bytes()
}

func buildBundleArchiveWithPAXHeader(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buffer := bytes.Buffer{}
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	paxHeader := &tar.Header{Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "bundle metadata"}}
	if err := tarWriter.WriteHeader(paxHeader); err != nil {
		t.Fatalf("write pax tar header: %v", err)
	}
	for name, content := range files {
		body := []byte(content)
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buffer.Bytes()
}

func TestRunComposeUpUsesProjectNameAndServiceDir(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	serviceDir := filepath.Join(rootDir, "service")
	argsFile := filepath.Join(rootDir, "args.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	if err := runComposeUp(context.Background(), serviceDir, "demo-project", func(string) error { return nil }); err != nil {
		t.Fatalf("run compose up: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if string(argsContent) != "compose --project-name demo-project up -d " {
		t.Fatalf("unexpected docker args %q", string(argsContent))
	}
	pwdContent, err := os.ReadFile(pwdFile)
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytes.TrimSpace(pwdContent)) != serviceDir {
		t.Fatalf("expected docker cwd %q, got %q", serviceDir, string(bytes.TrimSpace(pwdContent)))
	}
}

func TestRunComposeDownUsesProjectNameAndServiceDir(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	serviceDir := filepath.Join(rootDir, "service")
	argsFile := filepath.Join(rootDir, "args.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	if err := runComposeDown(context.Background(), serviceDir, "demo-project", func(string) error { return nil }); err != nil {
		t.Fatalf("run compose down: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if string(argsContent) != "compose --project-name demo-project down " {
		t.Fatalf("unexpected docker args %q", string(argsContent))
	}
	pwdContent, err := os.ReadFile(pwdFile)
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytes.TrimSpace(pwdContent)) != serviceDir {
		t.Fatalf("expected docker cwd %q, got %q", serviceDir, string(bytes.TrimSpace(pwdContent)))
	}
}

func TestRunComposePullUsesProjectNameAndServiceDir(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	serviceDir := filepath.Join(rootDir, "service")
	argsFile := filepath.Join(rootDir, "args.txt")
	envFile := filepath.Join(rootDir, "env.txt")
	pwdFile := filepath.Join(rootDir, "pwd.txt")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\npwd > \"$TEST_PWD_FILE\"\nprintf '%s ' \"$@\" > \"$TEST_ARGS_FILE\"\nprintf 'TERM=%s\\nCLICOLOR_FORCE=%s\\nFORCE_COLOR=%s\\nCOMPOSE_ANSI=%s\\nCOMPOSE_STATUS_STDOUT=%s\\nCOMPOSE_PROGRESS=%s\\n' \"$TERM\" \"$CLICOLOR_FORCE\" \"$FORCE_COLOR\" \"$COMPOSE_ANSI\" \"$COMPOSE_STATUS_STDOUT\" \"$COMPOSE_PROGRESS\" > \"$TEST_ENV_FILE\"\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS_FILE", argsFile)
	t.Setenv("TEST_ENV_FILE", envFile)
	t.Setenv("TEST_PWD_FILE", pwdFile)

	if err := runComposePull(context.Background(), serviceDir, "demo-project", func(string) error { return nil }); err != nil {
		t.Fatalf("run compose pull: %v", err)
	}

	argsContent, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	if string(argsContent) != "compose --project-name demo-project pull " {
		t.Fatalf("unexpected docker args %q", string(argsContent))
	}
	pwdContent, err := os.ReadFile(pwdFile)
	if err != nil {
		t.Fatalf("read pwd file: %v", err)
	}
	if string(bytes.TrimSpace(pwdContent)) != serviceDir {
		t.Fatalf("expected docker cwd %q, got %q", serviceDir, string(bytes.TrimSpace(pwdContent)))
	}
	envContent, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	for _, expected := range []string{
		"TERM=xterm-256color",
		"CLICOLOR_FORCE=1",
		"FORCE_COLOR=1",
		"COMPOSE_ANSI=always",
		"COMPOSE_STATUS_STDOUT=1",
		"COMPOSE_PROGRESS=tty",
	} {
		if !strings.Contains(string(envContent), expected) {
			t.Fatalf("expected %q in env, got %q", expected, string(envContent))
		}
	}
}

func TestRunComposeUpStreamsLogsBeforeCommandExit(t *testing.T) {
	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	serviceDir := filepath.Join(rootDir, "service")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}

	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/bin/sh\nprintf 'starting compose up\\n'\nsleep 0.3\nprintf 'compose up finished\\n' >&2\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	logsCh := make(chan string, 8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runComposeUp(context.Background(), serviceDir, "demo-project", func(output string) error {
			logsCh <- output
			return nil
		})
	}()

	select {
	case output := <-logsCh:
		if !strings.Contains(output, "starting compose up") {
			t.Fatalf("expected first streamed chunk, got %q", output)
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("expected task logs before command exit")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("run compose up: %v", err)
	}
}

func TestLoadComposeProjectNameNormalizesFallbackServiceName(t *testing.T) {
	rootDir := t.TempDir()
	serviceDir := filepath.Join(rootDir, "service")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "composia-meta.yaml"), []byte("name: Renovate\nnodes:\n  - main\n"), 0o644); err != nil {
		t.Fatalf("write service meta: %v", err)
	}

	projectName, err := loadComposeProjectName(serviceDir, "Renovate")
	if err != nil {
		t.Fatalf("load compose project name: %v", err)
	}
	if projectName != "renovate" {
		t.Fatalf("expected normalized project name, got %q", projectName)
	}
}

type errString string

func (value errString) Error() string {
	return string(value)
}
