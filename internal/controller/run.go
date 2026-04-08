package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	backupcfg "forgejo.alexma.top/alexma233/composia/internal/backup"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	secretutil "forgejo.alexma.top/alexma233/composia/internal/secret"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"github.com/google/uuid"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	heartbeatOfflineAfter = 45 * time.Second
	offlineSweepInterval  = 15 * time.Second
	pullNextTaskMaxWait   = 25 * time.Second
	pullNextTaskRetryWait = 500 * time.Millisecond
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.LoadController(configPath)
	if err != nil {
		return err
	}

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
	defer db.Close()

	nodeIDs := make([]string, 0, len(cfg.Nodes))
	availableNodeIDs := make(map[string]struct{}, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		nodeIDs = append(nodeIDs, node.ID)
		availableNodeIDs[node.ID] = struct{}{}
	}
	if err := db.SyncConfiguredNodes(ctx, nodeIDs); err != nil {
		return err
	}
	if err := db.MarkOfflineNodesBefore(ctx, time.Now().Add(-heartbeatOfflineAfter)); err != nil {
		return err
	}
	if _, err := db.RecoverRunningTasks(ctx, time.Now().UTC()); err != nil {
		return err
	}
	taskQueue := newTaskQueueNotifier()
	taskResults := newTaskResultNotifier()
	execManager := newExecTunnelManager()

	services, err := repo.DiscoverServices(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return err
	}
	declaredServices := make(map[string][]string, len(services))
	for _, service := range services {
		declaredServices[service.Name] = append([]string(nil), service.TargetNodes...)
	}
	if err := db.SyncDeclaredServices(ctx, declaredServices); err != nil {
		return err
	}
	go runControllerTasks(ctx, &controllerTaskExecutor{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue, dnsProviders: defaultDNSProviderFactory{}})

	mux := http.NewServeMux()
	agentTokens := cfg.NodeTokenMap()
	cliTokens := cfg.EnabledCLITokenMap()

	agentInterceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		nodeID, ok := agentTokens[token]
		if !ok {
			return "", errors.New("invalid agent token")
		}
		return nodeID, nil
	})

	cliInterceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		name, ok := cliTokens[token]
		if !ok {
			return "", errors.New("invalid CLI token")
		}
		return name, nil
	})
	repoMu := &sync.Mutex{}

	registerAgentHandlers(mux, cfg, db, agentInterceptor, taskQueue, taskResults, execManager)
	registerCLIHandlers(mux, cfg, db, cliInterceptor, availableNodeIDs, taskQueue, taskResults, execManager, repoMu)
	mux.HandleFunc("/ws/container-exec/", execManager.handleWebsocket)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go sweepOfflineNodes(ctx, db)
	go autoPullRepo(ctx, cfg, db, availableNodeIDs, repoMu)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("composia controller parsed %d declared services", len(services))
	log.Printf("composia controller listening on %s", cfg.ListenAddr)
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("run controller server: %w", err)
	}
	return nil
}

func registerAgentHandlers(mux *http.ServeMux, cfg *config.ControllerConfig, db *store.DB, interceptor connect.Interceptor, taskQueue *taskQueueNotifier, taskResults *taskResultNotifier, execManager *execTunnelManager) {
	agentPath, agentHandler := agentv1connect.NewAgentReportServiceHandler(
		&agentReportServer{db: db, cfg: cfg, availableNodeIDs: configuredNodeIDs(cfg), logState: &taskLogAckState{confirmedBy: make(map[string]uint64)}, taskQueue: taskQueue, taskResults: taskResults, execManager: execManager},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(agentPath, agentHandler)

	agentTaskPath, agentTaskHandler := agentv1connect.NewAgentTaskServiceHandler(
		&agentTaskServer{db: db, taskQueue: taskQueue},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(agentTaskPath, agentTaskHandler)

	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(
		&bundleServer{db: db, cfg: cfg},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(bundlePath, bundleHandler)
}

func registerCLIHandlers(mux *http.ServeMux, cfg *config.ControllerConfig, db *store.DB, interceptor connect.Interceptor, availableNodeIDs map[string]struct{}, taskQueue *taskQueueNotifier, taskResults *taskResultNotifier, execManager *execTunnelManager, repoMu *sync.Mutex) {
	systemPath, systemHandler := controllerv1connect.NewSystemServiceHandler(
		&systemServer{db: db, cfg: cfg},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(systemPath, systemHandler)

	repoPath, repoHandler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(repoPath, repoHandler)

	secretPath, secretHandler := controllerv1connect.NewSecretServiceHandler(
		&secretServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, repoMu: repoMu},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(secretPath, secretHandler)

	backupPath, backupHandler := controllerv1connect.NewBackupRecordServiceHandler(
		&backupRecordServer{db: db},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(backupPath, backupHandler)

	servicePath, serviceHandler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue, taskResults: taskResults},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(servicePath, serviceHandler)

	serviceInstancePath, serviceInstanceHandler := controllerv1connect.NewServiceInstanceServiceHandler(
		&serviceInstanceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue, taskResults: taskResults},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(serviceInstancePath, serviceInstanceHandler)

	nodePath, nodeHandler := controllerv1connect.NewNodeServiceHandler(
		&nodeServer{db: db, cfg: cfg, taskQueue: taskQueue, taskResults: taskResults},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(nodePath, nodeHandler)

	containerPath, containerHandler := controllerv1connect.NewContainerServiceHandler(
		&containerServer{db: db, cfg: cfg, taskQueue: taskQueue, taskResults: taskResults, execManager: execManager},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(containerPath, containerHandler)

	taskPath, taskHandler := controllerv1connect.NewTaskServiceHandler(
		&taskServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue},
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(taskPath, taskHandler)
}

func sweepOfflineNodes(ctx context.Context, db *store.DB) {
	ticker := time.NewTicker(offlineSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := db.MarkOfflineNodesBefore(context.Background(), time.Now().Add(-heartbeatOfflineAfter)); err != nil {
				log.Printf("offline sweep failed: %v", err)
			}
		}
	}
}

func autoPullRepo(ctx context.Context, cfg *config.ControllerConfig, db *store.DB, availableNodeIDs map[string]struct{}, repoMu *sync.Mutex) {
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
				log.Printf("auto-pull: repo updated from %s to %s", previousRevision[:8], newRevision[:8])
				if err := refreshDeclaredServices(db, cfg, availableNodeIDs); err != nil {
					log.Printf("auto-pull: refresh declared services failed: %v", err)
				}
			}
		}
	}
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
	authToken := ""
	if cfg.Git.Auth != nil && strings.TrimSpace(cfg.Git.Auth.TokenFile) != "" {
		tokenContent, err := os.ReadFile(strings.TrimSpace(cfg.Git.Auth.TokenFile))
		if err != nil {
			return store.RepoSyncState{SyncStatus: store.RepoSyncStatusUnknown}, fmt.Errorf("read git auth token: %w", err)
		}
		authToken = strings.TrimSpace(string(tokenContent))
	}
	previousState, err := db.GetRepoSyncState(ctx)
	if err != nil {
		previousState = store.RepoSyncState{}
	}
	pulledAt := time.Now().UTC().Format(time.RFC3339)
	if err := repo.FetchAndFastForward(cfg.RepoDir, strings.TrimSpace(cfg.Git.RemoteURL), branch, authToken); err != nil {
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

func refreshDeclaredServices(db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}) error {
	services, err := repo.DiscoverServices(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return err
	}
	declaredServices := make(map[string][]string, len(services))
	for _, service := range services {
		declaredServices[service.Name] = append([]string(nil), service.TargetNodes...)
	}
	return db.SyncDeclaredServices(context.Background(), declaredServices)
}

type agentReportServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	logState         *taskLogAckState
	taskQueue        *taskQueueNotifier
	taskResults      *taskResultNotifier
	execManager      *execTunnelManager
}

type agentTaskServer struct {
	db            *store.DB
	taskQueue     *taskQueueNotifier
	maxWait       time.Duration
	retryInterval time.Duration
}

type bundleServer struct {
	db  *store.DB
	cfg *config.ControllerConfig
}

type repoServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	repoMu           *sync.Mutex
}

type repoWriteResult struct {
	CommitID             string
	SyncStatus           string
	PushError            string
	LastSuccessfulPullAt string
}

type secretServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	repoMu           *sync.Mutex
}

type taskQueueNotifier struct {
	mu          sync.Mutex
	subscribers map[chan struct{}]struct{}
}

func newTaskQueueNotifier() *taskQueueNotifier {
	return &taskQueueNotifier{subscribers: make(map[chan struct{}]struct{})}
}

func (notifier *taskQueueNotifier) Subscribe() chan struct{} {
	if notifier == nil {
		return nil
	}
	ch := make(chan struct{}, 1)
	notifier.mu.Lock()
	notifier.subscribers[ch] = struct{}{}
	notifier.mu.Unlock()
	return ch
}

func (notifier *taskQueueNotifier) Unsubscribe(ch chan struct{}) {
	if notifier == nil || ch == nil {
		return
	}
	notifier.mu.Lock()
	if _, ok := notifier.subscribers[ch]; ok {
		delete(notifier.subscribers, ch)
		close(ch)
	}
	notifier.mu.Unlock()
}

func (notifier *taskQueueNotifier) Notify() {
	if notifier == nil {
		return
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	for ch := range notifier.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (server *agentReportServer) Heartbeat(ctx context.Context, req *connect.Request[agentv1.HeartbeatRequest]) (*connect.Response[agentv1.HeartbeatResponse], error) {
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	if req.Msg.GetRuntime() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("runtime is required"))
	}

	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}

	heartbeatAt := time.Now().UTC()
	if sentAt := req.Msg.GetSentAt(); sentAt != nil {
		heartbeatAt = sentAt.AsTime().UTC()
	}

	err := server.db.RecordHeartbeat(ctx, store.NodeHeartbeat{
		NodeID:              req.Msg.GetNodeId(),
		HeartbeatAt:         heartbeatAt,
		AgentVersion:        req.Msg.GetAgentVersion(),
		DockerServerVersion: req.Msg.GetRuntime().GetDockerServerVersion(),
		DiskTotalBytes:      req.Msg.GetRuntime().GetDiskTotalBytes(),
		DiskFreeBytes:       req.Msg.GetRuntime().GetDiskFreeBytes(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&agentv1.HeartbeatResponse{ReceivedAt: timestamppb.Now()}), nil
}

func (server *agentReportServer) ReportDockerStats(ctx context.Context, req *connect.Request[agentv1.ReportDockerStatsRequest]) (*connect.Response[agentv1.ReportDockerStatsResponse], error) {
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	if req.Msg.GetStats() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("stats is required"))
	}

	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}

	stats := req.Msg.GetStats()
	err := server.db.RecordDockerStats(ctx, store.DockerStats{
		NodeID:              req.Msg.GetNodeId(),
		ContainersTotal:     stats.GetContainersTotal(),
		ContainersRunning:   stats.GetContainersRunning(),
		ContainersStopped:   stats.GetContainersStopped(),
		ContainersPaused:    stats.GetContainersPaused(),
		Images:              stats.GetImages(),
		Networks:            stats.GetNetworks(),
		Volumes:             stats.GetVolumes(),
		VolumesSizeBytes:    stats.GetVolumesSizeBytes(),
		DisksUsageBytes:     stats.GetDisksUsageBytes(),
		DockerServerVersion: stats.GetDockerServerVersion(),
		ReportedAt:          time.Now().UTC(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&agentv1.ReportDockerStatsResponse{}), nil
}

func (server *agentReportServer) ReportTaskState(ctx context.Context, req *connect.Request[agentv1.ReportTaskStateRequest]) (*connect.Response[agentv1.ReportTaskStateResponse], error) {
	if req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.GetStatus() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("status is required"))
	}
	if err := ensureTaskNodeMatch(ctx, server.db, req.Msg.GetTaskId()); err != nil {
		return nil, err
	}

	finishedAt := time.Now().UTC()
	if req.Msg.GetFinishedAt() != nil {
		finishedAt = req.Msg.GetFinishedAt().AsTime().UTC()
	}
	if err := server.db.CompleteTask(ctx, req.Msg.GetTaskId(), task.Status(req.Msg.GetStatus()), finishedAt, req.Msg.GetErrorSummary()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := server.queuePostTaskFollowups(ctx, req.Msg.GetTaskId(), task.Status(req.Msg.GetStatus())); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	server.resetTaskLogAck(req.Msg.GetTaskId())
	server.taskResults.Notify(req.Msg.GetTaskId())
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&agentv1.ReportTaskStateResponse{}), nil
}

