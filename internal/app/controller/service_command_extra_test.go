package controller

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestEffectiveImageAutoApplyPrecedence(t *testing.T) {
	t.Parallel()

	cfg := &config.ControllerConfig{Updates: &config.ControllerUpdatesConfig{AutoApply: boolPtr(true)}}
	update := &repo.UpdateConfig{AutoApply: boolPtr(false)}
	image := repo.ImageUpdateConfig{}
	if effectiveImageAutoApply(cfg, update, image) {
		t.Fatalf("update-level false should override controller default")
	}
	image.AutoApply = boolPtr(true)
	if !effectiveImageAutoApply(cfg, update, image) {
		t.Fatalf("image-level true should override update-level false")
	}
	if effectiveImageAutoApply(nil, nil, repo.ImageUpdateConfig{}) {
		t.Fatalf("default auto apply should be false")
	}
}

func TestEffectiveBackupBeforeUpdatePrecedence(t *testing.T) {
	t.Parallel()

	cfg := &config.ControllerConfig{Updates: &config.ControllerUpdatesConfig{BackupBeforeUpdate: boolPtr(true)}}
	update := &repo.UpdateConfig{BackupBeforeUpdate: boolPtr(false)}
	image := &repo.ImageUpdateConfig{BackupBeforeUpdate: boolPtr(true)}
	requestOverride := false
	if effectiveBackupBeforeUpdate(cfg, update, image, &requestOverride) {
		t.Fatalf("request override should win")
	}
	if !effectiveBackupBeforeUpdate(cfg, update, image, nil) {
		t.Fatalf("image-level true should win without request override")
	}
	image.BackupBeforeUpdate = nil
	if effectiveBackupBeforeUpdate(cfg, update, image, nil) {
		t.Fatalf("update-level false should override controller default")
	}
}

func TestServiceImageUpdatesNeedBackupUsesSelectionsAndPlans(t *testing.T) {
	t.Parallel()

	images := map[string]repo.ImageUpdateConfig{
		"api":    {BackupBeforeUpdate: boolPtr(false)},
		"worker": {BackupBeforeUpdate: boolPtr(true)},
	}
	if !serviceImageUpdatesNeedBackup(nil, nil, images, nil, []*controllerv1.ImageUpdateSelection{{ImageName: "worker"}}, nil) {
		t.Fatalf("expected selected worker to require backup")
	}
	if serviceImageUpdatesNeedBackup(nil, nil, images, []plannedImageUpdate{{ImageName: "api"}}, nil, nil) {
		t.Fatalf("api plan should not require backup")
	}
}

func TestRunBackupsBeforeUpdateSkipsServiceWithoutBackupConfig(t *testing.T) {
	t.Parallel()

	server := &serviceCommandServer{}
	for _, service := range []repo.Service{
		{Name: "app"},
		{Name: "app", Meta: repo.ServiceMeta{Backup: &repo.BackupConfig{Data: []repo.BackupItem{{Name: "config", Enabled: boolPtr(false)}}}}},
	} {
		if err := server.runBackupsBeforeUpdate(context.Background(), service, []string{"main"}, task.SourceCLI, ""); err != nil {
			t.Fatalf("run backups before update: %v", err)
		}
	}
}

