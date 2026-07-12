package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"github.com/google/uuid"
)

var (
	ErrNoPendingTask         = errors.New("no pending task")
	ErrTaskNotFound          = errors.New("task not found")
	ErrTaskExecutionMismatch = errors.New("task execution does not match")
	ErrTaskExecutionConflict = errors.New("task execution result conflicts with the persisted result")
)

type ActiveServiceTaskError struct {
	ServiceName string
}

func (err ActiveServiceTaskError) Error() string {
	return fmt.Sprintf("service %q already has an active task", err.ServiceName)
}

type ActiveServiceInstanceTaskError struct {
	ServiceName string
	NodeID      string
}

type DuplicateTaskError struct{ DedupeKey string }

func (err DuplicateTaskError) Error() string {
	return fmt.Sprintf("task with dedupe key %q already exists", err.DedupeKey)
}

func (err ActiveServiceInstanceTaskError) Error() string {
	return fmt.Sprintf("service instance %q@%q already has an active task", err.ServiceName, err.NodeID)
}

type TaskAdmissionConstraints struct {
	RequireInactiveService          bool
	RequireInactiveServiceInstances []ServiceInstanceTarget
}

type ServiceInstanceTarget struct {
	ServiceName string
	NodeID      string
}

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

type sqlTaskExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (db *DB) CreateTask(ctx context.Context, record task.Record) (task.Record, error) {
	preparedRecord, err := prepareTaskRecord(record)
	if err != nil {
		return task.Record{}, err
	}
	if err := insertTaskRecord(ctx, db.sql, preparedRecord); err != nil {
		return task.Record{}, err
	}
	return preparedRecord, nil
}

func (db *DB) CreateTaskIfNoActiveServiceTask(ctx context.Context, record task.Record) (task.Record, error) {
	return db.CreateTaskWithConstraints(ctx, record, TaskAdmissionConstraints{RequireInactiveService: true})
}

func (db *DB) CreateTaskWithConstraints(ctx context.Context, record task.Record, constraints TaskAdmissionConstraints) (task.Record, error) {
	preparedRecord, err := prepareTaskRecord(record)
	if err != nil {
		return task.Record{}, err
	}
	if constraints.RequireInactiveService {
		if preparedRecord.ServiceName == "" {
			return task.Record{}, errors.New("service_name is required")
		}
	}
	instanceTargets := constraints.RequireInactiveServiceInstances
	if len(instanceTargets) > 0 {
		instanceTargets = append([]ServiceInstanceTarget(nil), instanceTargets...)
	}

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return task.Record{}, fmt.Errorf("begin create task transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if constraints.RequireInactiveService {
		active, err := hasActiveServiceTaskTx(ctx, tx, preparedRecord.ServiceName)
		if err != nil {
			return task.Record{}, err
		}
		if active {
			return task.Record{}, ActiveServiceTaskError{ServiceName: preparedRecord.ServiceName}
		}
	}

	for _, target := range instanceTargets {
		if target.ServiceName == "" {
			return task.Record{}, errors.New("service_name is required")
		}
		if target.NodeID == "" {
			return task.Record{}, errors.New("node_id is required")
		}
		active, err := hasActiveServiceInstanceTaskTx(ctx, tx, target.ServiceName, target.NodeID)
		if err != nil {
			return task.Record{}, err
		}
		if active {
			return task.Record{}, ActiveServiceInstanceTaskError(target)
		}
	}

	if err := insertTaskRecord(ctx, tx, preparedRecord); err != nil {
		return task.Record{}, err
	}
	if err := tx.Commit(); err != nil {
		return task.Record{}, fmt.Errorf("commit create task transaction: %w", err)
	}
	return preparedRecord, nil
}

func (db *DB) CreateTaskIfNoActiveServiceInstanceTask(ctx context.Context, record task.Record) (task.Record, error) {
	records, err := db.CreateTasksIfNoActiveServiceInstanceTasks(ctx, []task.Record{record})
	if err != nil {
		return task.Record{}, err
	}
	return records[0], nil
}

