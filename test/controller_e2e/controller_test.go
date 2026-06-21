//go:build e2e

package controller_e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
)

const (
	defaultControllerAddr = "http://127.0.0.1:7001"
	defaultAccessToken    = "dev-admin-token"
	testNodeID            = "main"
	testServiceName       = "host-service"
)

type controllerClients struct {
	system         controllerv1connect.SystemServiceClient
	nodeQuery      controllerv1connect.NodeQueryServiceClient
	repoQuery      controllerv1connect.RepoQueryServiceClient
	repoCommand    controllerv1connect.RepoCommandServiceClient
	serviceQuery   controllerv1connect.ServiceQueryServiceClient
	serviceCommand controllerv1connect.ServiceCommandServiceClient
	serviceInst    controllerv1connect.ServiceInstanceServiceClient
	task           controllerv1connect.TaskServiceClient
}

func TestControllerE2EAuth(t *testing.T) {
	ctx := testContext(t)
	baseURL := controllerAPIBaseURL()
	waitForController(t, newControllerClients())
	client := controllerv1connect.NewSystemServiceClient(http.DefaultClient, baseURL)

	_, err := client.GetSystemStatus(ctx, connect.NewRequest(&controllerv1.GetSystemStatusRequest{}))
	assertConnectCode(t, err, connect.CodeUnauthenticated)

	badClient := controllerv1connect.NewSystemServiceClient(
		http.DefaultClient,
		baseURL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("not-the-token")),
	)
	_, err = badClient.GetSystemStatus(ctx, connect.NewRequest(&controllerv1.GetSystemStatusRequest{}))
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestControllerE2ESystemAndNodes(t *testing.T) {
	ctx := testContext(t)
	clients := newControllerClients()
	waitForController(t, clients)

	status, err := clients.system.GetSystemStatus(ctx, connect.NewRequest(&controllerv1.GetSystemStatusRequest{}))
	if err != nil {
		t.Fatalf("get system status: %v", err)
	}
	if status.Msg.GetVersion() == "" {
		t.Fatalf("expected system status version")
	}
	if status.Msg.GetConfiguredNodeCount() != 1 {
		t.Fatalf("configured node count = %d, want 1", status.Msg.GetConfiguredNodeCount())
	}
	if status.Msg.GetServiceCount() == 0 {
		t.Fatalf("expected at least one declared service")
	}

	config, err := clients.system.GetCurrentConfig(ctx, connect.NewRequest(&controllerv1.GetCurrentConfigRequest{}))
	if err != nil {
		t.Fatalf("get current config: %v", err)
	}
	if config.Msg.GetListenAddr() == "" {
		t.Fatalf("expected redacted config listen address")
	}
	if !hasNodeConfig(config.Msg.GetNodes(), testNodeID) {
		t.Fatalf("expected config to include node %q", testNodeID)
	}
	if !hasAccessToken(config.Msg.GetAccessTokens(), "web") {
		t.Fatalf("expected config to include web access token metadata")
	}

	capabilities, err := clients.system.GetCapabilities(ctx, connect.NewRequest(&controllerv1.GetCapabilitiesRequest{}))
	if err != nil {
		t.Fatalf("get capabilities: %v", err)
	}
	if capabilities.Msg.GetGlobal() == nil || capabilities.Msg.GetGlobal().GetBackup().GetEnabled() {
		t.Fatalf("expected backup capability to be present and disabled in e2e fixture")
	}

	nodes, err := clients.nodeQuery.ListNodes(ctx, connect.NewRequest(&controllerv1.ListNodesRequest{}))
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if !hasNodeSummary(nodes.Msg.GetNodes(), testNodeID) {
		t.Fatalf("expected node list to include %q", testNodeID)
	}

	node, err := clients.nodeQuery.GetNode(ctx, connect.NewRequest(&controllerv1.GetNodeRequest{NodeId: testNodeID}))
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node.Msg.GetNode().GetDisplayName() != "Main" {
		t.Fatalf("node display name = %q, want Main", node.Msg.GetNode().GetDisplayName())
	}

	_, err = clients.nodeQuery.GetNode(ctx, connect.NewRequest(&controllerv1.GetNodeRequest{NodeId: "missing-node"}))
	assertConnectCode(t, err, connect.CodeNotFound)
}

