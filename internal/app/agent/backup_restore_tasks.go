package agent

import (
	"context"
	"encoding/json"
	"errors"
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
		err := errors.New("backup task is missing data_names")
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
	content, err := os.ReadFile(filepath.Join(serviceRoot, ".composia-backup.json")) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read backup runtime config: %w", err)
	}
	var cfg backupcfg.RuntimeConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("decode backup runtime config: %w", err)
	}
	if cfg.Rustic == nil {
		return nil, errors.New("backup runtime config is missing rustic provider")
	}
	if cfg.Rustic.ServiceDir == "" {
		return nil, errors.New("backup runtime config is missing rustic service_dir")
	}
	if cfg.Rustic.NodeID == "" {
		return nil, errors.New("backup runtime config is missing rustic node_id")
	}
	if len(cfg.Items) == 0 {
		return nil, errors.New("backup runtime config did not include any items")
	}
	return &cfg, nil
}

func loadRestoreRuntimeConfig(serviceRoot string) (*backupcfg.RestoreConfig, error) {
	content, err := os.ReadFile(filepath.Join(serviceRoot, ".composia-restore.json")) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read restore runtime config: %w", err)
	}
	var cfg backupcfg.RestoreConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("decode restore runtime config: %w", err)
	}
	if cfg.Rustic == nil {
		return nil, errors.New("restore runtime config is missing rustic provider")
	}
	if cfg.Rustic.ServiceDir == "" {
		return nil, errors.New("restore runtime config is missing rustic service_dir")
	}
	if len(cfg.Items) == 0 {
		return nil, errors.New("restore runtime config did not include any items")
	}
	return &cfg, nil
}

func backupRuntimeItem(ctx context.Context, cfg *config.AgentConfig, serviceRoot, rusticRoot, taskID string, item backupcfg.RuntimeItem, rustic *backupcfg.RusticConfig, logUploader *taskLogUploader) (artifactRef string, startedAt time.Time, finishedAt time.Time, retErr error) {
	startedAt = time.Now().UTC()
	stagingDir, err := dataProtectStageDir(cfg.StateDir, fmt.Sprintf("backup-%s-%s-", taskID, item.Name))
	if err != nil {
		return "", startedAt, time.Time{}, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	rusticSourceDir, err := rusticDataProtectPath(stagingDir, cfg, rustic)
	if err != nil {
		return "", startedAt, time.Time{}, err
	}

	if item.Strategy == "files.copy_after_stop" {
		compose, _, loadErr := loadComposeCommandConfig(serviceRoot, filepath.Base(serviceRoot))
		if loadErr != nil {
			return "", startedAt, time.Time{}, loadErr
		}
		if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("stopping compose project %s for cold backup item %s\n", compose.ProjectName, item.Name)); err != nil {
			return "", startedAt, time.Time{}, err
		}
		if err := runComposeDown(ctx, serviceRoot, compose, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); err != nil {
			return "", startedAt, time.Time{}, err
		}
		defer func() {
			if restartErr := runComposeUp(ctx, serviceRoot, compose, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); restartErr != nil {
				if retErr == nil {
					retErr = fmt.Errorf("restart compose project after cold backup: %w", restartErr)
					return
				}
				_ = uploadTaskLog(ctx, logUploader, fmt.Sprintf("restart compose project after cold backup failed: %v\n", restartErr))
			}
		}()
	}

	var extraVolumes []string
	switch item.Strategy {
	case "files.copy", "files.copy_after_stop":
		extraVolumes, err = buildBackupVolumeFlags(serviceRoot, rusticSourceDir, item)
		if err != nil {
			return "", startedAt, time.Time{}, err
		}
	case "database.pgdumpall":
		if err := stageBackupItem(ctx, serviceRoot, stagingDir, item, logUploader); err != nil {
			return "", startedAt, time.Time{}, err
		}
	default:
		return "", startedAt, time.Time{}, fmt.Errorf("backup strategy %q is not implemented yet", item.Strategy)
	}

	artifactRef, err = runRusticBackup(ctx, rusticRoot, rustic, rusticSourceDir, item, logUploader, extraVolumes)
	if err != nil {
		return "", startedAt, time.Time{}, err
	}
	return artifactRef, startedAt, time.Now().UTC(), nil
}

