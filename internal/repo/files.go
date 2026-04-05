package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrRepoPathNotFound = errors.New("repo path not found")
var ErrRepoPathInvalid = errors.New("repo path is invalid")
var ErrRepoPathNotDirectory = errors.New("repo path is not a directory")
var ErrRepoPathNotFile = errors.New("repo path is not a file")
var ErrRepoPathProtected = errors.New("repo path is protected")
var ErrRepoPathAlreadyExists = errors.New("repo path already exists")

type FileEntry struct {
	Path  string
	Name  string
	IsDir bool
	Size  int64
}

type FileContent struct {
	Path    string
	Content string
	Size    int64
}

type PathSnapshot struct {
	RelativePath string
	Exists       bool
	TempDir      string
}

func ListFiles(repoDir, relativePath string) ([]FileEntry, error) {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, relativePath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrRepoPathNotFound
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && strings.Contains(pathErr.Err.Error(), "not a directory") {
			return nil, ErrRepoPathNotDirectory
		}
		return nil, fmt.Errorf("read repo directory %q: %w", normalizedPath, err)
	}

	items := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("read repo entry info for %q: %w", entry.Name(), err)
		}
		entryPath := entry.Name()
		if normalizedPath != "" {
			entryPath = filepath.Join(normalizedPath, entry.Name())
		}
		items = append(items, FileEntry{
			Path:  filepath.ToSlash(entryPath),
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	sort.Slice(items, func(left, right int) bool {
		if items[left].IsDir != items[right].IsDir {
			return items[left].IsDir
		}
		return items[left].Name < items[right].Name
	})
	return items, nil
}

func ReadFile(repoDir, relativePath string) (FileContent, error) {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, relativePath)
	if err != nil {
		return FileContent{}, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileContent{}, ErrRepoPathNotFound
		}
		return FileContent{}, fmt.Errorf("stat repo file %q: %w", normalizedPath, err)
	}
	if info.IsDir() {
		return FileContent{}, ErrRepoPathNotFile
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return FileContent{}, fmt.Errorf("read repo file %q: %w", normalizedPath, err)
	}
	return FileContent{
		Path:    filepath.ToSlash(normalizedPath),
		Content: string(content),
		Size:    info.Size(),
	}, nil
}

func WriteFile(repoDir, relativePath, content string) (string, error) {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, relativePath)
	if err != nil {
		return "", err
	}
	if normalizedPath == "" {
		return "", ErrRepoPathInvalid
	}
	if normalizedPath == ".git" || strings.HasPrefix(normalizedPath, ".git/") {
		return "", ErrRepoPathProtected
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", fmt.Errorf("create repo parent directory for %q: %w", normalizedPath, err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write repo file %q: %w", normalizedPath, err)
	}
	return filepath.ToSlash(normalizedPath), nil
}

func CreateDirectory(repoDir, relativePath string) (string, error) {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, relativePath)
	if err != nil {
		return "", err
	}
	if normalizedPath == "" {
		return "", ErrRepoPathInvalid
	}
	if normalizedPath == ".git" || strings.HasPrefix(normalizedPath, ".git/") {
		return "", ErrRepoPathProtected
	}
	if _, err := os.Stat(absPath); err == nil {
		return "", ErrRepoPathAlreadyExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat repo directory %q: %w", normalizedPath, err)
	}
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return "", fmt.Errorf("create repo directory %q: %w", normalizedPath, err)
	}
	placeholderPath := filepath.Join(absPath, ".gitkeep")
	if err := os.WriteFile(placeholderPath, []byte(""), 0o644); err != nil {
		return "", fmt.Errorf("write repo directory placeholder for %q: %w", normalizedPath, err)
	}
	return filepath.ToSlash(normalizedPath), nil
}

func DeletePath(repoDir, relativePath string) (string, error) {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, relativePath)
	if err != nil {
		return "", err
	}
	if normalizedPath == "" {
		return "", ErrRepoPathInvalid
	}
	if normalizedPath == ".git" || strings.HasPrefix(normalizedPath, ".git/") {
		return "", ErrRepoPathProtected
	}
	if _, err := os.Stat(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrRepoPathNotFound
		}
		return "", fmt.Errorf("stat repo path %q: %w", normalizedPath, err)
	}
	if err := os.RemoveAll(absPath); err != nil {
		return "", fmt.Errorf("delete repo path %q: %w", normalizedPath, err)
	}
	return filepath.ToSlash(normalizedPath), nil
}

