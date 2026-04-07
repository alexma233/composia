package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cloudflarelibdns "github.com/libdns/cloudflare"
	"github.com/libdns/libdns"

	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

const cloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"

var managedDNSRecordTypes = []string{"A", "AAAA", "CNAME"}

type dnsProviderFactory interface {
	Cloudflare(cfg *config.ControllerConfig) (cloudflareDNSClient, error)
}

type defaultDNSProviderFactory struct{}

type cloudflareDNSClient interface {
	ListZones(ctx context.Context) ([]libdns.Zone, error)
	SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error)
	DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error)
	ApplyRecordOptions(ctx context.Context, zone, fqdn, recordType string, options dnsRecordOptions) error
}

type controllerTaskExecutor struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
	dnsProviders     dnsProviderFactory
}

type desiredServiceDNS struct {
	Zone       string
	FQDN       string
	RecordSets map[string][]libdns.Record
	Options    dnsRecordOptions
}

type dnsRecordOptions struct {
	Proxied *bool
	Comment string
}

type defaultCloudflareDNSClient struct {
	provider   *cloudflarelibdns.Provider
	apiToken   string
	apiBaseURL string
	httpClient *http.Client
}

type cloudflareZoneResponse struct {
	Result []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type cloudflareRecordResponse struct {
	Result []struct {
		ID string `json:"id"`
	} `json:"result"`
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func notifyTaskQueue(notifier *taskQueueNotifier) {
	if notifier != nil {
		notifier.Notify()
	}
}

func runControllerTasks(ctx context.Context, executor *controllerTaskExecutor) {
	if executor == nil || executor.db == nil {
		return
	}
	waitCh := make(chan struct{}, 1)
	if executor.taskQueue != nil {
		waitCh = executor.taskQueue.Subscribe()
		defer executor.taskQueue.Unsubscribe(waitCh)
	}

	for {
		if ctx.Err() != nil {
			return
		}
		if err := executor.runNextPendingTask(ctx); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNoPendingTask) {
			// The task remains terminally failed in SQLite; keep the worker alive.
			appendTaskLogRaw(filepath.Join(executor.cfg.LogDir, "tasks", "controller.log"), fmt.Sprintf("controller task worker error: %v\n", err))
		}

		select {
		case <-ctx.Done():
			return
		case <-waitCh:
		case <-time.After(2 * time.Second):
		}
	}
}

func (executor *controllerTaskExecutor) runNextPendingTask(ctx context.Context) error {
	record, err := executor.db.ClaimNextPendingTaskOfType(ctx, task.TypeDNSUpdate, time.Now().UTC())
	if err != nil {
		return err
	}
	switch record.Type {
	case task.TypeDNSUpdate:
		return executor.executeDNSUpdateTask(ctx, record)
	default:
		return executor.db.CompleteTask(ctx, record.TaskID, task.StatusFailed, time.Now().UTC(), fmt.Sprintf("controller task type %q is not implemented", record.Type))
	}
}

func (executor *controllerTaskExecutor) executeDNSUpdateTask(ctx context.Context, record task.Record) error {
	if err := appendTaskLogRaw(record.LogPath, fmt.Sprintf("starting controller dns_update task for service=%s repo_revision=%s\n", record.ServiceName, record.RepoRevision)); err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}
	startedAt := time.Now().UTC()
	if err := executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: task.StepDNSUpdate, Status: task.StatusRunning, StartedAt: &startedAt}); err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}

	params := taskParams(record.ParamsJSON)
	service, err := repo.FindServiceAtRevision(executor.cfg.RepoDir, record.RepoRevision, params.ServiceDir, executor.availableNodeIDs)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}
	client, err := executor.dnsProviders.Cloudflare(executor.cfg)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}
	desired, err := buildDesiredServiceDNS(ctx, service, executor.cfg, client)
	if err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}
	if err := appendTaskLogRaw(record.LogPath, fmt.Sprintf("resolved dns target hostname=%s zone=%s\n", desired.FQDN, desired.Zone)); err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}
	if err := syncServiceDNS(ctx, client, desired, record.LogPath); err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}
	finishedAt := time.Now().UTC()
	if err := executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: task.StepDNSUpdate, Status: task.StatusSucceeded, StartedAt: &startedAt, FinishedAt: &finishedAt}); err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}
	if err := appendTaskLogRaw(record.LogPath, "dns_update task finished successfully\n"); err != nil {
		return executor.failControllerTask(ctx, record, task.StepDNSUpdate, err)
	}
	return executor.db.CompleteTask(ctx, record.TaskID, task.StatusSucceeded, finishedAt, "")
}

