package cli

import (
	"fmt"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runSystem(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia system <status|reload>")
	}
	switch args[0] {
	case "status":
		if err := requireArgs(args[1:], 0, "composia system status"); err != nil {
			return err
		}
		response, err := application.client.system.GetSystemStatus(application.ctx, newRequest(&controllerv1.GetSystemStatusRequest{}))
		if err != nil {
			return err
		}
		if application.cfg.json {
			return application.printMessage(response.Msg)
		}
		return application.printSystemStatus(response)
	case "reload":
		if err := requireArgs(args[1:], 0, "composia system reload"); err != nil {
			return err
		}
		response, err := application.client.system.ReloadControllerConfig(application.ctx, newRequest(&controllerv1.ReloadControllerConfigRequest{}))
		if err != nil {
			return err
		}
		if application.cfg.json {
			return application.printMessage(response.Msg)
		}
		return writeKV(application.out, [][2]string{{"accepted", boolText(response.Msg.GetAccepted())}})
	default:
		return fmt.Errorf("unknown system command %q", args[0])
	}
}

func (application *app) printSystemStatus(response *connect.Response[controllerv1.GetSystemStatusResponse]) error {
	message := response.Msg
	now := ""
	if message.GetNow() != nil {
		now = message.GetNow().AsTime().Format(time.RFC3339)
	}
	return writeKV(application.out, [][2]string{
		{"version", message.GetVersion()},
		{"now", now},
		{"configured_node_count", uint64Text(message.GetConfiguredNodeCount())},
		{"online_node_count", uint64Text(message.GetOnlineNodeCount())},
		{"repo_dir", message.GetRepoDir()},
		{"state_dir", message.GetStateDir()},
		{"log_dir", message.GetLogDir()},
	})
}
