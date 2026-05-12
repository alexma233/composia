package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const DatabaseFileName = "composia.db"

const sqliteSchemaVersion = 3

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

type ServiceImageState struct {
	ServiceName          string
	NodeID               string
	ComposeService       string
	ImageRef             string
	LocalDigest          string
	RemoteDigest         string
	LocalDigestObserved  bool
	RemoteDigestObserved bool
	UpdateAvailable      bool
	CheckStatus          string
	ErrorSummary         string
	CheckedAt            time.Time
	UpdatedAt            time.Time
}

type ServiceImageUpdateCheck struct {
	ServiceName       string
	NodeID            string
	ImageName         string
	ImageRef          string
	PolicyType        string
	CurrentValue      string
	CurrentTag        string
	CurrentDigest     string
	CandidateTag      string
	CandidateDigest   string
	CandidateTagsJSON string
	UpdateAvailable   bool
	CheckStatus       string
	ErrorSummary      string
	CheckedAt         time.Time
	UpdatedAt         time.Time
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
	ServiceName           string
	NodeID                string
	IsDeclared            bool
	RuntimeStatus         string
	UpdatedAt             string
	LastTaskID            string
	PendingDeployRevision string
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

const (
	ImageCheckStatusUnknown = "unknown"
	ImageCheckStatusOK      = "ok"
	ImageCheckStatusError   = "error"
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
	defer func() { _ = tx.Rollback() }()

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
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE services SET is_declared = 0`); err != nil {
		return fmt.Errorf("mark services undeclared: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE service_instances SET is_declared = 0, pending_deploy_revision = NULL`); err != nil {
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

	whereClause := "WHERE services.is_declared = 1"
	args := make([]any, 0, 3)

	if runtimeStatusFilter != "" {
		whereClause += ` AND services.runtime_status = ?`
		args = append(args, runtimeStatusFilter)
	}

	var totalCount uint32
	countQuery := `SELECT COUNT(*) FROM services ` + whereClause
	if err := db.sql.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count declared services: %w", err)
	}

	offset := (page - 1) * limit
	query := `
		SELECT
			services.service_name,
			services.is_declared,
			services.runtime_status,
			services.updated_at,
			COUNT(service_instances.node_id) AS instance_count,
			COALESCE(SUM(CASE WHEN service_instances.runtime_status = ? THEN 1 ELSE 0 END), 0) AS running_count,
			COALESCE(SUM(CASE WHEN service_instances.is_declared = 1 THEN 1 ELSE 0 END), 0) AS target_node_count
		FROM services
		LEFT JOIN service_instances ON service_instances.service_name = services.service_name
		` + whereClause + `
		GROUP BY services.service_name, services.is_declared, services.runtime_status, services.updated_at
		ORDER BY services.service_name ASC
		LIMIT ? OFFSET ?`
	queryArgs := make([]any, 0, len(args)+3)
	queryArgs = append(queryArgs, ServiceRuntimeRunning)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := db.sql.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list declared services: %w", err)
	}
	defer func() { _ = rows.Close() }()

	services := make([]ServiceSummary, 0, limit)
	for rows.Next() {
		var service ServiceSummary
		if err := rows.Scan(
			&service.Name,
			&service.IsDeclared,
			&service.RuntimeStatus,
			&service.UpdatedAt,
			&service.InstanceCount,
			&service.RunningCount,
			&service.TargetNodeCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan declared service: %w", err)
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
		       COALESCE(SUM(CASE WHEN runtime_status = ? THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN is_declared = 1 THEN 1 ELSE 0 END), 0)
		FROM service_instances
		WHERE service_name = ?
	`, ServiceRuntimeRunning, serviceName).Scan(&snapshot.InstanceCount, &snapshot.RunningCount, &snapshot.TargetNodeCount); err != nil {
		return ServiceSnapshot{}, fmt.Errorf("get service snapshot instance counts %q: %w", serviceName, err)
	}
	return snapshot, nil
}

func (db *DB) ListServiceInstances(ctx context.Context, serviceName string) ([]ServiceInstanceSnapshot, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT service_name, node_id, is_declared, runtime_status, COALESCE(updated_at, ''), COALESCE(last_task_id, ''), COALESCE(pending_deploy_revision, '')
		FROM service_instances
		WHERE service_name = ?
		ORDER BY node_id ASC
	`, serviceName)
	if err != nil {
		return nil, fmt.Errorf("list service instances for %q: %w", serviceName, err)
	}
	defer func() { _ = rows.Close() }()

	instances := make([]ServiceInstanceSnapshot, 0)
	for rows.Next() {
		var snapshot ServiceInstanceSnapshot
		if err := rows.Scan(&snapshot.ServiceName, &snapshot.NodeID, &snapshot.IsDeclared, &snapshot.RuntimeStatus, &snapshot.UpdatedAt, &snapshot.LastTaskID, &snapshot.PendingDeployRevision); err != nil {
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
		SELECT service_name, node_id, is_declared, runtime_status, COALESCE(updated_at, ''), COALESCE(last_task_id, ''), COALESCE(pending_deploy_revision, '')
		FROM service_instances
		WHERE service_name = ? AND node_id = ?
	`, serviceName, nodeID).Scan(&snapshot.ServiceName, &snapshot.NodeID, &snapshot.IsDeclared, &snapshot.RuntimeStatus, &snapshot.UpdatedAt, &snapshot.LastTaskID, &snapshot.PendingDeployRevision)
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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = tx.Rollback() }()
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

func (db *DB) SetServiceInstancePendingDeploy(ctx context.Context, serviceName, nodeID, revision string) error {
	if _, err := db.sql.ExecContext(ctx, `
		UPDATE service_instances
		SET pending_deploy_revision = ?, updated_at = ?
		WHERE service_name = ? AND node_id = ?
	`, revision, time.Now().UTC().Format(time.RFC3339), serviceName, nodeID); err != nil {
		return fmt.Errorf("set pending deploy for %q@%q: %w", serviceName, nodeID, err)
	}
	return nil
}

func (db *DB) SetServicePendingDeploy(ctx context.Context, serviceName string, nodeIDs []string, revision string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set pending deploy for %q: %w", serviceName, err)
	}
	defer func() { _ = tx.Rollback() }()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	for _, nodeID := range nodeIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE service_instances
			SET pending_deploy_revision = ?, updated_at = ?
			WHERE service_name = ? AND node_id = ?
		`, revision, updatedAt, serviceName, nodeID); err != nil {
			return fmt.Errorf("set pending deploy for %q@%q: %w", serviceName, nodeID, err)
		}
	}
	return tx.Commit()
}

