package cli

import (
	"fmt"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"google.golang.org/protobuf/proto"
)

func (application *app) runNode(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia node <list|get|tasks|stats|reload-caddy|prune>")
	}
	switch args[0] {
	case "list":
		return application.runNodeList(args[1:])
	case "get":
		return application.runNodeGet(args[1:])
	case "tasks":
		return application.runNodeTasks(args[1:])
	case "stats":
		return application.runNodeStats(args[1:])
	case "reload-caddy":
		return application.runNodeReloadCaddy(args[1:])
	case "prune":
		return application.runNodePrune(args[1:])
	default:
		return fmt.Errorf("unknown node command %q", args[0])
	}
}

func (application *app) runNodeStats(args []string) error {
	if err := requireArgs(args, 1, "composia node stats <node>"); err != nil {
		return err
	}
	response, err := application.client.nodes.GetNodeDockerStats(application.ctx, newRequest(&controllerv1.GetNodeDockerStatsRequest{NodeId: args[0]}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	stats := response.Msg.GetStats()
	if stats == nil {
		return fmt.Errorf("docker stats for node %q were not found", args[0])
	}
	return application.writeKV([][2]string{
		{"containers_total", uintText(stats.GetContainersTotal())},
		{"containers_running", uintText(stats.GetContainersRunning())},
		{"containers_stopped", uintText(stats.GetContainersStopped())},
		{"containers_paused", uintText(stats.GetContainersPaused())},
		{"images", uintText(stats.GetImages())},
		{"networks", uintText(stats.GetNetworks())},
		{"volumes", uintText(stats.GetVolumes())},
		{"volumes_size_bytes", uint64Text(stats.GetVolumesSizeBytes())},
		{"disks_usage_bytes", uint64Text(stats.GetDisksUsageBytes())},
		{"docker_server_version", stats.GetDockerServerVersion()},
	})
}

func (application *app) runNodeList(args []string) error {
	if err := requireArgs(args, 0, "composia node list"); err != nil {
		return err
	}
	response, err := application.client.nodes.ListNodes(application.ctx, newRequest(&controllerv1.ListNodesRequest{}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetNodes()))
	for _, node := range response.Msg.GetNodes() {
		rows = append(rows, []string{
			node.GetNodeId(),
			node.GetDisplayName(),
			boolText(node.GetEnabled()),
			boolText(node.GetIsOnline()),
			node.GetLastHeartbeat(),
		})
	}
	return application.writeTable([]string{"NODE", "NAME", "ENABLED", "ONLINE", "LAST_HEARTBEAT"}, rows)
}

func (application *app) runNodeGet(args []string) error {
	if err := requireArgs(args, 1, "composia node get <node>"); err != nil {
		return err
	}
	response, err := application.client.nodes.GetNode(application.ctx, newRequest(&controllerv1.GetNodeRequest{NodeId: args[0]}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	node := response.Msg.GetNode()
	if node == nil {
		return fmt.Errorf("node %q was not found", args[0])
	}
	caddySyncEnabled, caddySyncReason := capabilityText(node.GetActions().GetCaddySync())
	caddyReloadEnabled, caddyReloadReason := capabilityText(node.GetActions().GetCaddyReload())
	rusticEnabled, rusticReason := capabilityText(node.GetActions().GetRusticMaintenance())
	return application.writeKV([][2]string{
		{"node_id", node.GetNodeId()},
		{"display_name", node.GetDisplayName()},
		{"enabled", boolText(node.GetEnabled())},
		{"is_online", boolText(node.GetIsOnline())},
		{"last_heartbeat", node.GetLastHeartbeat()},
		{"caddy_sync_enabled", boolText(caddySyncEnabled)},
		{"caddy_sync_reason", caddySyncReason},
		{"caddy_reload_enabled", boolText(caddyReloadEnabled)},
		{"caddy_reload_reason", caddyReloadReason},
		{"rustic_maintenance_enabled", boolText(rusticEnabled)},
		{"rustic_maintenance_reason", rusticReason},
	})
}

func (application *app) runNodeTasks(args []string) error {
	fs := newCommandFlagSet("node tasks")
	status := fs.String("status", "", "status filter")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia node tasks [--status status] <node>"); err != nil {
		return err
	}
	pageSize, page := pageValues()
	response, err := application.client.nodes.GetNodeTasks(application.ctx, newRequest(&controllerv1.GetNodeTasksRequest{NodeId: fs.Arg(0), Status: *status, PageSize: pageSize, Page: page}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetTasks()))
	for _, task := range response.Msg.GetTasks() {
		rows = append(rows, []string{task.GetTaskId(), task.GetType(), task.GetStatus(), task.GetServiceName(), task.GetNodeId(), task.GetCreatedAt()})
	}
	if err := application.writeTable([]string{"TASK", "TYPE", "STATUS", "SERVICE", "NODE", "CREATED"}, rows); err != nil {
		return err
	}
	return application.writeCount("total_count", response.Msg.GetTotalCount())
}

func (application *app) runNodeReloadCaddy(args []string) error {
	fs := newCommandFlagSet("node reload-caddy")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia node reload-caddy [--wait] [--follow] [--timeout duration] <node>"); err != nil {
		return err
	}
	response, err := application.client.nodeCommands.ReloadNodeCaddy(application.ctx, newRequest(&controllerv1.ReloadNodeCaddyRequest{NodeId: fs.Arg(0)}))
	if err != nil {
		return err
	}
	return application.printTaskIDWithWait(response.Msg, response.Msg.GetTaskId(), waitOptions)
}

func (application *app) runNodePrune(args []string) error {
	fs := newCommandFlagSet("node prune")
	target := fs.String("target", "all", "Docker prune target")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia node prune [--wait] [--follow] [--timeout duration] [--target all|container|image|network|volume] <node>"); err != nil {
		return err
	}
	response, err := application.client.nodeCommands.PruneNodeDocker(application.ctx, newRequest(&controllerv1.PruneNodeDockerRequest{NodeId: fs.Arg(0), Target: *target}))
	if err != nil {
		return err
	}
	return application.printTaskIDWithWait(response.Msg, response.Msg.GetTaskId(), waitOptions)
}

func (application *app) printTaskID(message proto.Message, taskID string) error {
	if application.isJSONOutput() {
		return application.printMessage(message)
	}
	return application.writeKV([][2]string{{"task_id", taskID}})
}

func (application *app) printTaskIDWithWait(message proto.Message, taskID string, options waitOptions) error {
	if err := application.printTaskID(message, taskID); err != nil {
		return err
	}
	if !options.shouldWait() {
		return nil
	}
	return application.waitTask(taskID, options)
}

func capabilityText(capability *controllerv1.Capability) (bool, string) {
	if capability == nil {
		return false, ""
	}
	return capability.GetEnabled(), capability.GetReasonCode()
}
