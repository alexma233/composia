package controller

import (
	"connectrpc.com/connect"
	"context"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	appnotify "forgejo.alexma.top/alexma233/composia/internal/app/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"log"
	"net/http"
	"sync"
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.LoadController(configPath)
	if err != nil {
		return err
	}
	reloadRequests := make(chan reloadRequest)
	stopReloadSignals := watchControllerReloadSignals(ctx, reloadRequests)
	defer stopReloadSignals()

	for {
		runtimeCtx, cancelRuntime := context.WithCancel(ctx)
		runtimeDone := make(chan error, 1)
		go func() {
			runtimeDone <- runControllerRuntime(runtimeCtx, cfg, func(reloadCtx context.Context) error {
				return requestControllerReload(reloadCtx, reloadRequests)
			})
		}()

		reloadAccepted := false
		for !reloadAccepted {
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
				nextCfg, err := loadReloadControllerConfig(configPath, cfg)
				request.respond(err)
				if err != nil {
					log.Printf("controller config reload rejected: %v", err)
					continue
				}
				cancelRuntime()
				if err := <-runtimeDone; err != nil {
					return err
				}
				cfg = nextCfg
				reloadAccepted = true
				log.Printf("controller config reloaded")
			}
		}
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

func registerAccessHandlers(mux *http.ServeMux, cfg *config.ControllerConfig, db *store.DB, interceptor connect.Interceptor, availableNodeIDs map[string]struct{}, taskQueue *taskQueueNotifier, taskResults *taskResultNotifier, dockerQueries *dockerQueryBroker, execManager *execTunnelManager, logManager *containerLogTunnelManager, repoMu *sync.Mutex, reload func(context.Context) error, notifier *appnotify.Notifier) {
	systemPath, systemHandler := controllerv1connect.NewSystemServiceHandler(
		&systemServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, reload: reload},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, systemPath, systemHandler)

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

	secretPath, secretHandler := controllerv1connect.NewSecretServiceHandler(
		&secretServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mountRPCHandler(mux, rpcutil.ControllerAPIBasePath, secretPath, secretHandler)

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
