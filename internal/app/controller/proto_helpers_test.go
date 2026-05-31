package controller

import (
	"testing"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestTaskTypeFromProto(t *testing.T) {
	t.Parallel()

	cases := []struct {
		proto controllerv1.TaskType
		want  task.Type
	}{
		{controllerv1.TaskType_TASK_TYPE_DEPLOY, task.TypeDeploy},
		{controllerv1.TaskType_TASK_TYPE_STOP, task.TypeStop},
		{controllerv1.TaskType_TASK_TYPE_RESTART, task.TypeRestart},
		{controllerv1.TaskType_TASK_TYPE_UPDATE, task.TypeUpdate},
		{controllerv1.TaskType_TASK_TYPE_BACKUP, task.TypeBackup},
		{controllerv1.TaskType_TASK_TYPE_RESTORE, task.TypeRestore},
		{controllerv1.TaskType_TASK_TYPE_MIGRATE, task.TypeMigrate},
		{controllerv1.TaskType_TASK_TYPE_DNS_UPDATE, task.TypeDNSUpdate},
		{controllerv1.TaskType_TASK_TYPE_CADDY_SYNC, task.TypeCaddySync},
		{controllerv1.TaskType_TASK_TYPE_CADDY_RELOAD, task.TypeCaddyReload},
		{controllerv1.TaskType_TASK_TYPE_IMAGE_CHECK, task.TypeImageCheck},
		{controllerv1.TaskType_TASK_TYPE_PRUNE, task.TypePrune},
		{controllerv1.TaskType_TASK_TYPE_RUSTIC_INIT, task.TypeRusticInit},
		{controllerv1.TaskType_TASK_TYPE_RUSTIC_FORGET, task.TypeRusticForget},
		{controllerv1.TaskType_TASK_TYPE_RUSTIC_PRUNE, task.TypeRusticPrune},
		{controllerv1.TaskType_TASK_TYPE_DOCKER_START, task.TypeDockerStart},
		{controllerv1.TaskType_TASK_TYPE_DOCKER_STOP, task.TypeDockerStop},
		{controllerv1.TaskType_TASK_TYPE_DOCKER_RESTART, task.TypeDockerRestart},
		{controllerv1.TaskType_TASK_TYPE_DOCKER_REMOVE_CONTAINER, task.TypeDockerRemoveContainer},
		{controllerv1.TaskType_TASK_TYPE_DOCKER_REMOVE_NETWORK, task.TypeDockerRemoveNetwork},
		{controllerv1.TaskType_TASK_TYPE_DOCKER_REMOVE_VOLUME, task.TypeDockerRemoveVolume},
		{controllerv1.TaskType_TASK_TYPE_DOCKER_REMOVE_IMAGE, task.TypeDockerRemoveImage},
		{controllerv1.TaskType_TASK_TYPE_MIGRATE_ROLLBACK, task.TypeMigrateRollback},
	}
	for _, tc := range cases {
		got, ok := taskTypeFromProto(tc.proto)
		if !ok || got != tc.want {
			t.Fatalf("taskTypeFromProto(%v) = %q, %v", tc.proto, got, ok)
		}
	}
	if got, ok := taskTypeFromProto(controllerv1.TaskType_TASK_TYPE_UNSPECIFIED); ok || got != "" {
		t.Fatalf("unspecified task type = %q, %v", got, ok)
	}
}

func TestProtoAgentTaskType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value task.Type
		want  agentv1.AgentTaskType
	}{
		{task.TypeDeploy, agentv1.AgentTaskType_AGENT_TASK_TYPE_DEPLOY},
		{task.TypeStop, agentv1.AgentTaskType_AGENT_TASK_TYPE_STOP},
		{task.TypeRestart, agentv1.AgentTaskType_AGENT_TASK_TYPE_RESTART},
		{task.TypeUpdate, agentv1.AgentTaskType_AGENT_TASK_TYPE_UPDATE},
		{task.TypeBackup, agentv1.AgentTaskType_AGENT_TASK_TYPE_BACKUP},
		{task.TypeRestore, agentv1.AgentTaskType_AGENT_TASK_TYPE_RESTORE},
		{task.TypeMigrate, agentv1.AgentTaskType_AGENT_TASK_TYPE_MIGRATE},
		{task.TypeDNSUpdate, agentv1.AgentTaskType_AGENT_TASK_TYPE_DNS_UPDATE},
		{task.TypeCaddySync, agentv1.AgentTaskType_AGENT_TASK_TYPE_CADDY_SYNC},
		{task.TypeCaddyReload, agentv1.AgentTaskType_AGENT_TASK_TYPE_CADDY_RELOAD},
		{task.TypeImageCheck, agentv1.AgentTaskType_AGENT_TASK_TYPE_IMAGE_CHECK},
		{task.TypePrune, agentv1.AgentTaskType_AGENT_TASK_TYPE_PRUNE},
		{task.TypeRusticInit, agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_INIT},
		{task.TypeRusticForget, agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_FORGET},
		{task.TypeRusticPrune, agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_PRUNE},
		{task.TypeDockerStart, agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_START},
		{task.TypeDockerStop, agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_STOP},
		{task.TypeDockerRestart, agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_RESTART},
		{task.TypeDockerRemoveContainer, agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_CONTAINER},
		{task.TypeDockerRemoveNetwork, agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_NETWORK},
		{task.TypeDockerRemoveVolume, agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_VOLUME},
		{task.TypeDockerRemoveImage, agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_IMAGE},
	}
	for _, tc := range cases {
		if got := protoAgentTaskType(tc.value); got != tc.want {
			t.Fatalf("protoAgentTaskType(%q) = %v", tc.value, got)
		}
	}
	if got := protoAgentTaskType(task.Type("unknown")); got != agentv1.AgentTaskType_AGENT_TASK_TYPE_UNSPECIFIED {
		t.Fatalf("unknown task type = %v", got)
	}
}

