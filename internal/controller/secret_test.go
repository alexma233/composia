package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"filippo.io/age"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	secretutil "forgejo.alexma.top/alexma233/composia/internal/secret"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestSecretServiceGetAndUpdateServiceSecretEnv(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", ".secret.env.enc"), ciphertext, 0o644); err != nil {
		t.Fatalf("write encrypted secret: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add encrypted secret")

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
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
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	repoMu := &sync.Mutex{}
	path, handler := controllerv1connect.NewSecretServiceHandler(
		&secretServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}, Secrets: secretsCfg}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewSecretServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	getResp, err := client.GetSecret(ctx, connect.NewRequest(&controllerv1.GetSecretRequest{ServiceName: "alpha", FilePath: ".secret.env.enc"}))
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if getResp.Msg.GetContent() != "TOKEN=before\n" {
		t.Fatalf("unexpected decrypted content %q", getResp.Msg.GetContent())
	}
	headRevision := mustCurrentRevision(t, repoDir)
	updateResp, err := client.UpdateSecret(ctx, connect.NewRequest(&controllerv1.UpdateSecretRequest{ServiceName: "alpha", FilePath: ".secret.env.enc", Content: "TOKEN=after\n", BaseRevision: headRevision}))
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
	if _, err := os.Stat(filepath.Join(repoDir, "alpha", ".secret.env")); !os.IsNotExist(err) {
		t.Fatalf("expected no plaintext secret in repo worktree, got err=%v", err)
	}
}

func TestSecretServiceUpdateSecretWithoutRecipientFile(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", ".secret.env.enc"), ciphertext, 0o644); err != nil {
		t.Fatalf("write encrypted secret: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add encrypted secret")

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
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
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	repoMu := &sync.Mutex{}
	path, handler := controllerv1connect.NewSecretServiceHandler(
		&secretServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main"}}, Secrets: secretsCfg}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewSecretServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	updateResp, err := client.UpdateSecret(ctx, connect.NewRequest(&controllerv1.UpdateSecretRequest{ServiceName: "alpha", FilePath: ".secret.env.enc", Content: "TOKEN=after\n", BaseRevision: mustCurrentRevision(t, repoDir)}))
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

func TestSecretServiceUpdateRejectsActiveServiceTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	secretsCfg := writeAgeTestConfig(t, rootDir)
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
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
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	repoMu := &sync.Mutex{}
	path, handler := controllerv1connect.NewSecretServiceHandler(
		&secretServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}, Secrets: secretsCfg}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewSecretServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	_, err = client.UpdateSecret(ctx, connect.NewRequest(&controllerv1.UpdateSecretRequest{ServiceName: "alpha", FilePath: ".secret.env.enc", Content: "TOKEN=x\n", BaseRevision: mustCurrentRevision(t, repoDir)}))
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
	if err := os.WriteFile(recipientPath, []byte(identity.Recipient().String()+"\n"), 0o644); err != nil {
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
