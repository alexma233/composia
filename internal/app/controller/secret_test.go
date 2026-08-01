package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"filippo.io/age"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	secretutil "forgejo.alexma.top/alexma233/composia/internal/platform/secret"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestRepoServicesDecryptAndEncryptFiles(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	secretsCfg := writeAgeTestConfig(t, rootDir)
	ciphertext, err := secretutil.Encrypt("TOKEN=before\n", secretsCfg)
	if err != nil {
		t.Fatalf("encrypt initial secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", ".secret.env.enc"), ciphertext, 0o600); err != nil {
		t.Fatalf("write encrypted secret: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add encrypted secret")

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != testAccessToken {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	repoMu := &sync.Mutex{}
	cfg := &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}, Secrets: secretsCfg}
	queryPath, queryHandler := controllerv1connect.NewRepoQueryServiceHandler(&repoQueryServer{db: db, cfg: cfg, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: repoMu}, connect.WithInterceptors(interceptor))
	commandPath, commandHandler := controllerv1connect.NewRepoCommandServiceHandler(&repoCommandServer{db: db, cfg: cfg, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: repoMu}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(queryPath, queryHandler)
	mux.Handle(commandPath, commandHandler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	queryClient := controllerv1connect.NewRepoQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(testAccessToken)))
	commandClient := controllerv1connect.NewRepoCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(testAccessToken)))
	getResp, err := queryClient.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: "alpha/.secret.env.enc"}))
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if getResp.Msg.GetContent() != "TOKEN=before\n" {
		t.Fatalf("unexpected decrypted content %q", getResp.Msg.GetContent())
	}
	if getResp.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unexpected encrypted response cache policy %q", getResp.Header().Get("Cache-Control"))
	}
	headRevision := mustCurrentRevision(t, repoDir)
	updateResp, err := commandClient.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "alpha/.secret.env.enc", Content: "TOKEN=after\n", BaseRevision: headRevision}))
	if err != nil {
		t.Fatalf("update service secret env: %v", err)
	}
	if updateResp.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id in update response")
	}
	if updateResp.Msg.GetSyncStatus() != store.RepoSyncStatusLocalOnly {
		t.Fatalf("expected local_only sync status, got %q", updateResp.Msg.GetSyncStatus())
	}
	plaintext, err := secretutil.DecryptFile(filepath.Join(repoDir, "alpha", ".secret.env.enc"), secretsCfg)
	if err != nil {
		t.Fatalf("decrypt updated secret: %v", err)
	}
	if plaintext != "TOKEN=after\n" {
		t.Fatalf("unexpected updated plaintext %q", plaintext)
	}
	stored, err := os.ReadFile(filepath.Join(repoDir, "alpha", ".secret.env.enc")) //nolint:gosec
	if err != nil || strings.Contains(string(stored), "TOKEN=after") {
		t.Fatalf("repo file contains plaintext: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "alpha", ".secret.env")); !os.IsNotExist(err) {
		t.Fatalf("expected no plaintext secret in repo worktree, got err=%v", err)
	}
}

func TestRepoServicesRequireSecretsConfigForEncryptedFiles(t *testing.T) {
	t.Parallel()

	repoDir := filepath.Join(t.TempDir(), "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{"secret.enc": "ciphertext"})
	cfg := &config.ControllerConfig{RepoDir: repoDir}
	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{cfg: cfg})
	commandClient := newRepoCommandServiceClient(t, &repoCommandServer{cfg: cfg, repoMu: &sync.Mutex{}})

	_, err := queryClient.GetRepoFile(context.Background(), connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: "secret.enc"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("encrypted read error = %v", err)
	}
	_, err = commandClient.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "secret.enc", Content: "plain", BaseRevision: mustCurrentRevision(t, repoDir)}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("encrypted write error = %v", err)
	}
	_, err = commandClient.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "secret.enc/.", Content: "plain", BaseRevision: mustCurrentRevision(t, repoDir)}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("normalized encrypted write error = %v", err)
	}
}

func TestRepoCommandEncryptsWithoutRecipientFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	logDir := filepath.Join(rootDir, "logs")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	secretsCfg := writeAgeTestConfigWithoutRecipient(t, rootDir)
	ciphertext, err := secretutil.Encrypt("TOKEN=before\n", secretsCfg)
	if err != nil {
		t.Fatalf("encrypt initial secret without recipient file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", ".secret.env.enc"), ciphertext, 0o600); err != nil {
		t.Fatalf("write encrypted secret: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add encrypted secret")

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != testAccessToken {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	repoMu := &sync.Mutex{}
	path, handler := controllerv1connect.NewRepoCommandServiceHandler(
		&repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}, Secrets: secretsCfg}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(testAccessToken)))
	updateResp, err := client.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "alpha/.secret.env.enc", Content: "TOKEN=after\n", BaseRevision: mustCurrentRevision(t, repoDir)}))
	if err != nil {
		t.Fatalf("update service secret env without recipient file: %v", err)
	}
	if updateResp.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id in update response")
	}
	plaintext, err := secretutil.DecryptFile(filepath.Join(repoDir, "alpha", ".secret.env.enc"), secretsCfg)
	if err != nil {
		t.Fatalf("decrypt updated secret: %v", err)
	}
	if plaintext != "TOKEN=after\n" {
		t.Fatalf("unexpected updated plaintext %q", plaintext)
	}
}

func TestRepoCommandEncryptedUpdateRejectsActiveServiceTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	secretsCfg := writeAgeTestConfig(t, rootDir)
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-alpha", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", Status: task.StatusPending}); err != nil {
		t.Fatalf("create active task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != testAccessToken {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	repoMu := &sync.Mutex{}
	path, handler := controllerv1connect.NewRepoCommandServiceHandler(
		&repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}, Secrets: secretsCfg}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(testAccessToken)))
	_, err = client.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "alpha/.secret.env.enc", Content: "TOKEN=x\n", BaseRevision: mustCurrentRevision(t, repoDir)}))
	if err == nil {
		t.Fatalf("expected active task conflict")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

func writeAgeTestConfig(t *testing.T, rootDir string) *config.ControllerSecretsConfig {
	t.Helper()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate age identity: %v", err)
	}
	identityPath := filepath.Join(rootDir, "age.key")
	recipientPath := filepath.Join(rootDir, "age.recipients")
	if err := os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatalf("write age identity: %v", err)
	}
	if err := os.WriteFile(recipientPath, []byte(identity.Recipient().String()+"\n"), 0o600); err != nil {
		t.Fatalf("write age recipient: %v", err)
	}
	armorEnabled := true
	return &config.ControllerSecretsConfig{Provider: "age", IdentityFile: identityPath, RecipientFile: recipientPath, Armor: &armorEnabled}
}

func writeAgeTestConfigWithoutRecipient(t *testing.T, rootDir string) *config.ControllerSecretsConfig {
	t.Helper()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate age identity: %v", err)
	}
	identityPath := filepath.Join(rootDir, "age.key")
	if err := os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatalf("write age identity: %v", err)
	}
	armorEnabled := true
	return &config.ControllerSecretsConfig{Provider: "age", IdentityFile: identityPath, Armor: &armorEnabled}
}

func mustCurrentRevision(t *testing.T, repoDir string) string {
	t.Helper()
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}
	return revision
}
