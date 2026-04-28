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
	serviceName := fs.String("service", "", "service name filter")
	status := fs.String("status", "", "status filter")
	dataName := fs.String("data", "", "data entry filter")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia backup list [--service name] [--status status] [--data name]"); err != nil {
		return err
	}
	pageSize, page := pageValues()
	response, err := application.client.backups.ListBackups(application.ctx, newRequest(&controllerv1.ListBackupsRequest{ServiceName: *serviceName, Status: *status, DataName: *dataName, PageSize: pageSize, Page: page}))
	if err != nil {
		return err
	}
	if application.cfg.json {
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
	if err := writeTable(application.out, []string{"BACKUP", "TASK", "SERVICE", "NODE", "DATA", "STATUS", "STARTED", "FINISHED"}, rows); err != nil {
		return err
	}
	_, err = fmt.Fprintf(application.out, "total_count: %d\n", response.Msg.GetTotalCount())
	return err
}

func (application *app) runBackupGet(args []string) error {
	if err := requireArgs(args, 1, "composia backup get <backup>"); err != nil {
		return err
	}
	response, err := application.client.backups.GetBackup(application.ctx, newRequest(&controllerv1.GetBackupRequest{BackupId: args[0]}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	backup := response.Msg
	restoreEnabled := false
	restoreReason := ""
	if backup.GetActions() != nil && backup.GetActions().GetRestore() != nil {
		restoreEnabled = backup.GetActions().GetRestore().GetEnabled()
		restoreReason = backup.GetActions().GetRestore().GetReasonCode()
	}
	return writeKV(application.out, [][2]string{
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia backup restore --node node <backup>"); err != nil {
		return err
	}
	if *nodeID == "" {
		return errorsWithUsage("node is required", "composia backup restore --node node <backup>")
	}
	response, err := application.client.backups.RestoreBackup(application.ctx, newRequest(&controllerv1.RestoreBackupRequest{BackupId: fs.Arg(0), NodeId: *nodeID}))
	if err != nil {
		return err
	}
	return application.printTaskAction(response.Msg)
}
