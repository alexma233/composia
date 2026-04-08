package repo

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
)

var yamlLinePattern = regexp.MustCompile(`line\s+(\d+)`)

type ValidationError struct {
	Path    string
	Line    uint32
	Message string
}

func ValidateRepo(repoDir string, availableNodeIDs map[string]struct{}) []ValidationError {
	errorsByPath := make([]ValidationError, 0)
	seenServiceNames := make(map[string]string)
	caddyInfraPaths := make([]string, 0, 1)
	rusticInfraPaths := make([]string, 0, 1)

	_ = filepath.WalkDir(repoDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			errorsByPath = append(errorsByPath, ValidationError{Path: relativePath(repoDir, path), Message: walkErr.Error()})
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != MetaFileName {
			return nil
		}

		meta, err := loadServiceMeta(path)
		if err != nil {
			errorsByPath = append(errorsByPath, validationErrorFrom(path, repoDir, err))
			return nil
		}
		if err := validateServiceMeta(path, &meta, availableNodeIDs); err != nil {
			errorsByPath = append(errorsByPath, validationErrorFrom(path, repoDir, err))
			return nil
		}
		if previousPath, exists := seenServiceNames[meta.Name]; exists {
			errorsByPath = append(errorsByPath,
				ValidationError{Path: relativePath(repoDir, previousPath), Message: fmt.Sprintf("service %q is declared more than once", meta.Name)},
				ValidationError{Path: relativePath(repoDir, path), Message: fmt.Sprintf("service %q is declared more than once", meta.Name)},
			)
			return nil
		}
		seenServiceNames[meta.Name] = path
		if meta.Infra != nil && meta.Infra.Caddy != nil {
			caddyInfraPaths = append(caddyInfraPaths, path)
		}
		if meta.Infra != nil && meta.Infra.Rustic != nil {
			rusticInfraPaths = append(rusticInfraPaths, path)
		}
		return nil
	})

	if len(caddyInfraPaths) > 1 {
		for _, path := range caddyInfraPaths {
			errorsByPath = append(errorsByPath, ValidationError{Path: relativePath(repoDir, path), Message: "infra.caddy may only be declared once in the repository"})
		}
	}

	if len(rusticInfraPaths) > 1 {
		for _, path := range rusticInfraPaths {
			errorsByPath = append(errorsByPath, ValidationError{Path: relativePath(repoDir, path), Message: "infra.rustic may only be declared once in the repository"})
		}
	}

	sort.SliceStable(errorsByPath, func(left, right int) bool {
		if errorsByPath[left].Path == errorsByPath[right].Path {
			return errorsByPath[left].Line < errorsByPath[right].Line
		}
		return errorsByPath[left].Path < errorsByPath[right].Path
	})
	return errorsByPath
}

func validationErrorFrom(path, repoDir string, err error) ValidationError {
	line := uint32(0)
	matches := yamlLinePattern.FindStringSubmatch(err.Error())
	if len(matches) == 2 {
		var parsed int
		_, _ = fmt.Sscanf(matches[1], "%d", &parsed)
		if parsed > 0 {
			line = uint32(parsed)
		}
	}
	return ValidationError{
		Path:    relativePath(repoDir, path),
		Line:    line,
		Message: err.Error(),
	}
}

func relativePath(repoDir, path string) string {
	relative, err := filepath.Rel(repoDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