func (db *DB) CreateTasksIfNoActiveServiceInstanceTasks(ctx context.Context, records []task.Record) ([]task.Record, error) {
	if len(records) == 0 {
		return nil, errors.New("at least one task record is required")
	}

	preparedRecords := make([]task.Record, 0, len(records))
	instanceKeys := make([]serviceInstanceKey, 0, len(records))
	seenKeys := make(map[serviceInstanceKey]struct{}, len(records))
	for _, record := range records {
		if record.ServiceName == "" {
			return nil, errors.New("service_name is required")
		}
		if record.NodeID == "" {
			return nil, errors.New("node_id is required")
		}
		preparedRecord, err := prepareTaskRecord(record)
		if err != nil {
			return nil, err
		}
		preparedRecords = append(preparedRecords, preparedRecord)

		key := serviceInstanceKey{ServiceName: preparedRecord.ServiceName, NodeID: preparedRecord.NodeID}
		if _, exists := seenKeys[key]; exists {
			return nil, fmt.Errorf("duplicate service instance task for %q@%q", key.ServiceName, key.NodeID)
		}
		seenKeys[key] = struct{}{}
		instanceKeys = append(instanceKeys, key)
	}

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create task transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, key := range instanceKeys {
		active, err := hasActiveServiceInstanceTaskTx(ctx, tx, key.ServiceName, key.NodeID)
		if err != nil {
			return nil, err
		}
		if active {
			return nil, ActiveServiceInstanceTaskError(key)
		}
	}

	for _, preparedRecord := range preparedRecords {
		if err := insertTaskRecord(ctx, tx, preparedRecord); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create task transaction: %w", err)
	}
	return preparedRecords, nil
}

type serviceInstanceKey struct {
	ServiceName string
	NodeID      string
}

