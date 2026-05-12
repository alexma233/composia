package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func executeDeployTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	params, err := decodeTaskParams(pulledTask.GetParamsJson())
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
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
	serviceMeta, err := loadServiceTaskMeta(bundle.RootPath)
	if err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		if serviceMeta.IsConfigInfra() {
			return uploadTaskLog(ctx, logUploader, "service declares infra.config; skipping docker compose up\n")
		}
		compose, _, err := loadComposeCommandConfig(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeUpForServiceTask(ctx, bundle.RootPath, compose, params, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeCaddySyncStep(ctx, client, cfg, pulledTask, logUploader, bundle.RootPath); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if !serviceMeta.IsConfigInfra() {
		reportServiceImageStatesBestEffort(ctx, client, pulledTask, bundle.RootPath, false, logUploader)
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
	params, err := decodeTaskParams(pulledTask.GetParamsJson())
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
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
	serviceMeta, err := loadServiceTaskMeta(bundle.RootPath)
	if err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepPull, func() error {
		if serviceMeta.IsConfigInfra() {
			return uploadTaskLog(ctx, logUploader, "service declares infra.config; skipping docker compose pull\n")
		}
		compose, _, err := loadComposeCommandConfig(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposePull(ctx, bundle.RootPath, compose, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		if serviceMeta.IsConfigInfra() {
			return uploadTaskLog(ctx, logUploader, "service declares infra.config; skipping docker compose up\n")
		}
		compose, _, err := loadComposeCommandConfig(bundle.RootPath, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeUpForServiceTask(ctx, bundle.RootPath, compose, params, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeCaddySyncStep(ctx, client, cfg, pulledTask, logUploader, bundle.RootPath); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if !serviceMeta.IsConfigInfra() {
		reportServiceImageStatesBestEffort(ctx, client, pulledTask, bundle.RootPath, false, logUploader)
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

func executeImageCheckTask(ctx context.Context, bundleClient agentv1connect.BundleServiceClient, client agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig, pulledTask *agentv1.AgentTask, logUploader *taskLogUploader) error {
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting image check task for service=%s node=%s repo_revision=%s\n", pulledTask.GetServiceName(), pulledTask.GetNodeId(), pulledTask.GetRepoRevision())); err != nil {
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
	serviceMeta, err := loadServiceTaskMeta(bundle.RootPath)
	if err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepImageCheck, func() error {
		if serviceMeta.IsConfigInfra() {
			return uploadTaskLog(ctx, logUploader, "service declares infra.config; skipping image check\n")
		}
		if err := reportServiceImageStates(ctx, client, pulledTask, bundle.RootPath, true, logUploader); err != nil {
			return err
		}
		return reportServiceImageUpdateChecks(ctx, client, pulledTask, bundle.RootPath, serviceMeta, logUploader)
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := uploadTaskLog(ctx, logUploader, "image check task finished successfully\n"); err != nil {
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
	serviceMeta, err := loadServiceTaskMeta(bundle.RootPath)
	if err != nil {
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
		if serviceMeta.IsConfigInfra() {
			return uploadTaskLog(ctx, logUploader, "service declares infra.config; skipping docker compose down\n")
		}
		compose, _, err := loadComposeCommandConfig(serviceRoot, pulledTask.GetServiceName())
		if err != nil {
			return err
		}
		return runComposeDown(ctx, serviceRoot, compose, func(output string) error {
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
	serviceMeta, err := loadServiceTaskMeta(serviceRoot)
	if err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := uploadTaskLog(ctx, logUploader, fmt.Sprintf("starting remote restart task for service=%s dir=%s\n", pulledTask.GetServiceName(), serviceRoot)); err != nil {
		return err
	}
	if serviceMeta.IsConfigInfra() {
		err := fmt.Errorf("service %q declares infra.config and cannot be restarted", pulledTask.GetServiceName())
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	compose, _, err := loadComposeCommandConfig(serviceRoot, pulledTask.GetServiceName())
	if err != nil {
		return failTask(ctx, client, pulledTask.GetTaskId(), err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeDown, func() error {
		return runComposeDown(ctx, serviceRoot, compose, func(output string) error {
			return uploadTaskLog(ctx, logUploader, output)
		})
	}); err != nil {
		return failServiceTask(ctx, client, cfg, pulledTask, err)
	}
	if err := executeTaskStep(ctx, client, logUploader, pulledTask.GetTaskId(), task.StepComposeUp, func() error {
		return runComposeUp(ctx, serviceRoot, compose, func(output string) error {
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

func loadServiceTaskMeta(serviceDir string) (repo.ServiceMeta, error) {
	return repo.LoadServiceMeta(filepath.Join(serviceDir, "composia-meta.yaml"))
}

func localServiceRoot(repoDir string, pulledTask *agentv1.AgentTask, bundle *bundleResult) (string, error) {
	if bundle != nil && bundle.RootPath != "" {
		return bundle.RootPath, nil
	}
	if pulledTask.GetServiceDir() == "" {
		return "", fmt.Errorf("task is missing service_dir")
	}
	serviceRoot, _, err := resolveRepoRelativePath(repoDir, pulledTask.GetServiceDir(), "service_dir")
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(serviceRoot, "composia-meta.yaml")); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("service bundle for %q is not present on agent", pulledTask.GetServiceName())
		}
		return "", fmt.Errorf("stat service bundle for %q: %w", pulledTask.GetServiceName(), err)
	}
	return serviceRoot, nil
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
