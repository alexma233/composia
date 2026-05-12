package controller

import (
	"bytes"
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type serviceCommandServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
	taskResults      *taskResultNotifier
	repoMu           *sync.Mutex
}

func (server *serviceCommandServer) UpdateServiceTargetNodes(ctx context.Context, req *connect.Request[controllerv1.UpdateServiceTargetNodesRequest]) (*connect.Response[controllerv1.UpdateServiceTargetNodesResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	updatedContent, err := repo.RewriteServiceTargetNodes(service.MetaPath, req.Msg.GetNodeIds(), server.availableNodeIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	repoSrv := &repoCommandServer{db: server.db, cfg: server.cfg, availableNodeIDs: server.availableNodeIDs, repoMu: server.repoMu}
	commitMessage := req.Msg.GetCommitMessage()
	if commitMessage == "" {
		commitMessage = fmt.Sprintf("update target nodes for %s", service.Name)
	}
	relativeMetaPath, err := filepath.Rel(server.cfg.RepoDir, service.MetaPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service meta path: %w", err))
	}
	result, err := repoSrv.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{relativeMetaPath}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return repoSrv.updateRepoFileTransaction(ctx, relativeMetaPath, updatedContent, commitMessage, baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.UpdateServiceTargetNodesResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *serviceCommandServer) RunServiceAction(ctx context.Context, req *connect.Request[controllerv1.RunServiceActionRequest]) (*connect.Response[controllerv1.RunServiceActionResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}

	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	var (
		taskType  task.Type
		nodeIDs   []string
		dataNames []string
	)

	switch req.Msg.GetAction() {
	case controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY:
		taskType = task.TypeDeploy
		nodeIDs = req.Msg.GetNodeIds()
	case controllerv1.ServiceAction_SERVICE_ACTION_UPDATE:
		taskType = task.TypeUpdate
		nodeIDs = req.Msg.GetNodeIds()
	case controllerv1.ServiceAction_SERVICE_ACTION_STOP:
		taskType = task.TypeStop
		nodeIDs = req.Msg.GetNodeIds()
	case controllerv1.ServiceAction_SERVICE_ACTION_RESTART:
		if service.Meta.IsConfigInfra() {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q declares infra.config and cannot be restarted", service.Name))
		}
		taskType = task.TypeRestart
		nodeIDs = req.Msg.GetNodeIds()
	case controllerv1.ServiceAction_SERVICE_ACTION_BACKUP:
		taskType = task.TypeBackup
		nodeIDs = req.Msg.GetNodeIds()
		dataNames, err = repo.ValidateRequestedBackupDataNames(service, req.Msg.GetDataNames())
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	case controllerv1.ServiceAction_SERVICE_ACTION_DNS_UPDATE:
		taskType = task.TypeDNSUpdate
		if service.Meta.Network == nil || service.Meta.Network.DNS == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not declare network.dns", service.Name))
		}
		if server.cfg.DNS == nil || server.cfg.DNS.Cloudflare == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller dns.cloudflare is not configured"))
		}
	case controllerv1.ServiceAction_SERVICE_ACTION_CADDY_SYNC:
		taskType = task.TypeCaddySync
		nodeIDs = req.Msg.GetNodeIds()
		if !repo.CaddyManaged(service) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not declare network.caddy", service.Name))
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("action is required"))
	}

	if taskType == task.TypeUpdate && (len(req.Msg.GetImageUpdates()) > 0 || req.Msg.GetUseAllDetectedImageUpdates()) {
		createdTasks, repoWrite, err := server.runServiceUpdateWithImageSelections(ctx, service, nodeIDs, req.Msg.GetImageUpdates(), req.Msg.GetUseAllDetectedImageUpdates(), req.Msg.BackupBeforeUpdate, req.Msg.GetBaseRevision(), req.Msg.GetCommitMessage(), requestTaskSource(req.Header()), composeRecreateModeParam(taskType, req.Msg.GetComposeRecreateMode()))
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(runServiceActionResponse(createdTasks, repoWrite)), nil
	}

	if taskType == task.TypeUpdate && effectiveBackupBeforeUpdate(server.cfg, service.Meta.Update, nil, req.Msg.BackupBeforeUpdate) {
		targetNodeIDs, err := resolveTargetNodeIDs(service, nodeIDs)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		if len(targetNodeIDs) == 0 {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not have any target nodes", service.Name))
		}
		if err := server.runBackupsBeforeUpdate(ctx, service, targetNodeIDs, requestTaskSource(req.Header())); err != nil {
			return nil, err
		}
	}

	createdTasks, err := server.createServiceTasksWithOptions(ctx, req.Msg.GetServiceName(), nodeIDs, taskType, dataNames, serviceTaskCreateOptions{Source: requestTaskSource(req.Header()), ComposeRecreateMode: composeRecreateModeParam(taskType, req.Msg.GetComposeRecreateMode())})
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(runServiceActionResponse(createdTasks, nil)), nil
}

