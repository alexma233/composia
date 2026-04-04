package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
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
