package agent

import (
	"context"
	"fmt"
	"os/exec"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func executePruneTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params := parsePruneParams(pulledTask)
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting prune task: target=%s\n", params.Target)); err != nil {
		return err
	}

	var pruneErr error
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepPrune, func() error {
		pruneErr = runDockerPrune(ctx, params.Target, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
		return pruneErr
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}

	if pruneErr != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), pruneErr)
	}

	if err := uploadTaskLog(ctx, logUploader, "prune task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func runDockerPrune(ctx context.Context, target string, uploadLog func(string) error) error {
	var args []string

	switch target {
	case "all":
		return runDockerPruneAll(ctx, uploadLog)
	case "containers":
		args = []string{"container", "prune", "-f"}
	case "networks":
		args = []string{"network", "prune", "-f"}
	case "images":
		args = []string{"image", "prune", "-f"}
	case "images_all":
		args = []string{"image", "prune", "-a", "-f"}
	case "volumes":
		args = []string{"volume", "prune", "-f"}
	case "system_all":
		args = []string{"system", "prune", "-a", "-f"}
	case "system_all_volumes":
		args = []string{"system", "prune", "-a", "--volumes", "-f"}
	case "builder":
		args = []string{"builder", "prune", "-f"}
	default:
		return fmt.Errorf("unknown prune target: %q", target)
	}

	cmd := exec.CommandContext(ctx, "docker", args...) //nolint:gosec
	if err := runCommandWithLiveLogs(cmd, uploadLog); err != nil {
		return fmt.Errorf("docker %s prune failed: %w", target, err)
	}
	return nil
}

func runDockerPruneAll(ctx context.Context, uploadLog func(string) error) error {
	targets := []string{"containers", "networks", "images", "volumes", "builder"}
	for _, target := range targets {
		if err := uploadLog(fmt.Sprintf("pruning %s...\n", target)); err != nil {
			return err
		}
		if err := runDockerPrune(ctx, target, uploadLog); err != nil {
			return err
		}
	}
	return nil
}
