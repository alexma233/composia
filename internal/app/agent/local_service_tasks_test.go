package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

type localTaskBundleServer struct {
	bundleTestServer
	t *testing.T
}

func (server localTaskBundleServer) GetServiceBundle(context.Context, *connect.Request[agentv1.GetServiceBundleRequest], *connect.ServerStream[agentv1.GetServiceBundleResponse]) error {
	server.t.Error("non-deployment task attempted to install a bundle")
	return errors.New("bundle installation forbidden")
}

func TestNonDeploymentTasksPreserveLocalFiles(t *testing.T) {
	for _, operation := range []struct {
		kind       task.Type
		checkFiles bool
		execute    func(context.Context, agentv1connect.BundleServiceClient, agentv1connect.AgentReportServiceClient, *config.AgentConfig, *agentv1.AgentTask, *taskLogUploader) error
	}{
		{task.TypeBackup, true, executeBackupTask},
		{task.TypeRestore, true, executeRestoreTask},
		{task.TypeCaddySync, true, executeCaddySyncTask},
		{task.TypeStop, false, executeStopTask},
		{task.TypeRestart, false, executeRestartTask},
		{task.TypeRusticInit, false, executeRusticInitTask},
		{task.TypeRusticForget, false, executeRusticForgetTask},
		{task.TypeRusticPrune, false, executeRusticPruneTask},
	} {
		for _, scenario := range []string{"match", "drift", "missing", "related-drift"} {
			if scenario == "related-drift" && !operation.checkFiles {
				continue
			}
			t.Run(string(operation.kind)+"/"+scenario, func(t *testing.T) {
				root := t.TempDir()
				cfg := &config.AgentConfig{RepoDir: filepath.Join(root, "repo"), StateDir: root, NodeID: "main"}
				primary := buildBundleArchive(t, map[string]string{
					"demo/composia-meta.yaml":    "name: demo\nnodes: [main]\ninfra:\n  rustic:\n    compose_service: rustic\nnetwork:\n  caddy:\n    enabled: true\n    source: Caddyfile\n",
					"demo/docker-compose.yaml":   "services: {}\n",
					"demo/settings.conf":         "managed config\n",
					"demo/Caddyfile":             "demo.local { respond ok }\n",
					"demo/.composia-backup.json": `{"rustic":{"service_dir":"backup","service_name":"backup","compose_service":"rustic","node_id":"main"},"items":[{"name":"data","strategy":"files.copy","artifact_ref":"snapshot"}]}`,
				})
				related := buildBundleArchive(t, map[string]string{
					"backup/composia-meta.yaml":  "name: backup\nnodes: [main]\ninfra:\n  rustic:\n    compose_service: rustic\n",
					"backup/docker-compose.yaml": "services: {}\n",
				})
				seedLocalBundle(t, cfg.RepoDir, primary)
				seedLocalBundle(t, cfg.RepoDir, related)
				meta := filepath.Join(cfg.RepoDir, "demo", "composia-meta.yaml")
				if scenario == "missing" {
					if err := os.Remove(meta); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "drift" {
					writeAgentTestFile(t, filepath.Join(cfg.RepoDir, "demo/settings.conf"), "local edit\n")
				}
				if scenario == "related-drift" {
					writeAgentTestFile(t, filepath.Join(cfg.RepoDir, "backup/docker-compose.yaml"), "local edit\n")
				}
				unmanaged := filepath.Join(cfg.RepoDir, "demo", "runtime-data")
				writeAgentTestFile(t, unmanaged, "must survive\n")
				before, err := os.Stat(unmanaged)
				if err != nil {
					t.Fatal(err)
				}
				dirBefore, err := os.Stat(filepath.Dir(unmanaged))
				if err != nil {
					t.Fatal(err)
				}
				generated := filepath.Join(cfg.CaddyGeneratedDir(), "previous.caddy")
				writeAgentTestFile(t, generated, "existing generated config\n")
				logFile := installFakeDockerScript(t, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\nprintf 'snapshot abc12345 saved.\\n'\n")
				bundles := localTaskBundleServer{t: t, bundleTestServer: bundleTestServer{bundlesByServiceDir: map[string]bundleTestResponse{"": {bundle: primary}, "backup": {bundle: related}}}}
				reports := &imageCheckReportServer{}
				mux := http.NewServeMux()
				path, handler := agentv1connect.NewBundleServiceHandler(bundles)
				mux.Handle(path, handler)
				path, handler = agentv1connect.NewAgentReportServiceHandler(reports)
				mux.Handle(path, handler)
				server := httptest.NewServer(mux)
				defer server.Close()
				bundleClient := agentv1connect.NewBundleServiceClient(server.Client(), server.URL)
				reportClient := agentv1connect.NewAgentReportServiceClient(server.Client(), server.URL)
				pulled := &agentv1.AgentTask{TaskId: "check", Type: protoAgentTaskType(operation.kind), ServiceName: "demo", ServiceDir: "demo", RepoRevision: "deadbeef", NodeId: "main", DataNames: []string{"data"}}
				if operation.kind == task.TypeCaddySync {
					pulled.ParamsJson = `{"service_dirs":["demo","backup"],"full_rebuild":true}`
				}
				err = operation.execute(t.Context(), bundleClient, reportClient, cfg, pulled, nil)
				wantFailure := scenario == "missing" || (operation.checkFiles && scenario != "match")
				if (err != nil) != wantFailure {
					t.Fatalf("unexpected task result: %v", err)
				}
				after, err := os.Stat(unmanaged)
				if err != nil {
					t.Fatal(err)
				}
				dirAfter, err := os.Stat(filepath.Dir(unmanaged))
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(before, after) || !os.SameFile(dirBefore, dirAfter) || readAgentTestFile(t, unmanaged) != "must survive\n" {
					t.Fatal("task replaced local files or directory")
				}
				if scenario == "missing" {
					if _, err := os.Stat(meta); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("task installed missing metadata")
					}
				}
				for _, name := range []string{".composia-backup.json", ".composia-restore.json"} {
					if _, err := os.Stat(filepath.Join(cfg.RepoDir, "demo", name)); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("runtime parameters were written into the service directory")
					}
				}
				if wantFailure {
					if commands, _ := os.ReadFile(logFile); len(commands) != 0 { //nolint:gosec // The path belongs to the test's temporary directory.
						t.Fatalf("preflight failure executed Docker: %s", commands)
					}
					if readAgentTestFile(t, generated) != "existing generated config\n" {
						t.Fatal("preflight failure changed generated files")
					}
				}
				if operation.checkFiles {
					reports.mu.Lock()
					defer reports.mu.Unlock()
					if len(reports.consistency) == 0 {
						t.Fatal("missing file check report")
					}
					last := reports.consistency[len(reports.consistency)-1]
					want := "consistent"
					if wantFailure {
						want = "drifted"
					}
					if last.GetFiles().GetStatus() != want || last.GetCompose().GetStatus() != "unknown" {
						t.Fatalf("unexpected file-only result: %v", last)
					}
					if reports.runtimeStatus != "" {
						t.Fatal("file-only task changed runtime status")
					}
				}
				if scenario == "drift" && !strings.Contains(readAgentTestFile(t, filepath.Join(cfg.RepoDir, "demo/settings.conf")), "local edit") {
					t.Fatal("local edit was overwritten")
				}
			})
		}
	}
}
