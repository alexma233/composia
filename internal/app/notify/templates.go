package notify

import (
	"fmt"
	"strings"
	"time"

	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
)

func renderEvent(event Event) (string, string, error) {
	switch event.Type {
	case corenotify.EventTaskFailed, corenotify.EventTaskCancelled, corenotify.EventTaskCompleted, corenotify.EventTaskAwaitingConfirmation:
		return renderTaskEvent(event)
	case corenotify.EventBackupCompleted, corenotify.EventBackupFailed:
		return renderBackupEvent(event)
	case corenotify.EventImageUpdateAvailable, corenotify.EventImageUpdateApplied:
		return renderImageUpdateEvent(event)
	case corenotify.EventNodeOffline, corenotify.EventNodeOnline:
		return renderNodeEvent(event)
	default:
		return "", "", fmt.Errorf("unsupported notification event %q", event.Type)
	}
}

func renderTaskEvent(event Event) (string, string, error) {
	if event.Task == nil {
		return "", "", fmt.Errorf("task payload is missing for %q", event.Type)
	}
	subject := fmt.Sprintf("[composia] %s %s", event.Type, taskLabel(event.Task.ServiceName, event.Task.NodeID))
	lines := []string{
		fmt.Sprintf("Event: %s", event.Type),
		fmt.Sprintf("Task ID: %s", event.Task.TaskID),
		fmt.Sprintf("Task Type: %s", event.Task.TaskType),
		fmt.Sprintf("Task Status: %s", event.Task.Status),
		fmt.Sprintf("Task Source: %s", displayString(string(event.Source))),
		fmt.Sprintf("Service: %s", displayString(event.Task.ServiceName)),
		fmt.Sprintf("Node: %s", displayString(event.Task.NodeID)),
		fmt.Sprintf("Triggered By: %s", displayString(event.Task.TriggeredBy)),
		fmt.Sprintf("Started At: %s", formatOptionalTime(event.Task.StartedAt)),
		fmt.Sprintf("Finished At: %s", formatOptionalTime(event.Task.FinishedAt)),
		fmt.Sprintf("Observed At: %s", formatTime(event.OccurredAt)),
	}
	if strings.TrimSpace(event.Task.ErrorSummary) != "" {
		lines = append(lines, fmt.Sprintf("Error: %s", event.Task.ErrorSummary))
	}
	return subject, strings.Join(lines, "\n"), nil
}

func renderBackupEvent(event Event) (string, string, error) {
	if event.Backup == nil {
		return "", "", fmt.Errorf("backup payload is missing for %q", event.Type)
	}
	subject := fmt.Sprintf("[composia] %s %s %s", event.Type, taskLabel(event.Backup.ServiceName, event.Backup.NodeID), displayString(event.Backup.DataName))
	lines := []string{
		fmt.Sprintf("Event: %s", event.Type),
		fmt.Sprintf("Task ID: %s", event.Backup.TaskID),
		fmt.Sprintf("Backup ID: %s", event.Backup.BackupID),
		fmt.Sprintf("Task Source: %s", displayString(string(event.Source))),
		fmt.Sprintf("Service: %s", displayString(event.Backup.ServiceName)),
		fmt.Sprintf("Node: %s", displayString(event.Backup.NodeID)),
		fmt.Sprintf("Data Name: %s", displayString(event.Backup.DataName)),
		fmt.Sprintf("Status: %s", displayString(event.Backup.Status)),
		fmt.Sprintf("Artifact Ref: %s", displayString(event.Backup.ArtifactRef)),
		fmt.Sprintf("Observed At: %s", formatTime(event.OccurredAt)),
	}
	if strings.TrimSpace(event.Backup.ErrorSummary) != "" {
		lines = append(lines, fmt.Sprintf("Error: %s", event.Backup.ErrorSummary))
	}
	return subject, strings.Join(lines, "\n"), nil
}

func renderImageUpdateEvent(event Event) (string, string, error) {
	if event.ImageUpdate == nil {
		return "", "", fmt.Errorf("image update payload is missing for %q", event.Type)
	}
	imageName := displayString(event.ImageUpdate.ImageName)
	if len(event.ImageUpdate.SelectedImageIDs) > 0 {
		imageName = strings.Join(event.ImageUpdate.SelectedImageIDs, ", ")
	}
	subject := fmt.Sprintf("[composia] %s %s %s", event.Type, taskLabel(event.ImageUpdate.ServiceName, event.ImageUpdate.NodeID), imageName)
	lines := []string{
		fmt.Sprintf("Event: %s", event.Type),
		fmt.Sprintf("Task Source: %s", displayString(string(event.Source))),
		fmt.Sprintf("Service: %s", displayString(event.ImageUpdate.ServiceName)),
		fmt.Sprintf("Node: %s", displayString(event.ImageUpdate.NodeID)),
		fmt.Sprintf("Image Name: %s", displayString(event.ImageUpdate.ImageName)),
		fmt.Sprintf("Image Ref: %s", displayString(event.ImageUpdate.ImageRef)),
		fmt.Sprintf("Candidate Tag: %s", displayString(event.ImageUpdate.CandidateTag)),
		fmt.Sprintf("Candidate Digest: %s", displayString(event.ImageUpdate.CandidateDigest)),
		fmt.Sprintf("Check Status: %s", displayString(event.ImageUpdate.CheckStatus)),
		fmt.Sprintf("Image Check Task ID: %s", displayString(event.ImageUpdate.TaskID)),
		fmt.Sprintf("Update Task ID: %s", displayString(event.ImageUpdate.UpdateTaskID)),
		fmt.Sprintf("Observed At: %s", formatTime(event.OccurredAt)),
	}
	if len(event.ImageUpdate.SelectedImageIDs) > 0 {
		lines = append(lines, fmt.Sprintf("Selected Images: %s", strings.Join(event.ImageUpdate.SelectedImageIDs, ", ")))
	}
	return subject, strings.Join(lines, "\n"), nil
}

func renderNodeEvent(event Event) (string, string, error) {
	if event.Node == nil {
		return "", "", fmt.Errorf("node payload is missing for %q", event.Type)
	}
	subject := fmt.Sprintf("[composia] %s %s", event.Type, displayString(event.Node.NodeID))
	lines := []string{
		fmt.Sprintf("Event: %s", event.Type),
		fmt.Sprintf("Node: %s", displayString(event.Node.NodeID)),
		fmt.Sprintf("Last Heartbeat: %s", displayString(event.Node.LastHeartbeat)),
		fmt.Sprintf("Observed At: %s", formatTime(event.OccurredAt)),
	}
	return subject, strings.Join(lines, "\n"), nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "n/a"
	}
	return value.UTC().Format(time.RFC3339)
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return "n/a"
	}
	return value.UTC().Format(time.RFC3339)
}

func displayString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "n/a"
	}
	return value
}
