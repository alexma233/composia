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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