func (db *DB) ClearServiceInstancePendingDeploy(ctx context.Context, serviceName, nodeID string) error {
	if _, err := db.sql.ExecContext(ctx, `
		UPDATE service_instances
		SET pending_deploy_revision = NULL, updated_at = ?
		WHERE service_name = ? AND node_id = ?
	`, time.Now().UTC().Format(time.RFC3339), serviceName, nodeID); err != nil {
		return fmt.Errorf("clear pending deploy for %q@%q: %w", serviceName, nodeID, err)
	}
	return nil
}

func (db *DB) ClearServicePendingDeploy(ctx context.Context, serviceName string) error {
	if _, err := db.sql.ExecContext(ctx, `
		UPDATE service_instances
		SET pending_deploy_revision = NULL, updated_at = ?
		WHERE service_name = ?
	`, time.Now().UTC().Format(time.RFC3339), serviceName); err != nil {
		return fmt.Errorf("clear pending deploy for %q: %w", serviceName, err)
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

func IsValidImageCheckStatus(status string) bool {
	switch status {
	case ImageCheckStatusUnknown, ImageCheckStatusOK, ImageCheckStatusError:
		return true
	default:
		return false
	}
}

func (db *DB) UpsertServiceImageStates(ctx context.Context, states []ServiceImageState) error {
	if len(states) == 0 {
		return nil
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin service image state upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, state := range states {
		if state.ServiceName == "" {
			return fmt.Errorf("service name is required")
		}
		if state.NodeID == "" {
			return fmt.Errorf("node id is required")
		}
		if state.ComposeService == "" {
			return fmt.Errorf("compose service is required")
		}
		if state.ImageRef == "" {
			return fmt.Errorf("image ref is required")
		}
		if state.CheckStatus == "" {
			state.CheckStatus = ImageCheckStatusUnknown
		}
		if !IsValidImageCheckStatus(state.CheckStatus) {
			return fmt.Errorf("invalid image check status %q", state.CheckStatus)
		}
		checkedAt := state.CheckedAt.UTC()
		if checkedAt.IsZero() {
			checkedAt = time.Now().UTC()
		}
		updatedAt := state.UpdatedAt.UTC()
		if updatedAt.IsZero() {
			updatedAt = checkedAt
		}
		updateAvailable := state.RemoteDigestObserved && state.LocalDigestObserved && state.RemoteDigest != "" && state.LocalDigest != "" && state.RemoteDigest != state.LocalDigest
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO service_image_states (
				service_name, node_id, compose_service, image_ref,
				local_digest, remote_digest, update_available,
				check_status, error_summary, checked_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(service_name, node_id, compose_service, image_ref) DO UPDATE SET
				local_digest = CASE WHEN ? THEN excluded.local_digest ELSE service_image_states.local_digest END,
				remote_digest = CASE WHEN ? THEN excluded.remote_digest ELSE service_image_states.remote_digest END,
				update_available = CASE
					WHEN (CASE WHEN ? THEN excluded.remote_digest ELSE service_image_states.remote_digest END) != ''
						AND (CASE WHEN ? THEN excluded.local_digest ELSE service_image_states.local_digest END) != ''
					THEN (CASE WHEN ? THEN excluded.remote_digest ELSE service_image_states.remote_digest END) != (CASE WHEN ? THEN excluded.local_digest ELSE service_image_states.local_digest END)
					ELSE 0
				END,
				check_status = excluded.check_status,
				error_summary = excluded.error_summary,
				checked_at = excluded.checked_at,
				updated_at = excluded.updated_at
		`,
			state.ServiceName, state.NodeID, state.ComposeService, state.ImageRef,
			state.LocalDigest, state.RemoteDigest, updateAvailable,
			state.CheckStatus, nullableString(state.ErrorSummary), checkedAt.Format(time.RFC3339), updatedAt.Format(time.RFC3339),
			state.LocalDigestObserved, state.RemoteDigestObserved,
			state.RemoteDigestObserved, state.LocalDigestObserved,
			state.RemoteDigestObserved, state.LocalDigestObserved,
		); err != nil {
			return fmt.Errorf("upsert service image state %q@%q %q %q: %w", state.ServiceName, state.NodeID, state.ComposeService, state.ImageRef, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit service image state upsert: %w", err)
	}
	return nil
}

func (db *DB) UpsertServiceImageUpdateChecks(ctx context.Context, checks []ServiceImageUpdateCheck) error {
	if len(checks) == 0 {
		return nil
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin service image update check upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, check := range checks {
		if check.ServiceName == "" {
			return fmt.Errorf("service name is required")
		}
		if check.NodeID == "" {
			return fmt.Errorf("node id is required")
		}
		if check.ImageName == "" {
			return fmt.Errorf("image name is required")
		}
		if check.ImageRef == "" {
			return fmt.Errorf("image ref is required")
		}
		if check.CheckStatus == "" {
			check.CheckStatus = ImageCheckStatusUnknown
		}
		if !IsValidImageCheckStatus(check.CheckStatus) {
			return fmt.Errorf("invalid image check status %q", check.CheckStatus)
		}
		checkedAt := check.CheckedAt.UTC()
		if checkedAt.IsZero() {
			checkedAt = time.Now().UTC()
		}
		updatedAt := check.UpdatedAt.UTC()
		if updatedAt.IsZero() {
			updatedAt = checkedAt
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO service_image_update_checks (
				service_name, node_id, image_name, image_ref, policy_type,
				current_value, current_tag, current_digest,
				candidate_tag, candidate_digest, candidate_tags_json,
				update_available, check_status, error_summary, checked_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(service_name, node_id, image_name) DO UPDATE SET
				image_ref = excluded.image_ref,
				policy_type = excluded.policy_type,
				current_value = excluded.current_value,
				current_tag = excluded.current_tag,
				current_digest = excluded.current_digest,
				candidate_tag = excluded.candidate_tag,
				candidate_digest = excluded.candidate_digest,
				candidate_tags_json = excluded.candidate_tags_json,
				update_available = excluded.update_available,
				check_status = excluded.check_status,
				error_summary = excluded.error_summary,
				checked_at = excluded.checked_at,
				updated_at = excluded.updated_at
		`,
			check.ServiceName, check.NodeID, check.ImageName, check.ImageRef, check.PolicyType,
			check.CurrentValue, check.CurrentTag, check.CurrentDigest,
			check.CandidateTag, check.CandidateDigest, nullableString(check.CandidateTagsJSON),
			check.UpdateAvailable, check.CheckStatus, nullableString(check.ErrorSummary), checkedAt.Format(time.RFC3339), updatedAt.Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("upsert service image update check %q@%q %q: %w", check.ServiceName, check.NodeID, check.ImageName, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit service image update check upsert: %w", err)
	}
	return nil
}

func (db *DB) LatestServiceImageUpdateChecks(ctx context.Context, serviceName string, nodeID string) ([]ServiceImageUpdateCheck, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	query := `
		SELECT service_name, node_id, image_name, image_ref, policy_type,
			current_value, current_tag, current_digest,
			candidate_tag, candidate_digest, COALESCE(candidate_tags_json, ''),
			update_available, check_status, COALESCE(error_summary, ''), checked_at, updated_at
		FROM service_image_update_checks
		WHERE service_name = ?`
	args := []any{serviceName}
	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}
	query += ` ORDER BY image_name, node_id`
	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query service image update checks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var checks []ServiceImageUpdateCheck
	for rows.Next() {
		var check ServiceImageUpdateCheck
		var checkedAt, updatedAt string
		if err := rows.Scan(&check.ServiceName, &check.NodeID, &check.ImageName, &check.ImageRef, &check.PolicyType, &check.CurrentValue, &check.CurrentTag, &check.CurrentDigest, &check.CandidateTag, &check.CandidateDigest, &check.CandidateTagsJSON, &check.UpdateAvailable, &check.CheckStatus, &check.ErrorSummary, &checkedAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan service image update check: %w", err)
		}
		parsedCheckedAt, err := time.Parse(time.RFC3339, checkedAt)
		if err != nil {
			return nil, fmt.Errorf("parse service image update check %q@%q %q checked_at: %w", check.ServiceName, check.NodeID, check.ImageName, err)
		}
		parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse service image update check %q@%q %q updated_at: %w", check.ServiceName, check.NodeID, check.ImageName, err)
		}
		check.CheckedAt = parsedCheckedAt.UTC()
		check.UpdatedAt = parsedUpdatedAt.UTC()
		checks = append(checks, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service image update checks: %w", err)
	}
	return checks, nil
}

type sqliteMigration struct {
	version    int
	statements []string
}

func (db *DB) migrate(ctx context.Context) error {
	migrations := []sqliteMigration{{
		version: 1,
		statements: []string{
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
			FOREIGN KEY (service_name, node_id) REFERENCES service_instances(service_name, node_id),
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
			node_id TEXT,
			data_name TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			artifact_ref TEXT,
			error_summary TEXT,
			FOREIGN KEY (task_id) REFERENCES tasks(task_id),
			FOREIGN KEY (service_name) REFERENCES services(service_name),
			FOREIGN KEY (service_name, node_id) REFERENCES service_instances(service_name, node_id),
			FOREIGN KEY (node_id) REFERENCES nodes(node_id)
		);`,
			`ALTER TABLE backups ADD COLUMN node_id TEXT;`,
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
			`CREATE TABLE IF NOT EXISTS service_image_states (
			service_name TEXT NOT NULL,
			node_id TEXT NOT NULL,
			compose_service TEXT NOT NULL,
			image_ref TEXT NOT NULL,
			local_digest TEXT NOT NULL DEFAULT '',
			remote_digest TEXT NOT NULL DEFAULT '',
			update_available INTEGER NOT NULL DEFAULT 0,
			check_status TEXT NOT NULL,
			error_summary TEXT,
			checked_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (service_name, node_id, compose_service, image_ref),
			FOREIGN KEY (service_name, node_id) REFERENCES service_instances(service_name, node_id)
		);`,
			`CREATE INDEX IF NOT EXISTS idx_service_image_states_update_available ON service_image_states(update_available, checked_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_service_image_states_service_node ON service_image_states(service_name, node_id);`,
			`CREATE TABLE IF NOT EXISTS service_image_update_checks (
			service_name TEXT NOT NULL,
			node_id TEXT NOT NULL,
			image_name TEXT NOT NULL,
			image_ref TEXT NOT NULL,
			policy_type TEXT NOT NULL,
			current_value TEXT NOT NULL DEFAULT '',
			current_tag TEXT NOT NULL DEFAULT '',
			current_digest TEXT NOT NULL DEFAULT '',
			candidate_tag TEXT NOT NULL DEFAULT '',
			candidate_digest TEXT NOT NULL DEFAULT '',
			candidate_tags_json TEXT,
			update_available INTEGER NOT NULL DEFAULT 0,
			check_status TEXT NOT NULL,
			error_summary TEXT,
			checked_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (service_name, node_id, image_name),
			FOREIGN KEY (service_name, node_id) REFERENCES service_instances(service_name, node_id)
		);`,
			`CREATE INDEX IF NOT EXISTS idx_service_image_update_checks_available ON service_image_update_checks(update_available, checked_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_service_image_update_checks_service_node ON service_image_update_checks(service_name, node_id);`,
		},
	}, {
		version: 2,
		statements: []string{
			`ALTER TABLE service_instances ADD COLUMN pending_deploy_revision TEXT;`,
		},
	}, {
		version: 3,
		statements: []string{
			`ALTER TABLE backups ADD COLUMN node_id TEXT;`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_active_service ON tasks(service_name) WHERE service_name IS NOT NULL AND node_id IS NULL AND type = 'migrate' AND status IN ('pending', 'running', 'awaiting_confirmation');`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_active_service_instance ON tasks(service_name, node_id) WHERE service_name IS NOT NULL AND node_id IS NOT NULL AND type IN ('deploy', 'update', 'stop', 'restart', 'backup', 'dns_update', 'caddy_sync', 'image_check') AND status IN ('pending', 'running', 'awaiting_confirmation');`,
			`CREATE TRIGGER IF NOT EXISTS trg_tasks_validate_service_instance_insert BEFORE INSERT ON tasks FOR EACH ROW WHEN NEW.service_name IS NOT NULL AND NEW.node_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM service_instances WHERE service_name = NEW.service_name AND node_id = NEW.node_id) BEGIN SELECT RAISE(ABORT, 'task service instance does not exist'); END;`,
			`CREATE TRIGGER IF NOT EXISTS trg_tasks_validate_service_instance_update BEFORE UPDATE OF service_name, node_id ON tasks FOR EACH ROW WHEN NEW.service_name IS NOT NULL AND NEW.node_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM service_instances WHERE service_name = NEW.service_name AND node_id = NEW.node_id) BEGIN SELECT RAISE(ABORT, 'task service instance does not exist'); END;`,
			`CREATE TABLE backups_v3 (
			backup_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			service_name TEXT NOT NULL,
			node_id TEXT,
			data_name TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			artifact_ref TEXT,
			error_summary TEXT,
			FOREIGN KEY (task_id) REFERENCES tasks(task_id),
			FOREIGN KEY (service_name) REFERENCES services(service_name),
			FOREIGN KEY (node_id) REFERENCES nodes(node_id),
			FOREIGN KEY (service_name, node_id) REFERENCES service_instances(service_name, node_id)
		);`,
			`INSERT INTO backups_v3 (
			backup_id,
			task_id,
			service_name,
			node_id,
			data_name,
			status,
			started_at,
			finished_at,
			artifact_ref,
			error_summary
		)
			SELECT
			backups.backup_id,
			backups.task_id,
			backups.service_name,
			COALESCE(backups.node_id, tasks.node_id),
			backups.data_name,
			backups.status,
			backups.started_at,
			backups.finished_at,
			backups.artifact_ref,
			backups.error_summary
			FROM backups
			LEFT JOIN tasks ON tasks.task_id = backups.task_id;`,
			`DROP TABLE backups;`,
			`ALTER TABLE backups_v3 RENAME TO backups;`,
			`CREATE INDEX IF NOT EXISTS idx_backups_service_finished_at ON backups(service_name, finished_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_backups_status_finished_at ON backups(status, finished_at DESC);`,
		},
	}}

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	currentVersion, err := sqliteUserVersion(ctx, tx)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if migration.version <= currentVersion {
			continue
		}
		for _, statement := range migration.statements {
			if err := applySQLiteMigrationStatement(ctx, tx, statement); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d;`, migration.version)); err != nil {
			return fmt.Errorf("set sqlite schema version %d: %w", migration.version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration: %w", err)
	}
	return nil
}

func sqliteUserVersion(ctx context.Context, tx *sql.Tx) (int, error) {
	var version int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version;`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read sqlite schema version: %w", err)
	}
	if version > sqliteSchemaVersion {
		return 0, fmt.Errorf("sqlite schema version %d is newer than supported version %d", version, sqliteSchemaVersion)
	}
	return version, nil
}

func applySQLiteMigrationStatement(ctx context.Context, tx *sql.Tx, statement string) error {
	if _, err := tx.ExecContext(ctx, statement); err != nil {
		if (statement == `ALTER TABLE backups ADD COLUMN node_id TEXT;` || statement == `ALTER TABLE service_instances ADD COLUMN pending_deploy_revision TEXT;`) && isDuplicateColumnError(err) {
			return nil
		}
		return fmt.Errorf("apply sqlite schema statement: %w", err)
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
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
	defer func() { _ = rows.Close() }()

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
	case allStatusesEqual(statuses, ServiceRuntimeRunning):
		aggregate = ServiceRuntimeRunning
	case allStatusesEqual(statuses, ServiceRuntimeStopped):
		aggregate = ServiceRuntimeStopped
	case hasStatus(statuses, ServiceRuntimeError):
		aggregate = ServiceRuntimeError
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
	parsedReportedAt, err := time.Parse(time.RFC3339, reportedAt)
	if err != nil {
		return DockerStats{}, fmt.Errorf("parse docker stats for node %q reported_at: %w", nodeID, err)
	}
	stats.ReportedAt = parsedReportedAt.UTC()
	return stats, nil
}
