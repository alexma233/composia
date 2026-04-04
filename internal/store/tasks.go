package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/task"
	"github.com/google/uuid"
)

var ErrNoPendingTask = errors.New("no pending task")
var ErrTaskNotFound = errors.New("task not found")

type TaskSummary struct {
	TaskID      string
	Type        string
	Status      string
	ServiceName string
	NodeID      string
	CreatedAt   string
}

type TaskDetail struct {
	Record task.Record
	Steps  []task.StepRecord
}

func (db *DB) CreateTask(ctx context.Context, record task.Record) (task.Record, error) {
	if record.Type == "" {
		return task.Record{}, fmt.Errorf("task type is required")
	}
	if record.Source == "" {
		return task.Record{}, fmt.Errorf("task source is required")
	}
	if record.Status == "" {
		record.Status = task.StatusPending
	}
	if record.TaskID == "" {
		record.TaskID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO tasks (
			task_id,
			type,
			source,
			triggered_by,
			service_name,
			node_id,
			status,
			params_json,
			log_path,
			repo_revision,
			result_revision,
			attempt_of_task_id,
			created_at,
			started_at,
			finished_at,
			error_summary
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.TaskID,
		string(record.Type),
		string(record.Source),
		nullableString(record.TriggeredBy),
		nullableString(record.ServiceName),
		nullableString(record.NodeID),
		string(record.Status),
		nullableString(record.ParamsJSON),
		nullableString(record.LogPath),
		nullableString(record.RepoRevision),
		nullableString(record.ResultRevision),
		nullableString(record.AttemptOfTaskID),
		record.CreatedAt.UTC().Format(time.RFC3339),
		nullableTime(record.StartedAt),
		nullableTime(record.FinishedAt),
		nullableString(record.ErrorSummary),
	)
	if err != nil {
		return task.Record{}, fmt.Errorf("create task %q: %w", record.TaskID, err)
	}
	return record, nil
}

func (db *DB) ClaimNextPendingTask(ctx context.Context, startedAt time.Time) (task.Record, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return task.Record{}, fmt.Errorf("begin claim task transaction: %w", err)
	}
	defer tx.Rollback()

	record, err := claimNextPendingTask(ctx, tx, startedAt)
	if err != nil {
		return task.Record{}, err
	}

	if err := tx.Commit(); err != nil {
		return task.Record{}, fmt.Errorf("commit claim task transaction: %w", err)
	}
	return record, nil
}

func claimNextPendingTask(ctx context.Context, tx *sql.Tx, startedAt time.Time) (task.Record, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT task_id, type, source, COALESCE(triggered_by, ''), COALESCE(service_name, ''), COALESCE(node_id, ''),
		       status, COALESCE(params_json, ''), COALESCE(log_path, ''), COALESCE(repo_revision, ''),
		       COALESCE(result_revision, ''), COALESCE(attempt_of_task_id, ''), created_at
		FROM tasks
		WHERE status = ?
		ORDER BY created_at ASC, task_id ASC
		LIMIT 1
	`, string(task.StatusPending))

	var record task.Record
	var rawType string
	var rawSource string
	var rawStatus string
	var createdAt string
	if err := row.Scan(
		&record.TaskID,
		&rawType,
		&rawSource,
		&record.TriggeredBy,
		&record.ServiceName,
		&record.NodeID,
		&rawStatus,
		&record.ParamsJSON,
		&record.LogPath,
		&record.RepoRevision,
		&record.ResultRevision,
		&record.AttemptOfTaskID,
		&createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.Record{}, ErrNoPendingTask
		}
		return task.Record{}, fmt.Errorf("query next pending task: %w", err)
	}

	updated, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, started_at = ?
		WHERE task_id = ? AND status = ?
	`, string(task.StatusRunning), startedAt.UTC().Format(time.RFC3339), record.TaskID, string(task.StatusPending))
	if err != nil {
		return task.Record{}, fmt.Errorf("mark task %q running: %w", record.TaskID, err)
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return task.Record{}, fmt.Errorf("read claim task rows affected: %w", err)
	}
	if affected == 0 {
		return task.Record{}, ErrNoPendingTask
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return task.Record{}, fmt.Errorf("parse task %q created_at: %w", record.TaskID, err)
	}

	record.Type = task.Type(rawType)
	record.Source = task.Source(rawSource)
	record.Status = task.Status(rawStatus)
	record.CreatedAt = parsedCreatedAt.UTC()
	record.Status = task.StatusRunning
	record.StartedAt = timePtr(startedAt.UTC())
	return record, nil
}

