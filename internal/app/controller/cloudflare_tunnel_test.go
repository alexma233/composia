package controller

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libdns/libdns"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

type fakeCloudflareTunnelClient struct {
	updates []fakeCloudflareTunnelUpdate
}

type fakeCloudflareTunnelUpdate struct {
	accountID string
	tunnelID  string
	ingress   []cloudflareTunnelIngress
}

func (client *fakeCloudflareTunnelClient) UpdateConfiguration(_ context.Context, accountID, tunnelID string, ingress []cloudflareTunnelIngress) error {
	client.updates = append(client.updates, fakeCloudflareTunnelUpdate{accountID: accountID, tunnelID: tunnelID, ingress: append([]cloudflareTunnelIngress(nil), ingress...)})
	return nil
}

type fakeCloudflareTunnelRuntimeStore struct {
	runningCounts map[string]uint32
}

func (runtimeStore fakeCloudflareTunnelRuntimeStore) GetServiceSnapshot(_ context.Context, serviceName string) (store.ServiceSnapshot, error) {
	runningCount, exists := runtimeStore.runningCounts[serviceName]
	if !exists {
		return store.ServiceSnapshot{}, store.ErrServiceNotFound
	}
	return store.ServiceSnapshot{Name: serviceName, RunningCount: runningCount}, nil
}

func TestSyncCloudflareTunnelsWritesRemoteConfigAndDNS(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"app/composia-meta.yaml": "name: app\nnodes:\n  - main\nnetwork:\n  cloudflare_tunnel:\n    hostname: app.example.com\n    service: http://app:8080\n    origin_request:\n      no_tls_verify: true\n      http_host_header: app.internal\n",
	})
	logPath := filepath.Join(rootDir, "task.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatalf("create task log: %v", err)
	}

	tunnelClient := &fakeCloudflareTunnelClient{}
	dnsClient := &fakeDNSClient{zones: []libdns.Zone{{Name: "example.com."}}}
	cfg := &config.ControllerConfig{
		RepoDir: repoDir,
		CloudflareTunnel: &config.ControllerCloudflareTunnelConfig{
			AccountID: "account-1",
			Tunnels: map[string]config.ControllerCloudflareTunnel{
				"edge": {TunnelID: "11111111-1111-1111-1111-111111111111"},
			},
			Nodes: map[string]config.ControllerCloudflareTunnelNodeMap{
				"main": {Tunnel: "edge"},
			},
		},
	}

	if err := syncCloudflareTunnels(context.Background(), cfg, map[string]struct{}{"main": {}}, tunnelClient, dnsClient, nil, currentRevision(t, repoDir), "", logPath); err != nil {
		t.Fatalf("sync cloudflare tunnel: %v", err)
	}

	if len(tunnelClient.updates) != 1 {
		t.Fatalf("expected one tunnel update, got %+v", tunnelClient.updates)
	}
	update := tunnelClient.updates[0]
	if update.accountID != "account-1" || update.tunnelID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected tunnel target: %+v", update)
	}
	if len(update.ingress) != 2 {
		t.Fatalf("expected service ingress and fallback, got %+v", update.ingress)
	}
	if update.ingress[0].Hostname != "app.example.com" || update.ingress[0].Service != "http://app:8080" {
		t.Fatalf("unexpected service ingress: %+v", update.ingress[0])
	}
	if update.ingress[0].OriginRequest == nil || update.ingress[0].OriginRequest.NoTLSVerify == nil || !*update.ingress[0].OriginRequest.NoTLSVerify || update.ingress[0].OriginRequest.HTTPHostHeader != "app.internal" {
		t.Fatalf("unexpected origin request: %+v", update.ingress[0].OriginRequest)
	}
	if update.ingress[1].Service != "http_status:404" {
		t.Fatalf("unexpected fallback ingress: %+v", update.ingress[1])
	}
	expectedOps := []string{"set example.com. CNAME app 11111111-1111-1111-1111-111111111111.cfargotunnel.com."}
	if strings.Join(dnsClient.operations, "|") != strings.Join(expectedOps, "|") {
		t.Fatalf("unexpected DNS operations: %+v", dnsClient.operations)
	}
}

func TestSyncCloudflareTunnelsIgnoresNonTopLevelServiceMeta(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"app/composia-meta.yaml":      "name: app\nnodes:\n  - main\nnetwork:\n  cloudflare_tunnel:\n    hostname: app.example.com\n    service: http://app:8080\n",
		"composia-meta.yaml":          "name: root\nnodes:\n  - main\nnetwork:\n  cloudflare_tunnel:\n    hostname: app.example.com\n    service: http://root:8080\n",
		"apps/api/composia-meta.yaml": "name: nested\nnodes:\n  - main\nnetwork:\n  cloudflare_tunnel:\n    hostname: app.example.com\n    service: http://nested:8080\n",
	})
	logPath := filepath.Join(rootDir, "task.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatalf("create task log: %v", err)
	}

	tunnelClient := &fakeCloudflareTunnelClient{}
	dnsClient := &fakeDNSClient{zones: []libdns.Zone{{Name: "example.com."}}}
	cfg := &config.ControllerConfig{
		RepoDir: repoDir,
		CloudflareTunnel: &config.ControllerCloudflareTunnelConfig{
			AccountID: "account-1",
			Tunnels: map[string]config.ControllerCloudflareTunnel{
				"edge": {TunnelID: "11111111-1111-1111-1111-111111111111"},
			},
			Nodes: map[string]config.ControllerCloudflareTunnelNodeMap{
				"main": {Tunnel: "edge"},
			},
		},
	}

	if err := syncCloudflareTunnels(context.Background(), cfg, map[string]struct{}{"main": {}}, tunnelClient, dnsClient, nil, currentRevision(t, repoDir), "", logPath); err != nil {
		t.Fatalf("sync cloudflare tunnel: %v", err)
	}
	if len(tunnelClient.updates) != 1 {
		t.Fatalf("expected one tunnel update, got %+v", tunnelClient.updates)
	}
	ingress := tunnelClient.updates[0].ingress
	if len(ingress) != 2 || ingress[0].Hostname != "app.example.com" || ingress[0].Service != "http://app:8080" || ingress[1].Service != "http_status:404" {
		t.Fatalf("expected only top-level app ingress and fallback, got %+v", ingress)
	}
	expectedOps := []string{"set example.com. CNAME app 11111111-1111-1111-1111-111111111111.cfargotunnel.com."}
	if strings.Join(dnsClient.operations, "|") != strings.Join(expectedOps, "|") {
		t.Fatalf("unexpected DNS operations: %+v", dnsClient.operations)
	}
}

