package controller

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/libdns/libdns"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

type fakeDNSClient struct {
	zones      []libdns.Zone
	operations []string
	options    []string
}

func (client *fakeDNSClient) ListZones(_ context.Context) ([]libdns.Zone, error) {
	return append([]libdns.Zone(nil), client.zones...), nil
}

func (client *fakeDNSClient) SetRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	for _, record := range records {
		rr := record.RR()
		client.operations = append(client.operations, "set "+zone+" "+rr.Type+" "+rr.Name+" "+rr.Data)
	}
	return records, nil
}

func (client *fakeDNSClient) DeleteRecords(_ context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	for _, record := range records {
		rr := record.RR()
		client.operations = append(client.operations, "delete "+zone+" "+rr.Type+" "+rr.Name)
	}
	return records, nil
}

func (client *fakeDNSClient) ApplyRecordOptions(_ context.Context, zone, fqdn, recordType string, options dnsRecordOptions) error {
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
	client dnsClient
}

func (factory fakeDNSProviderFactory) ForService(_ *config.ControllerConfig, _ string) (dnsClient, error) {
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
	if err := os.MkdirAll(filepath.Join(logDir, "tasks"), 0o750); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	db, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() { _ = db.Close() }()

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
	if err := os.WriteFile(filepath.Join(logDir, "tasks", "task-dns.log"), nil, 0o600); err != nil {
		t.Fatalf("create task log: %v", err)
	}

	fakeClient := &fakeDNSClient{zones: []libdns.Zone{{Name: "example.com."}}}
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
	logContent, err := os.ReadFile(filepath.Join(logDir, "tasks", "task-dns.log")) //nolint:gosec
	if err != nil {
		t.Fatalf("read dns task log: %v", err)
	}
	if !strings.Contains(string(logContent), "resolved dns target hostname=demo.example.com. zone=example.com.") {
		t.Fatalf("expected dns task log to include zone resolution, got %q", string(logContent))
	}
}

func TestFailControllerTaskNotifiesWaiters(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"demo": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "task.log")
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-controller", Type: task.TypeDNSUpdate, Source: task.SourceCLI, ServiceName: "demo", NodeID: "main", Status: task.StatusRunning, LogPath: logPath, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create controller task: %v", err)
	}

	notifier := newTaskResultNotifier()
	waitCh := notifier.Subscribe("task-controller")
	defer notifier.Unsubscribe("task-controller", waitCh)
	executor := &controllerTaskExecutor{db: db, taskResults: notifier}
	if err := executor.failControllerTask(ctx, task.Record{TaskID: "task-controller", LogPath: logPath}, task.StepDNSUpdate, errors.New("boom")); err == nil {
		t.Fatal("expected task failure error")
	}

	select {
	case <-waitCh:
	case <-time.After(time.Second):
		t.Fatal("expected controller task waiter notification")
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