func prepareTaskRecord(record task.Record) (task.Record, error) {
	if record.Type == "" {
		return task.Record{}, errors.New("task type is required")
	}
	if record.Source == "" {
		return task.Record{}, errors.New("task source is required")
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
	return record, nil
}

func insertTaskRecord(ctx context.Context, execer sqlTaskExecer, record task.Record) error {
	_, err := execer.ExecContext(ctx, `
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
			error_summary,
			dedupe_key
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		nullableString(record.DedupeKey),
	)
	if err != nil {
		if record.DedupeKey != "" && strings.Contains(strings.ToLower(err.Error()), "tasks.dedupe_key") {
			return DuplicateTaskError{DedupeKey: record.DedupeKey}
		}
		if isSQLiteUniqueConstraintError(err) {
			switch {
			case record.ServiceName != "" && record.NodeID != "":
				return ActiveServiceInstanceTaskError{ServiceName: record.ServiceName, NodeID: record.NodeID}
			case record.ServiceName != "":
				return ActiveServiceTaskError{ServiceName: record.ServiceName}
			}
		}
		return fmt.Errorf("create task %q: %w", record.TaskID, err)
	}
	return nil
}

func (db *DB) GetTaskByDedupeKey(ctx context.Context, dedupeKey string) (task.Record, error) {
	var taskID string
	if err := db.sql.QueryRowContext(ctx, `SELECT task_id FROM tasks WHERE dedupe_key = ?`, dedupeKey).Scan(&taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.Record{}, ErrTaskNotFound
		}
		return task.Record{}, fmt.Errorf("get task by dedupe key %q: %w", dedupeKey, err)
	}
	detail, err := db.GetTask(ctx, taskID)
	if err != nil {
		return task.Record{}, err
	}
	detail.Record.DedupeKey = dedupeKey
	return detail.Record, nil
}

func isSQLiteUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed") && !strings.Contains(message, "tasks.task_id")
}

func (db *DB) ClaimNextPendingTask(ctx context.Context, startedAt time.Time) (task.Record, error) {
	db.claimMu.Lock()
	defer db.claimMu.Unlock()

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return task.Record{}, fmt.Errorf("begin claim task transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	record, err := claimNextPendingTask(ctx, tx, startedAt)
	if err != nil {
		return task.Record{}, err
	}

	if err := tx.Commit(); err != nil {
		return task.Record{}, fmt.Errorf("commit claim task transaction: %w", err)
	}
	return record, nil
}

func (db *DB) ClaimNextPendingTaskForNode(ctx context.Context, nodeID string, startedAt, leaseExpiresAt time.Time) (task.Record, error) {
	db.claimMu.Lock()
	defer db.claimMu.Unlock()

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return task.Record{}, fmt.Errorf("begin claim task transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	record, err := claimNextPendingTaskForNode(ctx, tx, nodeID, startedAt, leaseExpiresAt)
	if err != nil {
		return task.Record{}, err
	}

	if err := tx.Commit(); err != nil {
		return task.Record{}, fmt.Errorf("commit claim task transaction: %w", err)
	}
	return record, nil
}

func (db *DB) ClaimNextPendingTaskOfType(ctx context.Context, taskType task.Type, startedAt time.Time) (task.Record, error) {
	db.claimMu.Lock()
	defer db.claimMu.Unlock()

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return task.Record{}, fmt.Errorf("begin claim task transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	record, err := claimNextPendingTaskOfType(ctx, tx, taskType, startedAt)
	if err != nil {
		return task.Record{}, err
	}

	if err := tx.Commit(); err != nil {
		return task.Record{}, fmt.Errorf("commit claim task transaction: %w", err)
	}
	return record, nil
}

func claimNextPendingTask(ctx context.Context, tx *sql.Tx, startedAt time.Time) (task.Record, error) {
	return claimPendingTaskByQuery(ctx, tx, startedAt, "query next pending task", `
		SELECT task_id, type, source, COALESCE(triggered_by, ''), COALESCE(service_name, ''), COALESCE(node_id, ''),
		       status, COALESCE(params_json, ''), COALESCE(log_path, ''), COALESCE(repo_revision, ''),
		       COALESCE(result_revision, ''), COALESCE(attempt_of_task_id, ''), created_at
		FROM tasks
		WHERE status = ?
		ORDER BY created_at ASC, task_id ASC
		LIMIT 1
	`, string(task.StatusPending))
}

func claimNextPendingTaskForNode(ctx context.Context, tx *sql.Tx, nodeID string, startedAt, leaseExpiresAt time.Time) (task.Record, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT task_id, type, source, COALESCE(triggered_by, ''), COALESCE(service_name, ''), COALESCE(node_id, ''),
		       status, COALESCE(params_json, ''), COALESCE(log_path, ''), COALESCE(repo_revision, ''),
		       COALESCE(result_revision, ''), COALESCE(attempt_of_task_id, ''), created_at
		FROM tasks
		WHERE status = ? AND node_id = ? AND type NOT IN (?, ?)
		ORDER BY created_at ASC, task_id ASC
		LIMIT 1
	`, string(task.StatusPending), nodeID, string(task.TypeDNSUpdate), string(task.TypeMigrate))
	record, createdAt, err := scanPendingTaskRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.Record{}, ErrNoPendingTask
		}
		return task.Record{}, fmt.Errorf("query next pending task for node %q: %w", nodeID, err)
	}
	executionID := uuid.NewString()
	updated, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, started_at = COALESCE(started_at, ?), execution_id = ?, execution_state = ?,
		    lease_expires_at = ?, execution_accepted_at = NULL, execution_heartbeat_at = NULL
		WHERE task_id = ? AND status = ?
	`, string(task.StatusRunning), startedAt.UTC().Format(time.RFC3339), executionID, string(task.ExecutionOffered), leaseExpiresAt.UTC().Format(time.RFC3339), record.TaskID, string(task.StatusPending))
	if err != nil {
		return task.Record{}, fmt.Errorf("offer task %q: %w", record.TaskID, err)
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return task.Record{}, fmt.Errorf("read offered task rows affected: %w", err)
	}
	if affected == 0 {
		return task.Record{}, ErrNoPendingTask
	}
	record.CreatedAt = createdAt.UTC()
	record.Status = task.StatusRunning
	record.StartedAt = timePtr(startedAt.UTC())
	record.ExecutionID = executionID
	record.ExecutionState = task.ExecutionOffered
	record.LeaseExpiresAt = timePtr(leaseExpiresAt.UTC())
	return record, nil
}

func claimNextPendingTaskOfType(ctx context.Context, tx *sql.Tx, taskType task.Type, startedAt time.Time) (task.Record, error) {
	return claimPendingTaskByQuery(ctx, tx, startedAt, fmt.Sprintf("query next pending task of type %q", taskType), `
		SELECT task_id, type, source, COALESCE(triggered_by, ''), COALESCE(service_name, ''), COALESCE(node_id, ''),
		       status, COALESCE(params_json, ''), COALESCE(log_path, ''), COALESCE(repo_revision, ''),
		       COALESCE(result_revision, ''), COALESCE(attempt_of_task_id, ''), created_at
		FROM tasks
		WHERE status = ? AND type = ?
		ORDER BY created_at ASC, task_id ASC
		LIMIT 1
	`, string(task.StatusPending), string(taskType))
}

func claimPendingTaskByQuery(ctx context.Context, tx *sql.Tx, startedAt time.Time, queryLabel, query string, args ...any) (task.Record, error) {
	row := tx.QueryRowContext(ctx, query, args...)

	record, createdAt, err := scanPendingTaskRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.Record{}, ErrNoPendingTask
		}
		return task.Record{}, fmt.Errorf("%s: %w", queryLabel, err)
	}

	if err := markTaskRunning(ctx, tx, record.TaskID, startedAt); err != nil {
		return task.Record{}, err
	}

	record.CreatedAt = createdAt.UTC()
	record.Status = task.StatusRunning
	record.StartedAt = timePtr(startedAt.UTC())
	return record, nil
}

func scanPendingTaskRecord(row *sql.Row) (task.Record, time.Time, error) {
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
		return task.Record{}, time.Time{}, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return task.Record{}, time.Time{}, fmt.Errorf("parse task %q created_at: %w", record.TaskID, err)
	}

	record.Type = task.Type(rawType)
	record.Source = task.Source(rawSource)
	record.Status = task.Status(rawStatus)
	return record, parsedCreatedAt, nil
}

func markTaskRunning(ctx context.Context, tx *sql.Tx, taskID string, startedAt time.Time) error {
	updated, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, started_at = COALESCE(started_at, ?)
		WHERE task_id = ? AND status = ?
	`, string(task.StatusRunning), startedAt.UTC().Format(time.RFC3339), taskID, string(task.StatusPending))
	if err != nil {
		return fmt.Errorf("mark task %q running: %w", taskID, err)
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return fmt.Errorf("read claim task rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNoPendingTask
	}
	return nil
}

