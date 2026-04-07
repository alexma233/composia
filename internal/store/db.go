package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const DatabaseFileName = "composia.db"

var ErrServiceNotFound = errors.New("service not found")

type DB struct {
	sql     *sql.DB
	path    string
	claimMu sync.Mutex
}

type RepoSyncState struct {
	SyncStatus           string
	LastSyncError        string
	LastSuccessfulPullAt string
}

const (
	RepoSyncStatusUnknown    = "unknown"
	RepoSyncStatusLocalOnly  = "local_only"
	RepoSyncStatusSynced     = "synced"
	RepoSyncStatusPullFailed = "pull_failed"
	RepoSyncStatusPushFailed = "push_failed"
)

type NodeHeartbeat struct {
	NodeID              string
	HeartbeatAt         time.Time
	AgentVersion        string
	DockerServerVersion string
	DiskTotalBytes      uint64
	DiskFreeBytes       uint64
}

type DockerStats struct {
	NodeID              string
	ContainersTotal     uint32
	ContainersRunning   uint32
	ContainersStopped   uint32
	ContainersPaused    uint32
	Images              uint32
	Networks            uint32
	Volumes             uint32
	VolumesSizeBytes    uint64
	DisksUsageBytes     uint64
	DockerServerVersion string
	ReportedAt          time.Time
}

type ServiceSummary struct {
	Name            string
	IsDeclared      bool
	RuntimeStatus   string
	UpdatedAt       string
	InstanceCount   uint32
	RunningCount    uint32
	TargetNodeCount uint32
}

type ServiceSnapshot struct {
	Name            string
	IsDeclared      bool
	RuntimeStatus   string
	UpdatedAt       string
	InstanceCount   uint32
	RunningCount    uint32
	TargetNodeCount uint32
}

type ServiceInstanceSnapshot struct {
	ServiceName   string
	NodeID        string
	IsDeclared    bool
	RuntimeStatus string
	UpdatedAt     string
	LastTaskID    string
}

type NodeSnapshot struct {
	NodeID        string
	IsConfigured  bool
	IsOnline      bool
	LastHeartbeat string
}

const (
	ServiceRuntimeRunning = "running"
	ServiceRuntimeStopped = "stopped"
	ServiceRuntimeError   = "error"
	ServiceRuntimeUnknown = "unknown"
)

func Open(stateDir string) (*DB, error) {
	databasePath := filepath.Join(stateDir, DatabaseFileName)
	sqlDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %q: %w", databasePath, err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	for _, pragma := range []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
	} {
		if _, err := sqlDB.ExecContext(context.Background(), pragma); err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("initialize sqlite pragma %q for %q: %w", pragma, databasePath, err)
		}
	}

	db := &DB{sql: sqlDB, path: databasePath}
	if err := db.migrate(context.Background()); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) Path() string {
	return db.path
}

func (db *DB) SQL() *sql.DB {
	return db.sql
}

func (db *DB) SyncConfiguredNodes(ctx context.Context, nodeIDs []string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin node sync transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE nodes SET is_configured = 0`); err != nil {
		return fmt.Errorf("mark nodes unconfigured: %w", err)
	}

	for _, nodeID := range nodeIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO nodes (node_id, is_configured, is_online)
			VALUES (?, 1, 0)
			ON CONFLICT(node_id) DO UPDATE SET is_configured = 1
		`, nodeID); err != nil {
			return fmt.Errorf("upsert configured node %q: %w", nodeID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit node sync transaction: %w", err)
	}
	return nil
}