type serviceTaskCreateOptions struct {
	AttemptOfTaskID       string
	Source                task.Source
	CreatedAt             *time.Time
	ComposeRecreateMode   string
	ImageNames            []string
	SemverAllow           []string
	ForgeCandidates       map[string][]string
	ForgeCandidateSources map[string]map[string][]string
}

func composeRecreateModeParam(taskType task.Type, mode controllerv1.ComposeRecreateMode) string {
	if taskType != task.TypeDeploy && taskType != task.TypeUpdate {
		return ""
	}
	switch mode {
	case controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_NO_RECREATE:
		return "no_recreate"
	case controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_FORCE_RECREATE:
		return "force_recreate"
	case controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_AUTO, controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_UNSPECIFIED:
		return "auto"
	default:
		return "auto"
	}
}

func imageUpdateSelectionNames(selections []*controllerv1.ImageUpdateSelection) []string {
	if len(selections) == 0 {
		return nil
	}
	names := make([]string, 0, len(selections))
	seen := make(map[string]struct{}, len(selections))
	for _, selection := range selections {
		name := strings.TrimSpace(selection.GetImageName())
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

type plannedImageUpdate struct {
	ImageName  string
	Tag        string
	Digest     string
	RepoBacked bool
}

func (server *serviceCommandServer) runServiceUpdateWithImageSelections(ctx context.Context, service repo.Service, nodeIDs []string, selections []*controllerv1.ImageUpdateSelection, useAllDetected bool, backupBeforeUpdateOverride *bool, baseRevision, commitMessage string, source task.Source, composeRecreateMode string) ([]task.Record, *repoWriteResult, error) {
	targetNodeIDs, err := resolveTargetNodeIDs(service, nodeIDs)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if len(targetNodeIDs) == 0 {
		return nil, nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not have any target nodes", service.Name))
	}
	planned, err := server.planRequestedServiceImageUpdates(ctx, service, targetNodeIDs, selections, useAllDetected)
	if err != nil {
		return nil, nil, err
	}
	if serviceImageUpdatesNeedBackup(server.cfg, service.Meta.Update, service.Meta.Update.Images, planned, selections, backupBeforeUpdateOverride) {
		if err := server.runBackupsBeforeUpdate(ctx, service, targetNodeIDs, source); err != nil {
			return nil, nil, err
		}
	}
	repoBackedPlanned := repoBackedPlannedImageUpdates(planned)
	var repoWrite *repoWriteResult
	if len(repoBackedPlanned) > 0 {
		if strings.TrimSpace(baseRevision) == "" {
			return nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required for repo-backed image updates"))
		}
		if strings.TrimSpace(commitMessage) == "" {
			return nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("commit_message is required for repo-backed image updates"))
		}
		result, err := server.applyPlannedServiceImageUpdates(ctx, service, repoBackedPlanned, baseRevision, commitMessage)
		if err != nil {
			return nil, nil, err
		}
		repoWrite = &result
	}
	createdTasks, err := server.createServiceTasksWithOptions(ctx, service.Name, targetNodeIDs, task.TypeUpdate, nil, serviceTaskCreateOptions{Source: source, ComposeRecreateMode: composeRecreateMode, ImageNames: imageUpdateSelectionNames(selections)})
	if err != nil {
		return nil, nil, err
	}
	return createdTasks, repoWrite, nil
}

func (server *serviceCommandServer) planRequestedServiceImageUpdates(ctx context.Context, service repo.Service, targetNodeIDs []string, selections []*controllerv1.ImageUpdateSelection, useAllDetected bool) ([]plannedImageUpdate, error) {
	if service.Meta.Update == nil || len(service.Meta.Update.Images) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not declare update.images", service.Name))
	}
	if len(targetNodeIDs) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not have any target nodes", service.Name))
	}
	checks, err := server.db.LatestServiceImageUpdateChecks(ctx, service.Name, targetNodeIDs[0])
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	checksByImage := make(map[string]store.ServiceImageUpdateCheck, len(checks))
	for _, check := range checks {
		checksByImage[check.ImageName] = check
	}
	planned := make([]plannedImageUpdate, 0)
	explicitNames := make(map[string]struct{}, len(selections))
	for _, selection := range selections {
		if imageName := strings.TrimSpace(selection.GetImageName()); imageName != "" {
			explicitNames[imageName] = struct{}{}
		}
	}
	if useAllDetected {
		for imageName, image := range service.Meta.Update.Images {
			if _, explicit := explicitNames[imageName]; explicit {
				continue
			}
			check, ok := checksByImage[imageName]
			if !ok || !check.UpdateAvailable {
				continue
			}
			if repo.IsDigestImageDiscovery(image.Discovery, service.Meta.Update.DiscoverySources) {
				planned = append(planned, plannedImageUpdate{ImageName: imageName})
				continue
			}
			if check.CandidateTag == "" {
				return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("detected image update %q is missing candidate_tag", imageName))
			}
			planned = append(planned, plannedImageUpdate{ImageName: imageName, Tag: check.CandidateTag, Digest: check.CandidateDigest, RepoBacked: true})
		}
	}
	seen := make(map[string]struct{}, len(selections))
	for _, selection := range selections {
		imageName := strings.TrimSpace(selection.GetImageName())
		if imageName == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("image update image_name is required"))
		}
		if _, exists := seen[imageName]; exists {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("image update %q is duplicated", imageName))
		}
		seen[imageName] = struct{}{}
		image, ok := service.Meta.Update.Images[imageName]
		if !ok {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not declare update.images[%q]", service.Name, imageName))
		}
		if repo.IsDigestImageDiscovery(image.Discovery, service.Meta.Update.DiscoverySources) {
			if selection.GetTargetTag() != "" || selection.GetUseDetected() {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("mutable image update %q does not accept target_tag or use_detected", imageName))
			}
			planned = append(planned, plannedImageUpdate{ImageName: imageName})
			continue
		}
		tag := strings.TrimSpace(selection.GetTargetTag())
		digest := ""
		if selection.GetUseDetected() {
			check, ok := checksByImage[imageName]
			if !ok || !check.UpdateAvailable || check.CandidateTag == "" {
				return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("image update %q does not have a detected candidate", imageName))
			}
			tag = check.CandidateTag
			digest = check.CandidateDigest
		}
		if tag == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pinned image update %q requires target_tag or use_detected", imageName))
		}
		planned = append(planned, plannedImageUpdate{ImageName: imageName, Tag: tag, Digest: digest, RepoBacked: true})
	}
	if useAllDetected && len(selections) == 0 && len(planned) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not have any detected image updates", service.Name))
	}
	return planned, nil
}

