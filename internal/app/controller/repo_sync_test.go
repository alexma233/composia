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
	"time"

	"connectrpc.com/connect"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestLocalFirstOfflineMutationsAndSyncAfterRestart(t *testing.T) {
	t.Parallel()
	repoDir, remoteDir, branch := createGitRepoWithBareRemote(t, t.TempDir(), map[string]string{"README.md": "initial\n"})
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	cfg := &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: remoteDir, Branch: branch}}
	server := &repoCommandServer{db: db, cfg: cfg, repoMu: &sync.Mutex{}}
	if err := os.Rename(remoteDir, remoteDir+".offline"); err != nil {
		t.Fatal(err)
	}
	if _, err := server.SyncRepo(context.Background(), connect.NewRequest(&controllerv1.SyncRepoRequest{})); err == nil {
		t.Fatal("sync must report the unavailable remote")
	}
	mutations := []func(string) (*connect.Response[controllerv1.RepoWriteResult], error){
		func(base string) (*connect.Response[controllerv1.RepoWriteResult], error) {
			return server.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "README.md", Content: "offline\n", BaseRevision: base}))
		},
		func(base string) (*connect.Response[controllerv1.RepoWriteResult], error) {
			return server.CreateRepoDirectory(context.Background(), connect.NewRequest(&controllerv1.CreateRepoDirectoryRequest{Path: "config", BaseRevision: base}))
		},
		func(base string) (*connect.Response[controllerv1.RepoWriteResult], error) {
			return server.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "config/compose.yaml", Content: "services: {}\n", BaseRevision: base}))
		},
		func(base string) (*connect.Response[controllerv1.RepoWriteResult], error) {
			return server.MoveRepoPath(context.Background(), connect.NewRequest(&controllerv1.MoveRepoPathRequest{SourcePath: "config", DestinationPath: "service", BaseRevision: base}))
		},
		func(base string) (*connect.Response[controllerv1.RepoWriteResult], error) {
			return server.DeleteRepoPath(context.Background(), connect.NewRequest(&controllerv1.DeleteRepoPathRequest{Path: "service", BaseRevision: base}))
		},
	}
	for index, mutate := range mutations {
		base, err := repo.CurrentRevision(repoDir)
		if err != nil {
			t.Fatal(err)
		}
		result, err := mutate(base)
		if err != nil {
			t.Fatalf("offline mutation %d: %v", index, err)
		}
		if result.Msg.GetSyncStatus() != store.RepoSyncStatusPendingSync || result.Msg.GetCommitId() == base {
			t.Fatalf("offline mutation %d: %+v", index, result.Msg)
		}
	}
	if err := os.Rename(remoteDir+".offline", remoteDir); err != nil {
		t.Fatal(err)
	}
	server = &repoCommandServer{db: db, cfg: cfg, repoMu: &sync.Mutex{}}
	result, err := server.SyncRepo(context.Background(), connect.NewRequest(&controllerv1.SyncRepoRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	remoteHead, err := repo.CurrentRevision(remoteDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Msg.GetSyncStatus() != store.RepoSyncStatusSynced || remoteHead != result.Msg.GetHeadRevision() {
		t.Fatalf("retry did not push local commits: %+v, remote=%s", result.Msg, remoteHead)
	}
}

func TestLocalFirstDivergencePreservesLocalHistoryAndAllowsWrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repoDir, remoteDir, branch := createGitRepoWithBareRemote(t, root, map[string]string{"README.md": "initial\n"})
	upstream := filepath.Join(root, "upstream")
	gitClone(t, remoteDir, upstream)
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	server := &repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: remoteDir, Branch: branch}}, repoMu: &sync.Mutex{}}
	localCommit := updateLocalFirstTestFile(t, server, "local\n")
	if _, err := repo.WriteFile(upstream, "README.md", "remote\n"); err != nil {
		t.Fatal(err)
	}
	remoteCommit, err := repo.CommitPath(upstream, "README.md", "remote change", "Test", "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PushCurrentBranch(upstream, remoteDir, branch, "", ""); err != nil {
		t.Fatal(err)
	}
	_, err = server.SyncRepo(context.Background(), connect.NewRequest(&controllerv1.SyncRepoRequest{}))
	if err == nil || !strings.Contains(err.Error(), "manual reconciliation") {
		t.Fatalf("expected manual reconciliation, got %v", err)
	}
	head, _ := repo.CurrentRevision(repoDir)
	remoteHead, _ := repo.CurrentRevision(remoteDir)
	clean, _ := repo.IsCleanWorkingTree(repoDir)
	if head != localCommit || remoteHead != remoteCommit || !clean {
		t.Fatalf("sync changed diverged history: local=%s remote=%s clean=%v", head, remoteHead, clean)
	}
	updateLocalFirstTestFile(t, server, "still editable\n")
}

