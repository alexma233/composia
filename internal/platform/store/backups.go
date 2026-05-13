package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

var ErrBackupNotFound = errors.New("backup not found")

type BackupSummary struct {
	BackupID    string
	TaskID      string
	ServiceName string
	NodeID      string
	DataName    string
	Status      string
	StartedAt   string
	FinishedAt  string
}

type BackupDetail struct {
	BackupID     string
	TaskID       string
	ServiceName  string
	NodeID       string
	DataName     string
	Status       string
	StartedAt    string
	FinishedAt   string
	ArtifactRef  string
	ErrorSummary string
}

func (db *DB) UpsertBackupRecord(ctx context.Context, detail BackupDetail) error {
	if detail.NodeID == "" {
		nodeID, err := db.TaskNodeID(ctx, detail.TaskID)
		if err != nil && !errors.Is(err, ErrTaskNotFound) {
			return fmt.Errorf("resolve backup node_id for %q: %w", detail.BackupID, err)
		}
		detail.NodeID = nodeID
	}
	if detail.NodeID == "" {
		return fmt.Errorf("backup %q requires node_id", detail.BackupID)
	}

	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO backups (
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(backup_id) DO UPDATE SET
			task_id = excluded.task_id,
			service_name = excluded.service_name,
			node_id = excluded.node_id,
			data_name = excluded.data_name,
			status = excluded.status,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			artifact_ref = excluded.artifact_ref,
			error_summary = excluded.error_summary
	`,
		detail.BackupID,
		detail.TaskID,
		detail.ServiceName,
		detail.NodeID,
		detail.DataName,
		detail.Status,
		detail.StartedAt,
		nullableString(detail.FinishedAt),
		nullableString(detail.ArtifactRef),
		nullableString(detail.ErrorSummary),
	)
	if err != nil {
		return fmt.Errorf("upsert backup record %q: %w", detail.BackupID, err)
	}
	return nil
}

func (db *DB) ListBackups(
	ctx context.Context,
	serviceNameFilters, statusFilters, dataNameFilters, nodeIDFilters []string,
	excludeServiceNameFilters, excludeStatusFilters, excludeDataNameFilters, excludeNodeIDFilters []string,
	page, limit uint32,
) ([]BackupSummary, uint32, error) {
	if limit == 0 {
		limit = 100
	}
	if page == 0 {
		page = 1
	}

	whereClause := "WHERE 1 = 1"
	args := make([]any, 0, 20)
	whereClause, args = appendBackupFilterInClause(whereClause, args, "backups.service_name", serviceNameFilters)
	whereClause, args = appendBackupFilterInClause(whereClause, args, "backups.status", statusFilters)
	whereClause, args = appendBackupFilterInClause(whereClause, args, "backups.data_name", dataNameFilters)
	whereClause, args = appendBackupFilterInClause(whereClause, args, "backups.node_id", nodeIDFilters)
	whereClause, args = appendBackupFilterNotInClause(whereClause, args, "backups.service_name", excludeServiceNameFilters)
	whereClause, args = appendBackupFilterNotInClause(whereClause, args, "backups.status", excludeStatusFilters)
	whereClause, args = appendBackupFilterNotInClause(whereClause, args, "backups.data_name", excludeDataNameFilters)
	whereClause, args = appendBackupFilterNotInClause(whereClause, args, "backups.node_id", excludeNodeIDFilters)

	var totalCount uint32
	countQuery := `SELECT COUNT(*) FROM backups ` + whereClause
	if err := db.sql.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count backups: %w", err)
	}

	offset := (page - 1) * limit
	query := `
		SELECT backups.backup_id, backups.task_id, backups.service_name, backups.node_id, backups.data_name, backups.status, backups.started_at, COALESCE(backups.finished_at, '')
		FROM backups
	` + whereClause
	query += ` ORDER BY COALESCE(backups.finished_at, backups.started_at) DESC, backups.backup_id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list backups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	backups := make([]BackupSummary, 0, limit)
	for rows.Next() {
		var backup BackupSummary
		if err := rows.Scan(&backup.BackupID, &backup.TaskID, &backup.ServiceName, &backup.NodeID, &backup.DataName, &backup.Status, &backup.StartedAt, &backup.FinishedAt); err != nil {
			return nil, 0, fmt.Errorf("scan backup summary: %w", err)
		}
		backups = append(backups, backup)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate backups: %w", err)
	}
	return backups, totalCount, nil
}

func appendBackupFilterInClause(whereClause string, args []any, expression string, values []string) (string, []any) {
	return appendStringFilterInClause(whereClause, args, expression, values)
}

func appendBackupFilterNotInClause(whereClause string, args []any, expression string, values []string) (string, []any) {
	return appendStringFilterNotInClause(whereClause, args, expression, values)
}

func (db *DB) GetBackup(ctx context.Context, backupID string) (BackupDetail, error) {
	var detail BackupDetail
	err := db.sql.QueryRowContext(ctx, `
		SELECT backups.backup_id, backups.task_id, backups.service_name, backups.node_id, backups.data_name, backups.status, backups.started_at, COALESCE(backups.finished_at, ''), COALESCE(backups.artifact_ref, ''), COALESCE(backups.error_summary, '')
		FROM backups
		WHERE backups.backup_id = ?
	`, backupID).Scan(&detail.BackupID, &detail.TaskID, &detail.ServiceName, &detail.NodeID, &detail.DataName, &detail.Status, &detail.StartedAt, &detail.FinishedAt, &detail.ArtifactRef, &detail.ErrorSummary)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BackupDetail{}, ErrBackupNotFound
		}
		return BackupDetail{}, fmt.Errorf("get backup %q: %w", backupID, err)
	}
	return detail, nil
}

func (db *DB) ListBackupsForTask(ctx context.Context, taskID string) ([]BackupDetail, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT backups.backup_id, backups.task_id, backups.service_name, backups.node_id, backups.data_name, backups.status, backups.started_at, COALESCE(backups.finished_at, ''), COALESCE(backups.artifact_ref, ''), COALESCE(backups.error_summary, '')
		FROM backups
		WHERE backups.task_id = ?
		ORDER BY backups.data_name ASC, backups.backup_id ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list backups for task %q: %w", taskID, err)
	}
	defer func() { _ = rows.Close() }()
	backups := make([]BackupDetail, 0)
	for rows.Next() {
		var detail BackupDetail
		if err := rows.Scan(&detail.BackupID, &detail.TaskID, &detail.ServiceName, &detail.NodeID, &detail.DataName, &detail.Status, &detail.StartedAt, &detail.FinishedAt, &detail.ArtifactRef, &detail.ErrorSummary); err != nil {
			return nil, fmt.Errorf("scan backup detail for task %q: %w", taskID, err)
		}
		backups = append(backups, detail)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate backups for task %q: %w", taskID, err)
	}
	return backups, nil
}

var _ = task.StatusSucceeded