func (db *DB) RecordHeartbeat(ctx context.Context, heartbeat NodeHeartbeat) error {
	if _, err := db.sql.ExecContext(ctx, `
		INSERT INTO nodes (
			node_id,
			is_configured,
			is_online,
			last_heartbeat,
			agent_version,
			docker_server_version,
			disk_total_bytes,
			disk_free_bytes
		)
		VALUES (?, 1, 1, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			is_online = 1,
			last_heartbeat = excluded.last_heartbeat,
			agent_version = excluded.agent_version,
			docker_server_version = excluded.docker_server_version,
			disk_total_bytes = excluded.disk_total_bytes,
			disk_free_bytes = excluded.disk_free_bytes
	`,
		heartbeat.NodeID,
		heartbeat.HeartbeatAt.UTC().Format(time.RFC3339),
		heartbeat.AgentVersion,
		heartbeat.DockerServerVersion,
		heartbeat.DiskTotalBytes,
		heartbeat.DiskFreeBytes,
	); err != nil {
		return fmt.Errorf("record heartbeat for node %q: %w", heartbeat.NodeID, err)
	}
	return nil
}

func (db *DB) MarkOfflineNodesBefore(ctx context.Context, cutoff time.Time) error {
	if _, err := db.sql.ExecContext(ctx, `
		UPDATE nodes
		SET is_online = 0
		WHERE last_heartbeat IS NULL OR last_heartbeat < ?
	`, cutoff.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("mark offline nodes before %s: %w", cutoff.UTC().Format(time.RFC3339), err)
	}
	return nil
}

func (db *DB) NodeCounts(ctx context.Context) (uint64, uint64, error) {
	var configured uint64
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes WHERE is_configured = 1`).Scan(&configured); err != nil {
		return 0, 0, fmt.Errorf("count configured nodes: %w", err)
	}

	var online uint64
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes WHERE is_configured = 1 AND is_online = 1`).Scan(&online); err != nil {
		return 0, 0, fmt.Errorf("count online nodes: %w", err)
	}

	return configured, online, nil
}

func (db *DB) SyncDeclaredServices(ctx context.Context, services map[string][]string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin service sync transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE services SET is_declared = 0`); err != nil {
		return fmt.Errorf("mark services undeclared: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE service_instances SET is_declared = 0`); err != nil {
		return fmt.Errorf("mark service instances undeclared: %w", err)
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	for serviceName, nodeIDs := range services {
		for _, nodeID := range nodeIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO nodes (node_id, is_configured, is_online)
				VALUES (?, 0, 0)
				ON CONFLICT(node_id) DO NOTHING
			`, nodeID); err != nil {
				return fmt.Errorf("ensure node %q for declared service %q: %w", nodeID, serviceName, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO services (service_name, is_declared, runtime_status, updated_at)
			VALUES (?, 1, 'unknown', ?)
			ON CONFLICT(service_name) DO UPDATE SET
				is_declared = 1,
				updated_at = excluded.updated_at
		`, serviceName, updatedAt); err != nil {
			return fmt.Errorf("upsert declared service %q: %w", serviceName, err)
		}
		for _, nodeID := range nodeIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO service_instances (service_name, node_id, is_declared, runtime_status, updated_at)
				VALUES (?, ?, 1, 'unknown', ?)
				ON CONFLICT(service_name, node_id) DO UPDATE SET
					is_declared = 1,
					updated_at = excluded.updated_at
			`, serviceName, nodeID, updatedAt); err != nil {
				return fmt.Errorf("upsert declared service instance %q@%q: %w", serviceName, nodeID, err)
			}
		}
		if err := refreshServiceAggregateStatusTx(ctx, tx, serviceName); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit service sync transaction: %w", err)
	}
	return nil
}

func (db *DB) ListDeclaredServices(ctx context.Context, runtimeStatusFilter string, page, limit uint32) ([]ServiceSummary, uint32, error) {
	if limit == 0 {
		limit = 100
	}
	if page == 0 {
		page = 1
	}

	whereClause := "WHERE is_declared = 1"
	args := make([]any, 0, 3)

	if runtimeStatusFilter != "" {
		whereClause += ` AND runtime_status = ?`
		args = append(args, runtimeStatusFilter)
	}

	var totalCount uint32
	countQuery := `SELECT COUNT(*) FROM services ` + whereClause
	if err := db.sql.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count declared services: %w", err)
	}

	offset := (page - 1) * limit
	query := `SELECT service_name, is_declared, runtime_status, updated_at FROM services ` + whereClause
	query += ` ORDER BY service_name ASC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list declared services: %w", err)
	}
	defer rows.Close()

	services := make([]ServiceSummary, 0, limit)
	for rows.Next() {
		var service ServiceSummary
		if err := rows.Scan(&service.Name, &service.IsDeclared, &service.RuntimeStatus, &service.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan declared service: %w", err)
		}
		if err := db.sql.QueryRowContext(ctx, `
			SELECT COUNT(*),
			       SUM(CASE WHEN runtime_status = ? THEN 1 ELSE 0 END),
			       SUM(CASE WHEN is_declared = 1 THEN 1 ELSE 0 END)
			FROM service_instances
			WHERE service_name = ?
		`, ServiceRuntimeRunning, service.Name).Scan(&service.InstanceCount, &service.RunningCount, &service.TargetNodeCount); err != nil {
			return nil, 0, fmt.Errorf("read declared service instance counts for %q: %w", service.Name, err)
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate declared services: %w", err)
	}

	return services, totalCount, nil
}

