package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func executePulledTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask) error {
	logUploader := newTaskLogUploader(client, pulledTask.GetTaskId())
	defer func() {
		if err := logUploader.Close(); err != nil {
			log.Printf("close task log uploader: %v", err)
		}
	}()

	switch pulledTask.GetType() {
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_DEPLOY:
		return executeDeployTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_UPDATE:
		return executeUpdateTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_BACKUP:
		return executeBackupTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_RESTORE:
		return executeRestoreTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_STOP:
		return executeStopTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_RESTART:
		return executeRestartTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_PRUNE:
		return executePruneTask(ctx, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_INIT:
		return executeRusticInitTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_FORGET:
		return executeRusticForgetTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_RUSTIC_PRUNE:
		return executeRusticPruneTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_CADDY_SYNC:
		return executeCaddySyncTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_CADDY_RELOAD:
		return executeCaddyReloadTask(ctx, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_IMAGE_CHECK:
		return executeImageCheckTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_START,
		agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_STOP,
		agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_RESTART,
		agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_CONTAINER,
		agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_NETWORK,
		agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_VOLUME,
		agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_IMAGE:
		return executeDockerTask(ctx, client, cfg, pulledTask, logUploader)
	default:
		return failTask(ctx, client, pulledTask.GetTaskId(), fmt.Errorf("task type %s is not implemented", pulledTask.GetType().String()))
	}
}

type pruneTaskParams struct {
	Target string `json:"target"`
}

type rusticMaintenanceTaskParams struct {
	ServiceDir  string `json:"service_dir,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	DataName    string `json:"data_name,omitempty"`
	RepoWide    bool   `json:"repo_wide,omitempty"`
}

func parsePruneParams(pulledTask *agentv1.AgentTask) pruneTaskParams {
	paramsJSON := pulledTask.GetParamsJson()
	if paramsJSON == "" {
		return pruneTaskParams{Target: "all"}
	}
	var params pruneTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return pruneTaskParams{Target: "all"}
	}
	if params.Target == "" {
		params.Target = "all"
	}
	return params
}

func parseRusticMaintenanceParams(pulledTask *agentv1.AgentTask) (rusticMaintenanceTaskParams, error) {
	paramsJSON := pulledTask.GetParamsJson()
	if paramsJSON == "" {
		return rusticMaintenanceTaskParams{}, nil
	}
	var params rusticMaintenanceTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return rusticMaintenanceTaskParams{}, fmt.Errorf("decode rustic maintenance task params: %w", err)
	}
	return params, nil
}

func decodeTaskParams(paramsJSON string) (controllerTaskParams, error) {
	if paramsJSON == "" {
		return controllerTaskParams{}, nil
	}
	var params controllerTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return controllerTaskParams{}, fmt.Errorf("decode task params: %w", err)
	}
	return params, nil
}

type controllerTaskParams struct {
	ServiceDirs           []string                       `json:"service_dirs,omitempty"`
	ImageNames            []string                       `json:"image_names,omitempty"`
	SemverAllow           []string                       `json:"semver_allow,omitempty"`
	ForgeCandidates       map[string][]string            `json:"forge_candidates,omitempty"`
	ForgeCandidateSources map[string]map[string][]string `json:"forge_candidate_sources,omitempty"`
	FullRebuild           bool                           `json:"full_rebuild,omitempty"`
	ComposeRecreateMode   string                         `json:"compose_recreate_mode,omitempty"`
}
