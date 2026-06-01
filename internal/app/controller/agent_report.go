package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	appnotify "forgejo.alexma.top/alexma233/composia/internal/app/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type agentReportServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	logState         *taskLogAckState
	taskQueue        *taskQueueNotifier
	taskResults      *taskResultNotifier
	dockerQueries    *dockerQueryBroker
	execManager      *execTunnelManager
	logManager       *containerLogTunnelManager
	repoMu           *sync.Mutex
	notifier         *appnotify.Notifier
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
	previousSnapshot, hadPreviousSnapshot := snapshotIfExists(ctx, server.db, req.Msg.GetNodeId())

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
	if hadPreviousSnapshot && !previousSnapshot.IsOnline && previousSnapshot.LastHeartbeat != "" {
		dispatchNodeNotification(server.notifier, corenotify.EventNodeOnline, store.NodeSnapshot{NodeID: req.Msg.GetNodeId(), IsOnline: true, LastHeartbeat: heartbeatAt.Format(time.RFC3339)})
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
	status, ok := taskStatusFromAgentProto(req.Msg.GetStatus())
	if !ok || (status != task.StatusSucceeded && status != task.StatusFailed && status != task.StatusCancelled) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("status must be a terminal task status"))
	}
	if err := ensureTaskNodeMatch(ctx, server.db, req.Msg.GetTaskId()); err != nil {
		return nil, err
	}

	finishedAt := time.Now().UTC()
	if req.Msg.GetFinishedAt() != nil {
		finishedAt = req.Msg.GetFinishedAt().AsTime().UTC()
	}
	if err := server.db.CompleteTask(ctx, req.Msg.GetTaskId(), status, finishedAt, req.Msg.GetErrorSummary()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	logControllerReceivedTaskState(ctx, server.db, req.Msg.GetTaskId(), status, req.Msg.GetErrorSummary())
	if err := server.queuePostTaskFollowups(ctx, detail.Record); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if eventType, ok := taskEventTypeForStatus(detail.Record.Status); ok {
		dispatchTaskRecordNotification(server.notifier, eventType, detail.Record)
	}
	server.resetTaskLogAck(req.Msg.GetTaskId())
	server.taskResults.Notify(req.Msg.GetTaskId())
	notifyTaskQueue(server.taskQueue)
	return connect.NewResponse(&agentv1.ReportTaskStateResponse{}), nil
}

func (server *agentReportServer) queuePostTaskFollowups(ctx context.Context, record task.Record) error {
	if record.Status != task.StatusSucceeded {
		return nil
	}
	switch record.Type {
	case task.TypeDeploy, task.TypeUpdate, task.TypeStop:
		if err := server.queueCaddyReloadForTask(ctx, record); err != nil {
			return err
		}
		if err := server.queueCloudflareTunnelSyncForTask(ctx, record); err != nil {
			return err
		}
		return nil
	case task.TypeImageCheck:
		return server.queueAutoApplyUpdateForImageCheck(ctx, record)
	default:
		return nil
	}
}

func (server *agentReportServer) queueCloudflareTunnelSyncForTask(ctx context.Context, record task.Record) error {
	if server.cfg == nil || server.cfg.CloudflareTunnel == nil {
		return nil
	}
	params, err := taskParams(record.ParamsJSON)
	if err != nil {
		return err
	}
	if params.ServiceDir == "" || record.RepoRevision == "" {
		return nil
	}
	service, err := repo.FindServiceAtRevision(server.cfg.RepoDir, record.RepoRevision, params.ServiceDir, server.availableNodeIDs)
	if err != nil {
		return fmt.Errorf("load service for post-task cloudflare tunnel sync: %w", err)
	}
	if !repo.CloudflareTunnelManaged(service) {
		return nil
	}
	excludedServiceDir := ""
	if record.Type == task.TypeStop {
		excludedServiceDir = params.ServiceDir
	}
	_, err = createServiceCloudflareTunnelSyncTask(ctx, server.db, server.cfg, server.availableNodeIDs, service.Name, excludedServiceDir, record.Source)
	return err
}