func (executor *controllerTaskExecutor) failControllerTask(ctx context.Context, record task.Record, stepName task.StepName, taskErr error) error {
	finishedAt := time.Now().UTC()
	_ = executor.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: record.TaskID, StepName: stepName, Status: task.StatusFailed, FinishedAt: &finishedAt})
	_ = appendTaskLogRaw(record.LogPath, fmt.Sprintf("task failed: %v\n", taskErr))
	if completeErr := executor.db.CompleteTask(ctx, record.TaskID, task.StatusFailed, finishedAt, taskErr.Error()); completeErr != nil {
		return completeErr
	}
	return taskErr
}

func buildDesiredServiceDNS(ctx context.Context, service repo.Service, cfg *config.ControllerConfig, client cloudflareDNSClient) (desiredServiceDNS, error) {
	if service.Meta.Network == nil || service.Meta.Network.DNS == nil {
		return desiredServiceDNS{}, fmt.Errorf("service %q does not declare network.dns", service.Name)
	}
	dnsConfig := service.Meta.Network.DNS
	if dnsConfig.Provider != "cloudflare" {
		return desiredServiceDNS{}, fmt.Errorf("service %q dns provider %q is not implemented", service.Name, dnsConfig.Provider)
	}
	hostname := strings.TrimSpace(dnsConfig.Hostname)
	if hostname == "" {
		return desiredServiceDNS{}, fmt.Errorf("service %q network.dns.hostname is required", service.Name)
	}
	fqdn := ensureTrailingDot(hostname)
	zone, err := matchingZone(ctx, client, fqdn)
	if err != nil {
		return desiredServiceDNS{}, err
	}
	relativeName := libdns.RelativeName(fqdn, zone)
	if relativeName == fqdn || relativeName == "" {
		return desiredServiceDNS{}, fmt.Errorf("hostname %q does not belong to a known Cloudflare zone", hostname)
	}
	recordSets, err := desiredDNSRecordSets(service, cfg, relativeName)
	if err != nil {
		return desiredServiceDNS{}, err
	}
	return desiredServiceDNS{
		Zone:       zone,
		FQDN:       fqdn,
		RecordSets: recordSets,
		Options: dnsRecordOptions{
			Proxied: dnsConfig.Proxied,
			Comment: strings.TrimSpace(dnsConfig.Comment),
		},
	}, nil
}

func matchingZone(ctx context.Context, client cloudflareDNSClient, fqdn string) (string, error) {
	zones, err := client.ListZones(ctx)
	if err != nil {
		return "", err
	}
	sort.Slice(zones, func(left, right int) bool {
		return len(zones[left].Name) > len(zones[right].Name)
	})
	for _, zone := range zones {
		name := ensureTrailingDot(zone.Name)
		if strings.HasSuffix(strings.ToLower(fqdn), strings.ToLower(name)) {
			return name, nil
		}
	}
	return "", fmt.Errorf("no Cloudflare zone matched hostname %q", fqdn)
}

func desiredDNSRecordSets(service repo.Service, cfg *config.ControllerConfig, relativeName string) (map[string][]libdns.Record, error) {
	dnsConfig := service.Meta.Network.DNS
	value := strings.TrimSpace(dnsConfig.Value)
	ttl := time.Duration(0)
	if dnsConfig.TTL != nil {
		ttl = time.Duration(*dnsConfig.TTL) * time.Second
	}
	if value != "" {
		return desiredDNSRecordSetsForValue(relativeName, ttl, strings.ToUpper(strings.TrimSpace(dnsConfig.RecordType)), value)
	}
	if len(service.TargetNodes) != 1 {
		return nil, fmt.Errorf("service %q must target exactly one node when network.dns.value is empty", service.Name)
	}
	node, err := configuredNode(cfg, service.TargetNodes[0])
	if err != nil {
		return nil, err
	}
	return desiredDNSRecordSetsForNode(relativeName, ttl, strings.ToUpper(strings.TrimSpace(dnsConfig.RecordType)), node)
}

