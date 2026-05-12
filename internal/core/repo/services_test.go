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
compose_files:
  - compose.yaml
  - compose.prod.yaml
nodes:
  - node-2
infra:
  caddy:
    compose_service: edge
    config_dir: /etc/caddy
network:
  caddy:
    enabled: true
    source: ./site_config.caddy
update:
  check_schedule: "0 4 * * *"
  images:
    app:
      image: ghcr.io/example/vaultwarden
      current:
        env:
          file: .env
          key: APP_VERSION
      discovery:
        sources:
          - type: auto
      filter:
        type: semver
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
	if got := strings.Join(services[0].Meta.ComposeFiles, ","); got != "compose.yaml,compose.prod.yaml" {
		t.Fatalf("expected normalized compose files, got %q", got)
	}
}

func TestDiscoverServicesParsesConfigInfraService(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "host-service", MetaFileName)
	writeFile(t, metaPath, strings.TrimSpace(`
name: host-service
nodes:
  - main
infra:
  config: {}
network:
  caddy:
    enabled: true
    source: ./host-service.caddy
`)+"\n")

	services, err := DiscoverServices(repoDir, map[string]struct{}{"main": {}})
	if err != nil {
		t.Fatalf("discover services: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if !services[0].Meta.IsConfigInfra() {
		t.Fatalf("expected infra.config service")
	}
	if len(services[0].Meta.ComposeFiles) != 0 {
		t.Fatalf("expected no compose files, got %+v", services[0].Meta.ComposeFiles)
	}
}

func TestDiscoverServicesSkipsServiceWithoutTargetNodes(t *testing.T) {
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
	writeFile(t, filepath.Join(repoDir, "service-a", MetaFileName), "name: shared\nnodes:\n  - node-1\n")
	writeFile(t, filepath.Join(repoDir, "service-b", MetaFileName), "name: shared\nnodes:\n  - node-1\n")

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
	if err == nil || !strings.Contains(err.Error(), "at least one target node is required") {
		t.Fatalf("expected strict target validation error, got %v", err)
	}
}

func TestFindServiceIgnoresUnrelatedInvalidDraft(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	writeFile(t, filepath.Join(repoDir, "alpha", MetaFileName), "name: alpha\nnodes:\n  - node-1\n")
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
	writeFile(t, metaPath, "name: alpha\nnodes:\n  - node-1\nnetwork:\n  caddy:\n    enabled: true\n    source: ./Caddyfile\n")

	updated, err := RewriteServiceTargetNodes(metaPath, []string{"node-1", "node-2"}, map[string]struct{}{"node-1": {}, "node-2": {}})
	if err != nil {
		t.Fatalf("rewrite target nodes: %v", err)
	}
	expected := "name: alpha\nnodes:\n  - node-1\n  - node-2\nnetwork:\n  caddy:\n    enabled: true\n    source: ./Caddyfile\n"
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
nodes:
  - main
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

func TestDiscoverServicesParsesUpdateImageOverrides(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "app", MetaFileName)
	writeFile(t, metaPath, strings.TrimSpace(`
name: app
nodes:
  - main
update:
  auto_apply: true
  check_schedule: "0 4 * * *"
  backup_before_update: true
  digest_pin: true
  images:
    api:
      image: ghcr.io/example/api
      auto_apply: false
      check_schedule: "15 4 * * *"
      backup_before_update: false
      digest_pin: false
      current:
        env:
          file: .env
          key: API_VERSION
      discovery:
        sources:
          - type: auto
      filter:
        type: semver
        allow:
          - patch
          - minor
    worker:
      image: ghcr.io/example/worker
      current:
        tag: nightly
      discovery:
        sources:
          - type: digest
`)+"\n")

	services, err := DiscoverServices(repoDir, map[string]struct{}{"main": {}})
	if err != nil {
		t.Fatalf("discover services: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	update := services[0].Meta.Update
	if update == nil || len(update.Images) != 2 {
		t.Fatalf("expected update images, got %+v", update)
	}
	api := update.Images["api"]
	if api.Current.Env == nil || api.Current.Env.File != ".env" || api.Current.Env.Key != "API_VERSION" {
		t.Fatalf("unexpected api current: %+v", api.Current)
	}
	if api.Filter == nil || api.Filter.Type != "semver" || len(api.Filter.Allow) != 2 {
		t.Fatalf("unexpected api filter: %+v", api.Filter)
	}
	if api.AutoApply == nil || *api.AutoApply {
		t.Fatalf("expected api auto_apply override false")
	}
	if api.BackupBeforeUpdate == nil || *api.BackupBeforeUpdate {
		t.Fatalf("expected api backup_before_update override false")
	}
	if api.DigestPin == nil || *api.DigestPin {
		t.Fatalf("expected api digest_pin override false")
	}
}

func TestDiscoverServicesRejectsInvalidUpdateImageSource(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "app", MetaFileName)
	writeFile(t, metaPath, strings.TrimSpace(`
name: app
nodes:
  - main
update:
  images:
    api:
      image: ghcr.io/example/api
      current:
        tag: latest
        env:
          file: .env
          key: API_VERSION
      discovery:
        sources:
          - type: auto
      filter:
        type: semver
`)+"\n")

	services, err := DiscoverServices(repoDir, map[string]struct{}{"main": {}})
	if err != nil {
		t.Fatalf("discover services: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected invalid image source to make service invalid, got %+v", services)
	}
}

func TestDiscoverServicesSkipsServiceWithUnsafeDataInclude(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metaPath := filepath.Join(repoDir, "vaultwarden", MetaFileName)
	writeFile(t, metaPath, strings.TrimSpace(`
name: vaultwarden
nodes:
  - main
data_protect:
  data:
    - name: config
      backup:
        strategy: files.copy
        include:
          - ../config
`)+"\n")

	services, err := DiscoverServices(repoDir, map[string]struct{}{"main": {}})
	if err != nil {
		t.Fatalf("discover services: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected unsafe include service to be skipped, got %+v", services)
	}
}

func TestComposeProjectNameUsesConfiguredValue(t *testing.T) {
	t.Parallel()

	if got := ComposeProjectName(" infra-caddy ", "Renovate"); got != "infra-caddy" {
		t.Fatalf("expected configured project name, got %q", got)
	}
}

func TestComposeProjectNameNormalizesFallbackServiceName(t *testing.T) {
	t.Parallel()

	if got := ComposeProjectName("", "Renovate App"); got != "renovate-app" {
		t.Fatalf("expected normalized project name, got %q", got)
	}
}

func TestComposeProjectNameFallsBackToDefaultWhenFallbackHasNoValidCharacters(t *testing.T) {
	t.Parallel()

	if got := ComposeProjectName("", "___"); got != "service" {
		t.Fatalf("expected default project name, got %q", got)
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
