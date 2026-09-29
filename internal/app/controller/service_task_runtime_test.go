package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	backupcfg "forgejo.alexma.top/alexma233/composia/internal/core/backup"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
)

func TestLocalTaskContractsAndRelatedServiceAuthorization(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	repoDir := filepath.Join(t.TempDir(), "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml":    "name: demo\nnodes: [main]\ndata_protect:\n  data:\n    - name: data\n      backup:\n        strategy: files.copy\n        include: [./data]\n      restore:\n        strategy: files.copy\n        include: [./data]\n",
		"backup/composia-meta.yaml":  "name: backup\nnodes: [main]\ninfra:\n  rustic:\n    compose_service: rustic\n",
		"foreign/composia-meta.yaml": "name: foreign\nnodes: [main]\n",
	})
	revision := currentRevision(t, repoDir)
	cfg := &config.ControllerConfig{RepoDir: repoDir, Nodes: []config.NodeConfig{{ID: "main"}, {ID: "other"}}}
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "other"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"demo": {"main"}, "backup": {"main"}, "foreign": {"main"}}); err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal(serviceTaskParams{ServiceDir: "demo", DataNames: []string{"data"}, RestoreItems: []restoreTaskItem{{DataName: "data", ArtifactRef: "snapshot"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []task.Type{task.TypeBackup, task.TypeRestore, task.TypeCaddySync, task.TypeStop, task.TypeRestart, task.TypeRusticInit, task.TypeRusticForget, task.TypeRusticPrune, task.TypeImageCheck, task.TypeDeploy, task.TypeUpdate} {
		name, parameters := "demo", string(params)
		if kind == task.TypeCaddySync {
			name, parameters = "", `{"service_dirs":["demo","backup"],"full_rebuild":true}`
		}
		if _, err := db.CreateTask(ctx, task.Record{TaskID: string(kind), Type: kind, Source: task.SourceCLI, ServiceName: name, NodeID: "main", Status: task.StatusSucceeded, RepoRevision: revision, ParamsJSON: parameters, CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	active := ""
	executions := map[string]string{}
	activate := func(id string) {
		t.Helper()
		if active != "" {
			if err := db.TransitionTaskStatus(ctx, active, task.StatusRunning, task.StatusSucceeded, ""); err != nil {
				t.Fatal(err)
			}
		}
		if err := db.TransitionTaskStatus(ctx, id, task.StatusSucceeded, task.StatusPending, ""); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		offered, err := db.ClaimNextPendingTaskForNode(ctx, "main", now, now.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if offered.TaskID != id {
			t.Fatalf("unexpected task claimed: %s", offered.TaskID)
		}
		if err := db.AcknowledgeTaskExecution(ctx, id, offered.ExecutionID, now, now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		executions[id] = offered.ExecutionID
		active = id
	}
	mux := http.NewServeMux()
	auth := connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) { return token, nil }))
	path, handler := agentv1connect.NewBundleServiceHandler(&bundleServer{db: db, cfg: cfg}, auth)
	mux.Handle(path, handler)
	path, handler = agentv1connect.NewAgentReportServiceHandler(&agentReportServer{db: db, cfg: cfg}, auth)
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	bundles := agentv1connect.NewBundleServiceClient(server.Client(), server.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main")))
	reports := agentv1connect.NewAgentReportServiceClient(server.Client(), server.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("main")))
	for _, kind := range []task.Type{task.TypeBackup, task.TypeRestore, task.TypeCaddySync, task.TypeStop, task.TypeRestart, task.TypeRusticInit, task.TypeRusticForget, task.TypeRusticPrune, task.TypeImageCheck, task.TypeDeploy, task.TypeUpdate} {
		activate(string(kind))
		stream, err := bundles.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: string(kind), ExecutionId: executions[string(kind)]}))
		if err == nil {
			for stream.Receive() {
			}
			err = stream.Err()
			_ = stream.Close()
		}
		if kind == task.TypeDeploy || kind == task.TypeUpdate {
			if err != nil {
				t.Fatalf("deployment bundle rejected: %v", err)
			}
		} else if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("%s can install a bundle: %v", kind, err)
		}
	}
	for _, kind := range []task.Type{task.TypeBackup, task.TypeRestore} {
		activate(string(kind))
		response, err := bundles.GetServiceTaskRuntime(ctx, connect.NewRequest(&agentv1.GetServiceTaskRuntimeRequest{TaskId: string(kind), ExecutionId: executions[string(kind)]}))
		if err != nil {
			t.Fatal(err)
		}
		if response.Msg.GetRepoRevision() != revision || response.Msg.GetServiceDir() != "demo" {
			t.Fatal("runtime provenance mismatch")
		}
		var runtime backupcfg.RestoreConfig
		if err := json.Unmarshal([]byte(response.Msg.GetConfigJson()), &runtime); err != nil {
			t.Fatal(err)
		}
		if runtime.Rustic.ServiceDir != "backup" || len(runtime.Items) != 1 {
			t.Fatalf("invalid runtime config: %+v", runtime)
		}
		if kind == task.TypeRestore && runtime.Items[0].ArtifactRef != "snapshot" {
			t.Fatal("restore artifact lost")
		}
	}
	for _, req := range []*agentv1.GetServiceTaskRuntimeRequest{
		{TaskId: string(task.TypeBackup), ExecutionId: "stale"},
		{TaskId: string(task.TypeDeploy), ExecutionId: "execution-" + string(task.TypeDeploy)},
	} {
		activate(req.GetTaskId())
		if req.ExecutionId != "stale" {
			req.ExecutionId = executions[req.GetTaskId()]
		}
		if _, err := bundles.GetServiceTaskRuntime(ctx, connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("invalid runtime request: %v", err)
		}
	}
	activate(string(task.TypeBackup))
	foreign := agentv1connect.NewBundleServiceClient(server.Client(), server.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("other")))
	if _, err := foreign.GetServiceTaskRuntime(ctx, connect.NewRequest(&agentv1.GetServiceTaskRuntimeRequest{TaskId: string(task.TypeBackup), ExecutionId: executions[string(task.TypeBackup)]})); err == nil {
		t.Fatal("foreign node obtained runtime parameters")
	}
	for _, kind := range []task.Type{task.TypeBackup, task.TypeRestore, task.TypeCaddySync} {
		activate(string(kind))
		before, err := db.GetServiceInstanceSnapshot(ctx, "backup", "main")
		if err != nil {
			t.Fatal(err)
		}
		for _, dir := range []string{"demo", "backup", "foreign", "../escape"} {
			_, manifestErr := bundles.GetServiceManifest(ctx, connect.NewRequest(&agentv1.GetServiceManifestRequest{TaskId: string(kind), ExecutionId: executions[string(kind)], ServiceDir: dir}))
			_, reportErr := reports.ReportServiceConsistencyCheck(ctx, connect.NewRequest(&agentv1.ReportServiceConsistencyCheckRequest{TaskId: string(kind), ExecutionId: executions[string(kind)], ServiceDir: dir, Files: &agentv1.ServiceConsistencyOutcome{Status: "consistent"}, Compose: &agentv1.ServiceConsistencyOutcome{Status: "unknown"}}))
			if dir == "demo" || dir == "backup" {
				if manifestErr != nil || reportErr != nil {
					t.Fatalf("authorized related service rejected: %v %v", manifestErr, reportErr)
				}
			} else if connect.CodeOf(manifestErr) != connect.CodePermissionDenied || connect.CodeOf(reportErr) != connect.CodePermissionDenied {
				t.Fatalf("foreign related service accepted: %v %v", manifestErr, reportErr)
			}
		}
		after, err := db.GetServiceInstanceSnapshot(ctx, "backup", "main")
		if err != nil {
			t.Fatal(err)
		}
		if after.Consistency == nil || after.Consistency.TaskID != string(kind) || after.RuntimeStatus != before.RuntimeStatus || after.UpdatedAt != before.UpdatedAt {
			t.Fatalf("related snapshot changed runtime or lost provenance: %+v", after)
		}
	}
	activate(string(task.TypeBackup))
	if err := db.CompleteTaskExecution(ctx, string(task.TypeBackup), executions[string(task.TypeBackup)], task.StatusSucceeded, time.Now().UTC(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := bundles.GetServiceTaskRuntime(ctx, connect.NewRequest(&agentv1.GetServiceTaskRuntimeRequest{TaskId: string(task.TypeBackup), ExecutionId: executions[string(task.TypeBackup)]})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("terminal runtime request accepted: %v", err)
	}
}
