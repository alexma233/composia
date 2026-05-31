package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	backupcfg "forgejo.alexma.top/alexma233/composia/internal/core/backup"
)

func TestLoadRestoreRuntimeConfig(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	writeAgentTestFile(t, filepath.Join(serviceRoot, ".composia-restore.json"), `{
  "rustic": {"service_dir": "infra/rustic", "node_id": "main"},
  "items": [{"name": "config", "strategy": "files.copy", "artifact_ref": "snap:config"}]
}`)

	cfg, err := loadRestoreRuntimeConfig(serviceRoot)
	if err != nil {
		t.Fatalf("loadRestoreRuntimeConfig returned error: %v", err)
	}
	if cfg.Rustic.ServiceDir != "infra/rustic" || len(cfg.Items) != 1 || cfg.Items[0].ArtifactRef != "snap:config" {
		t.Fatalf("unexpected restore config: %+v", cfg)
	}
}

func TestLoadRestoreRuntimeConfigRejectsMissingItems(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	writeAgentTestFile(t, filepath.Join(serviceRoot, ".composia-restore.json"), `{"rustic":{"service_dir":"infra/rustic"},"items":[]}`)

	_, err := loadRestoreRuntimeConfig(serviceRoot)
	if err == nil || !strings.Contains(err.Error(), "did not include any items") {
		t.Fatalf("expected missing items error, got %v", err)
	}
}

func TestPrepareRestoreVolumeFlagsRecreatesServicePathTargets(t *testing.T) {
	t.Parallel()

	serviceRoot := t.TempDir()
	stagingDir := t.TempDir()
	containerStagingDir := "/var/lib/composia/data-protect"
	configDir := filepath.Join(serviceRoot, "config")
	dataFile := filepath.Join(serviceRoot, "data.txt")
	writeAgentTestFile(t, filepath.Join(configDir, "old.txt"), "old")
	writeAgentTestFile(t, dataFile, "old data")

	flags, err := prepareRestoreVolumeFlags(context.Background(), serviceRoot, stagingDir, containerStagingDir, backupcfg.RestoreItem{
		Name:     "data",
		Strategy: "files.copy",
		Include:  []string{"./config", "./data.txt"},
	})
	if err != nil {
		t.Fatalf("prepareRestoreVolumeFlags returned error: %v", err)
	}

	if _, err := os.Stat(configDir); err != nil {
		t.Fatalf("expected config dir to be recreated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected old config file to be removed, stat err=%v", err)
	}
	if got := readAgentTestFile(t, dataFile); got != "" {
		t.Fatalf("expected data file to be truncated, got %q", got)
	}

	wantFlags := []string{
		"-v", configDir + ":" + filepath.Join(containerStagingDir, "data", "paths", "config"),
		"-v", dataFile + ":" + filepath.Join(containerStagingDir, "data", "paths", "data.txt"),
	}
	if strings.Join(flags, "\n") != strings.Join(wantFlags, "\n") {
		t.Fatalf("flags = %+v, want %+v", flags, wantFlags)
	}
}

func TestPrepareRestoreVolumeFlagsRejectsMissingServicePathTarget(t *testing.T) {
	t.Parallel()

	_, err := prepareRestoreVolumeFlags(context.Background(), t.TempDir(), t.TempDir(), "/stage", backupcfg.RestoreItem{
		Name:    "data",
		Include: []string{"./missing"},
	})
	if err == nil || !strings.Contains(err.Error(), "must exist for direct restore") {
		t.Fatalf("expected missing target error, got %v", err)
	}
}

func writeAgentTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
