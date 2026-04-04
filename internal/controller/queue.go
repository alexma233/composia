package controller

import (
	"context"
	"errors"
	"log"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

const taskWorkerPollInterval = 1 * time.Second

type taskExecutor func(context.Context, task.Record) error

func startTaskWorker(ctx context.Context, db *store.DB, execute taskExecutor) {
	go runTaskWorker(ctx, db, execute)
}

func runTaskWorker(ctx context.Context, db *store.DB, execute taskExecutor) {
	ticker := time.NewTicker(taskWorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runSingleTask(ctx, db, execute)
		}
	}
}

func runSingleTask(ctx context.Context, db *store.DB, execute taskExecutor) {
	record, err := db.ClaimNextPendingTask(ctx, time.Now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrNoPendingTask) {
			return
		}
		log.Printf("task worker claim failed: %v", err)
		return
	}

	status := task.StatusSucceeded
	errorSummary := ""
	if err := execute(ctx, record); err != nil {
		status = task.StatusFailed
		errorSummary = err.Error()
	}

	if err := db.CompleteTask(ctx, record.TaskID, status, time.Now().UTC(), errorSummary); err != nil {
		log.Printf("task worker completion failed for %s: %v", record.TaskID, err)
	}
}