func repoBackedPlannedImageUpdates(planned []plannedImageUpdate) []plannedImageUpdate {
	repoBacked := make([]plannedImageUpdate, 0, len(planned))
	for _, update := range planned {
		if update.RepoBacked {
			repoBacked = append(repoBacked, update)
		}
	}
	return repoBacked
}

func (server *serviceCommandServer) applyPlannedServiceImageUpdates(ctx context.Context, service repo.Service, planned []plannedImageUpdate, baseRevision, commitMessage string) (repoWriteResult, error) {
	serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
	if err != nil {
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	paths := make([]string, 0, len(planned))
	seenPaths := make(map[string]struct{}, len(planned))
	for _, update := range planned {
		image := service.Meta.Update.Images[update.ImageName]
		if repo.ImageUpdateCurrentFile(image.Current) == "" {
			return repoWriteResult{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("image update %q is not repo-backed", update.ImageName))
		}
		path := filepath.ToSlash(filepath.Join(serviceDir, repo.ImageUpdateCurrentFile(image.Current)))
		if _, exists := seenPaths[path]; !exists {
			seenPaths[path] = struct{}{}
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	repoSrv := &repoCommandServer{db: server.db, cfg: server.cfg, availableNodeIDs: server.availableNodeIDs, repoMu: server.repoMu}
	result, err := repoSrv.runRepoWrite(ctx, baseRevision, paths, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		contents := make(map[string]string, len(paths))
		for _, path := range paths {
			file, err := repo.ReadFile(server.cfg.RepoDir, path)
			if err != nil {
				return repoWriteResult{}, mapRepoMutationError(err)
			}
			contents[path] = file.Content
		}
		for _, update := range planned {
			image := service.Meta.Update.Images[update.ImageName]
			targetDigest := update.Digest
			if effectiveDigestPin(server.cfg, service.Meta.Update, image) && targetDigest == "" {
				var err error
				targetDigest, err = inspectControllerRemoteImageDigest(ctx, image.Image+":"+update.Tag)
				if err != nil {
					return repoWriteResult{}, connect.NewError(connect.CodeFailedPrecondition, err)
				}
			}
			targetValue := update.Tag
			if effectiveDigestPin(server.cfg, service.Meta.Update, image) && targetDigest != "" {
				targetValue = update.Tag + "@" + targetDigest
			}
			path := filepath.ToSlash(filepath.Join(serviceDir, repo.ImageUpdateCurrentFile(image.Current)))
			updatedContent, err := applyImageCurrentUpdate(contents[path], image.Current, image.Image, targetValue)
			if err != nil {
				return repoWriteResult{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("update image %q: %w", update.ImageName, err))
			}
			contents[path] = updatedContent
		}
		writtenSnapshots := make(map[string]string, len(contents))
		committed := false
		for _, path := range paths {
			previous := contents[path]
			current, err := repo.ReadFile(server.cfg.RepoDir, path)
			if err != nil {
				return repoWriteResult{}, mapRepoMutationError(err)
			}
			writtenSnapshots[path] = current.Content
			if _, err := repo.WriteFile(server.cfg.RepoDir, path, previous); err != nil {
				return repoWriteResult{}, mapRepoMutationError(err)
			}
		}
		defer func() {
			if committed {
				return
			}
			for path, content := range writtenSnapshots {
				_, _ = repo.WriteFile(server.cfg.RepoDir, path, content)
			}
		}()
		commitID, err := repoSrv.commitRepoPaths(paths[0], paths, strings.TrimSpace(commitMessage))
		if err != nil {
			return repoWriteResult{}, err
		}
		committed = true
		return repoSrv.finalizeRepoWrite(ctx, commitID, baseSyncState)
	})
	if err != nil {
		return repoWriteResult{}, err
	}
	return result, nil
}

func effectiveDigestPin(cfg *config.ControllerConfig, update *repo.UpdateConfig, image repo.ImageUpdateConfig) bool {
	var sources map[string]repo.ImageUpdateDiscovery
	if update != nil {
		sources = update.DiscoverySources
	}
	if repo.IsDigestImageDiscovery(image.Discovery, sources) {
		return false
	}
	if image.DigestPin != nil {
		return *image.DigestPin
	}
	if update != nil && update.DigestPin != nil {
		return *update.DigestPin
	}
	if cfg != nil && cfg.Updates != nil && cfg.Updates.DigestPin != nil {
		return *cfg.Updates.DigestPin
	}
	return true
}

func effectiveImageAutoApply(cfg *config.ControllerConfig, update *repo.UpdateConfig, image repo.ImageUpdateConfig) bool {
	if image.AutoApply != nil {
		return *image.AutoApply
	}
	if update != nil && update.AutoApply != nil {
		return *update.AutoApply
	}
	if cfg != nil && cfg.Updates != nil && cfg.Updates.AutoApply != nil {
		return *cfg.Updates.AutoApply
	}
	return false
}

func effectiveBackupBeforeUpdate(cfg *config.ControllerConfig, update *repo.UpdateConfig, image *repo.ImageUpdateConfig, requestOverride *bool) bool {
	if requestOverride != nil {
		return *requestOverride
	}
	if image != nil && image.BackupBeforeUpdate != nil {
		return *image.BackupBeforeUpdate
	}
	if update != nil && update.BackupBeforeUpdate != nil {
		return *update.BackupBeforeUpdate
	}
	if cfg != nil && cfg.Updates != nil && cfg.Updates.BackupBeforeUpdate != nil {
		return *cfg.Updates.BackupBeforeUpdate
	}
	return false
}

func serviceImageUpdatesNeedBackup(cfg *config.ControllerConfig, update *repo.UpdateConfig, images map[string]repo.ImageUpdateConfig, planned []plannedImageUpdate, selections []*controllerv1.ImageUpdateSelection, requestOverride *bool) bool {
	seen := make(map[string]struct{}, len(planned)+len(selections))
	for _, plan := range planned {
		seen[plan.ImageName] = struct{}{}
	}
	for _, selection := range selections {
		if name := strings.TrimSpace(selection.GetImageName()); name != "" {
			seen[name] = struct{}{}
		}
	}
	for imageName := range seen {
		image, ok := images[imageName]
		if ok && effectiveBackupBeforeUpdate(cfg, update, &image, requestOverride) {
			return true
		}
	}
	return false
}

func resolveUpdateBackupDataNames(service repo.Service) ([]string, error) {
	requested := repo.EnabledUpdateBackupDataNames(service)
	if service.Meta.Update != nil && len(service.Meta.Update.BackupData) > 0 {
		if len(requested) == 0 {
			return nil, fmt.Errorf("service %q does not have any enabled update backup data items", service.Name)
		}
	}
	return repo.ValidateRequestedBackupDataNames(service, requested)
}

func (server *serviceCommandServer) runBackupsBeforeUpdate(ctx context.Context, service repo.Service, targetNodeIDs []string, source task.Source) error {
	dataNames, err := resolveUpdateBackupDataNames(service)
	if err != nil {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	backupTasks := make([]task.Record, 0, len(targetNodeIDs))
	for _, nodeID := range targetNodeIDs {
		backupTask, err := server.createServiceTaskWithOptions(ctx, service.Name, []string{nodeID}, task.TypeBackup, dataNames, serviceTaskCreateOptions{Source: source})
		if err != nil {
			return err
		}
		backupTasks = append(backupTasks, backupTask)
	}
	for _, backupTask := range backupTasks {
		if err := server.waitTask(ctx, backupTask.TaskID, 10*time.Minute); err != nil {
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	return nil
}

func (server *serviceCommandServer) waitTask(ctx context.Context, taskID string, timeout time.Duration) error {
	return waitTask(ctx, server.db, server.taskResults, taskID, timeout, 5*time.Second)
}

func applyImageCurrentUpdate(content string, current repo.ImageUpdateCurrent, imageRef, targetValue string) (string, error) {
	if current.Env != nil {
		return replaceEnvFileValue(content, current.Env.Key, targetValue)
	}
	if current.YAML != nil {
		return replaceYAMLPathImageValue(content, current.YAML.Path, imageRef, targetValue)
	}
	return "", fmt.Errorf("current must specify env or yaml")
}

func replaceEnvFileValue(content, key, targetValue string) (string, error) {
	lines := strings.SplitAfter(content, "\n")
	found := false
	for index, line := range lines {
		lineBody := strings.TrimSuffix(line, "\n")
		trimmed := strings.TrimSpace(lineBody)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, _, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(name) == key {
			newline := ""
			if strings.HasSuffix(line, "\n") {
				newline = "\n"
			}
			lines[index] = key + "=" + targetValue + newline
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("source key %q not found", key)
	}
	return strings.Join(lines, ""), nil
}

func replaceYAMLPathImageValue(content, sourcePath, imageRef, targetValue string) (string, error) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(content), &node); err != nil {
		return "", fmt.Errorf("decode yaml source: %w", err)
	}
	target, err := yamlPathNode(&node, sourcePath)
	if err != nil {
		return "", err
	}
	if target.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("source path %q is not scalar", sourcePath)
	}
	target.Value = imageRef + ":" + targetValue
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(&node); err != nil {
		_ = encoder.Close()
		return "", fmt.Errorf("encode yaml source: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("close yaml encoder: %w", err)
	}
	return buffer.String(), nil
}

func yamlPathNode(root *yaml.Node, sourcePath string) (*yaml.Node, error) {
	current := root
	if current.Kind == yaml.DocumentNode && len(current.Content) > 0 {
		current = current.Content[0]
	}
	for _, part := range strings.Split(sourcePath, ".") {
		if current.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("source path %q is not a mapping", sourcePath)
		}
		var next *yaml.Node
		for index := 0; index+1 < len(current.Content); index += 2 {
			if current.Content[index].Value == part {
				next = current.Content[index+1]
				break
			}
		}
		if next == nil {
			return nil, fmt.Errorf("source path %q not found", sourcePath)
		}
		current = next
	}
	return current, nil
}

func inspectControllerRemoteImageDigest(ctx context.Context, imageRef string) (string, error) {
	command := exec.CommandContext(ctx, "docker", "buildx", "imagetools", "inspect", "--format", "{{.Digest}}", imageRef)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("docker buildx imagetools inspect failed for %q: %w %s", imageRef, err, strings.TrimSpace(stderr.String()))
	}
	digest := strings.TrimSpace(stdout.String())
	if digest == "" || digest == "<no value>" {
		return "", fmt.Errorf("docker buildx imagetools inspect did not return a digest for %q", imageRef)
	}
	return digest, nil
}

func (server *serviceCommandServer) createServiceTaskWithOptions(ctx context.Context, serviceName string, nodeIDs []string, taskType task.Type, dataNames []string, options serviceTaskCreateOptions) (task.Record, error) {
	createdTasks, err := server.createServiceTasksWithOptions(ctx, serviceName, nodeIDs, taskType, dataNames, options)
	if err != nil {
		return task.Record{}, err
	}
	return createdTasks[0], nil
}

func (server *serviceCommandServer) createServiceTasksWithOptions(ctx context.Context, serviceName string, nodeIDs []string, taskType task.Type, dataNames []string, options serviceTaskCreateOptions) ([]task.Record, error) {
	if serviceName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}

	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, serviceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	targetNodeIDs, err := resolveTargetNodeIDs(service, nodeIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if len(targetNodeIDs) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not have any target nodes", serviceName))
	}
	if taskType == task.TypeImageCheck && options.ForgeCandidates == nil {
		forgeCandidates, forgeCandidateSources, err := collectForgeImageCandidates(ctx, server.cfg, service, options.ImageNames)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		options.ForgeCandidates = forgeCandidates
		options.ForgeCandidateSources = forgeCandidateSources
	}
	for _, nodeID := range targetNodeIDs {
		if err := validateTaskTargetNode(ctx, server.db, server.cfg, nodeID, taskType); err != nil {
			return nil, err
		}
	}
	repoRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: serviceDir, DataNames: dataNames, ImageNames: options.ImageNames, SemverAllow: options.SemverAllow, ForgeCandidates: options.ForgeCandidates, ForgeCandidateSources: options.ForgeCandidateSources, ComposeRecreateMode: options.ComposeRecreateMode})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("encode task params: %w", err))
	}
	taskSource := options.Source
	if taskSource == "" {
		taskSource = task.SourceCLI
	}
	pendingTasks := make([]task.Record, 0, len(targetNodeIDs))
	for _, nodeID := range targetNodeIDs {
		taskID := uuid.NewString()
		pendingTasks = append(pendingTasks, task.Record{
			TaskID:          taskID,
			Type:            taskType,
			Source:          taskSource,
			TriggeredBy:     triggeredBy,
			ServiceName:     service.Name,
			NodeID:          nodeID,
			ParamsJSON:      string(paramsJSON),
			RepoRevision:    repoRevision,
			AttemptOfTaskID: options.AttemptOfTaskID,
			CreatedAt:       derefTime(options.CreatedAt),
			LogPath:         filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
		})
	}
	createdTasks, err := server.db.CreateTasksIfNoActiveServiceInstanceTasks(ctx, pendingTasks)
	if err != nil {
		return nil, connectTaskAdmissionError(err)
	}
	for _, createdTask := range createdTasks {
		if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
		}
	}
	if taskType == task.TypeDeploy || taskType == task.TypeUpdate {
		if err := server.db.ClearServicePendingDeploy(ctx, serviceName); err != nil {
			log.Printf("clear pending deploy for %q failed: %v", serviceName, err)
		}
	}
	notifyTaskQueue(server.taskQueue)
	return createdTasks, nil
}

func runServiceActionResponse(records []task.Record, repoWrite *repoWriteResult) *controllerv1.RunServiceActionResponse {
	response := &controllerv1.RunServiceActionResponse{Tasks: make([]*controllerv1.TaskActionResponse, 0, len(records))}
	for _, record := range records {
		response.Tasks = append(response.Tasks, taskActionResponse(record))
	}
	if repoWrite != nil {
		response.RepoWrite = &controllerv1.ServiceActionRepoWriteResult{
			CommitId:             repoWrite.CommitID,
			SyncStatus:           repoWrite.SyncStatus,
			PushError:            repoWrite.PushError,
			LastSuccessfulPullAt: repoWrite.LastSuccessfulPullAt,
		}
	}
	return response
}
