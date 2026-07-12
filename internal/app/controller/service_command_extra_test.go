package controller

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
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
