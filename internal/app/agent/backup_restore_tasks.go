package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	backupcfg "forgejo.alexma.top/alexma233/composia/internal/core/backup"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func executeBackupTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	if len(pulledTask.GetDataNames()) == 0 {
		err := fmt.Errorf("backup task is missing data_names")
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	var bundle *bundleResult
	var rusticBundle *bundleResult
	var runtimeConfig *backupcfg.RuntimeConfig
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		runtimeConfig, err = loadBackupRuntimeConfig(bundle.RootPath)
		if err != nil {
			return err
		}
		if runtimeConfig.Rustic.ServiceDir == bundle.RelativeRoot {
			rusticBundle = bundle
		} else {
			rusticBundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), runtimeConfig.Rustic.ServiceDir)
			if err != nil {
				return err
			}
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote backup task for service=%s data_names=%s\n", pulledTask.GetServiceName(), strings.Join(pulledTask.GetDataNames(), ","))); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepBackup, func() error {
		for _, item := range runtimeConfig.Items {
			artifactRef, startedAt, finishedAt, err := backupRuntimeItem(ctx, cfg, bundle.RootPath, rusticBundle.RootPath, pulledTask.GetTaskId(), item, runtimeConfig.Rustic, logUploader)
			if err != nil {
				_ = reportBackupResult(ctx, client, pulledTask.GetTaskId(), pulledTask.GetServiceName(), item.Name, "", task.StatusFailed, startedAt, time.Now().UTC(), err.Error())
				return err
			}
			if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("backup completed for %s artifact=%s\n", item.Name, artifactRef)); err != nil {
				return err
			}
			if err := reportBackupResult(ctx, client, pulledTask.GetTaskId(), pulledTask.GetServiceName(), item.Name, artifactRef, task.StatusSucceeded, startedAt, finishedAt, ""); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "backup task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRestoreTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	var bundle *bundleResult
	var rusticBundle *bundleResult
	var runtimeConfig *backupcfg.RestoreConfig
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		runtimeConfig, err = loadRestoreRuntimeConfig(bundle.RootPath)
		if err != nil {
			return err
		}
		if runtimeConfig.Rustic.ServiceDir == bundle.RelativeRoot {
			rusticBundle = bundle
		} else {
			rusticBundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), runtimeConfig.Rustic.ServiceDir)
			if err != nil {
				return err
			}
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote restore task for service=%s node=%s\n", pulledTask.GetServiceName(), pulledTask.GetNodeId())); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRestore, func() error {
		for _, item := range runtimeConfig.Items {
			if err := restoreRuntimeItem(ctx, cfg, serviceRoot, rusticBundle.RootPath, pulledTask.GetTaskId(), item, runtimeConfig.Rustic, logUploader); err != nil {
				return err
			}
			if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("restore completed for %s\n", item.Name)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "restore task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func loadBackupRuntimeConfig(serviceRoot string) (*backupcfg.RuntimeConfig, error) {
	content, err := os.ReadFile(filepath.Join(serviceRoot, ".composia-backup.json"))
	if err != nil {
		return nil, fmt.Errorf("read backup runtime config: %w", err)
	}
	var cfg backupcfg.RuntimeConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("decode backup runtime config: %w", err)
	}
	if cfg.Rustic == nil {
		return nil, fmt.Errorf("backup runtime config is missing rustic provider")
	}
	if cfg.Rustic.ServiceDir == "" {
		return nil, fmt.Errorf("backup runtime config is missing rustic service_dir")
	}
	if cfg.Rustic.NodeID == "" {
		return nil, fmt.Errorf("backup runtime config is missing rustic node_id")
	}
	if len(cfg.Items) == 0 {
		return nil, fmt.Errorf("backup runtime config did not include any items")
	}
	return &cfg, nil
}

