package controller

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/libdns/libdns"

	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/repo"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

type fakeCloudflareDNSClient struct {
	zones      []libdns.Zone
	operations []string
	options    []string
}

func (client *fakeCloudflareDNSClient) ListZones(_ context.Context) ([]libdns.Zone, error) {
	return append([]libdns.Zone(nil), client.zones...), nil
}

func (client *fakeCloudflareDNSClient) SetRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	for _, record := range records {
		rr := record.RR()
		client.operations = append(client.operations, "set "+zone+" "+rr.Type+" "+rr.Name+" "+rr.Data)
	}
	return records, nil
}

func (client *fakeCloudflareDNSClient) DeleteRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	for _, record := range records {
		rr := record.RR()
		client.operations = append(client.operations, "delete "+zone+" "+rr.Type+" "+rr.Name)
	}
	return records, nil
}

func (client *fakeCloudflareDNSClient) ApplyRecordOptions(_ context.Context, zone, fqdn, recordType string, options dnsRecordOptions) error {
	proxied := "nil"
	if options.Proxied != nil {
		if *options.Proxied {
			proxied = "true"
		} else {
			proxied = "false"
		}
	}
	client.options = append(client.options, zone+" "+fqdn+" "+recordType+" proxied="+proxied+" comment="+options.Comment)
	return nil
}

type fakeDNSProviderFactory struct {
	client cloudflareDNSClient
}

func (factory fakeDNSProviderFactory) Cloudflare(_ *config.ControllerConfig) (cloudflareDNSClient, error) {
	return factory.client, nil
}

func TestExecuteDNSUpdateTaskSyncsDualStackRecords(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	repoDir := filepath.Join(rootDir, "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\nnetwork:\n  dns:\n    provider: cloudflare\n    hostname: demo.example.com\n    proxied: true\n",
	})
	logDir := filepath.Join(rootDir, "logs")
	stateDir := filepath.Join(rootDir, "state")
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := syncDeclaredServicesForTests(ctx, db, "demo"); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{
		TaskID:       "task-dns",
		Type:         task.TypeDNSUpdate,
		Source:       task.SourceCLI,
		ServiceName:  "demo",
		NodeID:       "main",
		RepoRevision: currentRevision(t, repoDir),
		ParamsJSON:   `{"service_dir":"demo"}`,
		LogPath:      filepath.Join(logDir, "tasks", "task-dns.log"),
		CreatedAt:    time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create dns task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "tasks", "task-dns.log"), nil, 0o644); err != nil {
		t.Fatalf("create task log: %v", err)
	}

	fakeClient := &fakeCloudflareDNSClient{zones: []libdns.Zone{{Name: "example.com."}}}
	executor := &controllerTaskExecutor{
		db:               db,
		cfg:              &config.ControllerConfig{RepoDir: repoDir, LogDir: logDir, Nodes: []config.NodeConfig{{ID: "main", PublicIPv4: "203.0.113.10", PublicIPv6: "2001:db8::10"}}},
		availableNodeIDs: map[string]struct{}{"main": {}},
		dnsProviders:     fakeDNSProviderFactory{client: fakeClient},
	}

	record, err := db.GetTask(ctx, "task-dns")
	if err != nil {
		t.Fatalf("get dns task: %v", err)
	}
	if err := executor.executeDNSUpdateTask(ctx, record.Record); err != nil {
		t.Fatalf("execute dns update task: %v", err)
	}

	detail, err := db.GetTask(ctx, "task-dns")
	if err != nil {
		t.Fatalf("reload dns task: %v", err)
	}
	if detail.Record.Status != task.StatusSucceeded {
		t.Fatalf("expected succeeded dns task, got %q", detail.Record.Status)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].StepName != task.StepDNSUpdate || detail.Steps[0].Status != task.StatusSucceeded {
		t.Fatalf("unexpected dns task steps: %+v", detail.Steps)
	}
	expectedOps := []string{
		"delete example.com. CNAME demo",
		"set example.com. A demo 203.0.113.10",
		"set example.com. AAAA demo 2001:db8::10",
	}
	if strings.Join(fakeClient.operations, "|") != strings.Join(expectedOps, "|") {
		t.Fatalf("unexpected dns operations: %+v", fakeClient.operations)
	}
	expectedOptions := []string{
		"example.com. demo.example.com. A proxied=true comment=",
		"example.com. demo.example.com. AAAA proxied=true comment=",
	}
	if strings.Join(fakeClient.options, "|") != strings.Join(expectedOptions, "|") {
		t.Fatalf("unexpected dns options: %+v", fakeClient.options)
	}
	logContent, err := os.ReadFile(filepath.Join(logDir, "tasks", "task-dns.log"))
	if err != nil {
		t.Fatalf("read dns task log: %v", err)
	}
	if !strings.Contains(string(logContent), "resolved dns target hostname=demo.example.com. zone=example.com.") {
		t.Fatalf("expected dns task log to include zone resolution, got %q", string(logContent))
	}
}

func currentRevision(t *testing.T, repoDir string) string {
	t.Helper()
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("read current revision: %v", err)
	}
	return revision
}
