package controller

import (
	"time"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func protoTaskType(value task.Type) controllerv1.TaskType {
	switch value {
	case task.TypeDeploy:
		return controllerv1.TaskType_TASK_TYPE_DEPLOY
	case task.TypeStop:
		return controllerv1.TaskType_TASK_TYPE_STOP
	case task.TypeRestart:
		return controllerv1.TaskType_TASK_TYPE_RESTART
	case task.TypeUpdate:
		return controllerv1.TaskType_TASK_TYPE_UPDATE
	case task.TypeBackup:
		return controllerv1.TaskType_TASK_TYPE_BACKUP
	case task.TypeRestore:
		return controllerv1.TaskType_TASK_TYPE_RESTORE
	case task.TypeMigrate:
		return controllerv1.TaskType_TASK_TYPE_MIGRATE
	case task.TypeDNSUpdate:
		return controllerv1.TaskType_TASK_TYPE_DNS_UPDATE
	case task.TypeCaddySync:
		return controllerv1.TaskType_TASK_TYPE_CADDY_SYNC
	case task.TypeCaddyReload:
		return controllerv1.TaskType_TASK_TYPE_CADDY_RELOAD
	case task.TypeImageCheck:
		return controllerv1.TaskType_TASK_TYPE_IMAGE_CHECK
	case task.TypePrune:
		return controllerv1.TaskType_TASK_TYPE_PRUNE
	case task.TypeRusticInit:
		return controllerv1.TaskType_TASK_TYPE_RUSTIC_INIT
	case task.TypeRusticForget:
		return controllerv1.TaskType_TASK_TYPE_RUSTIC_FORGET
	case task.TypeRusticPrune:
		return controllerv1.TaskType_TASK_TYPE_RUSTIC_PRUNE
	case task.TypeDockerStart:
		return controllerv1.TaskType_TASK_TYPE_DOCKER_START
	case task.TypeDockerStop:
		return controllerv1.TaskType_TASK_TYPE_DOCKER_STOP
	case task.TypeDockerRestart:
		return controllerv1.TaskType_TASK_TYPE_DOCKER_RESTART
	case task.TypeDockerRemove:
		return controllerv1.TaskType_TASK_TYPE_DOCKER_REMOVE
	case task.TypeMigrateRollback:
		return controllerv1.TaskType_TASK_TYPE_MIGRATE_ROLLBACK
	default:
		return controllerv1.TaskType_TASK_TYPE_UNSPECIFIED
	}
}

func protoTaskStatus(value task.Status) controllerv1.TaskStatus {
	switch value {
	case task.StatusPending:
		return controllerv1.TaskStatus_TASK_STATUS_PENDING
	case task.StatusRunning:
		return controllerv1.TaskStatus_TASK_STATUS_RUNNING
	case task.StatusAwaitingConfirmation:
		return controllerv1.TaskStatus_TASK_STATUS_AWAITING_CONFIRMATION
	case task.StatusSucceeded:
		return controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED
	case task.StatusFailed:
		return controllerv1.TaskStatus_TASK_STATUS_FAILED
	case task.StatusCancelled:
		return controllerv1.TaskStatus_TASK_STATUS_CANCELLED
	default:
		return controllerv1.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

func protoTaskSource(value task.Source) controllerv1.TaskSource {
	switch value {
	case task.SourceWeb:
		return controllerv1.TaskSource_TASK_SOURCE_WEB
	case task.SourceCLI:
		return controllerv1.TaskSource_TASK_SOURCE_CLI
	case task.SourceOthers:
		return controllerv1.TaskSource_TASK_SOURCE_OTHERS
	case task.SourceSchedule:
		return controllerv1.TaskSource_TASK_SOURCE_SCHEDULE
	case task.SourceSystem:
		return controllerv1.TaskSource_TASK_SOURCE_SYSTEM
	case task.SourceAutoDeploy:
		return controllerv1.TaskSource_TASK_SOURCE_AUTO_DEPLOY
	default:
		return controllerv1.TaskSource_TASK_SOURCE_UNSPECIFIED
	}
}

func protoTaskStepName(value task.StepName) controllerv1.TaskStepName {
	switch value {
	case task.StepRender:
		return controllerv1.TaskStepName_TASK_STEP_NAME_RENDER
	case task.StepPull:
		return controllerv1.TaskStepName_TASK_STEP_NAME_PULL
	case task.StepBackup:
		return controllerv1.TaskStepName_TASK_STEP_NAME_BACKUP
	case task.StepComposeDown:
		return controllerv1.TaskStepName_TASK_STEP_NAME_COMPOSE_DOWN
	case task.StepComposeUp:
		return controllerv1.TaskStepName_TASK_STEP_NAME_COMPOSE_UP
	case task.StepTransfer:
		return controllerv1.TaskStepName_TASK_STEP_NAME_TRANSFER
	case task.StepRestore:
		return controllerv1.TaskStepName_TASK_STEP_NAME_RESTORE
	case task.StepDNSUpdate:
		return controllerv1.TaskStepName_TASK_STEP_NAME_DNS_UPDATE
	case task.StepCaddySync:
		return controllerv1.TaskStepName_TASK_STEP_NAME_CADDY_SYNC
	case task.StepCaddyReload:
		return controllerv1.TaskStepName_TASK_STEP_NAME_CADDY_RELOAD
	case task.StepImageCheck:
		return controllerv1.TaskStepName_TASK_STEP_NAME_IMAGE_CHECK
	case task.StepInit:
		return controllerv1.TaskStepName_TASK_STEP_NAME_INIT
	case task.StepPrune:
		return controllerv1.TaskStepName_TASK_STEP_NAME_PRUNE
	case task.StepAwaitingConfirmation:
		return controllerv1.TaskStepName_TASK_STEP_NAME_AWAITING_CONFIRMATION
	case task.StepPersistRepo:
		return controllerv1.TaskStepName_TASK_STEP_NAME_PERSIST_REPO
	case task.StepFinalize:
		return controllerv1.TaskStepName_TASK_STEP_NAME_FINALIZE
	case task.StepDockerStart:
		return controllerv1.TaskStepName_TASK_STEP_NAME_DOCKER_START
	case task.StepDockerStop:
		return controllerv1.TaskStepName_TASK_STEP_NAME_DOCKER_STOP
	case task.StepDockerRestart:
		return controllerv1.TaskStepName_TASK_STEP_NAME_DOCKER_RESTART
	case task.StepDockerRemove:
		return controllerv1.TaskStepName_TASK_STEP_NAME_DOCKER_REMOVE
	default:
		return controllerv1.TaskStepName_TASK_STEP_NAME_UNSPECIFIED
	}
}

func taskStatusFromProto(value controllerv1.TaskStatus) (task.Status, bool) {
	switch value {
	case controllerv1.TaskStatus_TASK_STATUS_PENDING:
		return task.StatusPending, true
	case controllerv1.TaskStatus_TASK_STATUS_RUNNING:
		return task.StatusRunning, true
	case controllerv1.TaskStatus_TASK_STATUS_AWAITING_CONFIRMATION:
		return task.StatusAwaitingConfirmation, true
	case controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED:
		return task.StatusSucceeded, true
	case controllerv1.TaskStatus_TASK_STATUS_FAILED:
		return task.StatusFailed, true
	case controllerv1.TaskStatus_TASK_STATUS_CANCELLED:
		return task.StatusCancelled, true
	default:
		return "", false
	}
}

func taskTypeFromProto(value controllerv1.TaskType) (task.Type, bool) {
	switch value {
	case controllerv1.TaskType_TASK_TYPE_DEPLOY:
		return task.TypeDeploy, true
	case controllerv1.TaskType_TASK_TYPE_STOP:
		return task.TypeStop, true
	case controllerv1.TaskType_TASK_TYPE_RESTART:
		return task.TypeRestart, true
	case controllerv1.TaskType_TASK_TYPE_UPDATE:
		return task.TypeUpdate, true
	case controllerv1.TaskType_TASK_TYPE_BACKUP:
		return task.TypeBackup, true
	case controllerv1.TaskType_TASK_TYPE_RESTORE:
		return task.TypeRestore, true
	case controllerv1.TaskType_TASK_TYPE_MIGRATE:
		return task.TypeMigrate, true
	case controllerv1.TaskType_TASK_TYPE_DNS_UPDATE:
		return task.TypeDNSUpdate, true
	case controllerv1.TaskType_TASK_TYPE_CADDY_SYNC:
		return task.TypeCaddySync, true
	case controllerv1.TaskType_TASK_TYPE_CADDY_RELOAD:
		return task.TypeCaddyReload, true
	case controllerv1.TaskType_TASK_TYPE_IMAGE_CHECK:
		return task.TypeImageCheck, true
	case controllerv1.TaskType_TASK_TYPE_PRUNE:
		return task.TypePrune, true
	case controllerv1.TaskType_TASK_TYPE_RUSTIC_INIT:
		return task.TypeRusticInit, true
	case controllerv1.TaskType_TASK_TYPE_RUSTIC_FORGET:
		return task.TypeRusticForget, true
	case controllerv1.TaskType_TASK_TYPE_RUSTIC_PRUNE:
		return task.TypeRusticPrune, true
	case controllerv1.TaskType_TASK_TYPE_DOCKER_START:
		return task.TypeDockerStart, true
	case controllerv1.TaskType_TASK_TYPE_DOCKER_STOP:
		return task.TypeDockerStop, true
	case controllerv1.TaskType_TASK_TYPE_DOCKER_RESTART:
		return task.TypeDockerRestart, true
	case controllerv1.TaskType_TASK_TYPE_DOCKER_REMOVE:
		return task.TypeDockerRemove, true
	case controllerv1.TaskType_TASK_TYPE_MIGRATE_ROLLBACK:
		return task.TypeMigrateRollback, true
	default:
		return "", false
	}
}

func taskStatusesFromProto(values []controllerv1.TaskStatus) []string {
	filters := make([]string, 0, len(values))
	for _, value := range values {
		if mapped, ok := taskStatusFromProto(value); ok {
			filters = append(filters, string(mapped))
		}
	}
	return filters
}

func taskTypesFromProto(values []controllerv1.TaskType) []string {
	filters := make([]string, 0, len(values))
	for _, value := range values {
		if mapped, ok := taskTypeFromProto(value); ok {
			filters = append(filters, string(mapped))
		}
	}
	return filters
}

func taskConfirmationDecisionFromProto(value controllerv1.TaskConfirmationDecision) string {
	switch value {
	case controllerv1.TaskConfirmationDecision_TASK_CONFIRMATION_DECISION_APPROVE:
		return confirmationDecisionApprove
	case controllerv1.TaskConfirmationDecision_TASK_CONFIRMATION_DECISION_REJECT:
		return confirmationDecisionReject
	default:
		return ""
	}
}

func protoTaskTime(value time.Time) *timestamppb.Timestamp {
	return timestamppb.New(value.UTC())
}

func protoNullableTaskTime(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return protoTaskTime(*value)
}

func protoCapabilityReason(value string) controllerv1.CapabilityReasonCode {
	switch value {
	case reasonMissingBackupIntegration:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_BACKUP_INTEGRATION
	case reasonMissingBackupDefinition:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_BACKUP_DEFINITION
	case reasonMissingRestoreDefinition:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_RESTORE_DEFINITION
	case reasonMissingMigrateDefinition:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_MIGRATE_DEFINITION
	case reasonMissingDNSIntegration:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_DNS_INTEGRATION
	case reasonMissingSecretsConfig:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_SECRETS_CONFIG
	case reasonMissingCaddyInfra:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_CADDY_INFRA
	case reasonMissingServiceMeta:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_SERVICE_META
	case reasonServiceNotDeclared:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_SERVICE_NOT_DECLARED
	case reasonServiceDNSNotDeclared:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_SERVICE_DNS_NOT_DECLARED
	case reasonServiceNotCaddyManaged:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_SERVICE_NOT_CADDY_MANAGED
	case reasonNodeDisabled:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_NODE_DISABLED
	case reasonNodeOffline:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_NODE_OFFLINE
	case reasonNodeNotEligible:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_NODE_NOT_ELIGIBLE
	case reasonNodeNotRusticManaged:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_NODE_NOT_RUSTIC_MANAGED
	case reasonMissingEligibleRusticNode:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_ELIGIBLE_RUSTIC_NODE
	case reasonMissingOnlineRusticNode:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_ONLINE_RUSTIC_NODE
	case reasonBackupNotSucceeded:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_BACKUP_NOT_SUCCEEDED
	case reasonBackupArtifactMissing:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_BACKUP_ARTIFACT_MISSING
	case reasonMissingRestoreTargetNode:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_MISSING_RESTORE_TARGET_NODE
	default:
		return controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_UNSPECIFIED
	}
}

func taskStatusFromAgentProto(value agentv1.AgentTaskStatus) (task.Status, bool) {
	switch value {
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_PENDING:
		return task.StatusPending, true
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_RUNNING:
		return task.StatusRunning, true
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_AWAITING_CONFIRMATION:
		return task.StatusAwaitingConfirmation, true
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_SUCCEEDED:
		return task.StatusSucceeded, true
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_FAILED:
		return task.StatusFailed, true
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_CANCELLED:
		return task.StatusCancelled, true
	default:
		return "", false
	}
}

func taskStepNameFromAgentProto(value agentv1.AgentTaskStepName) (task.StepName, bool) {
	switch value {
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_RENDER:
		return task.StepRender, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PULL:
		return task.StepPull, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_BACKUP:
		return task.StepBackup, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_COMPOSE_DOWN:
		return task.StepComposeDown, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_COMPOSE_UP:
		return task.StepComposeUp, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_TRANSFER:
		return task.StepTransfer, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_RESTORE:
		return task.StepRestore, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DNS_UPDATE:
		return task.StepDNSUpdate, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_CADDY_SYNC:
		return task.StepCaddySync, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_CADDY_RELOAD:
		return task.StepCaddyReload, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_IMAGE_CHECK:
		return task.StepImageCheck, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_INIT:
		return task.StepInit, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PRUNE:
		return task.StepPrune, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_AWAITING_CONFIRMATION:
		return task.StepAwaitingConfirmation, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PERSIST_REPO:
		return task.StepPersistRepo, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_FINALIZE:
		return task.StepFinalize, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_START:
		return task.StepDockerStart, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_STOP:
		return task.StepDockerStop, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_RESTART:
		return task.StepDockerRestart, true
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_REMOVE:
		return task.StepDockerRemove, true
	default:
		return "", false
	}
}

func protoAgentTaskType(value task.Type) agentv1.AgentTaskType {
	switch value {
	case task.TypeDeploy:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_DEPLOY
	case task.TypeStop:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_STOP
	case task.TypeRestart:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_RESTART
	case task.TypeUpdate:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_UPDATE
	case task.TypeBackup:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_BACKUP
	case task.TypeRestore:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_RESTORE
	case task.TypeMigrate:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_MIGRATE
	case task.TypeDNSUpdate:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_DNS_UPDATE
	case task.TypeCaddySync:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_CADDY_SYNC
	case task.TypeCaddyReload:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_CADDY_RELOAD
	case task.TypeImageCheck:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_IMAGE_CHECK
	case task.TypePrune:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_PRUNE
	case task.TypeRusticInit:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_INIT
	case task.TypeRusticForget:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_FORGET
	case task.TypeRusticPrune:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_PRUNE
	case task.TypeDockerStart:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_START
	case task.TypeDockerStop:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_STOP
	case task.TypeDockerRestart:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_RESTART
	case task.TypeDockerRemove:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE
	default:
		return agentv1.AgentTaskType_AGENT_TASK_TYPE_UNSPECIFIED
	}
}
