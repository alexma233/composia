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
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
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
	taskRetryAfterPollFail = 1 * time.Second
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.LoadAgent(configPath)
	if err != nil {
		return err
	}

	if err := ensureAgentDirs(cfg); err != nil {
		return err
	}

	httpClient := controllerHTTPClient(cfg.ControllerAddr)
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
	startPeriodicTask(ctx, heartbeatInterval, "initial heartbeat", "heartbeat", func() error {
		return sendHeartbeat(ctx, reportClient, cfg)
	})
	startPeriodicTask(ctx, 5*time.Minute, "initial docker stats report", "docker stats report", func() error {
		return reportDockerStats(ctx, reportClient, cfg)
	})

	startExecTunnelLoop(ctx, reportClient, cfg.NodeID)

	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			if err := pollAndRunTask(ctx, taskClient, bundleClient, reportClient, cfg); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("task poll failed: %v", err)
				if !sleepWithContext(ctx, taskRetryAfterPollFail) {
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		}
	}
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

	return executePulledTask(ctx, bundleClient, reportClient, cfg, response.Msg.GetTask())
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
	defer logUploader.Close()

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
	case string(task.TypeDockerList), string(task.TypeDockerInspect), string(task.TypeDockerStart), string(task.TypeDockerStop), string(task.TypeDockerRestart), string(task.TypeDockerLogs), string(task.TypeDockerRemove):
		return executeDockerTask(ctx, client, cfg, pulledTask, logUploader)
	default:
		return reportTaskCompletion(ctx, client, pulledTask.GetTaskId(), task.StatusFailed, fmt.Sprintf("task type %q is not implemented", pulledTask.GetType()))
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
			startedAt := time.Now().UTC()
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
	output, err := cmd.CombinedOutput()
	if outStr := string(output); outStr != "" {
		if logErr := uploadLog(outStr); logErr != nil {
			return logErr
		}
	}
	if err != nil {
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

func runRusticInit(ctx context.Context, serviceDir string, meta rusticTaskMeta, uploadLog func(string) error) error {
	args := buildRusticComposeRunArgs(meta.ComposeService, meta.Profile, append([]string{"init"}, meta.InitArgs...)...)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = serviceDir
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadLog(string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
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
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadLog(string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker compose run rustic forget failed: %w", err)
	}
	return nil
}

func runRusticPrune(ctx context.Context, serviceDir string, meta rusticTaskMeta, uploadLog func(string) error) error {
	args := buildRusticComposeRunArgs(meta.ComposeService, meta.Profile, "prune")
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = serviceDir
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadLog(string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker compose run rustic prune failed: %w", err)
	}
	return nil
}

func runCaddyReload(ctx context.Context, serviceDir, projectName, composeService, configDir string, uploadLog func(string) error) error {
	configPath := filepath.Join(configDir, "Caddyfile")
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "exec", "-T", composeService, "caddy", "reload", "--config", configPath, "--adapter", "caddyfile")
	command.Dir = serviceDir
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadLog(string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
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
	startedAt := timestamppb.Now()
	if _, err := client.ReportTaskStepState(ctx, connect.NewRequest(&agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusRunning), StartedAt: startedAt})); err != nil {
		return fmt.Errorf("report running step %s: %w", stepName, err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("step %s started\n", stepName)); err != nil {
		return err
	}
	if err := execute(); err != nil {
		finishedAt := timestamppb.Now()
		_, _ = client.ReportTaskStepState(ctx, connect.NewRequest(&agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusFailed), StartedAt: startedAt, FinishedAt: finishedAt}))
		_ = uploadTaskLog(ctx, logUploader, fmt.Sprintf("step %s failed: %v\n", stepName, err))
		return err
	}
	finishedAt := timestamppb.Now()
	if _, err := client.ReportTaskStepState(ctx, connect.NewRequest(&agentv1.ReportTaskStepStateRequest{TaskId: taskID, StepName: string(stepName), Status: string(task.StatusSucceeded), StartedAt: startedAt, FinishedAt: finishedAt})); err != nil {
		return fmt.Errorf("report succeeded step %s: %w", stepName, err)
	}
	return uploadTaskLog(ctx, logUploader, fmt.Sprintf("step %s succeeded\n", stepName))
}

