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

type ServiceSummary struct {
	Name          string
	IsDeclared    bool
	RuntimeStatus string
	UpdatedAt     string
}

type ServiceSnapshot struct {
	Name          string
	IsDeclared    bool
	RuntimeStatus string
	UpdatedAt     string
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

func (db *DB) SyncDeclaredServices(ctx context.Context, serviceNames []string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin service sync transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE services SET is_declared = 0`); err != nil {
		return fmt.Errorf("mark services undeclared: %w", err)
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	for _, serviceName := range serviceNames {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO services (service_name, is_declared, runtime_status, updated_at)
			VALUES (?, 1, 'unknown', ?)
			ON CONFLICT(service_name) DO UPDATE SET
				is_declared = 1,
				updated_at = excluded.updated_at
		`, serviceName, updatedAt); err != nil {
			return fmt.Errorf("upsert declared service %q: %w", serviceName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit service sync transaction: %w", err)
	}
	return nil
}

func (db *DB) ListDeclaredServices(ctx context.Context, runtimeStatusFilter, cursor string, limit uint32) ([]ServiceSummary, string, error) {
	if limit == 0 {
		limit = 100
	}

	query := `
		SELECT service_name, is_declared, runtime_status, updated_at
		FROM services
		WHERE is_declared = 1
	`
	args := make([]any, 0, 3)

	if runtimeStatusFilter != "" {
		query += ` AND runtime_status = ?`
		args = append(args, runtimeStatusFilter)
	}
	if cursor != "" {
		query += ` AND service_name > ?`
		args = append(args, cursor)
	}

	query += ` ORDER BY service_name ASC LIMIT ?`
	args = append(args, limit+1)

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list declared services: %w", err)
	}
	defer rows.Close()

	services := make([]ServiceSummary, 0, limit)
	var nextCursor string
	for rows.Next() {
		var service ServiceSummary
		if err := rows.Scan(&service.Name, &service.IsDeclared, &service.RuntimeStatus, &service.UpdatedAt); err != nil {
			return nil, "", fmt.Errorf("scan declared service: %w", err)
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate declared services: %w", err)
	}

	if uint32(len(services)) > limit {
		nextCursor = services[limit-1].Name
		services = services[:limit]
	}

	return services, nextCursor, nil
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

func (db *DB) UpdateServiceRuntimeStatus(ctx context.Context, serviceName, runtimeStatus string, updatedAt time.Time) error {
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}
	if !IsValidServiceRuntimeStatus(runtimeStatus) {
		return fmt.Errorf("invalid service runtime status %q", runtimeStatus)
	}
	result, err := db.sql.ExecContext(ctx, `
		UPDATE services
		SET runtime_status = ?, updated_at = ?
		WHERE service_name = ?
	`, runtimeStatus, updatedAt.UTC().Format(time.RFC3339), serviceName)
	if err != nil {
		return fmt.Errorf("update runtime status for service %q: %w", serviceName, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated runtime status rows for service %q: %w", serviceName, err)
	}
	if affected == 0 {
		return ErrServiceNotFound
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
		`CREATE INDEX IF NOT EXISTS idx_backups_service_finished_at ON backups(service_name, finished_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_backups_status_finished_at ON backups(status, finished_at DESC);`,
	}

	for _, statement := range statements {
		if _, err := db.sql.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply sqlite schema statement: %w", err)
		}
	}

	return nil
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