func TestControllerE2ERepoAndServices(t *testing.T) {
	ctx := testContext(t)
	clients := newControllerClients()
	waitForController(t, clients)

	head := repoHead(t, ctx, clients)
	if head.GetHeadRevision() == "" {
		t.Fatalf("expected repo head revision")
	}
	if !head.GetCleanWorktree() {
		t.Fatalf("expected e2e repo worktree to be clean")
	}

	files, err := clients.repoQuery.ListRepoFiles(ctx, connect.NewRequest(&controllerv1.ListRepoFilesRequest{}))
	if err != nil {
		t.Fatalf("list repo files: %v", err)
	}
	if !hasRepoEntry(files.Msg.GetEntries(), "host-service", true) {
		t.Fatalf("expected repo root to include host-service directory")
	}

	meta, err := clients.repoQuery.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: "host-service/composia-meta.yaml"}))
	if err != nil {
		t.Fatalf("get repo file: %v", err)
	}
	if !strings.Contains(meta.Msg.GetContent(), "name: host-service") {
		t.Fatalf("unexpected host-service metadata:\n%s", meta.Msg.GetContent())
	}

	validation, err := clients.repoQuery.ValidateRepo(ctx, connect.NewRequest(&controllerv1.ValidateRepoRequest{}))
	if err != nil {
		t.Fatalf("validate repo: %v", err)
	}
	if len(validation.Msg.GetErrors()) != 0 {
		t.Fatalf("expected valid fixture repo, got %d errors", len(validation.Msg.GetErrors()))
	}

	commits, err := clients.repoQuery.ListRepoCommits(ctx, connect.NewRequest(&controllerv1.ListRepoCommitsRequest{PageSize: 5}))
	if err != nil {
		t.Fatalf("list repo commits: %v", err)
	}
	if len(commits.Msg.GetCommits()) == 0 {
		t.Fatalf("expected seeded repo commit history")
	}

	workspaces, err := clients.serviceQuery.ListServiceWorkspaces(ctx, connect.NewRequest(&controllerv1.ListServiceWorkspacesRequest{}))
	if err != nil {
		t.Fatalf("list service workspaces: %v", err)
	}
	if !hasWorkspace(workspaces.Msg.GetWorkspaces(), "host-service", testServiceName) {
		t.Fatalf("expected host-service workspace")
	}

	services, err := clients.serviceQuery.ListServices(ctx, connect.NewRequest(&controllerv1.ListServicesRequest{PageSize: 10, Page: 1}))
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if !hasServiceSummary(services.Msg.GetServices(), testServiceName) {
		t.Fatalf("expected service list to include %q", testServiceName)
	}

	service, err := clients.serviceQuery.GetService(ctx, connect.NewRequest(&controllerv1.GetServiceRequest{ServiceName: testServiceName}))
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if service.Msg.GetName() != testServiceName || !containsString(service.Msg.GetNodes(), testNodeID) {
		t.Fatalf("unexpected service detail: %+v", service.Msg)
	}

	instances, err := clients.serviceInst.ListServiceInstances(ctx, connect.NewRequest(&controllerv1.ListServiceInstancesRequest{ServiceName: testServiceName}))
	if err != nil {
		t.Fatalf("list service instances: %v", err)
	}
	if !hasServiceInstance(instances.Msg.GetInstances(), testServiceName, testNodeID) {
		t.Fatalf("expected %s instance on %s", testServiceName, testNodeID)
	}

	_, err = clients.serviceQuery.GetService(ctx, connect.NewRequest(&controllerv1.GetServiceRequest{ServiceName: "missing-service"}))
	assertConnectCode(t, err, connect.CodeNotFound)
}

