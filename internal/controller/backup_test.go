package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
	"forgejo.alexma.top/alexma233/composia/internal/task"
)

func TestBackupRecordServiceListAndGetBackup(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncDeclaredServices(ctx, []string{"alpha", "bravo"}); err != nil {
		t.Fatalf("sync declared services: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-1", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "alpha", CreatedAt: time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-1: %v", err)
	}
	if _, err := db.CreateTask(ctx, task.Record{TaskID: "task-2", Type: task.TypeBackup, Source: task.SourceCLI, ServiceName: "bravo", CreatedAt: time.Date(2026, 4, 4, 14, 5, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("create task-2: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: "backup-1", TaskID: "task-1", ServiceName: "alpha", DataName: "config", Status: "succeeded", StartedAt: "2026-04-04T14:00:00Z", FinishedAt: "2026-04-04T14:01:00Z", ArtifactRef: "snapshot-1"}); err != nil {
		t.Fatalf("insert backup-1: %v", err)
	}
	if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: "backup-2", TaskID: "task-2", ServiceName: "bravo", DataName: "db", Status: "failed", StartedAt: "2026-04-04T14:05:00Z", FinishedAt: "2026-04-04T14:06:00Z"}); err != nil {
		t.Fatalf("insert backup-2: %v", err)
	}

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "cli-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})
	path, handler := controllerv1connect.NewBackupRecordServiceHandler(&backupRecordServer{db: db}, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewBackupRecordServiceClient(httpServer.Client(), httpServer.URL, connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("cli-token")))
	listResponse, err := client.ListBackups(ctx, connect.NewRequest(&controllerv1.ListBackupsRequest{ServiceName: "alpha"}))
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(listResponse.Msg.GetBackups()) != 1 || listResponse.Msg.GetBackups()[0].GetBackupId() != "backup-1" {
		t.Fatalf("unexpected backup list response: %+v", listResponse.Msg.GetBackups())
	}

	getResponse, err := client.GetBackup(ctx, connect.NewRequest(&controllerv1.GetBackupRequest{BackupId: "backup-1"}))
	if err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if getResponse.Msg.GetArtifactRef() != "snapshot-1" || getResponse.Msg.GetServiceName() != "alpha" {
		t.Fatalf("unexpected backup detail response: %+v", getResponse.Msg)
	}
}
