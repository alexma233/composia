package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestListFilesReturnsOneLevelEntries(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha", "nested"), 0o750); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"), []byte("name: alpha\n"), 0o600); err != nil {
		t.Fatalf("write meta file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o600); err != nil {
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
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha", "nested"), 0o750); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"), []byte("name: alpha\n"), 0o600); err != nil {
		t.Fatalf("write meta file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", "nested", "app.yaml"), []byte("demo: true\n"), 0o600); err != nil {
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
	if err := os.MkdirAll(filepath.Join(repoDir, "alpha"), 0o750); err != nil {
		t.Fatalf("create alpha dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "alpha", "composia-meta.yaml"), []byte("name: alpha\n"), 0o600); err != nil {
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
	if !errors.Is(err, ErrRepoPathInvalid) {
		t.Fatalf("expected ErrRepoPathInvalid, got %v", err)
	}
}

func TestReadFileRejectsSymlink(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	linkPath := filepath.Join(repoDir, "link.txt")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := ReadFile(repoDir, "link.txt")
	if !errors.Is(err, ErrRepoPathInvalid) {
		t.Fatalf("expected ErrRepoPathInvalid, got %v", err)
	}
}

func TestWriteFileRejectsSymlinkLeaf(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(repoDir, "link.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := WriteFile(repoDir, "link.txt", "changed\n")
	if !errors.Is(err, ErrRepoPathInvalid) {
		t.Fatalf("expected ErrRepoPathInvalid, got %v", err)
	}
	content, err := os.ReadFile(outsidePath) //nolint:gosec
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if string(content) != "secret\n" {
		t.Fatalf("outside file was modified: %q", string(content))
	}
}

func TestDeletePathRejectsSymlinkWithoutDeletingTarget(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(repoDir, "link.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := DeletePath(repoDir, "link.txt")
	if !errors.Is(err, ErrRepoPathInvalid) {
		t.Fatalf("expected ErrRepoPathInvalid, got %v", err)
	}
	if _, err := os.Stat(outsidePath); err != nil {
		t.Fatalf("outside target should remain: %v", err)
	}
}

func TestCapturePathRejectsDirectoryContainingSymlink(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "data"), 0o750); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	outsidePath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(repoDir, "data", "link.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := CapturePath(repoDir, "data")
	if !errors.Is(err, ErrRepoPathInvalid) {
		t.Fatalf("expected ErrRepoPathInvalid, got %v", err)
	}
}

func TestCreateDirectoryWritesPlaceholder(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	path, err := CreateDirectory(repoDir, "alpha/nested")
	if err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if path != "alpha/nested" {
		t.Fatalf("path = %q", path)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "alpha", "nested", ".gitkeep")); err != nil {
		t.Fatalf("expected placeholder: %v", err)
	}
	if _, err := CreateDirectory(repoDir, "alpha/nested"); !errors.Is(err, ErrRepoPathAlreadyExists) {
		t.Fatalf("expected ErrRepoPathAlreadyExists, got %v", err)
	}
}

func TestMovePathMovesFileAndRejectsNestedDirectoryMove(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if _, err := WriteFile(repoDir, "alpha/file.txt", "hello\n"); err != nil {
		t.Fatalf("write file: %v", err)
	}
	from, to, err := MovePath(repoDir, "alpha/file.txt", "bravo/file.txt")
	if err != nil {
		t.Fatalf("move file: %v", err)
	}
	if from != "alpha/file.txt" || to != "bravo/file.txt" {
		t.Fatalf("move result = %q -> %q", from, to)
	}
	content, err := os.ReadFile(filepath.Join(repoDir, "bravo", "file.txt")) //nolint:gosec
	if err != nil {
		t.Fatalf("read moved file: %v", err)
	}
	if string(content) != "hello\n" {
		t.Fatalf("moved content = %q", content)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "dir", "child"), 0o750); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	_, _, err = MovePath(repoDir, "dir", "dir/child/new")
	if !errors.Is(err, ErrRepoPathInvalid) {
		t.Fatalf("expected ErrRepoPathInvalid for nested move, got %v", err)
	}
}

func TestRestorePathRestoresFileAndMissingSnapshot(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if _, err := WriteFile(repoDir, "config.txt", "before\n"); err != nil {
		t.Fatalf("write file: %v", err)
	}
	snapshot, err := CapturePath(repoDir, "config.txt")
	if err != nil {
		t.Fatalf("capture path: %v", err)
	}
	defer func() { _ = CleanupPathSnapshot(snapshot) }()
	if _, err := WriteFile(repoDir, "config.txt", "after\n"); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	if err := RestorePath(repoDir, snapshot); err != nil {
		t.Fatalf("restore path: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(repoDir, "config.txt")) //nolint:gosec
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(content) != "before\n" {
		t.Fatalf("restored content = %q", content)
	}

	missingSnapshot, err := CapturePath(repoDir, "missing.txt")
	if err != nil {
		t.Fatalf("capture missing path: %v", err)
	}
	if _, err := WriteFile(repoDir, "missing.txt", "created\n"); err != nil {
		t.Fatalf("write missing replacement: %v", err)
	}
	if err := RestorePath(repoDir, missingSnapshot); err != nil {
		t.Fatalf("restore missing snapshot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "missing.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing.txt to be removed, stat err=%v", err)
	}
}

func TestCleanupPathSnapshotRemovesTempDir(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if _, err := WriteFile(repoDir, "config.txt", "content\n"); err != nil {
		t.Fatalf("write file: %v", err)
	}
	snapshot, err := CapturePath(repoDir, "config.txt")
	if err != nil {
		t.Fatalf("capture path: %v", err)
	}
	if snapshot.TempDir == "" {
		t.Fatalf("expected snapshot temp dir")
	}
	if err := CleanupPathSnapshot(snapshot); err != nil {
		t.Fatalf("cleanup snapshot: %v", err)
	}
	if _, err := os.Stat(snapshot.TempDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temp dir removal, stat err=%v", err)
	}
	if err := CleanupPathSnapshot(PathSnapshot{}); err != nil {
		t.Fatalf("cleanup empty snapshot: %v", err)
	}
}
