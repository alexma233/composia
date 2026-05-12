package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"google.golang.org/protobuf/proto"
)

func executeCaddyReloadTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, nil)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	caddyMeta, err := loadCaddyInfraMeta(serviceRoot, pulledTask.GetServiceName())
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting caddy reload task for service=%s compose_service=%s config_dir=%s\n", pulledTask.GetServiceName(), caddyMeta.ComposeService, caddyMeta.ConfigDir)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepCaddyReload, func() error {
		return runCaddyReload(ctx, serviceRoot, caddyMeta.Compose, caddyMeta.ComposeService, caddyMeta.ConfigDir, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "caddy reload task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeCaddySyncTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params, err := decodeTaskParams(pulledTask.GetParamsJson())
	if err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting caddy sync task for service=%s node=%s repo_revision=%s full_rebuild=%t\n", pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision(), params.FullRebuild)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		return uploadTaskLog(ctx, logUploader, "render step completed for caddy sync task\n")
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepCaddySync, func() error {
		return syncCaddyFilesForTask(ctx, bundleClient, client, cfg, pulledTask, params, logUploader)
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := uploadTaskLog(ctx, logUploader, "caddy sync task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

type caddyInfraMeta struct {
	ComposeService string
	ConfigDir      string
	Compose        composeCommandConfig
}

type caddyServiceMeta struct {
	Source string
}

func loadCaddyInfraMeta(serviceDir, fallback string) (caddyInfraMeta, error) {
	compose, meta, err := loadComposeCommandConfig(serviceDir, fallback)
	if err != nil {
		return caddyInfraMeta{}, err
	}
	return caddyInfraMeta{
		ComposeService: meta.CaddyComposeService(),
		ConfigDir:      meta.CaddyConfigDir(),
		Compose:        compose,
	}, nil
}

func loadServiceCaddyMeta(serviceDir string) (caddyServiceMeta, error) {
	meta, err := repo.LoadServiceMeta(filepath.Join(serviceDir, "composia-meta.yaml"))
	if err != nil {
		return caddyServiceMeta{}, err
	}
	return caddyServiceMeta{Source: repo.CaddySource(repo.Service{Meta: meta})}, nil
}

func runCaddyReload(ctx context.Context, serviceDir string, compose composeCommandConfig, composeService, configDir string, uploadLog func(string) error) error {
	configPath := filepath.Join(configDir, "Caddyfile")
	command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "exec", "-T", composeService, "caddy", "reload", "--config", configPath, "--adapter", "caddyfile")...)
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose exec caddy reload failed: %w", err)
	}
	return nil
}

func executeCaddySyncStep(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader, serviceRoot string) error {
	return executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepCaddySync, func() error {
		return syncServiceCaddyFile(ctx, cfg, pulledTask.GetServiceDir(), serviceRoot, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	})
}

func syncCaddyFilesForTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, params controllerTaskParams, logUploader *taskLogUploader) error {
	serviceDirs := append([]string(nil), params.ServiceDirs...)
	if len(serviceDirs) == 0 && pulledTask.GetServiceDir() != "" {
		serviceDirs = []string{pulledTask.GetServiceDir()}
	}
	if params.FullRebuild {
		entries, err := os.ReadDir(cfg.CaddyGeneratedDir())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read generated caddy directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".caddy") {
				continue
			}
			if err := os.Remove(filepath.Join(cfg.CaddyGeneratedDir(), entry.Name())); err != nil {
				return fmt.Errorf("remove generated caddy file %q: %w", entry.Name(), err)
			}
			if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("removed generated caddy file %s\n", filepath.Join(cfg.CaddyGeneratedDir(), entry.Name()))); err != nil {
				return err
			}
		}
	}
	for _, serviceDir := range serviceDirs {
		bundleTask := proto.Clone(pulledTask).(*agentv1.AgentTask)
		bundleTask.ServiceDir = serviceDir
		bundleTask.ServiceName = filepath.Base(serviceDir)
		bundle, err := downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), serviceDir)
		if err != nil {
			return err
		}
		serviceRoot, err := localServiceRoot(cfg.RepoDir, bundleTask, bundle)
		if err != nil {
			return err
		}
		if err := syncServiceCaddyFile(ctx, cfg, bundleTask.GetServiceDir(), serviceRoot, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		}); err != nil {
			return err
		}
	}
	return nil
}

func syncServiceCaddyFile(ctx context.Context, cfg *config.AgentConfig, serviceDir, serviceRoot string, uploadLog func(string) error) error {
	targetName, err := caddyGeneratedFileName(serviceDir)
	if err != nil {
		return err
	}
	meta, err := loadServiceCaddyMeta(serviceRoot)
	if err != nil {
		return err
	}
	if meta.Source == "" {
		if err := uploadLog(fmt.Sprintf("service_dir=%s does not enable network.caddy, skipping caddy sync\n", serviceDir)); err != nil {
			return err
		}
		return nil
	}
	sourcePath, err := resolveServiceCaddySourcePath(serviceRoot, meta.Source)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(cfg.CaddyGeneratedDir(), targetName)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create generated caddy directory for %q: %w", targetPath, err)
	}
	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read caddy source %q: %w", sourcePath, err)
	}
	if err := os.WriteFile(targetPath, contents, 0o644); err != nil {
		return fmt.Errorf("write generated caddy file %q: %w", targetPath, err)
	}
	if err := uploadLog(fmt.Sprintf("synced caddy file source=%s target=%s\n", sourcePath, targetPath)); err != nil {
		return err
	}
	return nil
}

func removeServiceCaddyFile(ctx context.Context, cfg *config.AgentConfig, serviceDir string, uploadLog func(string) error) error {
	targetName, err := caddyGeneratedFileName(serviceDir)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(cfg.CaddyGeneratedDir(), targetName)
	if err := os.Remove(targetPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := uploadLog(fmt.Sprintf("generated caddy file %s does not exist, skipping removal\n", targetPath)); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("remove generated caddy file %q: %w", targetPath, err)
	}
	if err := uploadLog(fmt.Sprintf("removed generated caddy file %s\n", targetPath)); err != nil {
		return err
	}
	return nil
}

func caddyGeneratedFileName(serviceDir string) (string, error) {
	cleanDir := filepath.Clean(strings.TrimSpace(serviceDir))
	if cleanDir == "" || cleanDir == "." {
		return "", fmt.Errorf("service_dir is required for caddy generated file")
	}
	base := filepath.Base(cleanDir)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "", fmt.Errorf("resolve caddy generated file name from service_dir %q", serviceDir)
	}
	return base + ".caddy", nil
}

func resolveServiceCaddySourcePath(serviceRoot, source string) (string, error) {
	cleanSource := filepath.Clean(strings.TrimSpace(source))
	if cleanSource == "." || cleanSource == "" {
		return "", fmt.Errorf("network.caddy.source must not be empty")
	}
	resolved := filepath.Join(serviceRoot, cleanSource)
	relative, err := filepath.Rel(serviceRoot, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve caddy source path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("network.caddy.source %q escapes service root", source)
	}
	return resolved, nil
}
