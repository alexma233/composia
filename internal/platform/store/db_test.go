package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestListDeclaredServicesAppliesCursorAndFilter(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
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
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
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
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
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

func TestNodeCountsAndMarkOfflineNodesBefore(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "worker"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC), AgentVersion: "1.0.0"}); err != nil {
		t.Fatalf("record main heartbeat: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, NodeHeartbeat{NodeID: "worker", HeartbeatAt: time.Date(2026, 5, 31, 4, 0, 0, 0, time.UTC), AgentVersion: "1.0.0"}); err != nil {
		t.Fatalf("record worker heartbeat: %v", err)
	}

	configured, online, err := db.NodeCounts(ctx)
	if err != nil {
		t.Fatalf("node counts: %v", err)
	}
	if configured != 2 || online != 2 {
		t.Fatalf("counts before offline = configured:%d online:%d", configured, online)
	}

	if err := db.MarkOfflineNodesBefore(ctx, time.Date(2026, 5, 31, 4, 3, 0, 0, time.UTC)); err != nil {
		t.Fatalf("mark offline nodes: %v", err)
	}
	configured, online, err = db.NodeCounts(ctx)
	if err != nil {
		t.Fatalf("node counts after offline: %v", err)
	}
	if configured != 2 || online != 1 {
		t.Fatalf("counts after offline = configured:%d online:%d", configured, online)
	}
}

func TestServiceCounts(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "alpha", "bravo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.UpdateServiceInstanceRuntimeStatus(ctx, "alpha", "main", ServiceRuntimeRunning, time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC)); err != nil {
		t.Fatalf("update alpha status: %v", err)
	}

	total, running, err := db.ServiceCounts(ctx)
	if err != nil {
		t.Fatalf("service counts: %v", err)
	}
	if total != 2 || running != 1 {
		t.Fatalf("service counts = total:%d running:%d", total, running)
	}
}

func TestRepoSyncStateRoundTrip(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	state, err := db.GetRepoSyncState(ctx)
	if err != nil {
		t.Fatalf("get initial repo sync state: %v", err)
	}
	if state != (RepoSyncState{}) {
		t.Fatalf("expected empty initial repo state, got %+v", state)
	}

	if err := db.UpsertRepoSyncState(ctx, RepoSyncState{LastSyncError: "pull failed"}); err != nil {
		t.Fatalf("upsert repo sync state: %v", err)
	}
	state, err = db.GetRepoSyncState(ctx)
	if err != nil {
		t.Fatalf("get repo sync state: %v", err)
	}
	if state.SyncStatus != RepoSyncStatusUnknown || state.LastSyncError != "pull failed" {
		t.Fatalf("unexpected defaulted repo state: %+v", state)
	}

	if err := db.UpsertRepoSyncState(ctx, RepoSyncState{SyncStatus: RepoSyncStatusSynced, LastSuccessfulPullAt: "2026-05-31T04:05:00Z"}); err != nil {
		t.Fatalf("upsert synced repo state: %v", err)
	}
	state, err = db.GetRepoSyncState(ctx)
	if err != nil {
		t.Fatalf("get synced repo state: %v", err)
	}
	if state.SyncStatus != RepoSyncStatusSynced || state.LastSyncError != "" || state.LastSuccessfulPullAt != "2026-05-31T04:05:00Z" {
		t.Fatalf("unexpected synced repo state: %+v", state)
	}
}