func (db *DB) GetServiceSnapshot(ctx context.Context, serviceName string) (ServiceSnapshot, error) {
	var snapshot ServiceSnapshot
	err := db.sql.QueryRowContext(ctx, `
		SELECT service_name, is_declared, runtime_status, updated_at
		FROM services
		WHERE service_name = ?
	`, serviceName).Scan(&snapshot.Name, &snapshot.IsDeclared, &snapshot.RuntimeStatus, &snapshot.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ServiceSnapshot{}, ErrServiceNotFound
		}
		return ServiceSnapshot{}, fmt.Errorf("get service snapshot %q: %w", serviceName, err)
	}
	if err := db.sql.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       SUM(CASE WHEN runtime_status = ? THEN 1 ELSE 0 END),
		       SUM(CASE WHEN is_declared = 1 THEN 1 ELSE 0 END)
		FROM service_instances
		WHERE service_name = ?
	`, ServiceRuntimeRunning, serviceName).Scan(&snapshot.InstanceCount, &snapshot.RunningCount, &snapshot.TargetNodeCount); err != nil {
		return ServiceSnapshot{}, fmt.Errorf("get service snapshot instance counts %q: %w", serviceName, err)
	}
	return snapshot, nil
}

func (db *DB) ListServiceInstances(ctx context.Context, serviceName string) ([]ServiceInstanceSnapshot, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT service_name, node_id, is_declared, runtime_status, COALESCE(updated_at, ''), COALESCE(last_task_id, '')
		FROM service_instances
		WHERE service_name = ?
		ORDER BY node_id ASC
	`, serviceName)
	if err != nil {
		return nil, fmt.Errorf("list service instances for %q: %w", serviceName, err)
	}
	defer rows.Close()

	instances := make([]ServiceInstanceSnapshot, 0)
	for rows.Next() {
		var snapshot ServiceInstanceSnapshot
		if err := rows.Scan(&snapshot.ServiceName, &snapshot.NodeID, &snapshot.IsDeclared, &snapshot.RuntimeStatus, &snapshot.UpdatedAt, &snapshot.LastTaskID); err != nil {
			return nil, fmt.Errorf("scan service instance for %q: %w", serviceName, err)
		}
		instances = append(instances, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service instances for %q: %w", serviceName, err)
	}
	return instances, nil
}

func (db *DB) GetServiceInstanceSnapshot(ctx context.Context, serviceName, nodeID string) (ServiceInstanceSnapshot, error) {
	var snapshot ServiceInstanceSnapshot
	err := db.sql.QueryRowContext(ctx, `
		SELECT service_name, node_id, is_declared, runtime_status, COALESCE(updated_at, ''), COALESCE(last_task_id, '')
		FROM service_instances
		WHERE service_name = ? AND node_id = ?
	`, serviceName, nodeID).Scan(&snapshot.ServiceName, &snapshot.NodeID, &snapshot.IsDeclared, &snapshot.RuntimeStatus, &snapshot.UpdatedAt, &snapshot.LastTaskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ServiceInstanceSnapshot{}, ErrServiceNotFound
		}
		return ServiceInstanceSnapshot{}, fmt.Errorf("get service instance snapshot %q@%q: %w", serviceName, nodeID, err)
	}
	return snapshot, nil
}

