package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/libdns/libdns"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

const (
	cloudflareTunnelAPIBaseURL   = "https://api.cloudflare.com/client/v4"
	defaultTunnelFallbackService = "http_status:404"
)

type cloudflareTunnelClient interface {
	UpdateConfiguration(ctx context.Context, accountID, tunnelID string, ingress []cloudflareTunnelIngress) error
}

type cloudflareTunnelRuntimeStore interface {
	GetServiceSnapshot(ctx context.Context, serviceName string) (store.ServiceSnapshot, error)
}

type defaultCloudflareTunnelClient struct {
	apiToken   string
	apiBaseURL string
	httpClient *http.Client
}

type cloudflareTunnelConfigPayload struct {
	Config cloudflareTunnelConfig `json:"config"`
}

type cloudflareTunnelConfig struct {
	Ingress []cloudflareTunnelIngress `json:"ingress"`
}

type cloudflareTunnelIngress struct {
	Hostname      string                         `json:"hostname,omitempty"`
	Service       string                         `json:"service"`
	Path          string                         `json:"path,omitempty"`
	OriginRequest *cloudflareTunnelOriginRequest `json:"originRequest,omitempty"`
}

type cloudflareTunnelOriginRequest struct {
	NoTLSVerify      *bool   `json:"noTLSVerify,omitempty"`
	HTTPHostHeader   string  `json:"httpHostHeader,omitempty"`
	OriginServerName string  `json:"originServerName,omitempty"`
	ConnectTimeout   *uint32 `json:"connectTimeout,omitempty"`
	TLSTimeout       *uint32 `json:"tlsTimeout,omitempty"`
}

type desiredCloudflareTunnelService struct {
	Service   repo.Service
	Directory string
	Tunnel    string
	Config    repo.CloudflareTunnelConfig
}

func (executor *controllerTaskExecutor) executeCloudflareTunnelSyncTask(ctx context.Context, record task.Record) error {
	if err := appendTaskLogRaw(record.LogPath, fmt.Sprintf("starting controller cloudflare_tunnel_sync task for service=%s repo_revision=%s\n", record.ServiceName, record.RepoRevision)); err != nil {
		return executor.failControllerTask(ctx, record, task.StepCloudflareTunnelSync, err)
	}
	startedAt := time.Now().UTC()
	logControllerTaskStepStarted(record, task.StepCloudflareTunnelSync)
	if err := executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: task.StepCloudflareTunnelSync, Status: task.StatusRunning, StartedAt: &startedAt}); err != nil {
		return executor.failControllerTask(ctx, record, task.StepCloudflareTunnelSync, err)
	}

	params, err := taskParams(record.ParamsJSON)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepCloudflareTunnelSync, err)
	}
	client, dnsClient, err := newCloudflareTunnelClients(executor.cfg)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepCloudflareTunnelSync, err)
	}
	if err := syncCloudflareTunnels(ctx, executor.cfg, executor.availableNodeIDs, client, dnsClient, executor.db, record.RepoRevision, params.ExcludedServiceDir, record.LogPath); err != nil {
		return executor.failControllerTask(ctx, record, task.StepCloudflareTunnelSync, err)
	}

	finishedAt := time.Now().UTC()
	if err := executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: task.StepCloudflareTunnelSync, Status: task.StatusSucceeded, StartedAt: &startedAt, FinishedAt: &finishedAt}); err != nil {
		return executor.failControllerTask(ctx, record, task.StepCloudflareTunnelSync, err)
	}
	logControllerTaskStepSucceeded(record, task.StepCloudflareTunnelSync, startedAt, finishedAt)
	if err := appendTaskLogRaw(record.LogPath, "cloudflare_tunnel_sync task finished successfully\n"); err != nil {
		return executor.failControllerTask(ctx, record, task.StepCloudflareTunnelSync, err)
	}
	if err := executor.db.CompleteTask(ctx, record.TaskID, task.StatusSucceeded, finishedAt, ""); err != nil {
		return err
	}
	record.Status = task.StatusSucceeded
	record.FinishedAt = &finishedAt
	notifyTaskResult(executor.taskResults, record.TaskID)
	dispatchTaskRecordNotification(executor.notifier, corenotify.EventTaskCompleted, record)
	logControllerTaskFinished(record, finishedAt)
	return nil
}

