package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"forgejo.alexma.top/alexma233/composia/internal/task"
)

var ErrBackupNotFound = errors.New("backup not found")

type BackupSummary struct {
	BackupID    string
	TaskID      string
	ServiceName string
	DataName    string
	Status      string
	StartedAt   string
	FinishedAt  string
}

type BackupDetail struct {
	BackupID     string
	TaskID       string
	ServiceName  string
	DataName     string
	Status       string
	StartedAt    string
	FinishedAt   string
	ArtifactRef  string
	ErrorSummary string
}

func (db *DB) UpsertBackupRecord(ctx context.Context, detail BackupDetail) error {
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO backups (
			backup_id,
			task_id,
			service_name,
			data_name,
			status,
			started_at,
			finished_at,
			artifact_ref,
			error_summary
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(backup_id) DO UPDATE SET
			task_id = excluded.task_id,
			service_name = excluded.service_name,
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

func (db *DB) ListBackups(ctx context.Context, serviceNameFilter, statusFilter, dataNameFilter, cursor string, limit uint32) ([]BackupSummary, string, error) {
	if limit == 0 {
		limit = 100
	}

	query := `
		SELECT backup_id, task_id, service_name, data_name, status, started_at, COALESCE(finished_at, '')
		FROM backups
		WHERE 1 = 1
	`
	args := make([]any, 0, 6)
	if serviceNameFilter != "" {
		query += ` AND service_name = ?`
		args = append(args, serviceNameFilter)
	}
	if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}
	if dataNameFilter != "" {
		query += ` AND data_name = ?`
		args = append(args, dataNameFilter)
	}
	if cursor != "" {
		cursorSortTime, err := db.backupSortTime(ctx, cursor)
		if err != nil {
			return nil, "", err
		}
		query += ` AND (COALESCE(finished_at, started_at) < ? OR (COALESCE(finished_at, started_at) = ? AND backup_id < ?))`
		args = append(args, cursorSortTime, cursorSortTime, cursor)
	}
	query += ` ORDER BY COALESCE(finished_at, started_at) DESC, backup_id DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list backups: %w", err)
	}
	defer rows.Close()

	backups := make([]BackupSummary, 0, limit)
	var nextCursor string
	for rows.Next() {
		var backup BackupSummary
		if err := rows.Scan(&backup.BackupID, &backup.TaskID, &backup.ServiceName, &backup.DataName, &backup.Status, &backup.StartedAt, &backup.FinishedAt); err != nil {
			return nil, "", fmt.Errorf("scan backup summary: %w", err)
		}
		backups = append(backups, backup)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate backups: %w", err)
	}
	if uint32(len(backups)) > limit {
		nextCursor = backups[limit-1].BackupID
		backups = backups[:limit]
	}
	return backups, nextCursor, nil
}

func (db *DB) GetBackup(ctx context.Context, backupID string) (BackupDetail, error) {
	var detail BackupDetail
	err := db.sql.QueryRowContext(ctx, `
		SELECT backup_id, task_id, service_name, data_name, status, started_at, COALESCE(finished_at, ''), COALESCE(artifact_ref, ''), COALESCE(error_summary, '')
		FROM backups
		WHERE backup_id = ?
	`, backupID).Scan(&detail.BackupID, &detail.TaskID, &detail.ServiceName, &detail.DataName, &detail.Status, &detail.StartedAt, &detail.FinishedAt, &detail.ArtifactRef, &detail.ErrorSummary)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BackupDetail{}, ErrBackupNotFound
		}
		return BackupDetail{}, fmt.Errorf("get backup %q: %w", backupID, err)
	}
	return detail, nil
}

func (db *DB) backupSortTime(ctx context.Context, backupID string) (string, error) {
	var sortTime string
	if err := db.sql.QueryRowContext(ctx, `SELECT COALESCE(finished_at, started_at) FROM backups WHERE backup_id = ?`, backupID).Scan(&sortTime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrBackupNotFound
		}
		return "", fmt.Errorf("read backup cursor %q: %w", backupID, err)
	}
	return sortTime, nil
}

var _ = task.StatusSucceeded
