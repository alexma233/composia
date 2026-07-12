package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrNoOutboxEvent = errors.New("no outbox event")

type TaskOutboxEvent struct {
	EventID                string
	TaskID                 string
	EventType              string
	Attempts               int
	FollowupsCompleted     bool
	NotificationDispatched bool
}

func (db *DB) NextTaskOutboxEvent(ctx context.Context, now time.Time) (TaskOutboxEvent, error) {
	var event TaskOutboxEvent
	err := db.sql.QueryRowContext(ctx, `
		SELECT event_id, task_id, event_type, attempts,
		       followups_completed_at IS NOT NULL, notification_dispatched_at IS NOT NULL
		FROM task_outbox
		WHERE processed_at IS NULL AND next_attempt_at <= ?
		ORDER BY created_at, event_id
		LIMIT 1
	`, now.UTC().Format(time.RFC3339)).Scan(&event.EventID, &event.TaskID, &event.EventType, &event.Attempts, &event.FollowupsCompleted, &event.NotificationDispatched)
	if errors.Is(err, sql.ErrNoRows) {
		return TaskOutboxEvent{}, ErrNoOutboxEvent
	}
	if err != nil {
		return TaskOutboxEvent{}, fmt.Errorf("read next task outbox event: %w", err)
	}
	return event, nil
}

func (db *DB) EnqueueTaskOutboxEvent(ctx context.Context, taskID, eventType string, createdAt time.Time) error {
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO task_outbox (event_id, task_id, event_type, next_attempt_at, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(task_id, event_type) DO NOTHING
	`, uuid.NewString(), taskID, eventType, createdAt.UTC().Format(time.RFC3339), createdAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("enqueue task outbox event for %q: %w", taskID, err)
	}
	return nil
}

func (db *DB) CompleteTaskOutboxFollowups(ctx context.Context, eventID string, completedAt time.Time) error {
	_, err := db.sql.ExecContext(ctx, `UPDATE task_outbox SET followups_completed_at = COALESCE(followups_completed_at, ?) WHERE event_id = ?`, completedAt.UTC().Format(time.RFC3339), eventID)
	if err != nil {
		return fmt.Errorf("complete task outbox followups %q: %w", eventID, err)
	}
	return nil
}

func (db *DB) CompleteTaskOutboxNotification(ctx context.Context, eventID string, dispatchedAt time.Time) error {
	_, err := db.sql.ExecContext(ctx, `UPDATE task_outbox SET notification_dispatched_at = COALESCE(notification_dispatched_at, ?) WHERE event_id = ?`, dispatchedAt.UTC().Format(time.RFC3339), eventID)
	if err != nil {
		return fmt.Errorf("complete task outbox notification %q: %w", eventID, err)
	}
	return nil
}

func (db *DB) CompleteTaskOutboxEvent(ctx context.Context, eventID string, processedAt time.Time) error {
	_, err := db.sql.ExecContext(ctx, `UPDATE task_outbox SET processed_at = ?, last_error = NULL WHERE event_id = ?`, processedAt.UTC().Format(time.RFC3339), eventID)
	if err != nil {
		return fmt.Errorf("complete task outbox event %q: %w", eventID, err)
	}
	return nil
}

func (db *DB) RetryTaskOutboxEvent(ctx context.Context, eventID string, attempts int, nextAttemptAt time.Time, eventErr error) error {
	_, err := db.sql.ExecContext(ctx, `UPDATE task_outbox SET attempts = ?, next_attempt_at = ?, last_error = ? WHERE event_id = ?`, attempts, nextAttemptAt.UTC().Format(time.RFC3339), eventErr.Error(), eventID)
	if err != nil {
		return fmt.Errorf("retry task outbox event %q: %w", eventID, err)
	}
	return nil
}
