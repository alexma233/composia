package cli

import (
	"fmt"
	"slices"
	"strings"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runService(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia service <list|get|update-candidates|deploy|update|stop|restart|backup|dns-update|caddy-sync|migrate>")
	}
	switch args[0] {
	case "list":
		return application.runServiceList(args[1:])
	case "get":
		return application.runServiceGet(args[1:])
	case "update-candidates":
		return application.runServiceUpdateCandidates(args[1:])
	case "deploy", "update", "stop", "restart", "backup", "dns-update", "caddy-sync":
		return application.runServiceAction(args[0], args[1:])
	case "migrate":
		return application.runServiceMigrate(args[1:])
	default:
		return fmt.Errorf("unknown service command %q", args[0])
	}
}

func (application *app) runServiceUpdateCandidates(args []string) error {
	fs := newCommandFlagSet("service update-candidates")
	nodeID := fs.String("node", "", "node ID filter")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia service update-candidates [--node node] <service>"); err != nil {
		return err
	}
	response, err := application.client.services.GetServiceImageUpdateChecks(application.ctx, newRequest(&controllerv1.GetServiceImageUpdateChecksRequest{ServiceName: fs.Arg(0), NodeId: *nodeID}))
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

func (application *app) runServiceGet(args []string) error {
	fs := newCommandFlagSet("service get")
	includeContainers := fs.Bool("containers", false, "include per-instance containers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia service get [--containers] <service>"); err != nil {
		return err
	}
	serviceName := fs.Arg(0)
	response, err := application.client.services.GetService(application.ctx, newRequest(&controllerv1.GetServiceRequest{ServiceName: serviceName, IncludeContainers: *includeContainers}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	return application.printServiceDetail(response.Msg)
}

func (application *app) runServiceAction(actionName string, args []string) error {
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
	fs.Var(&nodes, "node", "target node ID; repeat or comma-separate")
	fs.Var(&dataNames, "data", "data entry name for backup-like actions; repeat or comma-separate")
	if actionName == "deploy" || actionName == "update" {
		fs.StringVar(&recreateMode, "recreate", "auto", "compose recreate mode: auto, no_recreate, force_recreate")
	}
	if actionName == "update" {
		fs.Var(&imageNames, "image", "configured image update name; repeat or comma-separate")
		fs.Var(&setImages, "set-image", "configured image update assignment name=tag; repeat or comma-separate")
		fs.BoolVar(&useDetectedImages, "use-detected", false, "apply detected candidate for --image entries")
		fs.BoolVar(&useAllDetectedImages, "all-detected", false, "apply all detected image updates")
	}
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := commandUsageText("service " + actionName)
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	composeRecreateMode, err := composeRecreateModeFromName(recreateMode)
	if err != nil {
		return err
	}
	imageUpdates, err := imageUpdateSelections([]string(imageNames), []string(setImages), useDetectedImages)
	if err != nil {
		return err
	}
	response, err := application.client.serviceCommands.RunServiceAction(application.ctx, newRequest(&controllerv1.RunServiceActionRequest{
		ServiceName:                fs.Arg(0),
		Action:                     action,
		NodeIds:                    []string(nodes),
		DataNames:                  []string(dataNames),
		ComposeRecreateMode:        composeRecreateMode,
		ImageUpdates:               imageUpdates,
		UseAllDetectedImageUpdates: useAllDetectedImages,
	}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runServiceMigrate(args []string) error {
	fs := newCommandFlagSet("service migrate")
	sourceNodeID := fs.String("source", "", "source node ID")
	fs.StringVar(sourceNodeID, "from", "", "source node ID")
	targetNodeID := fs.String("target", "", "target node ID")
	fs.StringVar(targetNodeID, "to", "", "target node ID")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia service migrate [--wait] [--follow] [--timeout duration] --source node --target node <service>"); err != nil {
		return err
	}
	if strings.TrimSpace(*sourceNodeID) == "" {
		return errorsWithUsage("source node is required", "composia service migrate [--wait] [--follow] [--timeout duration] --source node --target node <service>")
	}
	if strings.TrimSpace(*targetNodeID) == "" {
		return errorsWithUsage("target node is required", "composia service migrate [--wait] [--follow] [--timeout duration] --source node --target node <service>")
	}
	response, err := application.client.serviceCommands.MigrateService(application.ctx, newRequest(&controllerv1.MigrateServiceRequest{
		ServiceName:  fs.Arg(0),
		SourceNodeId: strings.TrimSpace(*sourceNodeID),
		TargetNodeId: strings.TrimSpace(*targetNodeID),
	}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
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
			fmt.Sprintf("%d", len(instance.GetContainers())),
			instance.GetUpdatedAt(),
		})
	}
	return application.writeTable([]string{"SERVICE", "NODE", "STATUS", "DECLARED", "CONTAINERS", "UPDATED"}, rows)
}

func serviceActionFromName(name string) (controllerv1.ServiceAction, error) {
	switch name {
	case "deploy":
		return controllerv1.ServiceAction_SERVICE_ACTION_DEPLOY, nil
	case "update":
		return controllerv1.ServiceAction_SERVICE_ACTION_UPDATE, nil
	case "stop":
		return controllerv1.ServiceAction_SERVICE_ACTION_STOP, nil
	case "restart":
		return controllerv1.ServiceAction_SERVICE_ACTION_RESTART, nil
	case "backup":
		return controllerv1.ServiceAction_SERVICE_ACTION_BACKUP, nil
	case "dns-update":
		return controllerv1.ServiceAction_SERVICE_ACTION_DNS_UPDATE, nil
	case "caddy-sync":
		return controllerv1.ServiceAction_SERVICE_ACTION_CADDY_SYNC, nil
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

func composeRecreateModeFromName(name string) (controllerv1.ComposeRecreateMode, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto":
		return controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_AUTO, nil
	case "no_recreate", "no-recreate", "never":
		return controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_NO_RECREATE, nil
	case "force_recreate", "force-recreate", "always":
		return controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_FORCE_RECREATE, nil
	default:
		return controllerv1.ComposeRecreateMode_COMPOSE_RECREATE_MODE_UNSPECIFIED, fmt.Errorf("unsupported recreate mode %q; expected auto, no_recreate, or force_recreate", name)
	}
}

func errorsWithUsage(message string, usage string) error {
	return fmt.Errorf("%s; usage: %s", message, usage)
}
