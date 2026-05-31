package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDockerPruneBuildsTargetCommand(t *testing.T) {
	logFile := installFakeDocker(t)
	if err := runDockerPrune(context.Background(), "images_all", func(string) error { return nil }); err != nil {
		t.Fatalf("runDockerPrune returned error: %v", err)
	}
	if got := strings.TrimSpace(readAgentTestFile(t, logFile)); got != "image prune -a -f" {
		t.Fatalf("docker args = %q", got)
	}
}

func TestRunDockerPruneAllRunsEachTarget(t *testing.T) {
	logFile := installFakeDocker(t)
	var logs strings.Builder
	if err := runDockerPrune(context.Background(), "all", func(output string) error {
		_, writeErr := logs.WriteString(output)
		return writeErr
	}); err != nil {
		t.Fatalf("runDockerPrune all returned error: %v", err)
	}

	for _, want := range []string{"pruning containers...", "pruning networks...", "pruning images...", "pruning volumes...", "pruning builder..."} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("logs missing %q:\n%s", want, logs.String())
		}
	}
	got := readAgentTestFile(t, logFile)
	for _, want := range []string{"container prune -f", "network prune -f", "image prune -f", "volume prune -f", "builder prune -f"} {
		if !strings.Contains(got, want) {
			t.Fatalf("docker log missing %q:\n%s", want, got)
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