func reportTaskCompletion(ctx context.Context, client agentv1connect.AgentReportServiceClient, taskID string, status task.Status, errorSummary string) error {
	_, err := client.ReportTaskState(ctx, connect.NewRequest(&agentv1.ReportTaskStateRequest{TaskId: taskID, Status: string(status), ErrorSummary: errorSummary, FinishedAt: timestamppb.Now()}))
	if err != nil {
		return fmt.Errorf("report task completion: %w", err)
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
		return "", time.Time{}, time.Time{}, err
	}
	defer os.RemoveAll(stagingDir)
	if err := stageBackupItem(ctx, serviceRoot, stagingDir, item, logUploader); err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	rusticSourceDir, err := rusticDataProtectPath(stagingDir, cfg, rustic)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	artifactRef, err := runRusticBackup(ctx, rusticRoot, rustic, rusticSourceDir, item, logUploader)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	return artifactRef, startedAt, time.Now().UTC(), nil
}

func restoreRuntimeItem(ctx context.Context, cfg *config.AgentConfig, serviceRoot, rusticRoot, taskID string, item backupcfg.RestoreItem, rustic *backupcfg.RusticConfig, logUploader *taskLogUploader) error {
	stagingDir, err := dataProtectStageDir(cfg.StateDir, fmt.Sprintf("restore-%s-%s-", taskID, item.Name))
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingDir)
	rusticTargetDir, err := rusticDataProtectPath(stagingDir, cfg, rustic)
	if err != nil {
		return err
	}
	if err := runRusticRestore(ctx, rusticRoot, rustic, item.ArtifactRef, rusticTargetDir, logUploader); err != nil {
		return err
	}
	return applyRestoreItem(ctx, serviceRoot, stagingDir, item, logUploader)
}

func applyRestoreItem(ctx context.Context, serviceRoot, stagingDir string, item backupcfg.RestoreItem, logUploader *taskLogUploader) error {
	switch item.Strategy {
	case "files.copy":
		for _, include := range item.Include {
			if err := restoreInclude(ctx, serviceRoot, stagingDir, include); err != nil {
				return fmt.Errorf("restore item %s include %s: %w", item.Name, include, err)
			}
		}
		return nil
	case "files.untar":
		archivePath := filepath.Join(stagingDir, item.Name+".tar.gz")
		extractDir := filepath.Join(stagingDir, "__untar")
		if err := extractTarGz(archivePath, extractDir); err != nil {
			return err
		}
		for _, include := range item.Include {
			if err := restoreInclude(ctx, serviceRoot, extractDir, include); err != nil {
				return fmt.Errorf("restore untar item %s include %s: %w", item.Name, include, err)
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
	var sourcePath string
	var targetPath string
	if kind == repo.DataIncludeKindServicePath {
		sourcePath = filepath.Join(stagingDir, "paths", sanitizeStagePath(include))
		targetPath, err = resolveIncludeServicePath(serviceRoot, normalized)
		if err != nil {
			return err
		}
	} else {
		sourcePath = filepath.Join(stagingDir, "volumes", normalized)
		mountpoint, err := dockerVolumeMountpoint(ctx, normalized)
		if err != nil {
			return fmt.Errorf("resolve docker volume %q: %w", normalized, err)
		}
		targetPath = mountpoint
	}
	return replacePath(sourcePath, targetPath)
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
	case "files.tar_after_stop":
		projectName, err := loadComposeProjectName(serviceRoot, filepath.Base(serviceRoot))
		if err != nil {
			return err
		}
		if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("temporarily stopping compose project %s for backup item %s\n", projectName, item.Name)); err != nil {
			return err
		}
		if err := runComposeDown(ctx, serviceRoot, projectName, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); err != nil {
			return err
		}
		defer func() {
			if restartErr := runComposeUp(ctx, serviceRoot, projectName, func(output string) error { return uploadTaskLog(ctx, logUploader, output) }); restartErr != nil && retErr == nil {
				retErr = fmt.Errorf("restart compose project after backup: %w", restartErr)
			}
		}()
		return stageTarBackupItem(ctx, serviceRoot, stagingDir, item)
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

func stageTarBackupItem(ctx context.Context, serviceRoot, stagingDir string, item backupcfg.RuntimeItem) error {
	copyRoot := filepath.Join(stagingDir, "__tar_stage")
	for _, include := range item.Include {
		if err := stageInclude(ctx, serviceRoot, copyRoot, include); err != nil {
			return fmt.Errorf("stage tar backup item %s include %s: %w", item.Name, include, err)
		}
	}
	archivePath := filepath.Join(stagingDir, item.Name+".tar.gz")
	return createTarGzArchive(copyRoot, archivePath)
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
	mountpoint, err := dockerVolumeMountpoint(ctx, normalized)
	if err != nil {
		return fmt.Errorf("resolve docker volume %q: %w", normalized, err)
	}
	return copyIntoStage(mountpoint, filepath.Join(stagingDir, "volumes", normalized))
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
	defer sourceFile.Close()
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer targetFile.Close()
	_, err = io.Copy(targetFile, sourceFile)
	return err
}

func createTarGzArchive(sourceDir, archivePath string) error {
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create tar archive %q: %w", archivePath, err)
	}
	defer archiveFile.Close()
	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()
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
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tarWriter, file)
		return err
	})
}