func (db *DB) CompleteTask(ctx context.Context, taskID string, status task.Status, finishedAt time.Time, errorSummary string) error {
	if status != task.StatusSucceeded && status != task.StatusFailed && status != task.StatusCancelled {
		return fmt.Errorf("invalid terminal task status %q", status)
	}

	_, err := db.sql.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, finished_at = ?, error_summary = ?
		WHERE task_id = ?
	`, string(status), finishedAt.UTC().Format(time.RFC3339), nullableString(errorSummary), taskID)
	if err != nil {
		return fmt.Errorf("complete task %q: %w", taskID, err)
	}
	return nil
}

func (db *DB) RecoverRunningTasks(ctx context.Context, finishedAt time.Time) (int64, error) {
	result, err := db.sql.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, finished_at = ?, error_summary = ?
		WHERE status = ?
	`,
		string(task.StatusFailed),
		finishedAt.UTC().Format(time.RFC3339),
		"controller restarted during task execution",
		string(task.StatusRunning),
	)
	if err != nil {
		return 0, fmt.Errorf("recover running tasks: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read recovered running task count: %w", err)
	}
	return affected, nil
}

func (db *DB) HasActiveServiceTask(ctx context.Context, serviceName string) (bool, error) {
	var count int
	err := db.sql.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM tasks
		WHERE service_name = ? AND status IN (?, ?, ?)
	`,
		serviceName,
		string(task.StatusPending),
		string(task.StatusRunning),
		string(task.StatusAwaitingConfirmation),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count active tasks for service %q: %w", serviceName, err)
	}
	return count > 0, nil
}

func (db *DB) ListTasks(ctx context.Context, statusFilter, serviceNameFilter, cursor string, limit uint32) ([]TaskSummary, string, error) {
	if limit == 0 {
		limit = 100
	}

	query := `
		SELECT task_id, type, status, COALESCE(service_name, ''), COALESCE(node_id, ''), created_at
		FROM tasks
		WHERE 1 = 1
	`
	args := make([]any, 0, 5)

	if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}
	if serviceNameFilter != "" {
		query += ` AND service_name = ?`
		args = append(args, serviceNameFilter)
	}

	if cursor != "" {
		cursorCreatedAt, err := db.taskCreatedAt(ctx, cursor)
		if err != nil {
			return nil, "", err
		}
		query += ` AND (created_at < ? OR (created_at = ? AND task_id < ?))`
		args = append(args, cursorCreatedAt, cursorCreatedAt, cursor)
	}

	query += ` ORDER BY created_at DESC, task_id DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]TaskSummary, 0, limit)
	var nextCursor string
	for rows.Next() {
		var record TaskSummary
		if err := rows.Scan(&record.TaskID, &record.Type, &record.Status, &record.ServiceName, &record.NodeID, &record.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("scan task summary: %w", err)
		}
		tasks = append(tasks, record)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate task summaries: %w", err)
	}

	if uint32(len(tasks)) > limit {
		nextCursor = tasks[limit-1].TaskID
		tasks = tasks[:limit]
	}

	return tasks, nextCursor, nil
}

