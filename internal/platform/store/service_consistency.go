package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	ConsistencyUnknown       = "unknown"
	ConsistencyConsistent    = "consistent"
	ConsistencyDrifted       = "drifted"
	ConsistencyError         = "error"
	ConsistencyNotApplicable = "not_applicable"
)

type ServiceConsistencyOutcome struct {
	Status  string   `json:"status"`
	Reasons []string `json:"reasons,omitempty"`
}

type ServiceConsistencyCheck struct {
	Files        ServiceConsistencyOutcome `json:"files"`
	Compose      ServiceConsistencyOutcome `json:"compose"`
	CheckedAt    string                    `json:"checked_at"`
	RepoRevision string                    `json:"repo_revision"`
	TaskID       string                    `json:"task_id"`
}

func IsValidConsistencyStatus(status string) bool {
	switch status {
	case ConsistencyUnknown, ConsistencyConsistent, ConsistencyDrifted, ConsistencyError, ConsistencyNotApplicable:
		return true
	default:
		return false
	}
}

func decodeServiceConsistency(raw sql.NullString, snapshot *ServiceInstanceSnapshot) error {
	if !raw.Valid {
		return nil
	}
	if err := json.Unmarshal([]byte(raw.String), &snapshot.Consistency); err != nil {
		return fmt.Errorf("decode service consistency: %w", err)
	}
	return nil
}

// RecordServiceConsistencyCheck derives provenance and fences the write against execution changes.
// Runtime status and its timestamp deliberately remain untouched.
func (db *DB) RecordServiceConsistencyCheck(ctx context.Context, taskID, executionID, nodeID string, files, compose ServiceConsistencyOutcome) error {
	if !IsValidConsistencyStatus(files.Status) || !IsValidConsistencyStatus(compose.Status) {
		return errors.New("invalid consistency status")
	}
	if executionID == "" {
		return ErrTaskExecutionMismatch
	}
	record, err := db.ValidateTaskExecution(ctx, taskID, executionID, nodeID)
	if err != nil {
		return err
	}
	if record.ServiceName == "" || record.RepoRevision == "" {
		return errors.New("consistency requires a service-scoped task with a repo revision")
	}
	snapshot := ServiceConsistencyCheck{Files: files, Compose: compose, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), RepoRevision: record.RepoRevision, TaskID: record.TaskID}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	result, err := db.sql.ExecContext(ctx, `UPDATE service_instances SET consistency_json = ?
 WHERE service_name = ? AND node_id = ? AND EXISTS (
 SELECT 1 FROM tasks WHERE task_id = ? AND execution_id = ? AND node_id = ? AND service_name = ? AND repo_revision = ?
 AND status = 'running' AND execution_state IN ('accepted', 'lease_lost'))`, string(encoded), record.ServiceName, nodeID, taskID, executionID, nodeID, record.ServiceName, record.RepoRevision)
	if err != nil {
		return fmt.Errorf("record service consistency: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrTaskExecutionMismatch
	}
	return nil
}
