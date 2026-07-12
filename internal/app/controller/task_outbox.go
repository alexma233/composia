package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func runTaskOutbox(ctx context.Context, server *agentReportServer) {
	for {
		event, err := server.db.NextTaskOutboxEvent(ctx, time.Now().UTC())
		if errors.Is(err, store.ErrNoOutboxEvent) {
			if !sleepContext(ctx, time.Second) {
				return
			}
			continue
		}
		if err != nil {
			log.Printf("task outbox read failed: %v", err)
			if !sleepContext(ctx, time.Second) {
				return
			}
			continue
		}
		if err := server.processTaskOutboxEvent(ctx, event); err != nil {
			attempts := event.Attempts + 1
			delay := minDuration(time.Duration(1<<min(attempts, 9))*time.Second, 10*time.Minute)
			if retryErr := server.db.RetryTaskOutboxEvent(ctx, event.EventID, attempts, time.Now().UTC().Add(delay), err); retryErr != nil {
				log.Printf("task outbox retry failed: %v", retryErr)
			}
			continue
		}
		if err := server.db.CompleteTaskOutboxEvent(ctx, event.EventID, time.Now().UTC()); err != nil {
			log.Printf("task outbox completion failed: %v", err)
		}
	}
}

func (server *agentReportServer) processTaskOutboxEvent(ctx context.Context, event store.TaskOutboxEvent) error {
	detail, err := server.db.GetTask(ctx, event.TaskID)
	if err != nil {
		return err
	}
	switch event.EventType {
	case "task_completed":
		if !event.FollowupsCompleted {
			if err := server.queuePostTaskFollowups(ctx, detail.Record); err != nil {
				return err
			}
			if err := server.db.CompleteTaskOutboxFollowups(ctx, event.EventID, time.Now().UTC()); err != nil {
				return err
			}
		}
		if !event.NotificationDispatched {
			if eventType, ok := taskEventTypeForStatus(detail.Record.Status); ok {
				if err := sendTaskRecordNotification(ctx, server.notifier, eventType, detail.Record); err != nil {
					return err
				}
			}
			if err := server.db.CompleteTaskOutboxNotification(ctx, event.EventID, time.Now().UTC()); err != nil {
				return err
			}
		}
	case "image_update_applied":
		updateRecord, imageNames, err := server.persistedImageUpdateApplied(ctx, detail.Record)
		if err != nil {
			return err
		}
		if !event.NotificationDispatched {
			if err := sendImageUpdateAppliedNotification(ctx, server.notifier, detail.Record, updateRecord, imageNames); err != nil {
				return err
			}
			if err := server.db.CompleteTaskOutboxNotification(ctx, event.EventID, time.Now().UTC()); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported task outbox event type %q", event.EventType)
	}
	return nil
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
