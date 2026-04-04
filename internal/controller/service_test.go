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
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
)

func TestServiceServiceListServices(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	if err := db.SyncDeclaredServices(context.Background(), []string{"alpha", "bravo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewServiceServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")),
	)

	response, err := client.ListServices(context.Background(), connect.NewRequest(&controllerv1.ListServicesRequest{PageSize: 1}))
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(response.Msg.GetServices()) != 1 {
		t.Fatalf("expected 1 service in paged response, got %d", len(response.Msg.GetServices()))
	}
	if response.Msg.GetServices()[0].GetName() != "alpha" {
		t.Fatalf("expected first service alpha, got %q", response.Msg.GetServices()[0].GetName())
	}
	if response.Msg.GetServices()[0].GetRuntimeStatus() != "unknown" {
		t.Fatalf("expected runtime_status unknown, got %q", response.Msg.GetServices()[0].GetRuntimeStatus())
	}
	if response.Msg.GetNextCursor() != "alpha" {
		t.Fatalf("expected next cursor alpha, got %q", response.Msg.GetNextCursor())
	}
}

type assertError string

func (value assertError) Error() string {
	return string(value)
}
