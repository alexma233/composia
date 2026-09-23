package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"google.golang.org/protobuf/proto"
)

func TestReportServiceConsistencyCheckAuthorizationAndMapping(t *testing.T) {
	t.Parallel()
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := t.Context()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "other"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"demo": {"main", "other"}}); err != nil {
		t.Fatal(err)
	}
	empty, err := db.GetServiceInstanceSnapshot(ctx, "demo", "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range []*agentv1.ServiceConsistencyCheck{serviceInstanceSummaryMessage(empty).GetConsistency(), serviceInstanceDetailMessage(empty, nil).GetConsistency()} {
		if snapshot.GetFiles().GetStatus() != "unknown" || snapshot.GetCompose().GetStatus() != "unknown" || snapshot.GetCheckedAt() != "" {
			t.Fatalf("unchecked mapping: %+v", snapshot)
		}
	}
	now := time.Now().UTC()
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "check", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", RepoRevision: "revision", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	offered, err := db.ClaimNextPendingTaskForNode(ctx, "main", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AcknowledgeTaskExecution(ctx, "check", offered.ExecutionID, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	empty, err = db.GetServiceInstanceSnapshot(ctx, "demo", "main")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewAgentReportServiceHandler(&agentReportServer{db: db}, connect.WithInterceptors(rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) { return token, nil })))
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := func(node string) agentv1connect.AgentReportServiceClient {
		return agentv1connect.NewAgentReportServiceClient(server.Client(), server.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(node)))
	}
	request := &agentv1.ReportServiceConsistencyCheckRequest{TaskId: "check", ExecutionId: offered.ExecutionID, Files: &agentv1.ServiceConsistencyOutcome{Status: "consistent"}, Compose: &agentv1.ServiceConsistencyOutcome{Status: "drifted", Reasons: []string{"config differs"}}}
	if _, err := client("other").ReportServiceConsistencyCheck(ctx, connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("foreign report: %v", err)
	}
	stale := proto.CloneOf(request)
	stale.ExecutionId = "stale"
	if _, err := client("main").ReportServiceConsistencyCheck(ctx, connect.NewRequest(stale)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("stale report: %v", err)
	}
	invalid := proto.CloneOf(request)
	invalid.Files.Status = "invalid"
	if _, err := client("main").ReportServiceConsistencyCheck(ctx, connect.NewRequest(invalid)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid status: %v", err)
	}
	if _, err := client("main").ReportServiceConsistencyCheck(ctx, connect.NewRequest(request)); err != nil {
		t.Fatal(err)
	}
	record, err := db.GetServiceInstanceSnapshot(ctx, "demo", "main")
	if err != nil {
		t.Fatal(err)
	}
	summary, detail := serviceInstanceSummaryMessage(record).GetConsistency(), serviceInstanceDetailMessage(record, nil).GetConsistency()
	if !proto.Equal(summary, detail) || summary.GetCheckedAt() == "" || summary.GetTaskId() != "check" || summary.GetRepoRevision() != "revision" || !proto.Equal(summary.GetCompose(), request.GetCompose()) || !proto.Equal(summary.GetFiles(), request.GetFiles()) {
		t.Fatalf("query mapping: %+v", summary)
	}
	if record.RuntimeStatus != empty.RuntimeStatus || record.UpdatedAt != empty.UpdatedAt {
		t.Fatal("report changed runtime state")
	}
	if err := db.CompleteTaskExecution(ctx, "check", offered.ExecutionID, task.StatusSucceeded, now, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := client("main").ReportServiceConsistencyCheck(ctx, connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("terminal report: %v", err)
	}
}
