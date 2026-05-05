package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRepoCollectsErrorsAcrossFiles(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "beta"), 0o755); err != nil {
		t.Fatalf("create beta dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", MetaFileName), []byte("name: shared\nnodes:\n  - main\nunknown_field: true\n"), 0o644); err != nil {
		t.Fatalf("write alpha meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "beta", MetaFileName), []byte("name: shared\nnodes:\n  - missing-node\n"), 0o644); err != nil {
		t.Fatalf("write beta meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 2 {
		t.Fatalf("expected 2 validation errors, got %d: %+v", len(validationErrors), validationErrors)
	}
	if validationErrors[0].Path != "alpha/composia-meta.yaml" || !strings.Contains(validationErrors[0].Message, "unknown_field") {
		t.Fatalf("unexpected first validation error: %+v", validationErrors[0])
	}
	if validationErrors[1].Path != "beta/composia-meta.yaml" || !strings.Contains(validationErrors[1].Message, "missing-node") {
		t.Fatalf("unexpected second validation error: %+v", validationErrors[1])
	}
}

func TestValidateRepoReportsDuplicateServiceNames(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "beta"), 0o755); err != nil {
		t.Fatalf("create beta dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", MetaFileName), []byte("name: shared\nnodes:\n  - main\n"), 0o644); err != nil {
		t.Fatalf("write alpha meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "beta", MetaFileName), []byte("name: shared\nnodes:\n  - main\n"), 0o644); err != nil {
		t.Fatalf("write beta meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 2 {
		t.Fatalf("expected 2 duplicate name errors, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "declared more than once") || !strings.Contains(validationErrors[1].Message, "declared more than once") {
		t.Fatalf("unexpected duplicate errors: %+v", validationErrors)
	}
}

func TestValidateRepoReportsDuplicateCaddyInfraServices(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "beta"), 0o755); err != nil {
		t.Fatalf("create beta dir: %v", err)
	}
	alpha := "name: alpha\nnodes:\n  - main\ninfra:\n  caddy: {}\n"
	beta := "name: beta\nnodes:\n  - main\ninfra:\n  caddy: {}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", MetaFileName), []byte(alpha), 0o644); err != nil {
		t.Fatalf("write alpha meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "beta", MetaFileName), []byte(beta), 0o644); err != nil {
		t.Fatalf("write beta meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 2 {
		t.Fatalf("expected 2 caddy infra errors, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "infra.caddy may only be declared once") || !strings.Contains(validationErrors[1].Message, "infra.caddy may only be declared once") {
		t.Fatalf("unexpected caddy infra errors: %+v", validationErrors)
	}
}

func TestValidateRepoReportsDuplicateRusticInfraServices(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "beta"), 0o755); err != nil {
		t.Fatalf("create beta dir: %v", err)
	}
	alpha := "name: alpha\nnodes:\n  - main\ninfra:\n  rustic: {}\n"
	beta := "name: beta\nnodes:\n  - main\ninfra:\n  rustic: {}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", MetaFileName), []byte(alpha), 0o644); err != nil {
		t.Fatalf("write alpha meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "beta", MetaFileName), []byte(beta), 0o644); err != nil {
		t.Fatalf("write beta meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 2 {
		t.Fatalf("expected 2 rustic infra errors, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "infra.rustic may only be declared once") || !strings.Contains(validationErrors[1].Message, "infra.rustic may only be declared once") {
		t.Fatalf("unexpected rustic infra errors: %+v", validationErrors)
	}
}

func TestValidateRepoRejectsBlankRusticDataProtectDir(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	meta := "name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    data_protect_dir: \"   \"\n"
	if err := os.WriteFile(filepath.Join(repoDir, "backup", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "infra.rustic.data_protect_dir") {
		t.Fatalf("unexpected validation error: %+v", validationErrors[0])
	}
}

func TestValidateRepoRejectsBlankRusticInitArgs(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	meta := "name: backup\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n    init_args:\n      - --set-chunker\n      - \"   \"\n"
	if err := os.WriteFile(filepath.Join(repoDir, "backup", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "infra.rustic.init_args") {
		t.Fatalf("unexpected validation error: %+v", validationErrors[0])
	}
}

func TestValidateRepoRejectsConfigInfraWithRusticInfra(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "backup"), 0o755); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	meta := "name: backup\nnodes:\n  - main\ninfra:\n  config: {}\n  rustic: {}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "backup", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write backup meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "infra.config") || !strings.Contains(validationErrors[0].Message, "infra.rustic") {
		t.Fatalf("unexpected validation error: %+v", validationErrors[0])
	}
}

func TestValidateRepoRejectsConfigInfraWithCaddyInfra(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "edge"), 0o755); err != nil {
		t.Fatalf("create edge dir: %v", err)
	}
	meta := "name: edge\nnodes:\n  - main\ninfra:\n  config: {}\n  caddy: {}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "edge", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write edge meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "infra.config") || !strings.Contains(validationErrors[0].Message, "infra.caddy") {
		t.Fatalf("unexpected validation error: %+v", validationErrors[0])
	}
}

