package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appnotify "forgejo.alexma.top/alexma233/composia/internal/app/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func runControllerRuntime(ctx context.Context, cfg *config.ControllerConfig, reload func(context.Context) error) error {
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return fmt.Errorf("create controller state_dir %q: %w", cfg.StateDir, err)
	}
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return fmt.Errorf("create controller log_dir %q: %w", cfg.LogDir, err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.LogDir, "tasks"), 0o755); err != nil {
		return fmt.Errorf("create controller task log_dir %q: %w", filepath.Join(cfg.LogDir, "tasks"), err)
	}
	if err := repo.ValidateWorkingTree(cfg.RepoDir); err != nil {
		return err
	}
	db, err := store.Open(cfg.StateDir)
	if err != nil {
		return err
	}
	runtimeCtx, cancelRuntime := context.WithCancel(ctx)
	var background sync.WaitGroup
	defer func() {
		cancelRuntime()
		background.Wait()
		_ = db.Close()
	}()
	startBackground := func(run func()) {
		background.Add(1)
		go func() {
			defer background.Done()
			run()
		}()
	}

	nodeIDs := make([]string, 0, len(cfg.Nodes))
	availableNodeIDs := make(map[string]struct{}, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		nodeIDs = append(nodeIDs, node.ID)
		availableNodeIDs[node.ID] = struct{}{}
	}
	if err := db.SyncConfiguredNodes(runtimeCtx, nodeIDs); err != nil {
		return err
	}
	if err := db.MarkOfflineNodesBefore(runtimeCtx, time.Now().Add(-heartbeatOfflineAfter)); err != nil {
		return err
	}
	if _, err := db.RecoverRunningTasks(runtimeCtx, time.Now().UTC()); err != nil {
		return err
	}
	repoMu := &sync.Mutex{}
	taskQueue := newTaskQueueNotifier()
	taskResults := newTaskResultNotifier()
	dockerQueries := newDockerQueryBroker()
	execManager := newExecTunnelManager()
	logManager := newContainerLogTunnelManager()
	notifier, err := appnotify.New(cfg.Notifications)
	if err != nil {
		return fmt.Errorf("initialize notifications: %w", err)
	}

	services, err := repo.DiscoverServices(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return err
	}
	declaredServices := make(map[string][]string, len(services))
	for _, service := range services {
		declaredServices[service.Name] = append([]string(nil), service.TargetNodes...)
	}
	if err := db.SyncDeclaredServices(runtimeCtx, declaredServices); err != nil {
		return err
	}
	startBackground(func() {
		runControllerTasks(runtimeCtx, &controllerTaskExecutor{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue, dnsProviders: defaultDNSProviderFactory{}, taskResults: taskResults, repoMu: repoMu, notifier: notifier})
	})

	mux := http.NewServeMux()
	agentTokens := cfg.NodeTokenMap()
	accessTokens := cfg.EnabledAccessTokenMap()
	controllerStartedAt := time.Now().UTC()

	agentInterceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		nodeID, ok := agentTokens[token]
		if !ok {
			return "", errors.New("invalid agent token")
		}
		return nodeID, nil
	})

	accessInterceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		name, ok := accessTokens[token]
		if !ok {
			return "", errors.New("invalid access token")
		}
		return name, nil
	})
	registerAgentHandlers(mux, cfg, db, agentInterceptor, taskQueue, taskResults, dockerQueries, execManager, logManager, repoMu, notifier)
	registerAccessHandlers(mux, cfg, db, accessInterceptor, availableNodeIDs, taskQueue, taskResults, dockerQueries, execManager, logManager, repoMu, reload, notifier)
	registerMetricsHandler(mux, db, accessTokens, controllerStartedAt)
	registerAlertmanagerHandler(mux, cfg.Notifications, notifier)
	mux.HandleFunc(rpcutil.ControllerExecWSPath, execManager.handleWebsocket)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		Protocols:         protocols,
		ReadHeaderTimeout: 5 * time.Second,
	}

	startBackground(func() { sweepOfflineNodes(runtimeCtx, db, notifier) })
	startBackground(func() { autoPullRepo(runtimeCtx, cfg, db, availableNodeIDs, repoMu, taskQueue) })
	startBackground(func() { runScheduledTasks(runtimeCtx, db, cfg, availableNodeIDs, taskQueue, repoMu) })
	startBackground(func() {
		<-runtimeCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	})

	log.Printf("composia controller parsed %d declared services", len(services))
	log.Printf("composia controller listening on %s", cfg.ListenAddr)
	err = server.ListenAndServe()
	cancelRuntime()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("run controller server: %w", err)
	}
	return nil
}

