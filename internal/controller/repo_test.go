package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func TestRepoQueryServiceGetRepoHeadReturnsMinimalSummary(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithService(t, repoDir, "alpha", "main")

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoQueryServiceHandler(
		&repoQueryServer{cfg: &config.ControllerConfig{RepoDir: repoDir}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
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
	if response.Msg.GetSyncStatus() != store.RepoSyncStatusLocalOnly {
		t.Fatalf("expected local_only sync status, got %q", response.Msg.GetSyncStatus())
	}
}

func TestRepoQueryServiceListRepoFilesAndGetRepoFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\n",
		"README.md":                "hello\n",
	})

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoQueryServiceHandler(
		&repoQueryServer{cfg: &config.ControllerConfig{RepoDir: repoDir}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
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

	recursiveResponse, err := client.ListRepoFiles(context.Background(), connect.NewRequest(&controllerv1.ListRepoFilesRequest{Path: "alpha", Recursive: true}))
	if err != nil {
		t.Fatalf("list recursive repo files: %v", err)
	}
	if len(recursiveResponse.Msg.GetEntries()) != 1 {
		t.Fatalf("expected 1 recursive alpha entry, got %d", len(recursiveResponse.Msg.GetEntries()))
	}
	if recursiveResponse.Msg.GetEntries()[0].GetPath() != "alpha/composia-meta.yaml" {
		t.Fatalf("unexpected recursive repo entry: %+v", recursiveResponse.Msg.GetEntries()[0])
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

func TestRepoCommandServiceSyncRepoFastForwardsConfiguredRemote(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir, originDir, branch := createGitRepoWithBareRemote(t, rootDir, map[string]string{"README.md": "one\n"})
	upstreamDir := filepath.Join(rootDir, "upstream")
	gitClone(t, originDir, upstreamDir)
	if err := os.WriteFile(filepath.Join(upstreamDir, "README.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatalf("rewrite upstream README: %v", err)
	}
	runGit(t, upstreamDir, "add", ".")
	runGit(t, upstreamDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "upstream")
	runGit(t, upstreamDir, "push", originDir, "HEAD:refs/heads/"+branch)

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: originDir, Branch: branch}}, repoMu: &sync.Mutex{}})
	commandClient := newRepoCommandServiceClient(t, &repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: originDir, Branch: branch}}, repoMu: &sync.Mutex{}})
	response, err := commandClient.SyncRepo(context.Background(), connect.NewRequest(&controllerv1.SyncRepoRequest{}))
	if err != nil {
		t.Fatalf("sync repo: %v", err)
	}
	if response.Msg.GetSyncStatus() != store.RepoSyncStatusSynced {
		t.Fatalf("expected synced status, got %q", response.Msg.GetSyncStatus())
	}
	if response.Msg.GetLastSuccessfulPullAt() == "" {
		t.Fatalf("expected last_successful_pull_at in sync response")
	}
	content, err := os.ReadFile(filepath.Join(repoDir, "README.md"))
	if err != nil {
		t.Fatalf("read synced README: %v", err)
	}
	if string(content) != "two\n" {
		t.Fatalf("expected fast-forwarded README, got %q", string(content))
	}
	head, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head after sync: %v", err)
	}
	if !head.Msg.GetHasRemote() || head.Msg.GetSyncStatus() != store.RepoSyncStatusSynced {
		t.Fatalf("unexpected repo head after sync: %+v", head.Msg)
	}
}