func TestRunServiceUpdateValidatesRepoWriteBeforeBackup(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, baseRevision, commitMessage, missingField string
	}{
		{name: "revision", commitMessage: "update images", missingField: "base_revision"},
		{name: "message", baseRevision: "revision", missingField: "commit_message"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openControllerTestDB(t)
			defer func() { _ = db.Close() }()
			repoDir := filepath.Join(t.TempDir(), "repo")
			createGitRepoWithContent(t, repoDir, map[string]string{
				"app/composia-meta.yaml": "name: app\nnodes:\n  - main\ndata_protect:\n  data:\n    - name: db\n      backup:\n        strategy: files.copy\n        include:\n          - ./db\nbackup:\n  data:\n    - name: db\nupdate:\n  backup_before_update: true\n  images:\n    api:\n      image: ghcr.io/example/api\n      current:\n        env:\n          file: .env\n          key: API_VERSION\n      discovery:\n        sources:\n          - type: auto\n      filter:\n        type: semver\n",
				"app/.env":               "API_VERSION=1.2.3\n",
			})
			nodes := map[string]struct{}{"main": {}}
			service, err := repo.FindService(repoDir, nodes, "app")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
				t.Fatal(err)
			}
			if err := syncDeclaredServicesForTests(ctx, db, "app"); err != nil {
				t.Fatal(err)
			}
			server := &serviceCommandServer{db: db, cfg: &config.ControllerConfig{RepoDir: repoDir, LogDir: t.TempDir()}, availableNodeIDs: nodes}
			_, _, err = server.runServiceUpdateWithImageSelections(ctx, service, nil, []*controllerv1.ImageUpdateSelection{{ImageName: "api", TargetTag: "1.3.0"}}, false, nil, test.baseRevision, test.commitMessage, task.SourceWeb, "", "")
			if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), test.missingField) {
				t.Fatalf("expected missing %s error before backup, got %v", test.missingField, err)
			}
			_, count, err := db.ListTasks(context.Background(), nil, nil, nil, nil, nil, nil, nil, nil, 1, 10)
			if err != nil || count != 0 {
				t.Fatalf("invalid update must not create tasks: count=%d err=%v", count, err)
			}
		})
	}
}

func TestResolveUpdateBackupDataNames(t *testing.T) {
	t.Parallel()

	service := repo.Service{
		Name: "app",
		Meta: repo.ServiceMeta{
			Backup: &repo.BackupConfig{Data: []repo.BackupItem{{Name: "config"}, {Name: "db"}}},
			Update: &repo.UpdateConfig{BackupData: []repo.UpdateBackupDataItem{{Name: "db"}, {Name: "config", Enabled: boolPtr(false)}}},
		},
	}
	names, err := resolveUpdateBackupDataNames(service)
	if err != nil {
		t.Fatalf("resolve update backup data names: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"db"}) {
		t.Fatalf("names = %+v", names)
	}

	service.Meta.Update.BackupData = []repo.UpdateBackupDataItem{{Name: "config", Enabled: boolPtr(false)}}
	_, err = resolveUpdateBackupDataNames(service)
	if err == nil || !strings.Contains(err.Error(), "does not have any enabled update backup data items") {
		t.Fatalf("expected disabled update backup data error, got %v", err)
	}

	service.Meta.Update.BackupData = []repo.UpdateBackupDataItem{{Name: "missing"}}
	_, err = resolveUpdateBackupDataNames(service)
	if err == nil || !strings.Contains(err.Error(), "backup data \"missing\" is not enabled") {
		t.Fatalf("expected invalid backup data error, got %v", err)
	}
}

func TestApplyImageCurrentUpdateRejectsMissingSource(t *testing.T) {
	t.Parallel()

	_, err := applyImageCurrentUpdate("content", repo.ImageUpdateCurrent{}, "ghcr.io/example/app", "1.2.3")
	if err == nil || !strings.Contains(err.Error(), "current must specify env or yaml") {
		t.Fatalf("expected current source error, got %v", err)
	}
}

func TestApplyPlannedServiceImageUpdatesAcceptsPersistedRetry(t *testing.T) {
	t.Parallel()
	repoDir := filepath.Join(t.TempDir(), "repo")
	createGitRepoWithContent(t, repoDir, map[string]string{
		"demo/composia-meta.yaml": "name: demo\nnodes:\n  - main\nupdate:\n  images:\n    api:\n      image: ghcr.io/example/api\n      digest_pin: false\n      current:\n        env:\n          file: .env\n          key: API_VERSION\n      discovery:\n        sources:\n          - type: auto\n      filter:\n        type: semver\n",
		"demo/.env":               "API_VERSION=1.3.0\n",
	})
	service, err := repo.FindService(repoDir, map[string]struct{}{"main": {}}, "demo")
	if err != nil {
		t.Fatal(err)
	}
	revision, err := repo.CurrentRevision(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	server := &serviceCommandServer{cfg: &config.ControllerConfig{RepoDir: repoDir}, availableNodeIDs: map[string]struct{}{"main": {}}}
	result, err := server.applyPlannedServiceImageUpdates(context.Background(), service, []plannedImageUpdate{{ImageName: "api", Tag: "1.3.0", RepoBacked: true}}, revision, "retry image update", true)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.CommitID != "" {
		t.Fatalf("expected an idempotent no-op result, got %+v", result)
	}
}
