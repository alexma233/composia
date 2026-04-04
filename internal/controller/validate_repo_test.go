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

func TestRepoServiceValidateRepoReturnsStructuredErrors(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"alpha/composia-meta.yaml": "name: alpha\nnode: main\nunknown_field: true\n",
		"beta/composia-meta.yaml":  "name: beta\nnode: missing\n",
	})

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{cfg: &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}}},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewRepoServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	response, err := client.ValidateRepo(context.Background(), connect.NewRequest(&controllerv1.ValidateRepoRequest{}))
	if err != nil {
		t.Fatalf("validate repo: %v", err)
	}
	if len(response.Msg.GetErrors()) != 2 {
		t.Fatalf("expected 2 validation errors, got %d: %+v", len(response.Msg.GetErrors()), response.Msg.GetErrors())
	}
	if response.Msg.GetErrors()[0].GetPath() == "" || response.Msg.GetErrors()[0].GetMessage() == "" {
		t.Fatalf("expected structured validation errors, got %+v", response.Msg.GetErrors()[0])
	}
}
