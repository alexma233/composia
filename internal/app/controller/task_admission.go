package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"time"

	"connectrpc.com/connect"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"github.com/google/uuid"
)

func createNodeCaddyReloadTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, nodeID string, source task.Source, dedupeKeys ...string) (task.Record, error) {
	if nodeID == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	if err := validateTaskTargetNode(ctx, db, cfg, nodeID, task.TypeCaddyReload); err != nil {
		return task.Record{}, err
	}
	service, err := repo.FindCaddyInfraService(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	serviceDir, err := filepath.Rel(cfg.RepoDir, service.Directory)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve caddy service directory: %w", err))
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: serviceDir})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode task params: %w", err))
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	dedupeKey := ""
	if len(dedupeKeys) > 0 {
		dedupeKey = dedupeKeys[0]
	}
	createdTask, err := db.CreateTaskIfNoActiveServiceInstanceTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        task.TypeCaddyReload,
		Source:      source,
		TriggeredBy: triggeredBy,
		ServiceName: service.Name,
		NodeID:      nodeID,
		Status:      task.StatusPending,
		ParamsJSON:  string(paramsJSON),
		LogPath:     filepath.Join(cfg.LogDir, "tasks", taskID+".log"),
		DedupeKey:   dedupeKey,
	})
	if err != nil {
		if dedupeKey != "" {
			if existing, lookupErr := db.GetTaskByDedupeKey(ctx, dedupeKey); lookupErr == nil {
				return existing, nil
			}
		}
		var duplicate store.DuplicateTaskError
		if errors.As(err, &duplicate) {
			return db.GetTaskByDedupeKey(ctx, duplicate.DedupeKey)
		}
		return task.Record{}, connectTaskAdmissionError(err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o600); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	return createdTask, nil
}

func createNodeCaddySyncTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, nodeID, serviceName string, fullRebuild bool, source task.Source) (task.Record, error) {
	if nodeID == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	if err := validateTaskTargetNode(ctx, db, cfg, nodeID, task.TypeCaddySync); err != nil {
		return task.Record{}, err
	}
	repoRevision, err := repo.CurrentRevision(cfg.RepoDir)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("resolve repo revision: %w", err))
	}
	var (
		serviceDirs []string
		matchedName string
		serviceDir  string
	)
	if fullRebuild {
		services, err := repo.DiscoverServices(cfg.RepoDir, availableNodeIDs)
		if err != nil {
			return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		for _, service := range services {
			if !repo.CaddyManaged(service) {
				continue
			}
			for _, targetNodeID := range service.TargetNodes {
				if targetNodeID != nodeID {
					continue
				}
				relativeDir, err := filepath.Rel(cfg.RepoDir, service.Directory)
				if err != nil {
					return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
				}
				serviceDirs = append(serviceDirs, relativeDir)
				break
			}
		}
		slices.Sort(serviceDirs)
		matchedName = ""
	} else {
		if serviceName == "" {
			return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required when full_rebuild is false"))
		}
		service, err := repo.FindService(cfg.RepoDir, availableNodeIDs, serviceName)
		if err != nil {
			return task.Record{}, connect.NewError(connect.CodeNotFound, err)
		}
		if !repo.CaddyManaged(service) {
			return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not declare network.caddy", service.Name))
		}
		if _, err := resolveTargetNodeIDs(service, []string{nodeID}); err != nil {
			return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		relativeDir, err := filepath.Rel(cfg.RepoDir, service.Directory)
		if err != nil {
			return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
		}
		serviceDirs = []string{relativeDir}
		serviceDir = relativeDir
		matchedName = service.Name
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: serviceDir, ServiceDirs: serviceDirs, FullRebuild: fullRebuild})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode task params: %w", err))
	}
	taskID := uuid.NewString()
	createdTask, err := db.CreateTask(ctx, task.Record{TaskID: taskID, Type: task.TypeCaddySync, Source: source, ServiceName: matchedName, NodeID: nodeID, Status: task.StatusPending, ParamsJSON: string(paramsJSON), RepoRevision: repoRevision, LogPath: filepath.Join(cfg.LogDir, "tasks", taskID+".log")})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o600); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	return createdTask, nil
}

func createServiceCloudflareTunnelSyncTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, serviceName, excludedServiceDir string, source task.Source, dedupeKeys ...string) (task.Record, error) {
	if serviceName == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	service, err := repo.FindService(cfg.RepoDir, availableNodeIDs, serviceName)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeNotFound, err)
	}
	if !repo.CloudflareTunnelManaged(service) {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not declare network.cloudflare_tunnel", service.Name))
	}
	if cfg.CloudflareTunnel == nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller cloudflare_tunnel is not configured"))
	}
	repoRevision, err := repo.CurrentRevision(cfg.RepoDir)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("resolve repo revision: %w", err))
	}
	serviceDir, err := filepath.Rel(cfg.RepoDir, service.Directory)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: serviceDir, ExcludedServiceDir: excludedServiceDir})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode task params: %w", err))
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	dedupeKey := ""
	if len(dedupeKeys) > 0 {
		dedupeKey = dedupeKeys[0]
	}
	createdTask, err := db.CreateTaskIfNoActiveServiceTask(ctx, task.Record{TaskID: taskID, Type: task.TypeCloudflareTunnelSync, Source: source, TriggeredBy: triggeredBy, ServiceName: service.Name, Status: task.StatusPending, ParamsJSON: string(paramsJSON), RepoRevision: repoRevision, LogPath: filepath.Join(cfg.LogDir, "tasks", taskID+".log"), DedupeKey: dedupeKey})
	if err != nil {
		if dedupeKey != "" {
			if existing, lookupErr := db.GetTaskByDedupeKey(ctx, dedupeKey); lookupErr == nil {
				return existing, nil
			}
		}
		var duplicate store.DuplicateTaskError
		if errors.As(err, &duplicate) {
			return db.GetTaskByDedupeKey(ctx, duplicate.DedupeKey)
		}
		return task.Record{}, connectTaskAdmissionError(err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o600); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	return createdTask, nil
}

