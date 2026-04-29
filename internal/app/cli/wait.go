package cli

import (
	"context"
	"flag"
	"fmt"
	"time"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

type waitOptions struct {
	wait     *bool
	follow   *bool
	timeout  *time.Duration
	interval *time.Duration
}

func addWaitFlags(fs *flag.FlagSet) waitOptions {
	wait := fs.Bool("wait", false, "wait for the task to finish")
	follow := fs.Bool("follow", false, "stream task logs while waiting")
	timeout := fs.Duration("timeout", 0, "maximum wait duration")
	interval := fs.Duration("interval", time.Second, "poll interval")
	return waitOptions{wait: wait, follow: follow, timeout: timeout, interval: interval}
}

func (options waitOptions) shouldWait() bool {
	return (options.wait != nil && *options.wait) || (options.follow != nil && *options.follow)
}

func (application *app) printTaskActionWithWait(response *controllerv1.TaskActionResponse, options waitOptions) error {
	if err := application.printTaskAction(response); err != nil {
		return err
	}
	if !options.shouldWait() {
		return nil
	}
	return application.waitTask(response.GetTaskId(), options)
}

func (application *app) waitTask(taskID string, options waitOptions) error {
	if taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	ctx := application.ctx
	if options.timeout != nil && *options.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *options.timeout)
		defer cancel()
	}
	if options.follow != nil && *options.follow {
		if err := application.streamTaskLogs(ctx, taskID); err != nil {
			return err
		}
	}
	interval := time.Second
	if options.interval != nil && *options.interval > 0 {
		interval = *options.interval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		response, err := application.client.tasks.GetTask(ctx, newRequest(&controllerv1.GetTaskRequest{TaskId: taskID}))
		if err != nil {
			return err
		}
		status := response.Msg.GetStatus()
		if isTerminalStatus(status) {
			if !application.cfg.json {
				if err := writeKV(application.out, [][2]string{{"final_status", status}}); err != nil {
					return err
				}
			}
			if status != string(task.StatusSucceeded) {
				return fmt.Errorf("task %s finished with status %s", taskID, status)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (application *app) streamTaskLogs(ctx context.Context, taskID string) error {
	stream, err := application.client.tasks.TailTaskLogs(ctx, newRequest(&controllerv1.TailTaskLogsRequest{TaskId: taskID}))
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

func isTerminalStatus(status string) bool {
	switch status {
	case string(task.StatusSucceeded), string(task.StatusFailed), string(task.StatusCancelled):
		return true
	default:
		return false
	}
}
