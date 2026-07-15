package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	backupcfg "forgejo.alexma.top/alexma233/composia/internal/core/backup"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	secretutil "forgejo.alexma.top/alexma233/composia/internal/platform/secret"
)

func bundleExtraFiles(cfg *config.ControllerConfig, record task.Record, params serviceTaskParams, includeTaskRuntime bool) (map[string]string, error) {
	extraFiles := map[string]string{}
	if params.ServiceDir == "" {
		return extraFiles, nil
	}
	if cfg.Secrets != nil {
		encFiles, err := listEncryptedFiles(cfg.RepoDir, record.RepoRevision, params.ServiceDir)
		if err != nil {
			return nil, err
		}
		for _, encFile := range encFiles {
			fullPath := filepath.ToSlash(filepath.Join(params.ServiceDir, encFile))
			decrypted, err := decryptFileAtRevision(cfg, record.RepoRevision, fullPath)
			if err != nil {
				return nil, err
			}
			decryptedPath := strings.TrimSuffix(encFile, ".enc")
			extraFiles[filepath.ToSlash(filepath.Join(params.ServiceDir, decryptedPath))] = decrypted
		}
	}
	if includeTaskRuntime && record.Type == task.TypeBackup {
		payload, err := buildBackupRuntimePayload(cfg, record.ServiceName, record.NodeID, record.RepoRevision, params)
		if err != nil {
			return nil, err
		}
		if payload != "" {
			extraFiles[filepath.ToSlash(filepath.Join(params.ServiceDir, ".composia-backup.json"))] = payload
		}
	}
	if includeTaskRuntime && record.Type == task.TypeRestore {
		payload, err := buildRestoreRuntimePayload(cfg, record.ServiceName, record.NodeID, record.RepoRevision, params)
		if err != nil {
			return nil, err
		}
		if payload != "" {
			extraFiles[filepath.ToSlash(filepath.Join(params.ServiceDir, ".composia-restore.json"))] = payload
		}
	}
	if len(extraFiles) == 0 {
		return map[string]string{}, nil
	}
	return extraFiles, nil
}

func listEncryptedFiles(repoDir, revision, serviceDir string) ([]string, error) {
	files, err := repo.ListFilesAtRevision(repoDir, revision, serviceDir)
	if err != nil {
		if isMissingRevisionPathError(err) {
			return nil, nil
		}
		return nil, err
	}
	var encFiles []string
	for _, f := range files {
		if strings.HasSuffix(f, ".enc") {
			encFiles = append(encFiles, f)
		}
	}
	return encFiles, nil
}

func decryptFileAtRevision(cfg *config.ControllerConfig, revision, encFilePath string) (string, error) {
	secretContent, err := repo.ReadFileAtRevision(cfg.RepoDir, revision, encFilePath)
	if err != nil {
		if isMissingRevisionPathError(err) {
			return "", nil
		}
		return "", err
	}
	plaintext, err := secretutil.Decrypt([]byte(secretContent), cfg.Secrets)
	if err != nil {
		return "", err
	}
	return plaintext, nil
}

func isMissingRevisionPathError(err error) bool {
	message := err.Error()
	return strings.Contains(message, "does not exist") || strings.Contains(message, "exists on disk, but not in") || strings.Contains(message, "pathspec") || strings.Contains(message, "invalid object name") || strings.Contains(message, "not found")
}

