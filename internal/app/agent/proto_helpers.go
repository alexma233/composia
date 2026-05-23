package agent

import (
	"time"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

func protoAgentTaskStatus(value task.Status) agentv1.AgentTaskStatus {
	switch value {
	case task.StatusPending:
		return agentv1.AgentTaskStatus_AGENT_TASK_STATUS_PENDING
	case task.StatusRunning:
		return agentv1.AgentTaskStatus_AGENT_TASK_STATUS_RUNNING
	case task.StatusAwaitingConfirmation:
		return agentv1.AgentTaskStatus_AGENT_TASK_STATUS_AWAITING_CONFIRMATION
	case task.StatusSucceeded:
		return agentv1.AgentTaskStatus_AGENT_TASK_STATUS_SUCCEEDED
	case task.StatusFailed:
		return agentv1.AgentTaskStatus_AGENT_TASK_STATUS_FAILED
	case task.StatusCancelled:
		return agentv1.AgentTaskStatus_AGENT_TASK_STATUS_CANCELLED
	default:
		return agentv1.AgentTaskStatus_AGENT_TASK_STATUS_UNSPECIFIED
	}
}

func agentTaskStatusText(value agentv1.AgentTaskStatus) string {
	switch value {
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_PENDING:
		return string(task.StatusPending)
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_RUNNING:
		return string(task.StatusRunning)
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_AWAITING_CONFIRMATION:
		return string(task.StatusAwaitingConfirmation)
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_SUCCEEDED:
		return string(task.StatusSucceeded)
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_FAILED:
		return string(task.StatusFailed)
	case agentv1.AgentTaskStatus_AGENT_TASK_STATUS_CANCELLED:
		return string(task.StatusCancelled)
	default:
		return ""
	}
}

func protoAgentTaskStepName(value task.StepName) agentv1.AgentTaskStepName {
	switch value {
	case task.StepRender:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_RENDER
	case task.StepPull:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PULL
	case task.StepBackup:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_BACKUP
	case task.StepComposeDown:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_COMPOSE_DOWN
	case task.StepComposeUp:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_COMPOSE_UP
	case task.StepTransfer:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_TRANSFER
	case task.StepRestore:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_RESTORE
	case task.StepDNSUpdate:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DNS_UPDATE
	case task.StepCaddySync:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_CADDY_SYNC
	case task.StepCaddyReload:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_CADDY_RELOAD
	case task.StepImageCheck:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_IMAGE_CHECK
	case task.StepInit:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_INIT
	case task.StepPrune:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PRUNE
	case task.StepAwaitingConfirmation:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_AWAITING_CONFIRMATION
	case task.StepPersistRepo:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PERSIST_REPO
	case task.StepFinalize:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_FINALIZE
	case task.StepDockerStart:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_START
	case task.StepDockerStop:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_STOP
	case task.StepDockerRestart:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_RESTART
	case task.StepDockerRemove:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_REMOVE
	default:
		return agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_UNSPECIFIED
	}
}

func agentTaskStepNameToTask(value agentv1.AgentTaskStepName) task.StepName {
	switch value {
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_RENDER:
		return task.StepRender
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PULL:
		return task.StepPull
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_BACKUP:
		return task.StepBackup
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_COMPOSE_DOWN:
		return task.StepComposeDown
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_COMPOSE_UP:
		return task.StepComposeUp
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_TRANSFER:
		return task.StepTransfer
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_RESTORE:
		return task.StepRestore
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DNS_UPDATE:
		return task.StepDNSUpdate
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_CADDY_SYNC:
		return task.StepCaddySync
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_CADDY_RELOAD:
		return task.StepCaddyReload
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_IMAGE_CHECK:
		return task.StepImageCheck
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_INIT:
		return task.StepInit
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PRUNE:
		return task.StepPrune
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_AWAITING_CONFIRMATION:
		return task.StepAwaitingConfirmation
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_PERSIST_REPO:
		return task.StepPersistRepo
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_FINALIZE:
		return task.StepFinalize
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_START:
		return task.StepDockerStart
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_STOP:
		return task.StepDockerStop
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_RESTART:
		return task.StepDockerRestart
	case agentv1.AgentTaskStepName_AGENT_TASK_STEP_NAME_DOCKER_REMOVE:
		return task.StepDockerRemove
	default:
		return ""
	}
}

func protoDockerQueryErrorCode(err error) agentv1.DockerQueryErrorCode {
	if err == nil {
		return agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_UNSPECIFIED
	}
	switch dockerQueryErrorCode(err) {
	case "invalid_argument":
		return agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_INVALID_ARGUMENT
	case "not_found":
		return agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_NOT_FOUND
	case "failed_precondition":
		return agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_FAILED_PRECONDITION
	case "permission_denied":
		return agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_PERMISSION_DENIED
	case "deadline_exceeded":
		return agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_DEADLINE_EXCEEDED
	case "unavailable":
		return agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_UNAVAILABLE
	default:
		return agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_INTERNAL
	}
}

func protoExecTimestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value.UTC())
}

