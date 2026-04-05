package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

	services, err := repo.DiscoverServices(cfg.RepoDir, availableNodeIDs)
	if err != nil {
		return err
	}
	serviceNames := make([]string, 0, len(services))
	for _, service := range services {
		serviceNames = append(serviceNames, service.Name)
	}
	if err := db.SyncDeclaredServices(ctx, serviceNames); err != nil {
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

	agentPath, agentHandler := agentv1connect.NewAgentReportServiceHandler(
		&agentReportServer{db: db, logState: &taskLogAckState{confirmedBy: make(map[string]uint64)}, taskQueue: taskQueue},
		connect.WithInterceptors(agentInterceptor),
	)
	mux.Handle(agentPath, agentHandler)

	agentTaskPath, agentTaskHandler := agentv1connect.NewAgentTaskServiceHandler(
		&agentTaskServer{db: db, taskQueue: taskQueue},
		connect.WithInterceptors(agentInterceptor),
	)
	mux.Handle(agentTaskPath, agentTaskHandler)

	bundlePath, bundleHandler := agentv1connect.NewBundleServiceHandler(
		&bundleServer{db: db, cfg: cfg},
		connect.WithInterceptors(agentInterceptor),
	)
	mux.Handle(bundlePath, bundleHandler)

	systemPath, systemHandler := controllerv1connect.NewSystemServiceHandler(
		&systemServer{db: db, cfg: cfg},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(systemPath, systemHandler)
	repoMu := &sync.Mutex{}

	repoPath, repoHandler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, repoMu: repoMu},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(repoPath, repoHandler)

	secretPath, secretHandler := controllerv1connect.NewSecretServiceHandler(
		&secretServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, repoMu: repoMu},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(secretPath, secretHandler)

	backupPath, backupHandler := controllerv1connect.NewBackupRecordServiceHandler(
		&backupRecordServer{db: db},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(backupPath, backupHandler)

	servicePath, serviceHandler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(servicePath, serviceHandler)

	nodePath, nodeHandler := controllerv1connect.NewNodeServiceHandler(
		&nodeServer{db: db, cfg: cfg},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(nodePath, nodeHandler)

	taskPath, taskHandler := controllerv1connect.NewTaskServiceHandler(
		&taskServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs, taskQueue: taskQueue},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(taskPath, taskHandler)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go sweepOfflineNodes(ctx, db)
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

type agentReportServer struct {
	db        *store.DB
	logState  *taskLogAckState
	taskQueue *taskQueueNotifier
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
	server.resetTaskLogAck(req.Msg.GetTaskId())
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&agentv1.ReportTaskStateResponse{}), nil
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

func (server *agentReportServer) ReportServiceStatus(ctx context.Context, req *connect.Request[agentv1.ReportServiceStatusRequest]) (*connect.Response[agentv1.ReportServiceStatusResponse], error) {
	if req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if !store.IsValidServiceRuntimeStatus(req.Msg.GetRuntimeStatus()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid runtime_status %q", req.Msg.GetRuntimeStatus()))
	}
	reportedAt := time.Now().UTC()
	if req.Msg.GetReportedAt() != nil {
		reportedAt = req.Msg.GetReportedAt().AsTime().UTC()
	}
	if err := server.db.UpdateServiceRuntimeStatus(ctx, req.Msg.GetServiceName(), req.Msg.GetRuntimeStatus(), reportedAt); err != nil {
		if errors.Is(err, store.ErrServiceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&agentv1.ReportServiceStatusResponse{}), nil
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
		plaintext, err := readOptionalSecretPlaintext(cfg, record.RepoRevision, params.ServiceDir)
		if err != nil {
			return nil, err
		}
		if plaintext != "" {
			extraFiles[filepath.ToSlash(filepath.Join(params.ServiceDir, ".secret.env"))] = plaintext
		}
	}
	if record.Type == task.TypeBackup {
		payload, err := buildBackupRuntimePayload(cfg, record.ServiceName, record.RepoRevision, params)
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

func readOptionalSecretPlaintext(cfg *config.ControllerConfig, revision, serviceDir string) (string, error) {
	secretContent, err := repo.ReadFileAtRevision(cfg.RepoDir, revision, filepath.ToSlash(filepath.Join(serviceDir, ".secret.env.enc")))
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

func buildBackupRuntimePayload(cfg *config.ControllerConfig, serviceName, revision string, params serviceTaskParams) (string, error) {
	if cfg.Backup == nil || cfg.Backup.Rustic == nil {
		return "", fmt.Errorf("controller backup.rustic is required for backup tasks")
	}
	service, err := repo.FindService(cfg.RepoDir, configuredNodeIDs(cfg), serviceName)
	if err != nil {
		return "", err
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
		retain := ""
		for _, backupItem := range service.Meta.Backup.Data {
			if backupItem.Name == data.Name {
				if backupItem.Provider != "" {
					provider = backupItem.Provider
				}
				retain = backupItem.Retain
				break
			}
		}
		if provider != "rustic" {
			return "", fmt.Errorf("backup provider %q is not implemented", provider)
		}
		items = append(items, backupcfg.RuntimeItem{Name: data.Name, Strategy: data.Backup.Strategy, Service: data.Backup.Service, Include: append([]string(nil), data.Backup.Include...), Provider: provider, Tags: []string{"composia-service:" + serviceName, "composia-data:" + data.Name}, Retain: retain})
	}
	passwordContent, err := os.ReadFile(cfg.Backup.Rustic.PasswordFile)
	if err != nil {
		return "", fmt.Errorf("read rustic password file: %w", err)
	}
	envValues, err := loadBackupEnvFiles(cfg.Backup.Rustic.EnvFiles)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(backupcfg.RuntimeConfig{Rustic: &backupcfg.RusticConfig{Repository: cfg.Backup.Rustic.Repository, Password: strings.TrimSpace(string(passwordContent)), Env: envValues}, Items: items})
	if err != nil {
		return "", fmt.Errorf("marshal backup runtime config for %s at %s: %w", serviceName, revision, err)
	}
	return string(payload), nil
}

func loadBackupEnvFiles(paths []string) (map[string]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	values := map[string]string{}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read backup env file %q: %w", path, err)
		}
		for lineNumber, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				return nil, fmt.Errorf("backup env file %q line %d must be KEY=VALUE", path, lineNumber+1)
			}
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values, nil
}

func configuredNodeIDs(cfg *config.ControllerConfig) map[string]struct{} {
	result := make(map[string]struct{}, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		result[node.ID] = struct{}{}
	}
	return result
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
	db  *store.DB
	cfg *config.ControllerConfig
}

type serviceServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
}

type serviceTaskParams struct {
	ServiceDir string   `json:"service_dir"`
	DataNames  []string `json:"data_names,omitempty"`
}

type nodeServer struct {
	db  *store.DB
	cfg *config.ControllerConfig
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

func (server *serviceServer) ListServices(ctx context.Context, req *connect.Request[controllerv1.ListServicesRequest]) (*connect.Response[controllerv1.ListServicesResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListServicesRequest{}
	}

	services, nextCursor, err := server.db.ListDeclaredServices(ctx, req.Msg.GetRuntimeStatus(), req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.ListServicesResponse{
		Services:   make([]*controllerv1.ServiceSummary, 0, len(services)),
		NextCursor: nextCursor,
	}
	for _, service := range services {
		response.Services = append(response.Services, &controllerv1.ServiceSummary{
			Name:          service.Name,
			IsDeclared:    service.IsDeclared,
			RuntimeStatus: service.RuntimeStatus,
			UpdatedAt:     service.UpdatedAt,
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
	response := &controllerv1.GetServiceResponse{
		Name:          service.Name,
		RuntimeStatus: snapshot.RuntimeStatus,
		UpdatedAt:     snapshot.UpdatedAt,
		Node:          service.Node,
		Enabled:       service.Enabled,
		Directory:     filepath.ToSlash(mustRelativeServiceDir(server.cfg.RepoDir, service.Directory)),
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
	tasks, nextCursor, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), req.Msg.GetServiceName(), "", "", req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetServiceTasksResponse{
		Tasks:      make([]*controllerv1.TaskSummary, 0, len(tasks)),
		NextCursor: nextCursor,
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
	backups, nextCursor, err := server.db.ListBackups(ctx, req.Msg.GetServiceName(), req.Msg.GetStatus(), req.Msg.GetDataName(), req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetServiceBackupsResponse{
		Backups:    make([]*controllerv1.BackupSummary, 0, len(backups)),
		NextCursor: nextCursor,
	}
	for _, backup := range backups {
		response.Backups = append(response.Backups, backupSummaryMessage(backup))
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) BackupService(ctx context.Context, req *connect.Request[controllerv1.BackupServiceRequest]) (*connect.Response[controllerv1.BackupServiceResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	dataNames, err := repo.ValidateRequestedBackupDataNames(service, req.Msg.GetDataNames())
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	createdTask, err := server.createServiceTaskWithOptions(ctx, req.Msg.GetServiceName(), task.TypeBackup, dataNames, serviceTaskCreateOptions{Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}
	response := &controllerv1.BackupServiceResponse{
		TaskId:       createdTask.TaskID,
		Status:       string(createdTask.Status),
		RepoRevision: createdTask.RepoRevision,
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) UpdateServiceDNS(ctx context.Context, req *connect.Request[controllerv1.UpdateServiceDNSRequest]) (*connect.Response[controllerv1.UpdateServiceDNSResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if service.Meta.Network == nil || service.Meta.Network.DNS == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q does not declare network.dns", service.Name))
	}
	if server.cfg.DNS == nil || server.cfg.DNS.Cloudflare == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller dns.cloudflare is not configured"))
	}
	createdTask, err := server.createServiceTaskWithOptions(ctx, req.Msg.GetServiceName(), task.TypeDNSUpdate, nil, serviceTaskCreateOptions{Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}
	response := &controllerv1.UpdateServiceDNSResponse{
		TaskId:       createdTask.TaskID,
		Status:       string(createdTask.Status),
		RepoRevision: createdTask.RepoRevision,
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) DeployService(ctx context.Context, req *connect.Request[controllerv1.DeployServiceRequest]) (*connect.Response[controllerv1.DeployServiceResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	createdTask, err := server.createServiceTaskWithOptions(ctx, req.Msg.GetServiceName(), task.TypeDeploy, nil, serviceTaskCreateOptions{Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}

	response := &controllerv1.DeployServiceResponse{
		TaskId:       createdTask.TaskID,
		Status:       string(createdTask.Status),
		RepoRevision: createdTask.RepoRevision,
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) UpdateService(ctx context.Context, req *connect.Request[controllerv1.UpdateServiceRequest]) (*connect.Response[controllerv1.UpdateServiceResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	createdTask, err := server.createServiceTaskWithOptions(ctx, req.Msg.GetServiceName(), task.TypeUpdate, nil, serviceTaskCreateOptions{Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}
	response := &controllerv1.UpdateServiceResponse{
		TaskId:       createdTask.TaskID,
		Status:       string(createdTask.Status),
		RepoRevision: createdTask.RepoRevision,
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) StopService(ctx context.Context, req *connect.Request[controllerv1.StopServiceRequest]) (*connect.Response[controllerv1.StopServiceResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	createdTask, err := server.createServiceTaskWithOptions(ctx, req.Msg.GetServiceName(), task.TypeStop, nil, serviceTaskCreateOptions{Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}
	response := &controllerv1.StopServiceResponse{
		TaskId:       createdTask.TaskID,
		Status:       string(createdTask.Status),
		RepoRevision: createdTask.RepoRevision,
	}
	return connect.NewResponse(response), nil
}

func (server *serviceServer) RestartService(ctx context.Context, req *connect.Request[controllerv1.RestartServiceRequest]) (*connect.Response[controllerv1.RestartServiceResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	createdTask, err := server.createServiceTaskWithOptions(ctx, req.Msg.GetServiceName(), task.TypeRestart, nil, serviceTaskCreateOptions{Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}
	response := &controllerv1.RestartServiceResponse{
		TaskId:       createdTask.TaskID,
		Status:       string(createdTask.Status),
		RepoRevision: createdTask.RepoRevision,
	}
	return connect.NewResponse(response), nil
}

type serviceTaskCreateOptions struct {
	AttemptOfTaskID string
	Source          task.Source
}

func (server *serviceServer) createServiceTask(ctx context.Context, serviceName string, taskType task.Type, dataNames []string) (task.Record, error) {
	return server.createServiceTaskWithOptions(ctx, serviceName, taskType, dataNames, serviceTaskCreateOptions{})
}

func (server *serviceServer) createServiceTaskWithOptions(ctx context.Context, serviceName string, taskType task.Type, dataNames []string, options serviceTaskCreateOptions) (task.Record, error) {
	if serviceName == "" {
		return task.Record{}, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}

	hasActiveTask, err := server.db.HasActiveServiceTask(ctx, serviceName)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if hasActiveTask {
		return task.Record{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("service %q already has an active task", serviceName))
	}

	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, serviceName)
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeNotFound, err)
	}
	if err := validateTaskTargetNode(ctx, server.db, server.cfg, service.Node, taskType); err != nil {
		return task.Record{}, err
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
	taskID := uuid.NewString()
	taskSource := options.Source
	if taskSource == "" {
		taskSource = task.SourceCLI
	}
	createdTask, err := server.db.CreateTask(ctx, task.Record{
		TaskID:          taskID,
		Type:            taskType,
		Source:          taskSource,
		TriggeredBy:     triggeredBy,
		ServiceName:     service.Name,
		NodeID:          service.Node,
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
	notifyTaskQueue(server.taskQueue)
	return createdTask, nil
}

func createServiceTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, serviceName string, taskType task.Type, dataNames []string) (task.Record, error) {
	return (&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs}).createServiceTask(ctx, serviceName, taskType, dataNames)
}

func createServiceTaskWithOptions(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, serviceName string, taskType task.Type, dataNames []string, options serviceTaskCreateOptions) (task.Record, error) {
	return (&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs}).createServiceTaskWithOptions(ctx, serviceName, taskType, dataNames, options)
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
	tasks, nextCursor, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), "", req.Msg.GetNodeId(), "", req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetNodeTasksResponse{
		Tasks:      make([]*controllerv1.TaskSummary, 0, len(tasks)),
		NextCursor: nextCursor,
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
		Source:      task.SourceCLI,
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

func (server *nodeServer) ListNodeContainers(ctx context.Context, req *connect.Request[controllerv1.ListNodeContainersRequest]) (*connect.Response[controllerv1.ListNodeContainersResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (server *nodeServer) InspectNodeContainer(ctx context.Context, req *connect.Request[controllerv1.InspectNodeContainerRequest]) (*connect.Response[controllerv1.InspectNodeContainerResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (server *nodeServer) ListNodeNetworks(ctx context.Context, req *connect.Request[controllerv1.ListNodeNetworksRequest]) (*connect.Response[controllerv1.ListNodeNetworksResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (server *nodeServer) InspectNodeNetwork(ctx context.Context, req *connect.Request[controllerv1.InspectNodeNetworkRequest]) (*connect.Response[controllerv1.InspectNodeNetworkResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (server *nodeServer) ListNodeVolumes(ctx context.Context, req *connect.Request[controllerv1.ListNodeVolumesRequest]) (*connect.Response[controllerv1.ListNodeVolumesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (server *nodeServer) InspectNodeVolume(ctx context.Context, req *connect.Request[controllerv1.InspectNodeVolumeRequest]) (*connect.Response[controllerv1.InspectNodeVolumeResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (server *nodeServer) ListNodeImages(ctx context.Context, req *connect.Request[controllerv1.ListNodeImagesRequest]) (*connect.Response[controllerv1.ListNodeImagesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (server *nodeServer) InspectNodeImage(ctx context.Context, req *connect.Request[controllerv1.InspectNodeImageRequest]) (*connect.Response[controllerv1.InspectNodeImageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
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

	tasks, nextCursor, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), req.Msg.GetServiceName(), req.Msg.GetNodeId(), req.Msg.GetType(), req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.ListTasksResponse{
		Tasks:      make([]*controllerv1.TaskSummary, 0, len(tasks)),
		NextCursor: nextCursor,
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

func (server *backupRecordServer) ListBackups(ctx context.Context, req *connect.Request[controllerv1.ListBackupsRequest]) (*connect.Response[controllerv1.ListBackupsResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListBackupsRequest{}
	}
	backups, nextCursor, err := server.db.ListBackups(ctx, req.Msg.GetServiceName(), req.Msg.GetStatus(), req.Msg.GetDataName(), req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.ListBackupsResponse{
		Backups:    make([]*controllerv1.BackupSummary, 0, len(backups)),
		NextCursor: nextCursor,
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
	server.repoLock().Lock()
	defer server.repoLock().Unlock()

	baseSyncState, err := server.prepareRepoWrite(ctx, req.Msg.GetBaseRevision(), req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	result, err := server.updateRepoFileTransaction(ctx, req.Msg.GetPath(), req.Msg.GetContent(), req.Msg.GetCommitMessage(), baseSyncState)
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
	server.repoLock().Lock()
	defer server.repoLock().Unlock()

	baseSyncState, err := server.prepareRepoWritePaths(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetPath()})
	if err != nil {
		return nil, err
	}
	result, err := server.createRepoDirectoryTransaction(ctx, req.Msg.GetPath(), req.Msg.GetCommitMessage(), baseSyncState)
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
	server.repoLock().Lock()
	defer server.repoLock().Unlock()

	baseSyncState, err := server.prepareRepoWritePaths(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetSourcePath(), req.Msg.GetDestinationPath()})
	if err != nil {
		return nil, err
	}
	result, err := server.moveRepoPathTransaction(ctx, req.Msg.GetSourcePath(), req.Msg.GetDestinationPath(), req.Msg.GetCommitMessage(), baseSyncState)
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
	server.repoLock().Lock()
	defer server.repoLock().Unlock()

	baseSyncState, err := server.prepareRepoWritePaths(ctx, req.Msg.GetBaseRevision(), []string{req.Msg.GetPath()})
	if err != nil {
		return nil, err
	}
	result, err := server.deleteRepoPathTransaction(ctx, req.Msg.GetPath(), req.Msg.GetCommitMessage(), baseSyncState)
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
	if server.hasConfiguredRemote() {
		if _, err := server.syncRepoLocked(ctx); err != nil {
			return store.RepoSyncState{}, err
		}
	}
	currentRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return store.RepoSyncState{}, connect.NewError(connect.CodeInternal, err)
	}
	if currentRevision != baseRevision {
		return store.RepoSyncState{}, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("base_revision %q does not match current HEAD %q", baseRevision, currentRevision))
	}
	if err := server.ensureCleanWorktree(); err != nil {
		return store.RepoSyncState{}, err
	}
	if err := server.ensureRepoPathsUnlocked(ctx, relativePaths...); err != nil {
		return store.RepoSyncState{}, err
	}
	return server.repoSyncState(ctx)
}

func (server *repoServer) refreshDeclaredServices(ctx context.Context) error {
	if server.db == nil {
		return nil
	}
	services, err := repo.DiscoverServices(server.cfg.RepoDir, server.availableNodeIDs)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	serviceNames := make([]string, 0, len(services))
	for _, service := range services {
		serviceNames = append(serviceNames, service.Name)
	}
	if err := server.db.SyncDeclaredServices(ctx, serviceNames); err != nil {
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
	result := repoWriteResult{CommitID: commitID, SyncStatus: baseSyncState.SyncStatus, LastSuccessfulPullAt: baseSyncState.LastSuccessfulPullAt}
	if !server.hasConfiguredRemote() {
		result.SyncStatus = store.RepoSyncStatusLocalOnly
		if err := server.refreshDeclaredServices(ctx); err != nil {
			return repoWriteResult{}, err
		}
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
		if refreshErr := server.refreshDeclaredServices(ctx); refreshErr != nil {
			return repoWriteResult{}, refreshErr
		}
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
	if err := server.refreshDeclaredServices(ctx); err != nil {
		return repoWriteResult{}, err
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

func (server *secretServer) GetServiceSecretEnv(ctx context.Context, req *connect.Request[controllerv1.GetServiceSecretEnvRequest]) (*connect.Response[controllerv1.GetServiceSecretEnvResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if server.cfg.Secrets == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller secrets are not configured"))
	}
	service, secretPath, err := server.serviceSecretPath(req.Msg.GetServiceName())
	if err != nil {
		return nil, err
	}
	secretFile, err := repo.ReadFile(server.cfg.RepoDir, secretPath)
	if err != nil {
		if errors.Is(err, repo.ErrRepoPathNotFound) {
			return connect.NewResponse(&controllerv1.GetServiceSecretEnvResponse{ServiceName: service.Name, Content: ""}), nil
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	plaintext, err := secretutil.Decrypt([]byte(secretFile.Content), server.cfg.Secrets)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controllerv1.GetServiceSecretEnvResponse{ServiceName: service.Name, Content: plaintext}), nil
}

func (server *secretServer) UpdateServiceSecretEnv(ctx context.Context, req *connect.Request[controllerv1.UpdateServiceSecretEnvRequest]) (*connect.Response[controllerv1.UpdateServiceSecretEnvResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if req.Msg.GetBaseRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	if server.cfg.Secrets == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller secrets are not configured"))
	}
	service, secretPath, err := server.serviceSecretPath(req.Msg.GetServiceName())
	if err != nil {
		return nil, err
	}
	ciphertext, err := secretutil.Encrypt(req.Msg.GetContent(), server.cfg.Secrets)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	repoSrv := &repoServer{db: server.db, cfg: server.cfg, availableNodeIDs: server.availableNodeIDs, repoMu: server.repoMu}
	repoSrv.repoLock().Lock()
	defer repoSrv.repoLock().Unlock()
	baseSyncState, err := repoSrv.prepareRepoWrite(ctx, req.Msg.GetBaseRevision(), secretPath)
	if err != nil {
		return nil, err
	}
	commitMessage := req.Msg.GetCommitMessage()
	if commitMessage == "" {
		commitMessage = fmt.Sprintf("update secrets for %s", service.Name)
	}
	result, err := repoSrv.updateRepoFileTransaction(ctx, secretPath, string(ciphertext), commitMessage, baseSyncState)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.UpdateServiceSecretEnvResponse{
		CommitId:             result.CommitID,
		SyncStatus:           result.SyncStatus,
		PushError:            result.PushError,
		LastSuccessfulPullAt: result.LastSuccessfulPullAt,
	}), nil
}

func (server *secretServer) serviceSecretPath(serviceName string) (*repo.Service, string, error) {
	service, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, serviceName)
	if err != nil {
		return nil, "", connect.NewError(connect.CodeNotFound, err)
	}
	serviceDir, err := filepath.Rel(server.cfg.RepoDir, service.Directory)
	if err != nil {
		return nil, "", connect.NewError(connect.CodeInternal, fmt.Errorf("resolve service directory: %w", err))
	}
	return &service, filepath.ToSlash(filepath.Join(serviceDir, ".secret.env.enc")), nil
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

func (server *taskServer) RunTaskAgain(ctx context.Context, req *connect.Request[controllerv1.RunTaskAgainRequest]) (*connect.Response[controllerv1.RunTaskAgainResponse], error) {
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
	createdTask, err := createServiceTaskWithOptions(ctx, server.db, server.cfg, server.availableNodeIDs, detail.Record.ServiceName, rerunType, params.DataNames, serviceTaskCreateOptions{AttemptOfTaskID: detail.Record.TaskID, Source: requestTaskSource(req.Header())})
	if err != nil {
		return nil, err
	}
	response := &controllerv1.RunTaskAgainResponse{
		TaskId:       createdTask.TaskID,
		Status:       string(createdTask.Status),
		RepoRevision: createdTask.RepoRevision,
	}
	return connect.NewResponse(response), nil
}

func formatNullableTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
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