func buildBackupRuntimePayload(cfg *config.ControllerConfig, serviceName, nodeID, revision string, params serviceTaskParams) (string, error) {
	if params.ServiceDir == "" {
		return "", errors.New("backup runtime payload requires service_dir")
	}
	service, err := repo.FindServiceAtRevision(cfg.RepoDir, revision, params.ServiceDir, configuredNodeIDs(cfg))
	if err != nil {
		return "", err
	}
	rusticService, err := repo.FindRusticInfraServiceAtRevision(cfg.RepoDir, revision, configuredNodeIDs(cfg))
	if err != nil {
		return "", err
	}
	if err := validateRusticServiceTargetNode(rusticService, nodeID); err != nil {
		return "", err
	}
	rusticServiceDir, err := filepath.Rel(cfg.RepoDir, rusticService.Directory)
	if err != nil {
		return "", fmt.Errorf("resolve rustic service directory: %w", err)
	}
	selected := make(map[string]struct{}, len(params.DataNames))
	for _, name := range params.DataNames {
		selected[name] = struct{}{}
	}
	items := make([]backupcfg.RuntimeItem, 0, len(params.DataNames))
	for _, data := range service.Meta.DataProtect.Data {
		if _, ok := selected[data.Name]; !ok || data.Backup == nil {
			continue
		}
		provider := backupProviderRustic
		for _, backupItem := range service.Meta.Backup.Data {
			if backupItem.Name == data.Name {
				if backupItem.Provider != "" {
					provider = backupItem.Provider
				}
				break
			}
		}
		if provider != backupProviderRustic {
			return "", fmt.Errorf("backup provider %q is not implemented", provider)
		}
		items = append(items, backupcfg.RuntimeItem{Name: data.Name, Strategy: data.Backup.Strategy, Service: data.Backup.Service, Include: append([]string(nil), data.Backup.Include...), Provider: provider, Tags: []string{"composia-service:" + serviceName, "composia-data:" + data.Name}})
	}
	payload, err := json.Marshal(backupcfg.RuntimeConfig{Rustic: &backupcfg.RusticConfig{ServiceName: rusticService.Name, ServiceDir: rusticServiceDir, ComposeService: rusticService.Meta.RusticComposeService(), Profile: rusticService.Meta.RusticProfile(), DataProtectDir: rusticService.Meta.RusticDataProtectDir(), NodeID: nodeID}, Items: items})
	if err != nil {
		return "", fmt.Errorf("marshal backup runtime config for %s at %s: %w", serviceName, revision, err)
	}
	return string(payload), nil
}

func buildRestoreRuntimePayload(cfg *config.ControllerConfig, serviceName, nodeID, revision string, params serviceTaskParams) (string, error) {
	if params.ServiceDir == "" {
		return "", errors.New("restore runtime payload requires service_dir")
	}
	service, err := repo.FindServiceAtRevision(cfg.RepoDir, revision, params.ServiceDir, configuredNodeIDs(cfg))
	if err != nil {
		return "", err
	}
	rusticService, err := repo.FindRusticInfraServiceAtRevision(cfg.RepoDir, revision, configuredNodeIDs(cfg))
	if err != nil {
		return "", err
	}
	if err := validateRusticServiceTargetNode(rusticService, nodeID); err != nil {
		return "", err
	}
	rusticServiceDir, err := filepath.Rel(cfg.RepoDir, rusticService.Directory)
	if err != nil {
		return "", fmt.Errorf("resolve rustic service directory: %w", err)
	}
	artifactsByName := make(map[string]string, len(params.RestoreItems))
	for _, item := range params.RestoreItems {
		if item.DataName == "" || item.ArtifactRef == "" {
			continue
		}
		artifactsByName[item.DataName] = item.ArtifactRef
	}
	items := make([]backupcfg.RestoreItem, 0, len(params.RestoreItems))
	for _, data := range service.Meta.DataProtect.Data {
		artifactRef, ok := artifactsByName[data.Name]
		if !ok || data.Restore == nil {
			continue
		}
		items = append(items, backupcfg.RestoreItem{
			Name:        data.Name,
			Strategy:    data.Restore.Strategy,
			Service:     data.Restore.Service,
			Include:     append([]string(nil), data.Restore.Include...),
			Provider:    backupProviderRustic,
			ArtifactRef: artifactRef,
		})
	}
	payload, err := json.Marshal(backupcfg.RestoreConfig{Rustic: &backupcfg.RusticConfig{ServiceName: rusticService.Name, ServiceDir: rusticServiceDir, ComposeService: rusticService.Meta.RusticComposeService(), Profile: rusticService.Meta.RusticProfile(), DataProtectDir: rusticService.Meta.RusticDataProtectDir(), NodeID: nodeID}, Items: items})
	if err != nil {
		return "", fmt.Errorf("marshal restore runtime config for %s at %s: %w", serviceName, revision, err)
	}
	return string(payload), nil
}