func desiredDNSRecordSetsForValue(relativeName string, ttl time.Duration, recordType, value string) (map[string][]libdns.Record, error) {
	recordSets := make(map[string][]libdns.Record)
	if addr, err := netip.ParseAddr(value); err == nil {
		switch recordType {
		case "", "A", "AAAA":
			if addr.Is4() {
				if recordType == "AAAA" {
					return nil, fmt.Errorf("record_type AAAA requires an IPv6 value")
				}
				recordSets["A"] = []libdns.Record{libdns.Address{Name: relativeName, TTL: ttl, IP: addr}}
				return recordSets, nil
			}
			if recordType == "A" {
				return nil, fmt.Errorf("record_type A requires an IPv4 value")
			}
			recordSets["AAAA"] = []libdns.Record{libdns.Address{Name: relativeName, TTL: ttl, IP: addr}}
			return recordSets, nil
		case "CNAME":
			return nil, fmt.Errorf("record_type CNAME requires a hostname value")
		default:
			return nil, fmt.Errorf("unsupported dns record_type %q", recordType)
		}
	}

	target := ensureTrailingDot(value)
	switch recordType {
	case "", "CNAME":
		recordSets["CNAME"] = []libdns.Record{libdns.CNAME{Name: relativeName, TTL: ttl, Target: target}}
		return recordSets, nil
	case "A", "AAAA":
		return nil, fmt.Errorf("record_type %s requires an IP value", recordType)
	default:
		return nil, fmt.Errorf("unsupported dns record_type %q", recordType)
	}
}

func desiredDNSRecordSetsForNode(relativeName string, ttl time.Duration, recordType string, node config.NodeConfig) (map[string][]libdns.Record, error) {
	recordSets := make(map[string][]libdns.Record)
	switch recordType {
	case "":
		if node.PublicIPv4 != "" {
			addr, err := netip.ParseAddr(node.PublicIPv4)
			if err != nil {
				return nil, fmt.Errorf("node %q public_ipv4 is invalid: %w", node.ID, err)
			}
			recordSets["A"] = []libdns.Record{libdns.Address{Name: relativeName, TTL: ttl, IP: addr}}
		}
		if node.PublicIPv6 != "" {
			addr, err := netip.ParseAddr(node.PublicIPv6)
			if err != nil {
				return nil, fmt.Errorf("node %q public_ipv6 is invalid: %w", node.ID, err)
			}
			recordSets["AAAA"] = []libdns.Record{libdns.Address{Name: relativeName, TTL: ttl, IP: addr}}
		}
		if len(recordSets) == 0 {
			return nil, fmt.Errorf("service node %q does not have a public IPv4 or IPv6 address", node.ID)
		}
		return recordSets, nil
	case "A":
		if node.PublicIPv4 == "" {
			return nil, fmt.Errorf("service node %q does not have public_ipv4", node.ID)
		}
		addr, err := netip.ParseAddr(node.PublicIPv4)
		if err != nil {
			return nil, fmt.Errorf("node %q public_ipv4 is invalid: %w", node.ID, err)
		}
		recordSets["A"] = []libdns.Record{libdns.Address{Name: relativeName, TTL: ttl, IP: addr}}
		return recordSets, nil
	case "AAAA":
		if node.PublicIPv6 == "" {
			return nil, fmt.Errorf("service node %q does not have public_ipv6", node.ID)
		}
		addr, err := netip.ParseAddr(node.PublicIPv6)
		if err != nil {
			return nil, fmt.Errorf("node %q public_ipv6 is invalid: %w", node.ID, err)
		}
		recordSets["AAAA"] = []libdns.Record{libdns.Address{Name: relativeName, TTL: ttl, IP: addr}}
		return recordSets, nil
	case "CNAME":
		return nil, errors.New("record_type CNAME requires an explicit value")
	default:
		return nil, fmt.Errorf("unsupported dns record_type %q", recordType)
	}
}

func syncServiceDNS(ctx context.Context, client cloudflareDNSClient, desired desiredServiceDNS, logPath string) error {
	staleTypes := make([]string, 0, len(managedDNSRecordTypes))
	for _, recordType := range managedDNSRecordTypes {
		if _, ok := desired.RecordSets[recordType]; !ok {
			staleTypes = append(staleTypes, recordType)
		}
	}
	sort.Strings(staleTypes)
	for _, recordType := range staleTypes {
		if err := appendTaskLogRaw(logPath, fmt.Sprintf("deleting stale %s records for %s\n", recordType, desired.FQDN)); err != nil {
			return err
		}
		if _, err := client.DeleteRecords(ctx, desired.Zone, []libdns.Record{libdns.RR{Name: libdns.RelativeName(desired.FQDN, desired.Zone), Type: recordType}}); err != nil {
			return err
		}
	}
	writableTypes := make([]string, 0, len(desired.RecordSets))
	for recordType := range desired.RecordSets {
		writableTypes = append(writableTypes, recordType)
	}
	sort.Strings(writableTypes)
	for _, recordType := range writableTypes {
		records := desired.RecordSets[recordType]
		if err := appendTaskLogRaw(logPath, fmt.Sprintf("setting %s records for %s (%d record(s))\n", recordType, desired.FQDN, len(records))); err != nil {
			return err
		}
		if _, err := client.SetRecords(ctx, desired.Zone, records); err != nil {
			return err
		}
		if err := client.ApplyRecordOptions(ctx, desired.Zone, desired.FQDN, recordType, desired.Options); err != nil {
			return err
		}
	}
	return nil
}

