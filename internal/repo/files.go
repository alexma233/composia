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