func TestControllerE2ERepoWrites(t *testing.T) {
	ctx := testContext(t)
	clients := newControllerClients()
	waitForController(t, clients)

	baseRevision := repoHead(t, ctx, clients).GetHeadRevision()
	workspace := fmt.Sprintf("controller-e2e-%d", time.Now().UnixNano())

	created, err := clients.repoCommand.CreateRepoDirectory(ctx, connect.NewRequest(&controllerv1.CreateRepoDirectoryRequest{
		Path:          workspace,
		BaseRevision:  baseRevision,
		CommitMessage: "Create controller e2e workspace",
	}))
	if err != nil {
		t.Fatalf("create repo directory: %v", err)
	}
	if created.Msg.GetCommitId() == "" {
		t.Fatalf("expected create directory commit id")
	}

	_, err = clients.repoCommand.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:          workspace + "/README.md",
		Content:       "# Controller E2E\n",
		BaseRevision:  baseRevision,
		CommitMessage: "Stale controller e2e write",
	}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	baseRevision = created.Msg.GetCommitId()
	updated, err := clients.repoCommand.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:          workspace + "/README.md",
		Content:       "# Controller E2E\n",
		BaseRevision:  baseRevision,
		CommitMessage: "Write controller e2e file",
	}))
	if err != nil {
		t.Fatalf("update repo file: %v", err)
	}

	file, err := clients.repoQuery.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: workspace + "/README.md"}))
	if err != nil {
		t.Fatalf("get written repo file: %v", err)
	}
	if file.Msg.GetContent() != "# Controller E2E\n" {
		t.Fatalf("written file content = %q", file.Msg.GetContent())
	}

	moved, err := clients.repoCommand.MoveRepoPath(ctx, connect.NewRequest(&controllerv1.MoveRepoPathRequest{
		SourcePath:      workspace + "/README.md",
		DestinationPath: workspace + "/NOTES.md",
		BaseRevision:    updated.Msg.GetCommitId(),
		CommitMessage:   "Move controller e2e file",
	}))
	if err != nil {
		t.Fatalf("move repo path: %v", err)
	}

	_, err = clients.repoQuery.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: workspace + "/NOTES.md"}))
	if err != nil {
		t.Fatalf("get moved repo file: %v", err)
	}

	deleted, err := clients.repoCommand.DeleteRepoPath(ctx, connect.NewRequest(&controllerv1.DeleteRepoPathRequest{
		Path:          workspace,
		BaseRevision:  moved.Msg.GetCommitId(),
		CommitMessage: "Delete controller e2e workspace",
	}))
	if err != nil {
		t.Fatalf("delete repo path: %v", err)
	}
	if deleted.Msg.GetCommitId() == "" {
		t.Fatalf("expected delete commit id")
	}

	_, err = clients.repoQuery.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: workspace + "/NOTES.md"}))
	assertConnectCode(t, err, connect.CodeNotFound)
}