func (db *DB) ListNodeSnapshots(ctx context.Context) ([]NodeSnapshot, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT node_id, is_configured, is_online, COALESCE(last_heartbeat, '')
		FROM nodes
		ORDER BY node_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list node snapshots: %w", err)
	}
	defer rows.Close()

	nodes := make([]NodeSnapshot, 0)
	for rows.Next() {
		var node NodeSnapshot
		if err := rows.Scan(&node.NodeID, &node.IsConfigured, &node.IsOnline, &node.LastHeartbeat); err != nil {
			return nil, fmt.Errorf("scan node snapshot: %w", err)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node snapshots: %w", err)
	}
	return nodes, nil
}

func (db *DB) GetNodeSnapshot(ctx context.Context, nodeID string) (NodeSnapshot, error) {
	var snapshot NodeSnapshot
	err := db.sql.QueryRowContext(ctx, `
		SELECT node_id, is_configured, is_online, COALESCE(last_heartbeat, '')
		FROM nodes
		WHERE node_id = ?
	`, nodeID).Scan(&snapshot.NodeID, &snapshot.IsConfigured, &snapshot.IsOnline, &snapshot.LastHeartbeat)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return NodeSnapshot{}, fmt.Errorf("node %q not found", nodeID)
		}
		return NodeSnapshot{}, fmt.Errorf("get node snapshot %q: %w", nodeID, err)
	}
	return snapshot, nil
}

