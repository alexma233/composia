package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	backupcfg "forgejo.alexma.top/alexma233/composia/internal/backup"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"golang.org/x/net/http2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"
)

const (
	heartbeatInterval      = 15 * time.Second
	heartbeatTimeout       = 10 * time.Second
	pullNextTaskTimeout    = 30 * time.Second
	taskReportTimeout      = 10 * time.Second
	taskExecutionTimeout   = 6 * time.Hour
	taskRetryAfterPollFail = 1 * time.Second
	dockerVolumeTarImage   = "alpine:3.20"
	dockerVolumeImportCmd  = "rm -rf /target/..?* /target/.[!.]* /target/* && tar -C /target -xf -"
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.LoadAgent(configPath)
	if err != nil {
		return err
	}

	if err := ensureAgentDirs(cfg); err != nil {
		return err
	}
	if strings.HasPrefix(strings.ToLower(cfg.ControllerAddr), "http://") {
		log.Printf("warning: agent.controller_addr uses plain HTTP (%s); only use this behind a trusted reverse proxy or on a trusted local network", cfg.ControllerAddr)
	}

	httpClient := controllerHTTPClient(cfg.ControllerAddr)
	clientOptions := controllerClientOptions(cfg)
	reportClient := agentv1connect.NewAgentReportServiceClient(
		httpClient,
		cfg.ControllerAddr,
		clientOptions...,
	)
	taskClient := agentv1connect.NewAgentTaskServiceClient(
		httpClient,
		cfg.ControllerAddr,
		clientOptions...,
	)
	bundleClient := agentv1connect.NewBundleServiceClient(
		httpClient,
		cfg.ControllerAddr,
		clientOptions...,
	)

	log.Printf("composia agent loops started: node_id=%s controller=%s", cfg.NodeID, cfg.ControllerAddr)
	startPeriodicTask(ctx, heartbeatInterval, "initial heartbeat", "heartbeat", func() error {
		return sendHeartbeat(ctx, reportClient, cfg)
	})
	startPeriodicTask(ctx, 5*time.Minute, "initial docker stats report", "docker stats report", func() error {
		return reportDockerStats(ctx, reportClient, cfg)
	})

	startExecTunnelLoop(ctx, reportClient, cfg.NodeID)
	startContainerLogTunnelLoop(ctx, reportClient, cfg.NodeID)

	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			if err := pollAndRunTask(ctx, taskClient, bundleClient, reportClient, cfg); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("task loop failed: %v", err)
				if !sleepWithContext(ctx, taskRetryAfterPollFail) {
					return
				}
			}
		}
	}()

	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			if err := pollAndRunDockerQuery(ctx, taskClient, reportClient, cfg); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("docker query poll failed: %v", err)
				if !sleepWithContext(ctx, taskRetryAfterPollFail) {
					return
				}
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func controllerClientOptions(cfg *config.AgentConfig) []connect.ClientOption {
	options := []connect.ClientOption{connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(cfg.Token))}
	if cfg.ControllerGRPC {
		options = append([]connect.ClientOption{connect.WithGRPC()}, options...)
	}
	return options
}

func ensureAgentDirs(cfg *config.AgentConfig) error {
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return fmt.Errorf("create agent state_dir %q: %w", cfg.StateDir, err)
	}
	if err := os.MkdirAll(dataProtectStageRoot(cfg.StateDir), 0o755); err != nil {
		return fmt.Errorf("create agent data-protect dir %q: %w", dataProtectStageRoot(cfg.StateDir), err)
	}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		return fmt.Errorf("create agent repo_dir %q: %w", cfg.RepoDir, err)
	}
	if err := os.MkdirAll(cfg.CaddyGeneratedDir(), 0o755); err != nil {
		return fmt.Errorf("create agent caddy.generated_dir %q: %w", cfg.CaddyGeneratedDir(), err)
	}
	return nil
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

	callCtx, cancel := context.WithTimeout(ctx, heartbeatTimeout)
	defer cancel()

	_, err = client.Heartbeat(callCtx, connect.NewRequest(request))
	if err != nil {
		return fmt.Errorf("send heartbeat: %w", err)
	}
	return nil
}

func startPeriodicTask(ctx context.Context, interval time.Duration, initialLabel, repeatLabel string, run func() error) {
	go func() {
		if err := run(); err != nil {
			log.Printf("%s failed: %v", initialLabel, err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := run(); err != nil {
					log.Printf("%s failed: %v", repeatLabel, err)
				}
			}
		}
	}()
}

func reportDockerStats(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig) error {
	stats, err := collectDockerStats()
	if err != nil {
		return err
	}

	request := &agentv1.ReportDockerStatsRequest{
		NodeId: cfg.NodeID,
		Stats:  stats,
	}

	callCtx, cancel := context.WithTimeout(ctx, heartbeatTimeout)
	defer cancel()

	_, err = client.ReportDockerStats(callCtx, connect.NewRequest(request))
	if err != nil {
		return fmt.Errorf("report docker stats: %w", err)
	}
	return nil
}

func pollAndRunTask(ctx context.Context, taskClient agentv1connect.AgentTaskServiceClient, bundleClient agentv1connect.BundleServiceClient, reportClient agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig) error {
	callCtx, cancel := context.WithTimeout(ctx, pullNextTaskTimeout)
	defer cancel()

	response, err := taskClient.PullNextTask(callCtx, connect.NewRequest(&agentv1.PullNextTaskRequest{NodeId: cfg.NodeID}))
	if err != nil {
		return fmt.Errorf("pull next task: %w", err)
	}
	if !response.Msg.GetHasTask() || response.Msg.GetTask() == nil {
		return nil
	}

	pulledTask := response.Msg.GetTask()
	startedAt := time.Now()
	log.Printf("agent accepted task: task_id=%s type=%s service=%s node=%s repo_revision=%s", pulledTask.GetTaskId(), pulledTask.GetType(), pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision())
	err = executePulledTaskWithTimeout(ctx, bundleClient, reportClient, cfg, pulledTask, taskExecutionTimeout)
	duration := time.Since(startedAt).Round(time.Millisecond)
	if err != nil {
		log.Printf("agent task failed: task_id=%s type=%s service=%s node=%s duration=%s error=%v", pulledTask.GetTaskId(), pulledTask.GetType(), pulledTask.GetServiceName(), pulledTask.GetNodeId(), duration, err)
		return err
	}
	log.Printf("agent task finished: task_id=%s type=%s service=%s node=%s duration=%s", pulledTask.GetTaskId(), pulledTask.GetType(), pulledTask.GetServiceName(), pulledTask.GetNodeId(), duration)
	return nil
}

func executePulledTaskWithTimeout(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, timeout time.Duration) error {
	if timeout <= 0 {
		return executePulledTask(ctx, bundleClient, client, cfg, pulledTask)
	}

	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := executePulledTask(taskCtx, bundleClient, client, cfg, pulledTask)
	if err == nil {
		return nil
	}
	if ctx.Err() != nil && !errors.Is(taskCtx.Err(), context.DeadlineExceeded) {
		return err
	}

	taskTimedOut := errors.Is(taskCtx.Err(), context.DeadlineExceeded)
	failureSummary := err.Error()
	if taskTimedOut {
		failureSummary = fmt.Sprintf("task exceeded execution timeout of %s", timeout)
	}
	reportCtx, reportCancel := context.WithTimeout(context.Background(), taskReportTimeout)
	defer reportCancel()
	if reportErr := reportTaskCompletion(reportCtx, client, pulledTask.GetTaskId(), task.StatusFailed, failureSummary); reportErr != nil {
		return fmt.Errorf("%s (report failed: %v)", err, reportErr)
	}
	if taskTimedOut {
		return fmt.Errorf("%s: %w", failureSummary, err)
	}
	return err
}

