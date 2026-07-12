package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestTaskExecutionLeaseLifecycle(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 12, 1, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-lease", Type: task.TypePrune, Source: task.SourceSystem, NodeID: "main", CreatedAt: createdAt}); err != nil {
		t.Fatal(err)
	}
	offered, err := db.ClaimNextPendingTaskForNode(ctx, "main", createdAt.Add(time.Minute), createdAt.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if offered.ExecutionID == "" || offered.ExecutionState != task.ExecutionOffered {
		t.Fatalf("unexpected offer: %+v", offered)
	}
	if err := db.CompleteTaskExecution(ctx, offered.TaskID, offered.ExecutionID, task.StatusSucceeded, createdAt.Add(80*time.Second), ""); !errors.Is(err, ErrTaskExecutionMismatch) {
		t.Fatalf("expected unacknowledged completion to fail, got %v", err)
	}
	if err := db.AcknowledgeTaskExecution(ctx, offered.TaskID, offered.ExecutionID, createdAt.Add(90*time.Second), createdAt.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ValidateTaskExecution(ctx, offered.TaskID, offered.ExecutionID, "main"); err != nil {
		t.Fatal(err)
	}
	requeued, lost, err := db.SweepExpiredTaskExecutions(ctx, createdAt.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if requeued != 0 || lost != 1 {
		t.Fatalf("unexpected sweep counts requeued=%d lost=%d", requeued, lost)
	}
	detail, err := db.GetTask(ctx, offered.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Record.Status != task.StatusRunning || detail.Record.ExecutionState != task.ExecutionLeaseLost {
		t.Fatalf("unexpected lease-lost task: %+v", detail.Record)
	}
	if _, err := db.ValidateTaskExecution(ctx, offered.TaskID, offered.ExecutionID, "main"); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredUnacknowledgedTaskOfferIsRequeued(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 12, 2, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-offer", Type: task.TypePrune, Source: task.SourceSystem, NodeID: "main", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ClaimNextPendingTaskForNode(ctx, "main", now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	detail, err := db.GetTask(ctx, "task-offer")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AcknowledgeTaskExecution(ctx, detail.Record.TaskID, detail.Record.ExecutionID, now.Add(2*time.Minute), now.Add(3*time.Minute)); !errors.Is(err, ErrTaskExecutionMismatch) {
		t.Fatalf("expected expired offer acknowledgement to fail, got %v", err)
	}
	requeued, lost, err := db.SweepExpiredTaskExecutions(ctx, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if requeued != 1 || lost != 0 {
		t.Fatalf("unexpected sweep counts requeued=%d lost=%d", requeued, lost)
	}
	detail, err = db.GetTask(ctx, "task-offer")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Record.Status != task.StatusPending || detail.Record.ExecutionID != "" {
		t.Fatalf("unexpected requeued task: %+v", detail.Record)
	}
}

func TestCompleteTaskExecutionIsIdempotentAndEnqueuesOutbox(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"alpha": {"main"}}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-complete", Type: task.TypeDeploy, Source: task.SourceSystem, ServiceName: "alpha", NodeID: "main", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	offered, err := db.ClaimNextPendingTaskForNode(ctx, "main", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AcknowledgeTaskExecution(ctx, offered.TaskID, offered.ExecutionID, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	stepStartedAt := now.Add(10 * time.Second)
	if err := db.UpsertTaskStep(ctx, task.StepRecord{TaskID: offered.TaskID, StepName: task.StepComposeUp, Status: task.StatusRunning, StartedAt: &stepStartedAt}); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteTaskExecution(ctx, offered.TaskID, offered.ExecutionID, task.StatusSucceeded, now.Add(30*time.Second), ""); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteTaskExecution(ctx, offered.TaskID, offered.ExecutionID, task.StatusSucceeded, now.Add(40*time.Second), ""); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteTaskExecution(ctx, offered.TaskID, offered.ExecutionID, task.StatusSucceeded, now.Add(40*time.Second), "different result"); !errors.Is(err, ErrTaskExecutionConflict) {
		t.Fatalf("expected differing terminal result conflict, got %v", err)
	}
	if err := db.CompleteTaskExecution(ctx, offered.TaskID, offered.ExecutionID, task.StatusFailed, now.Add(40*time.Second), "late failure"); !errors.Is(err, ErrTaskExecutionConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	instance, err := db.GetServiceInstanceSnapshot(ctx, "alpha", "main")
	if err != nil {
		t.Fatal(err)
	}
	if instance.RuntimeStatus != ServiceRuntimeRunning {
		t.Fatalf("expected atomic running status, got %q", instance.RuntimeStatus)
	}
	detail, err := db.GetTask(ctx, offered.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].Status != task.StatusSucceeded || detail.Steps[0].FinishedAt == nil {
		t.Fatalf("expected running step to be finalized: %+v", detail.Steps)
	}
	event, err := db.NextTaskOutboxEvent(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if event.TaskID != offered.TaskID || event.EventType != "task_completed" {
		t.Fatalf("unexpected outbox event: %+v", event)
	}
	if err := db.CompleteTaskOutboxFollowups(ctx, event.EventID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteTaskOutboxNotification(ctx, event.EventID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteTaskOutboxNotification(ctx, event.EventID, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	event, err = db.NextTaskOutboxEvent(ctx, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !event.FollowupsCompleted || !event.NotificationDispatched {
		t.Fatalf("outbox stages were not persisted: %+v", event)
	}
}

func TestFailLostTaskExecutionUpdatesRuntimeAndSteps(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"alpha": {"main"}}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 12, 4, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-lost", Type: task.TypeDeploy, Source: task.SourceSystem, ServiceName: "alpha", NodeID: "main", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	offered, err := db.ClaimNextPendingTaskForNode(ctx, "main", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AcknowledgeTaskExecution(ctx, offered.TaskID, offered.ExecutionID, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertTaskStep(ctx, task.StepRecord{TaskID: offered.TaskID, StepName: task.StepComposeUp, Status: task.StatusRunning}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.SweepExpiredTaskExecutions(ctx, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := db.FailLostTaskExecution(ctx, offered.TaskID, now.Add(3*time.Minute), "lost"); err != nil {
		t.Fatal(err)
	}
	instance, err := db.GetServiceInstanceSnapshot(ctx, "alpha", "main")
	if err != nil {
		t.Fatal(err)
	}
	if instance.RuntimeStatus != ServiceRuntimeError {
		t.Fatalf("expected error runtime, got %q", instance.RuntimeStatus)
	}
	detail, err := db.GetTask(ctx, offered.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].Status != task.StatusFailed {
		t.Fatalf("expected failed step: %+v", detail.Steps)
	}
}