func loadRestoreRuntimeConfig(serviceRoot string) (*backupcfg.RestoreConfig, error) {
	content, err := os.ReadFile(filepath.Join(serviceRoot, ".composia-restore.json"))
	if err != nil {
		return nil, fmt.Errorf("read restore runtime config: %w", err)
	}
	var cfg backupcfg.RestoreConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("decode restore runtime config: %w", err)
	}
	if cfg.Rustic == nil {
		return nil, fmt.Errorf("restore runtime config is missing rustic provider")
	}
	if cfg.Rustic.ServiceDir == "" {
		return nil, fmt.Errorf("restore runtime config is missing rustic service_dir")
	}
	if len(cfg.Items) == 0 {
		return nil, fmt.Errorf("restore runtime config did not include any items")
	}
	return &cfg, nil
}

func backupRuntimeItem(ctx context.Context, cfg *config.AgentConfig, serviceRoot, rusticRoot, taskID string, item backupcfg.RuntimeItem, rustic *backupcfg.RusticConfig, logUploader *taskLogUploader) (string, time.Time, time.Time, error) {
	startedAt := time.Now().UTC()
	stagingDir, err := dataProtectStageDir(cfg.StateDir, fmt.Sprintf("backup-%s-%s-", taskID, item.Name))
	if err != nil {
		return "", startedAt, time.Time{}, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()
	if err := stageBackupItem(ctx, serviceRoot, stagingDir, item, logUploader); err != nil {
		return "", startedAt, time.Time{}, err
	}
	rusticSourceDir, err := rusticDataProtectPath(stagingDir, cfg, rustic)
	if err != nil {
		return "", startedAt, time.Time{}, err
	}
	artifactRef, err := runRusticBackup(ctx, rusticRoot, rustic, rusticSourceDir, item, logUploader)
	if err != nil {
		return "", startedAt, time.Time{}, err
	}
	return artifactRef, startedAt, time.Now().UTC(), nil
}

func restoreRuntimeItem(ctx context.Context, cfg *config.AgentConfig, serviceRoot, rusticRoot, taskID string, item backupcfg.RestoreItem, rustic *backupcfg.RusticConfig, logUploader *taskLogUploader) error {
	stagingDir, err := dataProtectStageDir(cfg.StateDir, fmt.Sprintf("restore-%s-%s-", taskID, item.Name))
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()
	rusticTargetDir, err := rusticDataProtectPath(stagingDir, cfg, rustic)
	if err != nil {
		return err
	}
	if err := runRusticRestore(ctx, rusticRoot, rustic, item.ArtifactRef, rusticTargetDir, logUploader); err != nil {
		return err
	}
	return applyRestoreItem(ctx, serviceRoot, filepath.Join(stagingDir, item.Name), item, logUploader)
}

func applyRestoreItem(ctx context.Context, serviceRoot, stagingDir string, item backupcfg.RestoreItem, logUploader *taskLogUploader) (retErr error) {
	switch item.Strategy {
	case "files.copy", "files.copy_after_stop":
		if item.Strategy == "files.copy_after_stop" {
			compose, _, err := loadComposeCommandConfig(serviceRoot, filepath.Base(serviceRoot))
			if err != nil {
				return err
			}
			if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("stopping compose project %s for cold restore item %s\n", compose.ProjectName, item.Name)); err != nil {
				return err
			}
			if err := runComposeDown(ctx, serviceRoot, compose, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); err != nil {
				return err
			}
			defer func() {
				if restartErr := runComposeUp(ctx, serviceRoot, compose, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); restartErr != nil {
					if retErr == nil {
						retErr = fmt.Errorf("restart compose project after cold restore: %w", restartErr)
						return
					}
					_ = uploadTaskLog(ctx, logUploader, fmt.Sprintf("restart compose project after cold restore failed: %v\n", restartErr))
				}
			}()
		}
		for _, include := range item.Include {
			if err := restoreInclude(ctx, serviceRoot, stagingDir, include); err != nil {
				return fmt.Errorf("restore item %s include %s: %w", item.Name, include, err)
			}
		}
		return nil
	case "database.pgimport":
		compose, _, err := loadComposeCommandConfig(serviceRoot, filepath.Base(serviceRoot))
		if err != nil {
			return err
		}
		dumpPath := filepath.Join(stagingDir, item.Name+".sql")
		return runComposePGImport(ctx, serviceRoot, compose, item.Service, dumpPath, func(output string) error { return uploadTaskLog(ctx, logUploader, output) })
	default:
		return fmt.Errorf("restore strategy %q is not implemented yet", item.Strategy)
	}
}