func TestSyncCloudflareTunnelsExcludesStoppedServiceAndDeletesDNS(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"app/composia-meta.yaml": "name: app\nnodes:\n  - main\nnetwork:\n  cloudflare_tunnel:\n    hostname: app.example.com\n    service: http://app:8080\n",
	})
	logPath := filepath.Join(rootDir, "task.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatalf("create task log: %v", err)
	}

	tunnelClient := &fakeCloudflareTunnelClient{}
	dnsClient := &fakeDNSClient{zones: []libdns.Zone{{Name: "example.com."}}}
	cfg := &config.ControllerConfig{
		RepoDir: repoDir,
		CloudflareTunnel: &config.ControllerCloudflareTunnelConfig{
			AccountID: "account-1",
			Tunnels: map[string]config.ControllerCloudflareTunnel{
				"edge": {TunnelID: "11111111-1111-1111-1111-111111111111"},
			},
			Nodes: map[string]config.ControllerCloudflareTunnelNodeMap{
				"main": {Tunnel: "edge"},
			},
		},
	}

	if err := syncCloudflareTunnels(context.Background(), cfg, map[string]struct{}{"main": {}}, tunnelClient, dnsClient, nil, currentRevision(t, repoDir), "app", logPath); err != nil {
		t.Fatalf("sync cloudflare tunnel: %v", err)
	}
	if len(tunnelClient.updates) != 1 || len(tunnelClient.updates[0].ingress) != 1 || tunnelClient.updates[0].ingress[0].Service != "http_status:404" {
		t.Fatalf("expected only fallback ingress, got %+v", tunnelClient.updates)
	}
	expectedOps := []string{"delete example.com. CNAME app"}
	if strings.Join(dnsClient.operations, "|") != strings.Join(expectedOps, "|") {
		t.Fatalf("unexpected DNS operations: %+v", dnsClient.operations)
	}
}

func TestSyncCloudflareTunnelsSkipsServicesWithoutRunningInstances(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"app/composia-meta.yaml": "name: app\nnodes:\n  - main\nnetwork:\n  cloudflare_tunnel:\n    hostname: app.example.com\n    service: http://app:8080\n",
		"api/composia-meta.yaml": "name: api\nnodes:\n  - main\nnetwork:\n  cloudflare_tunnel:\n    hostname: api.example.com\n    service: http://api:8080\n",
	})
	logPath := filepath.Join(rootDir, "task.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatalf("create task log: %v", err)
	}

	tunnelClient := &fakeCloudflareTunnelClient{}
	dnsClient := &fakeDNSClient{zones: []libdns.Zone{{Name: "example.com."}}}
	cfg := &config.ControllerConfig{
		RepoDir: repoDir,
		CloudflareTunnel: &config.ControllerCloudflareTunnelConfig{
			AccountID: "account-1",
			Tunnels: map[string]config.ControllerCloudflareTunnel{
				"edge": {TunnelID: "11111111-1111-1111-1111-111111111111"},
			},
			Nodes: map[string]config.ControllerCloudflareTunnelNodeMap{
				"main": {Tunnel: "edge"},
			},
		},
	}
	runtimeStore := fakeCloudflareTunnelRuntimeStore{runningCounts: map[string]uint32{"api": 1, "app": 0}}

	if err := syncCloudflareTunnels(context.Background(), cfg, map[string]struct{}{"main": {}}, tunnelClient, dnsClient, runtimeStore, currentRevision(t, repoDir), "", logPath); err != nil {
		t.Fatalf("sync cloudflare tunnel: %v", err)
	}
	if len(tunnelClient.updates) != 1 {
		t.Fatalf("expected one tunnel update, got %+v", tunnelClient.updates)
	}
	ingress := tunnelClient.updates[0].ingress
	if len(ingress) != 2 || ingress[0].Hostname != "api.example.com" || ingress[1].Service != "http_status:404" {
		t.Fatalf("expected running api ingress and fallback, got %+v", ingress)
	}
	expectedOps := []string{"set example.com. CNAME api 11111111-1111-1111-1111-111111111111.cfargotunnel.com."}
	if strings.Join(dnsClient.operations, "|") != strings.Join(expectedOps, "|") {
		t.Fatalf("unexpected DNS operations: %+v", dnsClient.operations)
	}
}
