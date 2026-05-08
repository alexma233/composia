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
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo", "charlie"); err != nil {
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

func TestUpdateServiceInstanceRuntimeStatusValidatesAndPersists(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	reportedAt := time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)
	if err := db.UpdateServiceInstanceRuntimeStatus(ctx, "alpha", "main", ServiceRuntimeRunning, reportedAt); err != nil {
		t.Fatalf("update service instance runtime status: %v", err)
	}

	snapshot, err := db.GetServiceSnapshot(ctx, "alpha")
	if err != nil {
		t.Fatalf("get service snapshot: %v", err)
	}
	if snapshot.RuntimeStatus != ServiceRuntimeRunning {
		t.Fatalf("expected runtime status running, got %q", snapshot.RuntimeStatus)
	}
	instance, err := db.GetServiceInstanceSnapshot(ctx, "alpha", "main")
	if err != nil {
		t.Fatalf("get service instance snapshot: %v", err)
	}
	if instance.UpdatedAt != reportedAt.Format(time.RFC3339) {
		t.Fatalf("expected instance updated_at %q, got %q", reportedAt.Format(time.RFC3339), instance.UpdatedAt)
	}

	if err := db.UpdateServiceInstanceRuntimeStatus(ctx, "alpha", "main", "broken", reportedAt); err == nil {
		t.Fatalf("expected invalid runtime status error")
	}
}

func TestServiceImageUpdateChecksRoundTrip(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	checkedAt := time.Date(2026, 5, 8, 4, 0, 0, 0, time.UTC)
	if err := db.UpsertServiceImageUpdateChecks(ctx, []ServiceImageUpdateCheck{{
		ServiceName:       "app",
		NodeID:            "main",
		ImageName:         "api",
		ImageRef:          "ghcr.io/example/api",
		PolicyType:        "semver",
		CurrentValue:      "1.2.3@sha256:old",
		CurrentTag:        "1.2.3",
		CurrentDigest:     "sha256:old",
		CandidateTag:      "1.3.0",
		CandidateDigest:   "sha256:new",
		CandidateTagsJSON: `["1.3.0"]`,
		UpdateAvailable:   true,
		CheckStatus:       ImageCheckStatusOK,
		CheckedAt:         checkedAt,
	}}); err != nil {
		t.Fatalf("upsert image update checks: %v", err)
	}

	checks, err := db.LatestServiceImageUpdateChecks(ctx, "app", "main")
	if err != nil {
		t.Fatalf("latest image update checks: %v", err)
	}
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %+v", checks)
	}
	check := checks[0]
	if !check.UpdateAvailable || check.CandidateTag != "1.3.0" || check.CandidateTagsJSON != `["1.3.0"]` {
		t.Fatalf("unexpected check: %+v", check)
	}
}
