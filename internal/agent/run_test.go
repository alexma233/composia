package agent

import (
	"os"
	"path/filepath"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/config"
)

func TestEnsureAgentDirsCreatesDataProtectDirOnStartup(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	cfg := &config.AgentConfig{
		RepoDir:  filepath.Join(rootDir, "repo"),
		StateDir: filepath.Join(rootDir, "state"),
	}

	if err := ensureAgentDirs(cfg); err != nil {
		t.Fatalf("ensure agent dirs: %v", err)
	}

	for _, dir := range []string{
		cfg.StateDir,
		dataProtectStageRoot(cfg.StateDir),
		cfg.RepoDir,
		cfg.CaddyGeneratedDir(),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %q: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %q to be a directory", dir)
		}
	}
}
