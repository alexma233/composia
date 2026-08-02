package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"connectrpc.com/connect"

	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	appnotify "forgejo.alexma.top/alexma233/composia/internal/app/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.LoadController(configPath)
	if err != nil {
		return err
	}
	activeRaw, err := os.ReadFile(configPath) //nolint:gosec // The path is the configured controller config.
	if err != nil {
		return fmt.Errorf("read controller config: %w", err)
	}
	reloadRequests := make(chan reloadRequest, 1)
	stopReloadSignals := watchControllerReloadSignals(ctx, reloadRequests)
	defer stopReloadSignals()

	for {
		runtimeCtx, cancelRuntime := context.WithCancel(ctx)
		runtimeDone := make(chan error, 1)
		runtimeReady := make(chan error, 1)
		go func() {
			runtimeDone <- runControllerRuntime(runtimeCtx, cfg, configPath, func(reloadCtx context.Context) error {
				return requestControllerReload(reloadCtx, reloadRequests)
			}, func(reloadCtx context.Context, revision string) error {
				return requestControllerReloadRevision(reloadCtx, reloadRequests, revision)
			}, runtimeReady)
		}()
		if err, done := waitControllerRuntimeReady(ctx, runtimeReady, runtimeDone); err != nil {
			cancelRuntime()
			if !done {
				<-runtimeDone
			}
			return err
		}

		runtimeActive := true
		for runtimeActive {
			select {
			case <-ctx.Done():
				cancelRuntime()
				if err := <-runtimeDone; err != nil {
					return err
				}
				return nil
			case err := <-runtimeDone:
				cancelRuntime()
				return err
			case request := <-reloadRequests:
				currentRaw, readErr := os.ReadFile(configPath) //nolint:gosec // The path is the configured controller config.
				if readErr != nil {
					err := fmt.Errorf("read controller config: %w", readErr)
					if request.expectedRevision != "" {
						cancelRuntime()
						<-runtimeDone
						return err
					}
					request.respond(err)
					continue
				}
				if request.expectedRevision != "" && configRevision(currentRaw) != request.expectedRevision {
					err := errors.New("controller config changed before reload")
					request.respond(err)
					log.Printf("controller config reload skipped: %v", err)
					continue
				}
				nextCfg, err := loadReloadControllerConfig(configPath, cfg)
				if err != nil {
					if rollbackErr := rollbackQueuedControllerConfig(configPath, request, currentRaw, activeRaw, err); rollbackErr != nil {
						cancelRuntime()
						<-runtimeDone
						return rollbackErr
					}
					continue
				}
				latestRaw, readErr := os.ReadFile(configPath) //nolint:gosec // The path is the configured controller config.
				if readErr != nil {
					err := fmt.Errorf("read controller config before reload: %w", readErr)
					if rollbackErr := rollbackQueuedControllerConfig(configPath, request, currentRaw, activeRaw, err); rollbackErr != nil {
						cancelRuntime()
						<-runtimeDone
						return rollbackErr
					}
					continue
				}
				if configRevision(latestRaw) != configRevision(currentRaw) {
					err := errors.New("controller config changed while reload was being prepared")
					request.respond(err)
					log.Printf("controller config reload skipped: %v", err)
					continue
				}
				currentRaw = latestRaw
				cancelRuntime()
				if err := <-runtimeDone; err != nil {
					request.respond(err)
					return err
				}

				nextCtx, cancelNext := context.WithCancel(ctx)
				nextDone := make(chan error, 1)
				nextReady := make(chan error, 1)
				go func() {
					nextDone <- runControllerRuntime(nextCtx, nextCfg, configPath, func(reloadCtx context.Context) error {
						return requestControllerReload(reloadCtx, reloadRequests)
					}, func(reloadCtx context.Context, revision string) error {
						return requestControllerReloadRevision(reloadCtx, reloadRequests, revision)
					}, nextReady)
				}()
				if readyErr, done := waitControllerRuntimeReady(ctx, nextReady, nextDone); readyErr != nil {
					cancelNext()
					if !done {
						<-nextDone
					}
					restoreErr := restoreControllerConfigIfCurrent(configPath, configRevision(currentRaw), activeRaw)
					if restoreErr != nil {
						request.respond(fmt.Errorf("new controller runtime failed: %v; restore failed: %w", readyErr, restoreErr))
						return fmt.Errorf("new controller runtime failed: %v; restore failed: %w", readyErr, restoreErr)
					}
					log.Printf("controller config reload rejected: %v", readyErr)
					request.respond(readyErr)
					runtimeActive = false
					break
				}
				cfg = nextCfg
				activeRaw = currentRaw
				cancelRuntime = cancelNext
				runtimeDone = nextDone
				runtimeActive = true
				log.Printf("controller config reloaded")
				request.respond(nil)
			}
		}
	}
}

func rollbackQueuedControllerConfig(configPath string, request reloadRequest, candidateRaw, activeRaw []byte, cause error) error {
	if request.expectedRevision == "" {
		request.respond(cause)
		log.Printf("controller config reload rejected: %v", cause)
		return nil
	}
	if err := restoreControllerConfigIfCurrent(configPath, configRevision(candidateRaw), activeRaw); err != nil {
		request.respond(fmt.Errorf("controller config reload rejected: %v; restore failed: %w", cause, err))
		return fmt.Errorf("controller config reload rejected: %v; restore failed: %w", cause, err)
	}
	request.respond(cause)
	log.Printf("controller config reload rejected and previous config restored: %v", cause)
	return nil
}