func restoreRuntimeItem(ctx context.Context, cfg *config.AgentConfig, serviceRoot, rusticRoot, taskID string, item backupcfg.RestoreItem, rustic *backupcfg.RusticConfig, logUploader *taskLogUploader) (retErr error) {
	stagingDir, err := dataProtectStageDir(cfg.StateDir, fmt.Sprintf("restore-%s-%s-", taskID, item.Name))
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()
	rusticTargetDir, err := rusticDataProtectPath(stagingDir, cfg, rustic)
	if err != nil {
		return err
	}

	var extraVolumes []string
	switch item.Strategy {
	case "files.copy", "files.copy_after_stop":
		if item.Strategy == "files.copy_after_stop" {
			compose, _, loadErr := loadComposeCommandConfig(serviceRoot, filepath.Base(serviceRoot))
			if loadErr != nil {
				return loadErr
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
		extraVolumes, err = prepareRestoreVolumeFlags(ctx, serviceRoot, stagingDir, rusticTargetDir, item)
		if err != nil {
			return err
		}
		if err := runRusticRestore(ctx, rusticRoot, rustic, item.ArtifactRef, rusticTargetDir, logUploader, extraVolumes); err != nil {
			return err
		}
		return nil
	case "database.pgimport":
		if err := runRusticRestore(ctx, rusticRoot, rustic, item.ArtifactRef, rusticTargetDir, logUploader, extraVolumes); err != nil {
			return err
		}
		return applyRestoreItem(ctx, serviceRoot, filepath.Join(stagingDir, item.Name), item, logUploader)
	default:
		return fmt.Errorf("restore strategy %q is not implemented yet", item.Strategy)
	}
}

func prepareRestoreVolumeFlags(ctx context.Context, serviceRoot, stagingDir, containerStagingDir string, item backupcfg.RestoreItem) ([]string, error) {
	if err := os.MkdirAll(filepath.Join(stagingDir, item.Name, "paths"), 0o750); err != nil {
		return nil, fmt.Errorf("create direct restore paths dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(stagingDir, item.Name, "volumes"), 0o750); err != nil {
		return nil, fmt.Errorf("create direct restore volumes dir: %w", err)
	}

	var flags []string
	for _, include := range item.Include {
		kind, normalized, err := repo.ClassifyDataInclude(include)
		if err != nil {
			return nil, err
		}
		if kind == repo.DataIncludeKindServicePath {
			targetPath, err := resolveIncludeServicePath(serviceRoot, normalized)
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(targetPath)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("restore target %q must exist for direct restore", targetPath)
				}
				return nil, fmt.Errorf("stat restore target %q: %w", targetPath, err)
			}
			if err := os.RemoveAll(targetPath); err != nil {
				return nil, fmt.Errorf("clear restore target %q: %w", targetPath, err)
			}
			if info.IsDir() {
				if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
					return nil, fmt.Errorf("recreate restore target %q: %w", targetPath, err)
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
					return nil, fmt.Errorf("create restore target parent %q: %w", filepath.Dir(targetPath), err)
				}
				file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm()) //nolint:gosec
				if err != nil {
					return nil, fmt.Errorf("recreate restore target file %q: %w", targetPath, err)
				}
				if err := file.Close(); err != nil {
					return nil, fmt.Errorf("close restore target file %q: %w", targetPath, err)
				}
			}
			containerPath := filepath.Join(containerStagingDir, item.Name, "paths", sanitizeStagePath(include))
			flags = append(flags, "-v", targetPath+":"+containerPath)
		} else {
			if err := clearDockerVolume(ctx, normalized); err != nil {
				return nil, err
			}
			containerPath := filepath.Join(containerStagingDir, item.Name, "volumes", sanitizeStagePath(normalized))
			flags = append(flags, "-v", normalized+":"+containerPath)
		}
	}
	return flags, nil
}

func applyRestoreItem(ctx context.Context, serviceRoot, stagingDir string, item backupcfg.RestoreItem, logUploader *taskLogUploader) error {
	switch item.Strategy {
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

func stageBackupItem(ctx context.Context, serviceRoot, stagingDir string, item backupcfg.RuntimeItem, logUploader *taskLogUploader) error {
	switch item.Strategy {
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

func buildBackupVolumeFlags(serviceRoot, containerStagingDir string, item backupcfg.RuntimeItem) ([]string, error) {
	var flags []string
	for _, include := range item.Include {
		kind, normalized, err := repo.ClassifyDataInclude(include)
		if err != nil {
			return nil, err
		}
		if kind == repo.DataIncludeKindServicePath {
			sourcePath, err := resolveIncludeServicePath(serviceRoot, normalized)
			if err != nil {
				return nil, err
			}
			if _, err := os.Stat(sourcePath); err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("backup source %q does not exist", sourcePath)
				}
				return nil, fmt.Errorf("stat backup source %q: %w", sourcePath, err)
			}
			targetPath := filepath.Join(containerStagingDir, "paths", sanitizeStagePath(include))
			flags = append(flags, "-v", sourcePath+":"+targetPath+":ro")
		} else {
			targetPath := filepath.Join(containerStagingDir, "volumes", sanitizeStagePath(normalized))
			flags = append(flags, "-v", normalized+":"+targetPath+":ro")
		}
	}
	return flags, nil
}

func sanitizeStagePath(value string) string {
	replacer := strings.NewReplacer("/", "_", `\\`, "_", ":", "_")
	return replacer.Replace(strings.TrimPrefix(strings.TrimPrefix(value, "./"), "/"))
}