func (db *DB) UpdateServiceInstanceRuntimeStatus(ctx context.Context, serviceName, nodeID, runtimeStatus string, updatedAt time.Time) error {
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}
	if nodeID == "" {
		return fmt.Errorf("node id is required")
	}
	if !IsValidServiceRuntimeStatus(runtimeStatus) {
		return fmt.Errorf("invalid service runtime status %q", runtimeStatus)
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin service instance runtime update for %q@%q: %w", serviceName, nodeID, err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE service_instances
		SET runtime_status = ?, updated_at = ?
		WHERE service_name = ? AND node_id = ?
	`, runtimeStatus, updatedAt.UTC().Format(time.RFC3339), serviceName, nodeID)
	if err != nil {
		return fmt.Errorf("update runtime status for service instance %q@%q: %w", serviceName, nodeID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated runtime status rows for service instance %q@%q: %w", serviceName, nodeID, err)
	}
	if affected == 0 {
		return ErrServiceNotFound
	}
	if err := refreshServiceAggregateStatusTx(ctx, tx, serviceName); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit service instance runtime update for %q@%q: %w", serviceName, nodeID, err)
	}
	return nil
}

func IsValidServiceRuntimeStatus(runtimeStatus string) bool {
	switch runtimeStatus {
	case ServiceRuntimeRunning, ServiceRuntimeStopped, ServiceRuntimeError, ServiceRuntimeUnknown:
		return true
	default:
		return false
	}
}

func (db *DB) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS nodes (
			node_id TEXT PRIMARY KEY,
			is_configured INTEGER NOT NULL,
			is_online INTEGER NOT NULL,
			last_heartbeat TEXT,
			agent_version TEXT,
			docker_server_version TEXT,
			disk_total_bytes INTEGER,
			disk_free_bytes INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS services (
			service_name TEXT PRIMARY KEY,
			is_declared INTEGER NOT NULL,
			runtime_status TEXT NOT NULL,
			last_task_id TEXT,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (last_task_id) REFERENCES tasks(task_id)
		);`,
		`CREATE TABLE IF NOT EXISTS service_instances (
			service_name TEXT NOT NULL,
			node_id TEXT NOT NULL,
			is_declared INTEGER NOT NULL,
			runtime_status TEXT NOT NULL,
			last_task_id TEXT,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (service_name, node_id),
			FOREIGN KEY (service_name) REFERENCES services(service_name),
			FOREIGN KEY (node_id) REFERENCES nodes(node_id),
			FOREIGN KEY (last_task_id) REFERENCES tasks(task_id)
		);`,
		`CREATE TABLE IF NOT EXISTS tasks (
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
			error_summary TEXT,
			FOREIGN KEY (service_name) REFERENCES services(service_name),
			FOREIGN KEY (node_id) REFERENCES nodes(node_id),
			FOREIGN KEY (attempt_of_task_id) REFERENCES tasks(task_id)
		);`,
		`CREATE TABLE IF NOT EXISTS task_steps (
			task_id TEXT NOT NULL,
			step_name TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT,
			finished_at TEXT,
			PRIMARY KEY (task_id, step_name),
			FOREIGN KEY (task_id) REFERENCES tasks(task_id)
		);`,
		`CREATE TABLE IF NOT EXISTS backups (
			backup_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			service_name TEXT NOT NULL,
			data_name TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			artifact_ref TEXT,
			error_summary TEXT,
			FOREIGN KEY (task_id) REFERENCES tasks(task_id),
			FOREIGN KEY (service_name) REFERENCES services(service_name)
		);`,
		`CREATE TABLE IF NOT EXISTS repo_state (
			singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
			sync_status TEXT NOT NULL,
			last_sync_error TEXT,
			last_successful_pull_at TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status_created_at ON tasks(status, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_service_created_at ON tasks(service_name, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_node_created_at ON tasks(node_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_services_declared_runtime_status ON services(is_declared, runtime_status);`,
		`CREATE INDEX IF NOT EXISTS idx_service_instances_service_node ON service_instances(service_name, node_id);`,
		`CREATE INDEX IF NOT EXISTS idx_service_instances_service_runtime_status ON service_instances(service_name, runtime_status);`,
		`CREATE INDEX IF NOT EXISTS idx_backups_service_finished_at ON backups(service_name, finished_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_backups_status_finished_at ON backups(status, finished_at DESC);`,
		`CREATE TABLE IF NOT EXISTS docker_stats (
			node_id TEXT PRIMARY KEY,
			containers_total INTEGER NOT NULL,
			containers_running INTEGER NOT NULL,
			containers_stopped INTEGER NOT NULL,
			containers_paused INTEGER NOT NULL,
			images INTEGER NOT NULL,
			networks INTEGER NOT NULL,
			volumes INTEGER NOT NULL,
			volumes_size_bytes INTEGER NOT NULL,
			disks_usage_bytes INTEGER NOT NULL,
			docker_server_version TEXT NOT NULL,
			reported_at TEXT NOT NULL,
			FOREIGN KEY (node_id) REFERENCES nodes(node_id)
		);`,
	}

	for _, statement := range statements {
		if _, err := db.sql.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply sqlite schema statement: %w", err)
		}
	}

	return nil
}

func refreshServiceAggregateStatusTx(ctx context.Context, tx *sql.Tx, serviceName string) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT runtime_status
		FROM service_instances
		WHERE service_name = ? AND is_declared = 1
	`, serviceName)
	if err != nil {
		return fmt.Errorf("list service instance runtime states for %q: %w", serviceName, err)
	}
	defer rows.Close()

	statuses := make([]string, 0)
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return fmt.Errorf("scan service instance runtime state for %q: %w", serviceName, err)
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate service instance runtime states for %q: %w", serviceName, err)
	}

	aggregate := ServiceRuntimeUnknown
	switch {
	case len(statuses) == 0:
		aggregate = ServiceRuntimeUnknown
	case allStatusesEqual(statuses, ServiceRuntimeRunning):
		aggregate = ServiceRuntimeRunning
	case allStatusesEqual(statuses, ServiceRuntimeStopped):
		aggregate = ServiceRuntimeStopped
	case hasStatus(statuses, ServiceRuntimeError):
		aggregate = ServiceRuntimeError
	default:
		aggregate = ServiceRuntimeUnknown
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE services
		SET runtime_status = ?
		WHERE service_name = ?
	`, aggregate, serviceName); err != nil {
		return fmt.Errorf("refresh service aggregate status for %q: %w", serviceName, err)
	}
	return nil
}

