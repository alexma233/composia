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
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	heartbeatOfflineAfter = 45 * time.Second
	offlineSweepInterval  = 15 * time.Second
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
		&agentReportServer{db: db},
		connect.WithInterceptors(agentInterceptor),
	)
	mux.Handle(agentPath, agentHandler)

	agentTaskPath, agentTaskHandler := agentv1connect.NewAgentTaskServiceHandler(
		&agentTaskServer{db: db},
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

	repoPath, repoHandler := controllerv1connect.NewRepoServiceHandler(
		&repoServer{cfg: cfg},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(repoPath, repoHandler)

	backupPath, backupHandler := controllerv1connect.NewBackupRecordServiceHandler(
		&backupRecordServer{db: db},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(backupPath, backupHandler)

	servicePath, serviceHandler := controllerv1connect.NewServiceServiceHandler(
		&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(servicePath, serviceHandler)

	nodePath, nodeHandler := controllerv1connect.NewNodeServiceHandler(
		&nodeServer{db: db, cfg: cfg},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(nodePath, nodeHandler)

	taskPath, taskHandler := controllerv1connect.NewTaskServiceHandler(
		&taskServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs},
		connect.WithInterceptors(cliInterceptor),
	)
	mux.Handle(taskPath, taskHandler)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
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
	db *store.DB
}

type agentTaskServer struct {
	db *store.DB
}

type bundleServer struct {
	db  *store.DB
	cfg *config.ControllerConfig
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

func (server *agentReportServer) UploadTaskLogs(ctx context.Context, req *connect.Request[agentv1.UploadTaskLogsRequest]) (*connect.Response[agentv1.UploadTaskLogsResponse], error) {
	if req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if err := ensureTaskNodeMatch(ctx, server.db, req.Msg.GetTaskId()); err != nil {
		return nil, err
	}
	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := appendTaskLogRaw(detail.Record.LogPath, req.Msg.GetContent()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&agentv1.UploadTaskLogsResponse{}), nil
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

	pipeReader, pipeWriter := io.Pipe()
	go func() {
		pipeWriter.CloseWithError(repo.StreamServiceBundle(ctx, server.cfg.RepoDir, detail.Record.RepoRevision, params.ServiceDir, pipeWriter))
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

func (server *agentTaskServer) PullNextTask(ctx context.Context, req *connect.Request[agentv1.PullNextTaskRequest]) (*connect.Response[agentv1.PullNextTaskResponse], error) {
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}

	record, err := server.db.ClaimNextPendingTaskForNode(ctx, req.Msg.GetNodeId(), time.Now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrNoPendingTask) {
			return connect.NewResponse(&agentv1.PullNextTaskResponse{HasTask: false}), nil
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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
		},
	}
	return connect.NewResponse(response), nil
}

type systemServer struct {
	db  *store.DB
	cfg *config.ControllerConfig
}

type serviceServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
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
}

type backupRecordServer struct {
	db *store.DB
}

type repoServer struct {
	cfg *config.ControllerConfig
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
	tasks, nextCursor, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), req.Msg.GetServiceName(), "", req.Msg.GetCursor(), req.Msg.GetPageSize())
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
	createdTask, err := server.createServiceTask(ctx, req.Msg.GetServiceName(), task.TypeBackup, dataNames)
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

func (server *serviceServer) DeployService(ctx context.Context, req *connect.Request[controllerv1.DeployServiceRequest]) (*connect.Response[controllerv1.DeployServiceResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	createdTask, err := server.createServiceTask(ctx, req.Msg.GetServiceName(), task.TypeDeploy, nil)
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
	createdTask, err := server.createServiceTask(ctx, req.Msg.GetServiceName(), task.TypeUpdate, nil)
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
	createdTask, err := server.createServiceTask(ctx, req.Msg.GetServiceName(), task.TypeStop, nil)
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
	createdTask, err := server.createServiceTask(ctx, req.Msg.GetServiceName(), task.TypeRestart, nil)
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

func (server *serviceServer) createServiceTask(ctx context.Context, serviceName string, taskType task.Type, dataNames []string) (task.Record, error) {
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
	createdTask, err := server.db.CreateTask(ctx, task.Record{
		TaskID:       taskID,
		Type:         taskType,
		Source:       task.SourceCLI,
		TriggeredBy:  triggeredBy,
		ServiceName:  service.Name,
		NodeID:       service.Node,
		ParamsJSON:   string(paramsJSON),
		RepoRevision: repoRevision,
		LogPath:      filepath.Join(server.cfg.LogDir, "tasks", fmt.Sprintf("%s.log", taskID)),
	})
	if err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.WriteFile(createdTask.LogPath, []byte(""), 0o644); err != nil {
		return task.Record{}, connect.NewError(connect.CodeInternal, fmt.Errorf("create task log file: %w", err))
	}
	return createdTask, nil
}

func createServiceTask(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, serviceName string, taskType task.Type, dataNames []string) (task.Record, error) {
	return (&serviceServer{db: db, cfg: cfg, availableNodeIDs: availableNodeIDs}).createServiceTask(ctx, serviceName, taskType, dataNames)
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
	tasks, nextCursor, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), "", req.Msg.GetNodeId(), req.Msg.GetCursor(), req.Msg.GetPageSize())
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

	tasks, nextCursor, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), req.Msg.GetServiceName(), "", req.Msg.GetCursor(), req.Msg.GetPageSize())
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

func (server *repoServer) GetRepoHead(_ context.Context, _ *connect.Request[controllerv1.GetRepoHeadRequest]) (*connect.Response[controllerv1.GetRepoHeadResponse], error) {
	headRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	branch, err := repo.CurrentBranch(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	hasRemote, err := repo.HasRemote(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cleanWorktree, err := repo.IsCleanWorkingTree(server.cfg.RepoDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetRepoHeadResponse{
		HeadRevision:  headRevision,
		Branch:        branch,
		HasRemote:     hasRemote,
		CleanWorktree: cleanWorktree,
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
		TaskId:       detail.Record.TaskID,
		Type:         string(detail.Record.Type),
		Source:       string(detail.Record.Source),
		ServiceName:  detail.Record.ServiceName,
		NodeId:       detail.Record.NodeID,
		Status:       string(detail.Record.Status),
		CreatedAt:    detail.Record.CreatedAt.UTC().Format(time.RFC3339),
		StartedAt:    formatNullableTime(detail.Record.StartedAt),
		FinishedAt:   formatNullableTime(detail.Record.FinishedAt),
		RepoRevision: detail.Record.RepoRevision,
		ErrorSummary: detail.Record.ErrorSummary,
		LogPath:      detail.Record.LogPath,
		Steps:        make([]*controllerv1.TaskStepSummary, 0, len(detail.Steps)),
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
	case task.TypeDeploy, task.TypeUpdate, task.TypeStop, task.TypeRestart:
		rerunType = detail.Record.Type
	default:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task type %q cannot be rerun yet", detail.Record.Type))
	}

	params := taskParams(detail.Record.ParamsJSON)
	createdTask, err := createServiceTask(ctx, server.db, server.cfg, server.availableNodeIDs, detail.Record.ServiceName, rerunType, params.DataNames)
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
