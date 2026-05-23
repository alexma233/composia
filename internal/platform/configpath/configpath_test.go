package configpath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUsesExplicitPath(t *testing.T) {
	got, err := Resolve("/tmp/custom.yaml", []string{"/missing"}, "controller")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "/tmp/custom.yaml" {
		t.Fatalf("path = %q", got)
	}
}

func TestResolveUsesFirstExistingDefault(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "missing.yaml")
	second := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(second, []byte("config"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	got, err := Resolve("", []string{first, second}, "agent")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != second {
		t.Fatalf("path = %q, want %q", got, second)
	}
}

func TestResolveReportsAllDefaults(t *testing.T) {
	_, err := Resolve("", []string{"/missing/one.yaml", "./missing-two.yaml"}, "controller")
	if err == nil || !strings.Contains(err.Error(), "pass --config") || !strings.Contains(err.Error(), "/missing/one.yaml") || !strings.Contains(err.Error(), "./missing-two.yaml") {
		t.Fatalf("unexpected error: %v", err)
	}
}
