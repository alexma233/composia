package repo

import (
	"fmt"
	"path/filepath"
	"strings"
)

type DataIncludeKind string

const (
	DataIncludeKindServicePath  DataIncludeKind = "service_path"
	DataIncludeKindDockerVolume DataIncludeKind = "docker_volume"
)

func ClassifyDataInclude(include string) (DataIncludeKind, string, error) {
	trimmed := strings.TrimSpace(include)
	if trimmed == "" {
		return "", "", fmt.Errorf("include must not be empty")
	}
	if filepath.IsAbs(trimmed) {
		return "", "", fmt.Errorf("include %q must not be an absolute path", include)
	}

	clean := filepath.Clean(trimmed)
	if clean == "." {
		if trimmed == "." || trimmed == "./" {
			return "", "", fmt.Errorf("include %q must not target the service root", include)
		}
		return DataIncludeKindDockerVolume, trimmed, nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("include %q must stay within the service root", include)
	}
	if strings.HasPrefix(trimmed, "./") || strings.Contains(trimmed, "/") {
		return DataIncludeKindServicePath, clean, nil
	}
	return DataIncludeKindDockerVolume, trimmed, nil
}
