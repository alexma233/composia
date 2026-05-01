package controller

import (
	"strings"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func TestValidateControllerReloadAllowsMutableFields(t *testing.T) {
	current := baseReloadTestConfig()
	next := baseReloadTestConfig()
	next.ControllerAddr = "https://controller.example.test"
	next.Nodes = append(next.Nodes, config.NodeConfig{ID: "edge", Token: "edge-token"})
	next.AccessTokens = []config.AccessTokenConfig{{Name: "operator", Token: "operator-token"}}

	if err := validateControllerReload(current, next); err != nil {
		t.Fatalf("validate controller reload: %v", err)
	}
}

func TestValidateControllerReloadRejectsListenAddrChange(t *testing.T) {
	current := baseReloadTestConfig()
	next := baseReloadTestConfig()
	next.ListenAddr = "127.0.0.1:7002"

	err := validateControllerReload(current, next)
	if err == nil {
		t.Fatal("expected listen_addr change to require restart")
	}
	if !strings.Contains(err.Error(), "controller.listen_addr") {
		t.Fatalf("expected listen_addr error, got %v", err)
	}
}

func TestValidateControllerReloadRejectsRepoDirChange(t *testing.T) {
	current := baseReloadTestConfig()
	next := baseReloadTestConfig()
	next.RepoDir = "/srv/composia/other-repo"

	err := validateControllerReload(current, next)
	if err == nil {
		t.Fatal("expected repo_dir change to require restart")
	}
	if !strings.Contains(err.Error(), "controller.repo_dir") {
		t.Fatalf("expected repo_dir error, got %v", err)
	}
}

func baseReloadTestConfig() *config.ControllerConfig {
	return &config.ControllerConfig{
		ListenAddr:     "127.0.0.1:7001",
		ControllerAddr: "http://127.0.0.1:7001",
		RepoDir:        "/srv/composia/repo",
		StateDir:       "/var/lib/composia",
		LogDir:         "/var/log/composia",
		Nodes:          []config.NodeConfig{{ID: "main", Token: "main-token"}},
	}
}
