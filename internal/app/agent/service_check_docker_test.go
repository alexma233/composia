package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run with COMPOSIA_DOCKER_TEST_IMAGE=alpine:latest and an optional COMPOSIA_DOCKER_TEST_OTHER_IMAGE=alpine:3.23.
func TestServiceCheckDockerIntegration(t *testing.T) {
	image := os.Getenv("COMPOSIA_DOCKER_TEST_IMAGE")
	if image == "" {
		t.Skip("requires an explicitly selected local Docker image with a repository digest and sleep")
	}
	root := t.TempDir()
	project := fmt.Sprintf("composia-image-check-%d", time.Now().UnixNano())
	ref := "alpine:" + project
	compose := composeCommandConfig{ProjectName: project, Files: []string{"compose.yaml"}}
	run := func(args ...string) string {
		t.Helper()
		output, err := serviceDockerOutput(t.Context(), root, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(output))
	}
	t.Log("Compose version:", run("compose", "version", "--short"))
	run("image", "tag", image, ref)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "docker", buildComposeArgs(compose, "down", "--remove-orphans", "--timeout", "1")...) //nolint:gosec
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Errorf("clean up test containers: %v: %s", err, output)
		}
		if output, err := exec.CommandContext(ctx, "docker", "image", "rm", ref).CombinedOutput(); err != nil { //nolint:gosec // Only the uniquely named test tag is removed.
			t.Errorf("clean up test image tag: %v: %s", err, output)
		}
	})
	writeAgentTestFile(t, filepath.Join(root, ".env"), "TEST_IMAGE="+ref+"\n")
	writeAgentTestFile(t, filepath.Join(root, "runtime.env"), "EXAMPLE=before\n")
	writeAgentTestFile(t, filepath.Join(root, "compose.yaml"), `services:
  app:
    image: ${TEST_IMAGE}
    command: ["sleep", "300"]
    env_file: runtime.env
    network_mode: none
    read_only: true
    cap_drop: [ALL]
    user: "65532:65532"
`)
	before, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	// This deployment has no Composia task record.
	run(buildComposeArgs(compose, "up", "-d", "--pull", "never", "--no-build")...)
	checked, err := checkServiceContainers(t.Context(), root, compose)
	if err != nil {
		t.Fatal(err)
	}
	observations, err := observeServiceImages(t.Context(), root, checked)
	if err != nil || len(observations) != 1 || observations[0].LocalDigest == "" {
		t.Fatalf("externally deployed configuration was rejected: %+v, %v", observations, err)
	}
	if other := os.Getenv("COMPOSIA_DOCKER_TEST_OTHER_IMAGE"); other != "" {
		oldID := run("image", "inspect", "--format", "{{.Id}}", image)
		otherID := run("image", "inspect", "--format", "{{.Id}}", other)
		if oldID == otherID {
			t.Fatal("the alternate image must have a different ID")
		}
		run("image", "tag", other, ref)
		checked, err := checkServiceContainers(t.Context(), root, compose)
		if err != nil {
			t.Fatal(err)
		}
		afterPull, err := observeServiceImages(t.Context(), root, checked)
		if err != nil || len(afterPull) != 1 || afterPull[0].LocalDigest != observations[0].LocalDigest {
			t.Fatalf("a changed tag cache affected the running observation: %+v, %v", afterPull, err)
		}
		run("image", "tag", image, ref)
	}
	writeAgentTestFile(t, filepath.Join(root, "runtime.env"), "EXAMPLE=after\n")
	if _, err := checkServiceContainers(t.Context(), root, compose); err == nil || !strings.Contains(err.Error(), "not applied") {
		t.Fatalf("unapplied env_file change was accepted: %v", err)
	}
	run(buildComposeArgs(compose, "up", "-d", "--pull", "never", "--no-build")...)
	if _, err := checkServiceContainers(t.Context(), root, compose); err != nil {
		t.Fatalf("reapplied env_file change was rejected: %v", err)
	}
	run(buildComposeArgs(compose, "stop", "--timeout", "1")...)
	checked, err = checkServiceContainers(t.Context(), root, compose)
	if err != nil {
		t.Fatalf("stopped matching container must be configuration-consistent: %v", err)
	}
	if _, err := observeServiceImages(t.Context(), root, checked); err == nil {
		t.Fatal("stopped container must not provide a running-image observation")
	}
	after, err := os.Stat(root)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("check replaced the service directory: %v", err)
	}
}