func TestDockerStatsRoundTrip(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	reportedAt := time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC)
	if err := db.RecordDockerStats(ctx, DockerStats{
		NodeID:              "main",
		ContainersTotal:     5,
		ContainersRunning:   3,
		ContainersStopped:   2,
		ContainersPaused:    1,
		Images:              9,
		Networks:            4,
		Volumes:             7,
		VolumesSizeBytes:    1024,
		DisksUsageBytes:     2048,
		DockerServerVersion: "27.0.0",
		ReportedAt:          reportedAt,
	}); err != nil {
		t.Fatalf("record docker stats: %v", err)
	}

	stats, err := db.GetNodeDockerStats(ctx, "main")
	if err != nil {
		t.Fatalf("get docker stats: %v", err)
	}
	if stats.NodeID != "main" || stats.ContainersRunning != 3 || stats.VolumesSizeBytes != 1024 || !stats.ReportedAt.Equal(reportedAt) {
		t.Fatalf("unexpected docker stats: %+v", stats)
	}

	missing, err := db.GetNodeDockerStats(ctx, "missing")
	if err != nil {
		t.Fatalf("get missing docker stats: %v", err)
	}
	if missing != (DockerStats{}) {
		t.Fatalf("expected empty missing docker stats, got %+v", missing)
	}
}

func TestDBPathAndSQLAccessors(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()
	if db.Path() == "" {
		t.Fatalf("expected db path")
	}
	if db.SQL() == nil {
		t.Fatalf("expected sql handle")
	}
}

func TestListServiceInstancesAndPendingDeploy(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "edge"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main", "edge"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SetServicePendingDeploy(ctx, "app", []string{"main", "edge"}, "rev-1"); err != nil {
		t.Fatalf("set service pending deploy: %v", err)
	}
	if err := db.SetServiceInstancePendingDeploy(ctx, "app", "main", "rev-main"); err != nil {
		t.Fatalf("set instance pending deploy: %v", err)
	}
	instances, err := db.ListServiceInstances(ctx, "app")
	if err != nil {
		t.Fatalf("list service instances: %v", err)
	}
	if len(instances) != 2 || instances[0].NodeID != "edge" || instances[0].PendingDeployRevision != "rev-1" || instances[1].NodeID != "main" || instances[1].PendingDeployRevision != "rev-main" {
		t.Fatalf("unexpected instances: %+v", instances)
	}
	if err := db.ClearServiceInstancePendingDeploy(ctx, "app", "main"); err != nil {
		t.Fatalf("clear instance pending deploy: %v", err)
	}
	main, err := db.GetServiceInstanceSnapshot(ctx, "app", "main")
	if err != nil {
		t.Fatalf("get main instance: %v", err)
	}
	if main.PendingDeployRevision != "" {
		t.Fatalf("main pending deploy = %q", main.PendingDeployRevision)
	}
	if err := db.ClearServicePendingDeploy(ctx, "app"); err != nil {
		t.Fatalf("clear service pending deploy: %v", err)
	}
	edge, err := db.GetServiceInstanceSnapshot(ctx, "app", "edge")
	if err != nil {
		t.Fatalf("get edge instance: %v", err)
	}
	if edge.PendingDeployRevision != "" {
		t.Fatalf("edge pending deploy = %q", edge.PendingDeployRevision)
	}
}

func TestListAndGetNodeSnapshots(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "edge"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	heartbeatAt := time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC)
	if err := db.RecordHeartbeat(ctx, NodeHeartbeat{NodeID: "main", HeartbeatAt: heartbeatAt}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	nodes, err := db.ListNodeSnapshots(ctx)
	if err != nil {
		t.Fatalf("list node snapshots: %v", err)
	}
	if len(nodes) != 2 || nodes[0].NodeID != "edge" || nodes[1].NodeID != "main" || !nodes[1].IsOnline {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}
	main, err := db.GetNodeSnapshot(ctx, "main")
	if err != nil {
		t.Fatalf("get main node: %v", err)
	}
	if main.LastHeartbeat != heartbeatAt.Format(time.RFC3339) {
		t.Fatalf("heartbeat = %q", main.LastHeartbeat)
	}
	if _, err := db.GetNodeSnapshot(ctx, "missing"); err == nil {
		t.Fatalf("expected missing node error")
	}
}

