package controller

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	secretutil "forgejo.alexma.top/alexma233/composia/internal/platform/secret"
)

func TestServiceManifestAuthorizationAndRevision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	createGitRepoWithService(t, repoDir, "demo", "main")
	secrets := writeAgeTestConfig(t, root)
	ciphertext, err := secretutil.Encrypt("PASSWORD=secret\n", secrets)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "demo", ".env.enc"), ciphertext, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "secret")
	revision := currentRevision(t, repoDir)
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := t.Context()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "other"}); err != nil {
		t.Fatal(err)
	}
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatal(err)
	}
	for _, record := range []task.Record{
		{TaskID: "check", Type: task.TypeImageCheck, Status: task.StatusRunning},
		{TaskID: "finished", Type: task.TypeImageCheck, Status: task.StatusSucceeded},
	} {
		record.ServiceName, record.NodeID, record.RepoRevision = "demo", "main", revision
		record.ParamsJSON = `{"service_dir":"demo"}`
		record.CreatedAt = time.Now().UTC()
		record.Source = task.SourceCLI
		if _, err := db.CreateTask(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	path, handler := agentv1connect.NewBundleServiceHandler(&bundleServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, Secrets: secrets, Nodes: []config.NodeConfig{{ID: "main"}, {ID: "other"}}}}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) { return token, nil })))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()
	client := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main")))
	request := func(taskID string) (*connect.Response[agentv1.GetServiceManifestResponse], error) {
		return client.GetServiceManifest(ctx, connect.NewRequest(&agentv1.GetServiceManifestRequest{TaskId: taskID}))
	}
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: "check"}))
	if err == nil {
		for stream.Receive() {
		}
		err = stream.Err()
		_ = stream.Close()
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("legacy image check could download a replacement bundle: %v", err)
	}
	response, err := request("check")
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetRepoRevision() != revision || response.Msg.GetRelativeRoot() != "demo" {
		t.Fatalf("unexpected response: %+v", response.Msg)
	}
	found := false
	for _, file := range response.Msg.GetFiles() {
		if file.GetPath() == ".env.enc" {
			t.Fatal("manifest included encrypted source instead of deployed plaintext")
		}
		if file.GetPath() == ".env" {
			found = file.GetMode() == 0o600 && file.GetSha256() == fmt.Sprintf("%x", sha256.Sum256([]byte("PASSWORD=secret\n")))
		}
	}
	if !found {
		t.Fatal("manifest is missing the rendered secret digest")
	}
	for _, tc := range []struct {
		id   string
		code connect.Code
	}{{"finished", connect.CodeFailedPrecondition}, {"", connect.CodeInvalidArgument}} {
		if _, err := request(tc.id); connect.CodeOf(err) != tc.code {
			t.Fatalf("task %q: %v", tc.id, err)
		}
	}
	other := agentv1connect.NewBundleServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("other")))
	if _, err := other.GetServiceManifest(ctx, connect.NewRequest(&agentv1.GetServiceManifestRequest{TaskId: "check"})); err == nil {
		t.Fatal("another node could read secret hashes")
	}
	if _, err := client.GetServiceManifest(ctx, connect.NewRequest(&agentv1.GetServiceManifestRequest{TaskId: "check", ExecutionId: "stale"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("stale execution accepted: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "unrelated.txt"), []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "unrelated change")
	if _, err := request("check"); err != nil {
		t.Fatalf("unrelated revision change blocked image check: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "demo", "new.conf"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "service change")
	if _, err := request("check"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("changed service configuration accepted: %v", err)
	}
	if err := db.CompleteTask(ctx, "check", task.StatusSucceeded, time.Now().UTC(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "deploy", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", Status: task.StatusRunning, RepoRevision: currentRevision(t, repoDir), ParamsJSON: `{"service_dir":"demo"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := request("deploy"); err != nil {
		t.Fatalf("active service task could not fetch manifest: %v", err)
	}
}
