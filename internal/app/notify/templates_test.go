package notify

import (
	"strings"
	"testing"
	"time"

	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

const testNotAvailable = "n/a"

func TestRenderTaskEvent(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 5, 31, 4, 4, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC)
	subject, body, err := renderEvent(Event{
		Type:       corenotify.EventTaskFailed,
		OccurredAt: finishedAt,
		Source:     task.SourceCLI,
		Task: &TaskEvent{
			TaskID:       "task-1",
			TaskType:     task.TypeDeploy,
			Status:       task.StatusFailed,
			ServiceName:  "web",
			NodeID:       "main",
			TriggeredBy:  "alex",
			ErrorSummary: "compose failed",
			StartedAt:    &startedAt,
			FinishedAt:   &finishedAt,
		},
	})
	if err != nil {
		t.Fatalf("renderEvent returned error: %v", err)
	}
	if subject != "[composia] task_failed web@main" {
		t.Fatalf("subject = %q", subject)
	}
	for _, want := range []string{"Task ID: task-1", "Task Type: deploy", "Task Source: cli", "Error: compose failed", "Observed At: 2026-05-31T04:05:00Z"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestRenderBackupEventRequiresPayload(t *testing.T) {
	t.Parallel()

	_, _, err := renderEvent(Event{Type: corenotify.EventBackupFailed})
	if err == nil || !strings.Contains(err.Error(), "backup payload is missing") {
		t.Fatalf("expected missing backup payload error, got %v", err)
	}
}

func TestRenderImageUpdateEventUsesSelectedImageIDs(t *testing.T) {
	t.Parallel()

	subject, body, err := renderEvent(Event{
		Type:       corenotify.EventImageUpdateApplied,
		OccurredAt: time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC),
		Source:     task.SourceSchedule,
		ImageUpdate: &ImageUpdateEvent{
			ServiceName:      "web",
			NodeID:           "main",
			ImageName:        "api",
			SelectedImageIDs: []string{"api", "worker"},
			UpdateTaskID:     "task-update",
		},
	})
	if err != nil {
		t.Fatalf("renderEvent returned error: %v", err)
	}
	if subject != "[composia] image_update_applied web@main api, worker" {
		t.Fatalf("subject = %q", subject)
	}
	if !strings.Contains(body, "Selected Images: api, worker") || !strings.Contains(body, "Update Task ID: task-update") {
		t.Fatalf("unexpected body:\n%s", body)
	}
}

func TestRenderAlertmanagerEventSortsMapBlocks(t *testing.T) {
	t.Parallel()

	_, body, err := renderEvent(Event{
		Type:       corenotify.EventAlertmanagerAlert,
		OccurredAt: time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC),
		Alertmanager: &AlertmanagerEvent{
			AlertName: "HighCPU",
			Labels: map[string]string{
				"severity":  "warning",
				"alertname": "HighCPU",
			},
			Annotations: map[string]string{
				"summary":     "CPU high",
				"description": "CPU is high",
			},
		},
	})
	if err != nil {
		t.Fatalf("renderEvent returned error: %v", err)
	}
	if !strings.Contains(body, "Labels:\nalertname=HighCPU\nseverity=warning") {
		t.Fatalf("labels are not sorted in body:\n%s", body)
	}
	if !strings.Contains(body, "Annotations:\ndescription=CPU is high\nsummary=CPU high") {
		t.Fatalf("annotations are not sorted in body:\n%s", body)
	}
}

func TestRenderNodeEvent(t *testing.T) {
	t.Parallel()

	subject, body, err := renderEvent(Event{Type: corenotify.EventNodeOffline, OccurredAt: time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC), Node: &NodeEvent{NodeID: "main", LastHeartbeat: "2026-05-31T04:00:00Z"}})
	if err != nil {
		t.Fatalf("renderEvent returned error: %v", err)
	}
	if subject != "[composia] node_offline main" {
		t.Fatalf("subject = %q", subject)
	}
	if !strings.Contains(body, "Node: main") || !strings.Contains(body, "Last Heartbeat: 2026-05-31T04:00:00Z") {
		t.Fatalf("unexpected body:\n%s", body)
	}
}

func TestRenderUnsupportedEvent(t *testing.T) {
	t.Parallel()

	_, _, err := renderEvent(Event{Type: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unsupported notification event") {
		t.Fatalf("expected unsupported event error, got %v", err)
	}
}

func TestFormatHelpers(t *testing.T) {
	t.Parallel()

	if got := displayString("  "); got != testNotAvailable {
		t.Fatalf("displayString empty = %q", got)
	}
	if got := displayString(" value "); got != "value" {
		t.Fatalf("displayString value = %q", got)
	}
	if got := formatTime(time.Time{}); got != testNotAvailable {
		t.Fatalf("formatTime zero = %q", got)
	}
	if got := formatOptionalTime(nil); got != testNotAvailable {
		t.Fatalf("formatOptionalTime nil = %q", got)
	}
}
