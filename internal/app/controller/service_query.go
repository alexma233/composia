package controller

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type serviceQueryServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
	taskResults      *taskResultNotifier
	dockerQueries    *dockerQueryBroker
	repoMu           *sync.Mutex
}

func (server *serviceQueryServer) ListServices(ctx context.Context, req *connect.Request[controllerv1.ListServicesRequest]) (*connect.Response[controllerv1.ListServicesResponse], error) {
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

func (server *serviceQueryServer) ListServiceWorkspaces(ctx context.Context, _ *connect.Request[controllerv1.ListServiceWorkspacesRequest]) (*connect.Response[controllerv1.ListServiceWorkspacesResponse], error) {
	workspaces, err := server.listServiceWorkspaceSummaries(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controllerv1.ListServiceWorkspacesResponse{Workspaces: workspaces}), nil
}

func (server *serviceQueryServer) GetService(ctx context.Context, req *connect.Request[controllerv1.GetServiceRequest]) (*connect.Response[controllerv1.GetServiceResponse], error) {
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
	snapshotByNodeID, err := buildNodeSnapshotMap(ctx, server.db)
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
		Actions:       buildServiceActionCapabilities(server.cfg, server.availableNodeIDs, snapshotByNodeID, service),
	}
	for _, instance := range instances {
		if !req.Msg.GetIncludeContainers() {
			response.Instances = append(response.Instances, serviceInstanceDetailMessage(instance, nil))
			continue
		}
		detail, err := buildServiceInstanceDetail(ctx, server.db, server.cfg, server.dockerQueries, service, instance, requestTaskSource(req.Header()))
		if err != nil {
			return nil, err
		}
		response.Instances = append(response.Instances, detail)
	}
	return connect.NewResponse(response), nil
}

