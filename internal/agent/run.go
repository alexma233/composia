package agent

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"
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
	bundleClient := agentv1connect.NewBundleServiceClient(
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
			if err := pollAndRunTask(ctx, taskClient, bundleClient, reportClient, cfg); err != nil {
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

func pollAndRunTask(ctx context.Context, taskClient agentv1connect.AgentTaskServiceClient, bundleClient agentv1connect.BundleServiceClient, reportClient agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig) error {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	response, err := taskClient.PullNextTask(callCtx, connect.NewRequest(&agentv1.PullNextTaskRequest{NodeId: cfg.NodeID}))
	if err != nil {
		return fmt.Errorf("pull next task: %w", err)
	}
	if !response.Msg.GetHasTask() || response.Msg.GetTask() == nil {
		return nil
	}

	return executePulledTask(ctx, bundleClient, reportClient, cfg, response.Msg.GetTask())
}

func executePulledTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask) error {
	switch pulledTask.GetType() {
	case string(task.TypeDeploy):
		return executeDeployTask(ctx, bundleClient, client, cfg, pulledTask)
	case string(task.TypeUpdate):
		return executeUpdateTask(ctx, bundleClient, client, cfg, pulledTask)
	case string(task.TypeBackup):
		return executeBackupTask(ctx, client, pulledTask)
	case string(task.TypeStop):
		return executeStopTask(ctx, client, cfg, pulledTask)
	case string(task.TypeRestart):
		return executeRestartTask(ctx, client, cfg, pulledTask)
	default:
		return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, fmt.Sprintf("task type %q is not implemented", pulledTask.GetType()))
	}
}

func executeDeployTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask) error {
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), fmt.Sprintf("starting remote deploy task for service=%s node=%s repo_revision=%s\n", pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision())); err != nil {
		return err
	}
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId())
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "render step completed after bundle download\n")
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		projectName, err := loadComposeProjectName(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeUp(ctx, bundle.RootPath, projectName, func(output string) error {
			return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), output)
		})
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepFinalize, func() error {
		return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "finalize step completed after compose up\n")
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeRunning); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "deploy task finished successfully\n"); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeUpdateTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask) error {
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), fmt.Sprintf("starting remote update task for service=%s node=%s repo_revision=%s\n", pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision())); err != nil {
		return err
	}
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId())
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "render step completed after bundle download\n")
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepPull, func() error {
		projectName, err := loadComposeProjectName(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposePull(ctx, bundle.RootPath, projectName, func(output string) error {
			return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), output)
		})
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		projectName, err := loadComposeProjectName(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeUp(ctx, bundle.RootPath, projectName, func(output string) error {
			return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), output)
		})
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepFinalize, func() error {
		return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "finalize step completed after compose pull and up\n")
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeRunning); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "update task finished successfully\n"); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeBackupTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, pulledTask *agentv1.AgentTask) error {
	if len(pulledTask.GetDataNames()) == 0 {
		err := fmt.Errorf("backup task is missing data_names")
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), fmt.Sprintf("starting remote backup task for service=%s data_names=%s\n", pulledTask.GetServiceName(), strings.Join(pulledTask.GetDataNames(), ","))); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepBackup, func() error {
		for _, dataName := range pulledTask.GetDataNames() {
			if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), fmt.Sprintf("backup placeholder completed for %s\n", dataName)); err != nil {
				return err
			}
			if err := reportBackupResult(ctx, client, pulledTask.GetTaskId(), pulledTask.GetServiceName(), dataName); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "backup task finished successfully\n"); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeStopTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask) error {
	if pulledTask.GetServiceDir() == "" {
		err := fmt.Errorf("task is missing service_dir")
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	serviceRoot := filepath.Join(cfg.RepoDir, pulledTask.GetServiceDir())
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), fmt.Sprintf("starting remote stop task for service=%s dir=%s\n", pulledTask.GetServiceName(), serviceRoot)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepComposeDown, func() error {
		projectName, err := loadComposeProjectName(serviceRoot, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeDown(ctx, serviceRoot, projectName, func(output string) error {
			return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), output)
		})
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeStopped); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "stop task finished successfully\n"); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRestartTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask) error {
	if pulledTask.GetServiceDir() == "" {
		err := fmt.Errorf("task is missing service_dir")
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	serviceRoot := filepath.Join(cfg.RepoDir, pulledTask.GetServiceDir())
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), fmt.Sprintf("starting remote restart task for service=%s dir=%s\n", pulledTask.GetServiceName(), serviceRoot)); err != nil {
		return err
	}
	projectName, err := loadComposeProjectName(serviceRoot, pulledTask.GetServiceName())
	if err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepComposeDown, func() error {
		return runComposeDown(ctx, serviceRoot, projectName, func(output string) error {
			return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), output)
		})
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := executeTaskStep(ctx, client, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		return runComposeUp(ctx, serviceRoot, projectName, func(output string) error {
			return uploadTaskLog(ctx, client, pulledTask.GetTaskId(), output)
		})
	}); err != nil {
		_ = reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeError)
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := reportServiceStatus(ctx, client, pulledTask.GetServiceName(), store.ServiceRuntimeRunning); err != nil {
		_ = reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, err.Error())
		return err
	}
	if err := uploadTaskLog(ctx, client, pulledTask.GetTaskId(), "restart task finished successfully\n"); err != nil {
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

func reportBackupResult(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID, serviceName, dataName string) error {
	now := timestamppb.Now()
	_, err := client.ReportBackupResult(ctx, connect.NewRequest(&agentv1.ReportBackupResultRequest{
		BackupId:    fmt.Sprintf("%s-%s", taskID, dataName),
		TaskId:      taskID,
		ServiceName: serviceName,
		DataName:    dataName,
		Status:      string(task.StatusSucceeded),
		StartedAt:   now,
		FinishedAt:  now,
		ArtifactRef: fmt.Sprintf("placeholder:%s:%s", taskID, dataName),
	}))
	if err != nil {
		return fmt.Errorf("report backup result: %w", err)
	}
	return nil
}

func reportServiceStatus(ctx context.Context, client agentv1connect.AgentReportServiceClient, serviceName, runtimeStatus string) error {
	_, err := client.ReportServiceStatus(ctx, connect.NewRequest(&agentv1.ReportServiceStatusRequest{
		ServiceName:   serviceName,
		RuntimeStatus: runtimeStatus,
		ReportedAt:    timestamppb.Now(),
	}))
	if err != nil {
		return fmt.Errorf("report service status: %w", err)
	}
	return nil
}

type bundleResult struct {
	ServiceName  string
	RelativeRoot string
	RootPath     string
}

func downloadServiceBundle(ctx context.Context, client agentv1connect.BundleServiceClient, cfg *config.AgentConfig, taskID string) (*bundleResult, error) {
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: taskID}))
	if err != nil {
		return nil, fmt.Errorf("get service bundle: %w", err)
	}
	defer stream.Close()

	tempFile, err := os.CreateTemp(cfg.StateDir, "bundle-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("create temp bundle file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer tempFile.Close()

	result := &bundleResult{}
	var relativeRoot string
	for stream.Receive() {
		message := stream.Msg()
		if result.ServiceName == "" && message.GetServiceName() != "" {
			result.ServiceName = message.GetServiceName()
		}
		if relativeRoot == "" && message.GetRelativeRoot() != "" {
			relativeRoot = message.GetRelativeRoot()
			result.RelativeRoot = relativeRoot
		}
		if _, err := tempFile.Write(message.GetData()); err != nil {
			return nil, fmt.Errorf("write temp bundle file: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("receive service bundle: %w", err)
	}
	if relativeRoot == "" {
		return nil, fmt.Errorf("bundle stream did not include relative_root metadata")
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp bundle file: %w", err)
	}

	targetRoot := filepath.Join(cfg.RepoDir, relativeRoot)
	if err := os.RemoveAll(targetRoot); err != nil {
		return nil, fmt.Errorf("remove old bundle target %q: %w", targetRoot, err)
	}
	if err := extractTarGz(tempPath, cfg.RepoDir); err != nil {
		return nil, err
	}
	result.RootPath = targetRoot
	return result, nil
}

func extractTarGz(archivePath, destinationDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive %q: %w", archivePath, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip archive %q: %w", archivePath, err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read tar archive %q: %w", archivePath, err)
		}

		targetPath := filepath.Join(destinationDir, header.Name)
		cleanTargetPath := filepath.Clean(targetPath)
		cleanDestinationDir := filepath.Clean(destinationDir) + string(os.PathSeparator)
		if !strings.HasPrefix(cleanTargetPath, cleanDestinationDir) && cleanTargetPath != filepath.Clean(destinationDir) {
			return fmt.Errorf("bundle entry %q escapes destination root", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTargetPath, 0o755); err != nil {
				return fmt.Errorf("create bundle directory %q: %w", cleanTargetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanTargetPath), 0o755); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", cleanTargetPath, err)
			}
			outFile, err := os.OpenFile(cleanTargetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create bundle file %q: %w", cleanTargetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("write bundle file %q: %w", cleanTargetPath, err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("close bundle file %q: %w", cleanTargetPath, err)
			}
		default:
			return fmt.Errorf("unsupported tar entry type %d for %q", header.Typeflag, header.Name)
		}
	}
}

func loadComposeProjectName(serviceDir, fallback string) (string, error) {
	metaPath := filepath.Join(serviceDir, "composia-meta.yaml")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		return "", fmt.Errorf("read service meta %q: %w", metaPath, err)
	}
	var meta struct {
		ProjectName string `yaml:"project_name"`
	}
	if err := yaml.Unmarshal(content, &meta); err != nil {
		return "", fmt.Errorf("decode service meta %q: %w", metaPath, err)
	}
	if meta.ProjectName != "" {
		return meta.ProjectName, nil
	}
	return fallback, nil
}

func runComposeUp(ctx context.Context, serviceDir, projectName string, uploadLog func(string) error) error {
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "up", "-d")
	command.Dir = serviceDir
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadLog(string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

func runComposeDown(ctx context.Context, serviceDir, projectName string, uploadLog func(string) error) error {
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "down")
	command.Dir = serviceDir
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadLog(string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	return nil
}

func runComposePull(ctx context.Context, serviceDir, projectName string, uploadLog func(string) error) error {
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "pull")
	command.Dir = serviceDir
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadLog(string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker compose pull failed: %w", err)
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
