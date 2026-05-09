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
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token: "main-token"

agent:
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

func TestLoadAgentAcceptsControllerGRPC(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
agent:
  controller_addr: "https://controller.example.com"
  controller_grpc: true
  node_id: "node-2"
  token: "node-token"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	agent, err := LoadAgent(configPath)
	if err != nil {
		t.Fatalf("load agent: %v", err)
	}
	if !agent.ControllerGRPC {
		t.Fatal("expected controller_grpc to be true")
	}
}

func TestAgentCaddyGeneratedDirDefault(t *testing.T) {
	t.Parallel()

	agent := &AgentConfig{RepoDir: "/srv/composia/repo", StateDir: "/srv/composia/state"}
	got := agent.CaddyGeneratedDir()
	want := "/srv/composia/state/caddy/generated"
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

func TestLoadControllerAcceptsUpdatesDefaults(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token: "main-token"
  updates:
    default_check_schedule: "0 4 * * *"
    backup_before_update: true
    digest_pin: true
    semver:
      default_allow:
        - patch
        - minor
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	controller, err := LoadController(configPath)
	if err != nil {
		t.Fatalf("load controller: %v", err)
	}
	if controller.Updates == nil || controller.Updates.Semver == nil {
		t.Fatalf("expected updates config")
	}
	if got := controller.Updates.DefaultCheckSchedule; got != "0 4 * * *" {
		t.Fatalf("expected default check schedule, got %q", got)
	}
	if len(controller.Updates.Semver.DefaultAllow) != 2 {
		t.Fatalf("expected semver defaults, got %+v", controller.Updates.Semver.DefaultAllow)
	}
}

func TestLoadControllerRejectsInvalidUpdatesDefaults(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token: "main-token"
  updates:
    default_check_schedule: "invalid"
    semver:
      default_allow:
        - patch
        - feature
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || (!strings.Contains(err.Error(), "controller.updates.default_check_schedule") && !strings.Contains(err.Error(), "controller.updates.semver.default_allow")) {
		t.Fatalf("expected updates validation error, got %v", err)
	}
}

func TestLoadControllerRejectsInvalidAlertmanagerListenPath(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token: "main-token"
  notifications:
    alertmanager:
      listen_path: "alerts"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || !strings.Contains(err.Error(), "controller.notifications.alertmanager.listen_path") {
		t.Fatalf("expected alertmanager path validation error, got %v", err)
	}
}

func TestLoadControllerAcceptsGitAuthUsername(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
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
      token: "git-token"
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

func TestLoadControllerRejectsDuplicateNodeTokens(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token: "shared-token"
    - id: "worker"
      token: "shared-token"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || !strings.Contains(err.Error(), `controller.nodes["worker"].token duplicates controller.nodes["main"].token`) {
		t.Fatalf("expected duplicate node token validation error, got %v", err)
	}
}

func TestLoadControllerRejectsDuplicateAccessTokens(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token: "main-token"
  access_tokens:
    - name: "web-ui"
      token: "shared-token"
    - name: "automation"
      token: "shared-token"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || !strings.Contains(err.Error(), `controller.access_tokens["automation"].token duplicates controller.access_tokens["web-ui"].token`) {
		t.Fatalf("expected duplicate access token validation error, got %v", err)
	}
}

func TestLoadControllerRejectsNodeAndAccessTokenCollision(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token: "shared-token"
  access_tokens:
    - name: "web-ui"
      token: "shared-token"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || !strings.Contains(err.Error(), `controller.access_tokens["web-ui"].token duplicates controller.nodes["main"].token`) {
		t.Fatalf("expected node/access token collision validation error, got %v", err)
	}
}

func TestLoadAgentResolvesTokenFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	tokenPath := filepath.Join(rootDir, "agent.token")
	if err := os.WriteFile(tokenPath, []byte(" agent-token\n"), 0o644); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	configPath := filepath.Join(rootDir, "config.yaml")
	content := strings.TrimSpace(`
agent:
  controller_addr: "https://controller.example.com"
  node_id: "node-2"
  token_file: "`+tokenPath+`"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	agent, err := LoadAgent(configPath)
	if err != nil {
		t.Fatalf("load agent: %v", err)
	}
	if agent.Token != "agent-token" {
		t.Fatalf("expected resolved agent token, got %q", agent.Token)
	}
}

func TestLoadAgentRejectsTokenAndTokenFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	tokenPath := filepath.Join(rootDir, "agent.token")
	if err := os.WriteFile(tokenPath, []byte("agent-token\n"), 0o644); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	configPath := filepath.Join(rootDir, "config.yaml")
	content := strings.TrimSpace(`
agent:
  controller_addr: "https://controller.example.com"
  node_id: "node-2"
  token: "inline-token"
  token_file: "`+tokenPath+`"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadAgent(configPath)
	if err == nil || !strings.Contains(err.Error(), "agent.token and agent.token_file must not both be set") {
		t.Fatalf("expected token/token_file validation error, got %v", err)
	}
}

func TestLoadControllerResolvesInlineOrFileSecrets(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	nodeTokenPath := filepath.Join(rootDir, "node.token")
	accessTokenPath := filepath.Join(rootDir, "access.token")
	gitTokenPath := filepath.Join(rootDir, "git.token")
	dnsTokenPath := filepath.Join(rootDir, "dns.token")
	smtpPasswordPath := filepath.Join(rootDir, "smtp.password")
	telegramTokenPath := filepath.Join(rootDir, "telegram.token")
	for path, value := range map[string]string{
		nodeTokenPath:     "node-token\n",
		accessTokenPath:   "access-token\n",
		gitTokenPath:      "git-token\n",
		dnsTokenPath:      "dns-token\n",
		smtpPasswordPath:  "smtp-password\n",
		telegramTokenPath: "telegram-token\n",
	} {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatalf("write secret file %q: %v", path, err)
		}
	}
	configPath := filepath.Join(rootDir, "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token_file: "`+nodeTokenPath+`"
  access_tokens:
    - name: "web-ui"
      token_file: "`+accessTokenPath+`"
  git:
    remote_url: "https://example.com/repo.git"
    pull_interval: "30s"
    auth:
      token_file: "`+gitTokenPath+`"
  dns:
    cloudflare:
      api_token_file: "`+dnsTokenPath+`"
  notifications:
    smtp:
      host: "smtp.example.com"
      port: 587
      from: "composia@example.com"
      to: ["ops@example.com"]
      password_file: "`+smtpPasswordPath+`"
    telegram:
      chat_id: "12345"
      bot_token_file: "`+telegramTokenPath+`"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	controller, err := LoadController(configPath)
	if err != nil {
		t.Fatalf("load controller: %v", err)
	}
	if got := controller.Nodes[0].Token; got != "node-token" {
		t.Fatalf("expected resolved node token, got %q", got)
	}
	if got := controller.AccessTokens[0].Token; got != "access-token" {
		t.Fatalf("expected resolved access token, got %q", got)
	}
	if got := controller.Git.Auth.Token; got != "git-token" {
		t.Fatalf("expected resolved git token, got %q", got)
	}
	if got := controller.DNS.Cloudflare.APIToken; got != "dns-token" {
		t.Fatalf("expected resolved dns token, got %q", got)
	}
	if got := controller.Notifications.SMTP.Password; got != "smtp-password" {
		t.Fatalf("expected resolved smtp password, got %q", got)
	}
	if got := controller.Notifications.Telegram.BotToken; got != "telegram-token" {
		t.Fatalf("expected resolved telegram token, got %q", got)
	}
}

func TestLoadControllerRejectsEmptyTokenFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	tokenPath := filepath.Join(rootDir, "node.token")
	if err := os.WriteFile(tokenPath, []byte("\n"), 0o644); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	configPath := filepath.Join(rootDir, "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token_file: "`+tokenPath+`"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || !strings.Contains(err.Error(), "controller.nodes[\"main\"].token_file") {
		t.Fatalf("expected empty token file validation error, got %v", err)
	}
}

func TestLoadControllerRejectsDuplicateResolvedTokens(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	tokenPath := filepath.Join(rootDir, "shared.token")
	if err := os.WriteFile(tokenPath, []byte("shared-token\n"), 0o644); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	configPath := filepath.Join(rootDir, "config.yaml")
	content := strings.TrimSpace(`
controller:
  listen_addr: ":8080"
  repo_dir: "/srv/composia/repo"
  state_dir: "/srv/composia/state-controller"
  log_dir: "/srv/composia/logs"
  nodes:
    - id: "main"
      token_file: "`+tokenPath+`"
  access_tokens:
    - name: "web-ui"
      token: "shared-token"
`) + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadController(configPath)
	if err == nil || !strings.Contains(err.Error(), `controller.access_tokens["web-ui"].token duplicates controller.nodes["main"].token`) {
		t.Fatalf("expected duplicate resolved token validation error, got %v", err)
	}
}
