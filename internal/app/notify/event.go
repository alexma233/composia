package notify

import (
	"time"

	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

type Event struct {
	Type        corenotify.EventType
	OccurredAt  time.Time
	Source      task.Source
	Task        *TaskEvent
	Backup      *BackupEvent
	ImageUpdate *ImageUpdateEvent
	Node        *NodeEvent
}

type TaskEvent struct {
	TaskID       string
	TaskType     task.Type
	Status       task.Status
	ServiceName  string
	NodeID       string
	TriggeredBy  string
	ErrorSummary string
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type BackupEvent struct {
	TaskID       string
	BackupID     string
	ServiceName  string
	NodeID       string
	DataName     string
	Status       string
	ArtifactRef  string
	ErrorSummary string
}

type ImageUpdateEvent struct {
	TaskID           string
	UpdateTaskID     string
	ServiceName      string
	NodeID           string
	ImageName        string
	ImageRef         string
	CandidateTag     string
	CandidateDigest  string
	CheckStatus      string
	SelectedImageIDs []string
}

type NodeEvent struct {
	NodeID        string
	LastHeartbeat string
}
