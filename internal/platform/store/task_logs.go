package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (db *DB) TaskLogConfirmedSeq(ctx context.Context, taskID string) (uint64, error) {
	var seq uint64
	if err := db.sql.QueryRowContext(ctx, `SELECT log_confirmed_seq FROM tasks WHERE task_id = ?`, taskID).Scan(&seq); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrTaskNotFound
		}
		return 0, fmt.Errorf("read task log confirmed seq for %q: %w", taskID, err)
	}
	return seq, nil
}

func (db *DB) SetTaskLogConfirmedSeq(ctx context.Context, taskID string, seq uint64) error {
	result, err := db.sql.ExecContext(ctx, `UPDATE tasks SET log_confirmed_seq = ? WHERE task_id = ? AND log_confirmed_seq < ?`, seq, taskID, seq)
	if err != nil {
		return fmt.Errorf("set task log confirmed seq for %q: %w", taskID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read task log confirmed seq rows for %q: %w", taskID, err)
	}
	if affected == 0 {
		current, err := db.TaskLogConfirmedSeq(ctx, taskID)
		if err != nil {
			return err
		}
		if current != seq {
			return fmt.Errorf("task log confirmed seq for %q is already %d", taskID, current)
		}
	}
	return nil
}
