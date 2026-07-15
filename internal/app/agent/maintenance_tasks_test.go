package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDockerPruneBuildsTargetCommand(t *testing.T) {
	args, isAll, err := dockerPruneArgs("images_all")
	if err != nil {
		t.Fatalf("dockerPruneArgs returned error: %v", err)
	}
	if isAll {
		t.Fatalf("images_all must not expand to all")
	}
	if got := strings.Join(args, " "); got != "image prune -a -f" {
		t.Fatalf("docker args = %q", got)
	}
}

func TestRunDockerPruneAllRunsEachTarget(t *testing.T) {
	args, isAll, err := dockerPruneArgs("all")
	if err != nil {
		t.Fatalf("dockerPruneArgs all returned error: %v", err)
	}
	if !isAll || args != nil {
		t.Fatalf("all = args %v isAll %t, want nil true", args, isAll)
	}

	for _, target := range []string{"containers", "networks", "images", "volumes", "builder"} {
		args, isAll, err := dockerPruneArgs(target)
		if err != nil {
			t.Fatalf("dockerPruneArgs %q: %v", target, err)
		}
		if isAll || len(args) == 0 {
			t.Fatalf("dockerPruneArgs %q = args %v isAll %t", target, args, isAll)
		}
	}
}

func TestRunDockerPruneRejectsUnknownTarget(t *testing.T) {
	t.Parallel()

	err := runDockerPrune(context.Background(), "bad", func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "unknown prune target") {
		t.Fatalf("expected unknown target error, got %v", err)
	}
}

func installFakeDocker(t *testing.T) string {
	t.Helper()

	return installFakeDockerScript(t, "#!/bin/sh\nprintf '%s\n' \"$*\" >> \"$TEST_DOCKER_LOG_FILE\"\n")
}

func installFakeDockerScript(t *testing.T, script string) string {
	t.Helper()

	rootDir := t.TempDir()
	binDir := filepath.Join(rootDir, "bin")
	logFile := filepath.Join(rootDir, "docker.log")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.WriteFile(dockerPath, []byte(script), 0o750); err != nil { //nolint:gosec
		t.Fatalf("write fake docker script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_DOCKER_LOG_FILE", logFile)
	return logFile
}

func readAgentTestFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(content)
}
