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
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestRepoServiceGetRepoHeadReturnsMinimalSummary(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{cfg: &config.ControllerConfig{RepoDir: repoDir}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	if response.Msg.GetHeadRevision() == "" || response.Msg.GetBranch() == "" {
		t.Fatalf("expected head revision and branch, got %+v", response.Msg)
	}
	if response.Msg.GetHasRemote() {
		t.Fatalf("expected no remote for temp repo")
	}
	if !response.Msg.GetCleanWorktree() {
		t.Fatalf("expected clean worktree")
	}
}

func TestRepoServiceListRepoFilesAndGetRepoFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\n",
		"README.md":                "hello\n",
	})

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{cfg: &config.ControllerConfig{RepoDir: repoDir}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	listResponse, err := client.ListRepoFiles(context.Background(), connect.NewRequest(&controllerv1.ListRepoFilesRequest{}))
	if err != nil {
		t.Fatalf("list repo files: %v", err)
	}
	if len(listResponse.Msg.GetEntries()) != 2 {
		t.Fatalf("expected 2 root entries, got %d", len(listResponse.Msg.GetEntries()))
	}
	if listResponse.Msg.GetEntries()[0].GetPath() != "alpha" || !listResponse.Msg.GetEntries()[0].GetIsDir() {
		t.Fatalf("unexpected first repo entry: %+v", listResponse.Msg.GetEntries()[0])
	}
	if listResponse.Msg.GetEntries()[1].GetPath() != "README.md" || listResponse.Msg.GetEntries()[1].GetSize() == 0 {
		t.Fatalf("unexpected second repo entry: %+v", listResponse.Msg.GetEntries()[1])
	}

	fileResponse, err := client.GetRepoFile(context.Background(), connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: "README.md"}))
	if err != nil {
		t.Fatalf("get repo file: %v", err)
	}
	if fileResponse.Msg.GetPath() != "README.md" || fileResponse.Msg.GetContent() != "hello\n" {
		t.Fatalf("unexpected repo file response: %+v", fileResponse.Msg)
	}
	if fileResponse.Msg.GetSize() != int64(len("hello\n")) {
		t.Fatalf("unexpected repo file size: %d", fileResponse.Msg.GetSize())
	}

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("rewrite repo file: %v", err)
	}
	getHeadResponse, err := client.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head after dirty write: %v", err)
	}
	if getHeadResponse.Msg.GetCleanWorktree() {
		t.Fatalf("expected dirty worktree after file modification")
	}
}

func TestRepoServiceListRepoCommitsReturnsPagedSummaries(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"README.md": "one\n",
	})
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatalf("rewrite README: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "second")

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{cfg: &config.ControllerConfig{RepoDir: repoDir}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	firstPage, err := client.ListRepoCommits(context.Background(), connect.NewRequest(&controllerv1.ListRepoCommitsRequest{PageSize: 1}))
	if err != nil {
		t.Fatalf("list first commit page: %v", err)
	}
	if len(firstPage.Msg.GetCommits()) != 1 || firstPage.Msg.GetCommits()[0].GetSubject() != "second" {
		t.Fatalf("unexpected first commit page: %+v", firstPage.Msg.GetCommits())
	}
	if firstPage.Msg.GetCommits()[0].GetCommittedAt() == "" {
		t.Fatalf("expected committed_at on first page")
	}
	if firstPage.Msg.GetNextCursor() == "" {
		t.Fatalf("expected next cursor on first page")
	}

	secondPage, err := client.ListRepoCommits(context.Background(), connect.NewRequest(&controllerv1.ListRepoCommitsRequest{PageSize: 1, Cursor: firstPage.Msg.GetNextCursor()}))
	if err != nil {
		t.Fatalf("list second commit page: %v", err)
	}
	if len(secondPage.Msg.GetCommits()) != 1 || secondPage.Msg.GetCommits()[0].GetSubject() != "initial" {
		t.Fatalf("unexpected second commit page: %+v", secondPage.Msg.GetCommits())
	}
}

func TestRepoServiceUpdateRepoFileCommitsAndKeepsWorktreeClean(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"README.md": "hello\n",
	})
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	head, err := client.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	updated, err := client.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:          "README.md",
		Content:       "updated\n",
		BaseRevision:  head.Msg.GetHeadRevision(),
		CommitMessage: "docs: update readme",
	}))
	if err != nil {
		t.Fatalf("update repo file: %v", err)
	}
	if updated.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id in response")
	}
	content, err := os.ReadFile(filepath.Join(repoDir, "README.md"))
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(content) != "updated\n" {
		t.Fatalf("unexpected updated content %q", string(content))
	}
	clean, err := repo.IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("check clean worktree: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean worktree after update")
	}
	currentRevision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}
	if currentRevision != updated.Msg.GetCommitId() {
		t.Fatalf("expected HEAD %q, got %q", updated.Msg.GetCommitId(), currentRevision)
	}
}

func TestRepoServiceUpdateRepoFileRevertsOnValidationFailure(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: &sync.Mutex{}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	head, err := client.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	_, err = client.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:         "alpha/composia-meta.yaml",
		Content:      "name: alpha\nnode: missing\n",
		BaseRevision: head.Msg.GetHeadRevision(),
	}))
	if err == nil {
		t.Fatalf("expected validation failure")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
	content, err := os.ReadFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"))
	if err != nil {
		t.Fatalf("read reverted file: %v", err)
	}
	if string(content) != "name: alpha\nnode: main\n" {
		t.Fatalf("expected original content after revert, got %q", string(content))
	}
	clean, err := repo.IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("check clean worktree: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean worktree after validation failure")
	}
}

func TestRepoServiceUpdateRepoFileRejectsServiceWithActiveTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnode: main\n",
		"README.md":                "hello\n",
	})
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-alpha", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "alpha", Status: task.StatusPending}); err != nil {
		t.Fatalf("create active task: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: &sync.Mutex{}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	head, err := client.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	_, err = client.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:         "alpha/composia-meta.yaml",
		Content:      "name: alpha\nnode: main\nenabled: true\n",
		BaseRevision: head.Msg.GetHeadRevision(),
	}))
	if err == nil {
		t.Fatalf("expected service conflict error")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
	updated, err := client.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:         "README.md",
		Content:      "safe\n",
		BaseRevision: head.Msg.GetHeadRevision(),
	}))
	if err != nil {
		t.Fatalf("expected unrelated repo write to succeed: %v", err)
	}
	if updated.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id for unrelated write")
	}
}
