package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestMetricsHandlerRequiresBearerToken(t *testing.T) {
	t.Parallel()

	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	mux := http.NewServeMux()
	registerMetricsHandler(mux, db, map[string]string{"metrics-token": "metrics"}, time.Date(2026, 5, 9, 11, 0, 0, 0, time.UTC))
	server := httptest.NewServer(mux)
	defer server.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+metricsPath, nil)
	if err != nil {
		t.Fatalf("create unauthenticated request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request metrics without auth: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, server.URL+metricsPath, nil)
	if err != nil {
		t.Fatalf("create authorized request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer metrics-token")
	authedResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request metrics with auth: %v", err)
	}
	defer func() { _ = authedResp.Body.Close() }()
	if authedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with auth, got %d", authedResp.StatusCode)
	}
	body, err := io.ReadAll(authedResp.Body)
	if err != nil {
		t.Fatalf("read metrics body: %v", err)
	}
	content := string(body)
	if !strings.Contains(content, "composia_controller_info") {
		t.Fatalf("expected controller info metric in response: %s", content)
	}
	if !strings.Contains(content, "composia_node_online{node_id=\"main\"} 1") {
		t.Fatalf("expected node online metric in response: %s", content)
	}
}

func TestMetricsCollectorIncludesTaskAndBackupCounts(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	db, err := store.Open(rootDir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"alpha": {"main"}}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	logPath := filepath.Join(rootDir, "task.log")
	created, err := db.CreateTask(ctx, taskRecordForMetrics(logPath))
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.CompleteTask(ctx, created.TaskID, task.StatusFailed, time.Date(2026, 5, 9, 12, 5, 0, 0, time.UTC), "boom"); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: "backup-1", TaskID: created.TaskID, ServiceName: "alpha", NodeID: "main", DataName: "db", Status: "succeeded", StartedAt: "2026-05-09T12:01:00Z", FinishedAt: "2026-05-09T12:02:00Z"}); err != nil {
		t.Fatalf("upsert backup: %v", err)
	}
	mux := http.NewServeMux()
	registerMetricsHandler(mux, db, map[string]string{"metrics-token": "metrics"}, time.Date(2026, 5, 9, 11, 0, 0, 0, time.UTC))
	server := httptest.NewServer(mux)
	defer server.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+metricsPath, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer metrics-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	content := string(body)
	if !strings.Contains(content, "composia_tasks{") || !strings.Contains(content, "type=\"deploy\"") || !strings.Contains(content, "status=\"failed\"") {
		t.Fatalf("expected task count metric in response: %s", content)
	}
	if !strings.Contains(content, "composia_backups{status=\"succeeded\"} 1") {
		t.Fatalf("expected backup count metric in response: %s", content)
	}
}

func taskRecordForMetrics(logPath string) task.Record {
	return task.Record{
		TaskID:      "task-metrics",
		Type:        task.TypeDeploy,
		Source:      task.SourceSchedule,
		ServiceName: "alpha",
		NodeID:      "main",
		Status:      task.StatusPending,
		LogPath:     logPath,
	}
}
