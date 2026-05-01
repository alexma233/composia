package agent

import (
	"strings"
	"testing"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func TestValidateAgentReloadAllowsMutableFields(t *testing.T) {
	current := baseReloadTestAgentConfig()
	next := baseReloadTestAgentConfig()
	next.ControllerAddr = "https://controller.example.test"
	next.ControllerGRPC = true
	next.Token = "next-token"
	next.Caddy = &config.AgentCaddyConfig{GeneratedDir: "/var/lib/composia/caddy-next"}

	if err := validateAgentReload(current, next); err != nil {
		t.Fatalf("validate agent reload: %v", err)
	}
}

func TestValidateAgentReloadRejectsNodeIDChange(t *testing.T) {
	current := baseReloadTestAgentConfig()
	next := baseReloadTestAgentConfig()
	next.NodeID = "edge"

	err := validateAgentReload(current, next)
	if err == nil {
		t.Fatal("expected node_id change to require restart")
	}
	if !strings.Contains(err.Error(), "agent.node_id") {
		t.Fatalf("expected node_id error, got %v", err)
	}
}

func TestValidateAgentReloadRejectsRepoDirChange(t *testing.T) {
	current := baseReloadTestAgentConfig()
	next := baseReloadTestAgentConfig()
	next.RepoDir = "/srv/composia/other-repo"

	err := validateAgentReload(current, next)
	if err == nil {
		t.Fatal("expected repo_dir change to require restart")
	}
	if !strings.Contains(err.Error(), "agent.repo_dir") {
		t.Fatalf("expected repo_dir error, got %v", err)
	}
}

func baseReloadTestAgentConfig() *config.AgentConfig {
	return &config.AgentConfig{
		ControllerAddr: "http://127.0.0.1:7001",
		NodeID:         "main",
		Token:          "main-token",
		RepoDir:        "/srv/composia/agent-repo",
		StateDir:       "/var/lib/composia-agent",
	}
}