func newCloudflareTunnelClients(cfg *config.ControllerConfig) (cloudflareTunnelClient, dnsClient, error) {
	if cfg == nil || cfg.CloudflareTunnel == nil {
		return nil, nil, errors.New("controller cloudflare_tunnel is not configured")
	}
	tunnelCfg := cfg.CloudflareTunnel
	token := strings.TrimSpace(tunnelCfg.APIToken)
	if token == "" {
		return nil, nil, errors.New("controller cloudflare_tunnel.api_token is empty")
	}
	tunnelClient := &defaultCloudflareTunnelClient{apiToken: token, apiBaseURL: cloudflareTunnelAPIBaseURL, httpClient: &http.Client{Timeout: 15 * time.Second}}
	dnsClient, err := newCloudflareDNSClient(&config.CloudflareDNSConfig{APIToken: token, Zones: tunnelCfg.Zones})
	if err != nil {
		return nil, nil, err
	}
	return tunnelClient, dnsClient, nil
}

func syncCloudflareTunnels(ctx context.Context, cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, client cloudflareTunnelClient, dnsClient dnsClient, runtimeStore cloudflareTunnelRuntimeStore, revision, excludedServiceDir, logPath string) error {
	services, err := cloudflareTunnelServicesAtRevision(cfg, availableNodeIDs, revision)
	if err != nil {
		return err
	}
	excludedServiceDir = strings.TrimSpace(excludedServiceDir)
	if excludedServiceDir != "" {
		excludedServiceDir = filepath.ToSlash(filepath.Clean(excludedServiceDir))
	}
	servicesByTunnel := make(map[string][]desiredCloudflareTunnelService)
	var excluded *desiredCloudflareTunnelService
	for _, service := range services {
		routable, err := cloudflareTunnelServiceIsRoutable(ctx, runtimeStore, service.Service.Name)
		if err != nil {
			return err
		}
		if excludedServiceDir != "" && service.Directory == excludedServiceDir && (runtimeStore == nil || !routable) {
			excluded = &service
			continue
		}
		if !routable {
			if err := appendTaskLogRaw(logPath, fmt.Sprintf("skipping Cloudflare Tunnel service=%s because it has no running instances\n", service.Service.Name)); err != nil {
				return err
			}
			continue
		}
		servicesByTunnel[service.Tunnel] = append(servicesByTunnel[service.Tunnel], service)
	}

	tunnelNames := make([]string, 0, len(cfg.CloudflareTunnel.Tunnels))
	for tunnelName := range cfg.CloudflareTunnel.Tunnels {
		tunnelNames = append(tunnelNames, tunnelName)
	}
	sort.Strings(tunnelNames)
	for _, tunnelName := range tunnelNames {
		tunnelCfg := cfg.CloudflareTunnel.Tunnels[tunnelName]
		ingress := cloudflareTunnelIngressRules(servicesByTunnel[tunnelName], fallbackCloudflareTunnelService(tunnelCfg))
		if err := appendTaskLogRaw(logPath, fmt.Sprintf("updating Cloudflare tunnel=%s ingress_rules=%d\n", tunnelName, len(ingress))); err != nil {
			return err
		}
		if err := client.UpdateConfiguration(ctx, cfg.CloudflareTunnel.AccountID, tunnelCfg.TunnelID, ingress); err != nil {
			return err
		}
		for _, service := range servicesByTunnel[tunnelName] {
			if err := syncCloudflareTunnelDNS(ctx, dnsClient, service.Config.Hostname, tunnelCfg.TunnelID, logPath); err != nil {
				return err
			}
		}
	}
	if excluded != nil {
		tunnelCfg := cfg.CloudflareTunnel.Tunnels[excluded.Tunnel]
		if err := deleteCloudflareTunnelDNS(ctx, dnsClient, excluded.Config.Hostname, tunnelCfg.TunnelID, logPath); err != nil {
			return err
		}
	}
	return nil
}

