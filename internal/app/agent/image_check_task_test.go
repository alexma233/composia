package agent

import (
	"context"
	"errors"
	"fmt"
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
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
)

type imageCheckManifestServer struct {
	agentv1connect.UnimplementedBundleServiceHandler
	t        *testing.T
	manifest *agentv1.GetServiceManifestResponse
}

func (server imageCheckManifestServer) GetServiceBundle(context.Context, *connect.Request[agentv1.GetServiceBundleRequest], *connect.ServerStream[agentv1.GetServiceBundleResponse]) error {
	server.t.Error("image check must not download a bundle")
	return errors.New("unexpected bundle download")
}

func (server imageCheckManifestServer) GetServiceManifest(_ context.Context, req *connect.Request[agentv1.GetServiceManifestRequest]) (*connect.Response[agentv1.GetServiceManifestResponse], error) {
	if req.Msg.GetTaskId() != "task-image-check" {
		return nil, errors.New("unexpected task")
	}
	return connect.NewResponse(server.manifest), nil
}

type imageCheckReportServer struct {
	agentExecutionTestReportServer
	images      []*agentv1.ServiceImageState
	consistency []*agentv1.ReportServiceConsistencyCheckRequest
	checks      []*agentv1.ServiceImageUpdateCheck
}

func (server *imageCheckReportServer) ReportServiceImageStates(_ context.Context, req *connect.Request[agentv1.ReportServiceImageStatesRequest]) (*connect.Response[agentv1.ReportServiceImageStatesResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.images = req.Msg.GetImages()
	return connect.NewResponse(&agentv1.ReportServiceImageStatesResponse{}), nil
}

func (server *imageCheckReportServer) ReportServiceConsistencyCheck(_ context.Context, req *connect.Request[agentv1.ReportServiceConsistencyCheckRequest]) (*connect.Response[agentv1.ReportServiceConsistencyCheckResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.consistency = append(server.consistency, req.Msg)
	return connect.NewResponse(&agentv1.ReportServiceConsistencyCheckResponse{}), nil
}

func (server *imageCheckReportServer) ReportServiceImageUpdateChecks(_ context.Context, req *connect.Request[agentv1.ReportServiceImageUpdateChecksRequest]) (*connect.Response[agentv1.ReportServiceImageUpdateChecksResponse], error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.checks = req.Msg.GetChecks()
	return connect.NewResponse(&agentv1.ReportServiceImageUpdateChecksResponse{}), nil
}

func TestExecuteImageCheckTaskIsReadOnly(t *testing.T) {
	for _, scenario := range []string{"config-infra", "running", "file-drift", "missing-root", "missing-meta", "wrong-revision", "compose-drift", "stopped", "registry-error"} {
		t.Run(scenario, func(t *testing.T) {
			const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			rootDir := t.TempDir()
			cfg := &config.AgentConfig{RepoDir: rootDir, StateDir: rootDir}
			meta := "name: demo\nnodes:\n  - main\n"
			if scenario == "config-infra" {
				meta += "infra:\n  config: {}\n"
			}
			if scenario == "registry-error" {
				meta += "update:\n  images:\n    app:\n      image: example/app\n      current:\n        yaml:\n          file: compose.yaml\n          path: services.app.image\n      discovery:\n        sources:\n          - type: digest\n"
			}
			metaPath := filepath.Join(rootDir, "demo", "composia-meta.yaml")
			writeAgentTestFile(t, metaPath, meta)
			manifest := &agentv1.GetServiceManifestResponse{RepoRevision: "deadbeef", RelativeRoot: "demo", Files: []*agentv1.ServiceManifestFile{testManifestFile("composia-meta.yaml", meta, 0o600)}}
			if scenario == "registry-error" {
				composeText := "services:\n  app:\n    image: example/app:latest\n"
				writeAgentTestFile(t, filepath.Join(rootDir, "demo", "compose.yaml"), composeText)
				manifest.Files = append(manifest.Files, testManifestFile("compose.yaml", composeText, 0o600))
			}
			if scenario == "file-drift" {
				manifest.Files[0].Sha256 = strings.Repeat("0", 64)
			}
			if scenario == "wrong-revision" {
				manifest.RepoRevision = "outdated"
			}
			before, err := os.Stat(metaPath)
			if err != nil {
				t.Fatal(err)
			}
			dirBefore, err := os.Stat(filepath.Dir(metaPath))
			if err != nil {
				t.Fatal(err)
			}
			missingFiles := scenario == "missing-root" || scenario == "missing-meta"
			if missingFiles {
				if err := os.Remove(metaPath); err != nil {
					t.Fatal(err)
				}
				if scenario == "missing-root" {
					if err := os.Remove(filepath.Dir(metaPath)); err != nil {
						t.Fatal(err)
					}
				}
			}
			containerHash := hash
			if scenario == "compose-drift" {
				containerHash = "old"
			}
			containerState := "running"
			if scenario == "stopped" {
				containerState = "exited"
			}
			logFile := installFakeDockerScript(t, fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> "$TEST_DOCKER_LOG_FILE"
case "$*" in
  'compose version --short') printf '5.5.1\n' ;;
  *'config --format json') printf '%%s\n' '{"services":{"app":{"image":"example/app:latest"}}}' ;;
  *'config --hash *') printf 'app %s\n' ;;
  'container ls '*) printf 'container-a\n' ;;
  'container inspect '*) printf '%%s\n' '[{"Id":"container-a","Image":"sha256:running","State":{"Status":"%s"},"Config":{"Labels":{"com.docker.compose.project":"demo","com.docker.compose.service":"app","com.docker.compose.config-hash":"%s"}}}]' ;;
  'image inspect --format {{json .RepoDigests}} sha256:running') printf '%%s\n' '["example/app@%s"]' ;;
  buildx*) echo "registry unavailable" >&2; exit 2 ;;
  *) exit 2 ;;
