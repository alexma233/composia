package agent

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	heartbeatInterval = 15 * time.Second
	taskPollInterval  = 1 * time.Second
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.LoadAgent(configPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return fmt.Errorf("create agent state_dir %q: %w", cfg.StateDir, err)
	}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		return fmt.Errorf("create agent repo_dir %q: %w", cfg.RepoDir, err)
	}
	if err := os.MkdirAll(cfg.CaddyGeneratedDir(), 0o755); err != nil {
		return fmt.Errorf("create agent caddy.generated_dir %q: %w", cfg.CaddyGeneratedDir(), err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	reportClient := agentv1connect.NewAgentReportServiceClient(
		httpClient,
		cfg.ControllerAddr,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(cfg.Token)),
	)
	taskClient := agentv1connect.NewAgentTaskServiceClient(
		httpClient,
		cfg.ControllerAddr,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(cfg.Token)),
	)

	log.Printf("composia agent loops started: node_id=%s controller=%s", cfg.NodeID, cfg.ControllerAddr)
	if err := sendHeartbeat(ctx, reportClient, cfg); err != nil {
		log.Printf("initial heartbeat failed: %v", err)
	}

	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()
	taskTicker := time.NewTicker(taskPollInterval)
	defer taskTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeatTicker.C:
			if err := sendHeartbeat(ctx, reportClient, cfg); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
		case <-taskTicker.C:
			if err := pollAndRunTask(ctx, taskClient, reportClient, cfg); err != nil {
				log.Printf("task poll failed: %v", err)
			}
		}
	}
}

func sendHeartbeat(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig) error {
	runtime, err := collectRuntimeSummary(cfg.StateDir)
	if err != nil {
		return err
	}

	request := &agentv1.HeartbeatRequest{
		NodeId:       cfg.NodeID,
		AgentVersion: version.Value,
		SentAt:       timestamppb.Now(),
		Runtime:      runtime,
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = client.Heartbeat(callCtx, connect.NewRequest(request))
	if err != nil {
		return fmt.Errorf("send heartbeat: %w", err)
	}
	return nil
}

func pollAndRunTask(ctx context.Context, taskClient agentv1connect.AgentTaskServiceClient, reportClient agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig) error {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	response, err := taskClient.PullNextTask(callCtx, connect.NewRequest(&agentv1.PullNextTaskRequest{NodeId: cfg.NodeID}))
	if err != nil {
		return fmt.Errorf("pull next task: %w", err)
	}
	if !response.Msg.GetHasTask() || response.Msg.GetTask() == nil {
		return nil
	}

	return executePulledTask(ctx, reportClient, response.Msg.GetTask())
}

func executePulledTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, pulledTask *agentv1.AgentTask) error {
	switch pulledTask.GetType() {
	case string(task.TypeDeploy):
		return executeDeployTask(ctx, client, pulledTask)
	default:
		return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, fmt.Sprintf("task type %q is not implemented", pulledTask.GetType()))
	}
}

func executeDeployTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, pulledTask *agentv1.AgentTask) error {
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), fmt.Sprintf("starting remote deploy task for service=%s node=%s repo_revision=%s\n", pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision())); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepRender, func() error {
		return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "render step completed with remote placeholder executor\n")
	}); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepFinalize, func() error {
		return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "finalize step completed with remote placeholder executor\n")
	}); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "deploy task finished successfully\n"); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeTaskStep(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID string, stepName task.StepName, execute func() error) error {
	startedAt := timestamppb.Now()
	if _, err := client.ReportTaskStepState(ctx, connect.NewRequest(&agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusRunning), StartedAt: startedAt})); err != nil {
		return fmt.Errorf("report running step %s: %w", stepName, err)
	}
	if err := uploadTaskLog(ctx, client, taskID, fmt.Sprintf("step %s started\n", stepName)); err != nil {
		return err
	}
	if err := execute(); err != nil {
		finishedAt := timestamppb.Now()
		_, _ = client.ReportTaskStepState(ctx, connect.NewRequest(&agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusFailed), StartedAt: startedAt, FinishedAt: finishedAt}))
		_, _ = client.UploadTaskLogs(ctx, connect.NewRequest(&agentv1.UploadTaskLogsRequest{TaskId: taskID, Content: fmt.Sprintf("step %s failed: %v\n", stepName, err)}))
		return err
	}
	finishedAt := timestamppb.Now()
	if _, err := client.ReportTaskStepState(ctx, connect.NewRequest(&agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusSucceeded), StartedAt: startedAt, FinishedAt: finishedAt})); err != nil {
		return fmt.Errorf("report succeeded step %s: %w", stepName, err)
	}
	return uploadTaskLog(ctx, client, taskID, fmt.Sprintf("step %s succeeded\n", stepName))
}

func reportTaskCompletion(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID string, status task.Status, errorSummary string) error {
	_, err := client.ReportTaskState(ctx, connect.NewRequest(&agentv1.ReportTaskStateRequest{TaskId: taskID, Status: string(status), ErrorSummary: errorSummary, FinishedAt: timestamppb.Now()}))
	if err != nil {
		return fmt.Errorf("report task completion: %w", err)
	}
	return nil
}

func uploadTaskLog(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID, content string) error {
	_, err := client.UploadTaskLogs(ctx, connect.NewRequest(&agentv1.UploadTaskLogsRequest{TaskId: taskID, Content: content}))
	if err != nil {
		return fmt.Errorf("upload task logs: %w", err)
	}
	return nil
}

func collectRuntimeSummary(path string) (*agentv1.NodeRuntimeSummary, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("read filesystem stats for %q: %w", path, err)
	}

	blockSize := uint64(stat.Bsize)
	dockerVersion := dockerServerVersion()

	return &agentv1.NodeRuntimeSummary{
		DockerServerVersion: dockerVersion,
		DiskTotalBytes:      stat.Blocks * blockSize,
		DiskFreeBytes:       stat.Bavail * blockSize,
	}, nil
}

func dockerServerVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return ""
	}
	return string(bytesTrimSpace(output))
}

func bytesTrimSpace(value []byte) []byte {
	start := 0
	for start < len(value) && (value[start] == ' ' || value[start] == '\n' || value[start] == '\t' || value[start] == '\r') {
		start++
	}

	end := len(value)
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\t' || value[end-1] == '\r') {
		end--
	}
	return value[start:end]
}
