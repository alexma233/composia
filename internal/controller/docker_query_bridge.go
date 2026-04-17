package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"github.com/google/uuid"
)

const dockerQueryMaxWait = 30 * time.Second

type dockerAgentQuery struct {
	QueryID    string
	NodeID     string
	Action     string
	Resource   string
	ID         string
	Tail       string
	Timestamps bool
	Since      string
	PageSize   uint32
	Page       uint32
	Search     string
	SortBy     string
	SortDesc   bool
}

type dockerAgentQueryResult struct {
	QueryID      string
	NodeID       string
	PayloadJSON  string
	ErrorMessage string
	ErrorCode    string
}

type dockerQueryBroker struct {
	mu                sync.Mutex
	pendingByNode     map[string][]dockerAgentQuery
	nodeSubscribers   map[string]map[chan struct{}]struct{}
	resultSubscribers map[string]map[chan struct{}]struct{}
	results           map[string]dockerAgentQueryResult
	assignedNodeByID  map[string]string
}

func newDockerQueryBroker() *dockerQueryBroker {
	return &dockerQueryBroker{
		pendingByNode:     make(map[string][]dockerAgentQuery),
		nodeSubscribers:   make(map[string]map[chan struct{}]struct{}),
		resultSubscribers: make(map[string]map[chan struct{}]struct{}),
		results:           make(map[string]dockerAgentQueryResult),
		assignedNodeByID:  make(map[string]string),
	}
}

func (broker *dockerQueryBroker) SubscribeNode(nodeID string) chan struct{} {
	if broker == nil || nodeID == "" {
		return nil
	}
	ch := make(chan struct{}, 1)
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.nodeSubscribers[nodeID] == nil {
		broker.nodeSubscribers[nodeID] = make(map[chan struct{}]struct{})
	}
	broker.nodeSubscribers[nodeID][ch] = struct{}{}
	return ch
}