func waitControllerRuntimeReady(ctx context.Context, ready <-chan error, done <-chan error) (error, bool) {
	select {
	case err := <-ready:
		return err, false
	case err := <-done:
		if err == nil {
			return fmt.Errorf("controller runtime stopped before becoming ready"), true
		}
		return err, true
	case <-ctx.Done():
		return ctx.Err(), false
	}
}

func registerAgentHandlers(mux *http.ServeMux, cfg *config.ControllerConfig, db *store.DB, interceptor connect.Interceptor, taskQueue *taskQueueNotifier, taskResults *taskResultNotifier, dockerQueries *dockerQueryBroker, execManager *execTunnelManager, logManager *containerLogTunnelManager, repoMu *sync.Mutex, notifier *appnotify.Notifier) {
	agentPath, agentHandler := agentv1connect.NewAgentReportServiceHandler(
		&agentReportServer{db: db, cfg: cfg, availableNodeIDs: configuredNodeIDs(cfg), logState: &taskLogAckState{confirmedBy: make(map[string]uint64)}, taskQueue: taskQueue, taskResults: taskResults, dockerQueries: dockerQueries, execManager: execManager, logManager: logManager, repoMu: repoMu, notifier: notifier},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.AgentAPIBasePath, agentPath, agentHandler)

	agentTaskPath, agentTaskHandler := agentv1connect.NewAgentTaskServiceHandler(
		&agentTaskServer{db: db, taskQueue: taskQueue, dockerQueries: dockerQueries},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.AgentAPIBasePath, agentTaskPath, agentTaskHandler)

	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(
		&bundleServer{db: db, cfg: cfg},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.AgentAPIBasePath, bundlePath, bundleHandler)
}

func registerAccessHandlers(mux *http.ServeMux, cfg *config.ControllerConfig, configPath string, db *store.DB, interceptor connect.Interceptor, availableNodeIDs map[string]struct{}, taskQueue *taskQueueNotifier, taskResults *taskResultNotifier, dockerQueries *dockerQueryBroker, execManager *execTunnelManager, logManager *containerLogTunnelManager, repoMu *sync.Mutex, reload func(context.Context) error, reloadRevision func(context.Context, string) error, notifier *appnotify.Notifier) {
	systemPath, systemHandler := controllerv1connect.NewSystemServiceHandler(
		&systemServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, reload: reload},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, systemPath, systemHandler)

	configPathHandler, configHandler := controllerv1connect.NewControllerConfigServiceHandler(
		&controllerConfigEditor{configPath: configPath, cfg: cfg, reload: reload, reloadRevision: reloadRevision},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, configPathHandler, configHandler)

	repoQueryPath, repoQueryHandler := controllerv1connect.NewRepoQueryServiceHandler(
		&repoQueryServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, repoQueryPath, repoQueryHandler)

	repoCommandPath, repoCommandHandler := controllerv1connect.NewRepoCommandServiceHandler(
		&repoCommandServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, repoCommandPath, repoCommandHandler)

	backupSvc := &backupServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue}
	backupQueryPath, backupQueryHandler := controllerv1connect.NewBackupQueryServiceHandler(
		backupSvc,
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, backupQueryPath, backupQueryHandler)

	backupCommandPath, backupCommandHandler := controllerv1connect.NewBackupCommandServiceHandler(
		backupSvc,
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, backupCommandPath, backupCommandHandler)

	serviceQueryPath, serviceQueryHandler := controllerv1connect.NewServiceQueryServiceHandler(
		&serviceQueryServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue, taskResults: taskResults, dockerQueries: dockerQueries, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, serviceQueryPath, serviceQueryHandler)

	serviceCommandPath, serviceCommandHandler := controllerv1connect.NewServiceCommandServiceHandler(
		&serviceCommandServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue, taskResults: taskResults, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, serviceCommandPath, serviceCommandHandler)

	serviceInstancePath, serviceInstanceHandler := controllerv1connect.NewServiceInstanceServiceHandler(
		&serviceInstanceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue, taskResults: taskResults, dockerQueries: dockerQueries},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, serviceInstancePath, serviceInstanceHandler)

	nodeQueryPath, nodeQueryHandler := controllerv1connect.NewNodeQueryServiceHandler(
		&nodeQueryServer{db: db, cfg: cfg, taskQueue: taskQueue, taskResults: taskResults},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, nodeQueryPath, nodeQueryHandler)

	nodeMaintenancePath, nodeMaintenanceHandler := controllerv1connect.NewNodeMaintenanceServiceHandler(
		&nodeMaintenanceServer{db: db, cfg: cfg, taskQueue: taskQueue, taskResults: taskResults},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, nodeMaintenancePath, nodeMaintenanceHandler)

	dockerQueryPath, dockerQueryHandler := controllerv1connect.NewDockerQueryServiceHandler(
		&dockerQueryServer{db: db, cfg: cfg, dockerQueries: dockerQueries},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, dockerQueryPath, dockerQueryHandler)

	dockerCommandPath, dockerCommandHandler := controllerv1connect.NewDockerCommandServiceHandler(
		&dockerCommandServer{db: db, cfg: cfg, taskQueue: taskQueue, taskResults: taskResults, dockerQueries: dockerQueries, execManager: execManager, logManager: logManager},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, dockerCommandPath, dockerCommandHandler)

	taskPath, taskHandler := controllerv1connect.NewTaskServiceHandler(
		&taskServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue, taskResults: taskResults, notifier: notifier},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, taskPath, taskHandler)
}

func mountRPCHandler(mux *http.ServeMux, basePath, rpcPath string, handler http.Handler) {
	mux.Handle(rpcutil.PrefixRPCPath(basePath, rpcPath), http.StripPrefix(basePath, handler))
}
