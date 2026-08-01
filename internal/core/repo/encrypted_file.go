package repo

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const encryptedFileSuffix = ".enc"

var ErrEncryptedFileConflict = errors.New("encrypted and plaintext files cannot coexist")

func IsEncryptedFilePath(path string) bool {
	return strings.HasSuffix(path, encryptedFileSuffix)
}

func RuntimeFilePath(path string) string {
	return strings.TrimSuffix(path, encryptedFileSuffix)
}

func IsValidEncryptedFilePath(path string) bool {
	return IsEncryptedFilePath(path) && filepath.Base(path) != encryptedFileSuffix
}

func HasEncryptedParent(path string) bool {
	parent := filepath.Dir(path)
	for parent != "." && parent != string(filepath.Separator) {
		if IsEncryptedFilePath(filepath.Base(parent)) {
			return true
		}
		parent = filepath.Dir(parent)
	}
	return false
}

func NormalizePath(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." {
		return "", nil
	}
	if filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", ErrRepoPathInvalid
	}
	return path, nil
}

func ValidateEncryptedFilePaths(paths []string) error {
	cleanedPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		cleaned := filepath.ToSlash(filepath.Clean(path))
		if cleaned == "." {
			continue
		}
		if HasEncryptedParent(cleaned) || IsEncryptedFilePath(cleaned) && !IsValidEncryptedFilePath(cleaned) {
			return fmt.Errorf("%w: %q", ErrRepoPathInvalid, cleaned)
		}
		cleanedPaths = append(cleanedPaths, cleaned)
	}
	for _, encryptedPath := range cleanedPaths {
		if !IsEncryptedFilePath(encryptedPath) {
			continue
		}
		runtimePath := RuntimeFilePath(encryptedPath)
		for _, actualPath := range cleanedPaths {
			if actualPath == encryptedPath {
				continue
			}
			if pathIsSameOrChild(actualPath, runtimePath) || pathIsSameOrChild(runtimePath, actualPath) {
				return fmt.Errorf("%w: %q and %q", ErrEncryptedFileConflict, runtimePath, actualPath)
			}
		}
	}
	return nil
}

func pathIsSameOrChild(path, parent string) bool {
	return path == parent || strings.HasPrefix(path, parent+"/")
}
