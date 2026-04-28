package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestRegisterAccessHandlersKeepsSystemAndServiceBackupCapabilitiesConsistent(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"rustic/composia-meta.yaml": "name: rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    profile: default\n    data_protect_dir: /data-protect\n",
		"alpha/composia-meta.yaml":  "name: alpha\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: config\n      backup:\n        strategy: files.copy\n        include:\n          - ./config\nbackup:\n  data:\n    - name: config\n",
	})

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
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "rustic"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	availableNodeIDs := map[string]struct{}{"main": {}}
	mux := http.NewServeMux()
	registerAccessHandlers(
		mux,
		&config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}}},
		db,
		interceptor,
		availableNodeIDs,
		nil,
		nil,
		nil,
		nil,
		nil,
		&sync.Mutex{},
	)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	systemClient := controllerv1connect.NewSystemServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
	)
	serviceClient := controllerv1connect.NewServiceQueryServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
	)

	configResponse, err := systemClient.GetCurrentConfig(ctx, connect.NewRequest(&controllerv1.GetCurrentConfigRequest{}))
	if err != nil {
		t.Fatalf("get current config: %v", err)
	}
	if !configResponse.Msg.GetBackup().GetHasRustic() {
		t.Fatalf("expected backup.has_rustic to be true, got %+v", configResponse.Msg.GetBackup())
	}

	capabilitiesResponse, err := systemClient.GetCapabilities(ctx, connect.NewRequest(&controllerv1.GetCapabilitiesRequest{}))
	if err != nil {
		t.Fatalf("get capabilities: %v", err)
	}
	if !capabilitiesResponse.Msg.GetGlobal().GetBackup().GetEnabled() {
		t.Fatalf("expected global backup capability enabled, got %+v", capabilitiesResponse.Msg.GetGlobal().GetBackup())
	}

	workspaceResponse, err := serviceClient.GetServiceWorkspace(ctx, connect.NewRequest(&controllerv1.GetServiceWorkspaceRequest{Folder: "alpha"}))
	if err != nil {
		t.Fatalf("get service workspace: %v", err)
	}
	if !workspaceResponse.Msg.GetWorkspace().GetActions().GetBackup().GetEnabled() {
		t.Fatalf("expected alpha backup action enabled, got %+v", workspaceResponse.Msg.GetWorkspace().GetActions().GetBackup())
	}
}
