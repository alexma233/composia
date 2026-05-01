package cli

import (
	"fmt"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runTask(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia task <list|get|logs|wait|run-again|approve|reject>")
	}
	switch args[0] {
	case "list":
		return application.runTaskList(args[1:])
	case "get":
		return application.runTaskGet(args[1:])
	case "logs":
		return application.runTaskLogs(args[1:])
	case "wait":
		return application.runTaskWait(args[1:])
	case "run-again":
		return application.runTaskAgain(args[1:])
	case "approve", "reject":
		return application.runTaskResolve(args[0], args[1:])
	default:
		return fmt.Errorf("unknown task command %q", args[0])
	}
}

func (application *app) runTaskList(args []string) error {
	fs := newCommandFlagSet("task list")
	var statuses stringListFlag
	var services stringListFlag
	var nodes stringListFlag
	var types stringListFlag
	var excludeStatuses stringListFlag
	var excludeServices stringListFlag
	var excludeNodes stringListFlag
	var excludeTypes stringListFlag
	fs.Var(&statuses, "status", "status filter; repeat or comma-separate")
	fs.Var(&services, "service", "service name filter; repeat or comma-separate")
	fs.Var(&nodes, "node", "node ID filter; repeat or comma-separate")
	fs.Var(&types, "type", "task type filter; repeat or comma-separate")
	fs.Var(&excludeStatuses, "exclude-status", "status exclusion; repeat or comma-separate")
	fs.Var(&excludeServices, "exclude-service", "service exclusion; repeat or comma-separate")
	fs.Var(&excludeNodes, "exclude-node", "node exclusion; repeat or comma-separate")
	fs.Var(&excludeTypes, "exclude-type", "type exclusion; repeat or comma-separate")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia task list [filters]"); err != nil {
		return err
	}
	pageSize, page := pageValues()
	response, err := application.client.tasks.ListTasks(application.ctx, newRequest(&controllerv1.ListTasksRequest{
		Status:             []string(statuses),
		ServiceName:        []string(services),
		NodeId:             []string(nodes),
		Type:               []string(types),
		ExcludeStatus:      []string(excludeStatuses),
		ExcludeServiceName: []string(excludeServices),
		ExcludeNodeId:      []string(excludeNodes),
		ExcludeType:        []string(excludeTypes),
		PageSize:           pageSize,
		Page:               page,
	}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetTasks()))
	for _, task := range response.Msg.GetTasks() {
		rows = append(rows, []string{
			task.GetTaskId(),
			task.GetType(),
			task.GetStatus(),
			task.GetServiceName(),
			task.GetNodeId(),
			task.GetCreatedAt(),
		})
	}
	if err := writeTable(application.out, []string{"TASK", "TYPE", "STATUS", "SERVICE", "NODE", "CREATED"}, rows); err != nil {
		return err
	}
	_, err = fmt.Fprintf(application.out, "total_count: %d\n", response.Msg.GetTotalCount())
	return err
}

func (application *app) runTaskGet(args []string) error {
	if err := requireArgs(args, 1, "composia task get <task>"); err != nil {
		return err
	}
	response, err := application.client.tasks.GetTask(application.ctx, newRequest(&controllerv1.GetTaskRequest{TaskId: args[0]}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	return application.printTaskDetail(response.Msg)
}

func (application *app) runTaskLogs(args []string) error {
	if err := requireArgs(args, 1, "composia task logs <task>"); err != nil {
		return err
	}
	return application.streamTaskLogs(application.ctx, args[0])
}

func (application *app) runTaskAgain(args []string) error {
	fs := newCommandFlagSet("task run-again")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia task run-again [--wait] [--follow] [--timeout duration] <task>"); err != nil {
		return err
	}
	response, err := application.client.tasks.RunTaskAgain(application.ctx, newRequest(&controllerv1.RunTaskAgainRequest{TaskId: fs.Arg(0)}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runTaskWait(args []string) error {
	fs := newCommandFlagSet("task wait")
	waitOptions := addWaitFlags(fs)
	*waitOptions.wait = true
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia task wait [--follow] [--timeout duration] [--interval duration] <task>"); err != nil {
		return err
	}
	return application.waitTask(fs.Arg(0), waitOptions)
}

func (application *app) runTaskResolve(decision string, args []string) error {
	fs := newCommandFlagSet("task " + decision)
	comment := fs.String("comment", "", "operator comment")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, fmt.Sprintf("composia task %s [--comment text] <task>", decision)); err != nil {
		return err
	}
	response, err := application.client.tasks.ResolveTaskConfirmation(application.ctx, newRequest(&controllerv1.ResolveTaskConfirmationRequest{TaskId: fs.Arg(0), Decision: decision, Comment: *comment}))
	if err != nil {
		return err
	}
	return application.printTaskAction(response.Msg)
}

func (application *app) printTaskDetail(task *controllerv1.GetTaskResponse) error {
	if err := writeKV(application.out, [][2]string{
		{"task_id", task.GetTaskId()},
		{"type", task.GetType()},
		{"source", task.GetSource()},
		{"service_name", task.GetServiceName()},
		{"node_id", task.GetNodeId()},
		{"status", task.GetStatus()},
		{"created_at", task.GetCreatedAt()},
		{"started_at", task.GetStartedAt()},
		{"finished_at", task.GetFinishedAt()},
		{"repo_revision", task.GetRepoRevision()},
		{"result_revision", task.GetResultRevision()},
		{"attempt_of_task_id", task.GetAttemptOfTaskId()},
		{"triggered_by", task.GetTriggeredBy()},
		{"log_path", task.GetLogPath()},
		{"error_summary", task.GetErrorSummary()},
	}); err != nil {
		return err
	}
	steps := task.GetSteps()
	if len(steps) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(application.out); err != nil {
		return err
	}
	rows := make([][]string, 0, len(steps))
	for _, step := range steps {
		rows = append(rows, []string{step.GetStepName(), step.GetStatus(), step.GetStartedAt(), step.GetFinishedAt()})
	}
	return writeTable(application.out, []string{"STEP", "STATUS", "STARTED", "FINISHED"}, rows)
}
