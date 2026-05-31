package controller

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestShouldAutoDeployService(t *testing.T) {
	t.Parallel()

	service := repo.Service{Meta: repo.ServiceMeta{AutoDeploy: boolPtr(true)}}
	if shouldAutoDeployService(&config.ControllerConfig{}, service, false) {
		t.Fatalf("missing controller auto_deploy config should disable auto deploy")
	}
	if !shouldAutoDeployService(&config.ControllerConfig{AutoDeploy: &config.ControllerAutoDeployConfig{Services: true}}, service, false) {
		t.Fatalf("expected service auto deploy")
	}
	if shouldAutoDeployService(&config.ControllerConfig{AutoDeploy: &config.ControllerAutoDeployConfig{Services: true}}, service, true) {
		t.Fatalf("infra service should require infra auto deploy flag")
	}
	if !shouldAutoDeployService(&config.ControllerConfig{AutoDeploy: &config.ControllerAutoDeployConfig{Infra: true}}, service, true) {
		t.Fatalf("expected infra auto deploy")
	}
	service.Meta.AutoDeploy = boolPtr(false)
	if shouldAutoDeployService(&config.ControllerConfig{AutoDeploy: &config.ControllerAutoDeployConfig{Services: true}}, service, false) {
		t.Fatalf("service-level auto_deploy false should disable")
	}
}

func TestShortRevision(t *testing.T) {
	t.Parallel()

	if got := shortRevision("1234567890abcdef"); got != "12345678" {
		t.Fatalf("shortRevision long = %q", got)
	}
	if got := shortRevision("1234567"); got != "1234567" {
		t.Fatalf("shortRevision short = %q", got)
	}
}

func TestAutoPullFetchAndFastForwardReturnsLocalOnlyForDirtyWorktree(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	gitRunControllerTest(t, repoDir, "init")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitRunControllerTest(t, repoDir, "add", ".")
	gitRunControllerTest(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("dirty README: %v", err)
	}
	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	state, err := autoPullFetchAndFastForward(context.Background(), &config.ControllerConfig{RepoDir: repoDir, Git: &config.ControllerGitConfig{RemoteURL: "https://example.com/repo.git", Branch: "main"}}, db)
	if err != nil {
		t.Fatalf("auto pull dirty worktree: %v", err)
	}
	if state.SyncStatus != store.RepoSyncStatusLocalOnly {
		t.Fatalf("sync status = %q", state.SyncStatus)
	}
}

func gitRunControllerTest(t *testing.T, repoDir string, args ...string) {
	t.Helper()

	commandArgs := append([]string{"-C", repoDir}, args...)
	output, err := exec.CommandContext(context.Background(), "git", commandArgs...).CombinedOutput() //nolint:gosec
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}