func (server *agentReportServer) queuePostTaskFollowups(ctx context.Context, taskID string, status task.Status) error {
	if status != task.StatusSucceeded {
		return nil
	}
	detail, err := server.db.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	switch detail.Record.Type {
	case task.TypeDeploy, task.TypeUpdate, task.TypeStop:
		return server.queueCaddyReloadForTask(ctx, detail.Record)
	default:
		return nil
	}
}

func (server *agentReportServer) queueCaddyReloadForTask(ctx context.Context, record task.Record) error {
	if server.cfg == nil {
		return nil
	}
	params := taskParams(record.ParamsJSON)
	if params.ServiceDir == "" || record.RepoRevision == "" || record.NodeID == "" {
		return nil
	}
	service, err := repo.FindServiceAtRevision(server.cfg.RepoDir, record.RepoRevision, params.ServiceDir, server.availableNodeIDs)
	if err != nil {
		return fmt.Errorf("load service for post-task caddy reload: %w", err)
	}
	if !repo.CaddyManaged(service) {
		return nil
	}
	if _, err := createNodeCaddyReloadTask(ctx, server.db, server.cfg, server.availableNodeIDs, record.NodeID, requestTaskSource(nil)); err != nil {
		return err
	}
	return nil
}

func (server *agentReportServer) ReportTaskStepState(ctx context.Context, req *connect.Request[agentv1.ReportTaskStepStateRequest]) (*connect.Response[agentv1.ReportTaskStepStateResponse], error) {
	if req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.GetStepName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("step_name is required"))
	}
	if req.Msg.GetStatus() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("status is required"))
	}
	if err := ensureTaskNodeMatch(ctx, server.db, req.Msg.GetTaskId()); err != nil {
		return nil, err
	}

	step := task.StepRecord{
		TaskID:     req.Msg.GetTaskId(),
		StepName:   task.StepName(req.Msg.GetStepName()),
		Status:     task.Status(req.Msg.GetStatus()),
		StartedAt:  protoTime(req.Msg.GetStartedAt()),
		FinishedAt: protoTime(req.Msg.GetFinishedAt()),
	}
	if err := server.db.UpsertTaskStep(ctx, step); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&agentv1.ReportTaskStepStateResponse{}), nil
}

func (server *agentReportServer) ReportBackupResult(ctx context.Context, req *connect.Request[agentv1.ReportBackupResultRequest]) (*connect.Response[agentv1.ReportBackupResultResponse], error) {
	if req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.GetBackupId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("backup_id is required"))
	}
	if req.Msg.GetDataName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("data_name is required"))
	}
	if req.Msg.GetStatus() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("status is required"))
	}
	if err := ensureTaskNodeMatch(ctx, server.db, req.Msg.GetTaskId()); err != nil {
		return nil, err
	}
	startedAt := time.Now().UTC().Format(time.RFC3339)
	if req.Msg.GetStartedAt() != nil {
		startedAt = req.Msg.GetStartedAt().AsTime().UTC().Format(time.RFC3339)
	}
	finishedAt := ""
	if req.Msg.GetFinishedAt() != nil {
		finishedAt = req.Msg.GetFinishedAt().AsTime().UTC().Format(time.RFC3339)
	}
	if err := server.db.UpsertBackupRecord(ctx, store.BackupDetail{
		BackupID:     req.Msg.GetBackupId(),
		TaskID:       req.Msg.GetTaskId(),
		ServiceName:  req.Msg.GetServiceName(),
		DataName:     req.Msg.GetDataName(),
		Status:       req.Msg.GetStatus(),
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		ArtifactRef:  req.Msg.GetArtifactRef(),
		ErrorSummary: req.Msg.GetErrorSummary(),
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&agentv1.ReportBackupResultResponse{}), nil
}

func (server *agentReportServer) ReportServiceInstanceStatus(ctx context.Context, req *connect.Request[agentv1.ReportServiceInstanceStatusRequest]) (*connect.Response[agentv1.ReportServiceInstanceStatusResponse], error) {
	if req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	if !store.IsValidServiceRuntimeStatus(req.Msg.GetRuntimeStatus()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid runtime_status %q", req.Msg.GetRuntimeStatus()))
	}
	reportedAt := time.Now().UTC()
	if req.Msg.GetReportedAt() != nil {
		reportedAt = req.Msg.GetReportedAt().AsTime().UTC()
	}
	if err := server.db.UpdateServiceInstanceRuntimeStatus(ctx, req.Msg.GetServiceName(), req.Msg.GetNodeId(), req.Msg.GetRuntimeStatus(), reportedAt); err != nil {
		if errors.Is(err, store.ErrServiceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&agentv1.ReportServiceInstanceStatusResponse{}), nil
}

func ensureTaskNodeMatch(ctx context.Context, db *store.DB, taskID string) error {
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("missing authenticated node"))
	}
	taskNodeID, err := db.TaskNodeID(ctx, taskID)
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return connect.NewError(connect.CodeNotFound, err)
		}
		return connect.NewError(connect.CodeInternal, err)
	}
	if taskNodeID != authenticatedNodeID {
		return connect.NewError(connect.CodePermissionDenied, errors.New("task does not belong to authenticated node"))
	}
	return nil
}

func (server *bundleServer) GetServiceBundle(ctx context.Context, req *connect.Request[agentv1.GetServiceBundleRequest], stream *connect.ServerStream[agentv1.GetServiceBundleResponse]) error {
	if req.Msg.GetTaskId() == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if err := ensureTaskNodeMatch(ctx, server.db, req.Msg.GetTaskId()); err != nil {
		return err
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return connect.NewError(connect.CodeNotFound, err)
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	var params serviceTaskParams
	if err := json.Unmarshal([]byte(detail.Record.ParamsJSON), &params); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("decode deploy task params: %w", err))
	}
	if params.ServiceDir == "" {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("deploy task is missing service_dir"))
	}
	extraFiles, err := bundleExtraFiles(server.cfg, detail.Record, params)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	pipeReader, pipeWriter := io.Pipe()
	go func() {
		pipeWriter.CloseWithError(repo.StreamServiceBundleWithExtras(ctx, server.cfg.RepoDir, detail.Record.RepoRevision, params.ServiceDir, extraFiles, pipeWriter))
	}()
	defer pipeReader.Close()

	buffer := make([]byte, 32*1024)
	firstChunk := true
	for {
		count, err := pipeReader.Read(buffer)
		if count > 0 {
			response := &agentv1.GetServiceBundleResponse{Data: bytes.Clone(buffer[:count])}
			if firstChunk {
				response.ServiceName = detail.Record.ServiceName
				response.RepoRevision = detail.Record.RepoRevision
				response.RelativeRoot = params.ServiceDir
				firstChunk = false
			}
			if sendErr := stream.Send(response); sendErr != nil {
				return sendErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return connect.NewError(connect.CodeInternal, fmt.Errorf("read bundle stream: %w", err))
		}
	}
}

func bundleExtraFiles(cfg *config.ControllerConfig, record task.Record, params serviceTaskParams) (map[string]string, error) {
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
	if record.Type == task.TypeBackup {
		payload, err := buildBackupRuntimePayload(cfg, record.ServiceName, record.NodeID, record.RepoRevision, params)
		if err != nil {
			return nil, err
		}
		if payload != "" {
			extraFiles[filepath.ToSlash(filepath.Join(params.ServiceDir, ".composia-backup.json"))] = payload
		}
	}
	if len(extraFiles) == 0 {
		return nil, nil
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
	service, err := repo.FindService(cfg.RepoDir, configuredNodeIDs(cfg), serviceName)
	if err != nil {
		return "", err
	}
	rusticService, err := repo.FindRusticInfraService(cfg.RepoDir, configuredNodeIDs(cfg))
	if err != nil {
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
		provider := "rustic"
		for _, backupItem := range service.Meta.Backup.Data {
			if backupItem.Name == data.Name {
				if backupItem.Provider != "" {
					provider = backupItem.Provider
				}
				break
			}
		}
		if provider != "rustic" {
			return "", fmt.Errorf("backup provider %q is not implemented", provider)
		}
		items = append(items, backupcfg.RuntimeItem{Name: data.Name, Strategy: data.Backup.Strategy, Service: data.Backup.Service, Include: append([]string(nil), data.Backup.Include...), Provider: provider, Tags: []string{"composia-service:" + serviceName, "composia-data:" + data.Name}})
	}
	payload, err := json.Marshal(backupcfg.RuntimeConfig{Rustic: &backupcfg.RusticConfig{ServiceName: rusticService.Name, ServiceDir: rusticServiceDir, ComposeService: rusticService.Meta.RusticComposeService(), Profile: rusticService.Meta.RusticProfile(), NodeID: nodeID}, Items: items})
	if err != nil {
		return "", fmt.Errorf("marshal backup runtime config for %s at %s: %w", serviceName, revision, err)
	}
	return string(payload), nil
}

func configuredNodeIDs(cfg *config.ControllerConfig) map[string]struct{} {
	result := make(map[string]struct{}, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		result[node.ID] = struct{}{}
	}
	return result
}

func createNodeCaddyReloadTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, nodeID string, source task.Source) (task.Record, error) {
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
	active, err := db.HasActiveServiceInstanceTask(ctx, service.Name, nodeID)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if active {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service instance %q@%q already has an active task", service.Name, nodeID))
	}
	serviceDir, err := filepath.Rel(cfg.RepoDir, service.Directory)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve caddy service directory: %w", err))
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: serviceDir})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode task params: %w", err))
	}
	taskID := uuid.NewString()
	createdTask, err := db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        task.TypeCaddyReload,
		Source:      source,
		ServiceName: service.Name,
		NodeID:      nodeID,
		Status:      task.StatusPending,
		ParamsJSON:  string(paramsJSON),
		LogPath:     filepath.Join(cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
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
	var (
		serviceDirs []string
		matchedName string
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
		matchedName = "caddy"
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
		matchedName = service.Name
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDirs: serviceDirs, FullRebuild: fullRebuild})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode task params: %w", err))
	}
	taskID := uuid.NewString()
	createdTask, err := db.CreateTask(ctx, task.Record{TaskID: taskID, Type: task.TypeCaddySync, Source: source, ServiceName: matchedName, NodeID: nodeID, Status: task.StatusPending, ParamsJSON: string(paramsJSON), LogPath: filepath.Join(cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID))})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
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
		return "", fmt.Errorf("rustic infra service does not have any eligible main nodes")
	}
	online := make([]string, 0, len(candidates))
	for _, nodeID := range candidates {
		if err := validateTaskTargetNode(ctx, db, cfg, nodeID, taskType); err == nil {
			online = append(online, nodeID)
		}
	}
	if len(online) == 0 {
		return "", fmt.Errorf("no eligible online rustic main node is available")
	}
	return online[rand.Intn(len(online))], nil
}

func createNodeRusticMaintenanceTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, nodeID string, taskType task.Type, params rusticPruneTaskParams, source task.Source) (task.Record, error) {
	if err := validateTaskTargetNode(ctx, db, cfg, nodeID, taskType); err != nil {
		return task.Record{}, err
	}
	rusticService, err := repo.FindRusticInfraService(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if !slices.Contains(rusticService.TargetNodes, nodeID) {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("rustic infra service is not declared on node %q", nodeID))
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode rustic prune task params: %w", err))
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	createdTask, err := db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        taskType,
		Source:      source,
		TriggeredBy: triggeredBy,
		ServiceName: rusticService.Name,
		NodeID:      nodeID,
		Status:      task.StatusPending,
		ParamsJSON:  string(paramsJSON),
		LogPath:     filepath.Join(cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	return createdTask, nil
}

func (server *agentTaskServer) PullNextTask(ctx context.Context, req *connect.Request[agentv1.PullNextTaskRequest]) (*connect.Response[agentv1.PullNextTaskResponse], error) {
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}

	waitCh := server.taskQueue.Subscribe()
	defer server.taskQueue.Unsubscribe(waitCh)
	deadline := time.Now().Add(server.longPollMaxWait())

	for {
		record, err := server.db.ClaimNextPendingTaskForNode(ctx, req.Msg.GetNodeId(), time.Now().UTC())
		if err == nil {
			response := &agentv1.PullNextTaskResponse{
				HasTask: true,
				Task: &agentv1.AgentTask{
					TaskId:       record.TaskID,
					Type:         string(record.Type),
					ServiceName:  record.ServiceName,
					NodeId:       record.NodeID,
					RepoRevision: record.RepoRevision,
					ServiceDir:   taskParams(record.ParamsJSON).ServiceDir,
					DataNames:    taskParams(record.ParamsJSON).DataNames,
					ParamsJson:   record.ParamsJSON,
				},
			}
			return connect.NewResponse(response), nil
		}
		if !errors.Is(err, store.ErrNoPendingTask) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return connect.NewResponse(&agentv1.PullNextTaskResponse{HasTask: false}), nil
		}
		waitFor := minDuration(remaining, server.longPollRetryInterval())
		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-waitCh:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (server *agentTaskServer) longPollMaxWait() time.Duration {
	if server.maxWait > 0 {
		return server.maxWait
	}
	return pullNextTaskMaxWait
}

func (server *agentTaskServer) longPollRetryInterval() time.Duration {
	if server.retryInterval > 0 {
		return server.retryInterval
	}
	return pullNextTaskRetryWait
}

type systemServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
}

type serviceServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
	taskResults      *taskResultNotifier
}

type serviceInstanceServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
	taskResults      *taskResultNotifier
}

type serviceTaskParams struct {
	ServiceDir  string   `json:"service_dir"`
	ServiceDirs []string `json:"service_dirs,omitempty"`
	DataNames   []string `json:"data_names,omitempty"`
	FullRebuild bool     `json:"full_rebuild,omitempty"`
}

type rusticPruneTaskParams struct {
	ServiceName string `json:"service_name,omitempty"`
	DataName    string `json:"data_name,omitempty"`
}

type nodeServer struct {
	db          *store.DB
	cfg         *config.ControllerConfig
	taskQueue   *taskQueueNotifier
	taskResults *taskResultNotifier
}

type containerServer struct {
	db          *store.DB
	cfg         *config.ControllerConfig
	taskQueue   *taskQueueNotifier
	taskResults *taskResultNotifier
	execManager *execTunnelManager
}

type taskResultNotifier struct {
	mu          sync.Mutex
	subscribers map[string]map[chan struct{}]struct{}
}

func newTaskResultNotifier() *taskResultNotifier {
	return &taskResultNotifier{subscribers: make(map[string]map[chan struct{}]struct{})}
}

func (notifier *taskResultNotifier) Subscribe(taskID string) chan struct{} {
	if notifier == nil || taskID == "" {
		return nil
	}
	ch := make(chan struct{}, 1)
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if notifier.subscribers[taskID] == nil {
		notifier.subscribers[taskID] = make(map[chan struct{}]struct{})
	}
	notifier.subscribers[taskID][ch] = struct{}{}
	return ch
}