func (broker *dockerQueryBroker) UnsubscribeNode(nodeID string, ch chan struct{}) {
	if broker == nil || nodeID == "" || ch == nil {
		return
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	subscribers := broker.nodeSubscribers[nodeID]
	if subscribers == nil {
		return
	}
	if _, ok := subscribers[ch]; ok {
		delete(subscribers, ch)
		close(ch)
	}
	if len(subscribers) == 0 {
		delete(broker.nodeSubscribers, nodeID)
	}
}

func (broker *dockerQueryBroker) Enqueue(query dockerAgentQuery) string {
	if broker == nil {
		return ""
	}
	if query.QueryID == "" {
		query.QueryID = uuid.NewString()
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.pendingByNode[query.NodeID] = append(broker.pendingByNode[query.NodeID], query)
	broker.notifyNodeLocked(query.NodeID)
	return query.QueryID
}

func (broker *dockerQueryBroker) Pull(nodeID string) (dockerAgentQuery, bool) {
	if broker == nil || nodeID == "" {
		return dockerAgentQuery{}, false
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	queue := broker.pendingByNode[nodeID]
	if len(queue) == 0 {
		return dockerAgentQuery{}, false
	}
	query := queue[0]
	if len(queue) == 1 {
		delete(broker.pendingByNode, nodeID)
	} else {
		broker.pendingByNode[nodeID] = queue[1:]
	}
	broker.assignedNodeByID[query.QueryID] = nodeID
	return query, true
}

func (broker *dockerQueryBroker) SubscribeResult(queryID string) chan struct{} {
	if broker == nil || queryID == "" {
		return nil
	}
	ch := make(chan struct{}, 1)
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.resultSubscribers[queryID] == nil {
		broker.resultSubscribers[queryID] = make(map[chan struct{}]struct{})
	}
	broker.resultSubscribers[queryID][ch] = struct{}{}
	return ch
}

func (broker *dockerQueryBroker) UnsubscribeResult(queryID string, ch chan struct{}) {
	if broker == nil || queryID == "" || ch == nil {
		return
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	subscribers := broker.resultSubscribers[queryID]
	if subscribers == nil {
		return
	}
	if _, ok := subscribers[ch]; ok {
		delete(subscribers, ch)
		close(ch)
	}
	if len(subscribers) == 0 {
		delete(broker.resultSubscribers, queryID)
	}
}

func (broker *dockerQueryBroker) StoreResult(result dockerAgentQueryResult) error {
	if broker == nil {
		return errors.New("docker query broker is not configured")
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	expectedNodeID := broker.assignedNodeByID[result.QueryID]
	if expectedNodeID == "" {
		return fmt.Errorf("docker query %q is not pending", result.QueryID)
	}
	if expectedNodeID != result.NodeID {
		return fmt.Errorf("docker query %q belongs to node %q, got %q", result.QueryID, expectedNodeID, result.NodeID)
	}
	delete(broker.assignedNodeByID, result.QueryID)
	broker.results[result.QueryID] = result
	broker.notifyResultLocked(result.QueryID)
	return nil
}

func (broker *dockerQueryBroker) PopResult(queryID string) (dockerAgentQueryResult, bool) {
	if broker == nil || queryID == "" {
		return dockerAgentQueryResult{}, false
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	result, ok := broker.results[queryID]
	if !ok {
		return dockerAgentQueryResult{}, false
	}
	delete(broker.results, queryID)
	return result, true
}

func (broker *dockerQueryBroker) Cancel(queryID string) {
	if broker == nil || queryID == "" {
		return
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	delete(broker.results, queryID)
	delete(broker.assignedNodeByID, queryID)
	for nodeID, queue := range broker.pendingByNode {
		filtered := queue[:0]
		for _, query := range queue {
			if query.QueryID == queryID {
				continue
			}
			filtered = append(filtered, query)
		}
		if len(filtered) == 0 {
			delete(broker.pendingByNode, nodeID)
			continue
		}
		broker.pendingByNode[nodeID] = filtered
	}
}

func (broker *dockerQueryBroker) WaitResult(ctx context.Context, queryID string, timeout time.Duration) (dockerAgentQueryResult, error) {
	if broker == nil {
		return dockerAgentQueryResult{}, errors.New("docker query broker is not configured")
	}
	deadline := time.Now().Add(timeout)
	waitCh := broker.SubscribeResult(queryID)
	defer broker.UnsubscribeResult(queryID, waitCh)

	for {
		if result, ok := broker.PopResult(queryID); ok {
			return result, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			broker.Cancel(queryID)
			return dockerAgentQueryResult{}, fmt.Errorf("timeout waiting for docker query result")
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			broker.Cancel(queryID)
			return dockerAgentQueryResult{}, ctx.Err()
		case <-waitCh:
			timer.Stop()
		case <-timer.C:
			broker.Cancel(queryID)
			return dockerAgentQueryResult{}, fmt.Errorf("timeout waiting for docker query result")
		}
	}
}

func (broker *dockerQueryBroker) notifyNodeLocked(nodeID string) {
	for ch := range broker.nodeSubscribers[nodeID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (broker *dockerQueryBroker) notifyResultLocked(queryID string) {
	for ch := range broker.resultSubscribers[queryID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (server *agentTaskServer) PullNextDockerQuery(ctx context.Context, req *connect.Request[agentv1.PullNextDockerQueryRequest]) (*connect.Response[agentv1.PullNextDockerQueryResponse], error) {
	if req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id is required"))
	}
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}
	if server.dockerQueries == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("docker query broker is not configured"))
	}

	waitCh := server.dockerQueries.SubscribeNode(req.Msg.GetNodeId())
	defer server.dockerQueries.UnsubscribeNode(req.Msg.GetNodeId(), waitCh)
	deadline := time.Now().Add(server.longPollMaxWait())

	for {
		query, ok := server.dockerQueries.Pull(req.Msg.GetNodeId())
		if ok {
			return connect.NewResponse(&agentv1.PullNextDockerQueryResponse{
				HasQuery: true,
				Query: &agentv1.DockerQueryTask{
					QueryId:    query.QueryID,
					NodeId:     query.NodeID,
					Action:     query.Action,
					Resource:   query.Resource,
					Id:         query.ID,
					Tail:       query.Tail,
					Timestamps: query.Timestamps,
					Since:      query.Since,
					PageSize:   query.PageSize,
					Page:       query.Page,
					Search:     query.Search,
					SortBy:     query.SortBy,
					SortDesc:   query.SortDesc,
				},
			}), nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return connect.NewResponse(&agentv1.PullNextDockerQueryResponse{HasQuery: false}), nil
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

func (server *agentReportServer) ReportDockerQueryResult(ctx context.Context, req *connect.Request[agentv1.ReportDockerQueryResultRequest]) (*connect.Response[agentv1.ReportDockerQueryResultResponse], error) {
	if req.Msg.GetQueryId() == "" || req.Msg.GetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("query_id and node_id are required"))
	}
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || authenticatedNodeID != req.Msg.GetNodeId() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("node_id does not match bearer token"))
	}
	if server.dockerQueries == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("docker query broker is not configured"))
	}
	if err := server.dockerQueries.StoreResult(dockerAgentQueryResult{
		QueryID:      req.Msg.GetQueryId(),
		NodeID:       req.Msg.GetNodeId(),
		PayloadJSON:  req.Msg.GetPayloadJson(),
		ErrorMessage: req.Msg.GetErrorMessage(),
		ErrorCode:    req.Msg.GetErrorCode(),
	}); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&agentv1.ReportDockerQueryResultResponse{}), nil
}

func executeDockerAgentQuery(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, broker *dockerQueryBroker, nodeID string, query dockerAgentQuery) (dockerAgentQueryResult, error) {
	if broker == nil {
		return dockerAgentQueryResult{}, connect.NewError(connect.CodeInternal, errors.New("docker query broker is not configured"))
	}
	if err := validateNodeForDockerQuery(ctx, db, cfg, nodeID); err != nil {
		return dockerAgentQueryResult{}, err
	}
	query.NodeID = nodeID
	queryID := broker.Enqueue(query)
	result, err := broker.WaitResult(ctx, queryID, dockerQueryMaxWait)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return dockerAgentQueryResult{}, connect.NewError(connect.CodeCanceled, err)
		case errors.Is(err, context.DeadlineExceeded):
			return dockerAgentQueryResult{}, connect.NewError(connect.CodeDeadlineExceeded, err)
		default:
			return dockerAgentQueryResult{}, connect.NewError(connect.CodeDeadlineExceeded, err)
		}
	}
	if result.ErrorCode != "" {
		return dockerAgentQueryResult{}, connect.NewError(dockerQueryConnectCode(result.ErrorCode), errors.New(result.ErrorMessage))
	}
	return result, nil
}

func (server *dockerQueryServer) executeDockerListQuery(ctx context.Context, header http.Header, nodeID, resource string, page, pageSize uint32, search, sortBy string, sortDesc bool) (*dockerListResult, error) {
	_ = header
	result, err := executeDockerAgentQuery(ctx, server.db, server.cfg, server.dockerQueries, nodeID, dockerAgentQuery{
		Action:   "list",
		Resource: resource,
		Page:     page,
		PageSize: pageSize,
		Search:   search,
		SortBy:   sortBy,
		SortDesc: sortDesc,
	})
	if err != nil {
		return nil, err
	}
	var payload dockerListResult
	if err := json.Unmarshal([]byte(result.PayloadJSON), &payload); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("decode docker list result: %w", err))
	}
	return &payload, nil
}

