package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListDeclaredServicesAppliesCursorAndFilter(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha", "bravo", "charlie"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}

	if _, err := db.sql.ExecContext(ctx, `UPDATE services SET runtime_status = 'running', updated_at = ? WHERE service_name IN ('alpha', 'charlie')`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("update running services: %v", err)
	}
	if _, err := db.sql.ExecContext(ctx, `UPDATE services SET runtime_status = 'stopped', updated_at = ? WHERE service_name = 'bravo'`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("update stopped service: %v", err)
	}

	services, totalCount, err := db.ListDeclaredServices(ctx, "", 1, 2)
	if err != nil {
		t.Fatalf("list declared services page 1: %v", err)
	}
	if len(services) != 2 || services[0].Name != "alpha" || services[1].Name != "bravo" {
		t.Fatalf("unexpected first page: %+v", services)
	}
	if totalCount != 3 {
		t.Fatalf("expected total count 3, got %d", totalCount)
	}

	services, _, err = db.ListDeclaredServices(ctx, "running", 1, 10)
	if err != nil {
		t.Fatalf("list filtered services: %v", err)
	}
	if len(services) != 2 || services[0].Name != "alpha" || services[1].Name != "charlie" {
		t.Fatalf("unexpected filtered services: %+v", services)
	}
}

func TestUpdateServiceRuntimeStatusValidatesAndPersists(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	reportedAt := time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)
	if err := db.UpdateServiceRuntimeStatus(ctx, "alpha", ServiceRuntimeRunning, reportedAt); err != nil {
		t.Fatalf("update service runtime status: %v", err)
	}

	snapshot, err := db.GetServiceSnapshot(ctx, "alpha")
	if err != nil {
		t.Fatalf("get service snapshot: %v", err)
	}
	if snapshot.RuntimeStatus != ServiceRuntimeRunning {
		t.Fatalf("expected runtime status running, got %q", snapshot.RuntimeStatus)
	}
	if snapshot.UpdatedAt != reportedAt.Format(time.RFC3339) {
		t.Fatalf("expected updated_at %q, got %q", reportedAt.Format(time.RFC3339), snapshot.UpdatedAt)
	}

	if err := db.UpdateServiceRuntimeStatus(ctx, "alpha", "broken", reportedAt); err == nil {
		t.Fatalf("expected invalid runtime status error")
	}
}
