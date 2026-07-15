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
	system          controllerv1connect.SystemServiceClient
	nodeQuery       controllerv1connect.NodeQueryServiceClient
	nodeMaintenance controllerv1connect.NodeMaintenanceServiceClient
	dockerQuery     controllerv1connect.DockerQueryServiceClient
	dockerCommand   controllerv1connect.DockerCommandServiceClient
	repoQuery       controllerv1connect.RepoQueryServiceClient
	repoCommand     controllerv1connect.RepoCommandServiceClient
	serviceQuery    controllerv1connect.ServiceQueryServiceClient
	serviceCommand  controllerv1connect.ServiceCommandServiceClient
	serviceInst     controllerv1connect.ServiceInstanceServiceClient
	task            controllerv1connect.TaskServiceClient
	backupQuery     controllerv1connect.BackupQueryServiceClient
	backupCommand   controllerv1connect.BackupCommandServiceClient
	secret          controllerv1connect.SecretServiceClient
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

	reload, err := clients.system.ReloadControllerConfig(ctx, connect.NewRequest(&controllerv1.ReloadControllerConfigRequest{}))
	if err != nil {
		t.Fatalf("reload controller config: %v", err)
	}
	if !reload.Msg.GetAccepted() {
		t.Fatalf("expected controller reload to be accepted")
	}
	waitForController(t, clients)
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

	_, err = clients.repoCommand.SyncRepo(ctx, connect.NewRequest(&controllerv1.SyncRepoRequest{}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.repoQuery.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: "missing/file.txt"}))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.repoQuery.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: "../outside"}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	workspaces, err := clients.serviceQuery.ListServiceWorkspaces(ctx, connect.NewRequest(&controllerv1.ListServiceWorkspacesRequest{}))
	if err != nil {
		t.Fatalf("list service workspaces: %v", err)
	}
	if !hasWorkspace(workspaces.Msg.GetWorkspaces(), "host-service", testServiceName) {
		t.Fatalf("expected host-service workspace")
	}

	workspace, err := clients.serviceQuery.GetServiceWorkspace(ctx, connect.NewRequest(&controllerv1.GetServiceWorkspaceRequest{Folder: "host-service"}))
	if err != nil {
		t.Fatalf("get service workspace: %v", err)
	}
	if workspace.Msg.GetWorkspace().GetServiceName() != testServiceName {
		t.Fatalf("workspace service name = %q, want %q", workspace.Msg.GetWorkspace().GetServiceName(), testServiceName)
	}

	_, err = clients.serviceQuery.GetServiceWorkspace(ctx, connect.NewRequest(&controllerv1.GetServiceWorkspaceRequest{Folder: "missing-workspace"}))
	assertConnectCode(t, err, connect.CodeNotFound)

	services, err := clients.serviceQuery.ListServices(ctx, connect.NewRequest(&controllerv1.ListServicesRequest{PageSize: 10, Page: 1}))
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if !hasServiceSummary(services.Msg.GetServices(), testServiceName) {
		t.Fatalf("expected service list to include %q", testServiceName)
	}

	servicePage, err := clients.serviceQuery.ListServices(ctx, connect.NewRequest(&controllerv1.ListServicesRequest{PageSize: 1, Page: 1}))
	if err != nil {
		t.Fatalf("list paged services: %v", err)
	}
	if servicePage.Msg.GetTotalCount() == 0 || len(servicePage.Msg.GetServices()) > 1 {
		t.Fatalf("unexpected paged service response: total=%d len=%d", servicePage.Msg.GetTotalCount(), len(servicePage.Msg.GetServices()))
	}

	missingStatusServices, err := clients.serviceQuery.ListServices(ctx, connect.NewRequest(&controllerv1.ListServicesRequest{RuntimeStatus: "controller-e2e-missing-status", PageSize: 10, Page: 1}))
	if err != nil {
		t.Fatalf("list services by missing status: %v", err)
	}
	if missingStatusServices.Msg.GetTotalCount() != 0 || len(missingStatusServices.Msg.GetServices()) != 0 {
		t.Fatalf("expected missing status service filter to return no services")
	}

	service, err := clients.serviceQuery.GetService(ctx, connect.NewRequest(&controllerv1.GetServiceRequest{ServiceName: testServiceName}))
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if service.Msg.GetName() != testServiceName || !containsString(service.Msg.GetNodes(), testNodeID) {
		t.Fatalf("unexpected service detail: %+v", service.Msg)
	}

	imageChecks, err := clients.serviceQuery.GetServiceImageUpdateChecks(ctx, connect.NewRequest(&controllerv1.GetServiceImageUpdateChecksRequest{ServiceName: testServiceName}))
	if err != nil {
		t.Fatalf("get service image update checks: %v", err)
	}
	if len(imageChecks.Msg.GetChecks()) != 0 {
		t.Fatalf("expected no image update checks in fixture, got %d", len(imageChecks.Msg.GetChecks()))
	}

	instances, err := clients.serviceInst.ListServiceInstances(ctx, connect.NewRequest(&controllerv1.ListServiceInstancesRequest{ServiceName: testServiceName}))
	if err != nil {
		t.Fatalf("list service instances: %v", err)
	}
	if !hasServiceInstance(instances.Msg.GetInstances(), testServiceName, testNodeID) {
		t.Fatalf("expected %s instance on %s", testServiceName, testNodeID)
	}

	instance, err := clients.serviceInst.GetServiceInstance(ctx, connect.NewRequest(&controllerv1.GetServiceInstanceRequest{ServiceName: testServiceName, NodeId: testNodeID}))
	if err != nil {
		t.Fatalf("get service instance: %v", err)
	}
	if instance.Msg.GetInstance().GetServiceName() != testServiceName || instance.Msg.GetInstance().GetNodeId() != testNodeID {
		t.Fatalf("unexpected service instance: %+v", instance.Msg.GetInstance())
	}

	_, err = clients.serviceInst.GetServiceInstance(ctx, connect.NewRequest(&controllerv1.GetServiceInstanceRequest{ServiceName: testServiceName, NodeId: "missing-node"}))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.serviceQuery.GetService(ctx, connect.NewRequest(&controllerv1.GetServiceRequest{ServiceName: "missing-service"}))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.serviceCommand.MigrateService(ctx, connect.NewRequest(&controllerv1.MigrateServiceRequest{ServiceName: testServiceName, SourceNodeId: testNodeID, TargetNodeId: testNodeID}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)
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

func TestControllerE2ERepoContractsAndValidation(t *testing.T) {
	ctx := testContext(t)
	clients := newControllerClients()
	waitForController(t, clients)

	recursive, err := clients.repoQuery.ListRepoFiles(ctx, connect.NewRequest(&controllerv1.ListRepoFilesRequest{Recursive: true}))
	if err != nil {
		t.Fatalf("list recursive repo files: %v", err)
	}
	if !hasRepoEntryPath(recursive.Msg.GetEntries(), "host-service/composia-meta.yaml", false) {
		t.Fatalf("expected recursive repo listing to include host-service/composia-meta.yaml")
	}

	_, err = clients.repoQuery.ListRepoFiles(ctx, connect.NewRequest(&controllerv1.ListRepoFilesRequest{Path: "host-service/composia-meta.yaml"}))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.repoQuery.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	baseRevision := repoHead(t, ctx, clients).GetHeadRevision()
	_, err = clients.repoCommand.CreateRepoDirectory(ctx, connect.NewRequest(&controllerv1.CreateRepoDirectoryRequest{Path: "host-service", BaseRevision: baseRevision, CommitMessage: "Create existing controller e2e directory"}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.repoCommand.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{Path: "host-service", Content: "not a file", BaseRevision: baseRevision, CommitMessage: "Write directory as file"}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.repoCommand.MoveRepoPath(ctx, connect.NewRequest(&controllerv1.MoveRepoPathRequest{SourcePath: "missing-source", DestinationPath: "missing-destination", BaseRevision: baseRevision, CommitMessage: "Move missing controller e2e path"}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.repoCommand.DeleteRepoPath(ctx, connect.NewRequest(&controllerv1.DeleteRepoPathRequest{Path: "missing-delete", BaseRevision: baseRevision, CommitMessage: "Delete missing controller e2e path"}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	workspace := fmt.Sprintf("controller-e2e-invalid-%d", time.Now().UnixNano())
	cleanupWorkspace := true
	defer func() {
		if cleanupWorkspace {
			cleanupRepoPath(t, ctx, clients, workspace)
		}
	}()

	created, err := clients.repoCommand.CreateRepoDirectory(ctx, connect.NewRequest(&controllerv1.CreateRepoDirectoryRequest{
		Path:          workspace,
		BaseRevision:  baseRevision,
		CommitMessage: "Create invalid controller e2e workspace",
	}))
	if err != nil {
		t.Fatalf("create invalid repo workspace: %v", err)
	}

	invalidMetaPath := workspace + "/composia-meta.yaml"
	updated, err := clients.repoCommand.UpdateRepoFile(ctx, connect.NewRequest(&controllerv1.UpdateRepoFileRequest{
		Path:          invalidMetaPath,
		Content:       "name: controller-e2e-invalid\nnodes:\n  - missing-node\n",
		BaseRevision:  created.Msg.GetCommitId(),
		CommitMessage: "Write invalid controller e2e metadata",
	}))
	if err != nil {
		t.Fatalf("write invalid repo metadata: %v", err)
	}

	validation, err := clients.repoQuery.ValidateRepo(ctx, connect.NewRequest(&controllerv1.ValidateRepoRequest{}))
	if err != nil {
		t.Fatalf("validate invalid repo: %v", err)
	}
	if !hasValidationErrorPath(validation.Msg.GetErrors(), invalidMetaPath) {
		t.Fatalf("expected validation error for %s, got %+v", invalidMetaPath, validation.Msg.GetErrors())
	}

	deleted, err := clients.repoCommand.DeleteRepoPath(ctx, connect.NewRequest(&controllerv1.DeleteRepoPathRequest{
		Path:          workspace,
		BaseRevision:  updated.Msg.GetCommitId(),
		CommitMessage: "Delete invalid controller e2e workspace",
	}))
	if err != nil {
		t.Fatalf("delete invalid repo workspace: %v", err)
	}
	if deleted.Msg.GetCommitId() == "" {
		t.Fatalf("expected invalid workspace delete commit id")
	}
	cleanupWorkspace = false
}

func TestControllerE2ESecrets(t *testing.T) {
	ctx := testContext(t)
	clients := newControllerClients()
	waitForController(t, clients)

	const secretFilePath = ".secret.env.enc"
	const initialContent = "TOKEN=controller-e2e-before\n"
	const updatedContent = "TOKEN=controller-e2e-after\n"

	secret, err := clients.secret.GetSecret(ctx, connect.NewRequest(&controllerv1.GetSecretRequest{ServiceName: testServiceName, FilePath: secretFilePath}))
	if err != nil {
		t.Fatalf("get configured secret: %v", err)
	}
	if secret.Msg.GetServiceName() != testServiceName || secret.Msg.GetFilePath() != secretFilePath || secret.Msg.GetContent() != initialContent {
		t.Fatalf("unexpected secret response: %+v", secret.Msg)
	}

	_, err = clients.secret.GetSecret(ctx, connect.NewRequest(&controllerv1.GetSecretRequest{ServiceName: testServiceName, FilePath: "../outside.env.enc"}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.secret.UpdateSecret(ctx, connect.NewRequest(&controllerv1.UpdateSecretRequest{ServiceName: testServiceName, FilePath: secretFilePath, Content: updatedContent}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	head := repoHead(t, ctx, clients)
	_, err = clients.secret.UpdateSecret(ctx, connect.NewRequest(&controllerv1.UpdateSecretRequest{ServiceName: testServiceName, FilePath: "/outside.env.enc", Content: updatedContent, BaseRevision: head.GetHeadRevision()}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	workspace := fmt.Sprintf("controller-e2e-secret-base-%d", time.Now().UnixNano())
	created, err := clients.repoCommand.CreateRepoDirectory(ctx, connect.NewRequest(&controllerv1.CreateRepoDirectoryRequest{
		Path:          workspace,
		BaseRevision:  head.GetHeadRevision(),
		CommitMessage: "Create secret base revision marker",
	}))
	if err != nil {
		t.Fatalf("create secret base revision marker: %v", err)
	}
	defer cleanupRepoPath(t, ctx, clients, workspace)

	_, err = clients.secret.UpdateSecret(ctx, connect.NewRequest(&controllerv1.UpdateSecretRequest{
		ServiceName:   testServiceName,
		FilePath:      secretFilePath,
		Content:       updatedContent,
		BaseRevision:  head.GetHeadRevision(),
		CommitMessage: "Stale controller e2e secret update",
	}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	updated, err := clients.secret.UpdateSecret(ctx, connect.NewRequest(&controllerv1.UpdateSecretRequest{
		ServiceName:   testServiceName,
		FilePath:      secretFilePath,
		Content:       updatedContent,
		BaseRevision:  created.Msg.GetCommitId(),
		CommitMessage: "Update controller e2e secret",
	}))
	if err != nil {
		t.Fatalf("update configured secret: %v", err)
	}
	if updated.Msg.GetCommitId() == "" {
		t.Fatalf("expected secret update commit id")
	}
	defer func() {
		head := repoHead(t, ctx, clients)
		_, err := clients.secret.UpdateSecret(ctx, connect.NewRequest(&controllerv1.UpdateSecretRequest{
			ServiceName:   testServiceName,
			FilePath:      secretFilePath,
			Content:       initialContent,
			BaseRevision:  head.GetHeadRevision(),
			CommitMessage: "Restore controller e2e secret",
		}))
		if err != nil {
			t.Logf("restore controller e2e secret: %v", err)
		}
	}()

	secret, err = clients.secret.GetSecret(ctx, connect.NewRequest(&controllerv1.GetSecretRequest{ServiceName: testServiceName, FilePath: secretFilePath}))
	if err != nil {
		t.Fatalf("get updated secret: %v", err)
	}
	if secret.Msg.GetContent() != updatedContent {
		t.Fatalf("updated secret content = %q, want %q", secret.Msg.GetContent(), updatedContent)
	}

	file, err := clients.repoQuery.GetRepoFile(ctx, connect.NewRequest(&controllerv1.GetRepoFileRequest{Path: "host-service/" + secretFilePath}))
	if err != nil {
		t.Fatalf("get encrypted secret file: %v", err)
	}
	if strings.Contains(file.Msg.GetContent(), updatedContent) {
		t.Fatalf("expected encrypted secret file not to contain plaintext")
	}
}

func TestControllerE2EFeatureContracts(t *testing.T) {
	ctx := testContext(t)
	clients := newControllerClients()
	waitForController(t, clients)

	backups, err := clients.backupQuery.ListBackups(ctx, connect.NewRequest(&controllerv1.ListBackupsRequest{PageSize: 10, Page: 1}))
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if backups.Msg.GetTotalCount() < uint32(len(backups.Msg.GetBackups())) {
		t.Fatalf("backup total_count = %d, returned %d", backups.Msg.GetTotalCount(), len(backups.Msg.GetBackups()))
	}

	_, err = clients.backupQuery.GetBackup(ctx, connect.NewRequest(&controllerv1.GetBackupRequest{BackupId: "missing-backup"}))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.backupCommand.RestoreBackup(ctx, connect.NewRequest(&controllerv1.RestoreBackupRequest{BackupId: "missing-backup", NodeId: testNodeID}))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.task.GetTask(ctx, connect.NewRequest(&controllerv1.GetTaskRequest{}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.task.GetTask(ctx, connect.NewRequest(&controllerv1.GetTaskRequest{TaskId: "missing-task"}))
	assertConnectCode(t, err, connect.CodeNotFound)

	taskLogStream, err := clients.task.TailTaskLogs(ctx, connect.NewRequest(&controllerv1.TailTaskLogsRequest{TaskId: "missing-task"}))
	assertStreamConnectCode(t, taskLogStream, err, connect.CodeNotFound)

	_, err = clients.task.RunTaskAgain(ctx, sourceRequest(&controllerv1.RunTaskAgainRequest{}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.task.RunTaskAgain(ctx, sourceRequest(&controllerv1.RunTaskAgainRequest{TaskId: "missing-task"}, "web"))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.task.ResolveTaskConfirmation(ctx, sourceRequest(&controllerv1.ResolveTaskConfirmationRequest{TaskId: "missing-task"}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.task.ResolveTaskConfirmation(ctx, sourceRequest(&controllerv1.ResolveTaskConfirmationRequest{TaskId: "missing-task", Decision: controllerv1.TaskConfirmationDecision_TASK_CONFIRMATION_DECISION_APPROVE}, "web"))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.serviceCommand.RunServiceAction(ctx, sourceRequest(&controllerv1.RunServiceActionRequest{}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.serviceCommand.RunServiceAction(ctx, sourceRequest(&controllerv1.RunServiceActionRequest{ServiceName: "missing-service", Action: controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY}, "web"))
	assertConnectCode(t, err, connect.CodeNotFound)

	_, err = clients.serviceCommand.RunServiceAction(ctx, sourceRequest(&controllerv1.RunServiceActionRequest{ServiceName: testServiceName}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.serviceCommand.RunServiceAction(ctx, sourceRequest(&controllerv1.RunServiceActionRequest{ServiceName: testServiceName, Action: controllerv1.ServiceAction_SERVICE_ACTION_DNS_UPDATE}, "web"))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.serviceCommand.MigrateService(ctx, sourceRequest(&controllerv1.MigrateServiceRequest{ServiceName: "missing-service", SourceNodeId: testNodeID, TargetNodeId: "other-node"}, "web"))
	assertConnectCode(t, err, connect.CodeNotFound)

	stats, err := clients.nodeQuery.GetNodeDockerStats(ctx, connect.NewRequest(&controllerv1.GetNodeDockerStatsRequest{NodeId: testNodeID}))
	if err != nil {
		t.Fatalf("get node docker stats: %v", err)
	}
	if stats.Msg.GetStats() == nil {
		t.Fatalf("expected docker stats message")
	}

	_, err = clients.nodeMaintenance.ReloadNodeCaddy(ctx, sourceRequest(&controllerv1.ReloadNodeCaddyRequest{NodeId: testNodeID}, "web"))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.nodeMaintenance.SyncNodeCaddyFiles(ctx, sourceRequest(&controllerv1.SyncNodeCaddyFilesRequest{NodeId: testNodeID, ServiceName: testServiceName}, "web"))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.nodeMaintenance.InitNodeRustic(ctx, sourceRequest(&controllerv1.InitNodeRusticRequest{NodeId: testNodeID}, "web"))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.nodeMaintenance.ForgetNodeRustic(ctx, sourceRequest(&controllerv1.ForgetNodeRusticRequest{NodeId: testNodeID, ServiceName: testServiceName, DataName: "data"}, "web"))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.nodeMaintenance.PruneNodeRustic(ctx, sourceRequest(&controllerv1.PruneNodeRusticRequest{NodeId: testNodeID, ServiceName: testServiceName, DataName: "data"}, "web"))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.nodeMaintenance.PruneNodeDocker(ctx, sourceRequest(&controllerv1.PruneNodeDockerRequest{}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.nodeQuery.GetNodeDockerStats(ctx, connect.NewRequest(&controllerv1.GetNodeDockerStatsRequest{}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.dockerQuery.ListNodeContainers(ctx, connect.NewRequest(&controllerv1.ListNodeContainersRequest{}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.dockerQuery.ListNodeContainers(ctx, connect.NewRequest(&controllerv1.ListNodeContainersRequest{NodeId: "missing-node"}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.dockerQuery.InspectNodeContainer(ctx, connect.NewRequest(&controllerv1.InspectNodeContainerRequest{NodeId: testNodeID}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.dockerCommand.RunContainerAction(ctx, sourceRequest(&controllerv1.RunContainerActionRequest{NodeId: testNodeID, ContainerId: "container"}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.dockerCommand.RemoveContainer(ctx, sourceRequest(&controllerv1.RemoveContainerRequest{NodeId: testNodeID}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	logStream, err := clients.dockerCommand.GetContainerLogs(ctx, connect.NewRequest(&controllerv1.GetContainerLogsRequest{NodeId: testNodeID}))
	assertStreamConnectCode(t, logStream, err, connect.CodeInvalidArgument)

	_, err = clients.dockerCommand.OpenContainerExec(ctx, connect.NewRequest(&controllerv1.OpenContainerExecRequest{NodeId: testNodeID, ContainerId: "container"}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.dockerCommand.RunContainerExec(ctx, connect.NewRequest(&controllerv1.RunContainerExecRequest{NodeId: testNodeID, ContainerId: "container"}))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.dockerCommand.RemoveNetwork(ctx, sourceRequest(&controllerv1.RemoveNetworkRequest{NodeId: testNodeID}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.dockerCommand.RemoveVolume(ctx, sourceRequest(&controllerv1.RemoveVolumeRequest{NodeId: testNodeID}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)

	_, err = clients.dockerCommand.RemoveImage(ctx, sourceRequest(&controllerv1.RemoveImageRequest{NodeId: testNodeID}, "web"))
	assertConnectCode(t, err, connect.CodeInvalidArgument)
}

func TestControllerE2EDockerQueriesAndExec(t *testing.T) {
	ctx := testContext(t)
	clients := newControllerClients()
	waitForController(t, clients)
	waitForNodeOnline(t, ctx, clients, testNodeID)

	containers := listContainers(t, ctx, clients)
	container := chooseExecContainer(t, containers)

	pagedContainers, err := clients.dockerQuery.ListNodeContainers(ctx, connect.NewRequest(&controllerv1.ListNodeContainersRequest{NodeId: testNodeID, PageSize: 1, Page: 1}))
	if err != nil {
		t.Fatalf("list paged node containers: %v", err)
	}
	if pagedContainers.Msg.GetTotalCount() == 0 || len(pagedContainers.Msg.GetContainers()) != 1 {
		t.Fatalf("expected one paged container with non-zero total, got total=%d len=%d", pagedContainers.Msg.GetTotalCount(), len(pagedContainers.Msg.GetContainers()))
	}

	searchedContainers, err := clients.dockerQuery.ListNodeContainers(ctx, connect.NewRequest(&controllerv1.ListNodeContainersRequest{NodeId: testNodeID, PageSize: 10, Page: 1, Search: container.GetName()}))
	if err != nil {
		t.Fatalf("search node containers: %v", err)
	}
	if !hasContainerInfo(searchedContainers.Msg.GetContainers(), container.GetId()) {
		t.Fatalf("expected container search for %q to include %q", container.GetName(), container.GetId())
	}

	containerInspect, err := clients.dockerQuery.InspectNodeContainer(ctx, connect.NewRequest(&controllerv1.InspectNodeContainerRequest{NodeId: testNodeID, ContainerId: container.GetId()}))
	if err != nil {
		t.Fatalf("inspect node container: %v", err)
	}
	if !strings.Contains(containerInspect.Msg.GetRawJson(), shortID(container.GetId())) && !strings.Contains(containerInspect.Msg.GetRawJson(), container.GetName()) {
		t.Fatalf("unexpected container inspect payload")
	}

	networks, err := clients.dockerQuery.ListNodeNetworks(ctx, connect.NewRequest(&controllerv1.ListNodeNetworksRequest{NodeId: testNodeID, PageSize: 10, Page: 1}))
	if err != nil {
		t.Fatalf("list node networks: %v", err)
	}
	if networks.Msg.GetTotalCount() == 0 || len(networks.Msg.GetNetworks()) == 0 {
		t.Fatalf("expected at least one docker network")
	}
	networkInspect, err := clients.dockerQuery.InspectNodeNetwork(ctx, connect.NewRequest(&controllerv1.InspectNodeNetworkRequest{NodeId: testNodeID, NetworkId: networks.Msg.GetNetworks()[0].GetId()}))
	if err != nil {
		t.Fatalf("inspect node network: %v", err)
	}
	if strings.TrimSpace(networkInspect.Msg.GetRawJson()) == "" {
		t.Fatalf("expected network inspect JSON")
	}

	volumes, err := clients.dockerQuery.ListNodeVolumes(ctx, connect.NewRequest(&controllerv1.ListNodeVolumesRequest{NodeId: testNodeID, PageSize: 10, Page: 1}))
	if err != nil {
		t.Fatalf("list node volumes: %v", err)
	}
	if len(volumes.Msg.GetVolumes()) > 0 {
		volumeInspect, err := clients.dockerQuery.InspectNodeVolume(ctx, connect.NewRequest(&controllerv1.InspectNodeVolumeRequest{NodeId: testNodeID, VolumeName: volumes.Msg.GetVolumes()[0].GetName()}))
		if err != nil {
			t.Fatalf("inspect node volume: %v", err)
		}
		if strings.TrimSpace(volumeInspect.Msg.GetRawJson()) == "" {
			t.Fatalf("expected volume inspect JSON")
		}
	}

	images, err := clients.dockerQuery.ListNodeImages(ctx, connect.NewRequest(&controllerv1.ListNodeImagesRequest{NodeId: testNodeID, PageSize: 10, Page: 1}))
	if err != nil {
		t.Fatalf("list node images: %v", err)
	}
	if images.Msg.GetTotalCount() == 0 || len(images.Msg.GetImages()) == 0 {
		t.Fatalf("expected at least one docker image")
	}
	imageInspect, err := clients.dockerQuery.InspectNodeImage(ctx, connect.NewRequest(&controllerv1.InspectNodeImageRequest{NodeId: testNodeID, ImageId: images.Msg.GetImages()[0].GetId()}))
	if err != nil {
		t.Fatalf("inspect node image: %v", err)
	}
	if strings.TrimSpace(imageInspect.Msg.GetRawJson()) == "" {
		t.Fatalf("expected image inspect JSON")
	}

	execResult, err := clients.dockerCommand.RunContainerExec(ctx, connect.NewRequest(&controllerv1.RunContainerExecRequest{
		NodeId:         testNodeID,
		ContainerId:    container.GetId(),
		Command:        []string{"/bin/sh", "-c", "printf composia-controller-e2e"},
		TimeoutSeconds: 10,
		MaxOutputBytes: 1024,
	}))
	if err != nil {
		t.Fatalf("run container exec: %v", err)
	}
	if execResult.Msg.GetExitCode() != 0 || execResult.Msg.GetStdout() != "composia-controller-e2e" {
		t.Fatalf("unexpected exec result: exit=%d stdout=%q stderr=%q", execResult.Msg.GetExitCode(), execResult.Msg.GetStdout(), execResult.Msg.GetStderr())
	}

	stdinResult, err := clients.dockerCommand.RunContainerExec(ctx, connect.NewRequest(&controllerv1.RunContainerExecRequest{
		NodeId:         testNodeID,
		ContainerId:    container.GetId(),
		Command:        []string{"/bin/sh", "-c", "cat"},
		Stdin:          []byte("stdin-through-exec"),
		TimeoutSeconds: 10,
		MaxOutputBytes: 1024,
	}))
	if err != nil {
		t.Fatalf("run container exec with stdin: %v", err)
	}
	if stdinResult.Msg.GetExitCode() != 0 || stdinResult.Msg.GetStdout() != "stdin-through-exec" {
		t.Fatalf("unexpected stdin exec result: exit=%d stdout=%q stderr=%q", stdinResult.Msg.GetExitCode(), stdinResult.Msg.GetStdout(), stdinResult.Msg.GetStderr())
	}

	truncatedResult, err := clients.dockerCommand.RunContainerExec(ctx, connect.NewRequest(&controllerv1.RunContainerExecRequest{
		NodeId:         testNodeID,
		ContainerId:    container.GetId(),
		Command:        []string{"/bin/sh", "-c", "printf 1234567890"},
		TimeoutSeconds: 10,
		MaxOutputBytes: 4,
	}))
	if err != nil {
		t.Fatalf("run truncated container exec: %v", err)
	}
	if truncatedResult.Msg.GetExitCode() != 0 || truncatedResult.Msg.GetStdout() != "1234" || !truncatedResult.Msg.GetStdoutTruncated() {
		t.Fatalf("unexpected truncated exec result: exit=%d stdout=%q truncated=%t", truncatedResult.Msg.GetExitCode(), truncatedResult.Msg.GetStdout(), truncatedResult.Msg.GetStdoutTruncated())
	}

	containerLogChunk := firstContainerLogChunk(t, ctx, clients, container.GetId())
	if strings.TrimSpace(containerLogChunk) == "" {
		t.Fatalf("expected non-empty container log chunk")
	}
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

	logChunk := firstTaskLogChunk(t, ctx, clients, taskID)
	if strings.TrimSpace(logChunk) == "" {
		t.Fatalf("expected non-empty task log chunk")
	}

	_, err = clients.task.ResolveTaskConfirmation(ctx, sourceRequest(&controllerv1.ResolveTaskConfirmationRequest{TaskId: taskID, Decision: controllerv1.TaskConfirmationDecision_TASK_CONFIRMATION_DECISION_APPROVE}, "web"))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

	_, err = clients.task.CreateMigrationRollback(ctx, sourceRequest(&controllerv1.CreateMigrationRollbackRequest{TaskId: taskID, DeploySource: true}, "web"))
	assertConnectCode(t, err, connect.CodeFailedPrecondition)

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

	excluded, err := clients.task.ListTasks(ctx, connect.NewRequest(&controllerv1.ListTasksRequest{
		ExcludeStatus:      []controllerv1.TaskStatus{controllerv1.TaskStatus_TASK_STATUS_SUCCEEDED},
		ExcludeServiceName: []string{"not-" + testServiceName},
		ExcludeType:        []controllerv1.TaskType{controllerv1.TaskType_TASK_TYPE_RESTART},
		PageSize:           20,
		Page:               1,
	}))
	if err != nil {
		t.Fatalf("list excluded tasks: %v", err)
	}
	if hasTaskSummary(excluded.Msg.GetTasks(), taskID) {
		t.Fatalf("expected exclude_status to remove succeeded task %q", taskID)
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
		system:          controllerv1connect.NewSystemServiceClient(http.DefaultClient, baseURL, opts...),
		nodeQuery:       controllerv1connect.NewNodeQueryServiceClient(http.DefaultClient, baseURL, opts...),
		nodeMaintenance: controllerv1connect.NewNodeMaintenanceServiceClient(http.DefaultClient, baseURL, opts...),
		dockerQuery:     controllerv1connect.NewDockerQueryServiceClient(http.DefaultClient, baseURL, opts...),
		dockerCommand:   controllerv1connect.NewDockerCommandServiceClient(http.DefaultClient, baseURL, opts...),
		repoQuery:       controllerv1connect.NewRepoQueryServiceClient(http.DefaultClient, baseURL, opts...),
		repoCommand:     controllerv1connect.NewRepoCommandServiceClient(http.DefaultClient, baseURL, opts...),
		serviceQuery:    controllerv1connect.NewServiceQueryServiceClient(http.DefaultClient, baseURL, opts...),
		serviceCommand:  controllerv1connect.NewServiceCommandServiceClient(http.DefaultClient, baseURL, opts...),
		serviceInst:     controllerv1connect.NewServiceInstanceServiceClient(http.DefaultClient, baseURL, opts...),
		task:            controllerv1connect.NewTaskServiceClient(http.DefaultClient, baseURL, opts...),
		backupQuery:     controllerv1connect.NewBackupQueryServiceClient(http.DefaultClient, baseURL, opts...),
		backupCommand:   controllerv1connect.NewBackupCommandServiceClient(http.DefaultClient, baseURL, opts...),
		secret:          controllerv1connect.NewSecretServiceClient(http.DefaultClient, baseURL, opts...),
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

func cleanupRepoPath(t *testing.T, ctx context.Context, clients controllerClients, path string) {
	t.Helper()
	head, err := clients.repoQuery.GetRepoHead(ctx, connect.NewRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		t.Logf("cleanup get repo head for %q: %v", path, err)
		return
	}
	_, err = clients.repoCommand.DeleteRepoPath(ctx, connect.NewRequest(&controllerv1.DeleteRepoPathRequest{
		Path:          path,
		BaseRevision:  head.Msg.GetHeadRevision(),
		CommitMessage: "Cleanup controller e2e workspace",
	}))
	if err == nil {
		return
	}
	if code := connect.CodeOf(err); code == connect.CodeFailedPrecondition || code == connect.CodeNotFound {
		return
	}
	t.Logf("cleanup delete repo path %q: %v", path, err)
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

func firstTaskLogChunk(t *testing.T, ctx context.Context, clients controllerClients, taskID string) string {
	t.Helper()
	streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	stream, err := clients.task.TailTaskLogs(streamCtx, connect.NewRequest(&controllerv1.TailTaskLogsRequest{TaskId: taskID}))
	if err != nil {
		t.Fatalf("tail task logs: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if !stream.Receive() {
		t.Fatalf("expected task log chunk, got err=%v", stream.Err())
	}
	return stream.Msg().GetContent()
}

func firstContainerLogChunk(t *testing.T, ctx context.Context, clients controllerClients, containerID string) string {
	t.Helper()
	streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	stream, err := clients.dockerCommand.GetContainerLogs(streamCtx, connect.NewRequest(&controllerv1.GetContainerLogsRequest{NodeId: testNodeID, ContainerId: containerID, Tail: "20"}))
	if err != nil {
		t.Fatalf("get container logs: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if !stream.Receive() {
		t.Fatalf("expected container log chunk, got err=%v", stream.Err())
	}
	return stream.Msg().GetContent()
}

func waitForNodeOnline(t *testing.T, ctx context.Context, clients controllerClients, nodeID string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := clients.nodeQuery.GetNode(ctx, connect.NewRequest(&controllerv1.GetNodeRequest{NodeId: nodeID}))
		if err != nil {
			t.Fatalf("get node while waiting online: %v", err)
		}
		if resp.Msg.GetNode().GetIsOnline() {
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("node %q did not become online", nodeID)
}

func listContainers(t *testing.T, ctx context.Context, clients controllerClients) []*controllerv1.ContainerInfo {
	t.Helper()
	resp, err := clients.dockerQuery.ListNodeContainers(ctx, connect.NewRequest(&controllerv1.ListNodeContainersRequest{NodeId: testNodeID, PageSize: 50, Page: 1}))
	if err != nil {
		t.Fatalf("list node containers: %v", err)
	}
	if resp.Msg.GetTotalCount() == 0 || len(resp.Msg.GetContainers()) == 0 {
		t.Fatalf("expected at least one docker container")
	}
	return resp.Msg.GetContainers()
}

func chooseExecContainer(t *testing.T, containers []*controllerv1.ContainerInfo) *controllerv1.ContainerInfo {
	t.Helper()
	for _, container := range containers {
		name := container.GetName()
		if container.GetState() == "running" && strings.Contains(name, "composia-e2e-exec") {
			return container
		}
	}
	t.Fatalf("expected running composia-e2e-exec fixture container")
	return nil
}

func shortID(id string) string {
	if len(id) < 12 {
		return id
	}
	return id[:12]
}

func sourceRequest[T any](msg *T, source string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Composia-Source", source)
	return req
}

func assertStreamConnectCode[T any](t *testing.T, stream *connect.ServerStreamForClient[T], err error, code connect.Code) {
	t.Helper()
	if err != nil {
		assertConnectCode(t, err, code)
		return
	}
	defer func() { _ = stream.Close() }()
	if stream.Receive() {
		t.Fatalf("expected stream connect code %s, got message", code)
	}
	assertConnectCode(t, stream.Err(), code)
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

func hasRepoEntryPath(entries []*controllerv1.RepoFileEntry, path string, isDir bool) bool {
	for _, entry := range entries {
		if entry.GetPath() == path && entry.GetIsDir() == isDir {
			return true
		}
	}
	return false
}

func hasValidationErrorPath(errors []*controllerv1.RepoValidationError, path string) bool {
	for _, validationError := range errors {
		if validationError.GetPath() == path {
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

func hasContainerInfo(containers []*controllerv1.ContainerInfo, containerID string) bool {
	for _, container := range containers {
		if container.GetId() == containerID {
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