func pollAndRunDockerQuery(ctx context.Context, taskClient agentv1connect.AgentTaskServiceClient, reportClient agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig) error {
	callCtx, cancel := context.WithTimeout(ctx, pullNextTaskTimeout)
	defer cancel()

	response, err := taskClient.PullNextDockerQuery(callCtx, connect.NewRequest(&agentv1.PullNextDockerQueryRequest{NodeId: cfg.NodeID}))
	if err != nil {
		return fmt.Errorf("pull next docker query: %w", err)
	}
	if !response.Msg.GetHasQuery() || response.Msg.GetQuery() == nil {
		return nil
	}

	query := response.Msg.GetQuery()
	queryCtx, queryCancel := context.WithTimeout(ctx, pullNextTaskTimeout)
	defer queryCancel()

	result, queryErr := executeDockerQuery(queryCtx, query)
	reportRequest := &agentv1.ReportDockerQueryResultRequest{
		QueryId: query.GetQueryId(),
		NodeId:  cfg.NodeID,
	}
	if queryErr != nil {
		reportRequest.ErrorMessage = queryErr.Error()
		reportRequest.ErrorCode = dockerQueryErrorCode(queryErr)
	} else {
		payloadJSON, err := json.Marshal(result)
		if err != nil {
			reportRequest.ErrorMessage = fmt.Sprintf("marshal docker query result: %v", err)
			reportRequest.ErrorCode = "internal"
		} else {
			reportRequest.PayloadJson = string(payloadJSON)
		}
	}

	reportCtx, reportCancel := context.WithTimeout(ctx, heartbeatTimeout)
	defer reportCancel()
	if _, err := reportClient.ReportDockerQueryResult(reportCtx, connect.NewRequest(reportRequest)); err != nil {
		return fmt.Errorf("report docker query result: %w", err)
	}
	return nil
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func executePulledTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask) error {
	logUploader := newTaskLogUploader(client, pulledTask.GetTaskId())
	defer func() {
		if err := logUploader.Close(); err != nil {
			log.Printf("close task log uploader: %v", err)
		}
	}()

	switch pulledTask.GetType() {
	case string(task.TypeDeploy):
		return executeDeployTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeUpdate):
		return executeUpdateTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeBackup):
		return executeBackupTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeRestore):
		return executeRestoreTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeStop):
		return executeStopTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeRestart):
		return executeRestartTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypePrune):
		return executePruneTask(ctx, client, cfg, pulledTask, logUploader)
	case string(task.TypeRusticInit):
		return executeRusticInitTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeRusticForget):
		return executeRusticForgetTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeRusticPrune):
		return executeRusticPruneTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeCaddySync):
		return executeCaddySyncTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	case string(task.TypeCaddyReload):
		return executeCaddyReloadTask(ctx, client, cfg, pulledTask, logUploader)
	case string(task.TypeDockerStart), string(task.TypeDockerStop), string(task.TypeDockerRestart), string(task.TypeDockerRemove):
		return executeDockerTask(ctx, client, cfg, pulledTask, logUploader)
	default:
		return failTask(ctx, client, pulledTask.GetTaskId(), fmt.Errorf("task type %q is not implemented", pulledTask.GetType()))
	}
}

func executeDeployTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote deploy task for service=%s node=%s repo_revision=%s\n", pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision())); err != nil {
		return err
	}
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		projectName, err := loadComposeProjectName(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeUp(ctx, bundle.RootPath, projectName, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeCaddySyncStep(ctx, client, cfg, pulledTask, logUploader, bundle.RootPath); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepFinalize, func() error {
		return uploadTaskLog(ctx, logUploader, "finalize step completed after compose up\n")
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := reportServiceStatus(ctx, client, cfg, pulledTask.GetServiceName(), store.ServiceRuntimeRunning); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "deploy task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeUpdateTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote update task for service=%s node=%s repo_revision=%s\n", pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision())); err != nil {
		return err
	}
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepPull, func() error {
		projectName, err := loadComposeProjectName(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposePull(ctx, bundle.RootPath, projectName, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		projectName, err := loadComposeProjectName(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeUp(ctx, bundle.RootPath, projectName, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeCaddySyncStep(ctx, client, cfg, pulledTask, logUploader, bundle.RootPath); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepFinalize, func() error {
		return uploadTaskLog(ctx, logUploader, "finalize step completed after compose pull and up\n")
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := reportServiceStatus(ctx, client, cfg, pulledTask.GetServiceName(), store.ServiceRuntimeRunning); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "update task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeBackupTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	if len(pulledTask.GetDataNames()) == 0 {
		err := fmt.Errorf("backup task is missing data_names")
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	var bundle *bundleResult
	var rusticBundle *bundleResult
	var runtimeConfig *backupcfg.RuntimeConfig
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		runtimeConfig, err = loadBackupRuntimeConfig(bundle.RootPath)
		if err != nil {
			return err
		}
		if runtimeConfig.Rustic.ServiceDir == bundle.RelativeRoot {
			rusticBundle = bundle
		} else {
			rusticBundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), runtimeConfig.Rustic.ServiceDir)
			if err != nil {
				return err
			}
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote backup task for service=%s data_names=%s\n", pulledTask.GetServiceName(), strings.Join(pulledTask.GetDataNames(), ","))); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepBackup, func() error {
		for _, item := range runtimeConfig.Items {
			artifactRef, startedAt, finishedAt, err := backupRuntimeItem(ctx, cfg, bundle.RootPath, rusticBundle.RootPath, pulledTask.GetTaskId(), item, runtimeConfig.Rustic, logUploader)
			if err != nil {
				_ = reportBackupResult(ctx, client, pulledTask.GetTaskId(), pulledTask.GetServiceName(), item.Name, "", task.StatusFailed, startedAt, time.Now().UTC(), err.Error())
				return err
			}
			if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("backup completed for %s artifact=%s\n", item.Name, artifactRef)); err != nil {
				return err
			}
			if err := reportBackupResult(ctx, client, pulledTask.GetTaskId(), pulledTask.GetServiceName(), item.Name, artifactRef, task.StatusSucceeded, startedAt, finishedAt, ""); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "backup task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRestoreTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	var bundle *bundleResult
	var rusticBundle *bundleResult
	var runtimeConfig *backupcfg.RestoreConfig
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		runtimeConfig, err = loadRestoreRuntimeConfig(bundle.RootPath)
		if err != nil {
			return err
		}
		if runtimeConfig.Rustic.ServiceDir == bundle.RelativeRoot {
			rusticBundle = bundle
		} else {
			rusticBundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), runtimeConfig.Rustic.ServiceDir)
			if err != nil {
				return err
			}
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote restore task for service=%s node=%s\n", pulledTask.GetServiceName(), pulledTask.GetNodeId())); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRestore, func() error {
		for _, item := range runtimeConfig.Items {
			if err := restoreRuntimeItem(ctx, cfg, serviceRoot, rusticBundle.RootPath, pulledTask.GetTaskId(), item, runtimeConfig.Rustic, logUploader); err != nil {
				return err
			}
			if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("restore completed for %s\n", item.Name)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "restore task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeStopTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote stop task for service=%s dir=%s\n", pulledTask.GetServiceName(), serviceRoot)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeDown, func() error {
		projectName, err := loadComposeProjectName(serviceRoot, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeDown(ctx, serviceRoot, projectName, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepCaddySync, func() error {
		return removeServiceCaddyFile(ctx, cfg, pulledTask.GetServiceDir(), func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := reportServiceStatus(ctx, client, cfg, pulledTask.GetServiceName(), store.ServiceRuntimeStopped); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "stop task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRestartTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote restart task for service=%s dir=%s\n", pulledTask.GetServiceName(), serviceRoot)); err != nil {
		return err
	}
	projectName, err := loadComposeProjectName(serviceRoot, pulledTask.GetServiceName())
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeDown, func() error {
		return runComposeDown(ctx, serviceRoot, projectName, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		return runComposeUp(ctx, serviceRoot, projectName, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := reportServiceStatus(ctx, client, cfg, pulledTask.GetServiceName(), store.ServiceRuntimeRunning); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "restart task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

type pruneTaskParams struct {
	Target string `json:"target"`
}

type rusticMaintenanceTaskParams struct {
	ServiceDir  string `json:"service_dir,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	DataName    string `json:"data_name,omitempty"`
	RepoWide    bool   `json:"repo_wide,omitempty"`
}

func executePruneTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params := parsePruneParams(pulledTask)
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting prune task: target=%s\n", params.Target)); err != nil {
		return err
	}

	var pruneErr error
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepPrune, func() error {
		pruneErr = runDockerPrune(ctx, params.Target, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
		return pruneErr
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}

	if pruneErr != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), pruneErr)
	}

	if err := uploadTaskLog(ctx, logUploader, "prune task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRusticPruneTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	rusticMeta, err := loadRusticTaskMeta(serviceRoot)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting rustic prune task for service=%s compose_service=%s\n", pulledTask.GetServiceName(), rusticMeta.ComposeService)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepPrune, func() error {
		return runRusticPrune(ctx, serviceRoot, rusticMeta, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "rustic prune task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRusticInitTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	rusticMeta, err := loadRusticTaskMeta(serviceRoot)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting rustic init task for service=%s compose_service=%s\n", pulledTask.GetServiceName(), rusticMeta.ComposeService)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepInit, func() error {
		return runRusticInit(ctx, serviceRoot, rusticMeta, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "rustic init task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeRusticForgetTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params := parseRusticMaintenanceParams(pulledTask)
	var bundle *bundleResult
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		var err error
		bundle, err = downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), "")
		if err != nil {
			return err
		}
		return uploadTaskLog(ctx, logUploader, "render step completed after bundle download\n")
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, bundle)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	rusticMeta, err := loadRusticTaskMeta(serviceRoot)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting rustic forget task for service=%s compose_service=%s repo_wide=%t\n", pulledTask.GetServiceName(), rusticMeta.ComposeService, params.RepoWide)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepPrune, func() error {
		return runRusticForget(ctx, serviceRoot, rusticMeta, params, pulledTask.GetNodeId(), func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "rustic forget task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeCaddyReloadTask(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	serviceRoot, err := localServiceRoot(cfg.RepoDir, pulledTask, nil)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	caddyMeta, err := loadCaddyInfraMeta(serviceRoot)
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting caddy reload task for service=%s compose_service=%s config_dir=%s\n", pulledTask.GetServiceName(), caddyMeta.ComposeService, caddyMeta.ConfigDir)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepCaddyReload, func() error {
		projectName, err := loadComposeProjectName(serviceRoot, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runCaddyReload(ctx, serviceRoot, projectName, caddyMeta.ComposeService, caddyMeta.ConfigDir, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := uploadTaskLog(ctx, logUploader, "caddy reload task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

func executeCaddySyncTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params := decodeTaskParams(pulledTask.GetParamsJson())
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting caddy sync task for service=%s node=%s repo_revision=%s full_rebuild=%t\n", pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision(), params.FullRebuild)); err != nil {
		return err
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepRender, func() error {
		return uploadTaskLog(ctx, logUploader, "render step completed for caddy sync task\n")
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepCaddySync, func() error {
		return syncCaddyFilesForTask(ctx, bundleClient, client, cfg, pulledTask, logUploader)
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := uploadTaskLog(ctx, logUploader, "caddy sync task finished successfully\n"); err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusSucceeded, "")
}

type caddyInfraMeta struct {
	ComposeService string
	ConfigDir      string
}

type caddyServiceMeta struct {
	Source string
}

type rusticTaskMeta struct {
	ComposeService string
	Profile        string
	InitArgs       []string
}

func loadCaddyInfraMeta(serviceDir string) (caddyInfraMeta, error) {
	meta, err := repo.LoadServiceMeta(filepath.Join(serviceDir, "composia-meta.yaml"))
	if err != nil {
		return caddyInfraMeta{}, err
	}
	return caddyInfraMeta{
		ComposeService: meta.CaddyComposeService(),
		ConfigDir:      meta.CaddyConfigDir(),
	}, nil
}

func loadServiceCaddyMeta(serviceDir string) (caddyServiceMeta, error) {
	meta, err := repo.LoadServiceMeta(filepath.Join(serviceDir, "composia-meta.yaml"))
	if err != nil {
		return caddyServiceMeta{}, err
	}
	return caddyServiceMeta{Source: repo.CaddySource(repo.Service{Meta: meta})}, nil
}

func loadRusticTaskMeta(serviceDir string) (rusticTaskMeta, error) {
	meta, err := repo.LoadServiceMeta(filepath.Join(serviceDir, "composia-meta.yaml"))
	if err != nil {
		return rusticTaskMeta{}, err
	}
	return rusticTaskMeta{ComposeService: meta.RusticComposeService(), Profile: meta.RusticProfile(), InitArgs: meta.RusticInitArgs()}, nil
}

func parsePruneParams(pulledTask *agentv1.AgentTask) pruneTaskParams {
	paramsJSON := pulledTask.GetParamsJson()
	if paramsJSON == "" {
		return pruneTaskParams{Target: "all"}
	}
	var params pruneTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return pruneTaskParams{Target: "all"}
	}
	if params.Target == "" {
		params.Target = "all"
	}
	return params
}

func parseRusticMaintenanceParams(pulledTask *agentv1.AgentTask) rusticMaintenanceTaskParams {
	paramsJSON := pulledTask.GetParamsJson()
	if paramsJSON == "" {
		return rusticMaintenanceTaskParams{}
	}
	var params rusticMaintenanceTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return rusticMaintenanceTaskParams{}
	}
	return params
}

func runDockerPrune(ctx context.Context, target string, uploadLog func(string) error) error {
	var args []string

	switch target {
	case "all":
		return runDockerPruneAll(ctx, uploadLog)
	case "containers":
		args = []string{"container", "prune", "-f"}
	case "networks":
		args = []string{"network", "prune", "-f"}
	case "images":
		args = []string{"image", "prune", "-f"}
	case "images_all":
		args = []string{"image", "prune", "-a", "-f"}
	case "volumes":
		args = []string{"volume", "prune", "-f"}
	case "system_all":
		args = []string{"system", "prune", "-a", "-f"}
	case "system_all_volumes":
		args = []string{"system", "prune", "-a", "--volumes", "-f"}
	case "builder":
		args = []string{"builder", "prune", "-f"}
	default:
		return fmt.Errorf("unknown prune target: %q", target)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	if err := runCommandWithLiveLogs(cmd, uploadLog); err != nil {
		return fmt.Errorf("docker %s prune failed: %w", target, err)
	}
	return nil
}

func runDockerPruneAll(ctx context.Context, uploadLog func(string) error) error {
	targets := []string{"containers", "networks", "images", "volumes", "builder"}
	for _, target := range targets {
		if err := uploadLog(fmt.Sprintf("pruning %s...\n", target)); err != nil {
			return err
		}
		if err := runDockerPrune(ctx, target, uploadLog); err != nil {
			return err
		}
	}
	return nil
}

func buildRusticComposeRunArgs(composeService, profile string, commandArgs ...string) []string {
	args := []string{"compose", "run", "--rm", composeService}
	if profile != "" {
		args = append(args, "-P", profile)
	}
	args = append(args, commandArgs...)
	return args
}

type commandLogWriter struct {
	mu         sync.Mutex
	uploadLog  func(string) error
	output     bytes.Buffer
	captureOut bool
}

func newCommandLogWriter(uploadLog func(string) error, captureOut bool) *commandLogWriter {
	return &commandLogWriter{uploadLog: uploadLog, captureOut: captureOut}
}

func (writer *commandLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()

	if writer.captureOut {
		if _, err := writer.output.Write(p); err != nil {
			return 0, err
		}
	}
	if writer.uploadLog != nil {
		if err := writer.uploadLog(string(p)); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (writer *commandLogWriter) String() string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.output.String()
}

func prepareCommandForTerminalLog(command *exec.Cmd) {
	if command == nil {
		return
	}

	env := command.Environ()
	env = setCommandEnv(env, "TERM", "xterm-256color")
	env = setCommandEnv(env, "CLICOLOR_FORCE", "1")
	env = setCommandEnv(env, "FORCE_COLOR", "1")
	env = setCommandEnv(env, "COMPOSE_ANSI", "always")
	env = setCommandEnv(env, "COMPOSE_STATUS_STDOUT", "1")
	env = setCommandEnv(env, "COMPOSE_PROGRESS", "tty")
	command.Env = env
}

func setCommandEnv(env []string, key, value string) []string {
	prefix := key + "="
	for index, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[index] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func runCommandWithLiveLogs(command *exec.Cmd, uploadLog func(string) error) error {
	writer := newCommandLogWriter(uploadLog, false)
	prepareCommandForTerminalLog(command)
	command.Stdout = writer
	command.Stderr = writer
	return command.Run()
}

func runCommandWithLiveLogsAndCapture(command *exec.Cmd, uploadLog func(string) error) (string, error) {
	writer := newCommandLogWriter(uploadLog, true)
	prepareCommandForTerminalLog(command)
	command.Stdout = writer
	command.Stderr = writer
	err := command.Run()
	return writer.String(), err
}

func runRusticInit(ctx context.Context, serviceDir string, meta rusticTaskMeta, uploadLog func(string) error) error {
	args := buildRusticComposeRunArgs(meta.ComposeService, meta.Profile, append([]string{"init"}, meta.InitArgs...)...)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose run rustic init failed: %w", err)
	}
	return nil
}

func runRusticForget(ctx context.Context, serviceDir string, meta rusticTaskMeta, params rusticMaintenanceTaskParams, nodeID string, uploadLog func(string) error) error {
	args := buildRusticComposeRunArgs(meta.ComposeService, meta.Profile, "forget")
	if !params.RepoWide && nodeID != "" {
		args = append(args, "--filter-host", nodeID)
	}
	if !params.RepoWide && params.ServiceName != "" {
		args = append(args, "--filter-tags", "composia-service:"+params.ServiceName)
	}
	if !params.RepoWide && params.DataName != "" {
		args = append(args, "--filter-tags", "composia-data:"+params.DataName)
	}
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose run rustic forget failed: %w", err)
	}
	return nil
}

func runRusticPrune(ctx context.Context, serviceDir string, meta rusticTaskMeta, uploadLog func(string) error) error {
	args := buildRusticComposeRunArgs(meta.ComposeService, meta.Profile, "prune")
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose run rustic prune failed: %w", err)
	}
	return nil
}

func runCaddyReload(ctx context.Context, serviceDir, projectName, composeService, configDir string, uploadLog func(string) error) error {
	configPath := filepath.Join(configDir, "Caddyfile")
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "exec", "-T", composeService, "caddy", "reload", "--config", configPath, "--adapter", "caddyfile")
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose exec caddy reload failed: %w", err)
	}
	return nil
}

func executeCaddySyncStep(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader, serviceRoot string) error {
	return executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepCaddySync, func() error {
		return syncServiceCaddyFile(ctx, cfg, pulledTask.GetServiceDir(), serviceRoot, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	})
}

func syncCaddyFilesForTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params := decodeTaskParams(pulledTask.GetParamsJson())
	serviceDirs := append([]string(nil), params.ServiceDirs...)
	if len(serviceDirs) == 0 && pulledTask.GetServiceDir() != "" {
		serviceDirs = []string{pulledTask.GetServiceDir()}
	}
	if params.FullRebuild {
		entries, err := os.ReadDir(cfg.CaddyGeneratedDir())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read generated caddy directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".caddy") {
				continue
			}
			if err := os.Remove(filepath.Join(cfg.CaddyGeneratedDir(), entry.Name())); err != nil {
				return fmt.Errorf("remove generated caddy file %q: %w", entry.Name(), err)
			}
			if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("removed generated caddy file %s\n", filepath.Join(cfg.CaddyGeneratedDir(), entry.Name()))); err != nil {
				return err
			}
		}
	}
	for _, serviceDir := range serviceDirs {
		bundleTask := proto.Clone(pulledTask).(*agentv1.AgentTask)
		bundleTask.ServiceDir = serviceDir
		bundleTask.ServiceName = filepath.Base(serviceDir)
		bundle, err := downloadServiceBundle(ctx, bundleClient, cfg, pulledTask.GetTaskId(), serviceDir)
		if err != nil {
			return err
		}
		serviceRoot, err := localServiceRoot(cfg.RepoDir, bundleTask, bundle)
		if err != nil {
			return err
		}
		if err := syncServiceCaddyFile(ctx, cfg, bundleTask.GetServiceDir(), serviceRoot, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		}); err != nil {
			return err
		}
	}
	return nil
}

func decodeTaskParams(paramsJSON string) controllerTaskParams {
	if paramsJSON == "" {
		return controllerTaskParams{}
	}
	var params controllerTaskParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return controllerTaskParams{}
	}
	return params
}

type controllerTaskParams struct {
	ServiceDirs []string `json:"service_dirs,omitempty"`
	FullRebuild bool     `json:"full_rebuild,omitempty"`
}

func syncServiceCaddyFile(ctx context.Context, cfg *config.AgentConfig, serviceDir, serviceRoot string, uploadLog func(string) error) error {
	targetName, err := caddyGeneratedFileName(serviceDir)
	if err != nil {
		return err
	}
	meta, err := loadServiceCaddyMeta(serviceRoot)
	if err != nil {
		return err
	}
	if meta.Source == "" {
		if err := uploadLog(fmt.Sprintf("service_dir=%s does not enable network.caddy, skipping caddy sync\n", serviceDir)); err != nil {
			return err
		}
		return nil
	}
	sourcePath, err := resolveServiceCaddySourcePath(serviceRoot, meta.Source)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(cfg.CaddyGeneratedDir(), targetName)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create generated caddy directory for %q: %w", targetPath, err)
	}
	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read caddy source %q: %w", sourcePath, err)
	}
	if err := os.WriteFile(targetPath, contents, 0o644); err != nil {
		return fmt.Errorf("write generated caddy file %q: %w", targetPath, err)
	}
	if err := uploadLog(fmt.Sprintf("synced caddy file source=%s target=%s\n", sourcePath, targetPath)); err != nil {
		return err
	}
	return nil
}

func removeServiceCaddyFile(ctx context.Context, cfg *config.AgentConfig, serviceDir string, uploadLog func(string) error) error {
	targetName, err := caddyGeneratedFileName(serviceDir)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(cfg.CaddyGeneratedDir(), targetName)
	if err := os.Remove(targetPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := uploadLog(fmt.Sprintf("generated caddy file %s does not exist, skipping removal\n", targetPath)); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("remove generated caddy file %q: %w", targetPath, err)
	}
	if err := uploadLog(fmt.Sprintf("removed generated caddy file %s\n", targetPath)); err != nil {
		return err
	}
	return nil
}

func caddyGeneratedFileName(serviceDir string) (string, error) {
	cleanDir := filepath.Clean(strings.TrimSpace(serviceDir))
	if cleanDir == "" || cleanDir == "." {
		return "", fmt.Errorf("service_dir is required for caddy generated file")
	}
	base := filepath.Base(cleanDir)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "", fmt.Errorf("resolve caddy generated file name from service_dir %q", serviceDir)
	}
	return base + ".caddy", nil
}

func resolveServiceCaddySourcePath(serviceRoot, source string) (string, error) {
	cleanSource := filepath.Clean(strings.TrimSpace(source))
	if cleanSource == "." || cleanSource == "" {
		return "", fmt.Errorf("network.caddy.source must not be empty")
	}
	resolved := filepath.Join(serviceRoot, cleanSource)
	relative, err := filepath.Rel(serviceRoot, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve caddy source path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("network.caddy.source %q escapes service root", source)
	}
	return resolved, nil
}

func localServiceRoot(repoDir string, pulledTask *agentv1.AgentTask, bundle *bundleResult) (string, error) {
	if bundle != nil && bundle.RootPath != "" {
		return bundle.RootPath, nil
	}
	if pulledTask.GetServiceDir() == "" {
		return "", fmt.Errorf("task is missing service_dir")
	}
	serviceRoot := filepath.Join(repoDir, pulledTask.GetServiceDir())
	if _, err := os.Stat(filepath.Join(serviceRoot, "composia-meta.yaml")); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("service bundle for %q is not present on agent", pulledTask.GetServiceName())
		}
		return "", fmt.Errorf("stat service bundle for %q: %w", pulledTask.GetServiceName(), err)
	}
	return serviceRoot, nil
}

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

func loadBackupRuntimeConfig(serviceRoot string) (*backupcfg.RuntimeConfig, error) {
	content, err := os.ReadFile(filepath.Join(serviceRoot, ".composia-backup.json"))
	if err != nil {
		return nil, fmt.Errorf("read backup runtime config: %w", err)
	}
	var cfg backupcfg.RuntimeConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("decode backup runtime config: %w", err)
	}
	if cfg.Rustic == nil {
		return nil, fmt.Errorf("backup runtime config is missing rustic provider")
	}
	if cfg.Rustic.ServiceDir == "" {
		return nil, fmt.Errorf("backup runtime config is missing rustic service_dir")
	}
	if cfg.Rustic.NodeID == "" {
		return nil, fmt.Errorf("backup runtime config is missing rustic node_id")
	}
	if len(cfg.Items) == 0 {
		return nil, fmt.Errorf("backup runtime config did not include any items")
	}
	return &cfg, nil
}

func dataProtectStageRoot(stateDir string) string {
	return filepath.Join(stateDir, "data-protect")
}

func dataProtectStageDir(stateDir, prefix string) (string, error) {
	stageRoot := dataProtectStageRoot(stateDir)
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return "", fmt.Errorf("create data-protect stage root %q: %w", stageRoot, err)
	}
	stageDir, err := os.MkdirTemp(stageRoot, prefix)
	if err != nil {
		return "", fmt.Errorf("create data-protect stage dir: %w", err)
	}
	return stageDir, nil
}

func rusticDataProtectPath(localPath string, cfg *config.AgentConfig, rustic *backupcfg.RusticConfig) (string, error) {
	if rustic == nil || strings.TrimSpace(rustic.DataProtectDir) == "" {
		return localPath, nil
	}
	stageRoot := dataProtectStageRoot(cfg.StateDir)
	relativePath, err := filepath.Rel(stageRoot, localPath)
	if err != nil {
		return "", fmt.Errorf("resolve relative data-protect path for %q: %w", localPath, err)
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside agent data-protect stage root %q", localPath, stageRoot)
	}
	return filepath.Join(rustic.DataProtectDir, relativePath), nil
}

func loadRestoreRuntimeConfig(serviceRoot string) (*backupcfg.RestoreConfig, error) {
	content, err := os.ReadFile(filepath.Join(serviceRoot, ".composia-restore.json"))
	if err != nil {
		return nil, fmt.Errorf("read restore runtime config: %w", err)
	}
	var cfg backupcfg.RestoreConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("decode restore runtime config: %w", err)
	}
	if cfg.Rustic == nil {
		return nil, fmt.Errorf("restore runtime config is missing rustic provider")
	}
	if cfg.Rustic.ServiceDir == "" {
		return nil, fmt.Errorf("restore runtime config is missing rustic service_dir")
	}
	if len(cfg.Items) == 0 {
		return nil, fmt.Errorf("restore runtime config did not include any items")
	}
	return &cfg, nil
}

func backupRuntimeItem(ctx context.Context, cfg *config.AgentConfig, serviceRoot, rusticRoot, taskID string, item backupcfg.RuntimeItem, rustic *backupcfg.RusticConfig, logUploader *taskLogUploader) (string, time.Time, time.Time, error) {
	startedAt := time.Now().UTC()
	stagingDir, err := dataProtectStageDir(cfg.StateDir, fmt.Sprintf("backup-%s-%s-", taskID, item.Name))
	if err != nil {
		return "", startedAt, time.Time{}, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()
	if err := stageBackupItem(ctx, serviceRoot, stagingDir, item, logUploader); err != nil {
		return "", startedAt, time.Time{}, err
	}
	rusticSourceDir, err := rusticDataProtectPath(stagingDir, cfg, rustic)
	if err != nil {
		return "", startedAt, time.Time{}, err
	}
	artifactRef, err := runRusticBackup(ctx, rusticRoot, rustic, rusticSourceDir, item, logUploader)
	if err != nil {
		return "", startedAt, time.Time{}, err
	}
	return artifactRef, startedAt, time.Now().UTC(), nil
}

func restoreRuntimeItem(ctx context.Context, cfg *config.AgentConfig, serviceRoot, rusticRoot, taskID string, item backupcfg.RestoreItem, rustic *backupcfg.RusticConfig, logUploader *taskLogUploader) error {
	stagingDir, err := dataProtectStageDir(cfg.StateDir, fmt.Sprintf("restore-%s-%s-", taskID, item.Name))
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()
	rusticTargetDir, err := rusticDataProtectPath(stagingDir, cfg, rustic)
	if err != nil {
		return err
	}
	if err := runRusticRestore(ctx, rusticRoot, rustic, item.ArtifactRef, rusticTargetDir, logUploader); err != nil {
		return err
	}
	return applyRestoreItem(ctx, serviceRoot, filepath.Join(stagingDir, item.Name), item, logUploader)
}

func applyRestoreItem(ctx context.Context, serviceRoot, stagingDir string, item backupcfg.RestoreItem, logUploader *taskLogUploader) (retErr error) {
	switch item.Strategy {
	case "files.copy", "files.copy_after_stop":
		if item.Strategy == "files.copy_after_stop" {
			projectName, err := loadComposeProjectName(serviceRoot, filepath.Base(serviceRoot))
			if err != nil {
				return err
			}
			if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("stopping compose project %s for cold restore item %s\n", projectName, item.Name)); err != nil {
				return err
			}
			if err := runComposeDown(ctx, serviceRoot, projectName, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); err != nil {
				return err
			}
			defer func() {
				if restartErr := runComposeUp(ctx, serviceRoot, projectName, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); restartErr != nil {
					if retErr == nil {
						retErr = fmt.Errorf("restart compose project after cold restore: %w", restartErr)
						return
					}
					_ = uploadTaskLog(ctx, logUploader, fmt.Sprintf("restart compose project after cold restore failed: %v\n", restartErr))
				}
			}()
		}
		for _, include := range item.Include {
			if err := restoreInclude(ctx, serviceRoot, stagingDir, include); err != nil {
				return fmt.Errorf("restore item %s include %s: %w", item.Name, include, err)
			}
		}
		return nil
	case "database.pgimport":
		projectName, err := loadComposeProjectName(serviceRoot, filepath.Base(serviceRoot))
		if err != nil {
			return err
		}
		dumpPath := filepath.Join(stagingDir, item.Name+".sql")
		return runComposePGImport(ctx, serviceRoot, projectName, item.Service, dumpPath, func(output string) error { return uploadTaskLog(ctx, logUploader, output) })
	default:
		return fmt.Errorf("restore strategy %q is not implemented yet", item.Strategy)
	}
}

func restoreInclude(ctx context.Context, serviceRoot, stagingDir, include string) error {
	kind, normalized, err := repo.ClassifyDataInclude(include)
	if err != nil {
		return err
	}
	if kind == repo.DataIncludeKindServicePath {
		sourcePath := filepath.Join(stagingDir, "paths", sanitizeStagePath(include))
		targetPath, err := resolveIncludeServicePath(serviceRoot, normalized)
		if err != nil {
			return err
		}
		return replacePath(sourcePath, targetPath)
	}
	return restoreDirToVolume(ctx, filepath.Join(stagingDir, "volumes", sanitizeStagePath(normalized)), normalized)
}

func replacePath(sourcePath, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat restore source %q: %w", sourcePath, err)
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("clear restore target %q: %w", targetPath, err)
	}
	if info.IsDir() {
		return copyDir(sourcePath, targetPath)
	}
	return copyFile(sourcePath, targetPath, info.Mode())
}

func stageBackupItem(ctx context.Context, serviceRoot, stagingDir string, item backupcfg.RuntimeItem, logUploader *taskLogUploader) (retErr error) {
	switch item.Strategy {
	case "files.copy":
		for _, include := range item.Include {
			if err := stageInclude(ctx, serviceRoot, stagingDir, include); err != nil {
				return fmt.Errorf("stage backup item %s include %s: %w", item.Name, include, err)
			}
		}
		return nil
	case "files.copy_after_stop":
		projectName, err := loadComposeProjectName(serviceRoot, filepath.Base(serviceRoot))
		if err != nil {
			return err
		}
		if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("stopping compose project %s for cold backup item %s\n", projectName, item.Name)); err != nil {
			return err
		}
		if err := runComposeDown(ctx, serviceRoot, projectName, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); err != nil {
			return err
		}
		defer func() {
			if restartErr := runComposeUp(ctx, serviceRoot, projectName, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); restartErr != nil && retErr == nil {
				retErr = fmt.Errorf("restart compose project after cold backup: %w", restartErr)
			}
		}()
		for _, include := range item.Include {
			if err := stageInclude(ctx, serviceRoot, stagingDir, include); err != nil {
				return fmt.Errorf("stage cold backup item %s include %s: %w", item.Name, include, err)
			}
		}
		return nil
	case "database.pgdumpall":
		projectName, err := loadComposeProjectName(serviceRoot, filepath.Base(serviceRoot))
		if err != nil {
			return err
		}
		targetPath := filepath.Join(stagingDir, item.Name+".sql")
		if err := runComposePGDumpAll(ctx, serviceRoot, projectName, item.Service, targetPath, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("backup strategy %q is not implemented yet", item.Strategy)
	}
}

func stageInclude(ctx context.Context, serviceRoot, stagingDir, include string) error {
	kind, normalized, err := repo.ClassifyDataInclude(include)
	if err != nil {
		return err
	}
	if kind == repo.DataIncludeKindServicePath {
		sourcePath, err := resolveIncludeServicePath(serviceRoot, normalized)
		if err != nil {
			return err
		}
		return copyIntoStage(sourcePath, filepath.Join(stagingDir, "paths", sanitizeStagePath(include)))
	}
	return stageVolumeToDir(ctx, filepath.Join(stagingDir, "volumes", sanitizeStagePath(normalized)), normalized)
}

func stageVolumeToDir(ctx context.Context, targetDir, volumeName string) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("clear staged volume dir %q: %w", targetDir, err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create staged volume dir %q: %w", targetDir, err)
	}
	return runDockerVolumeTarExport(ctx, volumeName, targetDir)
}

func restoreDirToVolume(ctx context.Context, sourceDir, volumeName string) error {
	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("stat restore volume source %q: %w", sourceDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("restore volume source %q must be a directory", sourceDir)
	}
	return runDockerVolumeTarImport(ctx, sourceDir, volumeName)
}

func resolveIncludeServicePath(serviceRoot, includePath string) (string, error) {
	serviceRoot = filepath.Clean(serviceRoot)
	targetPath := filepath.Join(serviceRoot, includePath)
	relPath, err := filepath.Rel(serviceRoot, targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve include path %q: %w", includePath, err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("include %q must stay within the service root", includePath)
	}
	return targetPath, nil
}

func copyIntoStage(sourcePath, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source path %q: %w", sourcePath, err)
	}
	if info.IsDir() {
		return copyDir(sourcePath, targetPath)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create stage dir for %q: %w", targetPath, err)
	}
	return copyFile(sourcePath, targetPath, info.Mode())
}

func copyDir(sourceDir, targetDir string) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		targetPath := targetDir
		if relPath != "." {
			targetPath = filepath.Join(targetDir, relPath)
		}
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(sourcePath, targetPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		_ = sourceFile.Close()
		return err
	}
	if _, err = io.Copy(targetFile, sourceFile); err != nil {
		_ = targetFile.Close()
		_ = sourceFile.Close()
		return err
	}
	if err := targetFile.Close(); err != nil {
		_ = sourceFile.Close()
		return err
	}
	if err := sourceFile.Close(); err != nil {
		return err
	}
	return nil
}

func writeTarStream(sourceDir string, tarWriter *tar.Writer) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		linkTarget := ""
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read symlink %q: %w", path, err)
			}
		}
		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err = io.Copy(tarWriter, file); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		return nil
	})
}

func runDockerVolumeTarExport(ctx context.Context, volumeName, targetDir string) error {
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "-v", volumeName+":/source:ro", dockerVolumeTarImage, "tar", "-C", "/source", "-cf", "-", ".")
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("prepare docker volume export stdout for %q: %w", volumeName, err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start docker volume export for %q: %w", volumeName, err)
	}
	extractErr := extractTarStream(stdout, targetDir)
	if extractErr != nil {
		_ = stdout.Close()
	}
	waitErr := command.Wait()
	if extractErr != nil {
		if waitErr != nil {
			return fmt.Errorf("extract docker volume %q tar stream: %w (docker wait error: %v)", volumeName, extractErr, formatDockerRunError("docker run volume export failed", waitErr, stderr.String()))
		}
		return fmt.Errorf("extract docker volume %q tar stream: %w", volumeName, extractErr)
	}
	if waitErr != nil {
		return formatDockerRunError("docker run volume export failed", waitErr, stderr.String())
	}
	return nil
}

func runDockerVolumeTarImport(ctx context.Context, sourceDir, volumeName string) error {
	command := exec.CommandContext(ctx, "docker", "run", "-i", "--rm", "-v", volumeName+":/target", dockerVolumeTarImage, "sh", "-c", dockerVolumeImportCmd)
	stdin, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("prepare docker volume import stdin for %q: %w", volumeName, err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start docker volume import for %q: %w", volumeName, err)
	}
	streamErr := writeTarToWriter(sourceDir, stdin)
	closeErr := stdin.Close()
	waitErr := command.Wait()
	if streamErr != nil {
		if waitErr != nil {
			return fmt.Errorf("write restore tar stream for docker volume %q: %w (docker wait error: %v)", volumeName, streamErr, formatDockerRunError("docker run volume import failed", waitErr, stderr.String()))
		}
		return fmt.Errorf("write restore tar stream for docker volume %q: %w", volumeName, streamErr)
	}
	if closeErr != nil {
		if waitErr != nil {
			return fmt.Errorf("close docker volume import stdin for %q: %w (docker wait error: %v)", volumeName, closeErr, formatDockerRunError("docker run volume import failed", waitErr, stderr.String()))
		}
		return fmt.Errorf("close docker volume import stdin for %q: %w", volumeName, closeErr)
	}
	if waitErr != nil {
		return formatDockerRunError("docker run volume import failed", waitErr, stderr.String())
	}
	return nil
}

func writeTarToWriter(sourceDir string, writer io.Writer) error {
	tarWriter := tar.NewWriter(writer)
	if err := writeTarStream(sourceDir, tarWriter); err != nil {
		_ = tarWriter.Close()
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("close tar stream: %w", err)
	}
	return nil
}

func formatDockerRunError(prefix string, err error, stderr string) error {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return fmt.Errorf("%s: %w: %s", prefix, err, trimmed)
}

func runComposePGDumpAll(ctx context.Context, serviceDir, projectName, serviceName, targetPath string, uploadLog func(string) error) error {
	if serviceName == "" {
		return fmt.Errorf("pgdumpall backup is missing service name")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create pgdump target dir: %w", err)
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create pgdump target file: %w", err)
	}
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "exec", "-T", serviceName, "pg_dumpall")
	command.Dir = serviceDir
	command.Stdout = targetFile
	command.Stderr = newCommandLogWriter(uploadLog, false)
	err = command.Run()
	if err != nil {
		_ = targetFile.Close()
		return fmt.Errorf("docker compose exec pg_dumpall failed: %w", err)
	}
	if err := targetFile.Close(); err != nil {
		return fmt.Errorf("close pgdump target file: %w", err)
	}
	return nil
}

func runComposePGImport(ctx context.Context, serviceDir, projectName, serviceName, sourcePath string, uploadLog func(string) error) error {
	if serviceName == "" {
		return fmt.Errorf("pgimport restore is missing service name")
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open pgimport source file: %w", err)
	}
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "exec", "-T", serviceName, "psql")
	command.Dir = serviceDir
	command.Stdin = sourceFile
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		_ = sourceFile.Close()
		return fmt.Errorf("docker compose exec psql failed: %w", err)
	}
	if err := sourceFile.Close(); err != nil {
		return fmt.Errorf("close pgimport source file: %w", err)
	}
	return nil
}

var rusticSnapshotRegexp = regexp.MustCompile(`(?m)snapshot\s+([0-9a-fA-F]+)\b[^\n]*\bsaved\.?`)

func runRusticBackup(ctx context.Context, rusticDir string, rustic *backupcfg.RusticConfig, sourceDir string, item backupcfg.RuntimeItem, logUploader *taskLogUploader) (string, error) {
	args := buildRusticComposeRunArgs(rustic.ComposeService, rustic.Profile, "backup", "--host", rustic.NodeID)
	for _, tag := range buildRusticTags(item.Tags) {
		args = append(args, "--tag", tag)
	}
	args = append(args, sourceDir, "--as-path", item.Name)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = rusticDir
	output, err := runCommandWithLiveLogsAndCapture(command, func(output string) error {
		return uploadTaskLog(ctx, logUploader, output)
	})
	if err != nil {
		return "", fmt.Errorf("docker compose run rustic backup failed: %w", err)
	}
	matches := rusticSnapshotRegexp.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", fmt.Errorf("could not parse rustic snapshot id from output")
	}
	return matches[1], nil
}

func runRusticRestore(ctx context.Context, rusticDir string, rustic *backupcfg.RusticConfig, artifactRef, targetDir string, logUploader *taskLogUploader) error {
	args := buildRusticComposeRunArgs(rustic.ComposeService, rustic.Profile, "restore", artifactRef, targetDir)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = rusticDir
	if err := runCommandWithLiveLogs(command, func(output string) error {
		return uploadTaskLog(ctx, logUploader, output)
	}); err != nil {
		return fmt.Errorf("docker compose run rustic restore failed: %w", err)
	}
	return nil
}
func buildRusticTags(explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	return nil
}

func sanitizeStagePath(value string) string {
	replacer := strings.NewReplacer("/", "_", `\\`, "_", ":", "_")
	return replacer.Replace(strings.TrimPrefix(strings.TrimPrefix(value, "./"), "/"))
}

func reportServiceStatus(ctx context.Context, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, serviceName, runtimeStatus string) error {
	_, err := client.ReportServiceInstanceStatus(ctx, connect.NewRequest(&agentv1.ReportServiceInstanceStatusRequest{
		ServiceName:   serviceName,
		NodeId:        cfg.NodeID,
		RuntimeStatus: runtimeStatus,
		ReportedAt:    timestamppb.Now(),
	}))
	if err != nil {
		return fmt.Errorf("report service instance status: %w", err)
	}
	return nil
}

type bundleResult struct {
	ServiceName  string
	RelativeRoot string
	RootPath     string
}

func downloadServiceBundle(ctx context.Context, client agentv1connect.BundleServiceClient, cfg *config.AgentConfig, taskID, serviceDir string) (*bundleResult, error) {
	stream, err := client.GetServiceBundle(ctx, connect.NewRequest(&agentv1.GetServiceBundleRequest{TaskId: taskID, ServiceDir: serviceDir}))
	if err != nil {
		return nil, fmt.Errorf("get service bundle: %w", err)
	}
	defer func() { _ = stream.Close() }()

	tempFile, err := os.CreateTemp(cfg.StateDir, "bundle-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("create temp bundle file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()
	defer func() {
		if tempFile != nil {
			_ = tempFile.Close()
		}
	}()

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
	tempFile = nil

	targetRoot := filepath.Join(cfg.RepoDir, relativeRoot)
	stageParentDir := filepath.Dir(targetRoot)
	if err := os.MkdirAll(stageParentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create bundle stage parent dir %q: %w", stageParentDir, err)
	}
	stageDir, err := os.MkdirTemp(stageParentDir, ".bundle-stage-*")
	if err != nil {
		return nil, fmt.Errorf("create bundle stage dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageDir) }()
	if err := extractTarGz(tempPath, stageDir); err != nil {
		return nil, err
	}
	stagedRoot := filepath.Join(stageDir, relativeRoot)
	if _, err := os.Stat(stagedRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("bundle archive did not contain expected root %q", relativeRoot)
		}
		return nil, fmt.Errorf("stat staged bundle root %q: %w", stagedRoot, err)
	}
	if err := replaceDirectory(targetRoot, stagedRoot); err != nil {
		return nil, err
	}
	result.RootPath = targetRoot
	return result, nil
}

func replaceDirectory(targetRoot, stagedRoot string) error {
	parentDir := filepath.Dir(targetRoot)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("create bundle parent dir %q: %w", parentDir, err)
	}
	backupRoot := targetRoot + ".bak"
	if err := os.RemoveAll(backupRoot); err != nil {
		return fmt.Errorf("remove old bundle backup %q: %w", backupRoot, err)
	}
	hadExisting := false
	if _, err := os.Stat(targetRoot); err == nil {
		hadExisting = true
		if err := os.Rename(targetRoot, backupRoot); err != nil {
			return fmt.Errorf("move existing bundle %q to backup: %w", targetRoot, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat existing bundle root %q: %w", targetRoot, err)
	}
	if err := os.Rename(stagedRoot, targetRoot); err != nil {
		if hadExisting {
			_ = os.Rename(backupRoot, targetRoot)
		}
		return fmt.Errorf("activate staged bundle %q: %w", targetRoot, err)
	}
	if hadExisting {
		if err := os.RemoveAll(backupRoot); err != nil {
			return fmt.Errorf("remove bundle backup %q: %w", backupRoot, err)
		}
	}
	return nil
}

func extractTarGz(archivePath, destinationDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive %q: %w", archivePath, err)
	}
	defer func() { _ = file.Close() }()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip archive %q: %w", archivePath, err)
	}
	defer func() { _ = gzipReader.Close() }()
	if err := extractTarStream(gzipReader, destinationDir); err != nil {
		return fmt.Errorf("extract tar archive %q: %w", archivePath, err)
	}
	return nil
}

func extractTarStream(reader io.Reader, destinationDir string) error {
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return fmt.Errorf("create tar destination %q: %w", destinationDir, err)
	}
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read tar stream: %w", err)
		}

		cleanTargetPath, err := tarEntryTargetPath(destinationDir, header.Name)
		if err != nil {
			return fmt.Errorf("tar entry %q escapes destination root: %w", header.Name, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTargetPath, 0o755); err != nil {
				return fmt.Errorf("create tar directory %q: %w", cleanTargetPath, err)
			}
			if err := os.Chmod(cleanTargetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("chmod tar directory %q: %w", cleanTargetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanTargetPath), 0o755); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", cleanTargetPath, err)
			}
			outFile, err := os.OpenFile(cleanTargetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create tar file %q: %w", cleanTargetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close()
				return fmt.Errorf("write tar file %q: %w", cleanTargetPath, err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("close tar file %q: %w", cleanTargetPath, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(cleanTargetPath), 0o755); err != nil {
				return fmt.Errorf("create parent directory for symlink %q: %w", cleanTargetPath, err)
			}
			if err := os.RemoveAll(cleanTargetPath); err != nil {
				return fmt.Errorf("clear tar symlink target %q: %w", cleanTargetPath, err)
			}
			if err := os.Symlink(header.Linkname, cleanTargetPath); err != nil {
				return fmt.Errorf("create tar symlink %q: %w", cleanTargetPath, err)
			}
		case tar.TypeLink:
			linkTargetPath, err := tarEntryTargetPath(destinationDir, header.Linkname)
			if err != nil {
				return fmt.Errorf("tar hardlink %q escapes destination root: %w", header.Linkname, err)
			}
			if err := os.MkdirAll(filepath.Dir(cleanTargetPath), 0o755); err != nil {
				return fmt.Errorf("create parent directory for hardlink %q: %w", cleanTargetPath, err)
			}
			if err := os.RemoveAll(cleanTargetPath); err != nil {
				return fmt.Errorf("clear tar hardlink target %q: %w", cleanTargetPath, err)
			}
			if err := os.Link(linkTargetPath, cleanTargetPath); err != nil {
				return fmt.Errorf("create tar hardlink %q: %w", cleanTargetPath, err)
			}
		case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			continue
		default:
			return fmt.Errorf("unsupported tar entry type %d for %q", header.Typeflag, header.Name)
		}
	}
}

func tarEntryTargetPath(destinationDir, entryName string) (string, error) {
	cleanEntryName := filepath.Clean(filepath.FromSlash(entryName))
	if filepath.IsAbs(cleanEntryName) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	if cleanEntryName == "." {
		return filepath.Clean(destinationDir), nil
	}
	if cleanEntryName == ".." || strings.HasPrefix(cleanEntryName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("parent traversal is not allowed")
	}
	cleanDestinationDir := filepath.Clean(destinationDir)
	cleanTargetPath := filepath.Join(cleanDestinationDir, cleanEntryName)
	if !strings.HasPrefix(cleanTargetPath, cleanDestinationDir+string(os.PathSeparator)) && cleanTargetPath != cleanDestinationDir {
		return "", fmt.Errorf("target path escapes destination root")
	}
	return cleanTargetPath, nil
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
	return repo.ComposeProjectName(meta.ProjectName, fallback), nil
}

func runComposeUp(ctx context.Context, serviceDir, projectName string, uploadLog func(string) error) error {
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "up", "-d")
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

func runComposeDown(ctx context.Context, serviceDir, projectName string, uploadLog func(string) error) error {
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "down")
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	return nil
}

func runComposePull(ctx context.Context, serviceDir, projectName string, uploadLog func(string) error) error {
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "pull")
	command.Dir = serviceDir
	if err := runCommandWithLiveLogs(command, uploadLog); err != nil {
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

func collectDockerStats() (*agentv1.DockerStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stats agentv1.DockerStats
	stats.DockerServerVersion = dockerServerVersion()

	containers, err := dockerContainerStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect container stats: %w", err)
	}
	stats.ContainersTotal = containers.total
	stats.ContainersRunning = containers.running
	stats.ContainersStopped = containers.stopped
	stats.ContainersPaused = containers.paused

	images, err := dockerImageCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect image count: %w", err)
	}
	stats.Images = images

	networks, err := dockerNetworkCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect network count: %w", err)
	}
	stats.Networks = networks

	volumes, volumesSize, err := dockerVolumeStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect volume stats: %w", err)
	}
	stats.Volumes = volumes
	stats.VolumesSizeBytes = volumesSize

	stats.DisksUsageBytes, err = dockerDiskUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect disk usage: %w", err)
	}

	return &stats, nil
}

type containerStats struct {
	total   uint32
	running uint32
	stopped uint32
	paused  uint32
}

func dockerContainerStats(ctx context.Context) (containerStats, error) {
	output, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.State}}").Output()
	if err != nil {
		return containerStats{}, nil
	}

	var stats containerStats
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case "running":
			stats.running++
		case "exited":
			stats.stopped++
		case "paused":
			stats.paused++
		}
	}
	stats.total = stats.running + stats.stopped + stats.paused
	return stats, nil
}

func dockerImageCount(ctx context.Context) (uint32, error) {
	output, err := exec.CommandContext(ctx, "docker", "images", "-q").Output()
	if err != nil {
		return 0, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := uint32(0)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}

func dockerNetworkCount(ctx context.Context) (uint32, error) {
	output, err := exec.CommandContext(ctx, "docker", "network", "ls", "-q").Output()
	if err != nil {
		return 0, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := uint32(0)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}

func dockerVolumeStats(ctx context.Context) (uint32, uint64, error) {
	output, err := exec.CommandContext(ctx, "docker", "volume", "ls", "-q").Output()
	if err != nil {
		return 0, 0, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := uint32(0)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	sizeOutput, err := exec.CommandContext(ctx, "docker", "system", "df", "-v", "--format", "{{.RealSize}}").Output()
	if err != nil {
		return count, 0, nil
	}

	var totalSize uint64
	sizeLines := strings.Split(strings.TrimSpace(string(sizeOutput)), "\n")
	for _, line := range sizeLines {
		line = strings.TrimSpace(line)
		if line == "" || line == "0B" {
			continue
		}
		if size, ok := parseSize(line); ok {
			totalSize += size
		}
	}

	return count, totalSize, nil
}

func parseSize(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	if strings.Contains(s, ",") {
		if strings.Contains(s, ".") {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			s = strings.ReplaceAll(s, ",", ".")
		}
	}

	s = strings.ToUpper(s)

	unitMultipliers := []struct {
		suffix string
		bytes  float64
	}{
		{suffix: "PIB", bytes: 1024 * 1024 * 1024 * 1024 * 1024},
		{suffix: "PB", bytes: 1000 * 1000 * 1000 * 1000 * 1000},
		{suffix: "TIB", bytes: 1024 * 1024 * 1024 * 1024},
		{suffix: "TB", bytes: 1000 * 1000 * 1000 * 1000},
		{suffix: "GIB", bytes: 1024 * 1024 * 1024},
		{suffix: "GB", bytes: 1000 * 1000 * 1000},
		{suffix: "MIB", bytes: 1024 * 1024},
		{suffix: "MB", bytes: 1000 * 1000},
		{suffix: "KIB", bytes: 1024},
		{suffix: "KB", bytes: 1000},
		{suffix: "B", bytes: 1},
	}

	mult := float64(1)
	for _, unit := range unitMultipliers {
		if strings.HasSuffix(s, unit.suffix) {
			mult = unit.bytes
			s = strings.TrimSpace(strings.TrimSuffix(s, unit.suffix))
			break
		}
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || value < 0 {
		return 0, false
	}

	return uint64(math.Round(value * mult)), true
}

func dockerDiskUsage(ctx context.Context) (uint64, error) {
	output, err := exec.CommandContext(ctx, "docker", "system", "df", "--format", "{{.Size}}").Output()
	if err != nil {
		return 0, nil
	}

	var total uint64
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "0B" {
			continue
		}
		if size, ok := parseSize(line); ok {
			total += size
		}
	}
	return total, nil
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

func controllerHTTPClient(controllerAddr string) *http.Client {
	if strings.HasPrefix(strings.ToLower(controllerAddr), "http://") {
		return &http.Client{Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, network, addr)
			},
		}}
	}
	return &http.Client{}
}