func cloudflareTunnelServiceIsRoutable(ctx context.Context, runtimeStore cloudflareTunnelRuntimeStore, serviceName string) (bool, error) {
	if runtimeStore == nil {
		return true, nil
	}
	snapshot, err := runtimeStore.GetServiceSnapshot(ctx, serviceName)
	if err != nil {
		if errors.Is(err, store.ErrServiceNotFound) {
			return false, nil
		}
		return false, err
	}
	return snapshot.RunningCount > 0, nil
}

func cloudflareTunnelServicesAtRevision(cfg *config.ControllerConfig, availableNodeIDs map[string]struct{}, revision string) ([]desiredCloudflareTunnelService, error) {
	if cfg == nil || cfg.CloudflareTunnel == nil {
		return nil, errors.New("controller cloudflare_tunnel is not configured")
	}
	paths, err := repo.ListServiceMetaFilesAtRevision(cfg.RepoDir, revision)
	if err != nil {
		return nil, err
	}
	services := make([]desiredCloudflareTunnelService, 0)
	seenRoutes := make(map[string]string)
	for _, path := range paths {
		serviceDir := filepath.ToSlash(filepath.Dir(path))
		service, err := repo.FindServiceAtRevision(cfg.RepoDir, revision, serviceDir, availableNodeIDs)
		if err != nil {
			return nil, err
		}
		if !repo.CloudflareTunnelManaged(service) {
			continue
		}
		tunnel, err := resolveCloudflareTunnelName(cfg, service)
		if err != nil {
			return nil, err
		}
		routeKey := tunnel + "\x00" + strings.ToLower(strings.TrimSpace(service.Meta.Network.CloudflareTunnel.Hostname)) + "\x00" + strings.TrimSpace(service.Meta.Network.CloudflareTunnel.Path)
		if previous, exists := seenRoutes[routeKey]; exists {
			return nil, fmt.Errorf("services %q and %q declare the same cloudflare tunnel route", previous, service.Name)
		}
		seenRoutes[routeKey] = service.Name
		services = append(services, desiredCloudflareTunnelService{Service: service, Directory: serviceDir, Tunnel: tunnel, Config: *service.Meta.Network.CloudflareTunnel})
	}
	sort.Slice(services, func(left, right int) bool {
		if services[left].Tunnel != services[right].Tunnel {
			return services[left].Tunnel < services[right].Tunnel
		}
		return services[left].Config.Hostname < services[right].Config.Hostname
	})
	return services, nil
}

func resolveCloudflareTunnelName(cfg *config.ControllerConfig, service repo.Service) (string, error) {
	tunnelName := strings.TrimSpace(service.Meta.Network.CloudflareTunnel.Tunnel)
	if tunnelName != "" {
		if _, exists := cfg.CloudflareTunnel.Tunnels[tunnelName]; !exists {
			return "", fmt.Errorf("service %q references unknown cloudflare tunnel %q", service.Name, tunnelName)
		}
		return tunnelName, nil
	}
	resolved := ""
	for _, nodeID := range service.TargetNodes {
		mapping, exists := cfg.CloudflareTunnel.Nodes[nodeID]
		if !exists || strings.TrimSpace(mapping.Tunnel) == "" {
			return "", fmt.Errorf("service %q target node %q does not have a controller.cloudflare_tunnel.nodes binding", service.Name, nodeID)
		}
		if _, exists := cfg.CloudflareTunnel.Tunnels[mapping.Tunnel]; !exists {
			return "", fmt.Errorf("service %q target node %q references unknown cloudflare tunnel %q", service.Name, nodeID, mapping.Tunnel)
		}
		if resolved == "" {
			resolved = mapping.Tunnel
			continue
		}
		if resolved != mapping.Tunnel {
			return "", fmt.Errorf("service %q targets multiple cloudflare tunnels; set network.cloudflare_tunnel.tunnel explicitly", service.Name)
		}
	}
	if resolved == "" {
		return "", fmt.Errorf("service %q cannot resolve cloudflare tunnel", service.Name)
	}
	return resolved, nil
}

