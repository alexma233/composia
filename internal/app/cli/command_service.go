package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runService(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: composia service <list|create|<service> [action]>")
	}
	switch args[0] {
	case "list": //nolint:goconst
		return application.runServiceList(args[1:])
	case "create":
		return application.runServiceCreate(args[1:])
	}

	serviceName := args[0]
	rest := args[1:]
	if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
		return application.runServiceGet(serviceName, rest)
	}
	switch rest[0] {
	case "edit":
		return application.runServiceEdit(serviceName, rest[1:])
	case "updates":
		return application.runServiceUpdateCandidates(serviceName, rest[1:])
	case "up", "down", actionUpdate, actionRestart, "backup", "dns-update", "caddy-sync", "cloudflare-tunnel-sync", "tunnel-sync":
		return application.runServiceAction(rest[0], serviceName, rest[1:])
	case "migrate":
		return application.runServiceMigrate(serviceName, rest[1:])
	case "logs":
		return application.runServiceLogs(serviceName, rest[1:])
	case "ps":
		return application.runServicePS(serviceName, rest[1:])
	case "exec":
		return application.runServiceExec(serviceName, rest[1:])
	default:
		return fmt.Errorf("unknown action %q for service %q", rest[0], serviceName)
	}
}

func (application *app) runServiceCreate(args []string) error {
	fs := newCommandFlagSet("service create")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia service create [--message text] <name>"); err != nil {
		return err
	}
	folder, err := normalizeServiceRelativePath(fs.Arg(0))
	if err != nil {
		return err
	}
	if folder == "" || strings.Contains(folder, "/") {
		return errors.New("service name must be a single top-level folder")
	}
	commitMessage := *message
	if commitMessage == "" {
		commitMessage = "create service " + folder
	}
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	composePath := serviceRepoPath(folder, "docker-compose.yaml")
	metaPath := serviceRepoPath(folder, "composia-meta.yaml")
	composeWrite, err := application.client.repoCommands.UpdateRepoFile(application.ctx, newRequest(&controllerv1.UpdateRepoFileRequest{Path: composePath, Content: "", BaseRevision: baseRevision, CommitMessage: commitMessage}))
	if err != nil {
		return repoWriteError(err)
	}
	metaWrite, err := application.client.repoCommands.UpdateRepoFile(application.ctx, newRequest(&controllerv1.UpdateRepoFileRequest{Path: metaPath, Content: "", BaseRevision: composeWrite.Msg.GetCommitId(), CommitMessage: commitMessage}))
	if err != nil {
		return repoWriteError(err)
	}
	if application.isJSONOutput() {
		return application.printMessage(metaWrite.Msg)
	}
	if err := application.writeKV([][2]string{
		{"folder", folder},
		{"compose", composePath},
		{"meta", metaPath},
	}); err != nil {
		return err
	}
	return application.printRepoWriteMessage(metaWrite.Msg)
}

func (application *app) runServiceEdit(serviceName string, args []string) error {
	fs := newCommandFlagSet("service edit")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia service <service> edit [--message text] <path>"); err != nil {
		return err
	}
	folder, err := application.resolveServiceFolder(serviceName)
	if err != nil {
		return err
	}
	filePath, err := normalizeServiceRelativePath(fs.Arg(0))
	if err != nil {
		return err
	}
	return application.editServiceRepoFile(folder, filePath, *message)
}

func (application *app) runServiceUpdateCandidates(serviceName string, args []string) error {
	fs := newCommandFlagSet("service updates")
	nodeID := fs.String("node", "", "node ID filter")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia service <service> updates [--node node]"); err != nil {
		return err
	}
	response, err := application.client.services.GetServiceImageUpdateChecks(application.ctx, newRequest(&controllerv1.GetServiceImageUpdateChecksRequest{ServiceName: serviceName, NodeId: *nodeID}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetChecks()))
	for _, check := range response.Msg.GetChecks() {
		rows = append(rows, []string{
			check.GetNodeId(),
			check.GetImageName(),
			check.GetPolicyType(),
			check.GetCurrentTag(),
			check.GetCandidateTag(),
			boolText(check.GetUpdateAvailable()),
			check.GetCheckStatus(),
			check.GetCheckedAt(),
		})
	}
	return application.writeTable([]string{"NODE", "IMAGE", "POLICY", "CURRENT", "CANDIDATE", "AVAILABLE", "STATUS", "CHECKED"}, rows)
}

