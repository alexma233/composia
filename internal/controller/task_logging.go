package controller

import (
	"context"
	"log"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func logControllerAssignedTask(record task.Record) {
	log.Printf("controller assigned task: task_id=%s type=%s service=%s node=%s repo_revision=%s", record.TaskID, record.Type, record.ServiceName, record.NodeID, record.RepoRevision)
}

func logControllerReceivedTaskState(ctx context.Context, db *store.DB, taskID string, status task.Status, errorSummary string) {
	record, ok := controllerLogTaskRecord(ctx, db, taskID)
	if !ok {
		log.Printf("controller received task state: task_id=%s status=%s", taskID, status)
		return
	}
	if errorSummary != "" {
		log.Printf("controller received task state: task_id=%s type=%s service=%s node=%s status=%s error=%q", record.TaskID, record.Type, record.ServiceName, record.NodeID, status, errorSummary)
		return
	}
	log.Printf("controller received task state: task_id=%s type=%s service=%s node=%s status=%s", record.TaskID, record.Type, record.ServiceName, record.NodeID, status)
}

func logControllerReceivedTaskStepState(ctx context.Context, db *store.DB, step task.StepRecord) {
	record, ok := controllerLogTaskRecord(ctx, db, step.TaskID)
	if !ok {
		log.Printf("controller received task step state: task_id=%s step=%s status=%s", step.TaskID, step.StepName, step.Status)
		return
	}
	log.Printf("controller received task step state: task_id=%s type=%s service=%s node=%s step=%s status=%s", record.TaskID, record.Type, record.ServiceName, record.NodeID, step.StepName, step.Status)
}

func logControllerTaskStarted(record task.Record) {
	log.Printf("controller task started: task_id=%s type=%s service=%s node=%s repo_revision=%s", record.TaskID, record.Type, record.ServiceName, record.NodeID, record.RepoRevision)
}

func logControllerTaskFinished(record task.Record, finishedAt time.Time) {
	log.Printf("controller task finished: task_id=%s type=%s service=%s node=%s duration=%s", record.TaskID, record.Type, record.ServiceName, record.NodeID, controllerTaskDuration(record, finishedAt))
}

func logControllerTaskFailed(record task.Record, finishedAt time.Time, err error) {
	log.Printf("controller task failed: task_id=%s type=%s service=%s node=%s duration=%s error=%q", record.TaskID, record.Type, record.ServiceName, record.NodeID, controllerTaskDuration(record, finishedAt), err.Error())
}

func logControllerTaskStepStarted(record task.Record, stepName task.StepName) {
	log.Printf("controller task step started: task_id=%s type=%s service=%s node=%s step=%s", record.TaskID, record.Type, record.ServiceName, record.NodeID, stepName)
}

func logControllerTaskStepSucceeded(record task.Record, stepName task.StepName, startedAt, finishedAt time.Time) {
	log.Printf("controller task step succeeded: task_id=%s type=%s service=%s node=%s step=%s duration=%s", record.TaskID, record.Type, record.ServiceName, record.NodeID, stepName, finishedAt.Sub(startedAt).Round(time.Millisecond))
}

func logControllerTaskStepFailed(record task.Record, stepName task.StepName, err error) {
	log.Printf("controller task step failed: task_id=%s type=%s service=%s node=%s step=%s error=%q", record.TaskID, record.Type, record.ServiceName, record.NodeID, stepName, err.Error())
}

func logControllerTaskAwaitingConfirmation(record task.Record) {
	log.Printf("controller task awaiting confirmation: task_id=%s type=%s service=%s node=%s step=%s", record.TaskID, record.Type, record.ServiceName, record.NodeID, task.StepAwaitingConfirmation)
}

func controllerLogTaskRecord(ctx context.Context, db *store.DB, taskID string) (task.Record, bool) {
	detail, err := db.GetTask(ctx, taskID)
	if err != nil {
		log.Printf("controller task log lookup failed: task_id=%s error=%q", taskID, err.Error())
		return task.Record{}, false
	}
	return detail.Record, true
}

func controllerTaskDuration(record task.Record, finishedAt time.Time) time.Duration {
	if record.StartedAt == nil {
		return 0
	}
	return finishedAt.Sub(*record.StartedAt).Round(time.Millisecond)
}