func TestValidateRepoRejectsConfigInfraComposeBackupStrategies(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "host-service"), 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	meta := strings.TrimSpace(`
name: host-service
nodes:
  - main
infra:
  config: {}
data_protect:
  data:
    - name: config
      backup:
        strategy: files.copy_after_stop
        include:
          - ./config
`) + "\n"
	if err := os.WriteFile(filepath.Join(repoDir, "host-service", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write service meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "infra.config") || !strings.Contains(validationErrors[0].Message, "files.copy") {
		t.Fatalf("unexpected validation error: %+v", validationErrors[0])
	}
}

func TestValidateRepoAllowsConfigInfraVolumeIncludes(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "host-service"), 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	meta := strings.TrimSpace(`
name: host-service
nodes:
  - main
infra:
  config: {}
data_protect:
  data:
    - name: config
      backup:
        strategy: files.copy
        include:
          - app-data
`) + "\n"
	if err := os.WriteFile(filepath.Join(repoDir, "host-service", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write service meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 0 {
		t.Fatalf("expected no validation errors, got %+v", validationErrors)
	}
}

func TestValidateRepoRejectsUnsafeDataProtectInclude(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	meta := strings.TrimSpace(`
name: alpha
nodes:
  - main
data_protect:
  data:
    - name: config
      backup:
        strategy: files.copy
        include:
          - /etc
`) + "\n"
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write alpha meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "include") || !strings.Contains(validationErrors[0].Message, "absolute path") {
		t.Fatalf("unexpected validation error: %+v", validationErrors[0])
	}
}

func TestValidateRepoRejectsComposeFilesOutsideServiceDir(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	meta := strings.TrimSpace(`
name: alpha
compose_files:
  - ../compose.yaml
nodes:
  - main
`) + "\n"
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write alpha meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "compose_files") || !strings.Contains(validationErrors[0].Message, "service directory") {
		t.Fatalf("unexpected validation error: %+v", validationErrors[0])
	}
}

func TestValidateRepoRejectsDuplicateComposeFiles(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	meta := strings.TrimSpace(`
name: alpha
compose_files:
  - compose.yaml
  - ./compose.yaml
nodes:
  - main
`) + "\n"
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", MetaFileName), []byte(meta), 0o644); err != nil {
		t.Fatalf("write alpha meta: %v", err)
	}

	validationErrors := ValidateRepo(repoDir, map[string]struct{}{"main": {}})
	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %+v", len(validationErrors), validationErrors)
	}
	if !strings.Contains(validationErrors[0].Message, "compose_files") || !strings.Contains(validationErrors[0].Message, "duplicates") {
		t.Fatalf("unexpected validation error: %+v", validationErrors[0])
	}
}