func MovePath(repoDir, sourcePath, destinationPath string) (string, string, error) {
	absSourcePath, normalizedSourcePath, err := resolveRepoPath(repoDir, sourcePath)
	if err != nil {
		return "", "", err
	}
	absDestinationPath, normalizedDestinationPath, err := resolveRepoPath(repoDir, destinationPath)
	if err != nil {
		return "", "", err
	}
	if normalizedSourcePath == "" || normalizedDestinationPath == "" {
		return "", "", ErrRepoPathInvalid
	}
	if normalizedSourcePath == normalizedDestinationPath {
		return "", "", ErrRepoPathInvalid
	}
	for _, pathValue := range []string{normalizedSourcePath, normalizedDestinationPath} {
		if pathValue == ".git" || strings.HasPrefix(pathValue, ".git/") {
			return "", "", ErrRepoPathProtected
		}
	}
	info, err := os.Stat(absSourcePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrRepoPathNotFound
		}
		return "", "", fmt.Errorf("stat repo source path %q: %w", normalizedSourcePath, err)
	}
	if _, err := os.Stat(absDestinationPath); err == nil {
		return "", "", ErrRepoPathAlreadyExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("stat repo destination path %q: %w", normalizedDestinationPath, err)
	}
	if info.IsDir() && strings.HasPrefix(normalizedDestinationPath+"/", normalizedSourcePath+"/") {
		return "", "", ErrRepoPathInvalid
	}
	if err := os.MkdirAll(filepath.Dir(absDestinationPath), 0o755); err != nil {
		return "", "", fmt.Errorf("create repo destination parent for %q: %w", normalizedDestinationPath, err)
	}
	if err := os.Rename(absSourcePath, absDestinationPath); err != nil {
		return "", "", fmt.Errorf("move repo path %q to %q: %w", normalizedSourcePath, normalizedDestinationPath, err)
	}
	return filepath.ToSlash(normalizedSourcePath), filepath.ToSlash(normalizedDestinationPath), nil
}

func CapturePath(repoDir, relativePath string) (PathSnapshot, error) {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, relativePath)
	if err != nil {
		return PathSnapshot{}, err
	}
	if normalizedPath == "" {
		return PathSnapshot{}, ErrRepoPathInvalid
	}
	if normalizedPath == ".git" || strings.HasPrefix(normalizedPath, ".git/") {
		return PathSnapshot{}, ErrRepoPathProtected
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PathSnapshot{RelativePath: filepath.ToSlash(normalizedPath)}, nil
		}
		return PathSnapshot{}, fmt.Errorf("stat repo path %q: %w", normalizedPath, err)
	}
	tempDir, err := os.MkdirTemp("", "composia-repo-snapshot-*")
	if err != nil {
		return PathSnapshot{}, fmt.Errorf("create path snapshot for %q: %w", normalizedPath, err)
	}
	tempPath := filepath.Join(tempDir, "data")
	if info.IsDir() {
		if err := copyDirectory(absPath, tempPath); err != nil {
			_ = os.RemoveAll(tempDir)
			return PathSnapshot{}, err
		}
	} else {
		if err := copyFile(absPath, tempPath, info.Mode()); err != nil {
			_ = os.RemoveAll(tempDir)
			return PathSnapshot{}, err
		}
	}
	return PathSnapshot{
		RelativePath: filepath.ToSlash(normalizedPath),
		Exists:       true,
		TempDir:      tempDir,
	}, nil
}

func RestorePath(repoDir string, snapshot PathSnapshot) error {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, snapshot.RelativePath)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(absPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear repo path %q during restore: %w", normalizedPath, err)
	}
	if !snapshot.Exists {
		return nil
	}
	tempPath := filepath.Join(snapshot.TempDir, "data")
	info, err := os.Stat(tempPath)
	if err != nil {
		return fmt.Errorf("stat snapshot for %q: %w", normalizedPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("create parent for restore of %q: %w", normalizedPath, err)
	}
	if info.IsDir() {
		return copyDirectory(tempPath, absPath)
	}
	return copyFile(tempPath, absPath, info.Mode())
}

func CleanupPathSnapshot(snapshot PathSnapshot) error {
	if snapshot.TempDir == "" {
		return nil
	}
	return os.RemoveAll(snapshot.TempDir)
}

func copyDirectory(sourcePath, destinationPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source directory %q: %w", sourcePath, err)
	}
	if err := os.MkdirAll(destinationPath, info.Mode().Perm()); err != nil {
		return fmt.Errorf("create destination directory %q: %w", destinationPath, err)
	}
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return fmt.Errorf("read source directory %q: %w", sourcePath, err)
	}
	for _, entry := range entries {
		sourceChild := filepath.Join(sourcePath, entry.Name())
		destinationChild := filepath.Join(destinationPath, entry.Name())
		if entry.IsDir() {
			if err := copyDirectory(sourceChild, destinationChild); err != nil {
				return err
			}
			continue
		}
		childInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat source file %q: %w", sourceChild, err)
		}
		if err := copyFile(sourceChild, destinationChild, childInfo.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(sourcePath, destinationPath string, mode os.FileMode) error {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read source file %q: %w", sourcePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create destination parent for %q: %w", destinationPath, err)
	}
	if err := os.WriteFile(destinationPath, content, mode.Perm()); err != nil {
		return fmt.Errorf("write destination file %q: %w", destinationPath, err)
	}
	return nil
}

func resolveRepoPath(repoDir, relativePath string) (string, string, error) {
	relativePath = filepath.Clean(strings.TrimSpace(relativePath))
	if relativePath == "." {
		relativePath = ""
	}
	if relativePath != "" {
		if filepath.IsAbs(relativePath) {
			return "", "", ErrRepoPathInvalid
		}
		if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			return "", "", ErrRepoPathInvalid
		}
	}

	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve repo root %q: %w", repoDir, err)
	}
	absPath := absRepoDir
	if relativePath != "" {
		absPath = filepath.Join(absRepoDir, relativePath)
	}
	cleanAbsPath := filepath.Clean(absPath)
	cleanRepoRoot := filepath.Clean(absRepoDir)
	if cleanAbsPath != cleanRepoRoot && !strings.HasPrefix(cleanAbsPath, cleanRepoRoot+string(filepath.Separator)) {
		return "", "", ErrRepoPathInvalid
	}
	return cleanAbsPath, relativePath, nil
}