func TestUpsertServiceImageStates(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	checkedAt := time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC)
	if err := db.UpsertServiceImageStates(ctx, []ServiceImageState{{ServiceName: "app", NodeID: "main", ComposeService: "web", ImageRef: "ghcr.io/example/app:1", LocalDigest: "sha256:old", RemoteDigest: "sha256:new", LocalDigestObserved: true, RemoteDigestObserved: true, CheckedAt: checkedAt}}); err != nil {
		t.Fatalf("upsert image state: %v", err)
	}
	row := db.sql.QueryRowContext(ctx, `SELECT update_available, check_status, checked_at FROM service_image_states WHERE service_name = 'app' AND node_id = 'main'`)
	var updateAvailable bool
	var checkStatus string
	var rawCheckedAt string
	if err := row.Scan(&updateAvailable, &checkStatus, &rawCheckedAt); err != nil {
		t.Fatalf("scan image state: %v", err)
	}
	if !updateAvailable || checkStatus != ImageCheckStatusUnknown || rawCheckedAt != checkedAt.Format(time.RFC3339) {
		t.Fatalf("unexpected image state update_available=%v status=%q checked_at=%q", updateAvailable, checkStatus, rawCheckedAt)
	}
	if err := db.UpsertServiceImageStates(ctx, []ServiceImageState{{ServiceName: "app", NodeID: "main", ComposeService: "web", ImageRef: "ghcr.io/example/app:1", LocalDigest: "sha256:old", RemoteDigest: "sha256:newer", RemoteDigestObserved: true, CheckStatus: ImageCheckStatusOK, CheckedAt: checkedAt.Add(time.Minute)}}); err != nil {
		t.Fatalf("upsert remote-only image state: %v", err)
	}
	row = db.sql.QueryRowContext(ctx, `SELECT local_digest, remote_digest, update_available, check_status FROM service_image_states WHERE service_name = 'app' AND node_id = 'main'`)
	var localDigest string
	var remoteDigest string
	if err := row.Scan(&localDigest, &remoteDigest, &updateAvailable, &checkStatus); err != nil {
		t.Fatalf("scan updated image state: %v", err)
	}
	if localDigest != "sha256:old" || remoteDigest != "sha256:newer" || !updateAvailable || checkStatus != ImageCheckStatusOK {
		t.Fatalf("unexpected updated image state local=%q remote=%q available=%v status=%q", localDigest, remoteDigest, updateAvailable, checkStatus)
	}
}

func TestMigrateSetsSQLiteUserVersion(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	db, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	var version int
	if err := db.sql.QueryRowContext(context.Background(), `PRAGMA user_version;`).Scan(&version); err != nil {
		t.Fatalf("read sqlite user_version: %v", err)
	}
	if version != sqliteSchemaVersion {
		t.Fatalf("expected sqlite user_version %d, got %d", sqliteSchemaVersion, version)
	}
}

