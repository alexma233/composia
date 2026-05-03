package store

import (
	"context"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestListBackupsAppliesFiltersAndCursor(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "edge"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", CreatedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-1: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-2", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "bravo", NodeID: "edge", CreatedAt: time.Date(2026, 4, 4, 12, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-2: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-3", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "edge", CreatedAt: time.Date(2026, 4, 4, 12, 10, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-3: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, BackupDetail{BackupID: "backup-1", TaskID: "task-1", ServiceName: "alpha", DataName: "config", Status: "succeeded", StartedAt: "2026-04-04T12:00:00Z", FinishedAt: "2026-04-04T12:01:00Z"}); err != nil {
		t.Fatalf("insert backup-1: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, BackupDetail{BackupID: "backup-2", TaskID: "task-2", ServiceName: "bravo", DataName: "db", Status: "failed", StartedAt: "2026-04-04T12:05:00Z", FinishedAt: "2026-04-04T12:06:00Z"}); err != nil {
		t.Fatalf("insert backup-2: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, BackupDetail{BackupID: "backup-3", TaskID: "task-3", ServiceName: "alpha", DataName: "db", Status: "succeeded", StartedAt: "2026-04-04T12:10:00Z", FinishedAt: "2026-04-04T12:11:00Z"}); err != nil {
		t.Fatalf("insert backup-3: %v", err)
	}

	backups, totalCount, err := db.ListBackups(ctx, nil, nil, nil, nil, nil, nil, nil, nil, 1, 1)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 1 || backups[0].BackupID != "backup-3" {
		t.Fatalf("unexpected backup page: %+v", backups)
	}
	if totalCount != 3 {
		t.Fatalf("expected total count 3, got %d", totalCount)
	}

	backups, _, err = db.ListBackups(ctx, []string{"alpha"}, []string{"succeeded"}, []string{"config"}, nil, nil, nil, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("list alpha backups: %v", err)
	}
	if len(backups) != 1 || backups[0].BackupID != "backup-1" {
		t.Fatalf("unexpected filtered backups: %+v", backups)
	}

	backups, _, err = db.ListBackups(ctx, nil, nil, nil, []string{"edge"}, nil, []string{"failed"}, []string{"config"}, nil, 1, 10)
	if err != nil {
		t.Fatalf("list included node backups with exclusions: %v", err)
	}
	if len(backups) != 1 || backups[0].BackupID != "backup-3" {
		t.Fatalf("unexpected node filtered backups: %+v", backups)
	}
}

func TestGetBackupReturnsDetail(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, BackupDetail{BackupID: "backup-1", TaskID: "task-1", ServiceName: "alpha", DataName: "config", Status: "succeeded", StartedAt: "2026-04-04T12:00:00Z", FinishedAt: "2026-04-04T12:01:00Z", ArtifactRef: "snapshot-1"}); err != nil {
		t.Fatalf("insert backup: %v", err)
	}

	backup, err := db.GetBackup(ctx, "backup-1")
	if err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if backup.ArtifactRef != "snapshot-1" || backup.DataName != "config" {
		t.Fatalf("unexpected backup detail: %+v", backup)
	}
}

func TestBackupNodeIDFallsBackToSourceTask(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", CreatedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, BackupDetail{BackupID: "backup-1", TaskID: "task-1", ServiceName: "alpha", DataName: "config", Status: "succeeded", StartedAt: "2026-04-04T12:00:00Z", ArtifactRef: "snapshot-1"}); err != nil {
		t.Fatalf("insert backup: %v", err)
	}

	backup, err := db.GetBackup(ctx, "backup-1")
	if err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if backup.NodeID != "main" {
		t.Fatalf("expected fallback node main, got %q", backup.NodeID)
	}
}
