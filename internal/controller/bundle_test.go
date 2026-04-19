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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = stream.Close() }()

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
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./demo.caddy\n",
		"demo/demo.caddy":         "demo.example.com { reverse_proxy 127.0.0.1:8080 }\n",
	})
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

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
	defer func() { _ = stream.Close() }()

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
		"alpha/composia-meta.yaml": "name: alpha\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./alpha.caddy\n",
		"alpha/alpha.caddy":        "alpha.example.com { reverse_proxy 127.0.0.1:8080 }\n",
		"bravo/composia-meta.yaml": "name: bravo\nnodes:\n  - main\nnetwork:\n  caddy:\n    enabled: true\n    source: ./bravo.caddy\n",
		"bravo/bravo.caddy":        "bravo.example.com { reverse_proxy 127.0.0.1:9090 }\n",
	})
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

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
	defer func() { _ = stream.Close() }()

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
	defer func() { _ = db.Close() }()
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
	defer func() { _ = stream.Close() }()

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
		"demo/composia-meta.yaml":   "name: demo\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config\nbackup:\n  data:\n    - name: config\n      provider: rustic\n",
		"backup/composia-meta.yaml": "name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: prod\n    data_protect_dir: /data-protect\n",
	})
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
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
	defer func() { _ = stream.Close() }()

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
	if !strings.Contains(payload, `"service_name":"backup"`) || !strings.Contains(payload, `"compose_service":"rustic"`) || !strings.Contains(payload, `"profile":"prod"`) || !strings.Contains(payload, `"data_protect_dir":"/data-protect"`) || !strings.Contains(payload, `"node_id":"main"`) || !strings.Contains(payload, `"strategy":"files.copy"`) || !strings.Contains(payload, `"composia-service:demo"`) {
		t.Fatalf("unexpected backup runtime payload %q", payload)
	}
}

func TestBundleServiceServiceOverrideSkipsBackupRuntimePayload(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "demo", "main")
	createGitRepoWithService(t, repoDir, "backup", "main")
	secretsCfg := writeAgeTestConfig(t, rootDir)
	ciphertext, err := secretutil.Encrypt("RUSTIC_PASSWORD=secret\n", secretsCfg)
	if err != nil {
		t.Fatalf("encrypt rustic secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "backup", ".secret.env.enc"), ciphertext, 0o644); err != nil {
		t.Fatalf("write encrypted rustic secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "demo", "composia-meta.yaml"), []byte("name: demo\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config\nbackup:\n  data:\n    - name: config\n      provider: rustic\n"), 0o644); err != nil {
		t.Fatalf("write demo meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add rustic secret")
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
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
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-backup-override", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", RepoRevision: revision, ParamsJSON: string(paramsJSON), CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create backup task: %v", err)
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
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: "task-backup-override", ServiceDir: "backup"}))
	if err != nil {
		t.Fatalf("get overridden backup bundle: %v", err)
	}
	defer func() { _ = stream.Close() }()

	archive := bytes.Buffer{}
	for stream.Receive() {
		archive.Write(stream.Msg().GetData())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("receive overridden backup bundle: %v", err)
	}
	entries := untarGzContents(t, archive.Bytes())
	if entries["backup/.secret.env"] != "RUSTIC_PASSWORD=secret\n" {
		t.Fatalf("expected decrypted rustic secret in bundle, got %q", entries["backup/.secret.env"])
	}
	if _, ok := entries["backup/.composia-backup.json"]; ok {
		t.Fatalf("did not expect backup runtime payload in override bundle: %+v", entries)
	}
}

func TestBundleServiceInjectsBackupRuntimeConfigFromTaskRevision(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml":   "name: demo\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config-v1\nbackup:\n  data:\n    - name: config\n      provider: rustic\n",
		"backup/composia-meta.yaml": "name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic-v1\n    profile: prod-v1\n    data_protect_dir: /data-protect-v1\n",
	})
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "demo", "composia-meta.yaml"), []byte("name: demo\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config-v2\nbackup:\n  data:\n    - name: config\n      provider: rustic\n"), 0o644); err != nil {
		t.Fatalf("update demo meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "backup", "composia-meta.yaml"), []byte("name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic-v2\n    profile: prod-v2\n    data_protect_dir: /data-protect-v2\n"), 0o644); err != nil {
		t.Fatalf("update backup meta: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "update runtime config")

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
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
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-backup-revision-bundle", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", RepoRevision: revision, ParamsJSON: string(paramsJSON), CreatedAt: time.Date(2026, 4, 4, 18, 0, 0, 0, time.UTC)}); err != nil {
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
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: "task-backup-revision-bundle"}))
	if err != nil {
		t.Fatalf("get backup bundle: %v", err)
	}
	defer func() { _ = stream.Close() }()

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
	if !strings.Contains(payload, `"compose_service":"rustic-v1"`) || !strings.Contains(payload, `"profile":"prod-v1"`) || !strings.Contains(payload, `"data_protect_dir":"/data-protect-v1"`) || !strings.Contains(payload, `"include":["./config-v1"]`) {
		t.Fatalf("expected task revision runtime payload, got %q", payload)
	}
	if strings.Contains(payload, `rustic-v2`) || strings.Contains(payload, `prod-v2`) || strings.Contains(payload, `config-v2`) {
		t.Fatalf("runtime payload leaked live HEAD state: %q", payload)
	}
}

func untarGzEntries(t *testing.T, content []byte) map[string]bool {
	t.Helper()
	entries := map[string]bool{}
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open gzip content: %v", err)
	}
	defer func() { _ = gzipReader.Close() }()

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
	defer func() { _ = gzipReader.Close() }()

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