func sweepOfflineNodes(ctx context.Context, db *store.DB, notifier *appnotify.Notifier) {
	ticker := time.NewTicker(offlineSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			before, err := buildNodeSnapshotMap(ctx, db)
			if err != nil {
				log.Printf("offline sweep snapshot failed: %v", err)
				continue
			}
			if err := db.MarkOfflineNodesBefore(ctx, time.Now().Add(-heartbeatOfflineAfter)); err != nil {
				log.Printf("offline sweep failed: %v", err)
				continue
			}
			after, err := buildNodeSnapshotMap(ctx, db)
			if err != nil {
				log.Printf("offline sweep snapshot refresh failed: %v", err)
				continue
			}
			for nodeID, previous := range before {
				current, ok := after[nodeID]
				if !ok || !previous.IsOnline || current.IsOnline {
					continue
				}
				dispatchNodeNotification(notifier, corenotify.EventNodeOffline, current)
			}
		}
	}
}

func autoPullRepo(ctx context.Context, cfg *config.ControllerConfig, db *store.DB, availableNodeIDs map[string]struct{}, repoMu *sync.Mutex, taskQueue *taskQueueNotifier) {
	if cfg.Git == nil || strings.TrimSpace(cfg.Git.RemoteURL) == "" || strings.TrimSpace(cfg.Git.PullInterval) == "" {
		return
	}
	interval, err := time.ParseDuration(strings.TrimSpace(cfg.Git.PullInterval))
	if err != nil {
		log.Printf("auto-pull: invalid pull_interval %q: %v", cfg.Git.PullInterval, err)
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			repoMu.Lock()
			previousRevision, _ := repo.CurrentRevision(cfg.RepoDir)
			_, pullErr := autoPullFetchAndFastForward(ctx, cfg, db)
			repoMu.Unlock()

			if pullErr != nil {
				log.Printf("auto-pull failed: %v", pullErr)
				continue
			}
			newRevision, _ := repo.CurrentRevision(cfg.RepoDir)
			if newRevision != previousRevision {
				log.Printf("auto-pull: repo updated from %s to %s", shortRevision(previousRevision), shortRevision(newRevision))
				if err := refreshDeclaredServices(ctx, db, cfg, availableNodeIDs); err != nil {
					log.Printf("auto-pull: refresh declared services failed: %v", err)
				}
				handleAutoDeploy(ctx, cfg, db, availableNodeIDs, previousRevision, newRevision, taskQueue)
			}
		}
	}
}

func handleAutoDeploy(ctx context.Context, cfg *config.ControllerConfig, db *store.DB, availableNodeIDs map[string]struct{}, oldRevision, newRevision string, taskQueue *taskQueueNotifier) {
	changedFiles, err := repo.DiffChangedFiles(cfg.RepoDir, oldRevision, newRevision)
	if err != nil {
		log.Printf("auto-deploy: diff changed files failed: %v", err)
		return
	}
	if len(changedFiles) == 0 {
		return
	}
	affectedNames, err := repo.AffectedServicesFromChangedFiles(cfg.RepoDir, changedFiles)
	if err != nil {
		log.Printf("auto-deploy: find affected services failed: %v", err)
		return
	}
	if len(affectedNames) == 0 {
		return
	}
	log.Printf("auto-deploy: detected %d affected services: %s", len(affectedNames), strings.Join(affectedNames, ", "))

	for _, serviceName := range affectedNames {
		service, err := repo.FindService(cfg.RepoDir, availableNodeIDs, serviceName)
		if err != nil {
			log.Printf("auto-deploy: lookup service %q failed: %v", serviceName, err)
			continue
		}
		isInfra := service.Meta.IsInfra()
		shouldAutoDeploy := shouldAutoDeployService(cfg, service, isInfra)

		if err := db.SetServicePendingDeploy(ctx, serviceName, service.TargetNodes, newRevision); err != nil {
			log.Printf("auto-deploy: set pending deploy for %q failed: %v", serviceName, err)
		}
		if shouldAutoDeploy {
			log.Printf("auto-deploy: triggering deploy for %q", serviceName)
			server := &serviceCommandServer{
				db:               db,
				cfg:              cfg,
				availableNodeIDs: availableNodeIDs,
				taskQueue:        taskQueue,
			}
			if _, err := server.createServiceTaskWithOptions(ctx, serviceName, service.TargetNodes, task.TypeDeploy, nil, serviceTaskCreateOptions{Source: task.SourceAutoDeploy}); err != nil {
				log.Printf("auto-deploy: create deploy task for %q failed: %v", serviceName, err)
			}
		}
	}
}