func TestMigrateVersionNineFailsLegacyRunningExecutions(t *testing.T) {
	t.Parallel()
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatal(err)
	}
	db, err := Open(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"alpha": {"main"}}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 12, 5, 0, 0, 0, time.UTC)
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "legacy-running", Type: task.TypeDeploy, Source: task.SourceSystem, ServiceName: "alpha", NodeID: "main", Status: task.StatusRunning, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertTaskStep(ctx, task.StepRecord{TaskID: "legacy-running", StepName: task.StepComposeUp, Status: task.StatusRunning}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "durable-running", Type: task.TypePrune, Source: task.SourceSystem, NodeID: "main", Status: task.StatusRunning, CreatedAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.ExecContext(ctx, `UPDATE tasks SET execution_id = 'execution-1', execution_state = 'accepted', lease_expires_at = '2026-07-12T05:10:00Z' WHERE task_id = 'durable-running'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.ExecContext(ctx, `PRAGMA user_version = 9`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	detail, err := db.GetTask(ctx, "legacy-running")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Record.Status != task.StatusFailed || detail.Record.FinishedAt == nil || detail.Record.ErrorSummary == "" {
		t.Fatalf("legacy task was not reconciled: %+v", detail.Record)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].Status != task.StatusFailed {
		t.Fatalf("legacy steps were not reconciled: %+v", detail.Steps)
	}
	instance, err := db.GetServiceInstanceSnapshot(ctx, "alpha", "main")
	if err != nil {
		t.Fatal(err)
	}
	if instance.RuntimeStatus != ServiceRuntimeError || instance.LastTaskID != "legacy-running" {
		t.Fatalf("legacy runtime was not reconciled: %+v", instance)
	}
	durable, err := db.GetTask(ctx, "durable-running")
	if err != nil {
		t.Fatal(err)
	}
	if durable.Record.Status != task.StatusRunning || durable.Record.ExecutionState != task.ExecutionAccepted {
		t.Fatalf("durable execution should survive migration: %+v", durable.Record)
	}
}

func TestMigrateFromVersionSevenDropsUnsupportedTasks(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	seedDB, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open seed sqlite db: %v", err)
	}
	if err := seedDB.Close(); err != nil {
		t.Fatalf("close seed sqlite db: %v", err)
	}

	legacySQL, err := sql.Open("sqlite", filepath.Join(stateDir, DatabaseFileName))
	if err != nil {
		t.Fatalf("open legacy sqlite db: %v", err)
	}
	legacyStatements := []string{
		`PRAGMA foreign_keys = OFF;`,
		`CREATE TABLE tasks_legacy (
			task_id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			source TEXT NOT NULL,
			triggered_by TEXT,
			service_name TEXT,
			node_id TEXT,
			status TEXT NOT NULL,
			params_json TEXT,
			log_path TEXT,
			repo_revision TEXT,
			result_revision TEXT,
			attempt_of_task_id TEXT,
			created_at TEXT NOT NULL,
			started_at TEXT,
			finished_at TEXT,
			error_summary TEXT
		);`,
		`INSERT INTO tasks_legacy SELECT task_id, type, source, triggered_by, service_name, node_id, status, params_json, log_path, repo_revision, result_revision, attempt_of_task_id, created_at, started_at, finished_at, error_summary FROM tasks;`,
		`DROP TABLE tasks;`,
		`ALTER TABLE tasks_legacy RENAME TO tasks;`,
		`INSERT INTO nodes (node_id, is_configured, is_online) VALUES ('main', 1, 0);`,
		`INSERT INTO services (service_name, is_declared, runtime_status, last_task_id, updated_at) VALUES ('app', 1, 'unknown', 'task-old-list', '2026-06-02T09:00:00Z');`,
		`INSERT INTO service_instances (service_name, node_id, is_declared, runtime_status, last_task_id, updated_at) VALUES ('app', 'main', 1, 'unknown', 'task-old-inspect', '2026-06-02T09:00:00Z');`,
		`INSERT INTO tasks (task_id, type, source, service_name, node_id, status, attempt_of_task_id, created_at) VALUES
			('task-old-remove', 'docker_remove', 'cli', 'app', 'main', 'succeeded', NULL, '2026-06-02T09:00:00Z'),
			('task-old-list', 'docker_list', 'cli', 'app', 'main', 'succeeded', NULL, '2026-06-02T09:01:00Z'),
			('task-old-inspect', 'docker_inspect', 'cli', 'app', 'main', 'succeeded', NULL, '2026-06-02T09:02:00Z'),
			('task-valid', 'deploy', 'cli', 'app', 'main', 'succeeded', 'task-old-remove', '2026-06-02T09:03:00Z');`,
		`INSERT INTO task_steps (task_id, step_name, status) VALUES
			('task-old-remove', 'docker_remove', 'succeeded'),
			('task-old-list', 'docker_list', 'succeeded'),
			('task-old-inspect', 'docker_inspect', 'succeeded'),
			('task-valid', 'compose_up', 'succeeded');`,
		`PRAGMA user_version = 7;`,
		`PRAGMA foreign_keys = ON;`,
	}
	for _, statement := range legacyStatements {
		if _, err := legacySQL.ExecContext(context.Background(), statement); err != nil {
			_ = legacySQL.Close()
			t.Fatalf("apply legacy setup statement %q: %v", statement, err)
		}
	}
	if err := legacySQL.Close(); err != nil {
		t.Fatalf("close legacy sqlite db: %v", err)
	}

	db, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open migrated sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	var unsupportedTaskCount int
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE type IN ('docker_remove', 'docker_list', 'docker_inspect');`).Scan(&unsupportedTaskCount); err != nil {
		t.Fatalf("count legacy tasks: %v", err)
	}
	if unsupportedTaskCount != 0 {
		t.Fatalf("expected unsupported legacy tasks to be dropped, got %d", unsupportedTaskCount)
	}
	var lastTaskID sql.NullString
	if err := db.sql.QueryRowContext(ctx, `SELECT last_task_id FROM services WHERE service_name = 'app';`).Scan(&lastTaskID); err != nil {
		t.Fatalf("read service last_task_id: %v", err)
	}
	if lastTaskID.Valid {
		t.Fatalf("expected service last_task_id to be cleared, got %q", lastTaskID.String)
	}
	if err := db.sql.QueryRowContext(ctx, `SELECT last_task_id FROM service_instances WHERE service_name = 'app' AND node_id = 'main';`).Scan(&lastTaskID); err != nil {
		t.Fatalf("read service instance last_task_id: %v", err)
	}
	if lastTaskID.Valid {
		t.Fatalf("expected service instance last_task_id to be cleared, got %q", lastTaskID.String)
	}
	var preservedTaskCount int
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE task_id = 'task-valid' AND type = 'deploy' AND attempt_of_task_id IS NULL;`).Scan(&preservedTaskCount); err != nil {
		t.Fatalf("count preserved task: %v", err)
	}
	if preservedTaskCount != 1 {
		t.Fatalf("expected valid task to be preserved with cleared attempt_of_task_id, got %d", preservedTaskCount)
	}
}

func TestMigrateBackfillsBackupNodeIDAndEnforcesInstanceIntegrity(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	seedDB, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open seed sqlite db: %v", err)
	}
	if err := seedDB.Close(); err != nil {
		t.Fatalf("close seed sqlite db: %v", err)
	}
	legacySQL, err := sql.Open("sqlite", filepath.Join(stateDir, DatabaseFileName))
	if err != nil {
		t.Fatalf("open legacy sqlite db: %v", err)
	}
	for _, statement := range []string{
		`PRAGMA foreign_keys = OFF;`,
		`DROP TABLE backups;`,
		`CREATE TABLE backups (
			backup_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			service_name TEXT NOT NULL,
			data_name TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			artifact_ref TEXT,
			error_summary TEXT
		);`,
		`PRAGMA user_version = 2;`,
		`PRAGMA foreign_keys = ON;`,
	} {
		if _, err := legacySQL.ExecContext(context.Background(), statement); err != nil {
			_ = legacySQL.Close()
			t.Fatalf("apply legacy setup statement %q: %v", statement, err)
		}
	}
	if err := legacySQL.Close(); err != nil {
		t.Fatalf("close legacy sqlite db: %v", err)
	}

	db, err := Open(stateDir)
	if err != nil {
		t.Fatalf("open migrated sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.sql.QueryContext(context.Background(), `PRAGMA table_info(backups);`)
	if err != nil {
		t.Fatalf("read backups columns: %v", err)
	}
	defer func() { _ = rows.Close() }()
	foundNodeID := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan backups column: %v", err)
		}
		if name == "node_id" {
			foundNodeID = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate backups columns: %v", err)
	}
	if !foundNodeID {
		t.Fatal("expected migration to add backups.node_id")
	}
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "edge"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"alpha": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", NodeID: "main", Status: task.StatusSucceeded, CreatedAt: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create backup task: %v", err)
	}

	if _, err := db.sql.ExecContext(ctx, `
		INSERT INTO backups (backup_id, task_id, service_name, node_id, data_name, status, started_at)
		VALUES ('backup-invalid', 'task-1', 'alpha', 'edge', 'config', 'succeeded', '2026-04-04T12:05:00Z')
	`); err == nil {
		t.Fatal("expected migrated backups table to enforce service instance integrity")
	}
}
