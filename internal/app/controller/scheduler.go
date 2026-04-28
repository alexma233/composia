package controller

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/schedule"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

const schedulerTickInterval = 15 * time.Second

func runScheduledTasks(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, taskQueue *taskQueueNotifier, repoMu *sync.Mutex) {
	if db == nil || cfg == nil || repoMu == nil {
		return
	}
	ticker := time.NewTicker(schedulerTickInterval)
	defer ticker.Stop()

	runPass := func() {
		repoMu.Lock()
		defer repoMu.Unlock()
		if err := runScheduledTasksPass(ctx, db, cfg, availableNodeIDs, taskQueue, time.Now().UTC()); err != nil {
			log.Printf("scheduler pass failed: %v", err)
		}
	}

	runPass()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runPass()
		}
	}
}

func runScheduledTasksPass(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, taskQueue *taskQueueNotifier, now time.Time) error {
	services, err := repo.DiscoverServices(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return err
	}
	windowStart := schedule.WindowStart(now)
	serviceServer := &serviceCommandServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue}
	for _, service := range services {
		if err := scheduleServiceBackups(ctx, db, serviceServer, service, windowStart); err != nil {
			log.Printf("scheduler backup scan failed for service=%s: %v", service.Name, err)
		}
	}
	if err := scheduleRusticMaintenance(ctx, db, cfg, availableNodeIDs, taskQueue, windowStart); err != nil {
		log.Printf("scheduler rustic maintenance scan failed: %v", err)
	}
	return nil
}

func scheduleServiceBackups(ctx context.Context, db *store.DB, serviceServer *serviceCommandServer, service repo.Service, now time.Time) error {
	if service.Meta.Backup == nil {
		return nil
	}
	serviceDir, err := filepath.Rel(serviceServer.cfg.RepoDir, service.Directory)
	if err != nil {
		return err
	}
	dueDataNamesByNode := make(map[string][]string, len(service.TargetNodes))
	for _, item := range service.Meta.Backup.Data {
		if item.Name == "" {
			continue
		}
		if item.Enabled != nil && !*item.Enabled {
			continue
		}
		spec := effectiveBackupSchedule(serviceServer.cfg, item.Schedule)
		if spec == "" {
			continue
		}
		parsed, err := schedule.Parse(spec)
		if err != nil {
			return err
		}
		if !schedule.DueNow(parsed, now) {
			continue
		}
		for _, nodeID := range service.TargetNodes {
			dueDataNamesByNode[nodeID] = append(dueDataNamesByNode[nodeID], item.Name)
		}
	}
	for nodeID, dataNames := range dueDataNamesByNode {
		slices.Sort(dataNames)
		dataNames = slices.Compact(dataNames)
		paramsJSONBytes, err := json.Marshal(serviceTaskParams{ServiceDir: serviceDir, DataNames: dataNames})
		if err != nil {
			return err
		}
		paramsJSON := string(paramsJSONBytes)
		exists, err := db.HasMatchingTaskInWindow(ctx, task.SourceSchedule, task.TypeBackup, service.Name, nodeID, paramsJSON, now)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		createdAt := now
		if _, err := serviceServer.createServiceTaskWithOptions(ctx, service.Name, []string{nodeID}, task.TypeBackup, dataNames, serviceTaskCreateOptions{Source: task.SourceSchedule, CreatedAt: &createdAt}); err != nil {
			log.Printf("scheduler skipped backup for service=%s data=%v node=%s: %v", service.Name, dataNames, nodeID, err)
		}
	}
	return nil
}

func effectiveBackupSchedule(cfg *config.ControllerConfig, itemSchedule string) string {
	itemSchedule = schedule.Normalize(itemSchedule)
	if schedule.IsDisabled(itemSchedule) {
		return ""
	}
	if itemSchedule != "" {
		return itemSchedule
	}
	if cfg == nil || cfg.Backup == nil {
		return ""
	}
	defaultSchedule := schedule.Normalize(cfg.Backup.DefaultSchedule)
	if defaultSchedule == "" || schedule.IsDisabled(defaultSchedule) {
		return ""
	}
	return defaultSchedule
}

func scheduleRusticMaintenance(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, taskQueue *taskQueueNotifier, now time.Time) error {
	if cfg.Rustic == nil || cfg.Rustic.Maintenance == nil {
		return nil
	}
	if err := scheduleRusticMaintenanceTask(ctx, db, cfg, availableNodeIDs, taskQueue, now, task.TypeRusticForget, schedule.Normalize(cfg.Rustic.Maintenance.ForgetSchedule)); err != nil {
		return err
	}
	if err := scheduleRusticMaintenanceTask(ctx, db, cfg, availableNodeIDs, taskQueue, now, task.TypeRusticPrune, schedule.Normalize(cfg.Rustic.Maintenance.PruneSchedule)); err != nil {
		return err
	}
	return nil
}

func scheduleRusticMaintenanceTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, taskQueue *taskQueueNotifier, now time.Time, taskType task.Type, spec string) error {
	if spec == "" || schedule.IsDisabled(spec) {
		return nil
	}
	parsed, err := schedule.Parse(spec)
	if err != nil {
		return err
	}
	if !schedule.DueNow(parsed, now) {
		return nil
	}
	rusticService, err := repo.FindRusticInfraService(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return err
	}
	serviceDir, err := filepath.Rel(cfg.RepoDir, rusticService.Directory)
	if err != nil {
		return err
	}
	nodeID, err := chooseScheduledRusticMainNode(ctx, db, cfg, rusticService, taskType)
	if err != nil {
		return err
	}
	paramsJSONBytes, err := json.Marshal(rusticMaintenanceTaskParams{ServiceDir: serviceDir, RepoWide: true})
	if err != nil {
		return err
	}
	paramsJSON := string(paramsJSONBytes)
	exists, err := db.HasMatchingTaskInWindow(ctx, task.SourceSchedule, taskType, rusticService.Name, nodeID, paramsJSON, now)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	createdAt := now
	if _, err := createNodeRusticMaintenanceTask(ctx, db, cfg, availableNodeIDs, nodeID, taskType, rusticMaintenanceTaskParams{ServiceDir: serviceDir, RepoWide: true}, task.SourceSchedule, &createdAt); err != nil {
		return err
	}
	notifyTaskQueue(taskQueue)
	return nil
}

func chooseScheduledRusticMainNode(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, rusticService repo.Service, taskType task.Type) (string, error) {
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
		return "", errNoEligibleRusticNode(taskType)
	}
	slices.Sort(candidates)
	for _, nodeID := range candidates {
		if err := validateTaskTargetNode(ctx, db, cfg, nodeID, taskType); err == nil {
			return nodeID, nil
		}
	}
	return "", errNoEligibleRusticNode(taskType)
}

func errNoEligibleRusticNode(taskType task.Type) error {
	return &scheduledRusticNodeError{taskType: taskType}
}

type scheduledRusticNodeError struct {
	taskType task.Type
}

func (err *scheduledRusticNodeError) Error() string {
	return "no eligible online rustic main node is available for " + string(err.taskType)
}
