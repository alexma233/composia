package cli

import (
	"fmt"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runRustic(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia rustic <init|forget|prune>")
	}
	switch args[0] {
	case "init":
		return application.runRusticInit(args[1:])
	case "forget", "prune":
		return application.runRusticMaintenance(args[0], args[1:])
	default:
		return fmt.Errorf("unknown rustic command %q", args[0])
	}
}

func (application *app) runRusticInit(args []string) error {
	fs := newCommandFlagSet("rustic init")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia rustic init [--wait] [--follow] [--timeout duration] <node>"); err != nil {
		return err
	}
	response, err := application.client.nodeCommands.InitNodeRustic(application.ctx, newRequest(&controllerv1.InitNodeRusticRequest{NodeId: fs.Arg(0)}))
	if err != nil {
		return err
	}
	return application.printTaskIDWithWait(response.Msg, response.Msg.GetTaskId(), waitOptions)
}

func (application *app) runRusticMaintenance(action string, args []string) error {
	fs := newCommandFlagSet("rustic " + action)
	serviceName := fs.String("service", "", "service name")
	dataName := fs.String("data", "", "data entry name")
	waitOptions := addWaitFlags(fs)
	usage := fmt.Sprintf("composia rustic %s [--wait] [--follow] [--timeout duration] [--service name] [--data name] <node>", action)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	if action == "forget" {
		response, err := application.client.nodeCommands.ForgetNodeRustic(application.ctx, newRequest(&controllerv1.ForgetNodeRusticRequest{NodeId: fs.Arg(0), ServiceName: *serviceName, DataName: *dataName}))
		if err != nil {
			return err
		}
		return application.printTaskIDWithWait(response.Msg, response.Msg.GetTaskId(), waitOptions)
	}
	response, err := application.client.nodeCommands.PruneNodeRustic(application.ctx, newRequest(&controllerv1.PruneNodeRusticRequest{NodeId: fs.Arg(0), ServiceName: *serviceName, DataName: *dataName}))
	if err != nil {
		return err
	}
	return application.printTaskIDWithWait(response.Msg, response.Msg.GetTaskId(), waitOptions)
}