func TestRepoQueryServiceListRepoCommitsReturnsPagedSummaries(t *testing.T) {
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
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoQueryServiceHandler(
		&repoQueryServer{cfg: &config.ControllerConfig{RepoDir: repoDir}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
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

func TestRepoCommandServiceUpdateRepoFileCommitsAndKeepsWorktreeClean(t *testing.T) {
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
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoCommandServiceHandler(
		&repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}})
	client := controllerv1connect.NewRepoCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	head, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
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
	if updated.Msg.GetSyncStatus() != store.RepoSyncStatusLocalOnly {
		t.Fatalf("expected local_only sync status, got %q", updated.Msg.GetSyncStatus())
	}
}

func TestRepoCommandServiceCreateRepoDirectoryCommitsPlaceholder(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{"README.md": "hello\n"})
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}})
	client := newRepoCommandServiceClient(t, &repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}})
	head, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	created, err := client.CreateRepoDirectory(context.Background(), connect.NewRequest(&controllerv1.CreateRepoDirectoryRequest{
		Path:         "alpha/config",
		BaseRevision: head.Msg.GetHeadRevision(),
	}))
	if err != nil {
		t.Fatalf("create repo directory: %v", err)
	}
	if created.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id for created directory")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "alpha", "config", ".gitkeep")); err != nil {
		t.Fatalf("expected .gitkeep placeholder: %v", err)
	}
	clean, err := repo.IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("check clean worktree: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean worktree after directory creation")
	}
}

func TestRepoCommandServiceMoveRepoPathRenamesTrackedFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{"alpha/app.env": "A=1\n"})
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}})
	client := newRepoCommandServiceClient(t, &repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}})
	head, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	moved, err := client.MoveRepoPath(context.Background(), connect.NewRequest(&controllerv1.MoveRepoPathRequest{
		SourcePath:      "alpha/app.env",
		DestinationPath: "alpha/config/app.env",
		BaseRevision:    head.Msg.GetHeadRevision(),
	}))
	if err != nil {
		t.Fatalf("move repo path: %v", err)
	}
	if moved.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id for move")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "alpha", "config", "app.env")); err != nil {
		t.Fatalf("expected moved file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "alpha", "app.env")); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be removed, got %v", err)
	}
}

func TestRepoCommandServiceDeleteRepoPathRemovesTrackedFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{"alpha/.env": "A=1\n"})
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}})
	client := newRepoCommandServiceClient(t, &repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir}, repoMu: &sync.Mutex{}})
	head, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	deleted, err := client.DeleteRepoPath(context.Background(), connect.NewRequest(&controllerv1.DeleteRepoPathRequest{
		Path:         "alpha/.env",
		BaseRevision: head.Msg.GetHeadRevision(),
	}))
	if err != nil {
		t.Fatalf("delete repo path: %v", err)
	}
	if deleted.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id for delete")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "alpha", ".env")); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, got %v", err)
	}
	clean, err := repo.IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("check clean worktree: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean worktree after delete")
	}
}

func TestRepoCommandServiceUpdateRepoFileReturnsPushFailureWithoutRollback(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir, originDir, branch := createGitRepoWithBareRemote(t, rootDir, map[string]string{"README.md": "hello\n"})
	t.Cleanup(func() {
		_ = chmodRecursive(originDir, 0o755)
	})
	if err := chmodRecursive(originDir, 0o555); err != nil {
		t.Fatalf("chmod origin read-only: %v", err)
	}

	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: originDir, Branch: branch}}, repoMu: &sync.Mutex{}})
	client := newRepoCommandServiceClient(t, &repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: originDir, Branch: branch}}, repoMu: &sync.Mutex{}})
	head, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	updated, err := client.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:         "README.md",
		Content:      "updated\n",
		BaseRevision: head.Msg.GetHeadRevision(),
	}))
	if err != nil {
		t.Fatalf("update repo file with push failure: %v", err)
	}
	if updated.Msg.GetCommitId() == "" {
		t.Fatalf("expected local commit id on push failure")
	}
	if updated.Msg.GetSyncStatus() != store.RepoSyncStatusPushFailed {
		t.Fatalf("expected push_failed sync status, got %q", updated.Msg.GetSyncStatus())
	}
	if updated.Msg.GetPushError() == "" {
		t.Fatalf("expected push error in response")
	}
	currentRevision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}
	if currentRevision != updated.Msg.GetCommitId() {
		t.Fatalf("expected HEAD to stay at local commit %q, got %q", updated.Msg.GetCommitId(), currentRevision)
	}
	clean, err := repo.IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("check clean worktree: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean worktree after push failure")
	}
	headAfter, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head after push failure: %v", err)
	}
	if headAfter.Msg.GetSyncStatus() != store.RepoSyncStatusPushFailed || headAfter.Msg.GetLastSyncError() == "" {
		t.Fatalf("unexpected repo head after push failure: %+v", headAfter.Msg)
	}
}