func TestLocalFirstCanQueueDeploymentFromAnUnpushedComposeCommit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"git-server/composia-meta.yaml": "name: git-server\nnodes: [main]\n",
		"git-server/compose.yaml":       "services: {}\n",
	})
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.ControllerConfig{RepoDir: repoDir, LogDir: filepath.Join(root, "logs"), Nodes: []config.NodeConfig{{ID: "main"}}, Git: &config.ControllerGitConfig{RemoteURL: filepath.Join(root, "unavailable.git"), Branch: "main"}}
	if err := os.MkdirAll(filepath.Join(cfg.LogDir, "tasks"), 0o750); err != nil {
		t.Fatal(err)
	}
	available := map[string]struct{}{"main": {}}
	repoServer := &repoCommandServer{db: db, cfg: cfg, availableNodeIDs: available, repoMu: &sync.Mutex{}}
	base, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	write, err := repoServer.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "git-server/compose.yaml", Content: "services:\n  git:\n    image: forgejo:latest\n", BaseRevision: base}))
	if err != nil {
		t.Fatal(err)
	}
	serviceServer := &serviceCommandServer{db: db, cfg: cfg, availableNodeIDs: available, repoMu: repoServer.repoMu}
	deployment, err := serviceServer.RunServiceAction(ctx, connect.NewRequest(&controllerv1.RunServiceActionRequest{ServiceName: "git-server", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}))
	if err != nil {
		t.Fatal(err)
	}
	queued := singleQueuedServiceActionTask(t, deployment.Msg)
	if queued.GetRepoRevision() != write.Msg.GetCommitId() {
		t.Fatalf("deployment revision=%s, local commit=%s", queued.GetRepoRevision(), write.Msg.GetCommitId())
	}
	manifest, err := repo.ServiceManifest(ctx, repoDir, queued.GetRepoRevision(), "git-server", nil)
	if err != nil || len(manifest) != 2 {
		t.Fatalf("cannot render unpushed service files: manifest=%+v error=%v", manifest, err)
	}
}

func TestSynchronousModeStillRejectsWritesWhenRemoteIsUnavailable(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	createGitRepoWithContent(t, repoDir, map[string]string{"README.md": "initial\n"})
	server := &repoCommandServer{cfg: &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: filepath.Join(t.TempDir(), "unavailable.git"), Branch: "main", LocalFirst: boolPtr(false)}}, repoMu: &sync.Mutex{}}
	base, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = server.UpdateRepoFile(context.Background(), connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "README.md", Content: "must not be saved\n", BaseRevision: base}))
	if err == nil {
		t.Fatal("synchronous mode must reject an unavailable remote")
	}
	head, _ := repo.CurrentRevision(repoDir)
	file, _ := repo.ReadFile(repoDir, "README.md")
	if head != base || file.Content != "initial\n" {
		t.Fatal("synchronous mode modified the repo despite failed sync")
	}
}

func TestLocalFirstSlowFetchDoesNotBlockWritesAndCanBeCanceled(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	release := make(chan struct{})
	disconnected := make(chan struct{})
	var once, disconnectOnce sync.Once
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		once.Do(func() { close(started) })
		select {
		case <-request.Context().Done():
			disconnectOnce.Do(func() { close(disconnected) })
		case <-release:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}
	}))
	defer remote.Close()
	defer close(release)
	repoDir := t.TempDir()
	createGitRepoWithContent(t, repoDir, map[string]string{"README.md": "initial\n"})
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	server := &repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: remote.URL, Branch: "main"}}, repoMu: &sync.Mutex{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := server.SyncRepo(ctx, connect.NewRequest(&controllerv1.SyncRepoRequest{}))
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("fetch did not start")
	}
	updateLocalFirstTestFile(t, server, "saved during fetch\n")
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled sync succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not stop fetch")
	}
	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation left the Git HTTP helper connected")
	}
}

func TestLocalFirstSlowPushPinsRevisionAndLeavesNewWritesPending(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repoDir, remoteDir, branch := createGitRepoWithBareRemote(t, root, map[string]string{"README.md": "initial\n"})
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	server := &repoCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: remoteDir, Branch: branch}}, repoMu: &sync.Mutex{}}
	pushedCommit := updateLocalFirstTestFile(t, server, "first\n")
	started := filepath.Join(root, "push-started")
	release := filepath.Join(root, "release-push")
	hook := "#!/bin/sh\ntouch '" + started + "'\nwhile [ ! -f '" + release + "' ]; do sleep 0.01; done\n"
	if err := os.WriteFile(filepath.Join(remoteDir, "hooks", "pre-receive"), []byte(hook), 0o700); err != nil { //nolint:gosec // The temporary Git hook must be executable to simulate a stalled push.
		t.Fatal(err)
	}
	defer func() { _ = os.WriteFile(release, nil, 0o600) }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := server.SyncRepo(ctx, connect.NewRequest(&controllerv1.SyncRepoRequest{}))
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("push did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	latestCommit := updateLocalFirstTestFile(t, server, "second\n")
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	remoteHead, _ := repo.CurrentRevision(remoteDir)
	state, _ := db.GetRepoSyncState(context.Background())
	if remoteHead != pushedCommit || state.SyncStatus != store.RepoSyncStatusPendingSync {
		t.Fatalf("new write incorrectly marked synced: remote=%s state=%+v", remoteHead, state)
	}
	if _, err := server.SyncRepo(context.Background(), connect.NewRequest(&controllerv1.SyncRepoRequest{})); err != nil {
		t.Fatal(err)
	}
	remoteHead, _ = repo.CurrentRevision(remoteDir)
	if remoteHead != latestCommit {
		t.Fatalf("retry pushed %s, want %s", remoteHead, latestCommit)
	}
}

func updateLocalFirstTestFile(t *testing.T, server *repoCommandServer, content string) string {
	t.Helper()
	base, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := server.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "README.md", Content: content, BaseRevision: base}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Msg.GetSyncStatus() != store.RepoSyncStatusPendingSync {
		t.Fatalf("expected pending sync: %+v", result.Msg)
	}
	return result.Msg.GetCommitId()
}