func (server *dockerQueryServer) executeDockerInspectQuery(ctx context.Context, header http.Header, nodeID, resource, id string) (*dockerListResult, error) {
	_ = header
	result, err := executeDockerAgentQuery(ctx, server.db, server.cfg, server.dockerQueries, nodeID, dockerAgentQuery{
		Action:   "inspect",
		Resource: resource,
		ID:       id,
	})
	if err != nil {
		return nil, err
	}
	var payload dockerListResult
	if err := json.Unmarshal([]byte(result.PayloadJSON), &payload); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("decode docker inspect result: %w", err))
	}
	return &payload, nil
}

func (server *containerServer) executeContainerLogsQuery(ctx context.Context, nodeID, containerID, tail string, timestamps bool, since string) (*dockerListResult, error) {
	result, err := executeDockerAgentQuery(ctx, server.db, server.cfg, server.dockerQueries, nodeID, dockerAgentQuery{
		Action:     "logs",
		Resource:   "container",
		ID:         containerID,
		Tail:       tail,
		Timestamps: timestamps,
		Since:      since,
	})
	if err != nil {
		return nil, err
	}
	var payload dockerListResult
	if err := json.Unmarshal([]byte(result.PayloadJSON), &payload); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("decode docker logs result: %w", err))
	}
	return &payload, nil
}

