package agent

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	backupcfg "forgejo.alexma.top/alexma233/composia/internal/core/backup"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func executeRusticPruneTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	rusticMeta, err := loadRusticTaskMeta(serviceRoot, pulledTask.GetServiceName())
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting rustic prune task for service=%s compose_service=%s\n", pulledTask.GetServiceName(), rusticMeta.ComposeService)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepPrune, func() error {
		return runRusticPrune(ctx, serviceRoot, rusticMeta, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "rustic prune task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRusticInitTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	rusticMeta, err := loadRusticTaskMeta(serviceRoot, pulledTask.GetServiceName())
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting rustic init task for service=%s compose_service=%s\n", pulledTask.GetServiceName(), rusticMeta.ComposeService)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepInit, func() error {
		return runRusticInit(ctx, serviceRoot, rusticMeta, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "rustic init task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRusticForgetTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params, err := parseRusticMaintenanceParams(pulledTask)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	rusticMeta, err := loadRusticTaskMeta(serviceRoot, pulledTask.GetServiceName())
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting rustic forget task for service=%s compose_service=%s repo_wide=%t\n", pulledTask.GetServiceName(), rusticMeta.ComposeService, params.RepoWide)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepPrune, func() error {
		return runRusticForget(ctx, serviceRoot, rusticMeta, params, pulledTask.GetNodeId(), func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "rustic forget task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

type rusticTaskMeta struct {
	ComposeService string
	Profile        string
	InitArgs       []string
	Compose        composeCommandConfig
}

func loadRusticTaskMeta(serviceDir, fallback string) (rusticTaskMeta, error) {
	compose, meta, err := loadComposeCommandConfig(serviceDir, fallback)
	if err != nil {
		return rusticTaskMeta{}, err
	}
	return rusticTaskMeta{ComposeService: meta.RusticComposeService(), Profile: meta.RusticProfile(), InitArgs: meta.RusticInitArgs(), Compose: compose}, nil
}

func runRusticInit(ctx context.Context, serviceDir string, meta rusticTaskMeta, uploadLog func(string) error) error {
	args := buildRusticComposeRunArgs(meta.Compose, meta.ComposeService, meta.Profile, nil, append([]string{"init"}, meta.InitArgs...)...)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose run rustic init failed: %w", err)
	}
	return nil
}

func runRusticForget(ctx context.Context, serviceDir string, meta rusticTaskMeta, params rusticMaintenanceTaskParams, nodeID string, uploadLog func(string) error) error {
	args := buildRusticComposeRunArgs(meta.Compose, meta.ComposeService, meta.Profile, nil, "forget")
	if !params.RepoWide && nodeID != "" {
		args = append(args, "--filter-host", nodeID)
	}
	if !params.RepoWide && params.ServiceName != "" {
		args = append(args, "--filter-tags", "composia-service:"+params.ServiceName)
	}
	if !params.RepoWide && params.DataName != "" {
		args = append(args, "--filter-tags", "composia-data:"+params.DataName)
	}
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose run rustic forget failed: %w", err)
	}
	return nil
}

func runRusticPrune(ctx context.Context, serviceDir string, meta rusticTaskMeta, uploadLog func(string) error) error {
	args := buildRusticComposeRunArgs(meta.Compose, meta.ComposeService, meta.Profile, nil, "prune")
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose run rustic prune failed: %w", err)
	}
	return nil
}

var rusticSnapshotRegexp = regexp.MustCompile(`(?m)snapshot\s+([0-9a-fA-F]+)\b[^\n]*\bsaved\.?`)

func runRusticBackup(ctx context.Context, rusticDir string, rustic *backupcfg.RusticConfig, sourceDir string, item backupcfg.RuntimeItem, logUploader *taskLogUploader, extraVolumes []string) (string, error) {
	compose, _, err := loadComposeCommandConfig(rusticDir, rustic.ServiceName)
	if err != nil {
		return "", err
	}
	args := buildRusticComposeRunArgs(compose, rustic.ComposeService, rustic.Profile, extraVolumes, "backup", "--host", rustic.NodeID)
	for _, tag := range buildRusticTags(item.Tags) {
		args = append(args, "--tag", tag)
	}
	args = append(args, sourceDir, "--as-path", item.Name)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = rusticDir
	output, err := runCommandWithLiveLogsAndCapture(command, func(output string) error {
		return uploadTaskLog(ctx, logUploader, output)
	})
	if err != nil {
		return "", fmt.Errorf("docker compose run rustic backup failed: %w", err)
	}
	matches := rusticSnapshotRegexp.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", fmt.Errorf("could not parse rustic snapshot id from output")
	}
	return matches[1], nil
}

func runRusticRestore(ctx context.Context, rusticDir string, rustic *backupcfg.RusticConfig, artifactRef, targetDir string, logUploader *taskLogUploader, extraVolumes []string) error {
	compose, _, err := loadComposeCommandConfig(rusticDir, rustic.ServiceName)
	if err != nil {
		return err
	}
	args := buildRusticComposeRunArgs(compose, rustic.ComposeService, rustic.Profile, extraVolumes, "restore", artifactRef, targetDir)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = rusticDir
	if err := runCommandWithLiveLogs(command, func(output string) error {
		return uploadTaskLog(ctx, logUploader, output)
	}); err != nil {
		return fmt.Errorf("docker compose run rustic restore failed: %w", err)
	}
	return nil
}

func buildRusticTags(explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	return nil
}