func (db *DB) TransitionTaskStatus(ctx context.Context, taskID string, fromStatus, toStatus task.Status, errorSummary string) error {
	if taskID == "" {
		return errors.New("task_id is required")
	}
	if fromStatus == "" || toStatus == "" {
		return errors.New("from_status and to_status are required")
	}

	result, err := db.sql.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, error_summary = ?, finished_at = NULL
		WHERE task_id = ? AND status = ?
	`, string(toStatus), nullableString(errorSummary), taskID, string(fromStatus))
	if err != nil {
		return fmt.Errorf("transition task %q from %q to %q: %w", taskID, fromStatus, toStatus, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read transition rows affected for task %q: %w", taskID, err)
	}
	if affected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (db *DB) UpdateTaskParamsJSON(ctx context.Context, taskID, paramsJSON string) error {
	if taskID == "" {
		return errors.New("task_id is required")
	}
	result, err := db.sql.ExecContext(ctx, `
		UPDATE tasks
		SET params_json = ?
		WHERE task_id = ?
	`, nullableString(paramsJSON), taskID)
	if err != nil {
		return fmt.Errorf("update params_json for task %q: %w", taskID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update params_json rows affected for task %q: %w", taskID, err)
	}
	if affected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (db *DB) CompleteTask(ctx context.Context, taskID string, status task.Status, finishedAt time.Time, errorSummary string) error {
	if taskID == "" {
		return errors.New("task_id is required")
	}
	if status != task.StatusSucceeded && status != task.StatusFailed && status != task.StatusCancelled {
		return fmt.Errorf("invalid terminal task status %q", status)
	}

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin complete task transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, finished_at = ?, error_summary = ?,
		    execution_state = CASE WHEN execution_id IS NULL THEN execution_state ELSE ? END,
		    lease_expires_at = NULL
		WHERE task_id = ? AND status IN (?, ?, ?)
	`, string(status), finishedAt.UTC().Format(time.RFC3339), nullableString(errorSummary), string(task.ExecutionCompleted), taskID, string(task.StatusPending), string(task.StatusRunning), string(task.StatusAwaitingConfirmation))
	if err != nil {
		return fmt.Errorf("complete task %q: %w", taskID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read complete task rows affected for %q: %w", taskID, err)
	}
	if affected == 0 {
		return fmt.Errorf("complete task %q: task is missing or already terminal", taskID)
	}
	if err := updateServiceFromCompletedTask(ctx, tx, taskID, status, finishedAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE task_steps SET status = ?, finished_at = COALESCE(finished_at, ?)
		WHERE task_id = ? AND status = ?
	`, string(status), finishedAt.UTC().Format(time.RFC3339), taskID, string(task.StatusRunning)); err != nil {
		return fmt.Errorf("finalize running steps for task %q: %w", taskID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit complete task transaction: %w", err)
	}
	return nil
}

func updateRuntimeFromAgentTask(ctx context.Context, tx *sql.Tx, taskID string, status task.Status, updatedAt time.Time) error {
	var taskType task.Type
	var serviceName, nodeID string
	if err := tx.QueryRowContext(ctx, `SELECT type, COALESCE(service_name, ''), COALESCE(node_id, '') FROM tasks WHERE task_id = ?`, taskID).Scan(&taskType, &serviceName, &nodeID); err != nil {
		return err
	}
	if serviceName == "" || nodeID == "" {
		return nil
	}
	if taskType != task.TypeDeploy && taskType != task.TypeUpdate && taskType != task.TypeStop && taskType != task.TypeRestart {
		return nil
	}
	runtimeStatus := ""
	switch status {
	case task.StatusFailed:
		runtimeStatus = ServiceRuntimeError
	case task.StatusSucceeded:
		if taskType == task.TypeStop {
			runtimeStatus = ServiceRuntimeStopped
		} else {
			runtimeStatus = ServiceRuntimeRunning
		}
	case task.StatusCancelled, task.StatusPending, task.StatusRunning, task.StatusAwaitingConfirmation:
		return nil
	}
	if runtimeStatus == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE service_instances SET runtime_status = ?, updated_at = ? WHERE service_name = ? AND node_id = ?`, runtimeStatus, updatedAt.UTC().Format(time.RFC3339), serviceName, nodeID); err != nil {
		return fmt.Errorf("update completed task runtime status: %w", err)
	}
	return refreshServiceAggregateStatusTx(ctx, tx, serviceName)
}