func TestControllerE2ETasks(t *testing.T) {
	ctx := testContext(t)
	clients := newControllerClients()
	waitForController(t, clients)

	restartReq := sourceRequest(&controllerv1.RunServiceActionRequest{
		ServiceName: testServiceName,
		Action:      controllerv1.ServiceAction_SERVICE_ACTION_RESTART,
	}, "web")
	_, err := clients.serviceCommand.RunServiceAction(ctx, restartReq)
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	deployReq := sourceRequest(&controllerv1.RunServiceActionRequest{
		ServiceName: testServiceName,
		Action:      controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY,
	}, "web")
	deploy, err := clients.serviceCommand.RunServiceAction(ctx, deployReq)
	if err != nil {
		t.Fatalf("run deploy action: %v", err)
	}
	if len(deploy.Msg.GetTasks()) != 1 || deploy.Msg.GetTasks()[0].GetTaskId() == "" {
		t.Fatalf("expected one deploy task, got %+v", deploy.Msg.GetTasks())
	}

	taskID := deploy.Msg.GetTasks()[0].GetTaskId()
	taskDetail := waitForTaskStatus(t, ctx, clients, taskID, controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED)
	if taskDetail.GetType() != controllerv1.TaskType_TASK_TYPE_DEPLOY {
		t.Fatalf("task type = %s, want DEPLOY", taskDetail.GetType())
	}
	if taskDetail.GetSource() != controllerv1.TaskSource_TASK_SOURCE_WEB {
		t.Fatalf("task source = %s, want WEB", taskDetail.GetSource())
	}
	if taskDetail.GetServiceName() != testServiceName || taskDetail.GetNodeId() != testNodeID {
		t.Fatalf("unexpected task target: service=%q node=%q", taskDetail.GetServiceName(), taskDetail.GetNodeId())
	}
	if len(taskDetail.GetSteps()) == 0 {
		t.Fatalf("expected deploy task steps")
	}

	filtered, err := clients.task.ListTasks(ctx, connect.NewRequest(&controllerv1.ListTasksRequest{
		Status:      []controllerv1.TaskStatus{controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED},
		ServiceName: []string{testServiceName},
		Type:        []controllerv1.TaskType{controllerv1.TaskType_TASK_TYPE_DEPLOY},
		PageSize:    10,
		Page:        1,
	}))
	if err != nil {
		t.Fatalf("list filtered tasks: %v", err)
	}
	if !hasTaskSummary(filtered.Msg.GetTasks(), taskID) {
		t.Fatalf("expected filtered tasks to include %q", taskID)
	}

	rerun, err := clients.task.RunTaskAgain(ctx, sourceRequest(&controllerv1.RunTaskAgainRequest{TaskId: taskID}, "cli"))
	if err != nil {
		t.Fatalf("run task again: %v", err)
	}
	if rerun.Msg.GetTaskId() == "" || rerun.Msg.GetTaskId() == taskID {
		t.Fatalf("unexpected rerun task id %q", rerun.Msg.GetTaskId())
	}

	rerunDetail := waitForTaskStatus(t, ctx, clients, rerun.Msg.GetTaskId(), controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED)
	if rerunDetail.GetAttemptOfTaskId() != taskID {
		t.Fatalf("rerun attempt_of_task_id = %q, want %q", rerunDetail.GetAttemptOfTaskId(), taskID)
	}
	if rerunDetail.GetSource() != controllerv1.TaskSource_TASK_SOURCE_CLI {
		t.Fatalf("rerun source = %s, want CLI", rerunDetail.GetSource())
	}
}

func newControllerClients() controllerClients {
	baseURL := controllerAPIBaseURL()
	opts := []connect.ClientOption{connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(accessToken()))}
	return controllerClients{
		system:         controllerv1connect.NewSystemServiceClient(http.DefaultClient, baseURL, opts...),
		nodeQuery:      controllerv1connect.NewNodeQueryServiceClient(http.DefaultClient, baseURL, opts...),
		repoQuery:      controllerv1connect.NewRepoQueryServiceClient(http.DefaultClient, baseURL, opts...),
		repoCommand:    controllerv1connect.NewRepoCommandServiceClient(http.DefaultClient, baseURL, opts...),
		serviceQuery:   controllerv1connect.NewServiceQueryServiceClient(http.DefaultClient, baseURL, opts...),
		serviceCommand: controllerv1connect.NewServiceCommandServiceClient(http.DefaultClient, baseURL, opts...),
		serviceInst:    controllerv1connect.NewServiceInstanceServiceClient(http.DefaultClient, baseURL, opts...),
		task:           controllerv1connect.NewTaskServiceClient(http.DefaultClient, baseURL, opts...),
	}
}

func controllerAPIBaseURL() string {
	addr := strings.TrimSpace(os.Getenv("COMPOSIA_E2E_CONTROLLER_ADDR"))
	if addr == "" {
		addr = defaultControllerAddr
	}
	return rpcutil.JoinBaseURL(addr, rpcutil.ControllerAPIBasePath)
}

func accessToken() string {
	token := strings.TrimSpace(os.Getenv("COMPOSIA_E2E_ACCESS_TOKEN"))
	if token == "" {
		return defaultAccessToken
	}
	return token
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func waitForController(t *testing.T, clients controllerClients) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, lastErr = clients.system.GetSystemStatus(ctx, connect.NewRequest(&controllerv1.GetSystemStatusRequest{}))
		cancel()
		if lastErr == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("controller did not become ready: %v", lastErr)
}