func shouldAutoDeployService(cfg *config.ControllerConfig, service repo.Service, isInfra bool) bool {
	if !service.Meta.AutoDeployEnabled() {
		return false
	}
	if cfg.AutoDeploy == nil {
		return false
	}
	if isInfra {
		return cfg.AutoDeploy.Infra
	}
	return cfg.AutoDeploy.Services
}

func autoPullFetchAndFastForward(ctx context.Context, cfg *config.ControllerConfig, db *store.DB) (store.RepoSyncState, error) {
	cleanWorktree, err := repo.IsCleanWorkingTree(cfg.RepoDir)
	if err != nil {
		return store.RepoSyncState{SyncStatus: store.RepoSyncStatusUnknown}, fmt.Errorf("check worktree clean: %w", err)
	}
	if !cleanWorktree {
		return store.RepoSyncState{SyncStatus: store.RepoSyncStatusLocalOnly}, nil
	}
	branch := strings.TrimSpace(cfg.Git.Branch)
	if branch == "" {
		branch, err = repo.CurrentBranch(cfg.RepoDir)
		if err != nil {
			return store.RepoSyncState{SyncStatus: store.RepoSyncStatusUnknown}, fmt.Errorf("get current branch: %w", err)
		}
	}
	authUsername := ""
	authToken := ""
	if cfg.Git.Auth != nil {
		authUsername = strings.TrimSpace(cfg.Git.Auth.Username)
		authToken = strings.TrimSpace(cfg.Git.Auth.Token)
	}
	previousState, err := db.GetRepoSyncState(ctx)
	if err != nil {
		previousState = store.RepoSyncState{}
	}
	pulledAt := time.Now().UTC().Format(time.RFC3339)
	if err := repo.FetchAndFastForward(cfg.RepoDir, strings.TrimSpace(cfg.Git.RemoteURL), branch, authUsername, authToken); err != nil {
		state := store.RepoSyncState{
			SyncStatus:           store.RepoSyncStatusPullFailed,
			LastSyncError:        err.Error(),
			LastSuccessfulPullAt: previousState.LastSuccessfulPullAt,
		}
		if persistErr := db.UpsertRepoSyncState(ctx, state); persistErr != nil {
			return store.RepoSyncState{}, fmt.Errorf("persist pull failed state: %w", persistErr)
		}
		return state, fmt.Errorf("fetch and fast-forward: %w", err)
	}
	state := store.RepoSyncState{
		SyncStatus:           store.RepoSyncStatusSynced,
		LastSyncError:        "",
		LastSuccessfulPullAt: pulledAt,
	}
	if err := db.UpsertRepoSyncState(ctx, state); err != nil {
		return store.RepoSyncState{}, fmt.Errorf("persist synced state: %w", err)
	}
	return state, nil
}

func refreshDeclaredServices(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}) error {
	services, err := repo.DiscoverServices(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return err
	}
	declaredServices := make(map[string][]string, len(services))
	for _, service := range services {
		declaredServices[service.Name] = append([]string(nil), service.TargetNodes...)
	}
	return db.SyncDeclaredServices(ctx, declaredServices)
}

func shortRevision(revision string) string {
	if len(revision) <= 8 {
		return revision
	}
	return revision[:8]
}