func (db *DB) GetTask(ctx context.Context, taskID string) (TaskDetail, error) {
	record, err := db.getTaskRecord(ctx, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	steps, err := db.ListTaskSteps(ctx, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	return TaskDetail{Record: record, Steps: steps}, nil
}

func (db *DB) ListTaskSteps(ctx context.Context, taskID string) ([]task.StepRecord, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT step_name, status, COALESCE(started_at, ''), COALESCE(finished_at, '')
		FROM task_steps
		WHERE task_id = ?
		ORDER BY started_at ASC, step_name ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task steps for %q: %w", taskID, err)
	}
	defer rows.Close()

	steps := make([]task.StepRecord, 0)
	for rows.Next() {
		var stepName string
		var status string
		var startedAt string
		var finishedAt string
		if err := rows.Scan(&stepName, &status, &startedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("scan task step for %q: %w", taskID, err)
		}
		steps = append(steps, task.StepRecord{
			TaskID:     taskID,
			StepName:   task.StepName(stepName),
			Status:     task.Status(status),
			StartedAt:  parseNullableRFC3339(startedAt),
			FinishedAt: parseNullableRFC3339(finishedAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task steps for %q: %w", taskID, err)
	}
	return steps, nil
}

func (db *DB) UpsertTaskStep(ctx context.Context, step task.StepRecord) error {
	if step.TaskID == "" {
		return fmt.Errorf("task step task_id is required")
	}
	if step.StepName == "" {
		return fmt.Errorf("task step step_name is required")
	}
	if step.Status == "" {
		return fmt.Errorf("task step status is required")
	}

	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO task_steps (task_id, step_name, status, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(task_id, step_name) DO UPDATE SET
			status = excluded.status,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at
	`,
		step.TaskID,
		string(step.StepName),
		string(step.Status),
		nullableTime(step.StartedAt),
		nullableTime(step.FinishedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert task step %q for task %q: %w", step.StepName, step.TaskID, err)
	}
	return nil
}

func (db *DB) taskCreatedAt(ctx context.Context, taskID string) (string, error) {
	var createdAt string
	if err := db.sql.QueryRowContext(ctx, `SELECT created_at FROM tasks WHERE task_id = ?`, taskID).Scan(&createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("task cursor %q does not exist", taskID)
		}
		return "", fmt.Errorf("read task cursor %q: %w", taskID, err)
	}
	return createdAt, nil
}

func (db *DB) getTaskRecord(ctx context.Context, taskID string) (task.Record, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT task_id, type, source, COALESCE(triggered_by, ''), COALESCE(service_name, ''), COALESCE(node_id, ''),
		       status, COALESCE(params_json, ''), COALESCE(log_path, ''), COALESCE(repo_revision, ''),
		       COALESCE(result_revision, ''), COALESCE(attempt_of_task_id, ''), created_at,
		       COALESCE(started_at, ''), COALESCE(finished_at, ''), COALESCE(error_summary, '')
		FROM tasks
		WHERE task_id = ?
	`, taskID)

	var record task.Record
	var rawType string
	var rawSource string
	var rawStatus string
	var createdAt string
	var startedAt string
	var finishedAt string
	if err := row.Scan(
		&record.TaskID,
		&rawType,
		&rawSource,
		&record.TriggeredBy,
		&record.ServiceName,
		&record.NodeID,
		&rawStatus,
		&record.ParamsJSON,
		&record.LogPath,
		&record.RepoRevision,
		&record.ResultRevision,
		&record.AttemptOfTaskID,
		&createdAt,
		&startedAt,
		&finishedAt,
		&record.ErrorSummary,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.Record{}, ErrTaskNotFound
		}
		return task.Record{}, fmt.Errorf("get task %q: %w", taskID, err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return task.Record{}, fmt.Errorf("parse task %q created_at: %w", taskID, err)
	}

	record.Type = task.Type(rawType)
	record.Source = task.Source(rawSource)
	record.Status = task.Status(rawStatus)
	record.CreatedAt = parsedCreatedAt.UTC()
	record.StartedAt = parseNullableRFC3339(startedAt)
	record.FinishedAt = parseNullableRFC3339(finishedAt)
	return record, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func parseNullableRFC3339(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}
