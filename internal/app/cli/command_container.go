package cli

import (
	"fmt"
	"strings"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runContainer(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia container <list|get|logs|start|stop|restart|remove|exec>")
	}
	switch args[0] {
	case "list":
		return application.runContainerList(args[1:])
	case "get":
		return application.runContainerGet(args[1:])
	case "logs":
		return application.runContainerLogs(args[1:])
	case "start", "stop", "restart":
		return application.runContainerAction(args[0], args[1:])
	case "remove":
		return application.runContainerRemove(args[1:])
	case "exec":
		return application.runContainerExec(args[1:])
	default:
		return fmt.Errorf("unknown container command %q", args[0])
	}
}

func (application *app) runContainerList(args []string) error {
	fs := newCommandFlagSet("container list")
	nodeID := fs.String("node", "", "node ID")
	search := fs.String("search", "", "search text")
	sortBy := fs.String("sort-by", "", "sort field")
	sortDesc := fs.Bool("desc", false, "sort descending")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia container list --node node [--search text] [--sort-by field] [--desc]"); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", "composia container list --node node [--search text] [--sort-by field] [--desc]")
	}
	pageSize, page := pageValues()
	response, err := application.client.docker.ListNodeContainers(application.ctx, newRequest(&controllerv1.ListNodeContainersRequest{
		NodeId:   strings.TrimSpace(*nodeID),
		PageSize: pageSize,
		Page:     page,
		Search:   *search,
		SortBy:   *sortBy,
		SortDesc: *sortDesc,
	}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetContainers()))
	for _, container := range response.Msg.GetContainers() {
		rows = append(rows, []string{
			container.GetId(),
			container.GetName(),
			container.GetImage(),
			container.GetState(),
			container.GetStatus(),
			strings.Join(container.GetPorts(), ","),
			strings.Join(container.GetNetworks(), ","),
		})
	}
	if err := writeTable(application.out, []string{"CONTAINER", "NAME", "IMAGE", "STATE", "STATUS", "PORTS", "NETWORKS"}, rows); err != nil {
		return err
	}
	_, err = fmt.Fprintf(application.out, "total_count: %d\n", response.Msg.GetTotalCount())
	return err
}

func (application *app) runContainerGet(args []string) error {
	fs := newCommandFlagSet("container get")
	nodeID := fs.String("node", "", "node ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia container get --node node <container>"); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", "composia container get --node node <container>")
	}
	response, err := application.client.docker.InspectNodeContainer(application.ctx, newRequest(&controllerv1.InspectNodeContainerRequest{NodeId: strings.TrimSpace(*nodeID), ContainerId: fs.Arg(0)}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	_, err = fmt.Fprintln(application.out, response.Msg.GetRawJson())
	return err
}

func (application *app) runContainerLogs(args []string) error {
	fs := newCommandFlagSet("container logs")
	nodeID := fs.String("node", "", "node ID")
	tail := fs.String("tail", "100", "number of lines or all")
	timestamps := fs.Bool("timestamps", false, "include timestamps")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia container logs --node node [--tail n|all] [--timestamps] <container>"); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", "composia container logs --node node [--tail n|all] [--timestamps] <container>")
	}
	stream, err := application.client.containers.GetContainerLogs(application.ctx, newRequest(&controllerv1.GetContainerLogsRequest{NodeId: strings.TrimSpace(*nodeID), ContainerId: fs.Arg(0), Tail: *tail, Timestamps: *timestamps}))
	if err != nil {
		return err
	}
	for stream.Receive() {
		if application.cfg.json {
			if err := application.printMessage(stream.Msg()); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprint(application.out, stream.Msg().GetContent()); err != nil {
			return err
		}
	}
	return stream.Err()
}

func (application *app) runContainerAction(actionName string, args []string) error {
	action, err := containerActionFromName(actionName)
	if err != nil {
		return err
	}
	fs := newCommandFlagSet("container " + actionName)
	nodeID := fs.String("node", "", "node ID")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := fmt.Sprintf("composia container %s [--wait] [--follow] [--timeout duration] --node node <container>", actionName)
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", usage)
	}
	response, err := application.client.containers.RunContainerAction(application.ctx, newRequest(&controllerv1.RunContainerActionRequest{NodeId: strings.TrimSpace(*nodeID), ContainerId: fs.Arg(0), Action: action}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runContainerRemove(args []string) error {
	fs := newCommandFlagSet("container remove")
	nodeID := fs.String("node", "", "node ID")
	force := fs.Bool("force", false, "force remove")
	volumes := fs.Bool("volumes", false, "remove anonymous volumes")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := "composia container remove [--wait] [--follow] [--timeout duration] --node node [--force] [--volumes] <container>"
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", usage)
	}
	response, err := application.client.containers.RemoveContainer(application.ctx, newRequest(&controllerv1.RemoveContainerRequest{NodeId: strings.TrimSpace(*nodeID), ContainerId: fs.Arg(0), Force: *force, RemoveVolumes: *volumes}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runContainerExec(args []string) error {
	fs := newCommandFlagSet("container exec")
	nodeID := fs.String("node", "", "node ID")
	rows := fs.Uint("rows", 24, "terminal rows")
	cols := fs.Uint("cols", 80, "terminal columns")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) < 2 {
		return fmt.Errorf("usage: composia container exec --node node <container> -- <command> [args...]")
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", "composia container exec --node node <container> -- <command> [args...]")
	}
	response, err := application.client.containers.OpenContainerExec(application.ctx, newRequest(&controllerv1.OpenContainerExecRequest{
		NodeId:      strings.TrimSpace(*nodeID),
		ContainerId: fs.Arg(0),
		Command:     fs.Args()[1:],
		Rows:        uint32(*rows),
		Cols:        uint32(*cols),
	}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	return writeKV(application.out, [][2]string{
		{"session_id", response.Msg.GetSessionId()},
		{"websocket_path", response.Msg.GetWebsocketPath()},
	})
}

func containerActionFromName(name string) (controllerv1.ContainerAction, error) {
	switch name {
	case "start":
		return controllerv1.ContainerAction_CONTAINER_ACTION_START, nil
	case "stop":
		return controllerv1.ContainerAction_CONTAINER_ACTION_STOP, nil
	case "restart":
		return controllerv1.ContainerAction_CONTAINER_ACTION_RESTART, nil
	default:
		return controllerv1.ContainerAction_CONTAINER_ACTION_UNSPECIFIED, fmt.Errorf("unknown container action %q", name)
	}
}
