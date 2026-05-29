package controller

import (
	"context"
	"fmt"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func waitTask(ctx context.Context, db *store.DB, notifier *taskResultNotifier, taskID string, timeout, pollInterval time.Duration) error {
	deadline := time.Now().Add(timeout)
	waitCh := notifier.Subscribe(taskID)
	defer notifier.Unsubscribe(taskID, waitCh)
	for {
		detail, err := db.GetTask(ctx, taskID)
		if err == nil {
			switch detail.Record.Status {
			case task.StatusPending, task.StatusRunning, task.StatusAwaitingConfirmation:
				// Keep waiting.
			case task.StatusSucceeded:
				return nil
			case task.StatusFailed, task.StatusCancelled:
				return fmt.Errorf("task %s failed: %s", taskID, detail.Record.ErrorSummary)
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timeout waiting for task %s", taskID)
		}
		waitFor := remaining
		if pollInterval > 0 {
			waitFor = minDuration(remaining, pollInterval)
		}
		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-waitCh:
			timer.Stop()
		case <-timer.C:
		}
	}
}
