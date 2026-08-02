package controller

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func TestEditableControllerConfigExcludesSecrets(t *testing.T) {
	secret := "inline-secret"
	cfg := &config.ControllerConfig{
		RepoDir:      "/srv/repo",
		StateDir:     "/srv/state",
		LogDir:       "/srv/logs",
		Nodes:        []config.NodeConfig{{ID: "main", Token: secret, DisplayName: "Main"}},
		AccessTokens: []config.AccessTokenConfig{{Name: "web", Token: "access-secret"}},
		Git:          &config.ControllerGitConfig{Auth: &config.ControllerGitAuthConfig{Token: "git-secret"}},
		RemoteConfig: &config.ControllerRemoteConfig{Enabled: true},
	}

	content, err := marshalEditableControllerConfig(cfg)
	if err != nil {
		t.Fatalf("marshal editable config: %v", err)
	}
	if strings.Contains(content, secret) || strings.Contains(content, "access-secret") || strings.Contains(content, "git-secret") {
		t.Fatalf("editable config leaked a secret: %s", content)
	}
	if strings.Contains(content, "remote_config") || strings.Contains(content, "access_tokens") {
		t.Fatalf("editable config exposed a non-editable section: %s", content)
	}
}

func TestEditableControllerConfigUpdateIsAtomicAndValidated(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	stateDir := filepath.Join(root, "state")
	logDir := filepath.Join(root, "logs")
	for _, path := range []string{repoDir, stateDir, logDir} {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
	}
	path := filepath.Join(root, "config.yaml")
	raw := []byte("controller:\n  listen_addr: ':7001'\n  repo_dir: " + repoDir + "\n  state_dir: " + stateDir + "\n  log_dir: " + logDir + "\n  remote_config:\n    enabled: true\n  auto_deploy:\n    services: false\n    infra: false\n  nodes:\n    - id: main\n      token: node-secret\n      display_name: Main\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.LoadController(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	var reloads atomic.Int32
	editor := &controllerConfigEditor{configPath: path, cfg: cfg, reload: func(context.Context) error { reloads.Add(1); return nil }}
	get, err := editor.GetEditableConfig(context.Background(), connect.NewRequest(&controllerv1.GetEditableConfigRequest{}))
	if err != nil {
		t.Fatalf("get editable config: %v", err)
	}
	updated := strings.Replace(get.Msg.GetYaml(), "services: false", "services: true", 1)
	write, err := editor.UpdateEditableConfig(context.Background(), connect.NewRequest(&controllerv1.UpdateEditableConfigRequest{Yaml: updated, BaseRevision: get.Msg.GetRevision()}))
	if err != nil {
		t.Fatalf("update editable config: %v", err)
	}
	if write.Msg.GetRevision() == get.Msg.GetRevision() || reloads.Load() != 1 {
		t.Fatalf("expected a new revision and one reload, got revision=%q reloads=%d", write.Msg.GetRevision(), reloads.Load())
	}
	content, err := os.ReadFile(path) //nolint:gosec // Test path.
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if !strings.Contains(string(content), "services: true") || !strings.Contains(string(content), "node-secret") {
		t.Fatalf("updated config lost data: %s", content)
	}
	if _, err := config.LoadController(path); err != nil {
		t.Fatalf("updated config is invalid: %v", err)
	}
}

func TestDecodeEditableControllerConfigRejectsUnknownFields(t *testing.T) {
	if _, err := decodeEditableControllerConfig("controller:\n  access_tokens: []\n"); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestEditableControllerConfigSparseUpdatePreservesUntouchedFields(t *testing.T) {
	raw := []byte("# keep this comment\ncontroller:\n  listen_addr: ':7001'\n  repo_dir: /srv/repo\n  state_dir: /srv/state\n  log_dir: /srv/logs\n  updates:\n    default_check_schedule: '0 1 * * *'\n    auto_apply: false\n  nodes:\n    - id: main\n      token: node-secret\n      display_name: Main\n      enabled: true\n")
	edited, err := decodeEditableControllerConfig("controller:\n  updates:\n    auto_apply: true\n  nodes:\n    - id: main\n      display_name: Renamed\n")
	if err != nil {
		t.Fatalf("decode sparse editable config: %v", err)
	}
	candidate, err := applyEditableControllerConfig(raw, edited)
	if err != nil {
		t.Fatalf("apply sparse editable config: %v", err)
	}
	text := string(candidate)
	for _, want := range []string{"# keep this comment", "default_check_schedule: '0 1 * * *'", "auto_apply: true", "token: node-secret", "display_name: Renamed", "enabled: true"} {
		if !strings.Contains(text, want) {
			t.Fatalf("candidate lost %q: %s", want, text)
		}
	}
}

func TestDecodeEditableControllerConfigRejectsAliases(t *testing.T) {
	if _, err := decodeEditableControllerConfig("controller: &controller\n  nodes: []\n  auto_deploy: *controller\n"); err == nil {
		t.Fatal("expected aliases to be rejected")
	}
}

func TestControllerReloadRevisionQueuesWithoutWaiting(t *testing.T) {
	requests := make(chan reloadRequest, 1)
	if err := requestControllerReloadRevision(context.Background(), requests, "revision"); err != nil {
		t.Fatalf("queue controller reload: %v", err)
	}
	request := <-requests
	if request.reply != nil {
		t.Fatal("queued controller reload must not wait for a runtime response")
	}
	if request.expectedRevision != "revision" {
		t.Fatalf("expected revision = %q, got %q", "revision", request.expectedRevision)
	}
}

func TestRestoreControllerConfigDoesNotOverwriteChangedRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	oldRaw := []byte("controller:\n  nodes: []\n")
	newRaw := []byte("controller:\n  nodes: []\n  remote_config:\n    enabled: true\n")
	if err := os.WriteFile(path, newRaw, 0o600); err != nil {
		t.Fatalf("write current config: %v", err)
	}

	err := restoreControllerConfigIfCurrent(path, configRevision(oldRaw), oldRaw)
	if !errors.Is(err, errControllerConfigChanged) {
		t.Fatalf("expected revision conflict, got %v", err)
	}
	content, err := os.ReadFile(path) //nolint:gosec // Test path.
	if err != nil {
		t.Fatalf("read current config: %v", err)
	}
	if string(content) != string(newRaw) {
		t.Fatalf("changed config was overwritten: %s", content)
	}
}

func TestAtomicConfigWriteMarksPostRenameSyncFailureAsInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("controller:\n  nodes: []\n")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	err := atomicWriteControllerConfigWithDirectorySync(path, content, 0o600, func(*os.File) error {
		return errors.New("injected directory sync failure")
	})
	if !controllerConfigWriteInstalled(err) {
		t.Fatalf("expected installed write error, got %v", err)
	}
	actual, err := os.ReadFile(path) //nolint:gosec // Test path.
	if err != nil {
		t.Fatalf("read replaced config: %v", err)
	}
	if string(actual) != string(content) {
		t.Fatalf("replacement was not installed: %s", actual)
	}
}