func dockerVolumeMountpoint(ctx context.Context, volumeName string) (string, error) {
	output, err := exec.CommandContext(ctx, "docker", "volume", "inspect", "-f", "{{ .Mountpoint }}", volumeName).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker volume inspect failed: %w %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
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
	defer targetFile.Close()
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "exec", "-T", serviceName, "pg_dumpall")
	command.Dir = serviceDir
	command.Stdout = targetFile
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err = command.Run()
	if stderr.Len() > 0 {
		if logErr := uploadLog(stderr.String()); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker compose exec pg_dumpall failed: %w", err)
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
	defer sourceFile.Close()
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", projectName, "exec", "-T", serviceName, "psql")
	command.Dir = serviceDir
	command.Stdin = sourceFile
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadLog(string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
		return fmt.Errorf("docker compose exec psql failed: %w", err)
	}
	return nil
}

var rusticSnapshotRegexp = regexp.MustCompile(`snapshot\s+([0-9a-fA-F]+)\s+saved`)

func runRusticBackup(ctx context.Context, rusticDir string, rustic *backupcfg.RusticConfig, sourceDir string, item backupcfg.RuntimeItem, logUploader *taskLogUploader) (string, error) {
	args := buildRusticComposeRunArgs(rustic.ComposeService, rustic.Profile, "backup", "--host", rustic.NodeID)
	for _, tag := range buildRusticTags(item.Tags) {
		args = append(args, "--tag", tag)
	}
	args = append(args, sourceDir, "--as-path", item.Name)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = rusticDir
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadTaskLog(ctx, logUploader, string(output)); logErr != nil {
			return "", logErr
		}
	}
	if err != nil {
		return "", fmt.Errorf("docker compose run rustic backup failed: %w", err)
	}
	matches := rusticSnapshotRegexp.FindStringSubmatch(string(output))
	if len(matches) != 2 {
		return "", fmt.Errorf("could not parse rustic snapshot id from output")
	}
	return matches[1], nil
}

func runRusticRestore(ctx context.Context, rusticDir string, rustic *backupcfg.RusticConfig, artifactRef, targetDir string, logUploader *taskLogUploader) error {
	args := buildRusticComposeRunArgs(rustic.ComposeService, rustic.Profile, "restore", artifactRef, targetDir)
	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = rusticDir
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		if logErr := uploadTaskLog(ctx, logUploader, string(output)); logErr != nil {
			return logErr
		}
	}
	if err != nil {
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
	stageParentDir := filepath.Dir(targetRoot)
	if err := os.MkdirAll(stageParentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create bundle stage parent dir %q: %w", stageParentDir, err)
	}
	stageDir, err := os.MkdirTemp(stageParentDir, ".bundle-stage-*")
	if err != nil {
		return nil, fmt.Errorf("create bundle stage dir: %w", err)
	}
	defer os.RemoveAll(stageDir)
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
		case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			// These entries carry tar metadata only and should not block bundle extraction.
			continue
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
	s = strings.ToUpper(s)

	mult := uint64(1)
	if strings.HasSuffix(s, "GIB") || strings.HasSuffix(s, "GB") {
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "GIB"), "GB")
	} else if strings.HasSuffix(s, "MIB") || strings.HasSuffix(s, "MB") {
		mult = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "MIB"), "MB")
	} else if strings.HasSuffix(s, "KIB") || strings.HasSuffix(s, "KB") {
		mult = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "KIB"), "KB")
	} else if strings.HasSuffix(s, "B") {
		s = strings.TrimSuffix(s, "B")
	}

	s = strings.TrimSpace(s)
	var size uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			size = size*10 + uint64(c-'0')
		} else if c != '.' && c != ',' {
			return 0, false
		}
	}
	return size * mult, true
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
