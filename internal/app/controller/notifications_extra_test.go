package controller

import (
	"context"
	"testing"
	"time"

	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestDetectNewImageUpdateChecks(t *testing.T) {
	t.Parallel()

	previous := []store.ServiceImageUpdateCheck{{ImageName: "api", UpdateAvailable: true, CandidateTag: "1.0.0", CandidateDigest: "sha256:old"}}
	current := []store.ServiceImageUpdateCheck{
		{ImageName: "api", UpdateAvailable: true, CandidateTag: "1.0.0", CandidateDigest: "sha256:old"},
		{ImageName: "worker", UpdateAvailable: true, CandidateTag: "2.0.0", CandidateDigest: "sha256:new"},
		{ImageName: "bad", UpdateAvailable: true, CheckStatus: store.ImageCheckStatusError},
	}
	newChecks := detectNewImageUpdateChecks(previous, current)
	if len(newChecks) != 1 || newChecks[0].ImageName != "worker" {
		t.Fatalf("new checks = %+v", newChecks)
	}
}

func TestNotificationEventTypeHelpers(t *testing.T) {
	t.Parallel()

	if eventType, ok := taskEventTypeForStatus(task.StatusSucceeded); !ok || eventType != corenotify.EventTaskCompleted {
		t.Fatalf("task succeeded event = %q/%v", eventType, ok)
	}
	if eventType, ok := taskEventTypeForStatus(task.StatusRunning); ok || eventType != "" {
		t.Fatalf("running event = %q/%v", eventType, ok)
	}
	if eventType, ok := backupEventTypeForStatus(string(task.StatusFailed)); !ok || eventType != corenotify.EventBackupFailed {
		t.Fatalf("backup failed event = %q/%v", eventType, ok)
	}
}

func TestNotificationTimeHelpers(t *testing.T) {
	t.Parallel()

	finishedAt := "2026-05-31T04:05:00Z"
	startedAt := "2026-05-31T04:00:00Z"
	if got := timeFromBackup(store.BackupDetail{StartedAt: startedAt, FinishedAt: finishedAt}); got.Format(time.RFC3339) != finishedAt {
		t.Fatalf("timeFromBackup finished = %s", got)
	}
	if got := timeFromBackup(store.BackupDetail{StartedAt: startedAt}); got.Format(time.RFC3339) != startedAt {
		t.Fatalf("timeFromBackup started = %s", got)
	}
	parsed := parseNodeHeartbeatTime(finishedAt)
	if parsed.Format(time.RFC3339) != finishedAt {
		t.Fatalf("parseNodeHeartbeatTime = %s", parsed)
	}
	value := time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC)
	if !derefTaskTime(nil, &value).Equal(value) {
		t.Fatalf("derefTaskTime did not return value")
	}
}

func TestSnapshotIfExists(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}
	snapshot, ok := snapshotIfExists(ctx, db, "main")
	if !ok || snapshot.NodeID != "main" {
		t.Fatalf("snapshot = %+v ok=%v", snapshot, ok)
	}
	if _, ok := snapshotIfExists(ctx, db, "missing"); ok {
		t.Fatalf("missing snapshot should not exist")
	}
}