func allStatusesEqual(statuses []string, expected string) bool {
	if len(statuses) == 0 {
		return false
	}
	for _, status := range statuses {
		if status != expected {
			return false
		}
	}
	return true
}

func hasStatus(statuses []string, expected string) bool {
	for _, status := range statuses {
		if status == expected {
			return true
		}
	}
	return false
}

func (db *DB) GetRepoSyncState(ctx context.Context) (RepoSyncState, error) {
	var state RepoSyncState
	err := db.sql.QueryRowContext(ctx, `
		SELECT sync_status, COALESCE(last_sync_error, ''), COALESCE(last_successful_pull_at, '')
		FROM repo_state
		WHERE singleton_id = 1
	`).Scan(&state.SyncStatus, &state.LastSyncError, &state.LastSuccessfulPullAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RepoSyncState{}, nil
	}
	if err != nil {
		return RepoSyncState{}, fmt.Errorf("get repo sync state: %w", err)
	}
	return state, nil
}

func (db *DB) UpsertRepoSyncState(ctx context.Context, state RepoSyncState) error {
	if state.SyncStatus == "" {
		state.SyncStatus = RepoSyncStatusUnknown
	}
	if _, err := db.sql.ExecContext(ctx, `
		INSERT INTO repo_state (singleton_id, sync_status, last_sync_error, last_successful_pull_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(singleton_id) DO UPDATE SET
			sync_status = excluded.sync_status,
			last_sync_error = excluded.last_sync_error,
			last_successful_pull_at = excluded.last_successful_pull_at
	`, state.SyncStatus, nullableString(state.LastSyncError), nullableString(state.LastSuccessfulPullAt)); err != nil {
		return fmt.Errorf("upsert repo sync state: %w", err)
	}
	return nil
}

func (db *DB) RecordDockerStats(ctx context.Context, stats DockerStats) error {
	if _, err := db.sql.ExecContext(ctx, `
		INSERT INTO docker_stats (
			node_id, containers_total, containers_running, containers_stopped,
			containers_paused, images, networks, volumes, volumes_size_bytes,
			disks_usage_bytes, docker_server_version, reported_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			containers_total = excluded.containers_total,
			containers_running = excluded.containers_running,
			containers_stopped = excluded.containers_stopped,
			containers_paused = excluded.containers_paused,
			images = excluded.images,
			networks = excluded.networks,
			volumes = excluded.volumes,
			volumes_size_bytes = excluded.volumes_size_bytes,
			disks_usage_bytes = excluded.disks_usage_bytes,
			docker_server_version = excluded.docker_server_version,
			reported_at = excluded.reported_at
	`, stats.NodeID, stats.ContainersTotal, stats.ContainersRunning, stats.ContainersStopped,
		stats.ContainersPaused, stats.Images, stats.Networks, stats.Volumes,
		stats.VolumesSizeBytes, stats.DisksUsageBytes, stats.DockerServerVersion,
		stats.ReportedAt.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("record docker stats: %w", err)
	}
	return nil
}

func (db *DB) GetNodeDockerStats(ctx context.Context, nodeID string) (DockerStats, error) {
	var stats DockerStats
	var reportedAt string
	err := db.sql.QueryRowContext(ctx, `
		SELECT node_id, containers_total, containers_running, containers_stopped,
			containers_paused, images, networks, volumes, volumes_size_bytes,
			disks_usage_bytes, docker_server_version, reported_at
		FROM docker_stats
		WHERE node_id = ?
	`, nodeID).Scan(&stats.NodeID, &stats.ContainersTotal, &stats.ContainersRunning,
		&stats.ContainersStopped, &stats.ContainersPaused, &stats.Images, &stats.Networks,
		&stats.Volumes, &stats.VolumesSizeBytes, &stats.DisksUsageBytes,
		&stats.DockerServerVersion, &reportedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DockerStats{}, nil
	}
	if err != nil {
		return DockerStats{}, fmt.Errorf("get docker stats for node %q: %w", nodeID, err)
	}
	stats.ReportedAt, _ = time.Parse(time.RFC3339, reportedAt)
	return stats, nil
}