func (db *DB) AcknowledgeTaskExecution(ctx context.Context, taskID, executionID string, acceptedAt, leaseExpiresAt time.Time) error {
	result, err := db.sql.ExecContext(ctx, `
		UPDATE tasks
		SET execution_state = ?, execution_accepted_at = COALESCE(execution_accepted_at, ?),
		    execution_heartbeat_at = ?, lease_expires_at = ?
		WHERE task_id = ? AND execution_id = ? AND status = ?
		  AND (execution_state = ? OR (execution_state = ? AND lease_expires_at >= ?))
	`, string(task.ExecutionAccepted), acceptedAt.UTC().Format(time.RFC3339), acceptedAt.UTC().Format(time.RFC3339), leaseExpiresAt.UTC().Format(time.RFC3339), taskID, executionID, string(task.StatusRunning), string(task.ExecutionAccepted), string(task.ExecutionOffered), acceptedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("acknowledge task execution %q: %w", taskID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read acknowledged task rows affected: %w", err)
	}
	if affected == 0 {
		return ErrTaskExecutionMismatch
	}
	return nil
}

func (db *DB) RenewTaskExecution(ctx context.Context, taskID, executionID string, renewedAt, leaseExpiresAt time.Time) error {
	result, err := db.sql.ExecContext(ctx, `
		UPDATE tasks SET execution_state = ?, execution_heartbeat_at = ?, lease_expires_at = ?
		WHERE task_id = ? AND execution_id = ? AND status = ? AND execution_state IN (?, ?)
	`, string(task.ExecutionAccepted), renewedAt.UTC().Format(time.RFC3339), leaseExpiresAt.UTC().Format(time.RFC3339), taskID, executionID, string(task.StatusRunning), string(task.ExecutionAccepted), string(task.ExecutionLeaseLost))
	if err != nil {
		return fmt.Errorf("renew task execution %q: %w", taskID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read renewed task rows affected: %w", err)
	}
	if affected == 0 {
		return ErrTaskExecutionMismatch
	}
	return nil
}

func (db *DB) ValidateTaskExecution(ctx context.Context, taskID, executionID, nodeID string) (task.Record, error) {
	detail, err := db.GetTask(ctx, taskID)
	if err != nil {
		return task.Record{}, err
	}
	record := detail.Record
	if executionID == "" && record.ExecutionID == "" && record.NodeID == nodeID && record.Status == task.StatusRunning {
		return record, nil
	}
	if executionID == "" || record.ExecutionID != executionID || record.NodeID != nodeID || record.Status != task.StatusRunning {
		return task.Record{}, ErrTaskExecutionMismatch
	}
	switch record.ExecutionState {
	case task.ExecutionAccepted, task.ExecutionLeaseLost:
		return record, nil
	default:
		return task.Record{}, ErrTaskExecutionMismatch
	}
}

func (db *DB) CompleteTaskExecution(ctx context.Context, taskID, executionID string, status task.Status, finishedAt time.Time, errorSummary string) error {
	if status != task.StatusSucceeded && status != task.StatusFailed && status != task.StatusCancelled {
		return fmt.Errorf("invalid terminal task status %q", status)
	}
	if executionID == "" {
		detail, err := db.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
		if detail.Record.ExecutionID != "" {
			return ErrTaskExecutionMismatch
		}
		return db.CompleteTask(ctx, taskID, status, finishedAt, errorSummary)
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin complete task execution transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var currentStatus, currentExecutionID, currentExecutionState, currentErrorSummary string
	if err := tx.QueryRowContext(ctx, `SELECT status, COALESCE(execution_id, ''), COALESCE(execution_state, ''), COALESCE(error_summary, '') FROM tasks WHERE task_id = ?`, taskID).Scan(&currentStatus, &currentExecutionID, &currentExecutionState, &currentErrorSummary); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTaskNotFound
		}
		return fmt.Errorf("read task execution %q: %w", taskID, err)
	}
	if currentExecutionID != executionID || executionID == "" {
		return ErrTaskExecutionMismatch
	}
	if task.Status(currentStatus) != task.StatusRunning {
		if task.Status(currentStatus) == status && currentErrorSummary == errorSummary {
			return nil
		}
		return ErrTaskExecutionConflict
	}
	if task.ExecutionState(currentExecutionState) != task.ExecutionAccepted && task.ExecutionState(currentExecutionState) != task.ExecutionLeaseLost {
		return ErrTaskExecutionMismatch
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?, finished_at = ?, error_summary = ?, execution_state = ?, lease_expires_at = NULL
		WHERE task_id = ? AND execution_id = ? AND status = ?
	`, string(status), finishedAt.UTC().Format(time.RFC3339), nullableString(errorSummary), string(task.ExecutionCompleted), taskID, executionID, string(task.StatusRunning))
	if err != nil {
		return fmt.Errorf("complete task execution %q: %w", taskID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read completed task execution rows affected: %w", err)
	}
	if affected == 0 {
		return ErrTaskExecutionConflict
	}
	if err := updateServiceFromCompletedTask(ctx, tx, taskID, status, finishedAt); err != nil {
		return err
	}
	if err := updateRuntimeFromAgentTask(ctx, tx, taskID, status, finishedAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE task_steps SET status = ?, finished_at = COALESCE(finished_at, ?)
		WHERE task_id = ? AND status = ?
	`, string(status), finishedAt.UTC().Format(time.RFC3339), taskID, string(task.StatusRunning)); err != nil {
		return fmt.Errorf("finalize running steps for task %q: %w", taskID, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO task_outbox (event_id, task_id, event_type, next_attempt_at, created_at)
		VALUES (?, ?, 'task_completed', ?, ?)
		ON CONFLICT(task_id, event_type) DO NOTHING
	`, uuid.NewString(), taskID, finishedAt.UTC().Format(time.RFC3339), finishedAt.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("enqueue completed task %q: %w", taskID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit completed task execution %q: %w", taskID, err)
	}
	return nil
}

func (db *DB) SweepExpiredTaskExecutions(ctx context.Context, now time.Time) (int64, int64, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin task execution sweep: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	requeued, err := tx.ExecContext(ctx, `
		UPDATE tasks SET status = ?, started_at = NULL, execution_id = NULL, execution_state = NULL,
		    lease_expires_at = NULL, execution_accepted_at = NULL, execution_heartbeat_at = NULL
		WHERE status = ? AND execution_state = ? AND lease_expires_at < ?
	`, string(task.StatusPending), string(task.StatusRunning), string(task.ExecutionOffered), now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, 0, fmt.Errorf("requeue expired task offers: %w", err)
	}
	lost, err := tx.ExecContext(ctx, `
		UPDATE tasks SET execution_state = ?
		WHERE status = ? AND execution_state = ? AND lease_expires_at < ?
	`, string(task.ExecutionLeaseLost), string(task.StatusRunning), string(task.ExecutionAccepted), now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, 0, fmt.Errorf("mark expired task leases lost: %w", err)
	}
	requeuedCount, _ := requeued.RowsAffected()
	lostCount, _ := lost.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit task execution sweep: %w", err)
	}
	return requeuedCount, lostCount, nil
}

func (db *DB) FailLostTaskExecution(ctx context.Context, taskID string, finishedAt time.Time, errorSummary string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin fail lost task execution: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
		UPDATE tasks SET status = ?, finished_at = ?, error_summary = ?, execution_state = ?, lease_expires_at = NULL
		WHERE task_id = ? AND status = ? AND execution_state = ?
	`, string(task.StatusFailed), finishedAt.UTC().Format(time.RFC3339), errorSummary, string(task.ExecutionCompleted), taskID, string(task.StatusRunning), string(task.ExecutionLeaseLost))
	if err != nil {
		return fmt.Errorf("fail lost task execution %q: %w", taskID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read failed lost task rows affected: %w", err)
	}
	if affected == 0 {
		return ErrTaskExecutionMismatch
	}
	if err := updateServiceFromCompletedTask(ctx, tx, taskID, task.StatusFailed, finishedAt); err != nil {
		return err
	}
	if err := updateRuntimeFromAgentTask(ctx, tx, taskID, task.StatusFailed, finishedAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE task_steps SET status = ?, finished_at = COALESCE(finished_at, ?) WHERE task_id = ? AND status = ?`, string(task.StatusFailed), finishedAt.UTC().Format(time.RFC3339), taskID, string(task.StatusRunning)); err != nil {
		return fmt.Errorf("fail running task steps: %w", err)
	}
	return tx.Commit()
}

func (db *DB) HasActiveServiceTask(ctx context.Context, serviceName string) (bool, error) {
	return hasActiveServiceTask(ctx, db.sql, serviceName)
}

func hasActiveServiceTaskTx(ctx context.Context, tx *sql.Tx, serviceName string) (bool, error) {
	return hasActiveServiceTask(ctx, tx, serviceName)
}

func hasActiveServiceTask(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, serviceName string,
) (bool, error) {
	var count int
	err := queryer.QueryRowContext(ctx, `
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

func (db *DB) HasActiveServiceInstanceTask(ctx context.Context, serviceName, nodeID string) (bool, error) {
	return hasActiveServiceInstanceTask(ctx, db.sql, serviceName, nodeID)
}

func hasActiveServiceInstanceTaskTx(ctx context.Context, tx *sql.Tx, serviceName, nodeID string) (bool, error) {
	return hasActiveServiceInstanceTask(ctx, tx, serviceName, nodeID)
}

func hasActiveServiceInstanceTask(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, serviceName, nodeID string,
) (bool, error) {
	var count int
	err := queryer.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM tasks
		WHERE service_name = ? AND node_id = ? AND status IN (?, ?, ?)
	`,
		serviceName,
		nodeID,
		string(task.StatusPending),
		string(task.StatusRunning),
		string(task.StatusAwaitingConfirmation),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count active tasks for service instance %q@%q: %w", serviceName, nodeID, err)
	}
	return count > 0, nil
}

func (db *DB) HasMatchingTaskInWindow(ctx context.Context, source task.Source, taskType task.Type, serviceName, nodeID, paramsJSON string, windowStart time.Time) (bool, error) {
	windowEnd := windowStart.UTC().Add(time.Minute)
	var count int
	err := db.sql.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM tasks
		WHERE source = ?
		  AND type = ?
		  AND COALESCE(service_name, '') = ?
		  AND COALESCE(node_id, '') = ?
		  AND COALESCE(params_json, '') = ?
		  AND created_at >= ?
		  AND created_at < ?
	`,
		string(source),
		string(taskType),
		serviceName,
		nodeID,
		paramsJSON,
		windowStart.UTC().Format(time.RFC3339),
		windowEnd.Format(time.RFC3339),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count matching tasks in window: %w", err)
	}
	return count > 0, nil
}

func (db *DB) TaskNodeID(ctx context.Context, taskID string) (string, error) {
	var nodeID string
	if err := db.sql.QueryRowContext(ctx, `SELECT COALESCE(node_id, '') FROM tasks WHERE task_id = ?`, taskID).Scan(&nodeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrTaskNotFound
		}
		return "", fmt.Errorf("read task node_id for %q: %w", taskID, err)
	}
	return nodeID, nil
}

func (db *DB) ListTasks(
	ctx context.Context,
	statusFilters, serviceNameFilters, nodeIDFilters, typeFilters []string,
	excludeStatusFilters, excludeServiceNameFilters, excludeNodeIDFilters, excludeTypeFilters []string,
	page, limit uint32,
) ([]TaskSummary, uint32, error) {
	if limit == 0 {
		limit = 100
	}
	if page == 0 {
		page = 1
	}

	whereClause := "WHERE 1 = 1"
	args := make([]any, 0, 20)

	whereClause, args = appendTaskFilterInClause(whereClause, args, "status", statusFilters)
	whereClause, args = appendTaskFilterInClause(whereClause, args, "service_name", serviceNameFilters)
	whereClause, args = appendTaskFilterInClause(whereClause, args, "node_id", nodeIDFilters)
	whereClause, args = appendTaskFilterInClause(whereClause, args, "type", typeFilters)
	whereClause, args = appendTaskFilterNotInClause(whereClause, args, "status", excludeStatusFilters)
	whereClause, args = appendTaskFilterNotInClause(whereClause, args, "service_name", excludeServiceNameFilters)
	whereClause, args = appendTaskFilterNotInClause(whereClause, args, "node_id", excludeNodeIDFilters)
	whereClause, args = appendTaskFilterNotInClause(whereClause, args, "type", excludeTypeFilters)

	var totalCount uint32
	countQuery := `SELECT COUNT(*) FROM tasks ` + whereClause
	if err := db.sql.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	offset := (page - 1) * limit
	query := `SELECT task_id, type, status, COALESCE(service_name, ''), COALESCE(node_id, ''), created_at FROM tasks ` + whereClause
	query += ` ORDER BY created_at DESC, task_id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tasks := make([]TaskSummary, 0, limit)
	for rows.Next() {
		var record TaskSummary
		if err := rows.Scan(&record.TaskID, &record.Type, &record.Status, &record.ServiceName, &record.NodeID, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan task summary: %w", err)
		}
		tasks = append(tasks, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate task summaries: %w", err)
	}

	return tasks, totalCount, nil
}

func appendTaskFilterInClause(whereClause string, args []any, column string, values []string) (string, []any) {
	return appendStringFilterInClause(whereClause, args, column, values)
}

func appendTaskFilterNotInClause(whereClause string, args []any, column string, values []string) (string, []any) {
	return appendStringFilterNotInClause(whereClause, args, column, values)
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
	defer func() { _ = rows.Close() }()

	steps := make([]task.StepRecord, 0)
	for rows.Next() {
		var stepName string
		var status string
		var startedAt string
		var finishedAt string
		if err := rows.Scan(&stepName, &status, &startedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("scan task step for %q: %w", taskID, err)
		}
		parsedStartedAt, err := parseNullableRFC3339(startedAt, fmt.Sprintf("task %q step %q started_at", taskID, stepName))
		if err != nil {
			return nil, err
		}
		parsedFinishedAt, err := parseNullableRFC3339(finishedAt, fmt.Sprintf("task %q step %q finished_at", taskID, stepName))
		if err != nil {
			return nil, err
		}
		steps = append(steps, task.StepRecord{
			TaskID:     taskID,
			StepName:   task.StepName(stepName),
			Status:     task.Status(status),
			StartedAt:  parsedStartedAt,
			FinishedAt: parsedFinishedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task steps for %q: %w", taskID, err)
	}
	return steps, nil
}

func (db *DB) UpsertTaskStep(ctx context.Context, step task.StepRecord) error {
	if step.TaskID == "" {
		return errors.New("task step task_id is required")
	}
	if step.StepName == "" {
		return errors.New("task step step_name is required")
	}
	if step.Status == "" {
		return errors.New("task step status is required")
	}

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin task step upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var current string
	err = tx.QueryRowContext(ctx, `SELECT status FROM task_steps WHERE task_id = ? AND step_name = ?`, step.TaskID, string(step.StepName)).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read task step %q for task %q: %w", step.StepName, step.TaskID, err)
	}
	if err == nil {
		currentStatus := task.Status(current)
		if isTerminalStatus(currentStatus) {
			if currentStatus == step.Status {
				return nil
			}
			return ErrTaskExecutionConflict
		}
		if currentStatus == task.StatusRunning && step.Status == task.StatusPending {
			return ErrTaskExecutionConflict
		}
	}
	_, err = tx.ExecContext(ctx, `
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
	return tx.Commit()
}

func isTerminalStatus(status task.Status) bool {
	return status == task.StatusSucceeded || status == task.StatusFailed || status == task.StatusCancelled
}

func (db *DB) getTaskRecord(ctx context.Context, taskID string) (task.Record, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT task_id, type, source, COALESCE(triggered_by, ''), COALESCE(service_name, ''), COALESCE(node_id, ''),
		       status, COALESCE(params_json, ''), COALESCE(log_path, ''), COALESCE(repo_revision, ''),
		       COALESCE(result_revision, ''), COALESCE(attempt_of_task_id, ''), created_at,
		       COALESCE(started_at, ''), COALESCE(finished_at, ''), COALESCE(error_summary, ''),
		       COALESCE(execution_id, ''), COALESCE(execution_state, ''), COALESCE(lease_expires_at, ''),
		       COALESCE(execution_accepted_at, ''), COALESCE(execution_heartbeat_at, '')
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
	var leaseExpiresAt, executionAcceptedAt, executionHeartbeatAt string
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
		&record.ExecutionID,
		&record.ExecutionState,
		&leaseExpiresAt,
		&executionAcceptedAt,
		&executionHeartbeatAt,
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
	parsedStartedAt, err := parseNullableRFC3339(startedAt, fmt.Sprintf("task %q started_at", taskID))
	if err != nil {
		return task.Record{}, err
	}
	parsedFinishedAt, err := parseNullableRFC3339(finishedAt, fmt.Sprintf("task %q finished_at", taskID))
	if err != nil {
		return task.Record{}, err
	}
	record.StartedAt = parsedStartedAt
	record.FinishedAt = parsedFinishedAt
	record.LeaseExpiresAt, err = parseNullableRFC3339(leaseExpiresAt, fmt.Sprintf("task %q lease_expires_at", taskID))
	if err != nil {
		return task.Record{}, err
	}
	record.ExecutionAcceptedAt, err = parseNullableRFC3339(executionAcceptedAt, fmt.Sprintf("task %q execution_accepted_at", taskID))
	if err != nil {
		return task.Record{}, err
	}
	record.ExecutionHeartbeatAt, err = parseNullableRFC3339(executionHeartbeatAt, fmt.Sprintf("task %q execution_heartbeat_at", taskID))
	if err != nil {
		return task.Record{}, err
	}
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

func parseNullableRFC3339(value, label string) (*time.Time, error) {
	if value == "" {
		return nil, nil //nolint:nilnil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", label, err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func updateServiceFromCompletedTask(ctx context.Context, tx *sql.Tx, taskID string, status task.Status, finishedAt time.Time) error {
	var serviceName string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(service_name, '') FROM tasks WHERE task_id = ?`, taskID).Scan(&serviceName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTaskNotFound
		}
		return fmt.Errorf("read completed task %q for service refresh: %w", taskID, err)
	}
	if serviceName == "" {
		return nil
	}
	var nodeID string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(node_id, '') FROM tasks WHERE task_id = ?`, taskID).Scan(&nodeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTaskNotFound
		}
		return fmt.Errorf("read completed task %q node for service refresh: %w", taskID, err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE services
		SET last_task_id = ?, updated_at = ?
		WHERE service_name = ?
	`, taskID, finishedAt.UTC().Format(time.RFC3339), serviceName); err != nil {
		return fmt.Errorf("update service summary for %q: %w", serviceName, err)
	}
	if nodeID != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE service_instances
			SET last_task_id = ?, updated_at = ?
			WHERE service_name = ? AND node_id = ?
		`, taskID, finishedAt.UTC().Format(time.RFC3339), serviceName, nodeID); err != nil {
			return fmt.Errorf("update service instance summary for %q@%q: %w", serviceName, nodeID, err)
		}
		if err := refreshServiceAggregateStatusTx(ctx, tx, serviceName); err != nil {
			return err
		}
	}
	return nil
}
