package notify

import (
	"fmt"
	"slices"
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
	case corenotify.EventAlertmanagerAlert:
		return renderAlertmanagerEvent(event)
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
		"Task ID: " + event.Task.TaskID,
		fmt.Sprintf("Task Type: %s", event.Task.TaskType),
		fmt.Sprintf("Task Status: %s", event.Task.Status),
		"Task Source: " + displayString(string(event.Source)),
		"Service: " + displayString(event.Task.ServiceName),
		"Node: " + displayString(event.Task.NodeID),
		"Triggered By: " + displayString(event.Task.TriggeredBy),
		"Started At: " + formatOptionalTime(event.Task.StartedAt),
		"Finished At: " + formatOptionalTime(event.Task.FinishedAt),
		"Observed At: " + formatTime(event.OccurredAt),
	}
	if strings.TrimSpace(event.Task.ErrorSummary) != "" {
		lines = append(lines, "Error: "+event.Task.ErrorSummary)
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
		"Task ID: " + event.Backup.TaskID,
		"Backup ID: " + event.Backup.BackupID,
		"Task Source: " + displayString(string(event.Source)),
		"Service: " + displayString(event.Backup.ServiceName),
		"Node: " + displayString(event.Backup.NodeID),
		"Data Name: " + displayString(event.Backup.DataName),
		"Status: " + displayString(event.Backup.Status),
		"Artifact Ref: " + displayString(event.Backup.ArtifactRef),
		"Observed At: " + formatTime(event.OccurredAt),
	}
	if strings.TrimSpace(event.Backup.ErrorSummary) != "" {
		lines = append(lines, "Error: "+event.Backup.ErrorSummary)
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
		"Task Source: " + displayString(string(event.Source)),
		"Service: " + displayString(event.ImageUpdate.ServiceName),
		"Node: " + displayString(event.ImageUpdate.NodeID),
		"Image Name: " + displayString(event.ImageUpdate.ImageName),
		"Image Ref: " + displayString(event.ImageUpdate.ImageRef),
		"Candidate Tag: " + displayString(event.ImageUpdate.CandidateTag),
		"Candidate Digest: " + displayString(event.ImageUpdate.CandidateDigest),
		"Check Status: " + displayString(event.ImageUpdate.CheckStatus),
		"Image Check Task ID: " + displayString(event.ImageUpdate.TaskID),
		"Update Task ID: " + displayString(event.ImageUpdate.UpdateTaskID),
		"Observed At: " + formatTime(event.OccurredAt),
	}
	if len(event.ImageUpdate.SelectedImageIDs) > 0 {
		lines = append(lines, "Selected Images: "+strings.Join(event.ImageUpdate.SelectedImageIDs, ", "))
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
		"Node: " + displayString(event.Node.NodeID),
		"Last Heartbeat: " + displayString(event.Node.LastHeartbeat),
		"Observed At: " + formatTime(event.OccurredAt),
	}
	return subject, strings.Join(lines, "\n"), nil
}

func renderAlertmanagerEvent(event Event) (string, string, error) {
	if event.Alertmanager == nil {
		return "", "", fmt.Errorf("alertmanager payload is missing for %q", event.Type)
	}
	subject := fmt.Sprintf("[composia] %s %s", event.Type, displayString(event.Alertmanager.AlertName))
	lines := make([]string, 0, 15+len(event.Alertmanager.Labels)+len(event.Alertmanager.Annotations))
	lines = append(lines,
		fmt.Sprintf("Event: %s", event.Type),
		"Receiver: "+displayString(event.Alertmanager.Receiver),
		"Alert Status: "+displayString(event.Alertmanager.Status),
		"Group Status: "+displayString(event.Alertmanager.GroupStatus),
		"Alert Name: "+displayString(event.Alertmanager.AlertName),
		"Severity: "+displayString(event.Alertmanager.Severity),
		"Instance: "+displayString(event.Alertmanager.Instance),
		"Summary: "+displayString(event.Alertmanager.Summary),
		"Description: "+displayString(event.Alertmanager.Description),
		"Starts At: "+formatOptionalTime(event.Alertmanager.StartsAt),
		"Ends At: "+formatOptionalTime(event.Alertmanager.EndsAt),
		"Generator URL: "+displayString(event.Alertmanager.GeneratorURL),
		"External URL: "+displayString(event.Alertmanager.ExternalURL),
		"Fingerprint: "+displayString(event.Alertmanager.Fingerprint),
		"Observed At: "+formatTime(event.OccurredAt),
	)
	lines = append(lines, formatStringMapBlock("Labels", event.Alertmanager.Labels)...)
	lines = append(lines, formatStringMapBlock("Annotations", event.Alertmanager.Annotations)...)
	return subject, strings.Join(lines, "\n"), nil
}

func formatStringMapBlock(title string, values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	lines := []string{title + ":"}
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, displayString(values[key])))
	}
	return lines
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
