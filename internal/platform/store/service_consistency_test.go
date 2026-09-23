package store

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestServiceConsistencyPersistsWithoutChangingRuntime(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "other"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"demo": {"main", "other"}, "foreign": {"main"}}); err != nil {
		t.Fatal(err)
	}
	before, err := db.GetServiceInstanceSnapshot(ctx, "demo", "main")
	if err != nil || before.Consistency != nil {
		t.Fatalf("unchecked instance: %+v %v", before, err)
	}
	now := time.Now().UTC()
	// A deploy task proves the storage contract is independent of image_check.
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "check", Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", RepoRevision: "revision", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	offered, err := db.ClaimNextPendingTaskForNode(ctx, "main", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	outcome := ServiceConsistencyOutcome{Status: ConsistencyConsistent}
	report := func(execution, node string) error {
		return db.RecordServiceConsistencyCheck(ctx, "check", execution, node, outcome, outcome)
	}
	if err := report(offered.ExecutionID, "main"); !errors.Is(err, ErrTaskExecutionMismatch) {
		t.Fatalf("unacknowledged report accepted: %v", err)
	}
	if err := db.AcknowledgeTaskExecution(ctx, "check", offered.ExecutionID, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateServiceInstanceRuntimeStatus(ctx, "demo", "main", ServiceRuntimeStopped, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	before, err = db.GetServiceInstanceSnapshot(ctx, "demo", "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"stale", "main"}, {offered.ExecutionID, "other"}, {"", "main"}} {
		if err := report(pair[0], pair[1]); !errors.Is(err, ErrTaskExecutionMismatch) {
			t.Fatalf("foreign/stale report accepted: %v", err)
		}
	}
	if err := report(offered.ExecutionID, "main"); err != nil {
		t.Fatal(err)
	}
	after, err := db.GetServiceInstanceSnapshot(ctx, "demo", "main")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := after.Consistency
	if snapshot == nil || snapshot.TaskID != "check" || snapshot.RepoRevision != "revision" || snapshot.Files.Status != ConsistencyConsistent || snapshot.Compose.Status != ConsistencyConsistent {
		t.Fatalf("snapshot: %+v", snapshot)
	}
	checkedAt, err := time.Parse(time.RFC3339Nano, snapshot.CheckedAt)
	if err != nil || checkedAt.Before(now) {
		t.Fatalf("server timestamp: %s %v", snapshot.CheckedAt, err)
	}
	after.Consistency = nil
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("runtime changed: before=%+v after=%+v", before, after)
	}
	for _, pair := range [][2]string{{"demo", "other"}, {"foreign", "main"}} {
		other, err := db.GetServiceInstanceSnapshot(ctx, pair[0], pair[1])
		if err != nil || other.Consistency != nil {
			t.Fatalf("foreign instance modified: %+v %v", other, err)
		}
	}
	if _, _, err := db.SweepExpiredTaskExecutions(ctx, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	outcome = ServiceConsistencyOutcome{Status: ConsistencyDrifted, Reasons: []string{"config hash differs"}}
	if err := report(offered.ExecutionID, "main"); err != nil {
		t.Fatalf("current lease-lost report rejected: %v", err)
	}
	if err := db.CompleteTaskExecution(ctx, "check", offered.ExecutionID, task.StatusSucceeded, now.Add(3*time.Minute), ""); err != nil {
		t.Fatal(err)
	}
	if err := report(offered.ExecutionID, "main"); !errors.Is(err, ErrTaskExecutionMismatch) {
		t.Fatalf("terminal report accepted: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	instances, err := db.ListServiceInstances(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 2 || instances[0].Consistency == nil || instances[0].Consistency.Files.Status != ConsistencyDrifted || !reflect.DeepEqual(instances[0].Consistency.Files.Reasons, outcome.Reasons) {
		t.Fatalf("snapshot lost across reopen: %+v", instances)
	}
}