func TestRepoCommandServiceUpdateRepoFileAllowsInvalidMetaDraft(t *testing.T) {
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
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoCommandServiceHandler(
		&repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: &sync.Mutex{}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: &sync.Mutex{}})
	client := controllerv1connect.NewRepoCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	head, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	updated, err := client.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:         "alpha/composia-meta.yaml",
		Content:      "name: alpha\nnodes:\n  - missing\n",
		BaseRevision: head.Msg.GetHeadRevision(),
	}))
	if err != nil {
		t.Fatalf("update repo file with invalid draft: %v", err)
	}
	if updated.Msg.GetCommitId() == "" {
		t.Fatalf("expected commit id for invalid draft save")
	}
	content, err := os.ReadFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"))
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(content) != "name: alpha\nnodes:\n  - missing\n" {
		t.Fatalf("expected invalid draft content to be saved, got %q", string(content))
	}
	clean, err := repo.IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("check clean worktree: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean worktree after invalid draft save")
	}
	validationErrors := repo.ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) == 0 {
		t.Fatalf("expected explicit repo validation to report invalid meta draft")
	}
}

func TestRepoCommandServiceUpdateRepoFileRejectsServiceWithActiveTask(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnodes:\n  - main\n",
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
	path, handler := controllerv1connect.NewRepoCommandServiceHandler(
		&repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: &sync.Mutex{}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	queryClient := newRepoQueryServiceClient(t, &repoQueryServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}, availableNodeIDs: map[string]struct{}{"main": {}}, repoMu: &sync.Mutex{}})
	client := controllerv1connect.NewRepoCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
	head, err := queryClient.GetRepoHead(context.Background(), connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	_, err = client.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:         "alpha/composia-meta.yaml",
		Content:      "name: alpha\nnodes:\n  - main\nenabled: true\n",
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

func newRepoCommandServiceClient(t *testing.T, server *repoCommandServer) controllerv1connect.RepoCommandServiceClient {
	t.Helper()
	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoCommandServiceHandler(server, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	t.Cleanup(httpServer.Close)
	return controllerv1connect.NewRepoCommandServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
}

func newRepoQueryServiceClient(t *testing.T, server *repoQueryServer) controllerv1connect.RepoQueryServiceClient {
	t.Helper()
	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoQueryServiceHandler(server, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	t.Cleanup(httpServer.Close)
	return controllerv1connect.NewRepoQueryServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")))
}

func createGitRepoWithBareRemote(t *testing.T, rootDir string, files map[string]string) (string, string, string) {
	t.Helper()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, files)
	originDir := filepath.Join(rootDir, "origin.git")
	if err := os.MkdirAll(originDir, 0o755); err != nil {
		t.Fatalf("create origin dir: %v", err)
	}
	runGit(t, originDir, "init", "--bare")
	branch, err := repo.CurrentBranch(repoDir)
	if err != nil {
		t.Fatalf("read current branch: %v", err)
	}
	runGit(t, repoDir, "push", originDir, "HEAD:refs/heads/"+branch)
	return repoDir, originDir, branch
}

func gitClone(t *testing.T, sourceDir, cloneDir string) {
	t.Helper()
	output, err := exec.Command("git", "clone", sourceDir, cloneDir).CombinedOutput()
	if err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, string(output))
	}
}

func chmodRecursive(root string, mode os.FileMode) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chmod(path, mode)
	})
}