func (defaultDNSProviderFactory) Cloudflare(cfg *config.ControllerConfig) (cloudflareDNSClient, error) {
	if cfg == nil || cfg.DNS == nil || cfg.DNS.Cloudflare == nil {
		return nil, errors.New("controller dns.cloudflare is not configured")
	}
	content, err := os.ReadFile(cfg.DNS.Cloudflare.APITokenFile)
	if err != nil {
		return nil, fmt.Errorf("read cloudflare api token file: %w", err)
	}
	token := strings.TrimSpace(string(content))
	if token == "" {
		return nil, errors.New("cloudflare api token file is empty")
	}
	provider := &cloudflarelibdns.Provider{APIToken: token}
	return &defaultCloudflareDNSClient{
		provider:   provider,
		apiToken:   token,
		apiBaseURL: cloudflareAPIBaseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (client *defaultCloudflareDNSClient) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	return client.provider.ListZones(ctx)
}

func (client *defaultCloudflareDNSClient) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return client.provider.SetRecords(ctx, zone, records)
}

func (client *defaultCloudflareDNSClient) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return client.provider.DeleteRecords(ctx, zone, records)
}

func (client *defaultCloudflareDNSClient) ApplyRecordOptions(ctx context.Context, zone, fqdn, recordType string, options dnsRecordOptions) error {
	if options.Proxied == nil && options.Comment == "" {
		return nil
	}
	zoneID, err := client.lookupZoneID(ctx, zone)
	if err != nil {
		return err
	}
	recordIDs, err := client.lookupRecordIDs(ctx, zoneID, fqdn, recordType)
	if err != nil {
		return err
	}
	if len(recordIDs) == 0 {
		return fmt.Errorf("no Cloudflare %s records found for %s after sync", recordType, fqdn)
	}
	payload := map[string]any{}
	if options.Proxied != nil {
		payload["proxied"] = *options.Proxied
	}
	if options.Comment != "" {
		payload["comment"] = options.Comment
	}
	if len(payload) == 0 {
		return nil
	}
	for _, recordID := range recordIDs {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("%s/zones/%s/dns_records/%s", client.apiBaseURL, zoneID, recordID), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+client.apiToken)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.httpClient.Do(req)
		if err != nil {
			return err
		}
		response.Body.Close()
		if response.StatusCode >= 300 {
			return fmt.Errorf("patch Cloudflare dns record %s: unexpected status %s", recordID, response.Status)
		}
	}
	return nil
}

func (client *defaultCloudflareDNSClient) lookupZoneID(ctx context.Context, zone string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/zones?name=%s", client.apiBaseURL, strings.TrimSuffix(zone, ".")), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+client.apiToken)
	var payload cloudflareZoneResponse
	if err := client.doJSON(req, &payload); err != nil {
		return "", err
	}
	if len(payload.Result) == 0 {
		return "", fmt.Errorf("Cloudflare zone %q was not found", zone)
	}
	return payload.Result[0].ID, nil
}

func (client *defaultCloudflareDNSClient) lookupRecordIDs(ctx context.Context, zoneID, fqdn, recordType string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/zones/%s/dns_records?name=%s&type=%s", client.apiBaseURL, zoneID, strings.TrimSuffix(fqdn, "."), recordType), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+client.apiToken)
	var payload cloudflareRecordResponse
	if err := client.doJSON(req, &payload); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(payload.Result))
	for _, record := range payload.Result {
		ids = append(ids, record.ID)
	}
	return ids, nil
}

func (client *defaultCloudflareDNSClient) doJSON(req *http.Request, target any) error {
	response, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API request failed with status %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func configuredNode(cfg *config.ControllerConfig, nodeID string) (config.NodeConfig, error) {
	for _, node := range cfg.Nodes {
		if node.ID == nodeID {
			return node, nil
		}
	}
	return config.NodeConfig{}, fmt.Errorf("node %q is not configured", nodeID)
}

func ensureTrailingDot(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasSuffix(trimmed, ".") {
		return trimmed
	}
	return trimmed + "."
}
