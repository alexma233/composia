package cli

import (
	"fmt"
	"strings"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runService(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia service <list|get|deploy|update|stop|restart|backup|dns-update|caddy-sync|migrate>")
	}
	switch args[0] {
	case "list":
		return application.runServiceList(args[1:])
	case "get":
		return application.runServiceGet(args[1:])
	case "deploy", "update", "stop", "restart", "backup", "dns-update", "caddy-sync":
		return application.runServiceAction(args[0], args[1:])
	case "migrate":
		return application.runServiceMigrate(args[1:])
	default:
		return fmt.Errorf("unknown service command %q", args[0])
	}
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
	pageSize, page := pageValues()
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
	fs.Var(&nodes, "node", "target node ID; repeat or comma-separate")
	fs.Var(&dataNames, "data", "data entry name for backup-like actions; repeat or comma-separate")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, fmt.Sprintf("composia service %s [--wait] [--follow] [--timeout duration] [--node node] [--data name] <service>", actionName)); err != nil {
		return err
	}
	response, err := application.client.serviceCommands.RunServiceAction(application.ctx, newRequest(&controllerv1.RunServiceActionRequest{
		ServiceName: fs.Arg(0),
		Action:      action,
		NodeIds:     []string(nodes),
		DataNames:   []string(dataNames),
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

func errorsWithUsage(message string, usage string) error {
	return fmt.Errorf("%s; usage: %s", message, usage)
}
