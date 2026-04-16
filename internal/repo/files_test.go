package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListFilesReturnsOneLevelEntries(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha", "nested"), 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"), []byte("name: alpha\n"), 0o644); err != nil {
		t.Fatalf("write meta file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	entries, err := ListFiles(repoDir, "", false)
	if err != nil {
		t.Fatalf("list root files: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 root entries, got %d", len(entries))
	}
	if entries[0].Path != "alpha" || !entries[0].IsDir {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Path != "README.md" || entries[1].IsDir {
		t.Fatalf("unexpected second entry: %+v", entries[1])
	}

	entries, err = ListFiles(repoDir, "alpha", false)
	if err != nil {
		t.Fatalf("list alpha files: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 alpha entries, got %d", len(entries))
	}
	if entries[0].Path != "alpha/nested" || !entries[0].IsDir {
		t.Fatalf("unexpected nested entry: %+v", entries[0])
	}
	if entries[1].Path != "alpha/composia-meta.yaml" || entries[1].IsDir {
		t.Fatalf("unexpected file entry: %+v", entries[1])
	}
}

func TestListFilesRecursiveReturnsNestedEntries(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha", "nested"), 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"), []byte("name: alpha\n"), 0o644); err != nil {
		t.Fatalf("write meta file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", "nested", "app.yaml"), []byte("demo: true\n"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	entries, err := ListFiles(repoDir, "alpha", true)
	if err != nil {
		t.Fatalf("list recursive alpha files: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 recursive entries, got %d", len(entries))
	}
	if entries[0].Path != "alpha/nested" || !entries[0].IsDir {
		t.Fatalf("unexpected first recursive entry: %+v", entries[0])
	}
	if entries[1].Path != "alpha/composia-meta.yaml" || entries[1].IsDir {
		t.Fatalf("unexpected second recursive entry: %+v", entries[1])
	}
	if entries[2].Path != "alpha/nested/app.yaml" || entries[2].IsDir {
		t.Fatalf("unexpected third recursive entry: %+v", entries[2])
	}
}

func TestReadFileReturnsContentAndSize(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o755); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"), []byte("name: alpha\n"), 0o644); err != nil {
		t.Fatalf("write meta file: %v", err)
	}

	file, err := ReadFile(repoDir, "alpha/composia-meta.yaml")
	if err != nil {
		t.Fatalf("read repo file: %v", err)
	}
	if file.Path != "alpha/composia-meta.yaml" || file.Content != "name: alpha\n" {
		t.Fatalf("unexpected file content: %+v", file)
	}
	if file.Size == 0 {
		t.Fatalf("expected non-zero file size")
	}
}

func TestResolveRepoPathRejectsTraversal(t *testing.T) {
	t.Parallel()

	_, _, err := resolveRepoPath(t.TempDir(), "../etc/passwd")
	if err != ErrRepoPathInvalid {
		t.Fatalf("expected ErrRepoPathInvalid, got %v", err)
	}
}