func (notifier *taskResultNotifier) Unsubscribe(taskID string, ch chan struct{}) {
	if notifier == nil || taskID == "" || ch == nil {
		return
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	subscribers := notifier.subscribers[taskID]
	if subscribers == nil {
		return
	}
	if _, ok := subscribers[ch]; ok {
		delete(subscribers, ch)
		close(ch)
	}
	if len(subscribers) == 0 {
		delete(notifier.subscribers, taskID)
	}
}

func (notifier *taskResultNotifier) Notify(taskID string) {
	if notifier == nil || taskID == "" {
		return
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	for ch := range notifier.subscribers[taskID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

type taskServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
}

type backupRecordServer struct {
	db *store.DB
}

func (server *systemServer) GetSystemStatus(ctx context.Context, _ *connect.Request[controllerv1.GetSystemStatusRequest]) (*connect.Response[controllerv1.GetSystemStatusResponse], error) {
	configured, online, err := server.db.NodeCounts(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.GetSystemStatusResponse{
		Version:             version.Value,
		Now:                 timestamppb.Now(),
		ConfiguredNodeCount: configured,
		OnlineNodeCount:     online,
		ControllerAddr:      server.cfg.ControllerAddr,
		RepoDir:             server.cfg.RepoDir,
		StateDir:            server.cfg.StateDir,
		LogDir:              server.cfg.LogDir,
	}
	return connect.NewResponse(response), nil
}

func (server *systemServer) GetCurrentConfig(ctx context.Context, _ *connect.Request[controllerv1.GetCurrentConfigRequest]) (*connect.Response[controllerv1.GetCurrentConfigResponse], error) {
	response := &controllerv1.GetCurrentConfigResponse{
		ListenAddr:     server.cfg.ListenAddr,
		ControllerAddr: server.cfg.ControllerAddr,
	}

	if server.cfg.Git != nil {
		response.Git = &controllerv1.GitConfigSummary{
			RemoteUrl:    server.cfg.Git.RemoteURL,
			Branch:       server.cfg.Git.Branch,
			PullInterval: server.cfg.Git.PullInterval,
			HasAuth:      server.cfg.Git.Auth != nil && server.cfg.Git.Auth.TokenFile != "",
			AuthorName:   server.cfg.Git.AuthorName,
			AuthorEmail:  server.cfg.Git.AuthorEmail,
		}
	}

	response.Nodes = make([]*controllerv1.NodeConfigSummary, 0, len(server.cfg.Nodes))
	for _, node := range server.cfg.Nodes {
		enabled := true
		if node.Enabled != nil {
			enabled = *node.Enabled
		}
		response.Nodes = append(response.Nodes, &controllerv1.NodeConfigSummary{
			Id:          node.ID,
			DisplayName: node.DisplayName,
			Enabled:     enabled,
			PublicIpv4:  node.PublicIPv4,
			PublicIpv6:  node.PublicIPv6,
		})
	}

	response.CliTokens = make([]*controllerv1.CLITokenSummary, 0, len(server.cfg.CLITokens))
	for _, token := range server.cfg.CLITokens {
		enabled := true
		if token.Enabled != nil {
			enabled = *token.Enabled
		}
		response.CliTokens = append(response.CliTokens, &controllerv1.CLITokenSummary{
			Name:    token.Name,
			Enabled: enabled,
			Comment: token.Comment,
		})
	}

	if server.cfg.DNS != nil && server.cfg.DNS.Cloudflare != nil {
		response.Dns = &controllerv1.DNSConfigSummary{
			HasCloudflare: server.cfg.DNS.Cloudflare.APITokenFile != "",
		}
	}

	if _, err := repo.FindRusticInfraService(server.cfg.RepoDir, server.availableNodeIDs); err == nil {
		response.Backup = &controllerv1.BackupConfigSummary{
			HasRustic: true,
		}
	}

	if server.cfg.Secrets != nil {
		response.Secrets = &controllerv1.SecretsConfigSummary{
			Provider:     server.cfg.Secrets.Provider,
			HasIdentity:  server.cfg.Secrets.IdentityFile != "",
			HasRecipient: server.cfg.Secrets.RecipientFile != "",
		}
	}

	return connect.NewResponse(response), nil
}

func (server *serviceServer) ListServices(ctx context.Context, req *connect.Request[controllerv1.ListServicesRequest]) (*connect.Response[controllerv1.ListServicesResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListServicesRequest{}
	}

	services, totalCount, err := server.db.ListDeclaredServices(ctx, req.Msg.GetRuntimeStatus(), req.Msg.GetPage(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.ListServicesResponse{
		Services:   make([]*controllerv1.ServiceSummary, 0, len(services)),
		TotalCount: totalCount,
	}
	for _, service := range services {
		response.Services = append(response.Services, &controllerv1.ServiceSummary{
			Name:            service.Name,
			IsDeclared:      service.IsDeclared,
			RuntimeStatus:   service.RuntimeStatus,
			UpdatedAt:       service.UpdatedAt,
			InstanceCount:   service.InstanceCount,
			RunningCount:    service.RunningCount,
			TargetNodeCount: service.TargetNodeCount,
		})
	}

	return connect.NewResponse(response), nil
}

func (server *serviceServer) GetService(ctx context.Context, req *connect.Request[controllerv1.GetServiceRequest]) (*connect.Response[controllerv1.GetServiceResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	snapshot, err := server.db.GetServiceSnapshot(ctx, service.Name)
	if err != nil {
		if errors.Is(err, store.ErrServiceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	instances, err := server.db.ListServiceInstances(ctx, service.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetServiceResponse{
		Name:          service.Name,
		RuntimeStatus: snapshot.RuntimeStatus,
		UpdatedAt:     snapshot.UpdatedAt,
		Nodes:         append([]string(nil), service.TargetNodes...),
		Enabled:       service.Enabled,
		Directory:     filepath.ToSlash(mustRelativeServiceDir(server.cfg.RepoDir, service.Directory)),
		Instances:     make([]*controllerv1.ServiceInstanceDetail, 0, len(instances)),
	}
	for _, instance := range instances {
		detail, err := buildServiceInstanceDetail(ctx, server.db, server.cfg, server.taskQueue, server.taskResults, service, instance)
		if err != nil {
			return nil, err
		}
		response.Instances = append(response.Instances, detail)
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) GetServiceTasks(ctx context.Context, req *connect.Request[controllerv1.GetServiceTasksRequest]) (*connect.Response[controllerv1.GetServiceTasksResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if _, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	tasks, totalCount, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), req.Msg.GetServiceName(), "", "", req.Msg.GetPage(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetServiceTasksResponse{
		Tasks:      make([]*controllerv1.TaskSummary, 0, len(tasks)),
		TotalCount: totalCount,
	}
	for _, record := range tasks {
		response.Tasks = append(response.Tasks, taskSummaryMessage(record))
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) GetServiceBackups(ctx context.Context, req *connect.Request[controllerv1.GetServiceBackupsRequest]) (*connect.Response[controllerv1.GetServiceBackupsResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if _, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	backups, totalCount, err := server.db.ListBackups(ctx, req.Msg.GetServiceName(), req.Msg.GetStatus(), req.Msg.GetDataName(), req.Msg.GetPage(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetServiceBackupsResponse{
		Backups:    make([]*controllerv1.BackupSummary, 0, len(backups)),
		TotalCount: totalCount,
	}
	for _, backup := range backups {
		response.Backups = append(response.Backups, backupSummaryMessage(backup))
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) RunServiceAction(ctx context.Context, req *connect.Request[controllerv1.RunServiceActionRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
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
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("action is required"))
	}

	createdTask, err := server.createServiceTaskWithOptions(ctx, req.Msg.GetServiceName(), nodeIDs, taskType, dataNames, serviceTaskCreateOptions{Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(taskActionResponse(createdTask)), nil
}

type serviceTaskCreateOptions struct {
	AttemptOfTaskID string
	Source          task.Source
}

func (server *serviceServer) createServiceTask(ctx context.Context, serviceName string, nodeIDs []string, taskType task.Type, dataNames []string) (task.Record, error) {
	return server.createServiceTaskWithOptions(ctx, serviceName, nodeIDs, taskType, dataNames, serviceTaskCreateOptions{})
}

func (server *serviceServer) createServiceTaskWithOptions(ctx context.Context, serviceName string, nodeIDs []string, taskType task.Type, dataNames []string, options serviceTaskCreateOptions) (task.Record, error) {
	if serviceName == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}

	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, serviceName)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeNotFound, err)
	}
	targetNodeIDs, err := resolveTargetNodeIDs(service, nodeIDs)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if len(targetNodeIDs) == 0 {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not have any target nodes", serviceName))
	}
	for _, nodeID := range targetNodeIDs {
		if err := validateTaskTargetNode(ctx, server.db, server.cfg, nodeID, taskType); err != nil {
			return task.Record{}, err
		}
		active, err := server.db.HasActiveServiceInstanceTask(ctx, serviceName, nodeID)
		if err != nil {
			return task.Record{}, connect.NewError(connect.CodeInternal, err)
		}
		if active {
			return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service instance %q@%q already has an active task", serviceName, nodeID))
		}
	}
	repoRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
	}
	paramsJSON, err := json.Marshal(serviceTaskParams{ServiceDir: serviceDir, DataNames: dataNames})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("encode task params: %w", err))
	}
	taskSource := options.Source
	if taskSource == "" {
		taskSource = task.SourceCLI
	}
	createdTasks := make([]task.Record, 0, len(targetNodeIDs))
	for _, nodeID := range targetNodeIDs {
		taskID := uuid.NewString()
		createdTask, err := server.db.CreateTask(ctx, task.Record{
			TaskID:          taskID,
			Type:            taskType,
			Source:          taskSource,
			TriggeredBy:     triggeredBy,
			ServiceName:     service.Name,
			NodeID:          nodeID,
			ParamsJSON:      string(paramsJSON),
			RepoRevision:    repoRevision,
			AttemptOfTaskID: options.AttemptOfTaskID,
			LogPath:         filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
		})
		if err != nil {
			return task.Record{}, connect.NewError(connect.CodeInternal, err)
		}
		if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
			return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
		}
		createdTasks = append(createdTasks, createdTask)
	}
	notifyTaskQueue(server.taskQueue)
	return createdTasks[0], nil
}

func createServiceTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, serviceName string, nodeIDs []string, taskType task.Type, dataNames []string) (task.Record, error) {
	return (&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs}).createServiceTask(ctx, serviceName, nodeIDs, taskType, dataNames)
}

func createServiceTaskWithOptions(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, serviceName string, nodeIDs []string, taskType task.Type, dataNames []string, options serviceTaskCreateOptions) (task.Record, error) {
	return (&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs}).createServiceTaskWithOptions(ctx, serviceName, nodeIDs, taskType, dataNames, options)
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

func taskParams(paramsJSON string) serviceTaskParams {
	if paramsJSON == "" {
		return serviceTaskParams{}
	}
	var params serviceTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return serviceTaskParams{}
	}
	return params
}

func (server *nodeServer) ListNodes(ctx context.Context, _ *connect.Request[controllerv1.ListNodesRequest]) (*connect.Response[controllerv1.ListNodesResponse], error) {
	snapshots, err := server.db.ListNodeSnapshots(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	snapshotByNodeID := make(map[string]store.NodeSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		snapshotByNodeID[snapshot.NodeID] = snapshot
	}

	response := &controllerv1.ListNodesResponse{
		Nodes: make([]*controllerv1.NodeSummary, 0, len(server.cfg.Nodes)),
	}
	for _, node := range server.cfg.Nodes {
		response.Nodes = append(response.Nodes, nodeSummary(node, snapshotByNodeID[node.ID]))
	}

	return connect.NewResponse(response), nil
}

func (server *nodeServer) GetNode(ctx context.Context, req *connect.Request[controllerv1.GetNodeRequest]) (*connect.Response[controllerv1.GetNodeResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	snapshots, err := server.db.ListNodeSnapshots(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	snapshotByNodeID := make(map[string]store.NodeSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		snapshotByNodeID[snapshot.NodeID] = snapshot
	}
	for _, node := range server.cfg.Nodes {
		if node.ID == req.Msg.GetNodeId() {
			return connect.NewResponse(&controllerv1.GetNodeResponse{Node: nodeSummary(node, snapshotByNodeID[node.ID])}), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %q is not configured", req.Msg.GetNodeId()))
}

func (server *nodeServer) GetNodeTasks(ctx context.Context, req *connect.Request[controllerv1.GetNodeTasksRequest]) (*connect.Response[controllerv1.GetNodeTasksResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	configured := false
	for _, node := range server.cfg.Nodes {
		if node.ID == req.Msg.GetNodeId() {
			configured = true
			break
		}
	}
	if !configured {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %q is not configured", req.Msg.GetNodeId()))
	}
	tasks, totalCount, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), "", req.Msg.GetNodeId(), "", req.Msg.GetPage(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetNodeTasksResponse{
		Tasks:      make([]*controllerv1.TaskSummary, 0, len(tasks)),
		TotalCount: totalCount,
	}
	for _, record := range tasks {
		response.Tasks = append(response.Tasks, taskSummaryMessage(record))
	}
	return connect.NewResponse(response), nil
}

func (server *nodeServer) GetNodeDockerStats(ctx context.Context, req *connect.Request[controllerv1.GetNodeDockerStatsRequest]) (*connect.Response[controllerv1.GetNodeDockerStatsResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	stats, err := server.db.GetNodeDockerStats(ctx, req.Msg.GetNodeId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&controllerv1.GetNodeDockerStatsResponse{
		Stats: &controllerv1.DockerStats{
			ContainersTotal:     stats.ContainersTotal,
			ContainersRunning:   stats.ContainersRunning,
			ContainersStopped:   stats.ContainersStopped,
			ContainersPaused:    stats.ContainersPaused,
			Images:              stats.Images,
			Networks:            stats.Networks,
			Volumes:             stats.Volumes,
			VolumesSizeBytes:    stats.VolumesSizeBytes,
			DisksUsageBytes:     stats.DisksUsageBytes,
			DockerServerVersion: stats.DockerServerVersion,
		},
	}), nil
}

func (server *nodeServer) ReloadNodeCaddy(ctx context.Context, req *connect.Request[controllerv1.ReloadNodeCaddyRequest]) (*connect.Response[controllerv1.ReloadNodeCaddyResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	createdTask, err := createNodeCaddyReloadTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), req.Msg.GetNodeId(), requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}
	createdTask.TriggeredBy, _ = rpcutil.BearerSubject(ctx)

	notifyTaskQueue(server.taskQueue)

	return connect.NewResponse(&controllerv1.ReloadNodeCaddyResponse{TaskId: createdTask.TaskID}), nil
}

func (server *nodeServer) SyncNodeCaddyFiles(ctx context.Context, req *connect.Request[controllerv1.SyncNodeCaddyFilesRequest]) (*connect.Response[controllerv1.SyncNodeCaddyFilesResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	createdTask, err := createNodeCaddySyncTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), req.Msg.GetNodeId(), req.Msg.GetServiceName(), req.Msg.GetFullRebuild(), requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&controllerv1.SyncNodeCaddyFilesResponse{TaskId: createdTask.TaskID}), nil
}

func (server *nodeServer) PruneNodeDocker(ctx context.Context, req *connect.Request[controllerv1.PruneNodeDockerRequest]) (*connect.Response[controllerv1.PruneNodeDockerResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	target := req.Msg.GetTarget()
	if target == "" {
		target = "all"
	}

	snapshot, err := server.db.GetNodeSnapshot(ctx, req.Msg.GetNodeId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !snapshot.IsOnline {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q is offline", req.Msg.GetNodeId()))
	}

	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	paramsJSON := fmt.Sprintf(`{"target":%q}`, target)

	_, err = server.db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        task.TypePrune,
		Source:      requestTaskSource(req.Header()),
		TriggeredBy: triggeredBy,
		NodeID:      req.Msg.GetNodeId(),
		Status:      task.StatusPending,
		ParamsJSON:  paramsJSON,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&controllerv1.PruneNodeDockerResponse{
		TaskId: taskID,
	}), nil
}

func (server *nodeServer) PruneNodeRustic(ctx context.Context, req *connect.Request[controllerv1.PruneNodeRusticRequest]) (*connect.Response[controllerv1.PruneNodeRusticResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}
	nodeID := req.Msg.GetNodeId()
	var err error
	if nodeID == "" {
		nodeID, err = chooseRusticMainNode(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), task.TypeRusticPrune)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	createdTask, err := createNodeRusticMaintenanceTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), nodeID, task.TypeRusticPrune, rusticPruneTaskParams{ServiceName: req.Msg.GetServiceName(), DataName: req.Msg.GetDataName()}, requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&controllerv1.PruneNodeRusticResponse{TaskId: createdTask.TaskID}), nil
}

func (server *nodeServer) ForgetNodeRustic(ctx context.Context, req *connect.Request[controllerv1.ForgetNodeRusticRequest]) (*connect.Response[controllerv1.ForgetNodeRusticResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}
	nodeID := req.Msg.GetNodeId()
	var err error
	if nodeID == "" {
		nodeID, err = chooseRusticMainNode(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), task.TypeRusticForget)
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	createdTask, err := createNodeRusticMaintenanceTask(ctx, server.db, server.cfg, configuredNodeIDs(server.cfg), nodeID, task.TypeRusticForget, rusticPruneTaskParams{ServiceName: req.Msg.GetServiceName(), DataName: req.Msg.GetDataName()}, requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&controllerv1.ForgetNodeRusticResponse{TaskId: createdTask.TaskID}), nil
}

func (server *nodeServer) ListNodeContainers(ctx context.Context, req *connect.Request[controllerv1.ListNodeContainersRequest]) (*connect.Response[controllerv1.ListNodeContainersResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	result, err := server.executeDockerListTask(ctx, req.Header(), req.Msg.GetNodeId(), "containers")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.ListNodeContainersResponse{
		Containers: result.Containers,
	}), nil
}

func (server *nodeServer) InspectNodeContainer(ctx context.Context, req *connect.Request[controllerv1.InspectNodeContainerRequest]) (*connect.Response[controllerv1.InspectNodeContainerResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetContainerId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and container_id are required"))
	}

	result, err := server.executeDockerInspectTask(ctx, req.Header(), req.Msg.GetNodeId(), "container", req.Msg.GetContainerId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.InspectNodeContainerResponse{
		RawJson: result.RawJSON,
	}), nil
}

func (server *nodeServer) ListNodeNetworks(ctx context.Context, req *connect.Request[controllerv1.ListNodeNetworksRequest]) (*connect.Response[controllerv1.ListNodeNetworksResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	result, err := server.executeDockerListTask(ctx, req.Header(), req.Msg.GetNodeId(), "networks")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.ListNodeNetworksResponse{
		Networks: result.Networks,
	}), nil
}

func (server *nodeServer) InspectNodeNetwork(ctx context.Context, req *connect.Request[controllerv1.InspectNodeNetworkRequest]) (*connect.Response[controllerv1.InspectNodeNetworkResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetNetworkId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and network_id are required"))
	}

	result, err := server.executeDockerInspectTask(ctx, req.Header(), req.Msg.GetNodeId(), "network", req.Msg.GetNetworkId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.InspectNodeNetworkResponse{
		RawJson: result.RawJSON,
	}), nil
}

func (server *nodeServer) ListNodeVolumes(ctx context.Context, req *connect.Request[controllerv1.ListNodeVolumesRequest]) (*connect.Response[controllerv1.ListNodeVolumesResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	result, err := server.executeDockerListTask(ctx, req.Header(), req.Msg.GetNodeId(), "volumes")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.ListNodeVolumesResponse{
		Volumes: result.Volumes,
	}), nil
}

func (server *nodeServer) InspectNodeVolume(ctx context.Context, req *connect.Request[controllerv1.InspectNodeVolumeRequest]) (*connect.Response[controllerv1.InspectNodeVolumeResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetVolumeName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and volume_name are required"))
	}

	result, err := server.executeDockerInspectTask(ctx, req.Header(), req.Msg.GetNodeId(), "volume", req.Msg.GetVolumeName())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.InspectNodeVolumeResponse{
		RawJson: result.RawJSON,
	}), nil
}

func (server *nodeServer) ListNodeImages(ctx context.Context, req *connect.Request[controllerv1.ListNodeImagesRequest]) (*connect.Response[controllerv1.ListNodeImagesResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}

	result, err := server.executeDockerListTask(ctx, req.Header(), req.Msg.GetNodeId(), "images")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.ListNodeImagesResponse{
		Images: result.Images,
	}), nil
}

func (server *nodeServer) InspectNodeImage(ctx context.Context, req *connect.Request[controllerv1.InspectNodeImageRequest]) (*connect.Response[controllerv1.InspectNodeImageResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetImageId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and image_id are required"))
	}

	result, err := server.executeDockerInspectTask(ctx, req.Header(), req.Msg.GetNodeId(), "image", req.Msg.GetImageId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&controllerv1.InspectNodeImageResponse{
		RawJson: result.RawJSON,
	}), nil
}

type dockerListResult struct {
	Containers []*controllerv1.ContainerInfo `json:"containers,omitempty"`
	Networks   []*controllerv1.NetworkInfo   `json:"networks,omitempty"`
	Volumes    []*controllerv1.VolumeInfo    `json:"volumes,omitempty"`
	Images     []*controllerv1.ImageInfo     `json:"images,omitempty"`
	RawJSON    string                        `json:"raw_json,omitempty"`
	Content    string                        `json:"content,omitempty"`
}

const (
	dockerTaskResultBegin = "COMPOSIA_DOCKER_RESULT_BEGIN"
	dockerTaskResultEnd   = "COMPOSIA_DOCKER_RESULT_END"
)

func (server *nodeServer) executeDockerListTask(ctx context.Context, header http.Header, nodeID, resource string) (*dockerListResult, error) {
	snapshot, err := server.db.GetNodeSnapshot(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !snapshot.IsOnline {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q is offline", nodeID))
	}

	taskID := uuid.NewString()
	paramsJSON := fmt.Sprintf(`{"action":"list","resource":%q}`, resource)

	createdTask, err := server.db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        task.TypeDockerList,
		Source:      requestTaskSource(header),
		TriggeredBy: "controller",
		NodeID:      nodeID,
		Status:      task.StatusPending,
		ParamsJSON:  paramsJSON,
		LogPath:     filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}

	notifyTaskQueue(server.taskQueue)

	detail, err := server.waitForTaskCompletion(ctx, taskID, 30*time.Second)
	if err != nil {
		return nil, err
	}
	result, err := readTaskLog(detail)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return server.parseDockerListResult(resource, result)
}

func (server *nodeServer) executeDockerInspectTask(ctx context.Context, header http.Header, nodeID, resource, id string) (*dockerListResult, error) {
	snapshot, err := server.db.GetNodeSnapshot(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !snapshot.IsOnline {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q is offline", nodeID))
	}

	taskID := uuid.NewString()
	paramsJSON := fmt.Sprintf(`{"action":"inspect","resource":%q,"id":%q}`, resource, id)

	createdTask, err := server.db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        task.TypeDockerInspect,
		Source:      requestTaskSource(header),
		TriggeredBy: "controller",
		NodeID:      nodeID,
		Status:      task.StatusPending,
		ParamsJSON:  paramsJSON,
		LogPath:     filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}

	notifyTaskQueue(server.taskQueue)

	detail, err := server.waitForTaskCompletion(ctx, taskID, 30*time.Second)
	if err != nil {
		return nil, err
	}
	result, err := readTaskLog(detail)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	payload, err := extractDockerTaskResult(result)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (server *containerServer) RunContainerAction(ctx context.Context, req *connect.Request[controllerv1.RunContainerActionRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id, container_id, and action are required"))
	}

	var taskType task.Type
	var action string
	switch req.Msg.GetAction() {
	case controllerv1.ContainerAction_CONTAINER_ACTION_START:
		taskType = task.TypeDockerStart
		action = "start"
	case controllerv1.ContainerAction_CONTAINER_ACTION_STOP:
		taskType = task.TypeDockerStop
		action = "stop"
	case controllerv1.ContainerAction_CONTAINER_ACTION_RESTART:
		taskType = task.TypeDockerRestart
		action = "restart"
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("action is required"))
	}

	record, err := server.createContainerTask(ctx, req.Header(), req.Msg.GetNodeId(), req.Msg.GetContainerId(), taskType, map[string]any{"action": action, "resource": "container", "id": req.Msg.GetContainerId()})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(record)), nil
}

func (server *containerServer) GetContainerLogs(ctx context.Context, req *connect.Request[controllerv1.GetContainerLogsRequest]) (*connect.Response[controllerv1.GetContainerLogsResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetContainerId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and container_id are required"))
	}
	detail, err := server.executeContainerLogsTask(ctx, req.Header(), req.Msg.GetNodeId(), req.Msg.GetContainerId(), req.Msg.GetTail(), req.Msg.GetTimestamps())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.GetContainerLogsResponse{Content: detail.Content}), nil
}

func (server *containerServer) createContainerTask(ctx context.Context, header http.Header, nodeID, containerID string, taskType task.Type, params map[string]any) (task.Record, error) {
	if nodeID == "" || containerID == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and container_id are required"))
	}
	if err := validateTaskTargetNode(ctx, server.db, server.cfg, nodeID, taskType); err != nil {
		return task.Record{}, err
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal params: %w", err))
	}
	triggeredBy, _ := rpcutil.BearerSubject(ctx)
	taskID := uuid.NewString()
	createdTask, err := server.db.CreateTask(ctx, task.Record{
		TaskID:      taskID,
		Type:        taskType,
		Source:      requestTaskSource(header),
		TriggeredBy: triggeredBy,
		NodeID:      nodeID,
		Status:      task.StatusPending,
		ParamsJSON:  string(paramsJSON),
		LogPath:     filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	notifyTaskQueue(server.taskQueue)
	return createdTask, nil
}

func (server *containerServer) executeContainerLogsTask(ctx context.Context, header http.Header, nodeID, containerID, tail string, timestamps bool) (*dockerListResult, error) {
	createdTask, err := server.createContainerTask(ctx, header, nodeID, containerID, task.TypeDockerLogs, map[string]any{
		"action":     "logs",
		"resource":   "container",
		"id":         containerID,
		"tail":       tail,
		"timestamps": timestamps,
	})
	if err != nil {
		return nil, err
	}
	detail, err := server.waitForTaskCompletion(ctx, createdTask.TaskID, 30*time.Second)
	if err != nil {
		return nil, connect.NewError(connect.CodeDeadlineExceeded, err)
	}
	result, err := readTaskLog(detail)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	payload, err := extractDockerTaskResult(result)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return payload, nil
}

func (server *containerServer) waitForTaskCompletion(ctx context.Context, taskID string, timeout time.Duration) (store.TaskDetail, error) {
	deadline := time.Now().Add(timeout)
	waitCh := server.taskResults.Subscribe(taskID)
	defer server.taskResults.Unsubscribe(taskID, waitCh)

	for {
		detail, err := server.db.GetTask(ctx, taskID)
		if err == nil {
			if detail.Record.Status == task.StatusSucceeded {
				return detail, nil
			}
			if detail.Record.Status == task.StatusFailed {
				return store.TaskDetail{}, fmt.Errorf("task failed: %s", detail.Record.ErrorSummary)
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return store.TaskDetail{}, fmt.Errorf("timeout waiting for task result")
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return store.TaskDetail{}, ctx.Err()
		case <-waitCh:
			timer.Stop()
		case <-timer.C:
			return store.TaskDetail{}, fmt.Errorf("timeout waiting for task result")
		}
	}
}

func (server *nodeServer) waitForTaskCompletion(ctx context.Context, taskID string, timeout time.Duration) (store.TaskDetail, error) {
	deadline := time.Now().Add(timeout)
	waitCh := server.taskResults.Subscribe(taskID)
	defer server.taskResults.Unsubscribe(taskID, waitCh)

	for {
		detail, err := server.db.GetTask(ctx, taskID)
		if err == nil {
			if detail.Record.Status == task.StatusSucceeded {
				return detail, nil
			}
			if detail.Record.Status == task.StatusFailed {
				return store.TaskDetail{}, fmt.Errorf("task failed: %s", detail.Record.ErrorSummary)
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return store.TaskDetail{}, fmt.Errorf("timeout waiting for task result")
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return store.TaskDetail{}, ctx.Err()
		case <-waitCh:
			timer.Stop()
		case <-timer.C:
			return store.TaskDetail{}, fmt.Errorf("timeout waiting for task result")
		}
	}
}

func readTaskLog(detail store.TaskDetail) (string, error) {
	logContent, err := os.ReadFile(detail.Record.LogPath)
	if err != nil {
		return "", fmt.Errorf("read task log: %w", err)
	}
	return string(logContent), nil
}

func (server *nodeServer) parseDockerListResult(resource, logContent string) (*dockerListResult, error) {
	payload, err := extractDockerTaskResult(logContent)
	if err == nil {
		return payload, nil
	}

	return server.parseLegacyDockerListResult(resource, logContent)
}

func extractDockerTaskResult(logContent string) (*dockerListResult, error) {
	start := strings.Index(logContent, dockerTaskResultBegin)
	if start == -1 {
		return nil, fmt.Errorf("docker task result marker not found")
	}
	start += len(dockerTaskResultBegin)
	end := strings.Index(logContent[start:], dockerTaskResultEnd)
	if end == -1 {
		return nil, fmt.Errorf("docker task result end marker not found")
	}
	payload := strings.TrimSpace(logContent[start : start+end])
	if payload == "" {
		return nil, fmt.Errorf("docker task result payload is empty")
	}

	var result dockerListResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return nil, fmt.Errorf("decode docker task result: %w", err)
	}
	return &result, nil
}

func (server *nodeServer) parseLegacyDockerListResult(resource, logContent string) (*dockerListResult, error) {
	lines := strings.Split(strings.TrimSpace(logContent), "\n")
	result := &dockerListResult{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "docker task finished successfully" {
			continue
		}
		if strings.HasPrefix(line, "starting docker") || strings.HasPrefix(line, "docker") {
			continue
		}

		switch resource {
		case "containers":
			var psData struct {
				ID      string `json:"ID"`
				Names   string `json:"Names"`
				Image   string `json:"Image"`
				State   string `json:"State"`
				Status  string `json:"Status"`
				Created string `json:"CreatedAt"`
				Labels  string `json:"Labels"`
			}
			if err := json.Unmarshal([]byte(line), &psData); err == nil {
				labels := parseDockerLabels(psData.Labels)
				result.Containers = append(result.Containers, &controllerv1.ContainerInfo{
					Id:      psData.ID,
					Name:    psData.Names,
					Image:   psData.Image,
					State:   psData.State,
					Status:  psData.Status,
					Created: psData.Created,
					Labels:  labels,
				})
			}
		case "networks":
			var netData struct {
				ID         string `json:"ID"`
				Name       string `json:"Name"`
				Driver     string `json:"Driver"`
				Scope      string `json:"Scope"`
				Internal   string `json:"Internal"`
				Attachable string `json:"Attachable"`
				Labels     string `json:"Labels"`
				CreatedAt  string `json:"CreatedAt"`
			}
			if err := json.Unmarshal([]byte(line), &netData); err == nil {
				labels := parseDockerLabels(netData.Labels)
				result.Networks = append(result.Networks, &controllerv1.NetworkInfo{
					Id:         netData.ID,
					Name:       netData.Name,
					Driver:     netData.Driver,
					Scope:      netData.Scope,
					Internal:   netData.Internal == "true",
					Attachable: netData.Attachable == "true",
					Created:    netData.CreatedAt,
					Labels:     labels,
				})
			}
		case "volumes":
			var volData struct {
				Name       string `json:"Name"`
				Driver     string `json:"Driver"`
				Mountpoint string `json:"Mountpoint"`
				Scope      string `json:"Scope"`
				Labels     string `json:"Labels"`
			}
			if err := json.Unmarshal([]byte(line), &volData); err == nil {
				labels := parseDockerLabels(volData.Labels)
				result.Volumes = append(result.Volumes, &controllerv1.VolumeInfo{
					Name:       volData.Name,
					Driver:     volData.Driver,
					Mountpoint: volData.Mountpoint,
					Scope:      volData.Scope,
					Labels:     labels,
				})
			}
		case "images":
			var imgData struct {
				ID         string `json:"ID"`
				Repository string `json:"Repository"`
				Tag        string `json:"Tag"`
				Size       string `json:"Size"`
				CreatedAt  string `json:"CreatedAt"`
			}
			if err := json.Unmarshal([]byte(line), &imgData); err == nil {
				var repoTags []string
				if imgData.Repository != "<none>" && imgData.Tag != "<none>" {
					repoTags = append(repoTags, imgData.Repository+":"+imgData.Tag)
				}
				result.Images = append(result.Images, &controllerv1.ImageInfo{
					Id:       imgData.ID,
					RepoTags: repoTags,
					Created:  imgData.CreatedAt,
				})
			}
		}
	}

	return result, nil
}

func parseDockerLabels(labelsStr string) map[string]string {
	labels := make(map[string]string)
	if labelsStr == "" {
		return labels
	}
	parts := strings.Split(labelsStr, ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			labels[kv[0]] = kv[1]
		}
	}
	return labels
}

func nodeSummary(node config.NodeConfig, snapshot store.NodeSnapshot) *controllerv1.NodeSummary {
	displayName := node.DisplayName
	if displayName == "" {
		displayName = node.ID
	}
	return &controllerv1.NodeSummary{
		NodeId:        node.ID,
		DisplayName:   displayName,
		Enabled:       node.Enabled == nil || *node.Enabled,
		IsOnline:      snapshot.IsOnline,
		LastHeartbeat: snapshot.LastHeartbeat,
	}
}

func (server *taskServer) ListTasks(ctx context.Context, req *connect.Request[controllerv1.ListTasksRequest]) (*connect.Response[controllerv1.ListTasksResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListTasksRequest{}
	}

	tasks, totalCount, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), req.Msg.GetServiceName(), req.Msg.GetNodeId(), req.Msg.GetType(), req.Msg.GetPage(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.ListTasksResponse{
		Tasks:      make([]*controllerv1.TaskSummary, 0, len(tasks)),
		TotalCount: totalCount,
	}
	for _, record := range tasks {
		response.Tasks = append(response.Tasks, taskSummaryMessage(record))
	}

	return connect.NewResponse(response), nil
}

func taskSummaryMessage(record store.TaskSummary) *controllerv1.TaskSummary {
	return &controllerv1.TaskSummary{
		TaskId:      record.TaskID,
		Type:        record.Type,
		Status:      record.Status,
		ServiceName: record.ServiceName,
		NodeId:      record.NodeID,
		CreatedAt:   record.CreatedAt,
	}
}

func serviceInstanceSummaryMessage(record store.ServiceInstanceSnapshot) *controllerv1.ServiceInstanceSummary {
	return &controllerv1.ServiceInstanceSummary{
		ServiceName:   record.ServiceName,
		NodeId:        record.NodeID,
		RuntimeStatus: record.RuntimeStatus,
		UpdatedAt:     record.UpdatedAt,
		IsDeclared:    record.IsDeclared,
	}
}

func serviceContainerSummaryMessage(container *controllerv1.ContainerInfo) *controllerv1.ServiceContainerSummary {
	if container == nil {
		return nil
	}
	labels := container.GetLabels()
	return &controllerv1.ServiceContainerSummary{
		ContainerId:    container.GetId(),
		Name:           container.GetName(),
		Image:          container.GetImage(),
		State:          container.GetState(),
		Status:         container.GetStatus(),
		Created:        container.GetCreated(),
		ComposeProject: labels["com.docker.compose.project"],
		ComposeService: labels["com.docker.compose.service"],
	}
}

func serviceInstanceDetailMessage(record store.ServiceInstanceSnapshot, containers []*controllerv1.ServiceContainerSummary) *controllerv1.ServiceInstanceDetail {
	return &controllerv1.ServiceInstanceDetail{
		ServiceName:   record.ServiceName,
		NodeId:        record.NodeID,
		RuntimeStatus: record.RuntimeStatus,
		UpdatedAt:     record.UpdatedAt,
		IsDeclared:    record.IsDeclared,
		Containers:    containers,
	}
}

func buildServiceInstanceDetail(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, taskQueue *taskQueueNotifier, taskResults *taskResultNotifier, service repo.Service, instance store.ServiceInstanceSnapshot) (*controllerv1.ServiceInstanceDetail, error) {
	nodeServer := &nodeServer{db: db, cfg: cfg, taskQueue: taskQueue, taskResults: taskResults}
	containers, err := listServiceInstanceContainers(ctx, nodeServer, service, instance.NodeID)
	if err != nil {
		var connectErr *connect.Error
		if errors.As(err, &connectErr) {
			if connectErr.Code() == connect.CodeFailedPrecondition || connectErr.Code() == connect.CodeNotFound {
				return serviceInstanceDetailMessage(instance, nil), nil
			}
			return nil, err
		}
		return nil, err
	}
	return serviceInstanceDetailMessage(instance, containers), nil
}

func listServiceInstanceContainers(ctx context.Context, nodeServer *nodeServer, service repo.Service, nodeID string) ([]*controllerv1.ServiceContainerSummary, error) {
	if nodeServer == nil {
		return nil, nil
	}
	result, err := nodeServer.executeDockerListTask(ctx, make(http.Header), nodeID, "containers")
	if err != nil {
		return nil, err
	}
	projectName := service.Meta.ProjectName
	if projectName == "" {
		projectName = service.Name
	}
	items := make([]*controllerv1.ServiceContainerSummary, 0, len(result.Containers))
	for _, container := range result.Containers {
		labels := container.GetLabels()
		if labels["com.docker.compose.project"] != projectName {
			continue
		}
		items = append(items, serviceContainerSummaryMessage(container))
	}
	return items, nil
}

func (server *backupRecordServer) ListBackups(ctx context.Context, req *connect.Request[controllerv1.ListBackupsRequest]) (*connect.Response[controllerv1.ListBackupsResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListBackupsRequest{}
	}
	backups, totalCount, err := server.db.ListBackups(ctx, req.Msg.GetServiceName(), req.Msg.GetStatus(), req.Msg.GetDataName(), req.Msg.GetPage(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.ListBackupsResponse{
		Backups:    make([]*controllerv1.BackupSummary, 0, len(backups)),
		TotalCount: totalCount,
	}
	for _, backup := range backups {
		response.Backups = append(response.Backups, backupSummaryMessage(backup))
	}
	return connect.NewResponse(response), nil
}

func (server *backupRecordServer) GetBackup(ctx context.Context, req *connect.Request[controllerv1.GetBackupRequest]) (*connect.Response[controllerv1.GetBackupResponse], error) {
	if req.Msg == nil || req.Msg.GetBackupId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("backup_id is required"))
	}
	backup, err := server.db.GetBackup(ctx, req.Msg.GetBackupId())
	if err != nil {
		if errors.Is(err, store.ErrBackupNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetBackupResponse{
		BackupId:     backup.BackupID,
		TaskId:       backup.TaskID,
		ServiceName:  backup.ServiceName,
		DataName:     backup.DataName,
		Status:       backup.Status,
		StartedAt:    backup.StartedAt,
		FinishedAt:   backup.FinishedAt,
		ArtifactRef:  backup.ArtifactRef,
		ErrorSummary: backup.ErrorSummary,
	}
	return connect.NewResponse(response), nil
}

func backupSummaryMessage(backup store.BackupSummary) *controllerv1.BackupSummary {
	return &controllerv1.BackupSummary{
		BackupId:    backup.BackupID,
		TaskId:      backup.TaskID,
		ServiceName: backup.ServiceName,
		DataName:    backup.DataName,
		Status:      backup.Status,
		StartedAt:   backup.StartedAt,
		FinishedAt:  backup.FinishedAt,
	}
}

func (server *repoServer) GetRepoHead(ctx context.Context, _ *connect.Request[controllerv1.GetRepoHeadRequest]) (*connect.Response[controllerv1.GetRepoHeadResponse], error) {
	headRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	branch, err := repo.CurrentBranch(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cleanWorktree, err := repo.IsCleanWorkingTree(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	syncState, err := server.repoSyncState(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetRepoHeadResponse{
		HeadRevision:         headRevision,
		Branch:               branch,
		HasRemote:            server.hasConfiguredRemote(),
		CleanWorktree:        cleanWorktree,
		SyncStatus:           syncState.SyncStatus,
		LastSyncError:        syncState.LastSyncError,
		LastSuccessfulPullAt: syncState.LastSuccessfulPullAt,
	}
	return connect.NewResponse(response), nil
}

func (server *repoServer) ListRepoFiles(_ context.Context, req *connect.Request[controllerv1.ListRepoFilesRequest]) (*connect.Response[controllerv1.ListRepoFilesResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListRepoFilesRequest{}
	}
	entries, err := repo.ListFiles(server.cfg.RepoDir, req.Msg.GetPath())
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrRepoPathInvalid):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, repo.ErrRepoPathNotFound), errors.Is(err, repo.ErrRepoPathNotDirectory):
			return nil, connect.NewError(connect.CodeNotFound, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	response := &controllerv1.ListRepoFilesResponse{Entries: make([]*controllerv1.RepoFileEntry, 0, len(entries))}
	for _, entry := range entries {
		response.Entries = append(response.Entries, &controllerv1.RepoFileEntry{
			Path:  entry.Path,
			Name:  entry.Name,
			IsDir: entry.IsDir,
			Size:  entry.Size,
		})
	}
	return connect.NewResponse(response), nil
}

func (server *repoServer) GetRepoFile(_ context.Context, req *connect.Request[controllerv1.GetRepoFileRequest]) (*connect.Response[controllerv1.GetRepoFileResponse], error) {
	if req.Msg == nil || req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	file, err := repo.ReadFile(server.cfg.RepoDir, req.Msg.GetPath())
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrRepoPathInvalid):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, repo.ErrRepoPathNotFound), errors.Is(err, repo.ErrRepoPathNotFile):
			return nil, connect.NewError(connect.CodeNotFound, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	response := &controllerv1.GetRepoFileResponse{
		Path:    file.Path,
		Content: file.Content,
		Size:    file.Size,
	}
	return connect.NewResponse(response), nil
}

func (server *repoServer) ListRepoCommits(_ context.Context, req *connect.Request[controllerv1.ListRepoCommitsRequest]) (*connect.Response[controllerv1.ListRepoCommitsResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListRepoCommitsRequest{}
	}
	commits, nextCursor, err := repo.ListCommits(server.cfg.RepoDir, req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.ListRepoCommitsResponse{
		Commits:    make([]*controllerv1.RepoCommitSummary, 0, len(commits)),
		NextCursor: nextCursor,
	}
	for _, commit := range commits {
		response.Commits = append(response.Commits, &controllerv1.RepoCommitSummary{
			CommitId:    commit.CommitID,
			Subject:     commit.Subject,
			CommittedAt: commit.CommittedAt,
		})
	}
	return connect.NewResponse(response), nil
}

func (server *repoServer) ValidateRepo(_ context.Context, _ *connect.Request[controllerv1.ValidateRepoRequest]) (*connect.Response[controllerv1.ValidateRepoResponse], error) {
	availableNodeIDs := make(map[string]struct{}, len(server.cfg.Nodes))
	for _, node := range server.cfg.Nodes {
		availableNodeIDs[node.ID] = struct{}{}
	}
	validationErrors := repo.ValidateRepo(server.cfg.RepoDir, availableNodeIDs)
	response := &controllerv1.ValidateRepoResponse{Errors: make([]*controllerv1.RepoValidationError, 0, len(validationErrors))}
	for _, validationError := range validationErrors {
		response.Errors = append(response.Errors, &controllerv1.RepoValidationError{
			Path:    validationError.Path,
			Line:    validationError.Line,
			Message: validationError.Message,
		})
	}
	return connect.NewResponse(response), nil
}

func (server *repoServer) SyncRepo(ctx context.Context, _ *connect.Request[controllerv1.SyncRepoRequest]) (*connect.Response[controllerv1.SyncRepoResponse], error) {
	if !server.hasConfiguredRemote() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("repo remote sync is not configured"))
	}
	server.repoLock().Lock()
	defer server.repoLock().Unlock()

	if _, err := server.syncRepoLocked(ctx); err != nil {
		return nil, err
	}
	if err := server.refreshDeclaredServices(ctx); err != nil {
		return nil, err
	}
	headRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	branch, err := repo.CurrentBranch(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	syncState, err := server.repoSyncState(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controllerv1.SyncRepoResponse{
		HeadRevision:         headRevision,
		Branch:               branch,
		SyncStatus:           syncState.SyncStatus,
		LastSyncError:        syncState.LastSyncError,
		LastSuccessfulPullAt: syncState.LastSuccessfulPullAt,
	}), nil
}

func (server *repoServer) UpdateRepoFile(ctx context.Context, req *connect.Request[controllerv1.UpdateRepoFileRequest]) (*connect.Response[controllerv1.UpdateRepoFileResponse], error) {
	if req.Msg == nil || req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	result, err := server.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetPath()}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return server.updateRepoFileTransaction(ctx, req.Msg.GetPath(), req.Msg.GetContent(), req.Msg.GetCommitMessage(), baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.UpdateRepoFileResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *repoServer) CreateRepoDirectory(ctx context.Context, req *connect.Request[controllerv1.CreateRepoDirectoryRequest]) (*connect.Response[controllerv1.CreateRepoDirectoryResponse], error) {
	if req.Msg == nil || req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	result, err := server.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetPath()}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return server.createRepoDirectoryTransaction(ctx, req.Msg.GetPath(), req.Msg.GetCommitMessage(), baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.CreateRepoDirectoryResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *repoServer) MoveRepoPath(ctx context.Context, req *connect.Request[controllerv1.MoveRepoPathRequest]) (*connect.Response[controllerv1.MoveRepoPathResponse], error) {
	if req.Msg == nil || req.Msg.GetSourcePath() == "" || req.Msg.GetDestinationPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_path and destination_path are required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	result, err := server.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetSourcePath(), req.Msg.GetDestinationPath()}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return server.moveRepoPathTransaction(ctx, req.Msg.GetSourcePath(), req.Msg.GetDestinationPath(), req.Msg.GetCommitMessage(), baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.MoveRepoPathResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *repoServer) DeleteRepoPath(ctx context.Context, req *connect.Request[controllerv1.DeleteRepoPathRequest]) (*connect.Response[controllerv1.DeleteRepoPathResponse], error) {
	if req.Msg == nil || req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	result, err := server.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetPath()}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return server.deleteRepoPathTransaction(ctx, req.Msg.GetPath(), req.Msg.GetCommitMessage(), baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.DeleteRepoPathResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *repoServer) repoLock() *sync.Mutex {
	if server.repoMu == nil {
		server.repoMu = &sync.Mutex{}
	}
	return server.repoMu
}

func (server *repoServer) runRepoWrite(ctx context.Context, baseRevision string, relativePaths []string, run func(baseSyncState store.RepoSyncState) (repoWriteResult, error)) (repoWriteResult, error) {
	server.repoLock().Lock()
	defer server.repoLock().Unlock()

	baseSyncState, err := server.prepareRepoWritePaths(ctx, baseRevision, relativePaths)
	if err != nil {
		return repoWriteResult{}, err
	}
	return run(baseSyncState)
}

func (server *repoServer) hasConfiguredRemote() bool {
	return server.cfg != nil && server.cfg.Git != nil && strings.TrimSpace(server.cfg.Git.RemoteURL) != ""
}

func (server *repoServer) repoSyncState(ctx context.Context) (store.RepoSyncState, error) {
	if !server.hasConfiguredRemote() {
		return store.RepoSyncState{SyncStatus: store.RepoSyncStatusLocalOnly}, nil
	}
	if server.db == nil {
		return store.RepoSyncState{SyncStatus: store.RepoSyncStatusUnknown}, nil
	}
	state, err := server.db.GetRepoSyncState(ctx)
	if err != nil {
		return store.RepoSyncState{}, err
	}
	if state.SyncStatus == "" {
		state.SyncStatus = store.RepoSyncStatusUnknown
	}
	return state, nil
}

func (server *repoServer) configuredRemoteBranch() (string, error) {
	if server.cfg != nil && server.cfg.Git != nil && strings.TrimSpace(server.cfg.Git.Branch) != "" {
		return strings.TrimSpace(server.cfg.Git.Branch), nil
	}
	branch, err := repo.CurrentBranch(server.cfg.RepoDir)
	if err != nil {
		return "", err
	}
	if branch == "" {
		return "", fmt.Errorf("cannot determine repo branch for remote sync")
	}
	return branch, nil
}

func (server *repoServer) configuredGitAuthToken() (string, error) {
	if server.cfg == nil || server.cfg.Git == nil || server.cfg.Git.Auth == nil || strings.TrimSpace(server.cfg.Git.Auth.TokenFile) == "" {
		return "", nil
	}
	content, err := os.ReadFile(strings.TrimSpace(server.cfg.Git.Auth.TokenFile))
	if err != nil {
		return "", fmt.Errorf("read git auth token file: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

func (server *repoServer) persistRepoSyncState(ctx context.Context, state store.RepoSyncState) error {
	if server.db == nil {
		return nil
	}
	return server.db.UpsertRepoSyncState(ctx, state)
}

func (server *repoServer) ensureCleanWorktree() error {
	cleanWorktree, err := repo.IsCleanWorkingTree(server.cfg.RepoDir)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !cleanWorktree {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("repo working tree is not clean"))
	}
	return nil
}

func (server *repoServer) syncRepoLocked(ctx context.Context) (store.RepoSyncState, error) {
	if !server.hasConfiguredRemote() {
		return store.RepoSyncState{SyncStatus: store.RepoSyncStatusLocalOnly}, connect.NewError(connect.CodeFailedPrecondition, errors.New("repo remote sync is not configured"))
	}
	if err := server.ensureCleanWorktree(); err != nil {
		return store.RepoSyncState{}, err
	}
	branch, err := server.configuredRemoteBranch()
	if err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	authToken, err := server.configuredGitAuthToken()
	if err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	previousState, err := server.repoSyncState(ctx)
	if err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	pulledAt := time.Now().UTC().Format(time.RFC3339)
	if err := repo.FetchAndFastForward(server.cfg.RepoDir, strings.TrimSpace(server.cfg.Git.RemoteURL), branch, authToken); err != nil {
		state := store.RepoSyncState{
			SyncStatus:           store.RepoSyncStatusPullFailed,
			LastSyncError:        err.Error(),
			LastSuccessfulPullAt: previousState.LastSuccessfulPullAt,
		}
		if persistErr := server.persistRepoSyncState(ctx, state); persistErr != nil {
			return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, persistErr)
		}
		return state, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	state := store.RepoSyncState{
		SyncStatus:           store.RepoSyncStatusSynced,
		LastSyncError:        "",
		LastSuccessfulPullAt: pulledAt,
	}
	if err := server.persistRepoSyncState(ctx, state); err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	return state, nil
}

func (server *repoServer) prepareRepoWrite(ctx context.Context, baseRevision, relativePath string) (store.RepoSyncState, error) {
	return server.prepareRepoWritePaths(ctx, baseRevision, []string{relativePath})
}

func (server *repoServer) prepareRepoWritePaths(ctx context.Context, baseRevision string, relativePaths []string) (store.RepoSyncState, error) {
	if err := server.syncRepoBeforeWrite(ctx); err != nil {
		return store.RepoSyncState{}, err
	}
	if err := server.verifyRepoWriteBaseRevision(baseRevision); err != nil {
		return store.RepoSyncState{}, err
	}
	if err := server.verifyRepoWriteAllowed(ctx, relativePaths...); err != nil {
		return store.RepoSyncState{}, err
	}
	return server.repoSyncState(ctx)
}

func (server *repoServer) syncRepoBeforeWrite(ctx context.Context) error {
	if !server.hasConfiguredRemote() {
		return nil
	}
	_, err := server.syncRepoLocked(ctx)
	return err
}

func (server *repoServer) verifyRepoWriteBaseRevision(baseRevision string) error {
	currentRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if currentRevision != baseRevision {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("base_revision %q does not match current HEAD %q", baseRevision, currentRevision))
	}
	return nil
}

func (server *repoServer) verifyRepoWriteAllowed(ctx context.Context, relativePaths ...string) error {
	if err := server.ensureCleanWorktree(); err != nil {
		return err
	}
	return server.ensureRepoPathsUnlocked(ctx, relativePaths...)
}

func (server *repoServer) refreshDeclaredServices(ctx context.Context) error {
	if server.db == nil {
		return nil
	}
	services, err := repo.DiscoverServices(server.cfg.RepoDir, server.availableNodeIDs)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	declaredServices := make(map[string][]string, len(services))
	for _, service := range services {
		declaredServices[service.Name] = append([]string(nil), service.TargetNodes...)
	}
	if err := server.db.SyncDeclaredServices(ctx, declaredServices); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	return nil
}

func (server *repoServer) ensureRepoPathUnlocked(ctx context.Context, relativePath string) error {
	return server.ensureRepoPathsUnlocked(ctx, relativePath)
}

func (server *repoServer) ensureRepoPathsUnlocked(ctx context.Context, relativePaths ...string) error {
	if server.db == nil {
		return nil
	}
	services, err := repo.DiscoverServices(server.cfg.RepoDir, server.availableNodeIDs)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, relativePath := range relativePaths {
		cleanPath := filepath.ToSlash(filepath.Clean(relativePath))
		for _, service := range services {
			serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
			}
			serviceDir = filepath.ToSlash(filepath.Clean(serviceDir))
			if !pathHitsServiceDir(cleanPath, serviceDir) {
				continue
			}
			active, err := server.db.HasActiveServiceTask(ctx, service.Name)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if active {
				return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q has an active task", service.Name))
			}
		}
	}
	return nil
}

func pathHitsServiceDir(targetPath, serviceDir string) bool {
	if targetPath == serviceDir {
		return true
	}
	return strings.HasPrefix(targetPath, serviceDir+"/")
}

func (server *repoServer) updateRepoFileTransaction(ctx context.Context, relativePath, content, commitMessage string, baseSyncState store.RepoSyncState) (_ repoWriteResult, retErr error) {
	previous, readErr := repo.ReadFile(server.cfg.RepoDir, relativePath)
	fileExisted := readErr == nil
	committed := false
	if readErr != nil && !errors.Is(readErr, repo.ErrRepoPathNotFound) {
		switch {
		case errors.Is(readErr, repo.ErrRepoPathInvalid), errors.Is(readErr, repo.ErrRepoPathProtected):
			return repoWriteResult{}, connect.NewError(connect.CodeInvalidArgument, readErr)
		case errors.Is(readErr, repo.ErrRepoPathNotFile):
			return repoWriteResult{}, connect.NewError(connect.CodeFailedPrecondition, readErr)
		default:
			return repoWriteResult{}, connect.NewError(connect.CodeInternal, readErr)
		}
	}
	writtenPath, err := repo.WriteFile(server.cfg.RepoDir, relativePath, content)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrRepoPathInvalid), errors.Is(err, repo.ErrRepoPathProtected):
			return repoWriteResult{}, connect.NewError(connect.CodeInvalidArgument, err)
		default:
			return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
		}
	}
	defer func() {
		if retErr == nil || committed {
			return
		}
		if fileExisted {
			_, _ = repo.WriteFile(server.cfg.RepoDir, writtenPath, previous.Content)
		} else {
			absolutePath := filepath.Join(server.cfg.RepoDir, filepath.FromSlash(writtenPath))
			_ = os.Remove(absolutePath)
		}
	}()
	authorName := ""
	authorEmail := ""
	if server.cfg.Git != nil {
		authorName = server.cfg.Git.AuthorName
		authorEmail = server.cfg.Git.AuthorEmail
	}
	commitID, err := repo.CommitPath(server.cfg.RepoDir, writtenPath, commitMessage, authorName, authorEmail)
	if err != nil {
		if errors.Is(err, repo.ErrNoGitChanges) {
			return repoWriteResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("repo file content did not change"))
		}
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	committed = true
	return server.finalizeRepoWrite(ctx, commitID, baseSyncState)
}

func (server *repoServer) createRepoDirectoryTransaction(ctx context.Context, relativePath, commitMessage string, baseSyncState store.RepoSyncState) (_ repoWriteResult, retErr error) {
	snapshot, err := repo.CapturePath(server.cfg.RepoDir, relativePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() { _ = repo.CleanupPathSnapshot(snapshot) }()
	committed := false
	createdPath, err := repo.CreateDirectory(server.cfg.RepoDir, relativePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() {
		if retErr == nil || committed {
			return
		}
		_ = repo.RestorePath(server.cfg.RepoDir, snapshot)
	}()
	message := commitMessage
	if message == "" {
		message = defaultRepoCommitMessage("add", createdPath)
	}
	commitID, err := server.commitRepoPaths(createdPath, []string{createdPath}, message)
	if err != nil {
		return repoWriteResult{}, err
	}
	committed = true
	return server.finalizeRepoWrite(ctx, commitID, baseSyncState)
}

func (server *repoServer) moveRepoPathTransaction(ctx context.Context, sourcePath, destinationPath, commitMessage string, baseSyncState store.RepoSyncState) (_ repoWriteResult, retErr error) {
	sourceSnapshot, err := repo.CapturePath(server.cfg.RepoDir, sourcePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() { _ = repo.CleanupPathSnapshot(sourceSnapshot) }()
	destinationSnapshot, err := repo.CapturePath(server.cfg.RepoDir, destinationPath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() { _ = repo.CleanupPathSnapshot(destinationSnapshot) }()
	committed := false
	movedSource, movedDestination, err := repo.MovePath(server.cfg.RepoDir, sourcePath, destinationPath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() {
		if retErr == nil || committed {
			return
		}
		_ = repo.RestorePath(server.cfg.RepoDir, destinationSnapshot)
		_ = repo.RestorePath(server.cfg.RepoDir, sourceSnapshot)
	}()
	message := commitMessage
	if message == "" {
		message = fmt.Sprintf("move %s to %s", movedSource, movedDestination)
	}
	commitID, err := server.commitRepoPaths(movedDestination, []string{movedSource, movedDestination}, message)
	if err != nil {
		return repoWriteResult{}, err
	}
	committed = true
	return server.finalizeRepoWrite(ctx, commitID, baseSyncState)
}

func (server *repoServer) deleteRepoPathTransaction(ctx context.Context, relativePath, commitMessage string, baseSyncState store.RepoSyncState) (_ repoWriteResult, retErr error) {
	snapshot, err := repo.CapturePath(server.cfg.RepoDir, relativePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() { _ = repo.CleanupPathSnapshot(snapshot) }()
	committed := false
	deletedPath, err := repo.DeletePath(server.cfg.RepoDir, relativePath)
	if err != nil {
		return repoWriteResult{}, mapRepoMutationError(err)
	}
	defer func() {
		if retErr == nil || committed {
			return
		}
		_ = repo.RestorePath(server.cfg.RepoDir, snapshot)
	}()
	message := commitMessage
	if message == "" {
		message = defaultRepoCommitMessage("remove", deletedPath)
	}
	commitID, err := server.commitRepoPaths(deletedPath, []string{deletedPath}, message)
	if err != nil {
		return repoWriteResult{}, err
	}
	committed = true
	return server.finalizeRepoWrite(ctx, commitID, baseSyncState)
}

func (server *repoServer) commitRepoPaths(primaryPath string, relativePaths []string, commitMessage string) (string, error) {
	authorName := ""
	authorEmail := ""
	if server.cfg.Git != nil {
		authorName = server.cfg.Git.AuthorName
		authorEmail = server.cfg.Git.AuthorEmail
	}
	commitID, err := repo.CommitPaths(server.cfg.RepoDir, relativePaths, commitMessage, authorName, authorEmail)
	if err != nil {
		if errors.Is(err, repo.ErrNoGitChanges) {
			return "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("repo path %q did not change", primaryPath))
		}
		return "", connect.NewError(connect.CodeInternal, err)
	}
	return commitID, nil
}

func (server *repoServer) finalizeRepoWrite(ctx context.Context, commitID string, baseSyncState store.RepoSyncState) (repoWriteResult, error) {
	result, err := server.finalizeRepoGitState(ctx, commitID, baseSyncState)
	if err != nil {
		return repoWriteResult{}, err
	}
	if err := server.refreshDeclaredServices(ctx); err != nil {
		return repoWriteResult{}, err
	}
	return result, nil
}

func (server *repoServer) finalizeRepoGitState(ctx context.Context, commitID string, baseSyncState store.RepoSyncState) (repoWriteResult, error) {
	result := repoWriteResult{CommitID: commitID, SyncStatus: baseSyncState.SyncStatus, LastSuccessfulPullAt: baseSyncState.LastSuccessfulPullAt}
	if !server.hasConfiguredRemote() {
		result.SyncStatus = store.RepoSyncStatusLocalOnly
		return result, nil
	}
	branch, err := server.configuredRemoteBranch()
	if err != nil {
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	authToken, err := server.configuredGitAuthToken()
	if err != nil {
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := repo.PushCurrentBranch(server.cfg.RepoDir, strings.TrimSpace(server.cfg.Git.RemoteURL), branch, authToken); err != nil {
		state := store.RepoSyncState{
			SyncStatus:           store.RepoSyncStatusPushFailed,
			LastSyncError:        err.Error(),
			LastSuccessfulPullAt: baseSyncState.LastSuccessfulPullAt,
		}
		if persistErr := server.persistRepoSyncState(ctx, state); persistErr != nil {
			return repoWriteResult{}, connect.NewError(connect.CodeInternal, persistErr)
		}
		result.SyncStatus = state.SyncStatus
		result.PushError = state.LastSyncError
		result.LastSuccessfulPullAt = state.LastSuccessfulPullAt
		return result, nil
	}
	state := store.RepoSyncState{
		SyncStatus:           store.RepoSyncStatusSynced,
		LastSyncError:        "",
		LastSuccessfulPullAt: baseSyncState.LastSuccessfulPullAt,
	}
	if err := server.persistRepoSyncState(ctx, state); err != nil {
		return repoWriteResult{}, connect.NewError(connect.CodeInternal, err)
	}
	result.SyncStatus = state.SyncStatus
	result.LastSuccessfulPullAt = state.LastSuccessfulPullAt
	return result, nil
}

func mapRepoMutationError(err error) error {
	switch {
	case errors.Is(err, repo.ErrRepoPathInvalid), errors.Is(err, repo.ErrRepoPathProtected):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, repo.ErrRepoPathNotFound), errors.Is(err, repo.ErrRepoPathAlreadyExists), errors.Is(err, repo.ErrRepoPathNotFile), errors.Is(err, repo.ErrRepoPathNotDirectory):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func defaultRepoCommitMessage(action, relativePath string) string {
	return fmt.Sprintf("%s %s", action, relativePath)
}

func (server *secretServer) GetSecret(ctx context.Context, req *connect.Request[controllerv1.GetSecretRequest]) (*connect.Response[controllerv1.GetSecretResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if req.Msg.GetFilePath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_path is required"))
	}
	if server.cfg.Secrets == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller secrets are not configured"))
	}
	service, filePath, err := server.resolveServiceFilePath(req.Msg.GetServiceName(), req.Msg.GetFilePath())
	if err != nil {
		return nil, err
	}
	secretFile, err := repo.ReadFile(server.cfg.RepoDir, filePath)
	if err != nil {
		if errors.Is(err, repo.ErrRepoPathNotFound) {
			return connect.NewResponse(&controllerv1.GetSecretResponse{ServiceName: service.Name, FilePath: req.Msg.GetFilePath(), Content: ""}), nil
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	plaintext, err := secretutil.Decrypt([]byte(secretFile.Content), server.cfg.Secrets)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controllerv1.GetSecretResponse{ServiceName: service.Name, FilePath: req.Msg.GetFilePath(), Content: plaintext}), nil
}

func (server *secretServer) UpdateSecret(ctx context.Context, req *connect.Request[controllerv1.UpdateSecretRequest]) (*connect.Response[controllerv1.UpdateSecretResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if req.Msg.GetFilePath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_path is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	if server.cfg.Secrets == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller secrets are not configured"))
	}
	service, filePath, err := server.resolveServiceFilePath(req.Msg.GetServiceName(), req.Msg.GetFilePath())
	if err != nil {
		return nil, err
	}
	ciphertext, err := secretutil.Encrypt(req.Msg.GetContent(), server.cfg.Secrets)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	repoSrv := &repoServer{db: server.db, cfg: server.cfg, availableNodeIDs: server.availableNodeIDs, repoMu: server.repoMu}
	commitMessage := req.Msg.GetCommitMessage()
	if commitMessage == "" {
		commitMessage = fmt.Sprintf("update encrypted file %s for %s", req.Msg.GetFilePath(), service.Name)
	}
	result, err := repoSrv.runRepoWrite(ctx, req.Msg.GetBaseRevision(), []string{filePath}, func(baseSyncState store.RepoSyncState) (repoWriteResult, error) {
		return repoSrv.updateRepoFileTransaction(ctx, filePath, string(ciphertext), commitMessage, baseSyncState)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.UpdateSecretResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *secretServer) resolveServiceFilePath(serviceName, filePath string) (*repo.Service, string, error) {
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, serviceName)
	if err != nil {
		return nil, "", connect.NewError(connect.CodeNotFound, err)
	}
	serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
	if err != nil {
		return nil, "", connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
	}
	cleanPath := filepath.ToSlash(filepath.Clean(filePath))
	if strings.HasPrefix(cleanPath, "../") || strings.Contains(cleanPath, "/../") {
		return nil, "", connect.NewError(connect.CodeInvalidArgument, errors.New("file_path must not escape service directory"))
	}
	if filepath.IsAbs(cleanPath) {
		return nil, "", connect.NewError(connect.CodeInvalidArgument, errors.New("file_path must be relative"))
	}
	fullPath := filepath.ToSlash(filepath.Join(serviceDir, cleanPath))
	return &service, fullPath, nil
}

func (server *taskServer) GetTask(ctx context.Context, req *connect.Request[controllerv1.GetTaskRequest]) (*connect.Response[controllerv1.GetTaskResponse], error) {
	if req.Msg == nil || req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.GetTaskResponse{
		TaskId:          detail.Record.TaskID,
		Type:            string(detail.Record.Type),
		Source:          string(detail.Record.Source),
		ServiceName:     detail.Record.ServiceName,
		NodeId:          detail.Record.NodeID,
		Status:          string(detail.Record.Status),
		CreatedAt:       detail.Record.CreatedAt.UTC().Format(time.RFC3339),
		StartedAt:       formatNullableTime(detail.Record.StartedAt),
		FinishedAt:      formatNullableTime(detail.Record.FinishedAt),
		RepoRevision:    detail.Record.RepoRevision,
		ErrorSummary:    detail.Record.ErrorSummary,
		LogPath:         detail.Record.LogPath,
		TriggeredBy:     detail.Record.TriggeredBy,
		ResultRevision:  detail.Record.ResultRevision,
		AttemptOfTaskId: detail.Record.AttemptOfTaskID,
		Steps:           make([]*controllerv1.TaskStepSummary, 0, len(detail.Steps)),
	}
	for _, step := range detail.Steps {
		response.Steps = append(response.Steps, &controllerv1.TaskStepSummary{
			StepName:   string(step.StepName),
			Status:     string(step.Status),
			StartedAt:  formatNullableTime(step.StartedAt),
			FinishedAt: formatNullableTime(step.FinishedAt),
		})
	}

	return connect.NewResponse(response), nil
}

func (server *taskServer) TailTaskLogs(ctx context.Context, req *connect.Request[controllerv1.TailTaskLogsRequest], stream *connect.ServerStream[controllerv1.TailTaskLogsResponse]) error {
	if req.Msg == nil || req.Msg.GetTaskId() == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return connect.NewError(connect.CodeNotFound, err)
		}
		return connect.NewError(connect.CodeInternal, err)
	}
	if detail.Record.LogPath == "" {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("task does not have a log file"))
	}

	var offset int64
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		content, nextOffset, err := readNewLogContent(detail.Record.LogPath, offset)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		offset = nextOffset
		if content != "" {
			if err := stream.Send(&controllerv1.TailTaskLogsResponse{Content: content}); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (server *taskServer) RunTaskAgain(ctx context.Context, req *connect.Request[controllerv1.RunTaskAgainRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if detail.Record.ServiceName == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task cannot be rerun without service_name"))
	}

	var rerunType task.Type
	switch detail.Record.Type {
	case task.TypeDeploy, task.TypeUpdate, task.TypeStop, task.TypeRestart, task.TypeBackup:
		rerunType = detail.Record.Type
	default:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task type %q cannot be rerun yet", detail.Record.Type))
	}

	params := taskParams(detail.Record.ParamsJSON)
	var targetNodeIDs []string
	if detail.Record.NodeID != "" {
		targetNodeIDs = []string{detail.Record.NodeID}
	}
	createdTask, err := createServiceTaskWithOptions(ctx, server.db, server.cfg, server.availableNodeIDs, detail.Record.ServiceName, targetNodeIDs, rerunType, params.DataNames, serviceTaskCreateOptions{AttemptOfTaskID: detail.Record.TaskID, Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(createdTask)), nil
}

func (server *serviceInstanceServer) ListServiceInstances(ctx context.Context, req *connect.Request[controllerv1.ListServiceInstancesRequest]) (*connect.Response[controllerv1.ListServiceInstancesResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if _, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	instances, err := server.db.ListServiceInstances(ctx, req.Msg.GetServiceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.ListServiceInstancesResponse{Instances: make([]*controllerv1.ServiceInstanceSummary, 0, len(instances))}
	for _, instance := range instances {
		response.Instances = append(response.Instances, serviceInstanceSummaryMessage(instance))
	}
	return connect.NewResponse(response), nil
}

func (server *serviceInstanceServer) GetServiceInstance(ctx context.Context, req *connect.Request[controllerv1.GetServiceInstanceRequest]) (*connect.Response[controllerv1.GetServiceInstanceResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name and node_id are required"))
	}
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	instance, err := server.db.GetServiceInstanceSnapshot(ctx, req.Msg.GetServiceName(), req.Msg.GetNodeId())
	if err != nil {
		if errors.Is(err, store.ErrServiceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	detail, err := buildServiceInstanceDetail(ctx, server.db, server.cfg, server.taskQueue, server.taskResults, service, instance)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.GetServiceInstanceResponse{Instance: detail}), nil
}

func (server *serviceInstanceServer) RunServiceInstanceAction(ctx context.Context, req *connect.Request[controllerv1.RunServiceInstanceActionRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name, node_id, and action are required"))
	}

	var taskType task.Type
	switch req.Msg.GetAction() {
	case controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_DEPLOY:
		taskType = task.TypeDeploy
	case controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_UPDATE:
		taskType = task.TypeUpdate
	case controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_STOP:
		taskType = task.TypeStop
	case controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_RESTART:
		taskType = task.TypeRestart
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("action is required"))
	}

	createdTask, err := server.createInstanceTask(ctx, req.Msg.GetServiceName(), req.Msg.GetNodeId(), taskType, nil, requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(createdTask)), nil
}

func (server *serviceInstanceServer) createInstanceTask(ctx context.Context, serviceName, nodeID string, taskType task.Type, dataNames []string, source task.Source) (task.Record, error) {
	if serviceName == "" || nodeID == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name and node_id are required"))
	}
	return createServiceTaskWithOptions(ctx, server.db, server.cfg, server.availableNodeIDs, serviceName, []string{nodeID}, taskType, dataNames, serviceTaskCreateOptions{Source: source})
}

func formatNullableTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func taskActionResponse(record task.Record) *controllerv1.TaskActionResponse {
	return &controllerv1.TaskActionResponse{
		TaskId:       record.TaskID,
		Status:       string(record.Status),
		RepoRevision: record.RepoRevision,
	}
}

func readNewLogContent(logPath string, offset int64) (string, int64, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return "", offset, fmt.Errorf("open task log %q: %w", logPath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", offset, fmt.Errorf("stat task log %q: %w", logPath, err)
	}
	if offset > stat.Size() {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", offset, fmt.Errorf("seek task log %q: %w", logPath, err)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return "", offset, fmt.Errorf("read task log %q: %w", logPath, err)
	}
	return string(content), stat.Size(), nil
}

func protoTime(value *timestamppb.Timestamp) *time.Time {
	if value == nil {
		return nil
	}
	parsed := value.AsTime().UTC()
	return &parsed
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func requestTaskSource(header http.Header) task.Source {
	switch strings.ToLower(strings.TrimSpace(header.Get("X-Composia-Source"))) {
	case string(task.SourceWeb):
		return task.SourceWeb
	case string(task.SourceSchedule):
		return task.SourceSchedule
	case string(task.SourceSystem):
		return task.SourceSystem
	default:
		return task.SourceCLI
	}
}

func mustRelativeServiceDir(repoDir, serviceDir string) string {
	relativePath, err := filepath.Rel(repoDir, serviceDir)
	if err != nil {
		return filepath.ToSlash(serviceDir)
	}
	return filepath.ToSlash(relativePath)
}
