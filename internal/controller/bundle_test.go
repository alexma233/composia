package controller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
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
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	secretutil "forgejo.alexma.top/alexma233/composia/internal/secret"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestBundleServiceStreamsTaskBundle(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "demo", "main")
	if err := os.WriteFile(filepath.Join(repoDir, "demo", "docker-compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add compose")
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "demo"})
	if err != nil {
		t.Fatalf("marshal deploy task params: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-bundle", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", RepoRevision: revision, ParamsJSON: string(paramsJSON), CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})
	mux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(&bundleServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}}, connect.WithInterceptors(interceptor))
	mux.Handle(bundlePath, bundleHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: "task-bundle"}))
	if err != nil {
		t.Fatalf("get service bundle: %v", err)
	}
	defer stream.Close()

	archive := bytes.Buffer{}
	var relativeRoot string
	for stream.Receive() {
		message := stream.Msg()
		if relativeRoot == "" {
			relativeRoot = message.GetRelativeRoot()
		}
		archive.Write(message.GetData())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("receive service bundle: %v", err)
	}
	if relativeRoot != "demo" {
		t.Fatalf("expected relative root demo, got %q", relativeRoot)
	}

	entries := untarGzEntries(t, archive.Bytes())
	if !entries["demo/composia-meta.yaml"] || !entries["demo/docker-compose.yaml"] {
		t.Fatalf("missing expected bundle entries: %+v", entries)
	}
}

func TestBundleServiceStreamsCaddySyncBundleWithSingleServiceDir(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnode: main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
		"demo/demo.caddy":         "demo.example.com { reverse_proxy 127.0.0.1:8080 }\n",
	})
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "demo", ServiceDirs: []string{"demo"}})
	if err != nil {
		t.Fatalf("marshal caddy sync task params: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-caddy-bundle", Type: task.TypeCaddySync, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", RepoRevision: revision, ParamsJSON: string(paramsJSON), CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create caddy sync task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})
	mux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(&bundleServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}}, connect.WithInterceptors(interceptor))
	mux.Handle(bundlePath, bundleHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: "task-caddy-bundle"}))
	if err != nil {
		t.Fatalf("get caddy sync bundle: %v", err)
	}
	defer stream.Close()

	archive := bytes.Buffer{}
	var relativeRoot string
	for stream.Receive() {
		message := stream.Msg()
		if relativeRoot == "" {
			relativeRoot = message.GetRelativeRoot()
		}
		archive.Write(message.GetData())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("receive caddy sync bundle: %v", err)
	}
	if relativeRoot != "demo" {
		t.Fatalf("expected relative root demo, got %q", relativeRoot)
	}

	entries := untarGzContents(t, archive.Bytes())
	if entries["demo/demo.caddy"] == "" {
		t.Fatalf("expected caddy source file in bundle, got %+v", entries)
	}
}

func TestBundleServiceStreamsRequestedServiceDirOverride(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnode: main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./alpha.caddy\n",
		"alpha/alpha.caddy":        "alpha.example.com { reverse_proxy 127.0.0.1:8080 }\n",
		"bravo/composia-meta.yaml": "name: bravo\nnode: main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./bravo.caddy\n",
		"bravo/bravo.caddy":        "bravo.example.com { reverse_proxy 127.0.0.1:9090 }\n",
	})
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDirs: []string{"alpha", "bravo"}, FullRebuild: true})
	if err != nil {
		t.Fatalf("marshal caddy sync task params: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-caddy-full-bundle", Type: task.TypeCaddySync, Source: task.SourceCLI, ServiceName: "", NodeID: "main", RepoRevision: revision, ParamsJSON: string(paramsJSON), CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create caddy sync task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})
	mux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(&bundleServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}}, connect.WithInterceptors(interceptor))
	mux.Handle(bundlePath, bundleHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: "task-caddy-full-bundle", ServiceDir: "bravo"}))
	if err != nil {
		t.Fatalf("get overridden caddy sync bundle: %v", err)
	}
	defer stream.Close()

	archive := bytes.Buffer{}
	var relativeRoot string
	for stream.Receive() {
		message := stream.Msg()
		if relativeRoot == "" {
			relativeRoot = message.GetRelativeRoot()
		}
		archive.Write(message.GetData())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("receive overridden caddy sync bundle: %v", err)
	}
	if relativeRoot != "bravo" {
		t.Fatalf("expected relative root bravo, got %q", relativeRoot)
	}

	entries := untarGzContents(t, archive.Bytes())
	if entries["bravo/bravo.caddy"] == "" || entries["alpha/alpha.caddy"] != "" {
		t.Fatalf("unexpected overridden bundle entries: %+v", entries)
	}
}

