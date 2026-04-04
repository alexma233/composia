package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadControllerRejectsSharedRepoDir(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  controller_addr: "http://127.0.0.1:8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token: "main-token"

agent:
  controller_addr: "http://127.0.0.1:8080"
  node_id: "main"
  token: "main-token"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-agent"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || !strings.Contains(err.Error(), "must not use the same path") {
		t.Fatalf("expected shared repo_dir validation error, got %v", err)
	}
}

func TestLoadAgentRejectsUnknownField(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
agent:
  controller_addr: "http://127.0.0.1:8080"
  node_id: "node-2"
  token: "node-token"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state"
  unexpected: true
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadAgent(configPath)
	if err == nil || !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("expected strict YAML field error, got %v", err)
	}
}

func TestAgentCaddyGeneratedDirDefault(t *testing.T) {
	t.Parallel()

	agent := &AgentConfig{RepoDir: "/srv/composia/repo"}
	got := agent.CaddyGeneratedDir()
	want := "/srv/composia/repo/caddy/config/site-generated"
	if got != want {
		t.Fatalf("expected default caddy dir %q, got %q", want, got)
	}
}
