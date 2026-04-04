package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
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
