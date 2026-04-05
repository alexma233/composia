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
	if services[0].Node != "node-2" {
		t.Fatalf("expected node node-2, got %q", services[0].Node)
	}
}

func TestDiscoverServicesRejectsMissingMainForDefaultNode(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "vaultwarden", MetaFileName)
	writeFile(t, metaPath, "name: vaultwarden\n")

	_, err := DiscoverServices(repoDir, map[string]struct{}{"node-2": {}})
	if err == nil || !strings.Contains(err.Error(), `node "main" is not configured`) {
		t.Fatalf("expected missing main node error, got %v", err)
	}
}

func TestDiscoverServicesRejectsDuplicateServiceNames(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeFile(t, filepath.Join(repoDir, "service-a", MetaFileName), "name: shared\nnode: node-1\n")
	writeFile(t, filepath.Join(repoDir, "service-b", MetaFileName), "name: shared\nnode: node-1\n")

	_, err := DiscoverServices(repoDir, map[string]struct{}{"node-1": {}})
	if err == nil || !strings.Contains(err.Error(), `service "shared" is declared more than once`) {
		t.Fatalf("expected duplicate service error, got %v", err)
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
