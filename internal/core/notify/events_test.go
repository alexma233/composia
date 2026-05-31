package notify

import "testing"

func TestIsValidEventType(t *testing.T) {
	t.Parallel()

	valid := []EventType{
		EventTaskFailed,
		EventTaskCancelled,
		EventTaskCompleted,
		EventTaskAwaitingConfirmation,
		EventBackupCompleted,
		EventBackupFailed,
		EventImageUpdateAvailable,
		EventImageUpdateApplied,
		EventNodeOffline,
		EventNodeOnline,
		EventAlertmanagerAlert,
	}
	for _, eventType := range valid {
		eventType := eventType
		t.Run(string(eventType), func(t *testing.T) {
			t.Parallel()
			if !IsValidEventType(string(eventType)) {
				t.Fatalf("expected %q to be valid", eventType)
			}
		})
	}

	for _, value := range []string{"", "TaskFailed", "task_failed ", "unknown"} {
		value := value
		t.Run("invalid_"+value, func(t *testing.T) {
			t.Parallel()
			if IsValidEventType(value) {
				t.Fatalf("expected %q to be invalid", value)
			}
		})
	}
}