func (application *app) runServiceList(args []string) error {
	fs := newCommandFlagSet("service list")
	status := fs.String("status", "", "runtime status filter")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia service list [--status status] [--page-size n] [--page n]"); err != nil {
		return err
	}
	pageSize, page, err := pageValues()
	if err != nil {
		return err
	}
	response, err := application.client.services.ListServices(application.ctx, newRequest(&controllerv1.ListServicesRequest{RuntimeStatus: *status, PageSize: pageSize, Page: page}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetServices()))
	for _, service := range response.Msg.GetServices() {
		rows = append(rows, []string{
			service.GetName(),
			boolText(service.GetIsDeclared()),
			service.GetRuntimeStatus(),
			uintText(service.GetInstanceCount()),
			uintText(service.GetRunningCount()),
			uintText(service.GetTargetNodeCount()),
			service.GetUpdatedAt(),
		})
	}
	if err := application.writeTable([]string{"NAME", "DECLARED", "STATUS", "INSTANCES", "RUNNING", "TARGETS", "UPDATED"}, rows); err != nil {
		return err
	}
	return application.writeCount("total_count", response.Msg.GetTotalCount())
}

func (application *app) runServiceGet(serviceName string, args []string) error {
	fs := newCommandFlagSet("service")
	includeContainers := fs.Bool("containers", false, "include per-instance containers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia service <service> [--containers]"); err != nil {
		return err
	}
	response, err := application.client.services.GetService(application.ctx, newRequest(&controllerv1.GetServiceRequest{ServiceName: serviceName, IncludeContainers: *includeContainers}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	return application.printServiceDetail(response.Msg)
}

func (application *app) editServiceRepoFile(folder string, filePath string, message string) error {
	repoPath := serviceRepoPath(folder, filePath)
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	content := ""
	fileResponse, err := application.client.repos.GetRepoFile(application.ctx, newRequest(&controllerv1.GetRepoFileRequest{Path: repoPath}))
	if err == nil {
		content = fileResponse.Msg.GetContent()
	} else if connect.CodeOf(err) != connect.CodeNotFound {
		return err
	}
	updatedContent, changed, err := editText(application.ctx, content, "composia-service-*", 0o600)
	if err != nil {
		return err
	}
	if !changed {
		_, err := fmt.Fprintln(application.out, "unchanged")
		return err
	}
	if message == "" {
		message = "update " + repoPath
	}
	response, err := application.client.repoCommands.UpdateRepoFile(application.ctx, newRequest(&controllerv1.UpdateRepoFileRequest{Path: repoPath, Content: updatedContent, BaseRevision: baseRevision, CommitMessage: message}))
	if err != nil {
		return repoWriteError(err)
	}
	return application.printRepoWriteMessage(response.Msg)
}

func (application *app) runServiceAction(actionName string, serviceName string, args []string) error {
	action, err := serviceActionFromName(actionName)
	if err != nil {
		return err
	}
	fs := newCommandFlagSet("service " + actionName)
	var nodes stringListFlag
	var dataNames stringListFlag
	var imageNames stringListFlag
	var setImages stringListFlag
	recreateMode := "auto"
	useDetectedImages := false
	useAllDetectedImages := false
	forceBackupBeforeUpdate := false
	skipBackupBeforeUpdate := false
	fs.Var(&nodes, "node", "target node ID; repeat or comma-separate")
	fs.Var(&dataNames, "data", "data entry name for backup-like actions; repeat or comma-separate")
	if actionName == "up" || actionName == actionUpdate {
		fs.StringVar(&recreateMode, "recreate", "auto", "compose recreate mode: auto, never, always")
	}
	if actionName == actionUpdate {
		fs.Var(&imageNames, "image", "configured image update name; repeat or comma-separate")
		fs.Var(&setImages, "set-image", "configured image update assignment name=tag; repeat or comma-separate")
		fs.BoolVar(&useDetectedImages, "use-detected", false, "apply detected candidate for --image entries")
		fs.BoolVar(&useAllDetectedImages, "all-detected", false, "apply all detected image updates")
		fs.BoolVar(&forceBackupBeforeUpdate, "backup", false, "force backup before update")
		fs.BoolVar(&skipBackupBeforeUpdate, "no-backup", false, "skip backup before update")
	}
	waitOptions := addWaitFlags(fs)
	detach := fs.Bool("detach", false, "return immediately without waiting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	usageActionName := actionName
	if usageActionName == "cloudflare-tunnel-sync" {
		usageActionName = "tunnel-sync"
	}
	usage := commandUsageText("service " + usageActionName)
	if err := requireArgs(fs.Args(), 0, usage); err != nil {
		return err
	}
	if forceBackupBeforeUpdate && skipBackupBeforeUpdate {
		return errorsWithUsage("--backup and --no-backup cannot be used together", usage)
	}
	if !*detach {
		*waitOptions.wait = true
		if actionName == "up" {
			*waitOptions.follow = true
		}
	}
	composeRecreateMode, err := composeRecreateModeFromName(recreateMode)
	if err != nil {
		return err
	}
	imageUpdates, err := imageUpdateSelections([]string(imageNames), []string(setImages), useDetectedImages)
	if err != nil {
		return err
	}
	var backupBeforeUpdate *bool
	if actionName == actionUpdate {
		switch {
		case forceBackupBeforeUpdate:
			backupBeforeUpdate = boolPtr(true)
		case skipBackupBeforeUpdate:
			backupBeforeUpdate = boolPtr(false)
		}
	}
	baseRevision := ""
	commitMessage := ""
	if action == controllerv1.ServiceAction_SERVICE_ACTION_UPDATE && (len(imageUpdates) > 0 || useAllDetectedImages) {
		head, err := application.client.repos.GetRepoHead(application.ctx, newRequest(&controllerv1.GetRepoHeadRequest{}))
		if err != nil {
			return err
		}
		baseRevision = strings.TrimSpace(head.Msg.GetHeadRevision())
		if baseRevision == "" {
			return errors.New("repo head_revision is required for repo-backed image updates")
		}
		commitMessage = "update images for " + serviceName
	}
	response, err := application.client.serviceCommands.RunServiceAction(application.ctx, newRequest(&controllerv1.RunServiceActionRequest{
		ServiceName:                serviceName,
		Action:                     action,
		NodeIds:                    []string(nodes),
		DataNames:                  []string(dataNames),
		ComposeRecreateMode:        composeRecreateMode,
		ImageUpdates:               imageUpdates,
		UseAllDetectedImageUpdates: useAllDetectedImages,
		BackupBeforeUpdate:         backupBeforeUpdate,
		BaseRevision:               baseRevision,
		CommitMessage:              commitMessage,
	}))
	if err != nil {
		return err
	}
	return application.printServiceActionWithWait(response.Msg, waitOptions)
}

func (application *app) runServiceMigrate(serviceName string, args []string) error {
	fs := newCommandFlagSet("service migrate")
	sourceNodeID := fs.String("source", "", "source node ID")
	fs.StringVar(sourceNodeID, "from", "", "source node ID")
	targetNodeID := fs.String("target", "", "target node ID")
	fs.StringVar(targetNodeID, "to", "", "target node ID")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia service <service> migrate [--wait] [--follow] [--timeout duration] --source node --target node"); err != nil {
		return err
	}
	if strings.TrimSpace(*sourceNodeID) == "" {
		return errorsWithUsage("source node is required", "composia service <service> migrate [--wait] [--follow] [--timeout duration] --source node --target node")
	}
	if strings.TrimSpace(*targetNodeID) == "" {
		return errorsWithUsage("target node is required", "composia service <service> migrate [--wait] [--follow] [--timeout duration] --source node --target node")
	}
	response, err := application.client.serviceCommands.MigrateService(application.ctx, newRequest(&controllerv1.MigrateServiceRequest{
		ServiceName:  serviceName,
		SourceNodeId: strings.TrimSpace(*sourceNodeID),
		TargetNodeId: strings.TrimSpace(*targetNodeID),
	}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runServiceLogs(serviceName string, args []string) error {
	fs := newCommandFlagSet("service logs")
	taskID := fs.String("task", "", "task ID to stream instead of resolving the latest service task")
	nodeID := fs.String("node", "", "node ID filter")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *taskID != "" {
		if err := requireArgs(fs.Args(), 0, "composia service <service> logs [--task task] [--node node]"); err != nil {
			return err
		}
		return application.streamTaskLogs(application.ctx, *taskID)
	}
	if err := requireArgs(fs.Args(), 0, "composia service <service> logs [--task task] [--node node]"); err != nil {
		return err
	}
	request := &controllerv1.ListTasksRequest{ServiceName: []string{serviceName}, PageSize: 1, Page: 1}
	if strings.TrimSpace(*nodeID) != "" {
		request.NodeId = []string{strings.TrimSpace(*nodeID)}
	}
	response, err := application.client.tasks.ListTasks(application.ctx, newRequest(request))
	if err != nil {
		return err
	}
	tasks := response.Msg.GetTasks()
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks found for service %q", serviceName)
	}
	return application.streamTaskLogs(application.ctx, tasks[0].GetTaskId())
}

func (application *app) runServicePS(serviceName string, args []string) error {
	if err := requireArgs(args, 0, "composia service <service> ps"); err != nil {
		return err
	}
	response, err := application.client.services.GetService(application.ctx, newRequest(&controllerv1.GetServiceRequest{ServiceName: serviceName, IncludeContainers: true}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := serviceContainerRows(response.Msg.GetInstances())
	return application.writeTable([]string{"NODE", "CONTAINER", "NAME", "IMAGE", "STATE", "STATUS", "SERVICE"}, rows)
}

func (application *app) runServiceExec(serviceName string, args []string) error {
	fs := newCommandFlagSet("service exec")
	nodeID := fs.String("node", "", "node ID filter")
	containerName := fs.String("container", "", "container ID, name, or compose service")
	noTTY := fs.Bool("no-tty", false, "run without an interactive terminal")
	timeout := fs.Duration("timeout", 30*time.Second, "non-interactive exec timeout")
	maxOutput := fs.Uint64("max-output", 1024*1024, "maximum bytes per output stream")
	if err := fs.Parse(args); err != nil {
		return err
	}
	command := fs.Args()
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	}
	target, err := application.resolveServiceExecTarget(serviceName, strings.TrimSpace(*nodeID), strings.TrimSpace(*containerName))
	if err != nil {
		return err
	}
	if !*noTTY {
		return application.runContainerExecTTY(target.nodeID, target.containerID, command, 24, 80)
	}
	timeoutSeconds, err := durationSeconds(*timeout)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(application.ctx, localExecWaitTimeout(*timeout))
	defer cancel()
	response, err := application.client.dockerCommands.RunContainerExec(ctx, newRequest(&controllerv1.RunContainerExecRequest{NodeId: target.nodeID, ContainerId: target.containerID, Command: command, TimeoutSeconds: timeoutSeconds, MaxOutputBytes: *maxOutput}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		if err := application.printMessage(response.Msg); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprint(application.out, response.Msg.GetStdout()); err != nil {
			return err
		}
		if _, err := fmt.Fprint(application.errOut, response.Msg.GetStderr()); err != nil {
			return err
		}
	}
	if response.Msg.GetTimedOut() {
		return fmt.Errorf("service exec timed out after %s", *timeout)
	}
	if response.Msg.GetExitCode() != 0 {
		return fmt.Errorf("service exec exited with code %d", response.Msg.GetExitCode())
	}
	return nil
}

func (application *app) printServiceDetail(service *controllerv1.GetServiceResponse) error {
	if err := application.writeKV([][2]string{
		{"name", service.GetName()},
		{"runtime_status", service.GetRuntimeStatus()},
		{"updated_at", service.GetUpdatedAt()},
		{"nodes", strings.Join(service.GetNodes(), ",")},
		{"enabled", boolText(service.GetEnabled())},
		{"directory", service.GetDirectory()},
	}); err != nil {
		return err
	}
	instances := service.GetInstances()
	if len(instances) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(application.out); err != nil {
		return err
	}
	rows := make([][]string, 0, len(instances))
	for _, instance := range instances {
		rows = append(rows, []string{
			instance.GetServiceName(),
			instance.GetNodeId(),
			instance.GetRuntimeStatus(),
			boolText(instance.GetIsDeclared()),
			strconv.Itoa(len(instance.GetContainers())),
			instance.GetUpdatedAt(),
		})
	}
	return application.writeTable([]string{"SERVICE", "NODE", "STATUS", "DECLARED", "CONTAINERS", "UPDATED"}, rows)
}

func serviceActionFromName(name string) (controllerv1.ServiceAction, error) {
	switch name {
	case "up", "deploy":
		return controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY, nil
	case actionUpdate:
		return controllerv1.ServiceAction_SERVICE_ACTION_UPDATE, nil
	case "down", "stop":
		return controllerv1.ServiceAction_SERVICE_ACTION_STOP, nil
	case actionRestart:
		return controllerv1.ServiceAction_SERVICE_ACTION_RESTART, nil
	case "backup":
		return controllerv1.ServiceAction_SERVICE_ACTION_BACKUP, nil
	case "dns-update":
		return controllerv1.ServiceAction_SERVICE_ACTION_DNS_UPDATE, nil
	case "caddy-sync":
		return controllerv1.ServiceAction_SERVICE_ACTION_CADDY_SYNC, nil
	case "cloudflare-tunnel-sync", "tunnel-sync":
		return controllerv1.ServiceAction_SERVICE_ACTION_CLOUDFLARE_TUNNEL_SYNC, nil
	default:
		return controllerv1.ServiceAction_SERVICE_ACTION_UNSPECIFIED, fmt.Errorf("unknown service action %q", name)
	}
}

func imageUpdateSelections(imageNames, setImages []string, useDetected bool) ([]*controllerv1.ImageUpdateSelection, error) {
	if len(imageNames) == 0 && len(setImages) == 0 {
		return nil, nil
	}
	selectionsByName := make(map[string]*controllerv1.ImageUpdateSelection, len(imageNames)+len(setImages))
	for _, imageName := range imageNames {
		imageName = strings.TrimSpace(imageName)
		if imageName == "" {
			continue
		}
		if _, exists := selectionsByName[imageName]; exists {
			return nil, fmt.Errorf("image update %q is duplicated", imageName)
		}
		selectionsByName[imageName] = &controllerv1.ImageUpdateSelection{ImageName: imageName, UseDetected: useDetected}
	}
	for _, assignment := range setImages {
		assignment = strings.TrimSpace(assignment)
		if assignment == "" {
			continue
		}
		imageName, targetTag, ok := strings.Cut(assignment, "=")
		imageName = strings.TrimSpace(imageName)
		targetTag = strings.TrimSpace(targetTag)
		if !ok || imageName == "" || targetTag == "" {
			return nil, fmt.Errorf("invalid set-image %q; expected name=tag", assignment)
		}
		if _, exists := selectionsByName[imageName]; exists {
			return nil, fmt.Errorf("image update %q is duplicated", imageName)
		}
		selectionsByName[imageName] = &controllerv1.ImageUpdateSelection{ImageName: imageName, TargetTag: targetTag}
	}
	selections := make([]*controllerv1.ImageUpdateSelection, 0, len(selectionsByName))
	for _, selection := range selectionsByName {
		selections = append(selections, selection)
	}
	slices.SortFunc(selections, func(left, right *controllerv1.ImageUpdateSelection) int {
		return strings.Compare(left.GetImageName(), right.GetImageName())
	})
	return selections, nil
}

func boolPtr(value bool) *bool {
	return &value
}

func composeRecreateModeFromName(name string) (controllerv1.ComposeRecreateMode, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto":
		return controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_AUTO, nil
	case "never":
		return controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_NO_RECREATE, nil
	case "always":
		return controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_FORCE_RECREATE, nil
	default:
		return controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_UNSPECIFIED, fmt.Errorf("unsupported recreate mode %q; expected auto, never, or always", name)
	}
}

func errorsWithUsage(message string, usage string) error {
	return fmt.Errorf("%s; usage: %s", message, usage)
}

type serviceExecTarget struct {
	nodeID      string
	containerID string
}

func (application *app) resolveServiceFolder(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("service is required")
	}
	response, err := application.client.services.ListServiceWorkspaces(application.ctx, newRequest(&controllerv1.ListServiceWorkspacesRequest{}))
	if err != nil {
		return "", err
	}
	matches := make([]string, 0, 1)
	for _, workspace := range response.Msg.GetWorkspaces() {
		if input == workspace.GetFolder() || input == workspace.GetServiceName() || input == workspace.GetDisplayName() {
			matches = append(matches, workspace.GetFolder())
		}
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("service %q matches multiple workspaces; use the folder name", input)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return normalizeServiceRelativePath(input)
}

func (application *app) resolveServiceExecTarget(serviceName string, nodeID string, containerFilter string) (serviceExecTarget, error) {
	response, err := application.client.services.GetService(application.ctx, newRequest(&controllerv1.GetServiceRequest{ServiceName: serviceName, IncludeContainers: true}))
	if err != nil {
		return serviceExecTarget{}, err
	}
	matches := make([]serviceExecTarget, 0, 1)
	for _, instance := range response.Msg.GetInstances() {
		if nodeID != "" && nodeID != instance.GetNodeId() {
			continue
		}
		for _, container := range instance.GetContainers() {
			if containerFilter != "" && containerFilter != container.GetContainerId() && containerFilter != container.GetName() && containerFilter != container.GetComposeService() {
				continue
			}
			matches = append(matches, serviceExecTarget{nodeID: instance.GetNodeId(), containerID: container.GetContainerId()})
		}
	}
	if len(matches) == 0 {
		return serviceExecTarget{}, fmt.Errorf("no container found for service %q", serviceName)
	}
	if len(matches) > 1 {
		return serviceExecTarget{}, fmt.Errorf("multiple containers found for service %q; pass --node and/or --container", serviceName)
	}
	return matches[0], nil
}

func serviceContainerRows(instances []*controllerv1.ServiceInstanceDetail) [][]string {
	rows := make([][]string, 0)
	for _, instance := range instances {
		for _, container := range instance.GetContainers() {
			rows = append(rows, []string{
				instance.GetNodeId(),
				container.GetContainerId(),
				container.GetName(),
				container.GetImage(),
				container.GetState(),
				container.GetStatus(),
				container.GetComposeService(),
			})
		}
	}
	return rows
}

func serviceRepoPath(folder string, filePath string) string {
	return strings.TrimRight(folder, "/") + "/" + strings.TrimLeft(filePath, "/")
}

func normalizeServiceRelativePath(input string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(input, "\\", "/"))
	if trimmed == "" {
		return "", nil
	}
	parts := strings.Split(trimmed, "/")
	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if part == "." || part == ".." {
			return "", errors.New("path must stay inside the service directory")
		}
		cleanParts = append(cleanParts, part)
	}
	return strings.Join(cleanParts, "/"), nil
}