func (server *agentReportServer) queueAutoApplyUpdateForImageCheck(ctx context.Context, record task.Record) error {
	params, err := taskParams(record.ParamsJSON)
	if err != nil {
		return err
	}
	if server.cfg == nil || params.ServiceDir == "" || record.RepoRevision == "" || record.NodeID == "" {
		return nil
	}
	service, err := repo.FindServiceAtRevision(server.cfg.RepoDir, record.RepoRevision, params.ServiceDir, server.availableNodeIDs)
	if err != nil {
		return fmt.Errorf("load service for image update auto apply: %w", err)
	}
	if len(service.TargetNodes) == 0 || service.TargetNodes[0] != record.NodeID {
		return nil
	}
	if service.Meta.Update == nil || len(service.Meta.Update.Images) == 0 {
		return nil
	}
	checks, err := server.db.LatestServiceImageUpdateChecks(ctx, service.Name, record.NodeID)
	if err != nil {
		return err
	}
	checksByImage := make(map[string]store.ServiceImageUpdateCheck, len(checks))
	for _, check := range checks {
		checksByImage[check.ImageName] = check
	}
	selections := make([]*controllerv1.ImageUpdateSelection, 0)
	for imageName, image := range service.Meta.Update.Images {
		if len(params.ImageNames) > 0 && !slices.Contains(params.ImageNames, imageName) {
			continue
		}
		if !effectiveImageAutoApply(server.cfg, service.Meta.Update, image) {
			continue
		}
		check, ok := checksByImage[imageName]
		if !ok || !check.UpdateAvailable {
			continue
		}
		selection := &controllerv1.ImageUpdateSelection{ImageName: imageName}
		if !repo.IsDigestImageDiscovery(image.Discovery, service.Meta.Update.DiscoverySources) {
			selection.UseDetected = true
		}
		selections = append(selections, selection)
	}
	if len(selections) == 0 {
		return nil
	}
	serviceServer := &serviceCommandServer{db: server.db, cfg: server.cfg, availableNodeIDs: server.availableNodeIDs, taskQueue: server.taskQueue, taskResults: server.taskResults, repoMu: server.repoMu}
	baseRevision, err := repo.CurrentRevision(server.cfg.RepoDir)
	if err != nil {
		return fmt.Errorf("read repo revision for image update auto apply: %w", err)
	}
	createdTasks, _, err := serviceServer.runServiceUpdateWithImageSelections(ctx, service, nil, selections, false, nil, baseRevision, "update images for "+service.Name, task.SourceSchedule, composeRecreateModeParam(task.TypeUpdate, controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_AUTO))
	if err == nil && len(createdTasks) > 0 {
		dispatchImageUpdateAppliedNotification(server.notifier, record, createdTasks[0], imageUpdateSelectionNames(selections))
	}
	return err
}

func (server *agentReportServer) queueCaddyReloadForTask(ctx context.Context, record task.Record) error {
	if server.cfg == nil {
		return nil
	}
	params, err := taskParams(record.ParamsJSON)
	if err != nil {
		return err
	}
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
	if _, err := createNodeCaddyReloadTask(ctx, server.db, server.cfg, server.availableNodeIDs, record.NodeID, record.Source); err != nil {
		return err
	}
	return nil
}

func (server *agentReportServer) ReportTaskStepState(ctx context.Context, req *connect.Request[agentv1.ReportTaskStepStateRequest]) (*connect.Response[agentv1.ReportTaskStepStateResponse], error) {
	if req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	stepName, ok := taskStepNameFromAgentProto(req.Msg.GetStepName())
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("step_name is required"))
	}
	status, ok := taskStatusFromAgentProto(req.Msg.GetStatus())
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("status is required"))
	}
	if err := ensureTaskNodeMatch(ctx, server.db, req.Msg.GetTaskId()); err != nil {
		return nil, err
	}

	step := task.StepRecord{
		TaskID:     req.Msg.GetTaskId(),
		StepName:   stepName,
		Status:     status,
		StartedAt:  protoTime(req.Msg.GetStartedAt()),
		FinishedAt: protoTime(req.Msg.GetFinishedAt()),
	}
	if err := server.db.UpsertTaskStep(ctx, step); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	logControllerReceivedTaskStepState(ctx, server.db, step)
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
	taskDetail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	startedAt := time.Now().UTC().Format(time.RFC3339)
	if req.Msg.GetStartedAt() != nil {
		startedAt = req.Msg.GetStartedAt().AsTime().UTC().Format(time.RFC3339)
	}
	finishedAt := ""
	if req.Msg.GetFinishedAt() != nil {
		finishedAt = req.Msg.GetFinishedAt().AsTime().UTC().Format(time.RFC3339)
	}
	backupDetail := store.BackupDetail{
		BackupID:     req.Msg.GetBackupId(),
		TaskID:       req.Msg.GetTaskId(),
		ServiceName:  req.Msg.GetServiceName(),
		NodeID:       taskDetail.Record.NodeID,
		DataName:     req.Msg.GetDataName(),
		Status:       req.Msg.GetStatus(),
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		ArtifactRef:  req.Msg.GetArtifactRef(),
		ErrorSummary: req.Msg.GetErrorSummary(),
	}
	if err := server.db.UpsertBackupRecord(ctx, backupDetail); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if eventType, ok := backupEventTypeForStatus(req.Msg.GetStatus()); ok {
		dispatchBackupNotification(server.notifier, eventType, taskDetail.Record.Source, backupDetail)
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

	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
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

func (server *agentReportServer) ReportServiceImageStates(ctx context.Context, req *connect.Request[agentv1.ReportServiceImageStatesRequest]) (*connect.Response[agentv1.ReportServiceImageStatesResponse], error) {
	if req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}
	reportedAt := time.Now().UTC()
	if req.Msg.GetReportedAt() != nil {
		reportedAt = req.Msg.GetReportedAt().AsTime().UTC()
	}
	states := make([]store.ServiceImageState, 0, len(req.Msg.GetImages()))
	for _, image := range req.Msg.GetImages() {
		if image.GetComposeService() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("compose_service is required"))
		}
		if image.GetImageRef() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("image_ref is required"))
		}
		status := image.GetCheckStatus()
		if status == "" {
			status = store.ImageCheckStatusUnknown
		}
		if !store.IsValidImageCheckStatus(status) {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid check_status %q", status))
		}
		states = append(states, store.ServiceImageState{
			ServiceName:          req.Msg.GetServiceName(),
			NodeID:               req.Msg.GetNodeId(),
			ComposeService:       image.GetComposeService(),
			ImageRef:             image.GetImageRef(),
			LocalDigest:          image.GetLocalDigest(),
			RemoteDigest:         image.GetRemoteDigest(),
			LocalDigestObserved:  image.GetLocalDigestObserved(),
			RemoteDigestObserved: image.GetRemoteDigestObserved(),
			CheckStatus:          status,
			ErrorSummary:         image.GetErrorSummary(),
			CheckedAt:            reportedAt,
			UpdatedAt:            reportedAt,
		})
	}
	if err := server.db.UpsertServiceImageStates(ctx, states); err != nil {
		if errors.Is(err, store.ErrServiceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&agentv1.ReportServiceImageStatesResponse{}), nil
}