func repoHead(t *testing.T, ctx context.Context, clients controllerClients) *controllerv1.GetRepoHeadResponse {
	t.Helper()
	head, err := clients.repoQuery.GetRepoHead(ctx, connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Fatalf("get repo head: %v", err)
	}
	return head.Msg
}

func waitForTaskStatus(t *testing.T, ctx context.Context, clients controllerClients, taskID string, want controllerv1.TaskStatus) *controllerv1.GetTaskResponse {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	var last *controllerv1.GetTaskResponse
	for time.Now().Before(deadline) {
		resp, err := clients.task.GetTask(ctx, connect.NewRequest(&controllerv1.GetTaskRequest{TaskId: taskID}))
		if err != nil {
			t.Fatalf("get task %s: %v", taskID, err)
		}
		last = resp.Msg
		if last.GetStatus() == want {
			return last
		}
		if isTerminalTaskStatus(last.GetStatus()) {
			t.Fatalf("task %s reached %s, want %s: %s", taskID, last.GetStatus(), want, last.GetErrorSummary())
		}
		time.Sleep(1 * time.Second)
	}
	if last == nil {
		t.Fatalf("task %s did not become visible", taskID)
	}
	t.Fatalf("task %s status = %s after timeout, want %s", taskID, last.GetStatus(), want)
	return nil
}

func sourceRequest[T any](msg *T, source string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Composia-Source", source)
	return req
}

func assertConnectCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected connect code %s, got nil", code)
	}
	if got := connect.CodeOf(err); got != code {
		t.Fatalf("connect code = %s, want %s: %v", got, code, err)
	}
}

func isTerminalTaskStatus(status controllerv1.TaskStatus) bool {
	switch status {
	case controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED, controllerv1.TaskStatus_TASK_STATUS_FAILED, controllerv1.TaskStatus_TASK_STATUS_CANCELLED:
		return true
	default:
		return false
	}
}

func hasNodeConfig(nodes []*controllerv1.NodeConfigSummary, nodeID string) bool {
	for _, node := range nodes {
		if node.GetId() == nodeID {
			return true
		}
	}
	return false
}

func hasAccessToken(tokens []*controllerv1.AccessTokenSummary, name string) bool {
	for _, token := range tokens {
		if token.GetName() == name && token.GetEnabled() {
			return true
		}
	}
	return false
}

func hasNodeSummary(nodes []*controllerv1.NodeSummary, nodeID string) bool {
	for _, node := range nodes {
		if node.GetNodeId() == nodeID {
			return true
		}
	}
	return false
}

func hasRepoEntry(entries []*controllerv1.RepoFileEntry, name string, isDir bool) bool {
	for _, entry := range entries {
		if entry.GetName() == name && entry.GetIsDir() == isDir {
			return true
		}
	}
	return false
}

func hasWorkspace(workspaces []*controllerv1.ServiceWorkspaceSummary, folder, serviceName string) bool {
	for _, workspace := range workspaces {
		if workspace.GetFolder() == folder && workspace.GetServiceName() == serviceName && workspace.GetIsDeclared() {
			return true
		}
	}
	return false
}

func hasServiceSummary(services []*controllerv1.ServiceSummary, name string) bool {
	for _, service := range services {
		if service.GetName() == name && service.GetIsDeclared() {
			return true
		}
	}
	return false
}

func hasServiceInstance(instances []*controllerv1.ServiceInstanceSummary, serviceName, nodeID string) bool {
	for _, instance := range instances {
		if instance.GetServiceName() == serviceName && instance.GetNodeId() == nodeID && instance.GetIsDeclared() {
			return true
		}
	}
	return false
}

func hasTaskSummary(tasks []*controllerv1.TaskSummary, taskID string) bool {
	for _, item := range tasks {
		if item.GetTaskId() == taskID {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
