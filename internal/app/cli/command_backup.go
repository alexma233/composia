package cli

import (
	"fmt"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runBackup(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia backup <list|get|restore>")
	}
	switch args[0] {
	case "list":
		return application.runBackupList(args[1:])
	case "get":
		return application.runBackupGet(args[1:])
	case "restore":
		return application.runBackupRestore(args[1:])
	default:
		return fmt.Errorf("unknown backup command %q", args[0])
	}
}

func (application *app) runBackupList(args []string) error {
	fs := newCommandFlagSet("backup list")
	var services stringListFlag
	var statuses stringListFlag
	var dataNames stringListFlag
	var nodes stringListFlag
	var excludeServices stringListFlag
	var excludeStatuses stringListFlag
	var excludeDataNames stringListFlag
	var excludeNodes stringListFlag
	fs.Var(&services, "service", "service name filter; repeat or comma-separate")
	fs.Var(&statuses, "status", "status filter; repeat or comma-separate")
	fs.Var(&dataNames, "data", "data entry filter; repeat or comma-separate")
	fs.Var(&nodes, "node", "node ID filter; repeat or comma-separate")
	fs.Var(&excludeServices, "exclude-service", "service exclusion; repeat or comma-separate")
	fs.Var(&excludeStatuses, "exclude-status", "status exclusion; repeat or comma-separate")
	fs.Var(&excludeDataNames, "exclude-data", "data entry exclusion; repeat or comma-separate")
	fs.Var(&excludeNodes, "exclude-node", "node exclusion; repeat or comma-separate")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia backup list [filters]"); err != nil {
		return err
	}
	pageSize, page := pageValues()
	response, err := application.client.backups.ListBackups(application.ctx, newRequest(&controllerv1.ListBackupsRequest{
		ServiceName:        []string(services),
		Status:             []string(statuses),
		DataName:           []string(dataNames),
		NodeId:             []string(nodes),
		ExcludeServiceName: []string(excludeServices),
		ExcludeStatus:      []string(excludeStatuses),
		ExcludeDataName:    []string(excludeDataNames),
		ExcludeNodeId:      []string(excludeNodes),
		PageSize:           pageSize,
		Page:               page,
	}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetBackups()))
	for _, backup := range response.Msg.GetBackups() {
		rows = append(rows, []string{
			backup.GetBackupId(),
			backup.GetTaskId(),
			backup.GetServiceName(),
			backup.GetNodeId(),
			backup.GetDataName(),
			backup.GetStatus(),
			backup.GetStartedAt(),
			backup.GetFinishedAt(),
		})
	}
	if err := application.writeTable([]string{"BACKUP", "TASK", "SERVICE", "NODE", "DATA", "STATUS", "STARTED", "FINISHED"}, rows); err != nil {
		return err
	}
	return application.writeCount("total_count", response.Msg.GetTotalCount())
}

func (application *app) runBackupGet(args []string) error {
	if err := requireArgs(args, 1, "composia backup get <backup>"); err != nil {
		return err
	}
	response, err := application.client.backups.GetBackup(application.ctx, newRequest(&controllerv1.GetBackupRequest{BackupId: args[0]}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	backup := response.Msg
	restoreEnabled := false
	restoreReason := ""
	if backup.GetActions() != nil && backup.GetActions().GetRestore() != nil {
		restoreEnabled = backup.GetActions().GetRestore().GetEnabled()
		restoreReason = backup.GetActions().GetRestore().GetReasonCode()
	}
	return application.writeKV([][2]string{
		{"backup_id", backup.GetBackupId()},
		{"task_id", backup.GetTaskId()},
		{"service_name", backup.GetServiceName()},
		{"node_id", backup.GetNodeId()},
		{"data_name", backup.GetDataName()},
		{"status", backup.GetStatus()},
		{"started_at", backup.GetStartedAt()},
		{"finished_at", backup.GetFinishedAt()},
		{"artifact_ref", backup.GetArtifactRef()},
		{"error_summary", backup.GetErrorSummary()},
		{"restore_enabled", boolText(restoreEnabled)},
		{"restore_reason", restoreReason},
	})
}

func (application *app) runBackupRestore(args []string) error {
	fs := newCommandFlagSet("backup restore")
	nodeID := fs.String("node", "", "destination node ID")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia backup restore [--wait] [--follow] [--timeout duration] --node node <backup>"); err != nil {
		return err
	}
	if *nodeID == "" {
		return errorsWithUsage("node is required", "composia backup restore [--wait] [--follow] [--timeout duration] --node node <backup>")
	}
	response, err := application.client.backups.RestoreBackup(application.ctx, newRequest(&controllerv1.RestoreBackupRequest{BackupId: fs.Arg(0), NodeId: *nodeID}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}