func restoreInclude(ctx context.Context, serviceRoot, stagingDir, include string) error {
	kind, normalized, err := repo.ClassifyDataInclude(include)
	if err != nil {
		return err
	}
	if kind == repo.DataIncludeKindServicePath {
		sourcePath := filepath.Join(stagingDir, "paths", sanitizeStagePath(include))
		targetPath, err := resolveIncludeServicePath(serviceRoot, normalized)
		if err != nil {
			return err
		}
		return replacePath(sourcePath, targetPath)
	}
	return restoreDirToVolume(ctx, filepath.Join(stagingDir, "volumes", sanitizeStagePath(normalized)), normalized)
}

func replacePath(sourcePath, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat restore source %q: %w", sourcePath, err)
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("clear restore target %q: %w", targetPath, err)
	}
	if info.IsDir() {
		return copyDir(sourcePath, targetPath)
	}
	return copyFile(sourcePath, targetPath, info.Mode())
}

func stageBackupItem(ctx context.Context, serviceRoot, stagingDir string, item backupcfg.RuntimeItem, logUploader *taskLogUploader) (retErr error) {
	switch item.Strategy {
	case "files.copy":
		for _, include := range item.Include {
			if err := stageInclude(ctx, serviceRoot, stagingDir, include); err != nil {
				return fmt.Errorf("stage backup item %s include %s: %w", item.Name, include, err)
			}
		}
		return nil
	case "files.copy_after_stop":
		compose, _, err := loadComposeCommandConfig(serviceRoot, filepath.Base(serviceRoot))
		if err != nil {
			return err
		}
		if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("stopping compose project %s for cold backup item %s\n", compose.ProjectName, item.Name)); err != nil {
			return err
		}
		if err := runComposeDown(ctx, serviceRoot, compose, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); err != nil {
			return err
		}
		defer func() {
			if restartErr := runComposeUp(ctx, serviceRoot, compose, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); restartErr != nil && retErr == nil {
				retErr = fmt.Errorf("restart compose project after cold backup: %w", restartErr)
			}
		}()
		for _, include := range item.Include {
			if err := stageInclude(ctx, serviceRoot, stagingDir, include); err != nil {
				return fmt.Errorf("stage cold backup item %s include %s: %w", item.Name, include, err)
			}
		}
		return nil
	case "database.pgdumpall":
		compose, _, err := loadComposeCommandConfig(serviceRoot, filepath.Base(serviceRoot))
		if err != nil {
			return err
		}
		targetPath := filepath.Join(stagingDir, item.Name+".sql")
		if err := runComposePGDumpAll(ctx, serviceRoot, compose, item.Service, targetPath, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("backup strategy %q is not implemented yet", item.Strategy)
	}
}

func stageInclude(ctx context.Context, serviceRoot, stagingDir, include string) error {
	kind, normalized, err := repo.ClassifyDataInclude(include)
	if err != nil {
		return err
	}
	if kind == repo.DataIncludeKindServicePath {
		sourcePath, err := resolveIncludeServicePath(serviceRoot, normalized)
		if err != nil {
			return err
		}
		return copyIntoStage(sourcePath, filepath.Join(stagingDir, "paths", sanitizeStagePath(include)))
	}
	return stageVolumeToDir(ctx, filepath.Join(stagingDir, "volumes", sanitizeStagePath(normalized)), normalized)
}

func resolveIncludeServicePath(serviceRoot, includePath string) (string, error) {
	serviceRoot = filepath.Clean(serviceRoot)
	targetPath := filepath.Join(serviceRoot, includePath)
	relPath, err := filepath.Rel(serviceRoot, targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve include path %q: %w", includePath, err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("include %q must stay within the service root", includePath)
	}
	return targetPath, nil
}

func sanitizeStagePath(value string) string {
	replacer := strings.NewReplacer("/", "_", `\\`, "_", ":", "_")
	return replacer.Replace(strings.TrimPrefix(strings.TrimPrefix(value, "./"), "/"))
}