func (server *agentReportServer) ReportServiceImageUpdateChecks(ctx context.Context, req *connect.Request[agentv1.ReportServiceImageUpdateChecksRequest]) (*connect.Response[agentv1.ReportServiceImageUpdateChecksResponse], error) {
	if req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}
	reportedAt := time.Now().UTC()
	if req.Msg.GetReportedAt() != nil {
		reportedAt = req.Msg.GetReportedAt().AsTime().UTC()
	}
	previousChecks, err := server.db.LatestServiceImageUpdateChecks(ctx, req.Msg.GetServiceName(), req.Msg.GetNodeId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	checks := make([]store.ServiceImageUpdateCheck, 0, len(req.Msg.GetChecks()))
	for _, check := range req.Msg.GetChecks() {
		if check.GetImageName() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("image_name is required"))
		}
		if check.GetImageRef() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("image_ref is required"))
		}
		status := check.GetCheckStatus()
		if status == "" {
			status = store.ImageCheckStatusUnknown
		}
		if !store.IsValidImageCheckStatus(status) {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid check_status %q", status))
		}
		candidateTagsJSON := ""
		if len(check.GetCandidateTags()) > 0 {
			encoded, err := json.Marshal(check.GetCandidateTags())
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			candidateTagsJSON = string(encoded)
		}
		checks = append(checks, store.ServiceImageUpdateCheck{
			ServiceName:       req.Msg.GetServiceName(),
			NodeID:            req.Msg.GetNodeId(),
			ImageName:         check.GetImageName(),
			ImageRef:          check.GetImageRef(),
			PolicyType:        check.GetPolicyType(),
			CurrentValue:      check.GetCurrentValue(),
			CurrentTag:        check.GetCurrentTag(),
			CurrentDigest:     check.GetCurrentDigest(),
			CandidateTag:      check.GetCandidateTag(),
			CandidateDigest:   check.GetCandidateDigest(),
			CandidateTagsJSON: candidateTagsJSON,
			UpdateAvailable:   check.GetUpdateAvailable(),
			CheckStatus:       status,
			ErrorSummary:      check.GetErrorSummary(),
			CheckedAt:         reportedAt,
			UpdatedAt:         reportedAt,
		})
	}
	if err := server.db.UpsertServiceImageUpdateChecks(ctx, checks); err != nil {
		if errors.Is(err, store.ErrServiceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	sourceRecord, err := latestTaskRecordForServiceNodeType(ctx, server.db, req.Msg.GetServiceName(), req.Msg.GetNodeId(), task.TypeImageCheck)
	if err != nil {
		sourceRecord = task.Record{}
	}
	for _, check := range detectNewImageUpdateChecks(previousChecks, checks) {
		dispatchImageUpdateAvailableNotification(server.notifier, sourceRecord.Source, sourceRecord.TaskID, check)
	}
	return connect.NewResponse(&agentv1.ReportServiceImageUpdateChecksResponse{}), nil
}