func chooseRusticMainNode(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, taskType task.Type) (string, error) {
	rusticService, err := repo.FindRusticInfraService(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return "", err
	}
	candidates := append([]string(nil), rusticService.TargetNodes...)
	if cfg.Rustic != nil && len(cfg.Rustic.MainNodes) > 0 {
		allowed := make(map[string]struct{}, len(cfg.Rustic.MainNodes))
		for _, nodeID := range cfg.Rustic.MainNodes {
			allowed[nodeID] = struct{}{}
		}
		filtered := make([]string, 0, len(candidates))
		for _, nodeID := range candidates {
			if _, ok := allowed[nodeID]; ok {
				filtered = append(filtered, nodeID)
			}
		}
		candidates = filtered
	}
	if len(candidates) == 0 {
		return "", errors.New("rustic infra service does not have any eligible main nodes")
	}
	online := make([]string, 0, len(candidates))
	for _, nodeID := range candidates {
		if err := validateTaskTargetNode(ctx, db, cfg, nodeID, taskType); err == nil {
			online = append(online, nodeID)
		}
	}
	if len(online) == 0 {
		return "", errors.New("no eligible online rustic main node is available")
	}
	return online[rand.Intn(len(online))], nil
}

func createNodeRusticMaintenanceTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, nodeID string, taskType task.Type, params rusticMaintenanceTaskParams, source task.Source, createdAt *time.Time) (task.Record, error) {
	if err := validateTaskTargetNode(ctx, db, cfg, nodeID, taskType); err != nil {
		return task.Record{}, err
	}
	rusticService, err := repo.FindRusticInfraService(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if err := validateRusticServiceTargetNode(rusticService, nodeID); err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	serviceDir, err := filepath.Rel(cfg.RepoDir, rusticService.Directory)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve rustic service directory: %w", err))
	}
	params.ServiceDir = serviceDir
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode rustic maintenance task params: %w", err))
	}
	repoRevision, err := repo.CurrentRevision(cfg.RepoDir)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	createdTask, err := db.CreateTask(ctx, task.Record{
		TaskID:       taskID,
		Type:         taskType,
		Source:       source,
		TriggeredBy:  triggeredBy,
		ServiceName:  rusticService.Name,
		NodeID:       nodeID,
		Status:       task.StatusPending,
		ParamsJSON:   string(paramsJSON),
		RepoRevision: repoRevision,
		CreatedAt:    derefTime(createdAt),
		LogPath:      filepath.Join(cfg.LogDir, "tasks", taskID+".log"),
	})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o600); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	return createdTask, nil
}

func validateRusticServiceTargetNode(rusticService repo.Service, nodeID string) error {
	if !slices.Contains(rusticService.TargetNodes, nodeID) {
		return fmt.Errorf("rustic infra service is not declared on node %q", nodeID)
	}
	return nil
}

func connectTaskAdmissionError(err error) error {
	var activeServiceErr store.ActiveServiceTaskError
	if errors.As(err, &activeServiceErr) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	var activeServiceInstanceErr store.ActiveServiceInstanceTaskError
	if errors.As(err, &activeServiceInstanceErr) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

func createServiceTaskWithOptions(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, serviceName string, nodeIDs []string, taskType task.Type, dataNames []string, options serviceTaskCreateOptions) (task.Record, error) {
	return (&serviceCommandServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs}).createServiceTaskWithOptions(ctx, serviceName, nodeIDs, taskType, dataNames, options)
}

func resolveTargetNodeIDs(service repo.Service, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return append([]string(nil), service.TargetNodes...), nil
	}
	allowed := make(map[string]struct{}, len(service.TargetNodes))
	for _, nodeID := range service.TargetNodes {
		allowed[nodeID] = struct{}{}
	}
	resolved := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, nodeID := range requested {
		if _, ok := allowed[nodeID]; !ok {
			return nil, fmt.Errorf("service %q is not declared on node %q", service.Name, nodeID)
		}
		if _, exists := seen[nodeID]; exists {
			continue
		}
		seen[nodeID] = struct{}{}
		resolved = append(resolved, nodeID)
	}
	return resolved, nil
}

func validateTaskTargetNode(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, nodeID string, taskType task.Type) error {
	var configuredNode *config.NodeConfig
	for index := range cfg.Nodes {
		if cfg.Nodes[index].ID == nodeID {
			configuredNode = &cfg.Nodes[index]
			break
		}
	}
	if configuredNode == nil {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q is not configured", nodeID))
	}
	if configuredNode.Enabled != nil && !*configuredNode.Enabled {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q is disabled", nodeID))
	}
	if !task.RequiresOnlineNode(taskType) {
		return nil
	}
	snapshot, err := db.GetNodeSnapshot(ctx, nodeID)
	if err != nil {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if !snapshot.IsOnline {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q is offline", nodeID))
	}
	return nil
}
