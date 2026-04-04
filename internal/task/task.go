package task

import "time"

type Type string

const (
	TypeDeploy      Type = "deploy"
	TypeStop        Type = "stop"
	TypeRestart     Type = "restart"
	TypeUpdate      Type = "update"
	TypeBackup      Type = "backup"
	TypeMigrate     Type = "migrate"
	TypeDNSUpdate   Type = "dns_update"
	TypeCaddyReload Type = "caddy_reload"
	TypePrune       Type = "prune"
)

type Status string

const (
	StatusPending              Status = "pending"
	StatusRunning              Status = "running"
	StatusAwaitingConfirmation Status = "awaiting_confirmation"
	StatusSucceeded            Status = "succeeded"
	StatusFailed               Status = "failed"
	StatusCancelled            Status = "cancelled"
)

type Source string

const (
	SourceWeb      Source = "web"
	SourceCLI      Source = "cli"
	SourceSchedule Source = "schedule"
	SourceSystem   Source = "system"
)

type Record struct {
	TaskID          string
	Type            Type
	Source          Source
	TriggeredBy     string
	ServiceName     string
	NodeID          string
	Status          Status
	ParamsJSON      string
	LogPath         string
	RepoRevision    string
	ResultRevision  string
	AttemptOfTaskID string
	CreatedAt       time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	ErrorSummary    string
}
