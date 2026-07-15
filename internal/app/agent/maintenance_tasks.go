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
	args, isAll, err := dockerPruneArgs(target)
	if err != nil {
		return err
	}
	if isAll {
		return runDockerPruneAll(ctx, uploadLog)
	}

	cmd := exec.CommandContext(ctx, "docker", args...) //nolint:gosec
	if err := runCommandWithLiveLogs(cmd, uploadLog); err != nil {
		return fmt.Errorf("docker %s prune failed: %w", target, err)
	}
	return nil
}

func dockerPruneArgs(target string) ([]string, bool, error) {
	switch target {
	case "all":
		return nil, true, nil
	case "containers":
		return []string{dockerResourceContainer, dockerCommandPrune, "-f"}, false, nil
	case "networks":
		return []string{dockerResourceNetwork, dockerCommandPrune, "-f"}, false, nil
	case "images":
		return []string{dockerResourceImage, dockerCommandPrune, "-f"}, false, nil
	case "images_all":
		return []string{dockerResourceImage, dockerCommandPrune, "-a", "-f"}, false, nil
	case "volumes":
		return []string{dockerResourceVolume, dockerCommandPrune, "-f"}, false, nil
	case "system_all":
		return []string{"system", dockerCommandPrune, "-a", "-f"}, false, nil
	case "system_all_volumes":
		return []string{"system", dockerCommandPrune, "-a", "--volumes", "-f"}, false, nil
	case "builder":
		return []string{"builder", dockerCommandPrune, "-f"}, false, nil
	default:
		return nil, false, fmt.Errorf("unknown prune target: %q", target)
	}
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
