package store

import (
	"context"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestListBackupsAppliesFiltersAndCursor(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-1: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-2", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "bravo", CreatedAt: time.Date(2026, 4, 4, 12, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-2: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, BackupDetail{BackupID: "backup-1", TaskID: "task-1", ServiceName: "alpha", DataName: "config", Status: "succeeded", StartedAt: "2026-04-04T12:00:00Z", FinishedAt: "2026-04-04T12:01:00Z"}); err != nil {
		t.Fatalf("insert backup-1: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, BackupDetail{BackupID: "backup-2", TaskID: "task-2", ServiceName: "bravo", DataName: "db", Status: "failed", StartedAt: "2026-04-04T12:05:00Z", FinishedAt: "2026-04-04T12:06:00Z"}); err != nil {
		t.Fatalf("insert backup-2: %v", err)
	}

	backups, totalCount, err := db.ListBackups(ctx, "", "", "", 1, 1)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 1 || backups[0].BackupID != "backup-2" {
		t.Fatalf("unexpected backup page: %+v", backups)
	}
	if totalCount != 2 {
		t.Fatalf("expected total count 2, got %d", totalCount)
	}

	backups, _, err = db.ListBackups(ctx, "alpha", "", "config", 1, 10)
	if err != nil {
		t.Fatalf("list alpha backups: %v", err)
	}
	if len(backups) != 1 || backups[0].BackupID != "backup-1" {
		t.Fatalf("unexpected filtered backups: %+v", backups)
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
