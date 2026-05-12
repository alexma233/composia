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
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	dockerQueryMaxWait      = 30 * time.Second
	dockerQueryCleanupGrace = time.Minute
)

type dockerAgentQuery struct {
	QueryID    string
	NodeID     string
	Action     string
	Resource   string
	ID         string
	Tail       string
	Timestamps bool
	PageSize   uint32
	Page       uint32
	Search     string
	SortBy     string
	SortDesc   bool
	Command    []string
	Stdin      []byte
	Timeout    time.Duration
	MaxOutput  uint64
	expiresAt  time.Time
}

type dockerAgentQueryResult struct {
	QueryID      string
	NodeID       string
	PayloadJSON  string
	ErrorMessage string
	ErrorCode    string
}

type assignedDockerQuery struct {
	nodeID    string
	expiresAt time.Time
}

type storedDockerQueryResult struct {
	result    dockerAgentQueryResult
	expiresAt time.Time
}

type dockerQueryBroker struct {
	mu                sync.Mutex
	pendingByNode     map[string][]dockerAgentQuery
	nodeSubscribers   map[string]map[chan struct{}]struct{}
	resultSubscribers map[string]map[chan struct{}]struct{}
	results           map[string]storedDockerQueryResult
	assignedByID      map[string]assignedDockerQuery
}

