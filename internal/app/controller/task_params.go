package controller

import (
	"encoding/json"
	"fmt"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type serviceTaskParams struct {
	ServiceDir            string                         `json:"service_dir"`
	ServiceDirs           []string                       `json:"service_dirs,omitempty"`
	DataNames             []string                       `json:"data_names,omitempty"`
	ImageNames            []string                       `json:"image_names,omitempty"`
	SemverAllow           []string                       `json:"semver_allow,omitempty"`
	ForgeCandidates       map[string][]string            `json:"forge_candidates,omitempty"`
	ForgeCandidateSources map[string]map[string][]string `json:"forge_candidate_sources,omitempty"`
	FullRebuild           bool                           `json:"full_rebuild,omitempty"`
	SourceNodeID          string                         `json:"source_node_id,omitempty"`
	TargetNodeID          string                         `json:"target_node_id,omitempty"`
	RestoreItems          []restoreTaskItem              `json:"restore_items,omitempty"`
	ComposeRecreateMode   string                         `json:"compose_recreate_mode,omitempty"`
}

type restoreTaskItem struct {
	DataName     string `json:"data_name"`
	ArtifactRef  string `json:"artifact_ref"`
	SourceTaskID string `json:"source_task_id,omitempty"`
}

type rusticMaintenanceTaskParams struct {
	ServiceDir  string `json:"service_dir,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	DataName    string `json:"data_name,omitempty"`
	RepoWide    bool   `json:"repo_wide,omitempty"`
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}

func taskParams(paramsJSON string) (serviceTaskParams, error) {
	if paramsJSON == "" {
		return serviceTaskParams{}, nil
	}
	var params serviceTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return serviceTaskParams{}, fmt.Errorf("decode task params: %w", err)
	}
	return params, nil
}

func findTaskStepStartedAt(steps []task.StepRecord, stepName task.StepName) *time.Time {
	for _, step := range steps {
		if step.StepName != stepName || step.StartedAt == nil {
			continue
		}
		startedAt := step.StartedAt.UTC()
		return &startedAt
	}
	return nil
}

func protoTime(value *timestamppb.Timestamp) *time.Time {
	if value == nil {
		return nil
	}
	parsed := value.AsTime().UTC()
	return &parsed
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
