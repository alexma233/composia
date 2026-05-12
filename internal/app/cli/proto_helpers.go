package cli

import (
	"strings"
	"time"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func taskStatusText(value controllerv1.TaskStatus) string {
	switch value {
	case controllerv1.TaskStatus_TASK_STATUS_PENDING:
		return "pending"
	case controllerv1.TaskStatus_TASK_STATUS_RUNNING:
		return "running"
	case controllerv1.TaskStatus_TASK_STATUS_AWAITING_CONFIRMATION:
		return "awaiting_confirmation"
	case controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED:
		return "succeeded"
	case controllerv1.TaskStatus_TASK_STATUS_FAILED:
		return "failed"
	case controllerv1.TaskStatus_TASK_STATUS_CANCELLED:
		return "cancelled"
	default:
		return ""
	}
}

func taskTypeText(value controllerv1.TaskType) string {
	return strings.ToLower(strings.TrimPrefix(value.String(), "TASK_TYPE_"))
}

func taskSourceText(value controllerv1.TaskSource) string {
	return strings.ToLower(strings.TrimPrefix(value.String(), "TASK_SOURCE_"))
}

func taskStepNameText(value controllerv1.TaskStepName) string {
	return strings.ToLower(strings.TrimPrefix(value.String(), "TASK_STEP_NAME_"))
}

func capabilityReasonText(value controllerv1.CapabilityReasonCode) string {
	if value == controllerv1.CapabilityReasonCode_CAPABILITY_REASON_CODE_UNSPECIFIED {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(value.String(), "CAPABILITY_REASON_CODE_"))
}

func taskStatusFromText(value string) controllerv1.TaskStatus {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "pending":
		return controllerv1.TaskStatus_TASK_STATUS_PENDING
	case "running":
		return controllerv1.TaskStatus_TASK_STATUS_RUNNING
	case "awaiting_confirmation":
		return controllerv1.TaskStatus_TASK_STATUS_AWAITING_CONFIRMATION
	case "succeeded":
		return controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED
	case "failed":
		return controllerv1.TaskStatus_TASK_STATUS_FAILED
	case "cancelled":
		return controllerv1.TaskStatus_TASK_STATUS_CANCELLED
	default:
		return controllerv1.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

func taskTypeFromText(value string) controllerv1.TaskType {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "deploy":
		return controllerv1.TaskType_TASK_TYPE_DEPLOY
	case "stop":
		return controllerv1.TaskType_TASK_TYPE_STOP
	case "restart":
		return controllerv1.TaskType_TASK_TYPE_RESTART
	case "update":
		return controllerv1.TaskType_TASK_TYPE_UPDATE
	case "backup":
		return controllerv1.TaskType_TASK_TYPE_BACKUP
	case "restore":
		return controllerv1.TaskType_TASK_TYPE_RESTORE
	case "migrate":
		return controllerv1.TaskType_TASK_TYPE_MIGRATE
	case "dns_update":
		return controllerv1.TaskType_TASK_TYPE_DNS_UPDATE
	case "caddy_sync":
		return controllerv1.TaskType_TASK_TYPE_CADDY_SYNC
	case "caddy_reload":
		return controllerv1.TaskType_TASK_TYPE_CADDY_RELOAD
	case "image_check":
		return controllerv1.TaskType_TASK_TYPE_IMAGE_CHECK
	case "prune":
		return controllerv1.TaskType_TASK_TYPE_PRUNE
	case "rustic_init":
		return controllerv1.TaskType_TASK_TYPE_RUSTIC_INIT
	case "rustic_forget":
		return controllerv1.TaskType_TASK_TYPE_RUSTIC_FORGET
	case "rustic_prune":
		return controllerv1.TaskType_TASK_TYPE_RUSTIC_PRUNE
	case "docker_start":
		return controllerv1.TaskType_TASK_TYPE_DOCKER_START
	case "docker_stop":
		return controllerv1.TaskType_TASK_TYPE_DOCKER_STOP
	case "docker_restart":
		return controllerv1.TaskType_TASK_TYPE_DOCKER_RESTART
	case "docker_remove":
		return controllerv1.TaskType_TASK_TYPE_DOCKER_REMOVE
	default:
		return controllerv1.TaskType_TASK_TYPE_UNSPECIFIED
	}
}

func taskStatusesFromTexts(values []string) []controllerv1.TaskStatus {
	statuses := make([]controllerv1.TaskStatus, 0, len(values))
	for _, value := range values {
		if status := taskStatusFromText(value); status != controllerv1.TaskStatus_TASK_STATUS_UNSPECIFIED {
			statuses = append(statuses, status)
		}
	}
	return statuses
}

func taskTypesFromTexts(values []string) []controllerv1.TaskType {
	types := make([]controllerv1.TaskType, 0, len(values))
	for _, value := range values {
		if taskType := taskTypeFromText(value); taskType != controllerv1.TaskType_TASK_TYPE_UNSPECIFIED {
			types = append(types, taskType)
		}
	}
	return types
}

func taskDecisionFromText(value string) controllerv1.TaskConfirmationDecision {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "approve":
		return controllerv1.TaskConfirmationDecision_TASK_CONFIRMATION_DECISION_APPROVE
	case "reject":
		return controllerv1.TaskConfirmationDecision_TASK_CONFIRMATION_DECISION_REJECT
	default:
		return controllerv1.TaskConfirmationDecision_TASK_CONFIRMATION_DECISION_UNSPECIFIED
	}
}

func formatProtoTimestamp(value *timestamppb.Timestamp) string {
	if value == nil {
		return ""
	}
	return value.AsTime().UTC().Format(time.RFC3339)
}
