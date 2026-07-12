package controller

import (
	"context"
	"fmt"
	"time"

	appnotify "forgejo.alexma.top/alexma233/composia/internal/app/notify"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func dispatchTaskRecordNotification(notifier *appnotify.Notifier, eventType corenotify.EventType, record task.Record) {
	if notifier == nil {
		return
	}
	notifier.Dispatch(taskRecordNotificationEvent(eventType, record))
}

func sendTaskRecordNotification(ctx context.Context, notifier *appnotify.Notifier, eventType corenotify.EventType, record task.Record) error {
	return notifier.Send(ctx, taskRecordNotificationEvent(eventType, record))
}

func taskRecordNotificationEvent(eventType corenotify.EventType, record task.Record) appnotify.Event {
	return appnotify.Event{
		Type:       eventType,
		OccurredAt: derefTaskTime(record.FinishedAt, record.StartedAt),
		Source:     record.Source,
		Task: &appnotify.TaskEvent{
			TaskID:       record.TaskID,
			TaskType:     record.Type,
			Status:       record.Status,
			ServiceName:  record.ServiceName,
			NodeID:       record.NodeID,
			TriggeredBy:  record.TriggeredBy,
			ErrorSummary: record.ErrorSummary,
			StartedAt:    record.StartedAt,
			FinishedAt:   record.FinishedAt,
		},
	}
}

func dispatchBackupNotification(notifier *appnotify.Notifier, eventType corenotify.EventType, source task.Source, detail store.BackupDetail) {
	if notifier == nil {
		return
	}
	notifier.Dispatch(appnotify.Event{
		Type:       eventType,
		OccurredAt: timeFromBackup(detail),
		Source:     source,
		Backup: &appnotify.BackupEvent{
			TaskID:       detail.TaskID,
			BackupID:     detail.BackupID,
			ServiceName:  detail.ServiceName,
			NodeID:       detail.NodeID,
			DataName:     detail.DataName,
			Status:       detail.Status,
			ArtifactRef:  detail.ArtifactRef,
			ErrorSummary: detail.ErrorSummary,
		},
	})
}

func dispatchImageUpdateAvailableNotification(notifier *appnotify.Notifier, source task.Source, taskID string, check store.ServiceImageUpdateCheck) {
	if notifier == nil {
		return
	}
	notifier.Dispatch(appnotify.Event{
		Type:       corenotify.EventImageUpdateAvailable,
		OccurredAt: check.CheckedAt,
		Source:     source,
		ImageUpdate: &appnotify.ImageUpdateEvent{
			TaskID:          taskID,
			ServiceName:     check.ServiceName,
			NodeID:          check.NodeID,
			ImageName:       check.ImageName,
			ImageRef:        check.ImageRef,
			CandidateTag:    check.CandidateTag,
			CandidateDigest: check.CandidateDigest,
			CheckStatus:     check.CheckStatus,
		},
	})
}

func sendImageUpdateAppliedNotification(ctx context.Context, notifier *appnotify.Notifier, sourceRecord, updateRecord task.Record, imageNames []string) error {
	return notifier.Send(ctx, imageUpdateAppliedNotificationEvent(sourceRecord, updateRecord, imageNames))
}

func imageUpdateAppliedNotificationEvent(sourceRecord, updateRecord task.Record, imageNames []string) appnotify.Event {
	return appnotify.Event{
		Type:       corenotify.EventImageUpdateApplied,
		OccurredAt: updateRecord.CreatedAt,
		Source:     sourceRecord.Source,
		ImageUpdate: &appnotify.ImageUpdateEvent{
			TaskID:           sourceRecord.TaskID,
			UpdateTaskID:     updateRecord.TaskID,
			ServiceName:      sourceRecord.ServiceName,
			NodeID:           sourceRecord.NodeID,
			SelectedImageIDs: append([]string(nil), imageNames...),
		},
	}
}

func dispatchNodeNotification(notifier *appnotify.Notifier, eventType corenotify.EventType, snapshot store.NodeSnapshot) {
	if notifier == nil {
		return
	}
	notifier.Dispatch(appnotify.Event{
		Type:       eventType,
		OccurredAt: parseNodeHeartbeatTime(snapshot.LastHeartbeat),
		Node: &appnotify.NodeEvent{
			NodeID:        snapshot.NodeID,
			LastHeartbeat: snapshot.LastHeartbeat,
		},
	})
}

func latestTaskRecordForServiceNodeType(ctx context.Context, db *store.DB, serviceName, nodeID string, taskType task.Type) (task.Record, error) {
	tasks, _, err := db.ListTasks(ctx, nil, []string{serviceName}, []string{nodeID}, []string{string(taskType)}, nil, nil, nil, nil, 1, 1)
	if err != nil {
		return task.Record{}, err
	}
	if len(tasks) == 0 {
		return task.Record{}, fmt.Errorf("no %s task found for %s@%s", taskType, serviceName, nodeID)
	}
	detail, err := db.GetTask(ctx, tasks[0].TaskID)
	if err != nil {
		return task.Record{}, err
	}
	return detail.Record, nil
}

func detectNewImageUpdateChecks(previous []store.ServiceImageUpdateCheck, current []store.ServiceImageUpdateCheck) []store.ServiceImageUpdateCheck {
	previousByImage := make(map[string]store.ServiceImageUpdateCheck, len(previous))
	for _, check := range previous {
		previousByImage[check.ImageName] = check
	}
	newlyAvailable := make([]store.ServiceImageUpdateCheck, 0)
	for _, check := range current {
		if !check.UpdateAvailable || check.CheckStatus == store.ImageCheckStatusError {
			continue
		}
		previousCheck, ok := previousByImage[check.ImageName]
		if !ok || !previousCheck.UpdateAvailable || previousCheck.CandidateTag != check.CandidateTag || previousCheck.CandidateDigest != check.CandidateDigest {
			newlyAvailable = append(newlyAvailable, check)
		}
	}
	return newlyAvailable
}

func taskEventTypeForStatus(status task.Status) (corenotify.EventType, bool) {
	switch status {
	case task.StatusSucceeded:
		return corenotify.EventTaskCompleted, true
	case task.StatusFailed:
		return corenotify.EventTaskFailed, true
	case task.StatusCancelled:
		return corenotify.EventTaskCancelled, true
	default:
		return "", false
	}
}

func backupEventTypeForStatus(status string) (corenotify.EventType, bool) {
	switch task.Status(status) {
	case task.StatusSucceeded:
		return corenotify.EventBackupCompleted, true
	case task.StatusFailed:
		return corenotify.EventBackupFailed, true
	default:
		return "", false
	}
}

func parseNodeHeartbeatTime(value string) time.Time {
	if value == "" {
		return time.Now().UTC()
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().UTC()
	}
	return parsed.UTC()
}

func timeFromBackup(detail store.BackupDetail) time.Time {
	if detail.FinishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, detail.FinishedAt); err == nil {
			return parsed.UTC()
		}
	}
	if detail.StartedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, detail.StartedAt); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func derefTaskTime(values ...*time.Time) time.Time {
	for _, value := range values {
		if value != nil {
			return value.UTC()
		}
	}
	return time.Now().UTC()
}

func snapshotIfExists(ctx context.Context, db *store.DB, nodeID string) (store.NodeSnapshot, bool) {
	snapshot, err := db.GetNodeSnapshot(ctx, nodeID)
	if err == nil {
		return snapshot, true
	}
	return store.NodeSnapshot{}, false
}