func TestTaskStepNameFromAgentProto(t *testing.T) {
	t.Parallel()

	cases := []struct {
		proto agentv1.AgentTaskStepName
		want  task.StepName
	}{
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_RENDER, task.StepRender},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PULL, task.StepPull},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_BACKUP, task.StepBackup},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_COMPOSE_DOWN, task.StepComposeDown},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_COMPOSE_UP, task.StepComposeUp},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_TRANSFER, task.StepTransfer},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_RESTORE, task.StepRestore},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DNS_UPDATE, task.StepDNSUpdate},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_CADDY_SYNC, task.StepCaddySync},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_CADDY_RELOAD, task.StepCaddyReload},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_IMAGE_CHECK, task.StepImageCheck},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_INIT, task.StepInit},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PRUNE, task.StepPrune},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_AWAITING_CONFIRMATION, task.StepAwaitingConfirmation},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PERSIST_REPO, task.StepPersistRepo},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_FINALIZE, task.StepFinalize},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_START, task.StepDockerStart},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_STOP, task.StepDockerStop},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_RESTART, task.StepDockerRestart},
		{agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_REMOVE, task.StepDockerRemove},
	}
	for _, tc := range cases {
		got, ok := taskStepNameFromAgentProto(tc.proto)
		if !ok || got != tc.want {
			t.Fatalf("taskStepNameFromAgentProto(%v) = %q, %v", tc.proto, got, ok)
		}
	}
	if got, ok := taskStepNameFromAgentProto(agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_UNSPECIFIED); ok || got != "" {
		t.Fatalf("unspecified step name = %q, %v", got, ok)
	}
}

func TestTaskStatusFromAgentProto(t *testing.T) {
	t.Parallel()

	cases := []struct {
		proto agentv1.AgentTaskStatus
		want  task.Status
	}{
		{agentv1.AgentTaskStatus_AGENT_TASK_STATUS_PENDING, task.StatusPending},
		{agentv1.AgentTaskStatus_AGENT_TASK_STATUS_RUNNING, task.StatusRunning},
		{agentv1.AgentTaskStatus_AGENT_TASK_STATUS_AWAITING_CONFIRMATION, task.StatusAwaitingConfirmation},
		{agentv1.AgentTaskStatus_AGENT_TASK_STATUS_SUCCEEDED, task.StatusSucceeded},
		{agentv1.AgentTaskStatus_AGENT_TASK_STATUS_FAILED, task.StatusFailed},
		{agentv1.AgentTaskStatus_AGENT_TASK_STATUS_CANCELLED, task.StatusCancelled},
	}
	for _, tc := range cases {
		got, ok := taskStatusFromAgentProto(tc.proto)
		if !ok || got != tc.want {
			t.Fatalf("taskStatusFromAgentProto(%v) = %q, %v", tc.proto, got, ok)
		}
	}
	if got, ok := taskStatusFromAgentProto(agentv1.AgentTaskStatus_AGENT_TASK_STATUS_UNSPECIFIED); ok || got != "" {
		t.Fatalf("unspecified agent status = %q, %v", got, ok)
	}
}