func TestBundleServiceInjectsDecryptedSecretEnv(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "demo", "main")
	secretsCfg := writeAgeTestConfig(t, rootDir)
	ciphertext, err := secretutil.Encrypt("TOKEN=secret\n", secretsCfg)
	if err != nil {
		t.Fatalf("encrypt secret env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "demo", ".secret.env.enc"), ciphertext, 0o644); err != nil {
		t.Fatalf("write encrypted secret env: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add secret")
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "demo"})
	if err != nil {
		t.Fatalf("marshal deploy task params: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-secret-bundle", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", RepoRevision: revision, ParamsJSON: string(paramsJSON), CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})
	mux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(&bundleServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Secrets: secretsCfg}}, connect.WithInterceptors(interceptor))
	mux.Handle(bundlePath, bundleHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: "task-secret-bundle"}))
	if err != nil {
		t.Fatalf("get service bundle: %v", err)
	}
	defer stream.Close()

	archive := bytes.Buffer{}
	for stream.Receive() {
		archive.Write(stream.Msg().GetData())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("receive service bundle: %v", err)
	}
	entries := untarGzContents(t, archive.Bytes())
	if entries["demo/.secret.env"] != "TOKEN=secret\n" {
		t.Fatalf("expected decrypted secret env in bundle, got %q", entries["demo/.secret.env"])
	}
	if _, ok := entries["demo/.secret.env.enc"]; !ok {
		t.Fatalf("expected encrypted secret env to remain in bundle entries")
	}
}

func TestBundleServiceInjectsBackupRuntimeConfig(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml":   "name: demo\nnode: main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config\nbackup:\n  data:\n    - name: config\n      provider: rustic\n",
		"backup/composia-meta.yaml": "name: backup\nnode: main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n",
	})
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer db.Close()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo", "backup"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: "demo", DataNames: []string{"config"}})
	if err != nil {
		t.Fatalf("marshal backup task params: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-backup-bundle", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", RepoRevision: revision, ParamsJSON: string(paramsJSON), CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create backup task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "main-token" {
			return "", assertError("unexpected token")
		}
		return "main", nil
	})
	mux := http.NewServeMux()
	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(&bundleServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}}, connect.WithInterceptors(interceptor))
	mux.Handle(bundlePath, bundleHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main-token")))
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: "task-backup-bundle"}))
	if err != nil {
		t.Fatalf("get backup bundle: %v", err)
	}
	defer stream.Close()

	archive := bytes.Buffer{}
	for stream.Receive() {
		archive.Write(stream.Msg().GetData())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("receive backup bundle: %v", err)
	}
	entries := untarGzContents(t, archive.Bytes())
	payload := entries["demo/.composia-backup.json"]
	if payload == "" {
		t.Fatalf("expected backup runtime config in bundle")
	}
	if !strings.Contains(payload, `"service_name":"backup"`) || !strings.Contains(payload, `"compose_service":"rustic"`) || !strings.Contains(payload, `"profile":"prod"`) || !strings.Contains(payload, `"node_id":"main"`) || !strings.Contains(payload, `"strategy":"files.copy"`) || !strings.Contains(payload, `"composia-service:demo"`) {
		t.Fatalf("unexpected backup runtime payload %q", payload)
	}
}

func untarGzEntries(t *testing.T, content []byte) map[string]bool {
	t.Helper()
	entries := map[string]bool{}
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open gzip content: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return entries
			}
			t.Fatalf("read tar entry: %v", err)
		}
		entries[header.Name] = true
	}
}

func untarGzContents(t *testing.T, content []byte) map[string]string {
	t.Helper()
	entries := map[string]string{}
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open gzip content: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return entries
			}
			t.Fatalf("read tar entry: %v", err)
		}
		body, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read tar body: %v", err)
		}
		entries[header.Name] = string(body)
	}
}
