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
		&taskServer{db: db},
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

	var params deployTaskParams
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
			ServiceDir:   taskServiceDir(record.ParamsJSON),
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

type deployTaskParams struct {
	ServiceDir string `json:"service_dir"`
}

type nodeServer struct {
	db  *store.DB
	cfg *config.ControllerConfig
}

type taskServer struct {
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

func (server *serviceServer) DeployService(ctx context.Context, req *connect.Request[controllerv1.DeployServiceRequest]) (*connect.Response[controllerv1.DeployServiceResponse], error) {
	if req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	createdTask, err := server.createServiceTask(ctx, req.Msg.GetServiceName(), task.TypeDeploy)
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

func (server *serviceServer) StopService(ctx context.Context, req *connect.Request[controllerv1.StopServiceRequest]) (*connect.Response[controllerv1.StopServiceResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	createdTask, err := server.createServiceTask(ctx, req.Msg.GetServiceName(), task.TypeStop)
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
	createdTask, err := server.createServiceTask(ctx, req.Msg.GetServiceName(), task.TypeRestart)
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

func (server *serviceServer) createServiceTask(ctx context.Context, serviceName string, taskType task.Type) (task.Record, error) {
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
	paramsJSON, err := json.Marshal(deployTaskParams{ServiceDir: serviceDir})
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

func taskServiceDir(paramsJSON string) string {
	if paramsJSON == "" {
		return ""
	}
	var params deployTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return ""
	}
	return params.ServiceDir
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
		snapshot := snapshotByNodeID[node.ID]
		displayName := node.DisplayName
		if displayName == "" {
			displayName = node.ID
		}

		response.Nodes = append(response.Nodes, &controllerv1.NodeSummary{
			NodeId:        node.ID,
			DisplayName:   displayName,
			Enabled:       node.Enabled == nil || *node.Enabled,
			IsOnline:      snapshot.IsOnline,
			LastHeartbeat: snapshot.LastHeartbeat,
		})
	}

	return connect.NewResponse(response), nil
}

func (server *taskServer) ListTasks(ctx context.Context, req *connect.Request[controllerv1.ListTasksRequest]) (*connect.Response[controllerv1.ListTasksResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListTasksRequest{}
	}

	tasks, nextCursor, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), req.Msg.GetServiceName(), req.Msg.GetCursor(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.ListTasksResponse{
		Tasks:      make([]*controllerv1.TaskSummary, 0, len(tasks)),
		NextCursor: nextCursor,
	}
	for _, record := range tasks {
		response.Tasks = append(response.Tasks, &controllerv1.TaskSummary{
			TaskId:      record.TaskID,
			Type:        record.Type,
			Status:      record.Status,
			ServiceName: record.ServiceName,
			NodeId:      record.NodeID,
			CreatedAt:   record.CreatedAt,
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

func formatNullableTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func protoTime(value *timestamppb.Timestamp) *time.Time {
	if value == nil {
		return nil
	}
	parsed := value.AsTime().UTC()
	return &parsed
}
