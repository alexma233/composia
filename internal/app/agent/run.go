package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"golang.org/x/net/http2"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	composeRecreateAuto    = "auto"
	composeRecreateNo      = "no_recreate"
	composeRecreateForce   = "force_recreate"
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.LoadAgent(configPath)
	if err != nil {
		return err
	}
	reloadRequests := make(chan agentReloadRequest)
	stopReloadSignals := watchAgentReloadSignals(ctx, reloadRequests)
	defer stopReloadSignals()

	for {
		runtimeCtx, cancelRuntime := context.WithCancel(ctx)
		runtimeDone := make(chan error, 1)
		go func() {
			runtimeDone <- runAgentRuntime(ctx, runtimeCtx, cfg)
		}()

		reloadAccepted := false
		for !reloadAccepted {
			select {
			case <-ctx.Done():
				cancelRuntime()
				if err := <-runtimeDone; err != nil {
					return err
				}
				return nil
			case err := <-runtimeDone:
				cancelRuntime()
				return err
			case request := <-reloadRequests:
				nextCfg, err := loadReloadAgentConfig(configPath, cfg)
				request.respond(err)
				if err != nil {
					log.Printf("agent config reload rejected: %v", err)
					continue
				}
				cancelRuntime()
				if err := <-runtimeDone; err != nil {
					return err
				}
				cfg = nextCfg
				reloadAccepted = true
				log.Printf("agent config reloaded")
			}
		}
	}
}

type agentReloadRequest struct {
	reply chan error
}

func (request agentReloadRequest) respond(err error) {
	if request.reply == nil {
		return
	}
	request.reply <- err
}

func requestAgentReload(ctx context.Context, requests chan<- agentReloadRequest) error {
	reply := make(chan error, 1)
	request := agentReloadRequest{reply: reply}
	select {
	case requests <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func loadReloadAgentConfig(configPath string, current *config.AgentConfig) (*config.AgentConfig, error) {
	next, err := config.LoadAgent(configPath)
	if err != nil {
		return nil, err
	}
	if err := validateAgentReload(current, next); err != nil {
		return nil, err
	}
	if err := ensureAgentDirs(next); err != nil {
		return nil, err
	}
	return next, nil
}

func validateAgentReload(current, next *config.AgentConfig) error {
	if current == nil || next == nil {
		return fmt.Errorf("agent config is missing")
	}
	immutable := []struct {
		name  string
		left  string
		right string
		path  bool
	}{
		{name: "agent.node_id", left: current.NodeID, right: next.NodeID},
		{name: "agent.repo_dir", left: current.RepoDir, right: next.RepoDir, path: true},
		{name: "agent.state_dir", left: current.StateDir, right: next.StateDir, path: true},
	}
	for _, field := range immutable {
		left := field.left
		right := field.right
		if field.path {
			left = filepath.Clean(left)
			right = filepath.Clean(right)
		}
		if left != right {
			return fmt.Errorf("%s changed and requires process restart", field.name)
		}
	}
	return nil
}

func runAgentRuntime(processCtx, runtimeCtx context.Context, cfg *config.AgentConfig) error {

	if err := ensureAgentDirs(cfg); err != nil {
		return err
	}
	if strings.HasPrefix(strings.ToLower(cfg.ControllerAddr), "http://") {
		log.Printf("warning: agent.controller_addr uses plain HTTP (%s); only use this behind a trusted reverse proxy or on a trusted local network", cfg.ControllerAddr)
	}

	httpClient := controllerHTTPClient(cfg.ControllerAddr)
	clientOptions, err := controllerClientOptions(cfg)
	if err != nil {
		return err
	}
	reportClient := agentv1connect.NewAgentReportServiceClient(
		httpClient,
		rpcutil.JoinBaseURL(cfg.ControllerAddr, rpcutil.AgentAPIBasePath),
		clientOptions...,
	)
	taskClient := agentv1connect.NewAgentTaskServiceClient(
		httpClient,
		rpcutil.JoinBaseURL(cfg.ControllerAddr, rpcutil.AgentAPIBasePath),
		clientOptions...,
	)
	bundleClient := agentv1connect.NewBundleServiceClient(
		httpClient,
		rpcutil.JoinBaseURL(cfg.ControllerAddr, rpcutil.AgentAPIBasePath),
		clientOptions...,
	)

	log.Printf("composia agent loops started: node_id=%s controller=%s", cfg.NodeID, cfg.ControllerAddr)
	startPeriodicTask(runtimeCtx, heartbeatInterval, "initial heartbeat", "heartbeat", func() error {
		return sendHeartbeat(runtimeCtx, reportClient, cfg)
	})
	startPeriodicTask(runtimeCtx, 5*time.Minute, "initial docker stats report", "docker stats report", func() error {
		return reportDockerStats(runtimeCtx, reportClient, cfg)
	})

	startExecTunnelLoop(runtimeCtx, reportClient, cfg.NodeID)
	startContainerLogTunnelLoop(runtimeCtx, reportClient, cfg.NodeID)

	taskLoopDone := make(chan struct{})
	go func() {
		defer close(taskLoopDone)
		for {
			if runtimeCtx.Err() != nil {
				return
			}
			if err := pollAndRunTask(runtimeCtx, processCtx, taskClient, bundleClient, reportClient, cfg); err != nil {
				if runtimeCtx.Err() != nil {
					return
				}
				log.Printf("task loop failed: %v", err)
				if !sleepWithContext(runtimeCtx, taskRetryAfterPollFail) {
					return
				}
			}
		}
	}()

	dockerQueryLoopDone := make(chan struct{})
	go func() {
		defer close(dockerQueryLoopDone)
		for {
			if runtimeCtx.Err() != nil {
				return
			}
			if err := pollAndRunDockerQuery(runtimeCtx, taskClient, reportClient, cfg); err != nil {
				if runtimeCtx.Err() != nil {
					return
				}
				log.Printf("docker query poll failed: %v", err)
				if !sleepWithContext(runtimeCtx, taskRetryAfterPollFail) {
					return
				}
			}
		}
	}()

	<-runtimeCtx.Done()
	<-taskLoopDone
	<-dockerQueryLoopDone
	return nil
}

func controllerClientOptions(cfg *config.AgentConfig) ([]connect.ClientOption, error) {
	customHeaders, err := rpcutil.NewStaticHeadersInterceptor(agentControllerHeaders(cfg.ControllerHeaders))
	if err != nil {
		return nil, err
	}
	options := []connect.ClientOption{connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(cfg.Token), customHeaders)}
	if cfg.ControllerGRPC {
		options = append([]connect.ClientOption{connect.WithGRPC()}, options...)
	}
	return options, nil
}

func agentControllerHeaders(headers []config.AgentControllerHeaderConfig) map[string]string {
	result := make(map[string]string, len(headers))
	for _, header := range headers {
		result[header.Name] = header.Value
	}
	return result
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
	stats, err := collectDockerStats(ctx)
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

func pollAndRunTask(pollCtx, taskCtx context.Context, taskClient agentv1connect.AgentTaskServiceClient, bundleClient agentv1connect.BundleServiceClient, reportClient agentv1connect.AgentReportServiceClient, cfg *config.AgentConfig) error {
	callCtx, cancel := context.WithTimeout(pollCtx, pullNextTaskTimeout)
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
	err = executePulledTaskWithTimeout(taskCtx, bundleClient, reportClient, cfg, pulledTask, taskExecutionTimeout)
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
		reportRequest.ErrorCode = protoDockerQueryErrorCode(queryErr)
	} else {
		applyDockerQueryResult(reportRequest, query, result)
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