func newDockerQueryBroker() *dockerQueryBroker {
	return &dockerQueryBroker{
		pendingByNode:     make(map[string][]dockerAgentQuery),
		nodeSubscribers:   make(map[string]map[chan struct{}]struct{}),
		resultSubscribers: make(map[string]map[chan struct{}]struct{}),
		results:           make(map[string]storedDockerQueryResult),
		assignedByID:      make(map[string]assignedDockerQuery),
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
	now := time.Now().UTC()
	if query.expiresAt.IsZero() {
		query.expiresAt = now.Add(dockerQueryTTL(query))
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.sweepExpiredLocked(now)
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
	broker.sweepExpiredLocked(time.Now().UTC())
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
	broker.assignedByID[query.QueryID] = assignedDockerQuery{nodeID: nodeID, expiresAt: query.expiresAt}
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
	now := time.Now().UTC()
	broker.sweepExpiredLocked(now)
	assigned := broker.assignedByID[result.QueryID]
	if assigned.nodeID == "" {
		return fmt.Errorf("docker query %q is not pending", result.QueryID)
	}
	if assigned.nodeID != result.NodeID {
		return fmt.Errorf("docker query %q belongs to node %q, got %q", result.QueryID, assigned.nodeID, result.NodeID)
	}
	delete(broker.assignedByID, result.QueryID)
	broker.results[result.QueryID] = storedDockerQueryResult{result: result, expiresAt: assigned.expiresAt.Add(dockerQueryCleanupGrace)}
	broker.notifyResultLocked(result.QueryID)
	return nil
}

func (broker *dockerQueryBroker) PopResult(queryID string) (dockerAgentQueryResult, bool) {
	if broker == nil || queryID == "" {
		return dockerAgentQueryResult{}, false
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.sweepExpiredLocked(time.Now().UTC())
	stored, ok := broker.results[queryID]
	if !ok {
		return dockerAgentQueryResult{}, false
	}
	delete(broker.results, queryID)
	return stored.result, true
}

func (broker *dockerQueryBroker) Cancel(queryID string) {
	if broker == nil || queryID == "" {
		return
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	delete(broker.results, queryID)
	delete(broker.assignedByID, queryID)
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

func dockerQueryTTL(query dockerAgentQuery) time.Duration {
	ttl := dockerQueryMaxWait + dockerQueryCleanupGrace
	if query.Timeout > 0 && query.Timeout+dockerQueryCleanupGrace > ttl {
		ttl = query.Timeout + dockerQueryCleanupGrace
	}
	return ttl
}

func (broker *dockerQueryBroker) sweepExpiredLocked(now time.Time) {
	for nodeID, queue := range broker.pendingByNode {
		filtered := queue[:0]
		for _, query := range queue {
			if !query.expiresAt.IsZero() && !query.expiresAt.After(now) {
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
	for queryID, assigned := range broker.assignedByID {
		if !assigned.expiresAt.IsZero() && !assigned.expiresAt.After(now) {
			delete(broker.assignedByID, queryID)
		}
	}
	for queryID, stored := range broker.results {
		if !stored.expiresAt.IsZero() && !stored.expiresAt.After(now) {
			delete(broker.results, queryID)
		}
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
			protoQuery, err := dockerProtoQueryTask(query)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			return connect.NewResponse(&agentv1.PullNextDockerQueryResponse{
				HasQuery: true,
				Query:    protoQuery,
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
	payloadJSON, err := dockerQueryPayloadJSON(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := server.dockerQueries.StoreResult(dockerAgentQueryResult{
		QueryID:      req.Msg.GetQueryId(),
		NodeID:       req.Msg.GetNodeId(),
		PayloadJSON:  payloadJSON,
		ErrorMessage: req.Msg.GetErrorMessage(),
		ErrorCode:    dockerQueryErrorCodeText(req.Msg.GetErrorCode()),
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
	waitTimeout := dockerQueryMaxWait
	if query.Timeout > 0 && query.Timeout+5*time.Second > waitTimeout {
		waitTimeout = query.Timeout + 5*time.Second
	}
	result, err := broker.WaitResult(ctx, queryID, waitTimeout)
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

func dockerProtoQueryTask(query dockerAgentQuery) (*agentv1.DockerQueryTask, error) {
	message := &agentv1.DockerQueryTask{QueryId: query.QueryID, NodeId: query.NodeID}
	switch query.Action {
	case "list":
		switch query.Resource {
		case "containers":
			message.Query = &agentv1.DockerQueryTask_ListContainers{ListContainers: &agentv1.ListContainersRequest{PageSize: query.PageSize, Page: query.Page, Search: query.Search, SortBy: query.SortBy, SortDesc: query.SortDesc}}
		case "networks":
			message.Query = &agentv1.DockerQueryTask_ListNetworks{ListNetworks: &agentv1.ListNetworksRequest{PageSize: query.PageSize, Page: query.Page, Search: query.Search, SortBy: query.SortBy, SortDesc: query.SortDesc}}
		case "volumes":
			message.Query = &agentv1.DockerQueryTask_ListVolumes{ListVolumes: &agentv1.ListVolumesRequest{PageSize: query.PageSize, Page: query.Page, Search: query.Search, SortBy: query.SortBy, SortDesc: query.SortDesc}}
		case "images":
			message.Query = &agentv1.DockerQueryTask_ListImages{ListImages: &agentv1.ListImagesRequest{PageSize: query.PageSize, Page: query.Page, Search: query.Search, SortBy: query.SortBy, SortDesc: query.SortDesc}}
		default:
			return nil, fmt.Errorf("unsupported docker list resource %q", query.Resource)
		}
	case "inspect":
		switch query.Resource {
		case "container":
			message.Query = &agentv1.DockerQueryTask_InspectContainer{InspectContainer: &agentv1.InspectContainerRequest{ContainerId: query.ID}}
		case "network":
			message.Query = &agentv1.DockerQueryTask_InspectNetwork{InspectNetwork: &agentv1.InspectNetworkRequest{NetworkId: query.ID}}
		case "volume":
			message.Query = &agentv1.DockerQueryTask_InspectVolume{InspectVolume: &agentv1.InspectVolumeRequest{VolumeName: query.ID}}
		case "image":
			message.Query = &agentv1.DockerQueryTask_InspectImage{InspectImage: &agentv1.InspectImageRequest{ImageId: query.ID}}
		default:
			return nil, fmt.Errorf("unsupported docker inspect resource %q", query.Resource)
		}
	case "logs":
		if query.Resource != "container" {
			return nil, fmt.Errorf("unsupported docker logs resource %q", query.Resource)
		}
		message.Query = &agentv1.DockerQueryTask_GetContainerLogs{GetContainerLogs: &agentv1.GetContainerLogsRequest{ContainerId: query.ID, Tail: query.Tail, Timestamps: query.Timestamps}}
	case "exec":
		if query.Resource != "container" {
			return nil, fmt.Errorf("unsupported docker exec resource %q", query.Resource)
		}
		timeoutSeconds := uint32(0)
		if query.Timeout > 0 {
			timeoutSeconds = uint32(query.Timeout.Round(time.Second) / time.Second)
		}
		message.Query = &agentv1.DockerQueryTask_RunContainerExec{RunContainerExec: &agentv1.DockerQueryRunContainerExecRequest{ContainerId: query.ID, Command: append([]string(nil), query.Command...), Stdin: append([]byte(nil), query.Stdin...), TimeoutSeconds: timeoutSeconds, MaxOutputBytes: query.MaxOutput}}
	default:
		return nil, fmt.Errorf("unsupported docker query action %q", query.Action)
	}
	return message, nil
}

func dockerQueryPayloadJSON(req *agentv1.ReportDockerQueryResultRequest) (string, error) {
	switch result := req.Result.(type) {
	case *agentv1.ReportDockerQueryResultRequest_ListContainers:
		return marshalDockerQueryPayload(dockerListResult{Containers: controllerContainerInfos(result.ListContainers.GetContainers()), TotalCount: result.ListContainers.GetTotalCount()})
	case *agentv1.ReportDockerQueryResultRequest_InspectContainer:
		return marshalDockerQueryPayload(dockerListResult{RawJSON: result.InspectContainer.GetRawJson()})
	case *agentv1.ReportDockerQueryResultRequest_ListNetworks:
		return marshalDockerQueryPayload(dockerListResult{Networks: controllerNetworkInfos(result.ListNetworks.GetNetworks()), TotalCount: result.ListNetworks.GetTotalCount()})
	case *agentv1.ReportDockerQueryResultRequest_InspectNetwork:
		return marshalDockerQueryPayload(dockerListResult{RawJSON: result.InspectNetwork.GetRawJson()})
	case *agentv1.ReportDockerQueryResultRequest_ListVolumes:
		return marshalDockerQueryPayload(dockerListResult{Volumes: controllerVolumeInfos(result.ListVolumes.GetVolumes()), TotalCount: result.ListVolumes.GetTotalCount()})
	case *agentv1.ReportDockerQueryResultRequest_InspectVolume:
		return marshalDockerQueryPayload(dockerListResult{RawJSON: result.InspectVolume.GetRawJson()})
	case *agentv1.ReportDockerQueryResultRequest_ListImages:
		return marshalDockerQueryPayload(dockerListResult{Images: controllerImageInfos(result.ListImages.GetImages()), TotalCount: result.ListImages.GetTotalCount()})
	case *agentv1.ReportDockerQueryResultRequest_InspectImage:
		return marshalDockerQueryPayload(dockerListResult{RawJSON: result.InspectImage.GetRawJson()})
	case *agentv1.ReportDockerQueryResultRequest_GetContainerLogs:
		return marshalDockerQueryPayload(dockerListResult{Content: result.GetContainerLogs.GetContent()})
	case *agentv1.ReportDockerQueryResultRequest_RunContainerExec:
		execResult := result.RunContainerExec
		return marshalDockerQueryPayload(dockerListResult{Exec: &dockerExecResult{ExitCode: execResult.GetExitCode(), Stdout: execResult.GetStdout(), Stderr: execResult.GetStderr(), TimedOut: execResult.GetTimedOut(), StdoutTruncated: execResult.GetStdoutTruncated(), StderrTruncated: execResult.GetStderrTruncated(), StartedAt: protoTimestampString(execResult.GetStartedAt()), FinishedAt: protoTimestampString(execResult.GetFinishedAt()), Duration: execResult.GetDuration()}})
	default:
		if req.GetErrorCode() != agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_UNSPECIFIED || req.GetErrorMessage() != "" {
			return "", nil
		}
		return "", errors.New("docker query result is required")
	}
}

func marshalDockerQueryPayload(payload dockerListResult) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode docker query result: %w", err)
	}
	return string(encoded), nil
}

func dockerQueryErrorCodeText(value agentv1.DockerQueryErrorCode) string {
	switch value {
	case agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_INVALID_ARGUMENT:
		return "invalid_argument"
	case agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_NOT_FOUND:
		return "not_found"
	case agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_FAILED_PRECONDITION:
		return "failed_precondition"
	case agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_PERMISSION_DENIED:
		return "permission_denied"
	case agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_DEADLINE_EXCEEDED:
		return "deadline_exceeded"
	case agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_UNAVAILABLE:
		return "unavailable"
	case agentv1.DockerQueryErrorCode_DOCKER_QUERY_ERROR_CODE_INTERNAL:
		return "internal"
	default:
		return ""
	}
}

func protoTimestampString(value *timestamppb.Timestamp) string {
	if value == nil {
		return ""
	}
	return value.AsTime().UTC().Format(time.RFC3339)
}

func controllerContainerInfos(values []*agentv1.ContainerInfo) []*controllerv1.ContainerInfo {
	infos := make([]*controllerv1.ContainerInfo, 0, len(values))
	for _, value := range values {
		infos = append(infos, &controllerv1.ContainerInfo{Id: value.GetId(), Name: value.GetName(), Image: value.GetImage(), State: value.GetState(), Status: value.GetStatus(), Created: value.GetCreated(), Labels: value.GetLabels(), Ports: append([]string(nil), value.GetPorts()...), Networks: append([]string(nil), value.GetNetworks()...), ImageId: value.GetImageId()})
	}
	return infos
}

func controllerNetworkInfos(values []*agentv1.NetworkInfo) []*controllerv1.NetworkInfo {
	infos := make([]*controllerv1.NetworkInfo, 0, len(values))
	for _, value := range values {
		infos = append(infos, &controllerv1.NetworkInfo{Id: value.GetId(), Name: value.GetName(), Driver: value.GetDriver(), Scope: value.GetScope(), Internal: value.GetInternal(), Attachable: value.GetAttachable(), Created: value.GetCreated(), Labels: value.GetLabels(), Subnet: value.GetSubnet(), Gateway: value.GetGateway(), ContainersCount: value.GetContainersCount(), Ipv6Enabled: value.GetIpv6Enabled()})
	}
	return infos
}

func controllerVolumeInfos(values []*agentv1.VolumeInfo) []*controllerv1.VolumeInfo {
	infos := make([]*controllerv1.VolumeInfo, 0, len(values))
	for _, value := range values {
		infos = append(infos, &controllerv1.VolumeInfo{Name: value.GetName(), Driver: value.GetDriver(), Mountpoint: value.GetMountpoint(), Scope: value.GetScope(), Created: value.GetCreated(), Labels: value.GetLabels(), SizeBytes: value.GetSizeBytes(), ContainersCount: value.GetContainersCount(), InUse: value.GetInUse()})
	}
	return infos
}

func controllerImageInfos(values []*agentv1.ImageInfo) []*controllerv1.ImageInfo {
	infos := make([]*controllerv1.ImageInfo, 0, len(values))
	for _, value := range values {
		infos = append(infos, &controllerv1.ImageInfo{Id: value.GetId(), RepoTags: append([]string(nil), value.GetRepoTags()...), Size: value.GetSize(), Created: value.GetCreated(), RepoDigests: append([]string(nil), value.GetRepoDigests()...), VirtualSize: value.GetVirtualSize(), Architecture: value.GetArchitecture(), Os: value.GetOs(), ContainersCount: value.GetContainersCount(), IsDangling: value.GetIsDangling()})
	}
	return infos
}