func (server *serviceQueryServer) GetServiceWorkspace(ctx context.Context, req *connect.Request[controllerv1.GetServiceWorkspaceRequest]) (*connect.Response[controllerv1.GetServiceWorkspaceResponse], error) {
	if req.Msg == nil || strings.TrimSpace(req.Msg.GetFolder()) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("folder is required"))
	}
	workspaces, err := server.listServiceWorkspaceSummaries(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	folder := filepath.ToSlash(strings.TrimSpace(req.Msg.GetFolder()))
	for _, workspace := range workspaces {
		if workspace.GetFolder() == folder {
			return connect.NewResponse(&controllerv1.GetServiceWorkspaceResponse{Workspace: workspace}), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("service folder %q not found", folder))
}

func (server *serviceQueryServer) GetServiceTasks(ctx context.Context, req *connect.Request[controllerv1.GetServiceTasksRequest]) (*connect.Response[controllerv1.GetServiceTasksResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if _, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	statusFilters := []string(nil)
	if status, ok := taskStatusFromProto(req.Msg.GetStatus()); ok {
		statusFilters = []string{string(status)}
	}
	tasks, totalCount, err := server.db.ListTasks(ctx, statusFilters, []string{req.Msg.GetServiceName()}, nil, nil, nil, nil, nil, nil, req.Msg.GetPage(), req.Msg.GetPageSize())
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

func (server *serviceQueryServer) GetServiceBackups(ctx context.Context, req *connect.Request[controllerv1.GetServiceBackupsRequest]) (*connect.Response[controllerv1.GetServiceBackupsResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	if _, err := repo.FindService(server.cfg.RepoDir, server.availableNodeIDs, req.Msg.GetServiceName()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	statusFilters := []string(nil)
	if req.Msg.GetStatus() != "" {
		statusFilters = []string{req.Msg.GetStatus()}
	}
	dataNameFilters := []string(nil)
	if req.Msg.GetDataName() != "" {
		dataNameFilters = []string{req.Msg.GetDataName()}
	}
	backups, totalCount, err := server.db.ListBackups(ctx, []string{req.Msg.GetServiceName()}, statusFilters, dataNameFilters, nil, nil, nil, nil, nil, req.Msg.GetPage(), req.Msg.GetPageSize())
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

func (server *serviceQueryServer) listServiceWorkspaceSummaries(ctx context.Context) ([]*controllerv1.ServiceWorkspaceSummary, error) {
	if server.cfg == nil || strings.TrimSpace(server.cfg.RepoDir) == "" {
		return nil, errors.New("controller repo_dir is not configured")
	}
	entries, err := repo.ListFiles(server.cfg.RepoDir, "", false)
	if err != nil {
		return nil, err
	}
	declaredServices, _, err := server.db.ListDeclaredServices(ctx, "", 1, 10000)
	if err != nil {
		return nil, err
	}
	snapshotByNodeID, err := buildNodeSnapshotMap(ctx, server.db)
	if err != nil {
		return nil, err
	}
	declaredByName := make(map[string]store.ServiceSummary, len(declaredServices))
	for _, service := range declaredServices {
		declaredByName[service.Name] = service
	}
	workspaces := make([]*controllerv1.ServiceWorkspaceSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}
		workspace, err := server.buildServiceWorkspaceSummary(entry.Path, entry.Name, declaredByName, snapshotByNodeID)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, nil
}

func (server *serviceQueryServer) buildServiceWorkspaceSummary(folder, defaultName string, declaredByName map[string]store.ServiceSummary, snapshotByNodeID map[string]store.NodeSnapshot) (*controllerv1.ServiceWorkspaceSummary, error) {
	workspace := &controllerv1.ServiceWorkspaceSummary{
		Folder:        folder,
		DisplayName:   defaultName,
		RuntimeStatus: "uninitialized",
		Nodes:         []string{},
		Actions:       buildDisabledServiceActionCapabilities(reasonMissingServiceMeta),
	}
	metaPath := filepath.Join(server.cfg.RepoDir, filepath.FromSlash(folder), repo.MetaFileName)
	metaInfo, err := os.Stat(metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return workspace, nil
		}
		return nil, fmt.Errorf("stat service meta %q: %w", metaPath, err)
	}
	if metaInfo.IsDir() {
		return nil, fmt.Errorf("service meta %q must be a file", metaPath)
	}
	workspace.HasMeta = true
	workspace.Actions = buildDisabledServiceActionCapabilities(reasonServiceNotDeclared)
	meta, err := repo.LoadServiceMeta(metaPath)
	if err != nil {
		workspace.RuntimeStatus = "needs_validation"
		return workspace, nil
	}
	serviceName := strings.TrimSpace(meta.Name)
	if serviceName != "" {
		workspace.DisplayName = serviceName
		workspace.ServiceName = serviceName
	}
	workspace.Nodes = normalizeWorkspaceNodeIDs(meta.Nodes)
	workspace.Enabled = meta.Enabled == nil || *meta.Enabled
	service, err := repo.LoadServiceFromMetaPath(metaPath, server.availableNodeIDs)
	if err != nil {
		workspace.RuntimeStatus = "needs_validation"
		return workspace, nil
	}
	workspace.Actions = buildServiceActionCapabilities(server.cfg, server.availableNodeIDs, snapshotByNodeID, service)
	declared, ok := declaredByName[workspace.ServiceName]
	if !ok {
		workspace.RuntimeStatus = "needs_validation"
		workspace.Actions = buildDisabledServiceActionCapabilities(reasonServiceNotDeclared)
		return workspace, nil
	}
	workspace.IsDeclared = true
	workspace.RuntimeStatus = declared.RuntimeStatus
	workspace.UpdatedAt = declared.UpdatedAt
	return workspace, nil
}

func normalizeWorkspaceNodeIDs(nodeIDs []string) []string {
	normalized := make([]string, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			continue
		}
		normalized = append(normalized, nodeID)
	}
	return normalized
}

func (server *serviceQueryServer) GetServiceImageUpdateChecks(ctx context.Context, req *connect.Request[controllerv1.GetServiceImageUpdateChecksRequest]) (*connect.Response[controllerv1.GetServiceImageUpdateChecksResponse], error) {
	if req.Msg == nil || req.Msg.GetServiceName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("service_name is required"))
	}
	checks, err := server.db.LatestServiceImageUpdateChecks(ctx, req.Msg.GetServiceName(), req.Msg.GetNodeId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := &controllerv1.GetServiceImageUpdateChecksResponse{Checks: make([]*controllerv1.ServiceImageUpdateCheckSummary, 0, len(checks))}
	for _, check := range checks {
		var candidateTags []string
		if check.CandidateTagsJSON != "" {
			_ = json.Unmarshal([]byte(check.CandidateTagsJSON), &candidateTags)
		}
		checkedAt := ""
		if !check.CheckedAt.IsZero() {
			checkedAt = check.CheckedAt.UTC().Format(time.RFC3339)
		}
		response.Checks = append(response.Checks, &controllerv1.ServiceImageUpdateCheckSummary{
			ServiceName:     check.ServiceName,
			NodeId:          check.NodeID,
			ImageName:       check.ImageName,
			ImageRef:        check.ImageRef,
			PolicyType:      check.PolicyType,
			CurrentValue:    check.CurrentValue,
			CurrentTag:      check.CurrentTag,
			CurrentDigest:   check.CurrentDigest,
			CandidateTag:    check.CandidateTag,
			CandidateDigest: check.CandidateDigest,
			CandidateTags:   candidateTags,
			UpdateAvailable: check.UpdateAvailable,
			CheckStatus:     check.CheckStatus,
			ErrorSummary:    check.ErrorSummary,
			CheckedAt:       checkedAt,
		})
	}
	return connect.NewResponse(response), nil
}
