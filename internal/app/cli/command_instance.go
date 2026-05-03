package cli

import (
	"fmt"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runInstance(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia instance <list|get|deploy|update|stop|restart|backup>")
	}
	switch args[0] {
	case "list":
		return application.runInstanceList(args[1:])
	case "get":
		return application.runInstanceGet(args[1:])
	case "deploy", "update", "stop", "restart":
		return application.runInstanceAction(args[0], args[1:])
	case "backup":
		return application.runInstanceBackup(args[1:])
	default:
		return fmt.Errorf("unknown instance command %q", args[0])
	}
}

func (application *app) runInstanceList(args []string) error {
	if err := requireArgs(args, 1, "composia instance list <service>"); err != nil {
		return err
	}
	response, err := application.client.instances.ListServiceInstances(application.ctx, newRequest(&controllerv1.ListServiceInstancesRequest{ServiceName: args[0]}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetInstances()))
	for _, instance := range response.Msg.GetInstances() {
		rows = append(rows, []string{
			instance.GetServiceName(),
			instance.GetNodeId(),
			instance.GetRuntimeStatus(),
			boolText(instance.GetIsDeclared()),
			instance.GetUpdatedAt(),
		})
	}
	return application.writeTable([]string{"SERVICE", "NODE", "STATUS", "DECLARED", "UPDATED"}, rows)
}

func (application *app) runInstanceGet(args []string) error {
	fs := newCommandFlagSet("instance get")
	includeContainers := fs.Bool("containers", false, "include containers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 2, "composia instance get [--containers] <service> <node>"); err != nil {
		return err
	}
	response, err := application.client.instances.GetServiceInstance(application.ctx, newRequest(&controllerv1.GetServiceInstanceRequest{ServiceName: fs.Arg(0), NodeId: fs.Arg(1), IncludeContainers: *includeContainers}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	if response.Msg.GetInstance() == nil {
		return fmt.Errorf("service instance %q on node %q was not found", fs.Arg(0), fs.Arg(1))
	}
	return application.printInstanceDetail(response.Msg.GetInstance())
}

func (application *app) runInstanceAction(actionName string, args []string) error {
	action, err := instanceActionFromName(actionName)
	if err != nil {
		return err
	}
	fs := newCommandFlagSet("instance " + actionName)
	recreateMode := "auto"
	if actionName == "deploy" || actionName == "update" {
		fs.StringVar(&recreateMode, "recreate", "auto", "compose recreate mode: auto, no_recreate, force_recreate")
	}
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 2, fmt.Sprintf("composia instance %s [--wait] [--follow] [--timeout duration] [--recreate auto|no_recreate|force_recreate] <service> <node>", actionName)); err != nil {
		return err
	}
	composeRecreateMode, err := composeRecreateModeFromName(recreateMode)
	if err != nil {
		return err
	}
	response, err := application.client.instances.RunServiceInstanceAction(application.ctx, newRequest(&controllerv1.RunServiceInstanceActionRequest{ServiceName: fs.Arg(0), NodeId: fs.Arg(1), Action: action, ComposeRecreateMode: composeRecreateMode}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runInstanceBackup(args []string) error {
	fs := newCommandFlagSet("instance backup")
	var dataNames stringListFlag
	fs.Var(&dataNames, "data", "data entry name; repeat or comma-separate")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 2, "composia instance backup [--wait] [--follow] [--timeout duration] [--data name] <service> <node>"); err != nil {
		return err
	}
	response, err := application.client.serviceCommands.RunServiceAction(application.ctx, newRequest(&controllerv1.RunServiceActionRequest{
		ServiceName: fs.Arg(0),
		Action:      controllerv1.ServiceAction_SERVICE_ACTION_BACKUP,
		NodeIds:     []string{fs.Arg(1)},
		DataNames:   []string(dataNames),
	}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) printInstanceDetail(instance *controllerv1.ServiceInstanceDetail) error {
	if err := application.writeKV([][2]string{
		{"service_name", instance.GetServiceName()},
		{"node_id", instance.GetNodeId()},
		{"runtime_status", instance.GetRuntimeStatus()},
		{"updated_at", instance.GetUpdatedAt()},
		{"is_declared", boolText(instance.GetIsDeclared())},
	}); err != nil {
		return err
	}
	containers := instance.GetContainers()
	if len(containers) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(application.out); err != nil {
		return err
	}
	rows := make([][]string, 0, len(containers))
	for _, container := range containers {
		rows = append(rows, []string{
			container.GetContainerId(),
			container.GetName(),
			container.GetImage(),
			container.GetState(),
			container.GetStatus(),
			container.GetComposeProject(),
			container.GetComposeService(),
		})
	}
	return application.writeTable([]string{"CONTAINER", "NAME", "IMAGE", "STATE", "STATUS", "PROJECT", "SERVICE"}, rows)
}

func instanceActionFromName(name string) (controllerv1.ServiceInstanceAction, error) {
	switch name {
	case "deploy":
		return controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_DEPLOY, nil
	case "update":
		return controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_UPDATE, nil
	case "stop":
		return controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_STOP, nil
	case "restart":
		return controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_RESTART, nil
	default:
		return controllerv1.ServiceInstanceAction_SERVICE_INSTANCE_ACTION_UNSPECIFIED, fmt.Errorf("unknown instance action %q", name)
	}
}
