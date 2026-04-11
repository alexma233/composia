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

func TestLoadControllerRejectsUnknownRusticMainNode(t *testing.T) {
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
  rustic:
    main_nodes:
      - "node-2"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || !strings.Contains(err.Error(), "controller.rustic.main_nodes") {
		t.Fatalf("expected rustic main_nodes validation error, got %v", err)
	}
}

func TestLoadControllerRejectsInvalidScheduledSpecs(t *testing.T) {
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
  backup:
    default_schedule: "invalid"
  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: "15 3 * * *"
      prune_schedule: "invalid"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || (!strings.Contains(err.Error(), "controller.backup.default_schedule") && !strings.Contains(err.Error(), "controller.rustic.maintenance.prune_schedule")) {
		t.Fatalf("expected schedule validation error, got %v", err)
	}
}

func TestLoadControllerAcceptsGitAuthUsername(t *testing.T) {
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
  git:
    remote_url: "https://example.com/repo.git"
    pull_interval: "30s"
    auth:
      username: "octocat"
      token_file: "/run/secrets/git-token"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	controller, err := LoadController(configPath)
	if err != nil {
		t.Fatalf("load controller: %v", err)
	}
	if controller.Git == nil || controller.Git.Auth == nil {
		t.Fatalf("expected git auth config to be present")
	}
	if controller.Git.Auth.Username != "octocat" {
		t.Fatalf("expected git auth username %q, got %q", "octocat", controller.Git.Auth.Username)
	}
}