func applyDockerQueryResult(request *agentv1.ReportDockerQueryResultRequest, query *agentv1.DockerQueryTask, result dockerTaskResult) {
	if request == nil {
		return
	}
	if len(result.Containers) > 0 || result.TotalCount > 0 {
		request.Result = &agentv1.ReportDockerQueryResultRequest_ListContainers{ListContainers: &agentv1.ListContainersResponse{Containers: result.Containers, TotalCount: result.TotalCount}}
		return
	}
	if len(result.Networks) > 0 {
		request.Result = &agentv1.ReportDockerQueryResultRequest_ListNetworks{ListNetworks: &agentv1.ListNetworksResponse{Networks: result.Networks, TotalCount: result.TotalCount}}
		return
	}
	if len(result.Volumes) > 0 {
		request.Result = &agentv1.ReportDockerQueryResultRequest_ListVolumes{ListVolumes: &agentv1.ListVolumesResponse{Volumes: result.Volumes, TotalCount: result.TotalCount}}
		return
	}
	if len(result.Images) > 0 {
		request.Result = &agentv1.ReportDockerQueryResultRequest_ListImages{ListImages: &agentv1.ListImagesResponse{Images: result.Images, TotalCount: result.TotalCount}}
		return
	}
	if result.Exec != nil {
		startedAt, _ := time.Parse(time.RFC3339, result.Exec.StartedAt)
		finishedAt, _ := time.Parse(time.RFC3339, result.Exec.FinishedAt)
		request.Result = &agentv1.ReportDockerQueryResultRequest_RunContainerExec{RunContainerExec: &agentv1.DockerQueryRunContainerExecResponse{ExitCode: result.Exec.ExitCode, Stdout: result.Exec.Stdout, Stderr: result.Exec.Stderr, TimedOut: result.Exec.TimedOut, StdoutTruncated: result.Exec.StdoutTruncated, StderrTruncated: result.Exec.StderrTruncated, StartedAt: protoExecTimestamp(startedAt), FinishedAt: protoExecTimestamp(finishedAt), Duration: result.Exec.Duration}}
		return
	}
	if result.RawJSON != "" {
		switch query.Query.(type) {
		case *agentv1.DockerQueryTask_InspectContainer:
			request.Result = &agentv1.ReportDockerQueryResultRequest_InspectContainer{InspectContainer: &agentv1.InspectContainerResponse{RawJson: result.RawJSON}}
		case *agentv1.DockerQueryTask_InspectNetwork:
			request.Result = &agentv1.ReportDockerQueryResultRequest_InspectNetwork{InspectNetwork: &agentv1.InspectNetworkResponse{RawJson: result.RawJSON}}
		case *agentv1.DockerQueryTask_InspectVolume:
			request.Result = &agentv1.ReportDockerQueryResultRequest_InspectVolume{InspectVolume: &agentv1.InspectVolumeResponse{RawJson: result.RawJSON}}
		case *agentv1.DockerQueryTask_InspectImage:
			request.Result = &agentv1.ReportDockerQueryResultRequest_InspectImage{InspectImage: &agentv1.InspectImageResponse{RawJson: result.RawJSON}}
		}
		return
	}
	request.Result = &agentv1.ReportDockerQueryResultRequest_GetContainerLogs{GetContainerLogs: &agentv1.GetContainerLogsResponse{Content: result.Content}}
}
