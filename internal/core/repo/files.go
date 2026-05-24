package repo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	filecopy "github.com/otiai10/copy"
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

func ListFiles(repoDir, relativePath string, recursive bool) ([]FileEntry, error) {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, relativePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrRepoPathNotFound
		}
		return nil, fmt.Errorf("stat repo path %q: %w", normalizedPath, err)
	}
	if !info.IsDir() {
		return nil, ErrRepoPathNotDirectory
	}
	if recursive {
		return listFilesRecursive(absPath, normalizedPath)
	}
	return listFilesOneLevel(absPath, normalizedPath)

}

func listFilesOneLevel(absPath, normalizedPath string) ([]FileEntry, error) {

	entries, err := os.ReadDir(absPath)
	if err != nil {
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

func listFilesRecursive(absPath, normalizedPath string) ([]FileEntry, error) {
	items := make([]FileEntry, 0)
	err := filepath.WalkDir(absPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absPath {
			return nil
		}
		if entry.Name() == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(absPath, path)
		if err != nil {
			return fmt.Errorf("resolve repo path under %q: %w", normalizedPath, err)
		}
		entryPath := filepath.ToSlash(relPath)
		if normalizedPath != "" {
			entryPath = filepath.ToSlash(filepath.Join(normalizedPath, relPath))
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("read repo entry info for %q: %w", entryPath, err)
		}
		items = append(items, FileEntry{
			Path:  entryPath,
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk repo directory %q: %w", normalizedPath, err)
	}

	sort.Slice(items, func(left, right int) bool {
		leftDepth := pathDepth(items[left].Path)
		rightDepth := pathDepth(items[right].Path)
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		leftParent := filepath.ToSlash(filepath.Dir(items[left].Path))
		rightParent := filepath.ToSlash(filepath.Dir(items[right].Path))
		if leftParent != rightParent {
			return leftParent < rightParent
		}
		if items[left].IsDir != items[right].IsDir {
			return items[left].IsDir
		}
		return items[left].Name < items[right].Name
	})
	return items, nil
}

func pathDepth(path string) int {
	if path == "" {
		return 0
	}
	return strings.Count(path, "/") + 1
}

func ReadFile(repoDir, relativePath string) (FileContent, error) {
	absPath, normalizedPath, err := resolveRepoPath(repoDir, relativePath)
	if err != nil {
		return FileContent{}, err
	}
	if err := rejectSymlinkPath(repoDir, normalizedPath, false); err != nil {
		return FileContent{}, err
	}

	info, err := os.Lstat(absPath)
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
	if err := rejectSymlinkPath(repoDir, normalizedPath, true); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", fmt.Errorf("create repo parent directory for %q: %w", normalizedPath, err)
	}
	file, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("write repo file %q: %w", normalizedPath, err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.WriteString(content); err != nil {
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
	if err := rejectSymlinkPath(repoDir, normalizedPath, true); err != nil {
		return "", err
	}
	if _, err := os.Lstat(absPath); err == nil {
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
	if err := rejectSymlinkPath(repoDir, normalizedPath, false); err != nil {
		return "", err
	}
	if _, err := os.Lstat(absPath); err != nil {
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
	if err := rejectSymlinkPath(repoDir, normalizedSourcePath, false); err != nil {
		return "", "", err
	}
	if err := rejectSymlinkPath(repoDir, normalizedDestinationPath, true); err != nil {
		return "", "", err
	}
	info, err := os.Lstat(absSourcePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrRepoPathNotFound
		}
		return "", "", fmt.Errorf("stat repo source path %q: %w", normalizedSourcePath, err)
	}
	if _, err := os.Lstat(absDestinationPath); err == nil {
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
	if err := rejectSymlinkPath(repoDir, normalizedPath, false); err != nil {
		return PathSnapshot{}, err
	}
	info, err := os.Lstat(absPath)
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
	if normalizedPath == "" {
		return ErrRepoPathInvalid
	}
	if normalizedPath == ".git" || strings.HasPrefix(normalizedPath, ".git/") {
		return ErrRepoPathProtected
	}
	if err := rejectSymlinkPath(repoDir, normalizedPath, true); err != nil {
		return err
	}
	if err := os.RemoveAll(absPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear repo path %q during restore: %w", normalizedPath, err)
	}
	if !snapshot.Exists {
		return nil
	}
	tempPath := filepath.Join(snapshot.TempDir, "data")
	info, err := os.Lstat(tempPath)
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
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source directory %q: %w", sourcePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("copy source directory %q: %w", sourcePath, ErrRepoPathInvalid)
	}
	if !info.IsDir() {
		return fmt.Errorf("copy source directory %q: %w", sourcePath, ErrRepoPathNotDirectory)
	}
	if err := rejectDestinationSymlink(destinationPath); err != nil {
		return err
	}
	if err := rejectCopySourceSymlinks(sourcePath); err != nil {
		return err
	}
	if err := filecopy.Copy(sourcePath, destinationPath, filecopy.Options{
		OnSymlink: func(string) filecopy.SymlinkAction { return filecopy.Skip },
	}); err != nil {
		return fmt.Errorf("copy source directory %q to %q: %w", sourcePath, destinationPath, err)
	}
	return nil
}

func rejectCopySourceSymlinks(sourcePath string) error {
	return filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("copy source path %q: %w", path, ErrRepoPathInvalid)
		}
		return nil
	})
}

func copyFile(sourcePath, destinationPath string, mode os.FileMode) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source file %q: %w", sourcePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("copy source file %q: %w", sourcePath, ErrRepoPathInvalid)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("copy source file %q: %w", sourcePath, ErrRepoPathNotFile)
	}
	if err := rejectDestinationSymlink(destinationPath); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create destination parent for %q: %w", destinationPath, err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", sourcePath, err)
	}
	defer func() { _ = source.Close() }()
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return fmt.Errorf("write destination file %q: %w", destinationPath, err)
	}
	defer func() { _ = destination.Close() }()
	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("copy source file %q to %q: %w", sourcePath, destinationPath, err)
	}
	return nil
}

func rejectDestinationSymlink(path string) error {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("destination path %q: %w", path, ErrRepoPathInvalid)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat destination path %q: %w", path, err)
	}
	return nil
}

func rejectSymlinkPath(repoDir, normalizedPath string, _ bool) error {
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return fmt.Errorf("resolve repo root %q: %w", repoDir, err)
	}
	info, err := os.Lstat(absRepoDir)
	if err != nil {
		return fmt.Errorf("stat repo root %q: %w", repoDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ErrRepoPathInvalid
	}
	if normalizedPath == "" {
		return nil
	}

	parts := strings.Split(filepath.Clean(normalizedPath), string(filepath.Separator))
	current := absRepoDir
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrRepoPathInvalid
		}
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
		absPath, err = securejoin.SecureJoin(absRepoDir, relativePath)
		if err != nil {
			return "", "", fmt.Errorf("resolve repo path %q: %w", relativePath, err)
		}
	}
	cleanAbsPath := filepath.Clean(absPath)
	cleanRepoRoot := filepath.Clean(absRepoDir)
	if cleanAbsPath != cleanRepoRoot && !strings.HasPrefix(cleanAbsPath, cleanRepoRoot+string(filepath.Separator)) {
		return "", "", ErrRepoPathInvalid
	}
	return cleanAbsPath, relativePath, nil
}
