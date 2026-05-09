package notify

type EventType string

const (
	EventTaskFailed               EventType = "task_failed"
	EventTaskCancelled            EventType = "task_cancelled"
	EventTaskCompleted            EventType = "task_completed"
	EventTaskAwaitingConfirmation EventType = "task_awaiting_confirmation"
	EventBackupCompleted          EventType = "backup_completed"
	EventBackupFailed             EventType = "backup_failed"
	EventImageUpdateAvailable     EventType = "image_update_available"
	EventImageUpdateApplied       EventType = "image_update_applied"
	EventNodeOffline              EventType = "node_offline"
	EventNodeOnline               EventType = "node_online"
	EventAlertmanagerAlert        EventType = "alertmanager_alert"
)

func IsValidEventType(value string) bool {
	switch EventType(value) {
	case EventTaskFailed,
		EventTaskCancelled,
		EventTaskCompleted,
		EventTaskAwaitingConfirmation,
		EventBackupCompleted,
		EventBackupFailed,
		EventImageUpdateAvailable,
		EventImageUpdateApplied,
		EventNodeOffline,
		EventNodeOnline,
		EventAlertmanagerAlert:
		return true
	default:
		return false
	}
}
