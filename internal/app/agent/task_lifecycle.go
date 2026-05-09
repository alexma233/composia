package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func executeTaskStep(ctx context.Context, client agentv1connect.AgentReportServiceClient, logUploader *taskLogUploader, taskID string, stepName task.StepName, execute func() error) error {
	stepStartedAt := time.Now()
	startedAt := timestamppb.New(stepStartedAt)
	stepDuration := func() time.Duration {
		return time.Since(stepStartedAt).Round(time.Millisecond)
	}
	logStepFailed := func(err error) {
		log.Printf("agent task step failed: task_id=%s step=%s duration=%s error=%v", taskID, stepName, stepDuration(), err)
	}

	log.Printf("agent task step started: task_id=%s step=%s", taskID, stepName)
	if err := reportTaskStepStateWithTimeout(ctx, client, &agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusRunning), StartedAt: startedAt}, taskReportTimeout); err != nil {
		logStepFailed(err)
		return fmt.Errorf("report running step %s: %w", stepName, err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("step %s started\n", stepName)); err != nil {
		logStepFailed(err)
		return err
	}
	if err := execute(); err != nil {
		finishedAt := timestamppb.Now()
		_ = reportTaskStepStateWithTimeout(ctx, client, &agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusFailed), StartedAt: startedAt, FinishedAt: finishedAt}, taskReportTimeout)
		_ = uploadTaskLog(ctx, logUploader, fmt.Sprintf("step %s failed: %v\n", stepName, err))
		logStepFailed(err)
		return err
	}
	finishedAt := timestamppb.Now()
	if err := reportTaskStepStateWithTimeout(ctx, client, &agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusSucceeded), StartedAt: startedAt, FinishedAt: finishedAt}, taskReportTimeout); err != nil {
		logStepFailed(err)
		return fmt.Errorf("report succeeded step %s: %w", stepName, err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("step %s succeeded\n", stepName)); err != nil {
		logStepFailed(err)
		return err
	}
	log.Printf("agent task step succeeded: task_id=%s step=%s duration=%s", taskID, stepName, stepDuration())
	return nil
}

func reportTaskCompletion(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID string, status task.Status, errorSummary string) error {
	return reportTaskCompletionWithTimeout(ctx, client, taskID, status, errorSummary, taskReportTimeout)
}

func reportTaskCompletionWithTimeout(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID string, status task.Status, errorSummary string, timeout time.Duration) error {
	callCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	_, err := client.ReportTaskState(callCtx, connect.NewRequest(&agentv1.ReportTaskStateRequest{TaskId: taskID, Status: string(status), ErrorSummary: errorSummary, FinishedAt: timestamppb.Now()}))
	if err != nil {
		return fmt.Errorf("report task completion: %w", err)
	}
	return nil
}

func reportTaskStepStateWithTimeout(ctx context.Context, client agentv1connect.AgentReportServiceClient, request *agentv1.ReportTaskStepStateRequest, timeout time.Duration) error {
	callCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	_, err := client.ReportTaskStepState(callCtx, connect.NewRequest(request))
	if err != nil {
		return err
	}
	return nil
}

func failTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID string, err error) error {
	_ = reportTaskCompletion(ctx, client, taskID, task.StatusFailed, err.Error())
	return err
}

func failServiceTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, err error) error {
	_ = reportServiceStatus(ctx, client, cfg, pulledTask.GetServiceName(), store.ServiceRuntimeError)
	return failTask(ctx, client, pulledTask.GetTaskId(), err)
}

func uploadTaskLog(ctx context.Context, logUploader *taskLogUploader, content string) error {
	if logUploader == nil {
		return nil
	}
	if err := logUploader.Upload(ctx, content); err != nil {
		return fmt.Errorf("upload task logs: %w", err)
	}
	return nil
}

func reportBackupResult(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID, serviceName, dataName, artifactRef string, status task.Status, startedAt, finishedAt time.Time, errorSummary string) error {
	_, err := client.ReportBackupResult(ctx, connect.NewRequest(&agentv1.ReportBackupResultRequest{
		BackupId:     fmt.Sprintf("%s-%s", taskID, dataName),
		TaskId:       taskID,
		ServiceName:  serviceName,
		DataName:     dataName,
		Status:       string(status),
		StartedAt:    timestamppb.New(startedAt),
		FinishedAt:   timestamppb.New(finishedAt),
		ArtifactRef:  artifactRef,
		ErrorSummary: errorSummary,
	}))
	if err != nil {
		return fmt.Errorf("report backup result: %w", err)
	}
	return nil
}