func cloudflareTunnelIngressRules(services []desiredCloudflareTunnelService, fallbackService string) []cloudflareTunnelIngress {
	rules := make([]cloudflareTunnelIngress, 0, len(services)+1)
	for _, service := range services {
		rules = append(rules, cloudflareTunnelIngress{
			Hostname:      strings.TrimSpace(service.Config.Hostname),
			Service:       strings.TrimSpace(service.Config.Service),
			Path:          strings.TrimSpace(service.Config.Path),
			OriginRequest: cloudflareTunnelOriginRequestFromMeta(service.Config.OriginRequest),
		})
	}
	rules = append(rules, cloudflareTunnelIngress{Service: fallbackService})
	return rules
}

func cloudflareTunnelOriginRequestFromMeta(meta *repo.CloudflareTunnelOriginRequest) *cloudflareTunnelOriginRequest {
	if meta == nil {
		return nil
	}
	request := &cloudflareTunnelOriginRequest{
		NoTLSVerify:      meta.NoTLSVerify,
		HTTPHostHeader:   strings.TrimSpace(meta.HTTPHostHeader),
		OriginServerName: strings.TrimSpace(meta.OriginServerName),
		ConnectTimeout:   meta.ConnectTimeout,
		TLSTimeout:       meta.TLSTimeout,
	}
	if request.NoTLSVerify == nil && request.HTTPHostHeader == "" && request.OriginServerName == "" && request.ConnectTimeout == nil && request.TLSTimeout == nil {
		return nil
	}
	return request
}

func fallbackCloudflareTunnelService(cfg config.ControllerCloudflareTunnel) string {
	if strings.TrimSpace(cfg.FallbackService) != "" {
		return strings.TrimSpace(cfg.FallbackService)
	}
	return defaultTunnelFallbackService
}

func syncCloudflareTunnelDNS(ctx context.Context, client dnsClient, hostname, tunnelID, logPath string) error {
	fqdn := ensureTrailingDot(hostname)
	zone, err := matchingZone(ctx, client, fqdn)
	if err != nil {
		return err
	}
	relativeName := libdns.RelativeName(fqdn, zone)
	target := ensureTrailingDot(tunnelID + ".cfargotunnel.com")
	proxied := true
	if err := appendTaskLogRaw(logPath, fmt.Sprintf("setting Cloudflare Tunnel CNAME hostname=%s target=%s\n", fqdn, target)); err != nil {
		return err
	}
	if _, err := client.SetRecords(ctx, zone, []libdns.Record{libdns.CNAME{Name: relativeName, Target: target}}); err != nil {
		return err
	}
	return client.ApplyRecordOptions(ctx, zone, fqdn, dnsRecordTypeCNAME, dnsRecordOptions{Proxied: &proxied, Comment: "Managed by Composia Cloudflare Tunnel"})
}

func deleteCloudflareTunnelDNS(ctx context.Context, client dnsClient, hostname, tunnelID, logPath string) error {
	fqdn := ensureTrailingDot(hostname)
	zone, err := matchingZone(ctx, client, fqdn)
	if err != nil {
		return err
	}
	if err := appendTaskLogRaw(logPath, fmt.Sprintf("deleting Cloudflare Tunnel CNAME hostname=%s target=%s.cfargotunnel.com\n", fqdn, tunnelID)); err != nil {
		return err
	}
	_, err = client.DeleteRecords(ctx, zone, []libdns.Record{libdns.RR{Name: libdns.RelativeName(fqdn, zone), Type: dnsRecordTypeCNAME}})
	return err
}

func (client *defaultCloudflareTunnelClient) UpdateConfiguration(ctx context.Context, accountID, tunnelID string, ingress []cloudflareTunnelIngress) error {
	payload := cloudflareTunnelConfigPayload{Config: cloudflareTunnelConfig{Ingress: ingress}}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s/configurations", client.apiBaseURL, strings.TrimSpace(accountID), strings.TrimSpace(tunnelID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+client.apiToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= 300 {
		return fmt.Errorf("update Cloudflare tunnel configuration %s: unexpected status %s", tunnelID, response.Status)
	}
	return nil
}

var _ cloudflareTunnelClient = (*defaultCloudflareTunnelClient)(nil)