esac
`, hash, containerState, containerHash, digest))
			interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
				if token != agentTestMainToken {
					return "", errors.New("unexpected token")
				}
				return agentTestMainNodeID, nil
			})
			bundleMux := http.NewServeMux()
			bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(imageCheckManifestServer{t: t, manifest: manifest}, connect.WithInterceptors(interceptor))
			bundleMux.Handle(bundlePath, bundleHandler)
			bundleHTTPServer := httptest.NewServer(bundleMux)
			defer bundleHTTPServer.Close()
			reportServer := &imageCheckReportServer{}
			reportMux := http.NewServeMux()
			reportPath, reportHandler := agentv1connect.NewAgentReportServiceHandler(reportServer, connect.WithInterceptors(interceptor))
			reportMux.Handle(reportPath, reportHandler)
			reportHTTPServer := httptest.NewUnstartedServer(reportMux)
			reportHTTPServer.EnableHTTP2 = true
			reportHTTPServer.StartTLS()
			defer reportHTTPServer.Close()
			bundleClient := agentv1connect.NewBundleServiceClient(bundleHTTPServer.Client(), bundleHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(agentTestMainToken)))
			reportClient := agentv1connect.NewAgentReportServiceClient(reportHTTPServer.Client(), reportHTTPServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(agentTestMainToken)))
			logUploader := newTaskLogUploader(reportClient, "task-image-check")
			defer func() { _ = logUploader.Close() }()
			pulledTask := &agentv1.AgentTask{TaskId: "task-image-check", Type: protoAgentTaskType(task.TypeImageCheck), ServiceName: "demo", NodeId: agentTestMainNodeID, RepoRevision: "deadbeef", ServiceDir: "demo"}
			err = executeImageCheckTask(t.Context(), bundleClient, reportClient, cfg, pulledTask, logUploader)
			wantSuccess := scenario == "config-infra" || scenario == "running" || scenario == "registry-error"
			if wantSuccess != (err == nil) {
				t.Fatalf("unexpected task result: %v", err)
			}
			reportServer.mu.Lock()
			defer reportServer.mu.Unlock()
			wantStatus := string(task.StatusFailed)
			if scenario == "config-infra" || scenario == "running" || scenario == "registry-error" {
				wantStatus = string(task.StatusSucceeded)
			}
			if reportServer.taskStatus != wantStatus {
				t.Fatalf("status=%s error=%s", reportServer.taskStatus, reportServer.taskErrorSummary)
			}
			if len(reportServer.consistency) != 1 {
				t.Fatalf("consistency reports=%d", len(reportServer.consistency))
			}
			snapshot := reportServer.consistency[0]
			wantFiles, wantCompose := "consistent", "consistent"
			switch scenario {
			case "config-infra":
				wantCompose = "not_applicable"
			case "file-drift", "missing-root", "missing-meta":
				wantFiles, wantCompose = "drifted", "unknown"
			case "wrong-revision":
				wantFiles, wantCompose = "error", "unknown"
			case "compose-drift":
				wantCompose = "drifted"
			}
			if snapshot.GetFiles().GetStatus() != wantFiles || snapshot.GetCompose().GetStatus() != wantCompose {
				t.Fatalf("unexpected consistency: %+v", snapshot)
			}
			if scenario == "registry-error" && (len(reportServer.checks) != 1 || reportServer.checks[0].GetCheckStatus() != "error" || !strings.Contains(reportServer.checks[0].GetErrorSummary(), "registry unavailable")) {
				t.Fatalf("expected registry error: %+v", reportServer.checks)
			}
			if reportServer.runtimeStatus != "" {
				t.Fatal("read-only check changed runtime status")
			}
			if scenario == "running" && (len(reportServer.images) != 1 || reportServer.images[0].GetLocalDigest() != digest) {
				t.Fatalf("unexpected running observations: %+v", reportServer.images)
			}
			if missingFiles {
				if _, err := os.Stat(metaPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing metadata was recreated: %v", err)
				}
				if scenario == "missing-root" {
					if _, err := os.Stat(filepath.Dir(metaPath)); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("missing service directory was recreated: %v", err)
					}
				}
				if contents, err := os.ReadFile(logFile); err == nil && len(contents) > 0 { //nolint:gosec // The path belongs to the test's temporary directory.
					t.Fatalf("unexpected Docker commands: %s", contents)
				}
				return
			}
			after, err := os.Stat(metaPath)
			if err != nil {
				t.Fatal(err)
			}
			dirAfter, err := os.Stat(filepath.Dir(metaPath))
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || !os.SameFile(dirBefore, dirAfter) || readAgentTestFile(t, metaPath) != meta {
				t.Fatal("image check changed deployed files")
			}
			if scenario == "file-drift" || scenario == "wrong-revision" || scenario == "config-infra" {
				if contents, err := os.ReadFile(logFile); err == nil && len(contents) > 0 { //nolint:gosec // The path belongs to the test's temporary directory.
					t.Fatalf("unexpected Docker commands: %s", contents)
				}
			}
		})
	}
}
