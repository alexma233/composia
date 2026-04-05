package task

import "time"

type Type string

const (
	TypeDeploy        Type = "deploy"
	TypeStop          Type = "stop"
	TypeRestart       Type = "restart"
	TypeUpdate        Type = "update"
	TypeBackup        Type = "backup"
	TypeMigrate       Type = "migrate"
	TypeDNSUpdate     Type = "dns_update"
	TypeCaddyReload   Type = "caddy_reload"
	TypePrune         Type = "prune"
	TypeDockerList    Type = "docker_list"
	TypeDockerInspect Type = "docker_inspect"
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

type StepName string

const (
	StepRender        StepName = "render"
	StepPull          StepName = "pull"
	StepBackup        StepName = "backup"
	StepComposeDown   StepName = "compose_down"
	StepComposeUp     StepName = "compose_up"
	StepTransfer      StepName = "transfer"
	StepRestore       StepName = "restore"
	StepDNSUpdate     StepName = "dns_update"
	StepCaddyReload   StepName = "caddy_reload"
	StepPrune         StepName = "prune"
	StepPersistRepo   StepName = "persist_repo"
	StepFinalize      StepName = "finalize"
	StepDockerList    StepName = "docker_list"
	StepDockerInspect StepName = "docker_inspect"
)

type StepRecord struct {
	TaskID     string
	StepName   StepName
	Status     Status
	StartedAt  *time.Time
	FinishedAt *time.Time
}

func IsControllerOwnedType(taskType Type) bool {
	switch taskType {
	case TypeDNSUpdate:
		return true
	default:
		return false
	}
}

func RequiresOnlineNode(taskType Type) bool {
	return !IsControllerOwnedType(taskType)
}