func validateNodeForDockerQuery(ctx context.Context, db *store.DB, cfg *config.ControllerConfig, nodeID string) error {
	return validateTaskTargetNode(ctx, db, cfg, nodeID, task.TypeDockerStart)
}

func dockerQueryConnectCode(value string) connect.Code {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "invalid_argument":
		return connect.CodeInvalidArgument
	case "not_found":
		return connect.CodeNotFound
	case "failed_precondition":
		return connect.CodeFailedPrecondition
	case "permission_denied":
		return connect.CodePermissionDenied
	case "deadline_exceeded":
		return connect.CodeDeadlineExceeded
	case "unavailable":
		return connect.CodeUnavailable
	default:
		return connect.CodeInternal
	}
}

func dockerControllerContainerInfo(item *agentv1.ContainerInfo) *controllerv1.ContainerInfo {
	if item == nil {
		return nil
	}
	return &controllerv1.ContainerInfo{
		Id:       item.GetId(),
		Name:     item.GetName(),
		Image:    item.GetImage(),
		State:    item.GetState(),
		Status:   item.GetStatus(),
		Created:  item.GetCreated(),
		Labels:   item.GetLabels(),
		Ports:    append([]string(nil), item.GetPorts()...),
		Networks: append([]string(nil), item.GetNetworks()...),
		ImageId:  item.GetImageId(),
	}
}

func dockerControllerNetworkInfo(item *agentv1.NetworkInfo) *controllerv1.NetworkInfo {
	if item == nil {
		return nil
	}
	return &controllerv1.NetworkInfo{
		Id:              item.GetId(),
		Name:            item.GetName(),
		Driver:          item.GetDriver(),
		Scope:           item.GetScope(),
		Internal:        item.GetInternal(),
		Attachable:      item.GetAttachable(),
		Created:         item.GetCreated(),
		Labels:          item.GetLabels(),
		Subnet:          item.GetSubnet(),
		Gateway:         item.GetGateway(),
		ContainersCount: item.GetContainersCount(),
		Ipv6Enabled:     item.GetIpv6Enabled(),
	}
}

func dockerControllerVolumeInfo(item *agentv1.VolumeInfo) *controllerv1.VolumeInfo {
	if item == nil {
		return nil
	}
	return &controllerv1.VolumeInfo{
		Name:            item.GetName(),
		Driver:          item.GetDriver(),
		Mountpoint:      item.GetMountpoint(),
		Scope:           item.GetScope(),
		Created:         item.GetCreated(),
		Labels:          item.GetLabels(),
		SizeBytes:       item.GetSizeBytes(),
		ContainersCount: item.GetContainersCount(),
		InUse:           item.GetInUse(),
	}
}

func dockerControllerImageInfo(item *agentv1.ImageInfo) *controllerv1.ImageInfo {
	if item == nil {
		return nil
	}
	return &controllerv1.ImageInfo{
		Id:              item.GetId(),
		RepoTags:        append([]string(nil), item.GetRepoTags()...),
		Size:            item.GetSize(),
		Created:         item.GetCreated(),
		RepoDigests:     append([]string(nil), item.GetRepoDigests()...),
		VirtualSize:     item.GetVirtualSize(),
		Architecture:    item.GetArchitecture(),
		Os:              item.GetOs(),
		ContainersCount: item.GetContainersCount(),
		IsDangling:      item.GetIsDangling(),
	}
}
