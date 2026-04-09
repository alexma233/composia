package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverServicesParsesValidService(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "vaultwarden", MetaFileName)
	writeFile(t, metaPath, strings.TrimSpace(`
name: vaultwarden
node: node-2
infra:
  caddy:
    compose_service: edge
    config_dir: /etc/caddy
network:
  caddy:
    enabled: true
    source: ./site_config.caddy
update:
  strategy: pull_and_recreate
data_protect:
  data:
    - name: config
      backup:
        strategy: files.copy
        include:
          - ./config
      restore:
        strategy: files.copy
        include:
          - ./config
backup:
  data:
    - name: config
migrate:
  data:
    - name: config
`)+"\n")

	services, err := DiscoverServices(repoDir, map[string]struct{}{"node-2": {}})
	if err != nil {
		t.Fatalf("discover services: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Name != "vaultwarden" {
		t.Fatalf("expected service name vaultwarden, got %q", services[0].Name)
	}
	if len(services[0].TargetNodes) != 1 || services[0].TargetNodes[0] != "node-2" {
		t.Fatalf("expected target nodes [node-2], got %+v", services[0].TargetNodes)
	}
	if got := services[0].Meta.CaddyComposeService(); got != "edge" {
		t.Fatalf("expected caddy compose service edge, got %q", got)
	}
	if got := services[0].Meta.CaddyConfigDir(); got != "/etc/caddy" {
		t.Fatalf("expected caddy config dir /etc/caddy, got %q", got)
	}
}

func TestDiscoverServicesSkipsInvalidDefaultNodeDraft(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "vaultwarden", MetaFileName)
	writeFile(t, metaPath, "name: vaultwarden\n")

	services, err := DiscoverServices(repoDir, map[string]struct{}{"node-2": {}})
	if err != nil {
		t.Fatalf("discover services: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected invalid draft to be skipped, got %+v", services)
	}
}

func TestDiscoverServicesSkipsDuplicateServiceNames(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeFile(t, filepath.Join(repoDir, "service-a", MetaFileName), "name: shared\nnode: node-1\n")
	writeFile(t, filepath.Join(repoDir, "service-b", MetaFileName), "name: shared\nnode: node-1\n")

	services, err := DiscoverServices(repoDir, map[string]struct{}{"node-1": {}})
	if err != nil {
		t.Fatalf("discover services: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected duplicate declared services to be skipped, got %+v", services)
	}
}

func TestFindServiceRejectsInvalidTargetService(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeFile(t, filepath.Join(repoDir, "draft", MetaFileName), "name: draft\n")

	_, err := FindService(repoDir, map[string]struct{}{"node-2": {}}, "draft")
	if err == nil || !strings.Contains(err.Error(), `node "main" is not configured`) {
		t.Fatalf("expected strict target validation error, got %v", err)
	}
}

func TestFindServiceIgnoresUnrelatedInvalidDraft(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeFile(t, filepath.Join(repoDir, "alpha", MetaFileName), "name: alpha\nnode: node-1\n")
	writeFile(t, filepath.Join(repoDir, "draft", MetaFileName), "name: draft\n")

	service, err := FindService(repoDir, map[string]struct{}{"node-1": {}}, "alpha")
	if err != nil {
		t.Fatalf("find valid service with unrelated invalid draft: %v", err)
	}
	if service.Name != "alpha" {
		t.Fatalf("unexpected service returned: %+v", service)
	}
}

func TestRewriteServiceTargetNodesUpdatesOnlyTargetNodes(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "alpha", MetaFileName)
	writeFile(t, metaPath, "name: alpha\nnode: node-1\nnetwork:\n  caddy:\n    enabled: true\n    source: ./Caddyfile\n")

	updated, err := RewriteServiceTargetNodes(metaPath, []string{"node-1", "node-2"}, map[string]struct{}{"node-1": {}, "node-2": {}})
	if err != nil {
		t.Fatalf("rewrite target nodes: %v", err)
	}
	expected := "name: alpha\nnetwork:\n  caddy:\n    enabled: true\n    source: ./Caddyfile\nnodes:\n  - node-1\n  - node-2\n"
	if updated != expected {
		t.Fatalf("unexpected rewritten meta:\n%s", updated)
	}
}

func TestDiscoverServicesRejectsInvalidBackupSchedule(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "vaultwarden", MetaFileName)
	writeFile(t, metaPath, strings.TrimSpace(`
name: vaultwarden
node: main
data_protect:
  data:
    - name: config
      backup:
        strategy: files.copy
        include:
          - ./config
backup:
  data:
    - name: config
      schedule: invalid
`)+"\n")

	services, err := DiscoverServices(repoDir, map[string]struct{}{"main": {}})
	if err != nil {
		t.Fatalf("discover services: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected invalid scheduled service to be skipped, got %+v", services)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
