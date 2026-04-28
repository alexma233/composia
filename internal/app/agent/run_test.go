package agent

import (
	"os"
	"path/filepath"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func TestParseSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  uint64
		ok    bool
	}{
		{name: "bytes", input: "512B", want: 512, ok: true},
		{name: "decimal kilobytes", input: "1.5kB", want: 1500, ok: true},
		{name: "decimal gigabytes", input: "4.8GB", want: 4800000000, ok: true},
		{name: "binary gibibytes", input: "1.5GiB", want: 1610612736, ok: true},
		{name: "decimal terabytes", input: "2.8TB", want: 2800000000000, ok: true},
		{name: "comma decimal", input: "1,5MB", want: 1500000, ok: true},
		{name: "invalid", input: "oops", want: 0, ok: false},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseSize(tc.input)
			if ok != tc.ok {
				t.Fatalf("parseSize(%q) ok = %v, want %v", tc.input, ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("parseSize(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

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
