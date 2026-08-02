package cli

import (
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runSystem(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: composia system <status|config edit|reload|capabilities>")
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
		if application.isJSONOutput() {
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
		if application.isJSONOutput() {
			return application.printMessage(response.Msg)
		}
		return application.writeKV([][2]string{{"accepted", boolText(response.Msg.GetAccepted())}})
	case "capabilities":
		if err := requireArgs(args[1:], 0, "composia system capabilities"); err != nil {
			return err
		}
		response, err := application.client.system.GetCapabilities(application.ctx, newRequest(&controllerv1.GetCapabilitiesRequest{}))
		if err != nil {
			return err
		}
		if application.isJSONOutput() {
			return application.printMessage(response.Msg)
		}
		return application.printSystemCapabilities(response.Msg)
	case "config":
		if len(args) != 2 || args[1] != "edit" {
			return errors.New("usage: composia system config edit")
		}
		return application.runControllerConfigEdit()
	default:
		return fmt.Errorf("unknown system command %q", args[0])
	}
}

func (application *app) runControllerConfigEdit() error {
	response, err := application.client.controllerConfig.GetEditableConfig(application.ctx, newRequest(&controllerv1.GetEditableConfigRequest{}))
	if err != nil {
		return err
	}
	updated, changed, err := editText(application.ctx, response.Msg.GetYaml(), ".composia-controller-config-*.yaml", 0o600)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	result, err := application.client.controllerConfig.UpdateEditableConfig(application.ctx, newRequest(&controllerv1.UpdateEditableConfigRequest{Yaml: updated, BaseRevision: response.Msg.GetRevision()}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(result.Msg)
	}
	return application.writeKV([][2]string{{"revision", result.Msg.GetRevision()}})
}

func (application *app) printSystemStatus(response *connect.Response[controllerv1.GetSystemStatusResponse]) error {
	message := response.Msg
	now := ""
	if message.GetNow() != nil {
		now = message.GetNow().AsTime().Format(time.RFC3339)
	}
	return application.writeKV([][2]string{
		{"version", message.GetVersion()},
		{"now", now},
		{"configured_node_count", uint64Text(message.GetConfiguredNodeCount())},
		{"online_node_count", uint64Text(message.GetOnlineNodeCount())},
		{"service_count", uint64Text(message.GetServiceCount())},
		{"running_service_count", uint64Text(message.GetRunningServiceCount())},
	})
}

func (application *app) printSystemCapabilities(message *controllerv1.GetCapabilitiesResponse) error {
	global := message.GetGlobal()
	rows := [][]string{
		capabilityRow("backup", global.GetBackup()),
		capabilityRow("dns", global.GetDns()),
		capabilityRow("secrets", global.GetSecrets()),
		capabilityRow("rustic_maintenance", global.GetRusticMaintenance()),
	}
	return application.writeTable([]string{"CAPABILITY", "ENABLED", "REASON"}, rows)
}

func capabilityRow(name string, capability *controllerv1.Capability) []string {
	enabled, reason := capabilityText(capability)
	return []string{name, boolText(enabled), reason}
}
