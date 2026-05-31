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

func TestPrepareRestoreVolumeFlagsClearsDockerVolume(t *testing.T) {
	logFile := installFakeDocker(t)

	flags, err := prepareRestoreVolumeFlags(context.Background(), t.TempDir(), t.TempDir(), "/stage", backupcfg.RestoreItem{
		Name:    "data",
		Include: []string{"app-data"},
	})
	if err != nil {
		t.Fatalf("prepareRestoreVolumeFlags returned error: %v", err)
	}
	if strings.TrimSpace(readAgentTestFile(t, logFile)) != "run --rm -v app-data:/target alpine:3.20 sh -c rm -rf /target/..?* /target/.[!.]* /target/*" {
		t.Fatalf("unexpected docker volume clear command:\n%s", readAgentTestFile(t, logFile))
	}
	want := []string{"-v", "app-data:" + filepath.Join("/stage", "data", "volumes", "app-data")}
	if strings.Join(flags, "\n") != strings.Join(want, "\n") {
		t.Fatalf("flags = %+v, want %+v", flags, want)
	}
}

func TestApplyRestoreItemRunsPGImport(t *testing.T) {
	rootDir := t.TempDir()
	serviceRoot := filepath.Join(rootDir, "postgres")
	stagingDir := filepath.Join(rootDir, "stage")
	stdinFile := filepath.Join(rootDir, "stdin.sql")
	logFile := installFakeDockerScript(t, "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\ncat > \"$TEST_STDIN_FILE\"\n")
	t.Setenv("TEST_STDIN_FILE", stdinFile)
	writeAgentTestFile(t, filepath.Join(serviceRoot, "composia-meta.yaml"), "name: postgres\nproject_name: infra-postgres\ncompose_files:\n  - compose.yaml\nnodes:\n  - main\n")
	writeAgentTestFile(t, filepath.Join(stagingDir, "db.sql"), "select 1;\n")

	err := applyRestoreItem(context.Background(), serviceRoot, stagingDir, backupcfg.RestoreItem{Name: "db", Strategy: "database.pgimport", Service: "postgres"}, nil)
	if err != nil {
		t.Fatalf("applyRestoreItem returned error: %v", err)
	}
	if got := strings.TrimSpace(readAgentTestFile(t, logFile)); got != "compose --project-name infra-postgres -f compose.yaml exec -T postgres psql" {
		t.Fatalf("docker args = %q", got)
	}
	if got := readAgentTestFile(t, stdinFile); got != "select 1;\n" {
		t.Fatalf("stdin = %q", got)
	}
}

func TestStageBackupItemRunsPGDumpAll(t *testing.T) {
	rootDir := t.TempDir()
	serviceRoot := filepath.Join(rootDir, "postgres")
	stagingDir := filepath.Join(rootDir, "stage")
	logFile := installFakeDockerScript(t, "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\nprintf 'dump sql\n'\n")
	writeAgentTestFile(t, filepath.Join(serviceRoot, "composia-meta.yaml"), "name: postgres\nproject_name: infra-postgres\ncompose_files:\n  - compose.yaml\nnodes:\n  - main\n")

	err := stageBackupItem(context.Background(), serviceRoot, stagingDir, backupcfg.RuntimeItem{Name: "db", Strategy: "database.pgdumpall", Service: "postgres"}, nil)
	if err != nil {
		t.Fatalf("stageBackupItem returned error: %v", err)
	}
	if got := strings.TrimSpace(readAgentTestFile(t, logFile)); got != "compose --project-name infra-postgres -f compose.yaml exec -T postgres pg_dumpall" {
		t.Fatalf("docker args = %q", got)
	}
	if got := readAgentTestFile(t, filepath.Join(stagingDir, "db.sql")); got != "dump sql\n" {
		t.Fatalf("dump = %q", got)
	}
}

func TestApplyRestoreItemRejectsUnknownStrategy(t *testing.T) {
	t.Parallel()

	err := applyRestoreItem(context.Background(), t.TempDir(), t.TempDir(), backupcfg.RestoreItem{Name: "db", Strategy: "unknown"}, nil)
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected unknown strategy error, got %v", err)
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
