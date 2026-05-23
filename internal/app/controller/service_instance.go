package controller

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

type serviceInstanceServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
	taskResults      *taskResultNotifier
	dockerQueries    *dockerQueryBroker
}

func serviceInstanceSummaryMessage(record store.ServiceInstanceSnapshot) *controllerv1.ServiceInstanceSummary {
	return &controllerv1.ServiceInstanceSummary{
		ServiceName:           record.ServiceName,
		NodeId:                record.NodeID,
		RuntimeStatus:         record.RuntimeStatus,
		UpdatedAt:             record.UpdatedAt,
		IsDeclared:            record.IsDeclared,
		PendingDeployRevision: record.PendingDeployRevision,
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
		ServiceName:           record.ServiceName,
		NodeId:                record.NodeID,
		RuntimeStatus:         record.RuntimeStatus,
		UpdatedAt:             record.UpdatedAt,
		IsDeclared:            record.IsDeclared,
		Containers:            containers,
		PendingDeployRevision: record.PendingDeployRevision,
	}
}

func buildServiceInstanceDetail(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, dockerQueries *dockerQueryBroker, service repo.Service, instance store.ServiceInstanceSnapshot, source task.Source) (*controllerv1.ServiceInstanceDetail, error) {
	dockerQuery := &dockerQueryServer{db: db, cfg: cfg, dockerQueries: dockerQueries}
	containers, err := listServiceInstanceContainers(ctx, dockerQuery, service, instance.NodeID, source)
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

func listServiceInstanceContainers(ctx context.Context, dockerQuery *dockerQueryServer, service repo.Service, nodeID string, source task.Source) ([]*controllerv1.ServiceContainerSummary, error) {
	if dockerQuery == nil {
		return nil, nil
	}
	result, err := dockerQuery.executeDockerListQuery(ctx, sourceHeader(source), nodeID, "containers", 0, 0, "", "", false)
	if err != nil {
		return nil, err
	}
	projectName := repo.ComposeProjectName(service.Meta.ProjectName, service.Name)
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
	if !req.Msg.GetIncludeContainers() {
		return connect.NewResponse(&controllerv1.GetServiceInstanceResponse{Instance: serviceInstanceDetailMessage(instance, nil)}), nil
	}
	detail, err := buildServiceInstanceDetail(ctx, server.db, server.cfg, server.dockerQueries, service, instance, requestTaskSource(req.Header()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.GetServiceInstanceResponse{Instance: detail}), nil
}
